package sheet

import (
	"slices"
	"strings"
	"testing"
)

func TestSafe(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"=1", "'=1"},
		{"-3+A1", "'-3+A1"},
		{"@SUM(A1)", "'@SUM(A1)"},
		{"+", "'+"},
		{"\tcmd", "'\tcmd"},
		{"\rcmd", "'\rcmd"},
		// What a spreadsheet strips before reading the first character.
		{"\ufeff=1+1", "'\ufeff=1+1"},
		{" =x", "' =x"},
		{"\u00a0=x", "'\u00a0=x"},
		{"\ufeff \u00a0@x", "'\ufeff \u00a0@x"},
		// Plain numbers keep their sign, BOM or not.
		{"-3", "-3"},
		{"+1.5", "+1.5"},
		{"-0.5e3", "-0.5e3"},
		{".5", ".5"},
		{"\ufeff-3", "\ufeff-3"},
		// Not a formula start: a full-width equals sign, a unicode minus.
		{"＝1", "＝1"},
		{"−3+A1", "−3+A1"},
		{"", ""},
		{"   ", "   "},
		{"plain", "plain"},
	} {
		if got := Safe([]string{c.in})[0]; got != c.want {
			t.Errorf("Safe(%q) = %q, want %q", c.in, got, c.want)
		}
	}
	in := []string{"=a", "b"}
	if out := Safe(in); !slices.Equal(in, []string{"=a", "b"}) || out[0] != "'=a" {
		t.Errorf("Safe changed its input or missed a cell: in %q, out %q", in, out)
	}
}

// Every CSV field is quoted, so a `;` can't split a cell for Excel in a
// locale whose list separator is `;` (most of Europe): `x;=WEBSERVICE(…)`
// stays one cell of text, never `x` plus a live formula. TSV keeps
// encoding/csv's quoting: that split is Excel's rule for CSV.
func TestWriteQuotesEveryCSVField(t *testing.T) {
	rows := [][]string{{"a", "b"}, {"x;=WEBSERVICE(B9&B10)", "-3"}, {`say "hi"`, ""}}
	var csv, tsv strings.Builder
	if err := Write(&csv, ',', rows); err != nil {
		t.Fatal(err)
	}
	if want := "\"a\",\"b\"\n\"x;=WEBSERVICE(B9&B10)\",\"-3\"\n\"say \"\"hi\"\"\",\"\"\n"; csv.String() != want {
		t.Errorf("csv:\n%q\nwant\n%q", csv.String(), want)
	}
	if err := Write(&tsv, '\t', [][]string{{"a", "=1"}, {"x;y", "-3"}}); err != nil {
		t.Fatal(err)
	}
	if want := "a\t'=1\nx;y\t-3\n"; tsv.String() != want {
		t.Errorf("tsv:\n%q\nwant\n%q", tsv.String(), want)
	}
}
