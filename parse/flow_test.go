package parse

import (
	"encoding/json"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// Every row of the ADR's Flow table, the typed words, the runes that must be
// escaped and keys that need quotes: each gives its exact text, and reads back
// as the same typed value.
func TestFlowRoundTrip(t *testing.T) {
	record := Map{
		{"path", "sources/acme/10-K.pdf"},
		{"sha256", "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"},
		{"fetched", "2026-10-01T12:00:00Z"},
		{"pages", json.Number("40")},
		{"meta", Map{{"form", "10-K"}, {"year", json.Number("2015")}}},
	}
	rows := []struct {
		v    any
		want string
	}{
		{Map{{"income", "2025-12-31"}, {"balance", "2025-09-30"}}, "{income: 2025-12-31, balance: 2025-09-30}"},
		{[]any{"finance", "10-K", "sec"}, "[finance, 10-K, sec]"},
		{[]any{"10", "yes", "null", "0x1F", ".5", "1_000"}, `["10", "yes", "null", "0x1F", ".5", "1_000"]`},
		{[]any{json.Number("1.10"), true, nil, nil}, "[1.10, true, null, null]"},
		{[]any{"it's", "a, b", "x: y", " pad ", "#c", "-x"}, `["it's", "a, b", "x: y", " pad ", "#c", "-x"]`},
		{"\x01", `"\x01"`},
		{"\u2028", `"\u2028"`},
		{"l1\nl2\n", `"l1\nl2\n"`},
		{Map{{"1", "a"}, {"a: b", "c"}, {"", "d"}}, `{"1": a, "a: b": c, "": d}`},
		{record, "{path: sources/acme/10-K.pdf, sha256: 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08, fetched: 2026-10-01T12:00:00Z, pages: 40, meta: {form: 10-K, year: 2015}}"},
		{[]any{}, "[]"},
		{Map{}, "{}"},
		{"", `""`},
		{"a b:c", "a b:c"},
		{"café", "café"},
		// A key that is exactly << is quoted: bare, it would read as a merge key.
		{Map{{"<<", Map{{"a", json.Number("1")}}}, {"c", json.Number("2")}}, `{"<<": {a: 1}, c: 2}`},
		// A ? anywhere is quoted: yaml.v3 can't read `[Why?, b]`.
		{[]any{"Why?", "a?b", "a ?", "?x"}, `["Why?", "a?b", "a ?", "?x"]`},
		// keys needing quotes
		{Map{{"#k", "v"}, {"true", "v"}, {"-k", "v"}, {"k:", "v"}}, `{"#k": v, "true": v, "-k": v, "k:": v}`},
	}
	// The typed words: each one alone is quoted.
	for _, w := range []string{"yes", "No", "on", "y", "+1", ".5", "1.", "0x1F", "0o17", "0b101", "1_000", "1:20", ".inf", ".NaN", "~", "Null", "OFF"} {
		rows = append(rows, struct {
			v    any
			want string
		}{w, `"` + w + `"`})
	}
	// The runes that a bare string must not hold: each is escaped.
	for r, esc := range map[rune]string{0x01: `\x01`, 0x7f: `\x7f`, 0x85: `\u0085`, 0x2028: `\u2028`, 0x2029: `\u2029`, 0xfeff: `\ufeff`, 0xfffe: `\ufffe`, 0xa0: `\u00a0`} {
		rows = append(rows, struct {
			v    any
			want string
		}{"a" + string(r) + "b", `"a` + esc + `b"`})
	}
	for _, row := range rows {
		got := Flow(row.v)
		if got != row.want {
			t.Errorf("Flow(%#v) = %s, want %s", row.v, got, row.want)
			continue
		}
		_, typed, _, err := ReadFrontmatter("---\nk: " + got + "\n---\n")
		if back := get(typed, "k"); err != nil || !reflect.DeepEqual(back, row.v) {
			t.Errorf("%s reads back %#v (%v), want %#v", got, back, err, row.v)
		}
	}
}

// Flow is always one line: no \n, \r or other YAML line break, however the
// value is made.
func TestFlowOneLine(t *testing.T) {
	long := make([]any, 300)
	for i := range long {
		long[i] = "item"
	}
	var deep any = "leaf"
	for range 6 {
		deep = Map{{"k", deep}, {"n", json.Number("1")}}
	}
	for _, v := range []any{"l1\nl2\n", long, deep, "a\u2028b", []any{"x\r\ny", Map{{"t", "a\tb"}}}} {
		if got := Flow(v); strings.ContainsAny(got, "\n\r\u0085\u2028\u2029") {
			t.Errorf("Flow(%#v) = %q: more than one line", v, got)
		}
	}
}

// Empty is the one "no value" test.
func TestEmpty(t *testing.T) {
	for _, tc := range []struct {
		yaml  string
		empty bool
	}{
		{"", true}, // absent: Get gives nil
		{"k: ~", true},
		{`k: ""`, true},
		{"k: []", true},
		{"k: {}", true},
		{`k: [""]`, true},
		{"k: [~]", true},
		{"k:\n  -", true},
		{"k: [a]", false},
		{"k: {a: 1}", false},
		{"k: 0", false},
		{"k: false", false},
	} {
		_, typed, _, err := ReadFrontmatter("---\n" + tc.yaml + "\n---\n")
		if err != nil {
			t.Fatalf("%q: %v", tc.yaml, err)
		}
		if got := Empty(get(typed, "k")); got != tc.empty {
			t.Errorf("%q: Empty = %v, want %v", tc.yaml, got, tc.empty)
		}
	}
}

// Flow writes each byte once, into one buffer: 9,000 levels of nesting around
// a 100 KB scalar allocate less than 4 times the output (the buffer doubling
// as it grows). Building the text per node would copy the scalar at every
// level, about 900 MB here. Bytes are counted, never time.
func TestFlowBytesLinear(t *testing.T) {
	var v any = strings.Repeat("x", 100<<10)
	for range 9000 {
		v = []any{v}
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	out := Flow(v)
	runtime.ReadMemStats(&after)
	if want := 9000*2 + 100<<10; len(out) != want {
		t.Fatalf("output %d bytes, want %d", len(out), want)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc >= 4*uint64(len(out)) {
		t.Errorf("Flow allocated %d bytes for %d bytes of output", alloc, len(out))
	}
}
