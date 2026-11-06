package parse

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// spanRows is the span table: header lines (after the opening ---), the fields
// Scan finds (key, span and kind, with lines counted from the first header
// line), and what both views read for the key named in read. A header that
// isn't valid YAML reads nothing and has no fields.
var spanRows = []struct {
	lines  []string
	fields string
	read   string // the key whose value is checked; "" for none
	str    any    // Frontmatter's value for read
	typed  any    // TypedFrontmatter's value for read
}{
	{[]string{"k:", "  - a", "  -", "  - b"}, "k 0-3 List", "k", []string{"a", "", "b"}, []any{"a", nil, "b"}},
	{[]string{"k: # c", "  - a"}, "k 0-1 List", "k", []string{"a"}, []any{"a"}},
	{[]string{"k:", "  - a", "# c", "  - b", "t: x"}, "k 0-3 List, t 4 Scalar", "k", []string{"a", "b"}, []any{"a", "b"}},
	{[]string{"k:", "  - a", "", "  - b"}, "k 0-3 List", "k", []string{"a", "b"}, []any{"a", "b"}},
	{[]string{"k:", "  - a", "", "# c", "", "  - b"}, "k 0-5 List", "k", []string{"a", "b"}, []any{"a", "b"}},
	{[]string{"k:", "  - a", "", "t: x"}, "k 0-1 List, t 3 Scalar", "k", []string{"a"}, []any{"a"}},
	{[]string{"k:", "  # note", "  - a"}, "k 0-2 List", "k", []string{"a"}, []any{"a"}},
	{[]string{"k:", "- a"}, "k 0-1 List", "k", []string{"a"}, []any{"a"}},
	{[]string{"k:", "  - a long", "    value"}, "k 0-2 List", "k", []string{"a long value"}, []any{"a long value"}},
	{[]string{"k:", "  - a", "  b: 1"}, "", "", nil, nil}, // not valid YAML: no fields, no read
	{[]string{"k:", "  - a", "a #b: x", "  - b"}, "", "", nil, nil},
	{[]string{"k:", "  - a", "    b # c"}, "k 0-2 List", "k", []string{"a b"}, []any{"a b"}},
	{[]string{"k:", "  -", "    value", "  - b"}, "k 0-3 List", "k", []string{"value", "b"}, []any{"value", "b"}},
	{[]string{"k:", "  - 1", "  - 2.5"}, "k 0-2 List", "k", []string{"1", "2.5"}, []any{json.Number("1"), json.Number("2.5")}},
	{[]string{"d: |", "  line", "", "  line"}, "d 0-3 Text", "d", "line\n\nline\n", "line\n\nline\n"},
	{[]string{"d: |", "  a", "    b", "", "  c"}, "d 0-4 Text", "d", "a\n  b\n\nc\n", "a\n  b\n\nc\n"},
	{[]string{"d: >", "  # Heading", "  text"}, "d 0-2 Text", "d", "# Heading text\n", "# Heading text\n"},
	{[]string{"d: |", "  10"}, "d 0-1 Text", "d", "10\n", "10\n"},
	{[]string{"d: |", "  a # c"}, "d 0-1 Text", "d", "a # c\n", "a # c\n"},
	{[]string{"d: |", "  a", "# c", "  b"}, "", "", nil, nil},
	// A quote opens only at the start of a token: the value, a list item, or a
	// flow-list item. Quoted text is decoded as YAML decodes it.
	{[]string{"title: Don't panic # draft"}, "title 0 Scalar", "title", "Don't panic", "Don't panic"},
	{[]string{"k:", "  - it's # c"}, "k 0-1 List", "k", []string{"it's"}, []any{"it's"}},
	{[]string{`k: "a # b" # c`}, "k 0 Scalar", "k", "a # b", "a # b"},
	{[]string{"k: [it's, b] # c"}, "k 0 List", "k", []string{"it's", "b"}, []any{"it's", "b"}},
	{[]string{`k: ["a, b # c", 'd'] # e`}, "k 0 List", "k", []string{"a, b # c", "d"}, []any{"a, b # c", "d"}},
	{[]string{"k: 'it''s # x' # c"}, "k 0 Scalar", "k", "it's # x", "it's # x"},
	{[]string{`k: "a \" # b" # c`}, "k 0 Scalar", "k", `a " # b`, `a " # b`},
	{[]string{"m:", "  a: 1 # c"}, "m 0-1 Object", "m", "{a: 1}", Map{{"a", json.Number("1")}}},
	{[]string{"m:", "  a: 1"}, "m 0-1 Object", "m", "{a: 1}", Map{{"a", json.Number("1")}}},
	{[]string{"m:", "  a: 1", "  # c", "  b: x"}, "m 0-3 Object", "m", "{a: 1, b: x}", Map{{"a", json.Number("1")}, {"b", "x"}}},
	{[]string{"m: {a: 1}"}, "m 0 Object", "m", "{a: 1}", Map{{"a", json.Number("1")}}},
	{[]string{"k: a long", "  value"}, "k 0-1 Scalar", "k", "a long value", "a long value"},
	{[]string{"k: a", "  b # c"}, "k 0-1 Scalar", "k", "a b", "a b"},
	{[]string{"k: [a,", "  b]"}, "k 0-1 List", "k", []string{"a", "b"}, []any{"a", "b"}},
	{[]string{"k: v", "- x"}, "", "", nil, nil},
	{[]string{"k: v", "  # note"}, "k 0-1 Scalar", "k", "v", "v"},
	{[]string{"  x: 1", "k: v"}, "", "", nil, nil},
	{[]string{"k: 1", "t: x", "k: 2"}, "k 0 Scalar, t 1 Scalar, k 2 Scalar", "k", "2", json.Number("2")},
	{[]string{"k:"}, "k 0 Scalar", "k", "", nil},
	{[]string{"it's: x # c"}, "it's 0 Scalar", "it's", "x", "x"},
	{[]string{"a #b: x"}, "", "", nil, nil},
	{[]string{": x"}, "", "", nil, nil},
}

// describe prints fields as the span table writes them.
func describe(h layout) string {
	var out []string
	for _, f := range h.fields {
		span := fmt.Sprint(f.line - 1)
		if f.end > f.line {
			span += fmt.Sprintf("-%d", f.end-1)
		}
		out = append(out, fmt.Sprintf("%s %s %s", f.key, span, f.kind))
	}
	return strings.Join(out, ", ")
}

func TestScanSpanTable(t *testing.T) {
	for _, row := range spanRows {
		content := "---\n" + strings.Join(row.lines, "\n") + "\n---\n"
		h, err := scanLayout(content)
		if err != nil {
			t.Fatalf("%q: %v", row.lines, err)
		}
		if got := describe(h); got != row.fields {
			t.Errorf("%q: fields %q, want %q", row.lines, got, row.fields)
		}
		strs, _ := Frontmatter(content)
		typed, _ := typedFrontmatter(content)
		if row.read == "" {
			if len(strs) != 0 || len(typed) != 0 {
				t.Errorf("%q: read %v / %v, want nothing", row.lines, strs, typed)
			}
			continue
		}
		if !reflect.DeepEqual(strs[row.read], row.str) || !reflect.DeepEqual(get(typed, row.read), row.typed) {
			t.Errorf("%q: read %#v / %#v, want %#v / %#v", row.lines, strs[row.read], get(typed, row.read), row.str, row.typed)
		}
		t.Logf("%-40s | %-34s | %s", strings.Join(row.lines, " / "), row.fields, readText(row.str))
	}
}

// readText prints a read value for the span-table log.
func readText(v any) string {
	if v == nil {
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// Line numbers count from the start of the file, fences included.
func TestScanLineNumbers(t *testing.T) {
	h, err := scanLayout("---\ntype: note\ntags:\n  - a\n---\n# Body\n")
	if err != nil {
		t.Fatal(err)
	}
	if h.open != 0 || h.close != 4 || describe(h) != "type 0 Scalar, tags 1-2 List" {
		t.Errorf("Open %d, Close %d, fields %q", h.open, h.close, describe(h))
	}
	if h.fields[1].line != 2 || h.fields[1].end != 3 {
		t.Errorf("tags at %d-%d, want 2-3", h.fields[1].line, h.fields[1].end)
	}
}

func TestScanNoHeader(t *testing.T) {
	for _, content := range []string{"# Body\n", "", "---suffix\nk: v\n---\n", " ---\nk: v\n---\n"} {
		h, err := scanLayout(content)
		if err != nil || h.open != -1 || h.close != -1 || len(h.fields) != 0 {
			t.Errorf("%q: %+v, %v", content, h, err)
		}
	}
}

func TestScanUnterminated(t *testing.T) {
	for _, content := range []string{"---\nk: v\n", "---", "---\nk: v\n---suffix\n"} {
		_, err := scanLayout(content)
		if err == nil || err.Error() != "line 1: unterminated frontmatter (missing closing --- fence)" {
			t.Errorf("%q: err = %v", content, err)
		}
	}
}

// A CRLF file scans like an LF one: the \r is not part of any key or value.
func TestScanCRLF(t *testing.T) {
	content := "---\r\nk:\r\n  - a\r\n\r\n  - b\r\nd: |\r\n  x\r\n---\r\n# Body\r\n"
	h, err := scanLayout(content)
	if err != nil || describe(h) != "k 0-3 List, d 4-5 Text" || h.close != 7 {
		t.Fatalf("%q, Close %d, %v", describe(h), h.close, err)
	}
	fm, body := Frontmatter(content)
	if !reflect.DeepEqual(fm["k"], []string{"a", "b"}) || fm["d"] != "x\n" || body != "# Body\r\n" {
		t.Errorf("fm %#v, body %q", fm, body)
	}
}

// Values gives each occurrence of a key what Strings gives the read.
func TestFieldValues(t *testing.T) {
	h, err := scanLayout("---\nk: a\nl: [b, c]\nm:\n  - d\ne:\nk: \"\"\n---\n")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"a"}, {"b", "c"}, {"d"}, nil, nil}
	for i, f := range h.fields {
		if !reflect.DeepEqual(f.values(), want[i]) {
			t.Errorf("%s (line %d): Values() = %#v, want %#v", f.key, f.line, f.values(), want[i])
		}
	}
}

// A key's lines end before the blank lines and column-0 # lines that follow its
// value; an indented # line goes with it; a # line inside a quoted value is
// part of the value.
func TestSpanTrimsTrailingComments(t *testing.T) {
	for _, tc := range []struct {
		lines []string
		want  string
	}{
		{[]string{"k:", "  - a", "", "# about t", "t: x"}, "k 0-1 List, t 4 Scalar"},
		{[]string{"k:", "  - a", "  # still k", "t: x"}, "k 0-2 List, t 3 Scalar"},
		{[]string{"k: \"a", "# in the value\"", "t: x"}, "k 0-1 Scalar, t 2 Scalar"},
		{[]string{"k: v", "# a comment at the end"}, "k 0 Scalar"},
		{[]string{"k: [a, \"x", "#y\"]", "t: x"}, "k 0-1 List, t 2 Scalar"}, // a quoted item, at any depth
	} {
		h, err := scanLayout("---\n" + strings.Join(tc.lines, "\n") + "\n---\n")
		if err != nil || describe(h) != tc.want {
			t.Errorf("%q: %q, %v; want %q", tc.lines, describe(h), err, tc.want)
		}
	}
	// unset removes an indented trailing comment with its key, and leaves a
	// column-0 one.
	got, err := UnsetField("---\nk:\n  - a\n  # still k\n# about t\nt: x\n---\n", "k")
	if want := "---\n# about t\nt: x\n---\n"; err != nil || got != want {
		t.Errorf("unset k: %q, %v", got, err)
	}
	// So does a flow list whose quoted item ends on such a line.
	got, err = SetField("---\nk: [a, \"x\n#y\"]\nt: x\n---\n", "k", "z")
	if want := "---\nk: z\nt: x\n---\n"; err != nil || got != want {
		t.Errorf("set k over a quoted flow item: %q, %v", got, err)
	}
	// A quoted value ending on a line that starts with # goes whole.
	got, err = UnsetField("---\nk: \"a\n# in the value\"\nt: x\n---\n", "k")
	if want := "---\nt: x\n---\n"; err != nil || got != want {
		t.Errorf("unset a quoted k: %q, %v", got, err)
	}
}

// The comment on a key's line is kept with its spacing as written: tabs, or
// several spaces, before the #, and spaces after the comment.
func TestKeyLineCommentSpacing(t *testing.T) {
	for _, tc := range []struct {
		header string
		list   bool // set the key to the list [11], not the scalar 11
		want   string
	}{
		{"n: 10\t# ten", false, "n: 11\t# ten"},
		{"n: 10    # ten", false, "n: 11    # ten"},
		{"n: 10 \t # ten", false, "n: 11 \t # ten"},
		{"n: 10 # ten  ", false, "n: 11 # ten  "},
		{"n: 'a'  # ten", false, "n: '11'  # ten"},
		{"n: # ten\n  - a", true, "n: # ten\n  - \"11\""},
	} {
		content := "---\n" + tc.header + "\n---\n"
		got, err := SetField(content, "n", "11")
		if tc.list {
			got, err = SetFieldList(content, "n", []string{"11"})
		}
		if want := "---\n" + tc.want + "\n---\n"; err != nil || got != want {
			t.Errorf("%q: %v\n got %q\nwant %q", tc.header, err, got, want)
		}
	}
}

// quotedEnd finds where a quoted value ends: an escaped quote right before a
// line break doesn't end it, nor does ” inside single quotes, and an anchor
// or a tag before the opening quote is stepped over. Each value here ends on
// a line starting with #, which the span keeps.
func TestQuotedEnd(t *testing.T) {
	for _, tc := range []struct {
		lines []string
		want  string
	}{
		{[]string{"k: \"a \\\"", "# b\"", "t: x"}, "k 0-1 Scalar, t 2 Scalar"},
		{[]string{"k: 'it''s", "# b'", "t: x"}, "k 0-1 Scalar, t 2 Scalar"},
		{[]string{"k: 'a ''", "# b'", "t: x"}, "k 0-1 Scalar, t 2 Scalar"},
		{[]string{"k: &a \"x", "# y\"", "t: *a"}, "k 0-1 Scalar, t 2 Scalar"},
		{[]string{"k: !!str \"x", "# y\"", "t: x"}, "k 0-1 Scalar, t 2 Scalar"},
		{[]string{"k: !t &a 'x", "# y'", "t: x"}, "k 0-1 Scalar, t 2 Scalar"},
	} {
		h, err := scanLayout("---\n" + strings.Join(tc.lines, "\n") + "\n---\n")
		if err != nil || describe(h) != tc.want {
			t.Errorf("%q: %q, %v; want %q", tc.lines, describe(h), err, tc.want)
		}
	}
}
