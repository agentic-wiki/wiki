package parse

import (
	"fmt"
	"strings"
	"testing"
)

// checkHeader runs CheckFrontmatter on a header made of lines: the shape
// issues' messages, and the syntax error.
func checkHeader(lines ...string) (shape []string, syntax error) {
	syntax, errs := CheckFrontmatter("---\n" + strings.Join(lines, "\n") + "\n---\nbody\n")
	for _, err := range errs {
		shape = append(shape, err.Error())
	}
	return shape, syntax
}

// The read and check refuse the same headers, with the same message. The one
// exception is a key written twice: the read keeps its last value, and check
// reports it.
func TestReadAndCheckAgree(t *testing.T) {
	big := strings.Repeat("x", 10<<10)
	for name, tc := range map[string]struct {
		content   string
		readReads bool
	}{
		"invalid YAML":        {"---\ntitle: Fix: x\n---\n", false},
		"not a mapping":       {"---\n- a\n- b\n---\n", false},
		"the value bomb":      {aliasBomb(), false},
		"the byte bomb":       {"---\ns: &s " + big + "\nl: [" + strings.TrimSuffix(strings.Repeat("*s, ", 9000), ", ") + "]\n---\n", false},
		"a recursive alias":   {"---\nk: &a [*a]\n---\n", false},
		"1 MiB + 1 byte":      {"---\nk: " + strings.Repeat("a", 1<<20) + "\n---\n", false},
		"unterminated":        {"---\nk: v\n", false},
		"a key written twice": {"---\nk: 1\nk: 2\n---\n", true},
		"nested shapes":       {requesterHeader(6), true},
		"no header":           {"# body\n", true},
	} {
		_, _, _, readErr := ReadFrontmatter(tc.content)
		checkErr := ValidateFrontmatter(tc.content)
		switch {
		case tc.readReads && readErr != nil:
			t.Errorf("%s: the read refuses it: %v", name, readErr)
		case !tc.readReads && (readErr == nil || checkErr == nil || readErr.Error() != checkErr.Error()):
			t.Errorf("%s: read %v, check %v; want the same refusal", name, readErr, checkErr)
		case name == "a key written twice" && checkErr == nil:
			t.Errorf("%s: check passes it", name)
		case tc.readReads && name != "a key written twice" && checkErr != nil:
			t.Errorf("%s: check refuses it: %v", name, checkErr)
		}
	}
}

// The requester's nested shapes pass check with no issue: a list of maps, a
// map of dates, and the anchors and comments around them.
func TestCheckAcceptsNestedShapes(t *testing.T) {
	for name, content := range map[string]string{
		"requester": requesterHeader(6),
		"anchored": "---\ndata_as_of: &d\n  income: 2025-12-31 # audited\n  balance: 2025-09-30\n" +
			"copy: *d\nraw:\n  - {path: a.pdf, meta: {form: 10-K}}\n  - path: b.pdf\n    pages: 2\n---\n",
	} {
		syntax, shape := CheckFrontmatter(content)
		if syntax != nil || len(shape) != 0 {
			t.Errorf("%s: %v, %v", name, syntax, shape)
		}
	}
}

// A key that is a list or a map is reported at any depth, on its own line.
func TestCheckReportsComplexKeyAtDepth(t *testing.T) {
	const msg = "a key that is a list or an object; write a plain key, e.g. k: x"
	for name, tc := range map[string]struct {
		lines []string
		want  string
	}{
		"top level, a list":      {[]string{"t: x", "? [a, b]", ": x"}, "line 3: " + msg},
		"top level, an object":   {[]string{"t: x", "? {a: 1}", ": x"}, "line 3: " + msg},
		"nested in a map":        {[]string{"m:", "  a: 1", "  ? [a, b]", "  : x"}, "line 4: " + msg},
		"nested in a list's map": {[]string{"l:", "  - a: 1", "    ? {b: 2}", "    : x"}, "line 4: " + msg},
	} {
		shape, syntax := checkHeader(tc.lines...)
		if syntax != nil || !contains(shape, tc.want) {
			t.Errorf("%s: %v, %q; want %q", name, syntax, shape, tc.want)
		}
	}
}

// A merge key is reported, at any depth, and not applied by the read.
func TestCheckReportsMergeKey(t *testing.T) {
	shape, syntax := checkHeader("base: &b {a: 1}", "m:", "  <<: *b", "  c: 2")
	if want := `line 4: "<<" merges keys, which wiki does not read; write the keys out`; syntax != nil || !contains(shape, want) {
		t.Errorf("%v, %q; want %q", syntax, shape, want)
	}
	_, typed := readKey(t, "m", "base: &b {a: 1}", "m:", "  <<: *b", "  c: 2")
	if got := Flow(typed); got != `{"<<": {a: 1}, c: 2}` {
		t.Errorf("read %s: the merge key is applied", got)
	}
}

// A key written with escapes is reported with its decoded text, at any depth.
// A quoted key without an escape is fine.
func TestCheckReportsEscapedKey(t *testing.T) {
	shape, syntax := checkHeader(`"st\x61tus": open`, `"my key": v`, "m:", `  "a\x62": 1`, `  'it''s': 2`)
	want := []string{`line 2: key "status" is written with escapes; write it plainly`, `line 5: key "ab" is written with escapes; write it plainly`}
	if syntax != nil || strings.Join(shape, "\n") != strings.Join(want, "\n") {
		t.Errorf("%v\n got %q\nwant %q", syntax, shape, want)
	}
}

// A header whose keys aren't each a scalar key at column 0 is reported: a
// flow-map header (Shade's case C) and a `?` key (case D). A value continued
// on an unindented line (cases A, B, E1, E2 and `foo #bar"`) is YAML's own
// reading, and wiki edits it as such: no issue.
func TestCheckReportsUneditable(t *testing.T) {
	const keys = "wiki can't edit this header safely: write each key on its own line at column 0"
	for name, tc := range map[string]struct {
		lines []string
		want  string
	}{
		"C: a flow-map header": {[]string{"{title: x, type: note}"}, "line 2: " + keys},
		"D: a ? key":           {[]string{"? title", ": x", "type: note"}, "line 2: " + keys},
	} {
		shape, syntax := checkHeader(tc.lines...)
		if syntax != nil || !contains(shape, tc.want) {
			t.Errorf("%s: %v, %q; want %q", name, syntax, shape, tc.want)
		}
	}
	for name, lines := range map[string][]string{
		"A: a key line inside a quoted scalar":     {"title: \"first", "status: x\"", "type: note"},
		"B: a key line inside a flow list":         {"tags: [a,", "status: b]", "type: note"},
		"E1: a quoted value continued at column 0": {"title: \"Ship it", "soon\"", "type: note"},
		"E2: a flow list continued at column 0":    {"tags: [a,", "b]", "type: note"},
		"a # inside a continued quoted value":      {"title: \"a", "foo #bar\"", "type: note"},
		"a comment line at column 0 between keys":  {"title: x", "# note", "type: note"},
	} {
		if shape, syntax := checkHeader(lines...); syntax != nil || len(shape) != 0 {
			t.Errorf("%s: %v, %q; want no issue", name, syntax, shape)
		}
	}
}

// Keys are identified by decoded text alone, at every depth (Shade's design
// finding 3): a tag or quotes don't make a second key. An alias is never
// followed, so a map used twice through it is not a duplicate.
func TestCheckDuplicateByText(t *testing.T) {
	for name, tc := range map[string]struct {
		lines []string
		want  string
	}{
		"a tag":               {[]string{"status: open", "!x status: done"}, `line 3: mapping key "status" already defined at line 2`},
		"quotes, nested":      {[]string{"m: {1: a, \"1\": b}"}, `line 2: mapping key "1" already defined at line 2`},
		"in a list's map":     {[]string{"l:", "  - a: 1", "    b: 2", "    a: 3"}, `line 5: mapping key "a" already defined at line 3`},
		"a key written twice": {[]string{"k: 1", "t: x", "k: 2"}, `line 4: mapping key "k" already defined at line 2`},
	} {
		_, syntax := checkHeader(tc.lines...)
		if syntax == nil || syntax.Error() != tc.want {
			t.Errorf("%s: %v, want %q", name, syntax, tc.want)
		}
	}
	for name, lines := range map[string][]string{
		"a map used twice through an alias": {"a: &x {k: 1}", "b: *x", "c: [*x, *x]"},
		"a recursive anchor's own map":      {"a: &x {k: 1, l: [1]}", "b: {k: 2, x: *x}"},
	} {
		if shape, syntax := checkHeader(lines...); syntax != nil || len(shape) != 0 {
			t.Errorf("%s: %v, %q", name, syntax, shape)
		}
	}
}

// One row per remaining shape message, and the cases that give none. Lines
// count from the start of the file, the opening --- being line 1.
func TestCheckShapeIssues(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header []string
		want   []string
	}{
		// An empty explicit key is also a header wiki can't edit: one message for
		// it, not one per line it leaves outside every span.
		{"empty key", []string{"t: x", "?", ": x"}, []string{
			"line 3: a value with no key; write a key before the colon",
			"line 3: wiki can't edit this header safely: write each key on its own line at column 0",
		}},
		{"null key", []string{"t: x", "~: x"}, []string{"line 3: a value with no key; write a key before the colon"}},
		{"several, in line order", []string{"a: {? [x]: 1}", "b: ok", "\"c\\x20\": 1"}, []string{
			"line 2: a key that is a list or an object; write a plain key, e.g. k: x",
			`line 4: key "c " is written with escapes; write it plainly`,
		}},
		// No issue: scalars, lists, maps, text, comments, blank lines, and
		// values continued on indented lines.
		{"valid shapes", []string{
			"type: note", "n: 10", "tags: [a, b]", "k:", "  - a", "", "  # c", "  - b",
			"d: |", "  line", "", "  # Heading", "e: >", "  folded", "z:", "q: \"a # b\" # c", "v: x # note", "  # trailing",
			"m: {a: 1}", "l:", "  - [x, y]", "  - a: 1", "c: a long", "  value",
		}, nil},
	} {
		got, syntax := checkHeader(tc.header...)
		if syntax != nil || strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
			t.Errorf("%s: %v\n got %q\nwant %q", tc.name, syntax, got, tc.want)
		}
	}
	for _, content := range []string{"# no header\n", "---\n---\n", "---\n# only a comment\n---\n"} {
		if syntax, shape := CheckFrontmatter(content); syntax != nil || shape != nil {
			t.Errorf("%q: %v, %v", content, syntax, shape)
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// A key's tag or anchor comes before its opening quote: the escape is still
// found, at the top level and nested.
func TestCheckReportsEscapedKeyAfterProperties(t *testing.T) {
	const want = `key "status" is written with escapes; write it plainly`
	for name, tc := range map[string]struct {
		lines []string
		line  int
	}{
		"a tag":            {[]string{"!x \"st\\x61tus\": done"}, 2},
		"an anchor":        {[]string{"&a \"st\\x61tus\": done"}, 2},
		"nested, !!str":    {[]string{"m:", "  !!str \"st\\x61tus\": done"}, 3},
		"a tag and anchor": {[]string{"m:", "  !x  &a \"st\\x61tus\": done"}, 3},
	} {
		shape, syntax := checkHeader(tc.lines...)
		if w := fmt.Sprintf("line %d: %s", tc.line, want); syntax != nil || !contains(shape, w) {
			t.Errorf("%s: %v, %q; want %q", name, syntax, shape, w)
		}
	}
}
