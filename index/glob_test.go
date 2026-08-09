package index

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		// exact single-file patterns (no wildcard) still work
		{"/AGENTS.md", "/AGENTS.md", true},
		{"/AGENTS.md", "/sub/AGENTS.md", false}, // exact means root-level only
		{"/a/b.md", "/a/b.md", true},
		{"/a/b.md", "/a/c.md", false},

		// dir/** subtree: the scaffolded ignore_orphans form
		{"/backlog/**", "/backlog/idea.md", true},
		{"/backlog/**", "/backlog/sub/deep.md", true},
		{"/backlog/**", "/archive/idea.md", false},
		{"/backlog/**", "/backlog", true}, // ** matches zero segments

		// * and ? stay within one segment (never cross a /)
		{"/drafts/*.md", "/drafts/a.md", true},
		{"/drafts/*.md", "/drafts/sub/a.md", false},
		{"/*.md", "/a.md", true},
		{"/*.md", "/sub/a.md", false},
		{"/note-?.md", "/note-1.md", true},
		{"/note-?.md", "/note-12.md", false},

		// ** anywhere / everything
		{"/**", "/anything/deep.md", true},
		{"/**/tmp/**", "/tmp/x.md", true},
		{"/**/tmp/**", "/a/b/tmp/c.md", true},
		{"/**/tmp/**", "/a/tmpx/c.md", false},
		{"/a/**/b.md", "/a/b.md", true}, // ** matches zero
		{"/a/**/b.md", "/a/x/y/b.md", true},
		{"/a/**/b.md", "/a/x/c.md", false},

		// character classes (path.Match, within one segment)
		{"/note-[0-9].md", "/note-3.md", true},
		{"/note-[0-9].md", "/note-x.md", false},

		// several ** in one pattern
		{"/**/a/**/b.md", "/x/a/y/z/b.md", true},
		{"/**/a/**/b.md", "/a/b.md", true}, // both ** match zero
		{"/**/a/**/b.md", "/x/y/b.md", false},

		// leading/trailing/degenerate inputs
		{"/**", "/", true}, // ** matches zero segments -> matches the root
		{"/a.md", "/", false},
		{"", "/a.md", false}, // empty pattern matches nothing but empty
		{"", "", true},

		// a malformed pattern never matches (path.Match ErrBadPattern -> no match, no panic)
		{"/a[.md", "/a[.md", false},

		// the path is cleaned before matching: `..` is resolved, not matched literally
		{"/a/**/b.md", "/a/x/../b.md", true},                // -> /a/b.md
		{"/a/**/b.md", "/a/x/y/../../../../../b.md", false}, // -> /b.md (climbs above a, no leading a)
		{"/a/b.md", "/a/./b.md", true},                      // -> /a/b.md
	}
	for _, c := range cases {
		if got := matchGlob(c.pattern, c.path); got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}

func TestMatchAnyGlob(t *testing.T) {
	pats := []string{"/backlog/**", "/archive/**", "/AGENTS.md"}
	if !matchAnyGlob(pats, "/archive/old.md") {
		t.Error("should match a subtree pattern")
	}
	if !matchAnyGlob(pats, "/AGENTS.md") {
		t.Error("should match an exact pattern")
	}
	if matchAnyGlob(pats, "/now/task.md") {
		t.Error("should not match an unlisted path")
	}
	if matchAnyGlob(nil, "/anything.md") {
		t.Error("no patterns must match nothing")
	}
}
