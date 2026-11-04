package parse

import (
	"strings"
	"testing"
)

// One row per shape message, and the cases that must give none. Lines count
// from the start of the file, the opening --- being line 1.
func TestShapeIssues(t *testing.T) {
	nested := `"m" holds a nested object; values are scalars or lists of scalars (flatten it, e.g. m_a: 1)`
	for _, tc := range []struct {
		name   string
		header []string
		want   []string
	}{
		{"block map", []string{"type: note", "m:", "  a: 1"}, []string{"line 3: " + nested}},
		{"flow map", []string{"m: {a: 1}"}, []string{"line 2: " + nested}},
		{"alias to a map", []string{"base: &b", "  a: 1", "m: *b"}, []string{
			`line 2: "base" holds a nested object; values are scalars or lists of scalars (flatten it, e.g. base_a: 1)`,
			"line 4: " + nested,
		}},
		{"list of objects", []string{"m:", "  - a: 1"}, []string{`line 2: "m" holds a list of objects; values are scalars or lists of scalars`}},
		{"list inside a list", []string{"m: [[a], b]"}, []string{`line 2: "m" holds a list inside a list; values are scalars or lists of scalars`}},
		{"block list inside a list", []string{"m:", "  - - a"}, []string{`line 2: "m" holds a list inside a list; values are scalars or lists of scalars`}},
		{"scalar continues", []string{"k: a long", "  value"}, []string{`line 2: "k" continues on the next line; write the value on one line, or use | or > for text`}},
		{"flow list continues", []string{"t: x", "k: [a,", "  b]"}, []string{`line 3: "k" continues on the next line; write the value on one line, or use | or > for text`}},
		{"item continues", []string{"k:", "  - a", "  - a long", "    value"}, []string{`line 4: "k" continues an item on the next line; write each item on one line`}},
		{"empty key", []string{"t: x", "?", ": x"}, []string{"line 3: a value with no key; write a key before the colon"}},
		{"list as a key", []string{"? [a, b]", ": x"}, []string{"line 2: a key that is a list or an object; write a plain key, e.g. k: x"}},
		{"object as a key", []string{"t: x", "? {a: 1}", ": x"}, []string{"line 3: a key that is a list or an object; write a plain key, e.g. k: x"}},
		{"several, in line order", []string{"a: {x: 1}", "b: ok", "c: one", "  two"}, []string{
			`line 2: "a" holds a nested object; values are scalars or lists of scalars (flatten it, e.g. a_x: 1)`,
			`line 4: "c" continues on the next line; write the value on one line, or use | or > for text`,
		}},
		// No issue: scalars, lists of scalars, text, comments and blank lines.
		{"valid shapes", []string{
			"type: note", "n: 10", "tags: [a, b]", "k:", "  - a", "", "  # c", "  - b",
			"d: |", "  line", "", "  # Heading", "e: >", "  folded", "z:", "q: \"a # b\" # c", "v: x # note", "  # trailing",
		}, nil},
		// ShapeIssues assumes valid YAML; check reports syntax errors first.
		{"invalid YAML", []string{"k:", "  - a", "  b: 1"}, nil},
		{"duplicate key", []string{"m: {a: 1}", "m: {b: 2}"}, nil},
	} {
		content := "---\n" + strings.Join(tc.header, "\n") + "\n---\nbody\n"
		var got []string
		for _, err := range ShapeIssues(content) {
			got = append(got, err.Error())
		}
		if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
	}
	for _, content := range []string{"# no header\n", "---\n---\n", "---\nk: v\n"} {
		if got := ShapeIssues(content); got != nil {
			t.Errorf("%q: %v, want nil", content, got)
		}
	}
}
