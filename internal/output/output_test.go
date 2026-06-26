package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestEmit(t *testing.T) {
	var b bytes.Buffer

	// text: one line each
	if err := Emit(&b, "text", []string{"a", "b"}, nil); err != nil {
		t.Fatal(err)
	}
	if b.String() != "a\nb\n" {
		t.Errorf("text = %q", b.String())
	}

	// json: marshals the value
	b.Reset()
	Emit(&b, "json", nil, struct {
		X int `json:"x"`
	}{5})
	if !strings.Contains(b.String(), `"x": 5`) {
		t.Errorf("json object = %q", b.String())
	}

	// json: a nil slice renders as [] (not null)
	b.Reset()
	var empty []string
	Emit(&b, "json", nil, empty)
	if got := strings.TrimSpace(b.String()); got != "[]" {
		t.Errorf("nil slice json = %q, want []", got)
	}
}

type sample struct {
	Path   string   `json:"path"`
	Count  int      `json:"count"`
	Done   bool     `json:"done"`
	Tags   []string `json:"tags"`
	Hidden string   `json:"-"` // excluded from CSV/TSV columns
}

func TestEmitCSV(t *testing.T) {
	rows := []sample{
		{Path: "/a.md", Count: 2, Done: true, Tags: []string{"x", "y"}, Hidden: "no"},
		{Path: "has,comma", Count: 0, Done: false, Tags: nil},
	}
	var b bytes.Buffer
	if err := Emit(&b, "csv", nil, rows); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	// header from json tags; json:"-" and unexported fields are excluded
	if !strings.HasPrefix(got, "path,count,done,tags\n") {
		t.Errorf("header wrong: %q", got)
	}
	if strings.Contains(got, "Hidden") {
		t.Errorf("json:\"-\" field leaked into csv: %q", got)
	}
	// list field joins with "; "; bool and int render
	if !strings.Contains(got, "/a.md,2,true,x; y") {
		t.Errorf("row 1 wrong: %q", got)
	}
	// a value holding the delimiter gets quoted by encoding/csv
	if !strings.Contains(got, `"has,comma"`) {
		t.Errorf("comma not quoted: %q", got)
	}
}

func TestEmitTSV(t *testing.T) {
	var b bytes.Buffer
	Emit(&b, "tsv", nil, []*sample{{Path: "/a.md", Count: 1, Tags: []string{"x"}}})
	got := b.String()
	if !strings.Contains(got, "path\tcount\tdone\ttags") {
		t.Errorf("tsv header: %q", got)
	}
	if !strings.Contains(got, "/a.md\t1\tfalse\tx") {
		t.Errorf("tsv row: %q", got)
	}
}

func TestEmitDelimitedEdges(t *testing.T) {
	// an empty typed slice still yields the header (column type is known)
	var b bytes.Buffer
	Emit(&b, "csv", nil, []sample{})
	if strings.TrimSpace(b.String()) != "path,count,done,tags" {
		t.Errorf("empty slice should be header only, got %q", b.String())
	}
	// a non-slice result falls back to the text lines
	b.Reset()
	Emit(&b, "csv", []string{"line one", "line two"}, struct{ X int }{1})
	if !strings.Contains(b.String(), "line one") || !strings.Contains(b.String(), "line two") {
		t.Errorf("non-slice csv should fall back to text: %q", b.String())
	}
	// a slice of scalars becomes a single "value" column
	b.Reset()
	Emit(&b, "csv", nil, []string{"a", "b"})
	if !strings.Contains(b.String(), "value\na\nb") {
		t.Errorf("scalar slice csv: %q", b.String())
	}
}
