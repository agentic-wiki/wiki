package parse

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// RewriteValues respells values item by item and leaves every other byte; it
// quotes a new value that needs it; and it refuses what SetField refuses.
func TestRewriteValues(t *testing.T) {
	move := map[string]map[string]string{"related": {"/a.md": "/b.md"}}
	for _, tc := range []struct {
		name, header string
		rewrites     map[string]map[string]string
		want         string // the header after; "" when refused
		refusal      string // part of the refusal
	}{
		{"a scalar", "related: /a.md # see", move, "related: /b.md # see", ""},
		{"a flow list, items side by side", "related: [/a.md,/a.md, /c.md]", move, "related: [/b.md,/b.md, /c.md]", ""},
		{"a block list", "related:\n  - /c.md\n  - /a.md # here", move, "related:\n  - /c.md\n  - /b.md # here", ""},
		{"quotes kept", "related: ['/a.md', \"/a.md\"]", move, "related: ['/b.md', \"/b.md\"]", ""},
		{"another key untouched", "see: /a.md\nrelated: /a.md", move, "see: /a.md\nrelated: /b.md", ""},
		{"a longer value isn't cut", "related: [/a.md, /a.md.md]", map[string]map[string]string{"related": {"/a.md": "/b.md", "/a.md.md": "/c.md"}}, "related: [/b.md, /c.md]", ""},
		{"a new value that needs quotes", "related: [/a.md, x]", map[string]map[string]string{"related": {"/a.md": "/x: y.md"}}, `related: ["/x: y.md", x]`, ""},
		{"a quoted value that no longer fits its quote", "related: '/a.md'", map[string]map[string]string{"related": {"/a.md": "/it's.md"}}, `related: "/it's.md"`, ""},
		{"CRLF", "related: /a.md\r", move, "related: /b.md\r", ""},
		// A trailing comment is never respelled.
		{"a comment after a scalar", "related: /a.md # was /a.md", move, "related: /b.md # was /a.md", ""},
		{"a comment after a flow list", "related: [/a.md] # was /a.md", move, "related: [/b.md] # was /a.md", ""},
		{"a comment after a block item", "related:\n  - /a.md # was /a.md", move, "related:\n  - /b.md # was /a.md", ""},
		{"a # inside quotes is no comment", "related: [\"/a.md\", \"x # /a.md\"]", move, "related: [\"/b.md\", \"x # /a.md\"]", ""},
		{"no header change", "type: note", move, "type: note", ""},
		// Refusals, as SetField refuses.
		{"a key written twice", "related: /a.md\nrelated: /c.md", move, "", `has key "related" twice`},
		{"a header that can't be edited safely", "{type: note, related: /a.md}", move, "", "is not a key on its own line at column 0"},
		{"a value continued at column 0 is edited as YAML reads it", "title: \"Ship it\nsoon\"\nrelated: /a.md", move, "title: \"Ship it\nsoon\"\nrelated: /b.md", ""},
		{"a header that doesn't read", "title: Fix: x\nrelated: /a.md", move, "", "not read"},
		{"a quoted item holding it as prose", "related: [/a.md, \"see /a.md\"]", move, "related: [/b.md, \"see /a.md\"]", ""},
		{"a bare item holding it as prose", "related: [/a.md, see /a.md]", move, "related: [/b.md, see /a.md]", ""}, // only whole scalars
		{"a value written with an escape", "related: \"\\x2fa.md\"", move, "", "would not rewrite related as planned"},
		{"another key through an anchor", "related: &r /a.md\nother: *r", move, "", "would change other through an anchor"},
	} {
		content := "---\n" + tc.header + "\n---\nbody\n"
		got, err := rewriteMap(content, tc.rewrites)
		switch {
		case tc.refusal != "":
			var refused *EditError
			if got != "" || !errors.As(err, &refused) || !strings.Contains(err.Error(), tc.refusal) {
				t.Errorf("%s: %q, %v; want a refusal with %q", tc.name, got, err, tc.refusal)
			}
		case err != nil || got != "---\n"+tc.want+"\n---\nbody\n":
			t.Errorf("%s: %v\n got %q\nwant %q", tc.name, err, got, "---\n"+tc.want+"\n---\nbody\n")
		}
	}
	if got, err := rewriteMap("no header\n", move); err != nil || got != "no header\n" {
		t.Errorf("no header: %q, %v", got, err)
	}
}

// A destination holding " #" or ": " is quoted in every context, and each
// rewritten key reads back as exactly the new path (Shade's M-1).
func TestRewriteValuesQuotesBreakingPaths(t *testing.T) {
	for _, dest := range []string{"/b #c.md", "/b: c.md"} {
		for ctx, tc := range map[string]struct {
			header string
			want   any
		}{
			"a plain value": {"related: /a.md", dest},
			"a flow list":   {"related: [/a.md, /x.md]", []string{dest, "/x.md"}},
			"a block list":  {"related:\n  - /a.md\n  - /x.md", []string{dest, "/x.md"}},
		} {
			out, err := rewriteMap("---\n"+tc.header+"\n---\n", map[string]map[string]string{"related": {"/a.md": dest}})
			if err != nil {
				t.Errorf("%q in %s: %v", dest, ctx, err)
				continue
			}
			strs, _, _, err := ReadFrontmatter(out)
			if err != nil || !reflect.DeepEqual(strs["related"], tc.want) {
				t.Errorf("%q in %s: %q reads %#v, %v", dest, ctx, out, strs["related"], err)
			}
		}
	}
}

// RewriteValues respells each scalar where yaml.v3 says it is: two items on
// one line (right to left, so neither moves the other), an anchored item and a
// tagged one (past the property), and a quoted one (its quote kept).
func TestRewriteValuesPositions(t *testing.T) {
	move := map[string]map[string]string{"related": {"/a.md": "/bb.md", "/c.md": "/d.md"}}
	for _, tc := range []struct{ header, want string }{
		{"related: [/a.md, /c.md]", "related: [/bb.md, /d.md]"},
		{"related: [/c.md,/a.md,/c.md]", "related: [/d.md,/bb.md,/d.md]"},
		{"related: [&x /a.md, x]", "related: [&x /bb.md, x]"},
		{"related: [!!str /a.md, x]", "related: [!!str /bb.md, x]"},
		{"related: [!t  &x '/a.md', x]", "related: [!t  &x '/bb.md', x]"},
		{"related:\n  - \"/a.md\" # c\n  - /c.md", "related:\n  - \"/bb.md\" # c\n  - /d.md"},
		{"related: [é, /a.md]", "related: [é, /bb.md]"}, // a column in runes, not bytes
		{"related: café /a.md", "related: café /a.md"},  // not a whole scalar
	} {
		got, err := rewriteMap("---\n"+tc.header+"\n---\n", move)
		if want := "---\n" + tc.want + "\n---\n"; err != nil || got != want {
			t.Errorf("%q: %v\n got %q\nwant %q", tc.header, err, got, want)
		}
	}
}

// rewriteMap is RewriteValues with its rewrites given as key → old → new.
func rewriteMap(content string, m map[string]map[string]string) (string, error) {
	out, _, err := RewriteValues(content, func(key, v string) (string, bool) {
		nv, ok := m[key][v]
		return nv, ok
	})
	return out, err
}

// RewriteValues asks about every occurrence of a key written twice: a rewrite
// in either refuses, whichever line it is on. It counts what it rewrote, and a
// header with nothing to rewrite is returned as it is, editable or not.
func TestRewriteValuesAsksEveryOccurrence(t *testing.T) {
	toB := func(_, v string) (string, bool) { return "/b.md", v == "/a.md" }
	for _, header := range []string{"related: /a.md\nt: x\nrelated: /c.md", "related: /c.md\nt: x\nrelated: /a.md"} {
		out, refs, err := RewriteValues("---\n"+header+"\n---\n", toB)
		var refused *EditError
		if out != "" || refs != 0 || !errors.As(err, &refused) || !strings.Contains(err.Error(), `has key "related" twice (lines 2 and 4)`) {
			t.Errorf("%q: %q, %d, %v", header, out, refs, err)
		}
	}
	out, refs, err := RewriteValues("---\nrelated: [/a.md, /c.md, /a.md]\nsee: /a.md\n---\n", toB)
	if want := "---\nrelated: [/b.md, /c.md, /b.md]\nsee: /b.md\n---\n"; err != nil || refs != 3 || out != want {
		t.Errorf("%q, %d, %v", out, refs, err)
	}
	for _, header := range []string{"{type: note, other: /c.md}", "k: 1\nk: 2"} {
		content := "---\n" + header + "\n---\n"
		if out, refs, err := RewriteValues(content, toB); err != nil || refs != 0 || out != content {
			t.Errorf("%q, nothing to rewrite: %q, %d, %v", header, out, refs, err)
		}
	}
}

// One line holding a flow list of 9,999 quoted items, within every limit,
// costs a check and a rewrite allocation linear in its size: each line's
// rune columns are turned into byte offsets once, not once per scalar. The
// item is non-ASCII, so the conversion can't be skipped. Bytes are counted,
// never time.
func TestColumnsLinearOnOneLongLine(t *testing.T) {
	items := strings.TrimSuffix(strings.Repeat(`"é", `, 9999), ", ") // the list is the 10,000th value
	content := "---\nk: [" + items + "]\n---\n"
	measure := func(f func()) uint64 {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		f()
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}
	limit := uint64(400 * len(content)) // the decode's own nodes included; per-scalar conversion would be ~10,000 × the line
	if got := measure(func() {
		if syntax, shape := CheckFrontmatter(content); syntax != nil || len(shape) != 0 {
			t.Fatalf("check: %v %v", syntax, shape)
		}
	}); got > limit {
		t.Errorf("check allocated %d bytes for a %d-byte header (limit %d)", got, len(content), limit)
	} else {
		t.Logf("check: %d bytes allocated for a %d-byte header", got, len(content))
	}
	var out string
	if got := measure(func() {
		var err error
		out, _, err = RewriteValues(content, func(_, v string) (string, bool) { return "ü", v == "é" })
		if err != nil {
			t.Fatal(err)
		}
	}); got > limit {
		t.Errorf("rewrite allocated %d bytes for a %d-byte header (limit %d)", got, len(content), limit)
	} else {
		t.Logf("rewrite: %d bytes allocated for a %d-byte header", got, len(content))
	}
	if strings.Count(out, `"ü"`) != 9999 {
		t.Errorf("rewrite: %d items rewritten", strings.Count(out, `"ü"`))
	}
}

// One line holding a flow map of 9,998 double-quoted keys, one written with an
// escape: reading it and checking it allocate linearly in its size. escaped()
// looks at every double-quoted key's source, through the same column cache.
func TestEscapedKeysLinearOnOneLongLine(t *testing.T) {
	var b strings.Builder
	b.WriteString("---\nm: {\"\\x61\": 1")
	for i := range 9997 {
		fmt.Fprintf(&b, ", \"é%d\": 1", i)
	}
	b.WriteString("}\n---\n")
	content := b.String()
	measure := func(f func()) uint64 {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		f()
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}
	limit := uint64(400 * len(content))
	for name, f := range map[string]func(){
		"read": func() {
			if _, _, _, err := ReadFrontmatter(content); err != nil {
				t.Fatal(err)
			}
		},
		"check": func() {
			syntax, shape := CheckFrontmatter(content)
			if syntax != nil || len(shape) != 1 || !strings.Contains(shape[0].Error(), `key "a" is written with escapes`) {
				t.Fatalf("check: %v %v", syntax, shape)
			}
		},
	} {
		if got := measure(f); got > limit {
			t.Errorf("%s allocated %d bytes for a %d-byte header (limit %d)", name, got, len(content), limit)
		} else {
			t.Logf("%s: %d bytes allocated for a %d-byte header", name, got, len(content))
		}
	}
}
