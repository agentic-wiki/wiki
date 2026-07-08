package wikilink

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name, in string
		want     []Link
	}{
		{"basic", "See [[note]].", []Link{{Raw: "note", Line: 1}}},
		{"embed", "![[image.png]]", []Link{{Raw: "image.png", IsEmbed: true, Line: 1}}},
		{"display", "[[note|display text]]", []Link{{Raw: "note|display text", Line: 1}}},
		{"anchor", "[[note#heading]]", []Link{{Raw: "note#heading", Line: 1}}},
		{"anchor+display", "[[note#heading|shown]]", []Link{{Raw: "note#heading|shown", Line: 1}}},
		{"path-qualified", "[[folder/note]]", []Link{{Raw: "folder/note", Line: 1}}},
		{"two on a line", "[[a]] and [[b]]", []Link{{Raw: "a", Line: 1}, {Raw: "b", Line: 1}}},
		{"across lines", "[[a]]\n\n[[b]]", []Link{{Raw: "a", Line: 1}, {Raw: "b", Line: 3}}},
		{"fenced skipped", "```\n[[inside]]\n```\n[[outside]]", []Link{{Raw: "outside", Line: 4}}},
		{"inline-code skipped", "text `[[inline]]` and [[real]]", []Link{{Raw: "real", Line: 1}}},
		{"embed with anchor", "![[note#section]]", []Link{{Raw: "note#section", IsEmbed: true, Line: 1}}},
		{"block id", "[[note#^block-id]]", []Link{{Raw: "note#^block-id", Line: 1}}},
		{"empty brackets ignored", "[[]] and [[x]]", []Link{{Raw: "x", Line: 1}}},
		{"unclosed ignored", "[[open and text", nil},
		{"none", "just prose, [not](a/wikilink.md)", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Parse(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("Parse(%q) = %+v, want %+v", c.in, got, c.want)
			}
		})
	}
}

func TestSplit(t *testing.T) {
	cases := []struct {
		raw, target, anchor, display string
	}{
		{"note", "note", "", ""},
		{"folder/note", "folder/note", "", ""},
		{"note#heading", "note", "heading", ""},
		{"note|display text", "note", "", "display text"},
		{"note#heading|shown", "note", "heading", "shown"},
		{`inbox/index\|inbox`, "inbox/index", "", "inbox"}, // escaped pipe (table cell)
		{"  spaced  ", "spaced", "", ""},
		{"", "", "", ""},
	}
	for _, c := range cases {
		target, anchor, display := Link{Raw: c.raw}.Split()
		if target != c.target || anchor != c.anchor || display != c.display {
			t.Errorf("Split(%q) = (%q,%q,%q), want (%q,%q,%q)", c.raw, target, anchor, display, c.target, c.anchor, c.display)
		}
	}
}

func TestFull(t *testing.T) {
	if got := (Link{Raw: "note"}).Full(); got != "[[note]]" {
		t.Errorf("Full = %q", got)
	}
	if got := (Link{Raw: "img.png", IsEmbed: true}).Full(); got != "![[img.png]]" {
		t.Errorf("embed Full = %q", got)
	}
}

func TestResolve(t *testing.T) {
	paths := []string{
		"/index.md",
		"/note-a.md",
		"/note-b.md",
		"/sub/note-a.md", // same basename as /note-a.md, deeper
		"/sub/child.md",
		"/sub/deep/other.md",
		"/sub/dup.md",  // equal-depth pair with /team/dup.md, for the same-folder tiebreak
		"/team/dup.md",
	}
	none := map[string]string{}
	cases := []struct {
		name, target, from, want string
		aliases                  map[string]string
	}{
		{name: "basename match", target: "note-b", want: "/note-b.md"},
		{name: "with .md", target: "note-b.md", want: "/note-b.md"},
		{name: "path-qualified exact", target: "sub/child", want: "/sub/child.md"},
		{name: "path-qualified miss", target: "sub/missing", want: ""},
		{name: "no match", target: "nope", want: ""},
		{name: "empty", target: "", want: ""},
		// ambiguous basename: fewest segments wins (Obsidian shortest-path), even
		// from a deep source, so depth beats same-folder
		{name: "ambiguous prefers shallowest", target: "note-a", from: "/sub/child.md", want: "/note-a.md"},
		// among equal-depth candidates, the source's own folder wins the tie
		{name: "equal-depth prefers same folder", target: "dup", from: "/sub/child.md", want: "/sub/dup.md"},
		{name: "equal-depth, other source, alphabetical", target: "dup", from: "/x.md", want: "/sub/dup.md"},
		// an alias resolves a name no file carries
		{name: "alias", target: "Nickname", aliases: map[string]string{"Nickname": "/note-b.md"}, want: "/note-b.md"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			al := c.aliases
			if al == nil {
				al = none
			}
			if got := Resolve(c.target, c.from, paths, al); got != c.want {
				t.Errorf("Resolve(%q, from=%q) = %q, want %q", c.target, c.from, got, c.want)
			}
		})
	}
}

func TestAliasMap(t *testing.T) {
	m := AliasMap(map[string][]string{
		"/people/dana.md": {"Dana", "D. Smith", ""},
		"/products/x.md":  {"Product X"},
	})
	if m["Dana"] != "/people/dana.md" || m["D. Smith"] != "/people/dana.md" || m["Product X"] != "/products/x.md" {
		t.Errorf("AliasMap = %+v", m)
	}
	if _, ok := m[""]; ok {
		t.Error("empty alias should be dropped")
	}
}
