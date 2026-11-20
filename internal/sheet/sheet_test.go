package sheet

import (
	"slices"
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
