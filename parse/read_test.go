package parse

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// readKey reads key from a header made of lines, in both views.
func readKey(t *testing.T, key string, lines ...string) (str, typed any) {
	t.Helper()
	strs, m, _, err := ReadFrontmatter("---\n" + strings.Join(lines, "\n") + "\n---\n")
	if err != nil {
		t.Fatalf("%q: %v", lines, err)
	}
	return strs[key], get(m, key)
}

// Quoted text reads as YAML reads it: escapes decoded, spaces inside the
// quotes kept, and a quoted value over two lines joined with a space.
func TestReadQuotedEscapes(t *testing.T) {
	for _, tc := range []struct {
		lines []string
		want  string
	}{
		{[]string{`k: 'company''s'`}, "company's"},
		{[]string{`k: "a \"q\""`}, `a "q"`},
		{[]string{`k: "a\\b"`}, `a\b`},
		{[]string{"k: \"caf\\u00e9\""}, "caf\u00e9"}, // a backslash-u escape
		{[]string{`k: "\t"`}, "\t"},
		{[]string{`k: 'text '`}, "text "},
		{[]string{`k: " a "`}, " a "},
		{[]string{`k: "x`, `  y"`}, "x y"},
	} {
		str, typed := readKey(t, "k", tc.lines...)
		if str != tc.want || typed != tc.want {
			t.Errorf("%q: %q / %#v, want %q", tc.lines, str, typed, tc.want)
		}
	}
}

// Block scalars read as YAML reads them: > folds, | keeps the final newline,
// |- drops it, |+ keeps them all. They are strings in the typed view.
func TestReadBlockScalars(t *testing.T) {
	for _, tc := range []struct {
		lines []string
		want  string
	}{
		{[]string{"k: >", "  one", "  two", "", "  three"}, "one two\nthree\n"},
		{[]string{"k: |", "  l1", "  l2"}, "l1\nl2\n"},
		{[]string{"k: |-", "  l1", "  l2"}, "l1\nl2"},
		{[]string{"k: |+", "  l1", "", "next: x"}, "l1\n\n"},
		{[]string{"k: |", "  10"}, "10\n"},
	} {
		str, typed := readKey(t, "k", tc.lines...)
		if str != tc.want || typed != tc.want {
			t.Errorf("%q: %q / %#v, want %q", tc.lines, str, typed, tc.want)
		}
	}
}

// A plain value continued on the next line is joined with a space, whether it
// is a scalar, a flow list or a block-list item.
func TestReadPlainContinuation(t *testing.T) {
	for _, tc := range []struct {
		lines      []string
		str, typed any
	}{
		{[]string{"k: a long", "  value"}, "a long value", "a long value"},
		{[]string{"k: [a,", "  b]"}, []string{"a", "b"}, []any{"a", "b"}},
		{[]string{"k:", "  - one", "    two"}, []string{"one two"}, []any{"one two"}},
	} {
		str, typed := readKey(t, "k", tc.lines...)
		if !reflect.DeepEqual(str, tc.str) || !reflect.DeepEqual(typed, tc.typed) {
			t.Errorf("%q: %#v / %#v, want %#v / %#v", tc.lines, str, typed, tc.str, tc.typed)
		}
	}
}

// requesterHeader is the shape of the requesting team's dataset pages: six
// scalars, data_as_of (a map of dates), and raw (records items × 7 fields, one
// of them a nested map). They will send a real one for a fixture.
func requesterHeader(records int) string {
	var b strings.Builder
	b.WriteString("---\ntype: dataset\ntitle: 'Acme''s filings'\ncompany: acme\nticker: ACME\ncik: \"0000123456\"\nupdated: 2026-10-01\n")
	b.WriteString("data_as_of:\n  income: 2025-12-31\n  balance: 2025-09-30\nraw:\n")
	for i := range records {
		fmt.Fprintf(&b, "  - path: sources/acme/10-K-%d.pdf\n    sha256: 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a0%d\n", 2015+i, i)
		fmt.Fprintf(&b, "    fetched: 2026-10-01T12:00:00Z\n    pages: %d\n    bytes: 1048576\n    source: sec\n    meta:\n      form: 10-K\n      year: %d\n", 40+i, 2015+i)
	}
	b.WriteString("---\n# Acme\n")
	return b.String()
}

// The requester's nested shapes: raw, a list of maps, reads as one Flow per
// record in the string view; data_as_of, a map, as its Flow. The typed view
// keeps Maps, keys in file order, the top level included.
func TestReadNested(t *testing.T) {
	strs, typed, body, err := ReadFrontmatter(requesterHeader(2))
	if err != nil || body != "# Acme\n" {
		t.Fatalf("err %v, body %q", err, body)
	}
	if got := strs["data_as_of"]; got != "{income: 2025-12-31, balance: 2025-09-30}" {
		t.Errorf("data_as_of string view %q", got)
	}
	wantRaw := []string{
		"{path: sources/acme/10-K-2015.pdf, sha256: 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a00, fetched: 2026-10-01T12:00:00Z, pages: 40, bytes: 1048576, source: sec, meta: {form: 10-K, year: 2015}}",
		"{path: sources/acme/10-K-2016.pdf, sha256: 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a01, fetched: 2026-10-01T12:00:00Z, pages: 41, bytes: 1048576, source: sec, meta: {form: 10-K, year: 2016}}",
	}
	if !reflect.DeepEqual(strs["raw"], wantRaw) {
		t.Errorf("raw string view\n got %#v\nwant %#v", strs["raw"], wantRaw)
	}
	var keys []string
	for _, p := range typed {
		keys = append(keys, p.Key)
	}
	if want := []string{"type", "title", "company", "ticker", "cik", "updated", "data_as_of", "raw"}; !reflect.DeepEqual(keys, want) {
		t.Errorf("top-level keys %q, want file order %q", keys, want)
	}
	if got := get(typed, "data_as_of"); !reflect.DeepEqual(got, Map{{"income", "2025-12-31"}, {"balance", "2025-09-30"}}) {
		t.Errorf("data_as_of typed %#v", got)
	}
	first := get(typed, "raw").([]any)[0]
	want := Map{
		{"path", "sources/acme/10-K-2015.pdf"},
		{"sha256", "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a00"},
		{"fetched", "2026-10-01T12:00:00Z"},
		{"pages", json.Number("40")},
		{"bytes", json.Number("1048576")},
		{"source", "sec"},
		{"meta", Map{{"form", "10-K"}, {"year", json.Number("2015")}}},
	}
	if !reflect.DeepEqual(first, want) {
		t.Errorf("raw[0] typed\n got %#v\nwant %#v", first, want)
	}
}

// Rule T applies at every depth, and dates stay strings as written.
func TestReadRuleTAtDepth(t *testing.T) {
	want := []any{"2025-12-31", "0x1F", json.Number("1.10"), true, nil}
	for name, lines := range map[string][]string{
		"in a list":          {"k: [2025-12-31, 0x1F, 1.10, True, ~]"},
		"in a map in a list": {"k:", "  - m:", "      - 2025-12-31", "      - 0x1F", "      - 1.10", "      - True", "      - ~"},
	} {
		_, typed := readKey(t, "k", lines...)
		got := typed.([]any)
		if name == "in a map in a list" {
			got = get(got[0].(Map), "m").([]any)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %#v, want %#v", name, got, want)
		}
	}
	_, m := readKey(t, "m", "m:", "  d: 2025-12-31", "  n: 1.10", "  b: True", "  z: ~", "  s: '10'")
	if want := (Map{{"d", "2025-12-31"}, {"n", json.Number("1.10")}, {"b", true}, {"z", nil}, {"s", "10"}}); !reflect.DeepEqual(m, want) {
		t.Errorf("in a map: %#v", m)
	}
}

// Keys are their decoded text at every depth: a tag or quotes don't make
// another key, and a key written twice keeps its last value at its first
// position.
func TestKeysByDecodedText(t *testing.T) {
	for _, tc := range []struct {
		lines []string
		key   string
		want  any
	}{
		{[]string{"status: open", "!x status: done"}, "status", "done"},
		{[]string{"1: a", `"1": b`}, "1", "b"},
		{[]string{`"my key": v`}, "my key", "v"},
	} {
		strs, typed, _, err := ReadFrontmatter("---\n" + strings.Join(tc.lines, "\n") + "\n---\n")
		if err != nil || len(typed) != 1 || len(strs) != 1 || strs[tc.key] != tc.want || get(typed, tc.key) != tc.want {
			t.Errorf("%q: %#v / %#v, %v", tc.lines, strs, typed, err)
		}
	}
	_, m := readKey(t, "m", "m:", "  a: 1", "  b: 2", "  a: 3")
	if want := (Map{{"a", json.Number("3")}, {"b", json.Number("2")}}); !reflect.DeepEqual(m, want) {
		t.Errorf("nested key written twice: %#v, want %#v", m, want)
	}
}

// Map marshals to a JSON object in file order, at every depth.
func TestMapJSONFileOrder(t *testing.T) {
	_, typed, _, err := ReadFrontmatter("---\nz: 1\na: {y: [b, {d: 1, c: 2}], x: true}\nm: ~\n---\n")
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(typed)
	if want := `{"z":1,"a":{"y":["b",{"d":1,"c":2}],"x":true},"m":null}`; err != nil || string(got) != want {
		t.Errorf("got %s, %v\nwant %s", got, err, want)
	}
}

// aliasBomb is the 9-level "billion laughs": its first list holds ten copies
// of leaf, and each level after is a list of ten aliases to the one before,
// so the last expands to 10^9 leaves.
func aliasBomb(leaf string) string {
	lines := []string{"a: &a [" + strings.TrimSuffix(strings.Repeat(leaf+",", 10), ",") + "]"}
	for i, name := range "bcdefghi" {
		prev := string("abcdefgh"[i])
		lines = append(lines, fmt.Sprintf("%c: &%c [*%s,*%s,*%s,*%s,*%s,*%s,*%s,*%s,*%s,*%s]", name, name, prev, prev, prev, prev, prev, prev, prev, prev, prev, prev))
	}
	return "---\n" + strings.Join(lines, "\n") + "\n---\n"
}

// padded puts n comment lines of 1,000 characters at the top of a header:
// raw bytes that hold no value and no text.
func padded(content string, n int) string {
	return "---\ntype: note\n" + strings.Repeat("# "+strings.Repeat("0", 998)+"\n", n) + strings.TrimPrefix(content, "---\n")
}

// Only what aliases produce is budgeted, at every use: bombs (whatever their
// leaves, padded or not) and the recursive alias stop at the value budget, and
// a 10 KB scalar aliased 9,000 times at the text budget.
func TestReadAliasBudget(t *testing.T) {
	big := strings.Repeat("x", 10<<10)
	aliases := strings.TrimSuffix(strings.Repeat("*s, ", 9000), ", ")
	for name, tc := range map[string]struct {
		content string
		want    error
	}{
		"9-level bomb":             {aliasBomb(`"x"`), errTooManyValues},
		"9-level bomb of []":       {aliasBomb("[]"), errTooManyValues},
		"9-level bomb of {}":       {aliasBomb("{}"), errTooManyValues},
		"9-level bomb of ~":        {aliasBomb("~"), errTooManyValues},
		"padded 9-level bomb":      {padded(aliasBomb("x"), 1000), errTooManyValues},
		"recursive alias":          {"---\nk: &a [*a]\n---\n", errTooManyValues},
		"recursive alias in a map": {"---\nk: &a {k: *a}\n---\n", errTooManyValues},
		"9,000 × 10 KB":            {"---\ns: &s " + big + "\nl: [" + aliases + "]\n---\n", errTooManyBytes},
	} {
		strs, typed, _, err := ReadFrontmatter(tc.content)
		if !errors.Is(err, tc.want) || len(strs) != 0 || len(typed) != 0 {
			t.Errorf("%s: err %v, want %v; %d / %d keys", name, err, tc.want, len(strs), len(typed))
		}
	}
	// The same aliases, a few times, read fine and expand.
	_, typed := readKey(t, "l", "s: &s v", "l: [*s, *s]")
	if !reflect.DeepEqual(typed, []any{"v", "v"}) {
		t.Errorf("aliases: %#v", typed)
	}
}

// Each alias use is counted, the written definition never: a written list of
// 6,000 values, anchored, reads when used once (6,001 values through the
// alias, the list included) and is refused when used twice (12,002).
func TestAliasBudgetCountsEachUse(t *testing.T) {
	list := "a: &a [" + strings.TrimSuffix(strings.Repeat("0,", 6000), ",") + "]"
	if _, typed := readKey(t, "b", list, "b: *a"); len(typed.([]any)) != 6000 {
		t.Errorf("used once: %d values, want 6000", len(typed.([]any)))
	}
	if _, _, _, err := ReadFrontmatter("---\n" + list + "\nb: [*a, *a]\n---\n"); !errors.Is(err, errTooManyValues) {
		t.Errorf("used twice: %v, want %v", err, errTooManyValues)
	}
}

// What is written in the file has no budget of its own, only the 1 MiB raw
// cap: a map or a list item of 600 KB reads with its Flow, and so does the
// densest written header, 1 MiB of `[0,0,…]` (over 500,000 values). The same
// header through check reports nothing.
func TestWrittenContentHasNoBudget(t *testing.T) {
	big := strings.Repeat("x", 600<<10)
	if str, _ := readKey(t, "m", "m:", "  k: "+big); str != "{k: "+big+"}" {
		t.Error("a map of 600 KB didn't read")
	}
	if str, _ := readKey(t, "l", "l:", "  - ["+big+"]"); !reflect.DeepEqual(str, []string{"[" + big + "]"}) {
		t.Error("a list item of 600 KB didn't read")
	}
	if _, _, _, err := ReadFrontmatter(requesterHeader(6)); err != nil {
		t.Errorf("requester shape: %v", err)
	}

	n := (1<<20 - len("---\nk: []\n")) / 2 // header text, fence included, just under 1 MiB
	zeros := "---\nk: [" + strings.TrimSuffix(strings.Repeat("0,", n), ",") + "]\n---\n"
	_, typed, _, err := ReadFrontmatter(zeros)
	if err != nil {
		t.Fatalf("1 MiB of [0,0,…]: %v", err)
	}
	if got := len(get(typed, "k").([]any)); got != n {
		t.Errorf("1 MiB of [0,0,…]: %d values, want %d", got, n)
	}
	if syntax, shape := CheckFrontmatter(zeros); syntax != nil || len(shape) != 0 {
		t.Errorf("check of 1 MiB of [0,0,…]: %v %v", syntax, shape)
	}
}

// Shade's C-1, in a child process, since a stack overflow can't be recovered
// and would kill the test binary: a recursive alias (`a: &a [*a]`) behind
// 1,040 comment lines of 1,000 characters, just under the 1 MiB raw cap. The
// alias budget refuses it at 10,000 levels, in the read and in check.
func TestPaddedRecursiveAliasIsRefused(t *testing.T) {
	if os.Getenv("PARSE_TEST_C1") != "" {
		content := padded("---\na: &a [*a]\n---\n", 1040)
		_, _, _, err := ReadFrontmatter(content)
		syntax, _ := CheckFrontmatter(content)
		fmt.Printf("%d bytes; read: %v; check: %v\n", len(content), err, syntax)
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestPaddedRecursiveAliasIsRefused$")
	cmd.Env = append(os.Environ(), "PARSE_TEST_C1=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the child died: %v\n%.2000s", err, out)
	}
	want := fmt.Sprintf("read: %v; check: %v\n", errTooManyValues, errTooManyValues)
	if !strings.Contains(string(out), want) {
		t.Errorf("child output %q, want it to end %q", out, want)
	}
	t.Logf("%s", out)
}

// A header over 1 MiB is refused before yaml.v3 sees it: this one isn't valid
// YAML, yet the error is the size. Exactly 1 MiB reads.
func TestReadSizeCap(t *testing.T) {
	header := func(size int) string { // the header text, fence included, is size bytes
		return "---\nk: " + strings.Repeat("a", size-len("---\nk: \n")) + "\n"
	}
	if _, _, _, err := ReadFrontmatter(header(1<<20) + "---\n"); err != nil {
		t.Errorf("1 MiB: %v", err)
	}
	invalid := "---\nk: [" + strings.Repeat("a", 1<<20) + "\n---\n"
	if _, _, _, err := ReadFrontmatter(invalid); !errors.Is(err, errHeaderTooLarge) {
		t.Errorf("1 MiB + 1 byte of invalid YAML: %v, want %v", err, errHeaderTooLarge)
	}
	if _, _, _, err := ReadFrontmatter(header(1<<20+1) + "---\n"); !errors.Is(err, errHeaderTooLarge) {
		t.Errorf("1 MiB + 1 byte: %v, want %v", err, errHeaderTooLarge)
	}
}

// A header that isn't valid YAML reads no fields, says why, and still splits
// off its body.
func TestReadInvalidHeaderHasNoFields(t *testing.T) {
	for _, content := range []string{
		"---\ntitle: Fix: x\ntype: note\n---\n# Body\n",
		"---\n- a\n- b\n---\n# Body\n",
	} {
		strs, typed, body, err := ReadFrontmatter(content)
		if err == nil || len(strs) != 0 || len(typed) != 0 || body != "# Body\n" {
			t.Errorf("%q: err %v, %v / %v, body %q", content, err, strs, typed, body)
		}
	}
	_, _, body, err := ReadFrontmatter("---\nk: v\n")
	if !errors.Is(err, errUnterminated) || body != "---\nk: v\n" {
		t.Errorf("unterminated: err %v, body %q", err, body)
	}
}

// Body splits without decoding: on a heavy header it allocates a handful of
// times (growing its line slice), where a decode allocates per value.
func TestBodyDoesNotDecode(t *testing.T) {
	content := requesterHeader(200)
	if got := Body(content); got != "# Acme\n" {
		t.Fatalf("body %q", got)
	}
	decodes := testing.AllocsPerRun(5, func() { _, _, _, _ = ReadFrontmatter(content) })
	splits := testing.AllocsPerRun(5, func() { _ = Body(content) })
	if splits > 30 || splits*50 > decodes {
		t.Errorf("Body allocates %.0f times, a decode %.0f", splits, decodes)
	}
	for _, c := range []string{"no header\n", "---\nk: v\n", aliasBomb(`"x"`) + "body"} {
		_, _, want, _ := ReadFrontmatter(c)
		if got := Body(c); got != want {
			t.Errorf("%q: Body %q, ReadFrontmatter's body %q", c, got, want)
		}
	}
}

// A field's key is YAML's decoded key.
func TestFieldKeyIsDecoded(t *testing.T) {
	h, err := scanLayout("---\n\"my key\": v\n!x status: open\n'it''s': x\n---\n")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, f := range h.fields {
		keys = append(keys, f.key)
	}
	if want := []string{"my key", "status", "it's"}; !reflect.DeepEqual(keys, want) {
		t.Errorf("keys %q, want %q", keys, want)
	}
}

// A key written twice has a field per occurrence, each with its own value.
func TestFieldValuesPerOccurrence(t *testing.T) {
	h, err := scanLayout("---\nk: 1\nt: x\nk: [a, b]\n---\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(h.fields) != 3 || !reflect.DeepEqual(h.fields[0].values(), []string{"1"}) || !reflect.DeepEqual(h.fields[2].values(), []string{"a", "b"}) {
		t.Errorf("fields %+v", h.fields)
	}
}

// Fields are matched to YAML's keys by line: comments and blank lines before
// the keys, a key after a | block, a key written twice, and CRLF line ends.
// The quoted keys show the match: only YAML unquotes them.
func TestFieldLineMatch(t *testing.T) {
	for _, nl := range []string{"\n", "\r\n"} {
		content := strings.ReplaceAll("---\n# c\n\n\"a\": 1\nd: |\n  text\n\"b\": 2\n# c\n\"a\": 3\n---\n", "\n", nl)
		h, err := scanLayout(content)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, f := range h.fields {
			got = append(got, fmt.Sprintf("%s@%d=%q", f.key, f.line, f.values()))
		}
		want := []string{`a@3=["1"]`, `d@4=["text\n"]`, `b@6=["2"]`, `a@8=["3"]`}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%q: %q, want %q", nl, got, want)
		}
	}
}

// Tags and anchors: only !!str forces a string, another tag changes nothing,
// and an alias reads as the value its anchor names, in both views.
func TestReadAnchorsAndTags(t *testing.T) {
	strs, typed, _, err := ReadFrontmatter("---\ns: !!str 10\nx: !x 10\na: &x 1\nb: *x\n---\n")
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]struct {
		str   string
		typed any
	}{
		"s": {"10", "10"},
		"x": {"10", json.Number("10")},
		"a": {"1", json.Number("1")},
		"b": {"1", json.Number("1")},
	} {
		if strs[key] != want.str || get(typed, key) != want.typed {
			t.Errorf("%s: %#v / %#v, want %#v / %#v", key, strs[key], get(typed, key), want.str, want.typed)
		}
	}
}

// A header that isn't valid YAML gives the same reason, with no "yaml: "
// prefix, from the read, from check, and inside a refused write.
func TestReadErrorHasNoYAMLPrefix(t *testing.T) {
	const content = "---\ntitle: a: b\n---\nbody\n"
	const reason = "line 2: mapping values are not allowed in this context"
	if _, _, _, err := ReadFrontmatter(content); err == nil || err.Error() != reason {
		t.Errorf("read: %v", err)
	}
	if syntax, _ := CheckFrontmatter(content); syntax == nil || syntax.Error() != reason {
		t.Errorf("check: %v", syntax)
	}
	if _, err := SetField(content, "k", "v"); err == nil || err.Error() != "frontmatter not read ("+reason+"); fix the file first" {
		t.Errorf("write: %v", err)
	}
}
