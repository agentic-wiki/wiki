package output

import (
	"bytes"
	"encoding/json"
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

// csv rows come from the json shape: a slice of structs is one row each, with
// the json names as the header, and every cell by Cell.
func TestEmitCSV(t *testing.T) {
	rows := []sample{
		{Path: "/a.md", Count: 2, Done: true, Tags: []string{"x", "<y>"}, Hidden: "no"},
		{Path: "has,comma", Count: 0, Done: false, Tags: nil},
	}
	var b bytes.Buffer
	if err := Emit(&b, "csv", nil, rows); err != nil {
		t.Fatal(err)
	}
	want := "path,count,done,tags\n/a.md,2,true,\"[\"\"x\"\",\"\"<y>\"\"]\"\n\"has,comma\",0,false,\n"
	if b.String() != want {
		t.Errorf("csv:\n got %q\nwant %q", b.String(), want)
	}
}

func TestEmitTSV(t *testing.T) {
	var b bytes.Buffer
	Emit(&b, "tsv", nil, []*sample{{Path: "/a.md", Count: 1, Tags: []string{"x"}}})
	if want := "path\tcount\tdone\ttags\n/a.md\t1\tfalse\t\"[\"\"x\"\"]\"\n"; b.String() != want {
		t.Errorf("tsv:\n got %q\nwant %q", b.String(), want)
	}
}

func TestEmitRowsEdges(t *testing.T) {
	// an empty typed slice still yields the header (column type is known)
	var b bytes.Buffer
	Emit(&b, "csv", nil, []sample{})
	if b.String() != "path,count,done,tags\n" {
		t.Errorf("empty slice should be header only, got %q", b.String())
	}
	// a single struct is one row, never the text lines
	b.Reset()
	Emit(&b, "csv", []string{"line one"}, struct {
		X int    `json:"x"`
		Y string `json:"y"`
	}{1, "a\nb"})
	if want := "x,y\n1,\"a\nb\"\n"; b.String() != want {
		t.Errorf("struct csv: got %q, want %q", b.String(), want)
	}
	// anything else has no rows: an error, not a fallback
	b.Reset()
	if err := Emit(&b, "csv", []string{"a"}, []string{"a", "b"}); err == nil || b.Len() != 0 {
		t.Errorf("scalar slice csv: %q, %v; want an error and no output", b.String(), err)
	}
}

func TestCell(t *testing.T) {
	for _, tc := range []struct {
		v    any
		want string
	}{
		{nil, ""},
		{"text, as is", "text, as is"},
		{json.Number("1.10"), "1.10"},
		{true, "true"},
		{3, "3"},
		{[]string{"a", "b"}, `["a","b"]`},
		{[]any{json.Number("1"), "<b>", nil}, `[1,"<b>",null]`},
		{[]string{}, ""},
		{[]string(nil), ""},
		{map[string]int{"a": 1}, `{"a":1}`},
	} {
		if got := Cell(tc.v); got != tc.want {
			t.Errorf("Cell(%#v) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestTable(t *testing.T) {
	header := []string{"date", "amount"}
	rows := [][]string{{"2026-01", "100"}, {"2026-02", "200"}}
	values := [][]any{{"2026-01", json.Number("100")}, {"2026-02", json.Number("200")}}

	var csvb bytes.Buffer
	if err := Table(&csvb, "csv", header, rows, values); err != nil {
		t.Fatal(err)
	}
	if got := csvb.String(); !strings.Contains(got, "date,amount\n") || !strings.Contains(got, "2026-01,100\n") {
		t.Errorf("csv = %q", got)
	}

	var tsvb bytes.Buffer
	Table(&tsvb, "tsv", header, rows, values)
	if got := tsvb.String(); !strings.Contains(got, "date\tamount\n") {
		t.Errorf("tsv = %q", got)
	}

	// json: array of row objects with keys in HEADER order, not alphabetical
	var jsb bytes.Buffer
	Table(&jsb, "json", header, rows, values)
	js := jsb.String()
	if !strings.Contains(js, `"date": "2026-01"`) || !strings.Contains(js, `"amount": 100`) {
		t.Errorf("json = %q", js)
	}
	if strings.Index(js, `"date"`) > strings.Index(js, `"amount"`) {
		t.Errorf("json column order not preserved (a map would sort amount before date): %q", js)
	}

	// text: aligned columns, values present
	var txt bytes.Buffer
	Table(&txt, "text", header, rows, values)
	if got := txt.String(); !strings.Contains(got, "date") || !strings.Contains(got, "2026-02") {
		t.Errorf("text = %q", got)
	}
}

// jsonl prints the same rows as csv, one compact object per row with the
// row's own keys (a Missing value is left out), values in their JSON types,
// and nothing at all for an empty result.
func TestEmitJSONL(t *testing.T) {
	rows := Rows{
		Header: []string{"_path", "n", "tags", "z"},
		Values: [][]any{
			{"/a.md", json.Number("10"), []any{"x", "<y>"}, nil},
			{"/b.md", Missing, Missing, "s"},
		},
	}
	var b bytes.Buffer
	if err := EmitRows(&b, "jsonl", nil, nil, rows); err != nil {
		t.Fatal(err)
	}
	want := `{"_path":"/a.md","n":10,"tags":["x","<y>"],"z":null}` + "\n" + `{"_path":"/b.md","z":"s"}` + "\n"
	if b.String() != want {
		t.Errorf("jsonl:\n got %q\nwant %q", b.String(), want)
	}
	// csv of the same rows: Missing and nil are both empty cells
	b.Reset()
	EmitRows(&b, "csv", nil, nil, rows)
	if want := "_path,n,tags,z\n/a.md,10,\"[\"\"x\"\",\"\"<y>\"\"]\",\n/b.md,,,s\n"; b.String() != want {
		t.Errorf("csv:\n got %q\nwant %q", b.String(), want)
	}
	// an empty result is nothing in jsonl, the header alone in csv
	b.Reset()
	Emit(&b, "jsonl", nil, []sample{})
	if b.Len() != 0 {
		t.Errorf("empty jsonl = %q, want nothing", b.String())
	}
	// struct rows honour omitempty, as json does
	b.Reset()
	Emit(&b, "jsonl", nil, []struct {
		A string `json:"a"`
		B int    `json:"b,omitempty"`
	}{{"x", 0}, {"y", 2}})
	if want := `{"a":"x"}` + "\n" + `{"a":"y","b":2}` + "\n"; b.String() != want {
		t.Errorf("omitempty jsonl = %q, want %q", b.String(), want)
	}
}

// A field marked omitempty is Missing exactly when json leaves it out: an
// empty slice (nil or not), map or string, false, 0 or nil, but never a struct.
func TestRowsOfOmitempty(t *testing.T) {
	type inner struct {
		A int `json:"a"`
	}
	rows, err := RowsOf(struct {
		S  []string       `json:"s,omitempty"`
		M  map[string]int `json:"m,omitempty"`
		T  string         `json:"t,omitempty"`
		B  bool           `json:"b,omitempty"`
		St inner          `json:"st,omitempty"`
		N  int            `json:"n"`
	}{S: []string{}, M: map[string]int{}})
	if err != nil {
		t.Fatal(err)
	}
	got := rows.Values[0]
	for i, want := range []bool{true, true, true, true, false, false} {
		if _, isMissing := got[i].(missing); isMissing != want {
			t.Errorf("%s: Missing = %v, want %v (value %#v)", rows.Header[i], isMissing, want, got[i])
		}
	}
}
