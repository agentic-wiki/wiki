package index

import (
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/parse"
)

// Unit level on purpose: controls, NEL and \ can't be file names, so TestWritersRoundTrip can't cover them.
//
// Every link a writer spells reads back through parse.Links and ResolveDest
// to the same Dest and the same suffix: for paths holding the characters a
// link can't carry as written, from a plain source and a <…> one, with every
// suffix, spelled root-absolute and relative. This is the test that catches
// the spelling bugs.
func TestSpellDestRoundTrip(t *testing.T) {
	paths := []string{
		"/a b.md", "/50%.md", "/a#b.md", "/q?.md", "/(x).md", "/<y>.md", "/back\\slash.md",
		"/ctl\x01.md", "/del\x7f.md", "/nel\u0085.md", "/bom\ufeff.md", "/café.md", "/c:/x.md",
		"/d/sub dir/x.md", "/my%20file.md", "/d/a b/%2e%2e/x.md",
	}
	suffixes := []struct{ suffix, anchor string }{{"", ""}, {"?q=1", ""}, {"#x", "x"}, {"?q=1#x", "x"}}
	for _, from := range []string{"/a.md", "/d/a.md"} {
		for _, p := range paths {
			for _, angled := range []bool{false, true} {
				for _, s := range suffixes {
					for _, spelled := range []string{p, RelativeLink(from, p)} {
						dest := spell(spelled, true) + s.suffix
						if angled {
							dest = "<" + dest + ">"
						}
						set := parse.Links("[t](" + dest + " \"title\")\n")
						links := append(set.Absolute, set.Relative...)
						if len(links) != 1 {
							t.Errorf("from %s: %q reads as %+v, want one internal link", from, dest, set)
							continue
						}
						got := ResolveDest(from, links[0].Target)
						if got != (Dest{Path: p, Anchor: s.anchor}) || suffixOf(links[0].Target) != s.suffix {
							t.Errorf("from %s: %q reads back as %+v suffix %q, want %q suffix %q", from, dest, got, suffixOf(links[0].Target), p, s.suffix)
						}
					}
				}
			}
		}
	}
}

// The frontmatter mode encodes only what the resolver would misread (%, #, ?,
// controls, non-printing runes): a space stays a space, and every path reads
// back as itself.
func TestSpellRefRoundTrip(t *testing.T) {
	for _, p := range []string{"/new name.md", "/50%.md", "/a#b.md", "/q?.md", "/ctl\x01.md", "/nel\u0085.md", "/café.md", "/(x) <y>.md"} {
		v := spell(p, false)
		if got := ResolveDest("/d/a.md", v); got != (Dest{Path: p}) {
			t.Errorf("%q spelled %q reads back as %+v", p, v, got)
		}
	}
	if got := spell("/new name.md", false); got != "/new name.md" {
		t.Errorf("a space is spelled %q", got)
	}
}

func TestSpellDestEncodes(t *testing.T) {
	for p, want := range map[string]string{
		"/my b\\.md":    "/my%20b%5C.md",
		"/my%20file.md": "/my%2520file.md",
		"/a#b.md":       "/a%23b.md",
		"/x\n.md":       "/x%0A.md",
		"/LS\u2028.md":  "/LS%E2%80%A8.md",
		"/café.md":      "/café.md",
		"/C:/x.md":      "/C:/x.md",
		"/(a)<b>?.md":   "/%28a%29%3Cb%3E%3F.md",
	} {
		if got := spell(p, true); got != want {
			t.Errorf("spell(%q, true) = %q, want %q", p, got, want)
		}
	}
	if got := RelativeLink("/a.md", "/C:/x.md"); got != "./C:/x.md" {
		t.Errorf("RelativeLink to /C:/x.md = %q, want ./C:/x.md (never a scheme)", got)
	}
	if got := RelativeLink("/d/c.md", "/a#b.md"); got != "../a#b.md" {
		t.Errorf("RelativeLink cut the path at #: %q", got)
	}
}

// The resolver, one row per case of the design's table.
func TestResolveDest(t *testing.T) {
	for _, tc := range []struct {
		from, dest string
		want       Dest
	}{
		{"/d/a.md", "b.md", Dest{Path: "/d/b.md"}},
		{"/d/a.md", "/b.md", Dest{Path: "/b.md"}},
		{"/d/a.md", "../b.md", Dest{Path: "/b.md"}},
		{"/a.md", "../x.md", Dest{Path: "../x.md", Outside: true}},
		{"/a.md", "my%20file.md", Dest{Path: "/my file.md"}},
		{"/a.md", "b.md?raw=1#x", Dest{Path: "/b.md", Anchor: "x"}},
		{"/a.md", "caf%C3%A9.md#caf%C3%A9", Dest{Path: "/café.md", Anchor: "café"}},
		{"/a.md", "%2e%2e/%2e%2e/x.md", Dest{Path: "../../x.md", Outside: true}},
		{"/a.md", "..%2Fx.md", Dest{Path: "../x.md", Outside: true}},
		{"/a.md", "a%2F..%2F..%2Fx.md", Dest{Path: "../x.md", Outside: true}},
		{"/a.md", "a%2Fb.md", Dest{Path: "/a/b.md"}},
		{"/d/a.md", "%2e%2e/x.md", Dest{Path: "/x.md"}},
		{"/a.md", "%252e%252e/x.md", Dest{Path: "/%2e%2e/x.md"}},
		{"/a.md", "/etc/passwd.md", Dest{Path: "/etc/passwd.md"}},
		{"/a.md", "C%3A/x.md", Dest{Path: "/C:/x.md"}},
		{"/a.md", "..x.md", Dest{Path: "/..x.md"}},
		{"/a.md", "/..d/x.md", Dest{Path: "/..d/x.md"}},
		{"/a.md", "bad%zz.md", Dest{Path: "/bad%zz.md"}},
		{"/a.md", "%00.md", Dest{Path: "/%00.md"}},
		{"/a.md", "%FF.md", Dest{Path: "/%FF.md"}},
		{"/a.md", "/../PRD.md", Dest{Path: "../PRD.md", Outside: true}},
		{"/d/a.md", ".", Dest{Path: "/d"}},
		{"/a.md", "/", Dest{Path: "/"}},
	} {
		if got := ResolveDest(tc.from, tc.dest); got != tc.want {
			t.Errorf("ResolveDest(%q, %q) = %+v, want %+v", tc.from, tc.dest, got, tc.want)
		}
	}
	// Climbing far is outside, in linear time: no path is ever built per step.
	if got := ResolveDest("/a.md", strings.Repeat("../", 100000)+"x.md"); !got.Outside {
		t.Errorf("100,000 climbs: %+v, want outside", got.Path[:20])
	}
}

// External destinations are never internal: a scheme, and //, /\ or \\ raw or
// decoded, which a browser reads as another host.
func TestExternalDestinations(t *testing.T) {
	for _, dest := range []string{"C:/x.md", "https://x/y.md", "//evil.example/x.md", "/\\evil.example/x.md", "/%5Cevil.example/x.md", "\\\\evil/x.md", "%2F%2Fevil/x.md"} {
		set := parse.Links("[t](" + dest + ")\n")
		if len(set.External) != 1 || len(set.Absolute)+len(set.Relative) != 0 {
			t.Errorf("%q: %+v, want external", dest, set)
		}
		if !parse.IsExternal(dest) {
			t.Errorf("IsExternal(%q) = false", dest)
		}
	}
	for _, dest := range []string{"/a.md", "a.md", "C%3A/x.md", "..x.md"} {
		if parse.IsExternal(dest) {
			t.Errorf("IsExternal(%q) = true", dest)
		}
	}
}

// Both modes encode \, so a reference to a file whose name holds one reads
// back as that file. One root-absolute ref can't be written: a name starting
// with \ at the root decodes to /\..., which IsExternal reads as another host
// (step 0), so it is never a reference.
func TestSpellEncodesBackslashInBothModes(t *testing.T) {
	if got := spell("/d/\\name.md", false); got != "/d/%5Cname.md" {
		t.Errorf("ref mode: %q", got)
	}
	if d, ok := ref("/x.md", spell("/d/\\name.md", false)); !ok || d.Path != "/d/\\name.md" {
		t.Errorf("a ref to /d/\\name.md reads as %+v %v", d, ok)
	}
	if _, ok := ref("/x.md", spell("/\\name.md", false)); ok {
		t.Error("a root-absolute ref starting /\\ is a ref; step 0 makes it external")
	}
}
