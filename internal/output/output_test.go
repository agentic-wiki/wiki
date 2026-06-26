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
