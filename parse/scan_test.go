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
// line), and what both views read for the key named in read.
var spanRows = []struct {
	lines  []string
	fields string
	read   string // the key whose value is checked; "" for none
	str    any    // Frontmatter's value for read
	typed  any    // TypedFrontmatter's value for read
}{
	{[]string{"k:", "  - a", "  -", "  - b"}, "k 0-3 List", "k", []string{"a", "b"}, []any{"a", "b"}},
	{[]string{"k: # c", "  - a"}, "k 0-1 List", "k", []string{"a"}, []any{"a"}},
	{[]string{"k:", "  - a", "# c", "  - b", "t: x"}, "k 0-3 List, t 4 Scalar", "k", []string{"a", "b"}, []any{"a", "b"}},
	{[]string{"k:", "  - a", "", "  - b"}, "k 0-3 List", "k", []string{"a", "b"}, []any{"a", "b"}},
	{[]string{"k:", "  - a", "", "# c", "", "  - b"}, "k 0-5 List", "k", []string{"a", "b"}, []any{"a", "b"}},
	{[]string{"k:", "  - a", "", "t: x"}, "k 0-1 List, t 3 Scalar", "k", []string{"a"}, []any{"a"}},
	{[]string{"k:", "  # note", "  - a"}, "k 0-2 List", "k", []string{"a"}, []any{"a"}},
	{[]string{"k:", "- a"}, "k 0-1 List", "k", []string{"a"}, []any{"a"}},
	{[]string{"k:", "  - a long", "    value"}, "k 0-2 List", "k", []string{"a long\nvalue"}, []any{"a long\nvalue"}},
	{[]string{"k:", "  - a", "  b: 1"}, "k 0-2 List", "k", []string{"a\nb: 1"}, []any{"a\nb: 1"}},
	{[]string{"k:", "  - a", "a #b: x", "  - b"}, "k 0-3 List", "k", []string{"a", "b"}, []any{"a", "b"}},
	{[]string{"k:", "  - a", "    b # c"}, "k 0-2 List", "k", []string{"a\nb"}, []any{"a\nb"}},
	{[]string{"k:", "  -", "    value", "  - b"}, "k 0-3 List", "k", []string{"value", "b"}, []any{"value", "b"}},
	{[]string{"k:", "  - 1", "  - 2.5"}, "k 0-2 List", "k", []string{"1", "2.5"}, []any{json.Number("1"), json.Number("2.5")}},
	{[]string{"d: |", "  line", "", "  line"}, "d 0-3 Text", "d", "line\n\nline", "line\n\nline"},
	{[]string{"d: |", "  a", "    b", "", "  c"}, "d 0-4 Text", "d", "a\n  b\n\nc", "a\n  b\n\nc"},
	{[]string{"d: >", "  # Heading", "  text"}, "d 0-2 Text", "d", "# Heading\ntext", "# Heading\ntext"},
	{[]string{"d: |", "  10"}, "d 0-1 Text", "d", "10", "10"},
	{[]string{"d: |", "  a # c"}, "d 0-1 Text", "d", "a # c", "a # c"},
	{[]string{"d: |", "  a", "# c", "  b"}, "d 0-3 Text", "d", "a\nb", "a\nb"},
	// A quote opens only at the start of a token: the value, a list item, or a
	// flow-list item. '' and \" don't end a quote, though they aren't decoded.
	{[]string{"title: Don't panic # draft"}, "title 0 Scalar", "title", "Don't panic", "Don't panic"},
	{[]string{"k:", "  - it's # c"}, "k 0-1 List", "k", []string{"it's"}, []any{"it's"}},
	{[]string{`k: "a # b" # c`}, "k 0 Scalar", "k", "a # b", "a # b"},
	{[]string{"k: [it's, b] # c"}, "k 0 List", "k", []string{"it's", "b"}, []any{"it's", "b"}},
	{[]string{`k: ["a, b # c", 'd'] # e`}, "k 0 List", "k", []string{"a, b # c", "d"}, []any{"a, b # c", "d"}},
	{[]string{"k: 'it''s # x' # c"}, "k 0 Scalar", "k", "it''s # x", "it''s # x"},
	{[]string{`k: "a \" # b" # c`}, "k 0 Scalar", "k", `a \" # b`, `a \" # b`},
	{[]string{"m:", "  a: 1 # c"}, "m 0-1 Object", "m", "a: 1 # c", "a: 1 # c"},
	{[]string{"m:", "  a: 1"}, "m 0-1 Object", "m", "a: 1", "a: 1"},
	{[]string{"m:", "  a: 1", "  # c", "  b: x"}, "m 0-3 Object", "m", "a: 1\nb: x", "a: 1\nb: x"},
	{[]string{"m: {a: 1}"}, "m 0 Object", "m", "{a: 1}", "{a: 1}"},
	{[]string{"k: a long", "  value"}, "k 0-1 Scalar", "k", "a long\nvalue", "a long\nvalue"},
	{[]string{"k: a", "  b # c"}, "k 0-1 Scalar", "k", "a\nb", "a\nb"},
	{[]string{"k: [a,", "  b]"}, "k 0-1 List", "k", "[a,\nb]", "[a,\nb]"},
	{[]string{"k: v", "- x"}, "k 0-1 Scalar", "k", "v\n- x", "v\n- x"},
	{[]string{"k: v", "  # note"}, "k 0-1 Scalar", "k", "v", "v"},
	{[]string{"  x: 1", "k: v"}, "k 1 Scalar", "k", "v", "v"},
	{[]string{"k: 1", "t: x", "k: 2"}, "k 0 Scalar, t 1 Scalar, k 2 Scalar", "k", "2", json.Number("2")},
	{[]string{"k:"}, "k 0 Scalar", "k", "", nil},
	{[]string{"it's: x # c"}, "it's 0 Scalar", "it's", "x", "x"},
	{[]string{"a #b: x"}, "", "", nil, nil},
	{[]string{": x"}, "", "", nil, nil},
}

// describe prints fields as the span table writes them.
func describe(h Header) string {
	var out []string
	for _, f := range h.Fields {
		span := fmt.Sprint(f.Line - 1)
		if f.End > f.Line {
			span += fmt.Sprintf("-%d", f.End-1)
		}
		out = append(out, fmt.Sprintf("%s %s %s", f.Key, span, f.kind))
	}
	return strings.Join(out, ", ")
}

func TestScanSpanTable(t *testing.T) {
	for _, row := range spanRows {
		content := "---\n" + strings.Join(row.lines, "\n") + "\n---\n"
		h, err := Scan(content)
		if err != nil {
			t.Fatalf("%q: %v", row.lines, err)
		}
		if got := describe(h); got != row.fields {
			t.Errorf("%q: fields %q, want %q", row.lines, got, row.fields)
		}
		strs, _ := Frontmatter(content)
		typed, _ := TypedFrontmatter(content)
		if row.read == "" {
			if len(strs) != 0 || len(typed) != 0 {
				t.Errorf("%q: read %v / %v, want nothing", row.lines, strs, typed)
			}
			continue
		}
		if !reflect.DeepEqual(strs[row.read], row.str) || !reflect.DeepEqual(typed[row.read], row.typed) {
			t.Errorf("%q: read %#v / %#v, want %#v / %#v", row.lines, strs[row.read], typed[row.read], row.str, row.typed)
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
	h, err := Scan("---\ntype: note\ntags:\n  - a\n---\n# Body\n")
	if err != nil {
		t.Fatal(err)
	}
	if h.Open != 0 || h.Close != 4 || describe(h) != "type 0 Scalar, tags 1-2 List" {
		t.Errorf("Open %d, Close %d, fields %q", h.Open, h.Close, describe(h))
	}
	if h.Fields[1].Line != 2 || h.Fields[1].End != 3 {
		t.Errorf("tags at %d-%d, want 2-3", h.Fields[1].Line, h.Fields[1].End)
	}
}

func TestScanNoHeader(t *testing.T) {
	for _, content := range []string{"# Body\n", "", "---suffix\nk: v\n---\n", " ---\nk: v\n---\n"} {
		h, err := Scan(content)
		if err != nil || h.Open != -1 || h.Close != -1 || len(h.Fields) != 0 {
			t.Errorf("%q: %+v, %v", content, h, err)
		}
	}
}

func TestScanUnterminated(t *testing.T) {
	for _, content := range []string{"---\nk: v\n", "---", "---\nk: v\n---suffix\n"} {
		_, err := Scan(content)
		if err == nil || err.Error() != "line 1: unterminated frontmatter (missing closing --- fence)" {
			t.Errorf("%q: err = %v", content, err)
		}
	}
}

// A CRLF file scans like an LF one: the \r is not part of any key or value.
func TestScanCRLF(t *testing.T) {
	content := "---\r\nk:\r\n  - a\r\n\r\n  - b\r\nd: |\r\n  x\r\n---\r\n# Body\r\n"
	h, err := Scan(content)
	if err != nil || describe(h) != "k 0-3 List, d 4-5 Text" || h.Close != 7 {
		t.Fatalf("%q, Close %d, %v", describe(h), h.Close, err)
	}
	fm, body := Frontmatter(content)
	if !reflect.DeepEqual(fm["k"], []string{"a", "b"}) || fm["d"] != "x" || body != "# Body\r\n" {
		t.Errorf("fm %#v, body %q", fm, body)
	}
}

// Values gives each occurrence of a key what Strings gives the read.
func TestFieldValues(t *testing.T) {
	h, err := Scan("---\nk: a\nl: [b, c]\nm:\n  - d\ne:\nk: \"\"\n---\n")
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"a"}, {"b", "c"}, {"d"}, nil, nil}
	for i, f := range h.Fields {
		if !reflect.DeepEqual(f.Values(), want[i]) {
			t.Errorf("%s (line %d): Values() = %#v, want %#v", f.Key, f.Line, f.Values(), want[i])
		}
	}
}
