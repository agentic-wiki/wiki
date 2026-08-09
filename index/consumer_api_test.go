package index

import (
	"slices"
	"strings"
	"testing"
)

// The accessors below exist only for library consumers: no command calls them,
// so nothing else would notice if they broke.

// The whole point of the copy is that a consumer reading frontmatter cannot
// edit the engine's state by accident.
func TestFrontmatterIsACopy(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"a.md":     "---\ntype: task\ntitle: A\ntags: [ui, api]\n---\nbody\n",
	})
	e, err := idx.Resolve("/a.md")
	if err != nil {
		t.Fatal(err)
	}

	fm := e.Frontmatter()
	if got := fm["title"]; got != "A" {
		t.Errorf("title=%v", got)
	}
	if got, ok := fm["tags"].([]string); !ok || !slices.Equal(got, []string{"ui", "api"}) {
		t.Fatalf("tags=%#v", fm["tags"])
	}

	// Mutating the returned map, and the slices inside it, must not reach the entry.
	fm["title"] = "clobbered"
	delete(fm, "type")
	fm["tags"].([]string)[0] = "clobbered"

	if e.Field("title") != "A" {
		t.Errorf("mutating the copy changed the entry: title=%q", e.Field("title"))
	}
	if e.Type != "task" {
		t.Errorf("deleting from the copy changed the entry: type=%q", e.Type)
	}
	if got := e.FieldList("tags"); !slices.Equal(got, []string{"ui", "api"}) {
		t.Errorf("mutating a copied list reached the entry: %#v", got)
	}
	// A second call is unaffected by what was done to the first.
	if e.Frontmatter()["title"] != "A" {
		t.Error("a later call returned the mutated value")
	}
}

// A body link is written relative and the graph is keyed root-absolute, so a
// consumer rendering a body needs both directions. These are inverses.
func TestResolveAndRelativeLinkRoundTrip(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md":      "---\nokf_version: \"0.1\"\n---\n",
		"a.md":          "---\ntype: note\n---\n",
		"sub/b.md":      "---\ntype: note\n---\n",
		"sub/deep/c.md": "---\ntype: note\n---\n",
	})

	cases := []struct{ from, written, want string }{
		{"/sub/b.md", "./deep/c.md", "/sub/deep/c.md"},
		{"/sub/b.md", "../a.md", "/a.md"},
		{"/sub/deep/c.md", "../../a.md", "/a.md"},
		{"/a.md", "./sub/b.md", "/sub/b.md"},
		// A root-absolute link still resolves; it is valid, just not canonical.
		{"/sub/b.md", "/a.md", "/a.md"},
		// An anchor rides along with the target.
		{"/sub/b.md", "../a.md#heading", "/a.md#heading"},
	}
	for _, tc := range cases {
		got, outside := idx.ResolveLink(tc.from, tc.written)
		if outside {
			t.Errorf("ResolveLink(%q, %q) reported outside", tc.from, tc.written)
			continue
		}
		if got != tc.want {
			t.Errorf("ResolveLink(%q, %q) = %q, want %q", tc.from, tc.written, got, tc.want)
		}
	}

	// RelativeLink is the inverse: re-spelling a resolved target from the same
	// file must resolve back to it.
	for _, tc := range cases {
		target, _, _ := strings.Cut(tc.want, "#")
		rel := RelativeLink(tc.from, target)
		back, _ := idx.ResolveLink(tc.from, rel)
		if back != target {
			t.Errorf("RelativeLink(%q, %q) = %q, which resolves back to %q", tc.from, target, rel, back)
		}
	}
}

// A target above the bundle root is neither an edge nor broken, and a consumer
// must be able to tell, or it will try to navigate out of the bundle.
func TestResolveLinkReportsOutside(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"sub/b.md": "---\ntype: note\n---\n",
	})
	if _, outside := idx.ResolveLink("/sub/b.md", "../../escape.md"); !outside {
		t.Error("a target above the bundle root should report outside")
	}
	if _, outside := idx.ResolveLink("/sub/b.md", "../index.md"); outside {
		t.Error("a target inside the bundle should not report outside")
	}
}

// The query syntax is part of the query contract, so its parse belongs to the
// library. The subtleties are easy to regress: != is matched before =, so a
// value may contain =, and values are unquoted the way frontmatter is.
func TestParseFilter(t *testing.T) {
	for _, tc := range []struct {
		in      string
		key     string
		value   string
		negate  bool
		wantErr bool
	}{
		{in: "type=task", key: "type", value: "task"},
		{in: "status!=done", key: "status", value: "done", negate: true},
		{in: "status=", key: "status", value: ""},
		{in: "status!=", key: "status", value: "", negate: true},
		// != is matched first, so this is a negation, not a key named "a!".
		{in: "a!=b", key: "a", value: "b", negate: true},
		// A value may contain =, because the split takes the first operator only.
		{in: "url=a=b", key: "url", value: "a=b"},
		{in: `title="quoted value"`, key: "title", value: "quoted value"},
		{in: "nope", wantErr: true},
		{in: "", wantErr: true},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseFilter(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParseFilter(%q) should fail, got %+v", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFilter(%q): %v", tc.in, err)
			}
			if got.Key != tc.key || got.Value != tc.value || got.Negate != tc.negate {
				t.Errorf("ParseFilter(%q) = %+v, want {%q %q %v}", tc.in, got, tc.key, tc.value, tc.negate)
			}
		})
	}
}
