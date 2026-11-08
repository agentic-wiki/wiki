package index

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/bundle"
	"github.com/agentic-wiki/wiki/parse"
)

// rebuild reads the bundle again, as the next command would.
func rebuild(t *testing.T, idx *Index) *Index {
	t.Helper()
	b, err := bundle.Discover(idx.Bundle.Dir)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := Build(b)
	if err != nil {
		t.Fatal(err)
	}
	return fresh
}

func fileText(t *testing.T, idx *Index, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(idx.Bundle.Dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// The writers' round trip on real files: every link move, tidy or dump writes
// reads back to the file it names, with its suffix as written. The names are
// ones every OS allows; the spelling of the rest is TestSpellDestRoundTrip's.
func TestWritersRoundTrip(t *testing.T) {
	for _, name := range []string{"a b.md", "50%.md", "a#b.md", "(x).md", "café.md", "x%20y.md"} {
		want := "/d/" + name
		// move: a plain and a <…> source, with every suffix.
		idx := build(t, map[string]string{
			"index.md": "---\nokf_version: \"0.1\"\n---\n",
			"old.md":   "---\ntype: note\n---\n# Old\n",
			"e/ref.md": "---\ntype: note\n---\n[a](../old.md) [b](../old.md?q=1#x) [c](<../old.md#x> \"t\")\n",
		})
		if _, err := idx.Move("/old.md", want, false, false); err != nil {
			t.Fatalf("move to %q: %v", name, err)
		}
		idx = rebuild(t, idx)
		ref, _ := idx.Resolve("/e/ref.md")
		var suffixes []string
		for _, l := range ref.Links {
			if l.Target != want {
				t.Errorf("move to %q: %q reads as %q", name, l.Raw, l.Target)
			}
			suffixes = append(suffixes, suffixOf(l.Raw))
		}
		if !slices.Equal(suffixes, []string{"", "?q=1#x", "#x"}) || !strings.Contains(fileText(t, idx, "e/ref.md"), "[c](<") {
			t.Errorf("move to %q: suffixes %q, file %q", name, suffixes, fileText(t, idx, "e/ref.md"))
		}

		// tidy --links: a root-absolute link becomes relative, and reads back.
		os.WriteFile(filepath.Join(idx.Bundle.Dir, "e", "abs.md"), []byte("---\ntype: note\n---\n[t]("+spell(want, true)+"?q=1#x)\n"), 0o644)
		idx = rebuild(t, idx)
		if _, err := idx.NormalizeLinks(true); err != nil {
			t.Fatal(err)
		}
		idx = rebuild(t, idx)
		abs, _ := idx.Resolve("/e/abs.md")
		if len(abs.Links) != 1 || abs.Links[0].Target != want || !strings.HasPrefix(abs.Links[0].Raw, "../") || suffixOf(abs.Links[0].Raw) != "?q=1#x" {
			t.Errorf("tidy to %q: %+v", name, abs.Links)
		}

		// dump: root-absolute, and reads back from anywhere.
		out := dump(t, idx, []*Entry{ref}, false)
		set := parse.Links(out)
		for _, l := range set.Absolute {
			if d := ResolveDest("/x/y.md", l.Target); d.Path != want {
				t.Errorf("dump of %q: %q reads as %q", name, l.Target, d.Path)
			}
		}
		if len(set.Absolute) != 3 {
			t.Errorf("dump of %q: %q", name, out)
		}
	}
}

// A rewrite changes only the path it moves: a ?query, an anchor and a <…>
// form stay as written, and a # inside the file name is spelled %23.
func TestMoveKeepsTheSuffix(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"b.md":     "---\ntype: note\n---\n",
		"a#b.md":   "---\ntype: note\n---\n# H\n",
		"r.md":     "---\ntype: note\n---\n[a](./b.md?raw=1#x) [a](<./b.md?raw=1>) [x](./a%23b.md#h)\n",
	})
	if _, err := idx.Move("/b.md", "/e.md", false, false); err != nil {
		t.Fatal(err)
	}
	idx = rebuild(t, idx)
	if _, err := idx.Move("/a#b.md", "/f.md", false, false); err != nil {
		t.Fatal(err)
	}
	if got, want := fileText(t, idx, "r.md"), "---\ntype: note\n---\n[a](./e.md?raw=1#x) [a](<./e.md?raw=1>) [x](./f.md#h)\n"; got != want {
		t.Errorf("r.md = %q, want %q", got, want)
	}
}

// tidy --links spells a # in a file name %23, and never cuts the path at it.
func TestTidyLinksSpellsTheHash(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"a#b.md":   "---\ntype: note\n---\n",
		"d/c.md":   "---\ntype: note\n---\n[x](/a%23b.md) [y](</my%20dir/n.md#s>)\n",
	})
	fixes, err := idx.NormalizeLinks(true)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fileText(t, idx, "d/c.md"), "---\ntype: note\n---\n[x](../a%23b.md) [y](<../my%20dir/n.md#s>)\n"; got != want {
		t.Errorf("d/c.md = %q, want %q", got, want)
	}
	if len(fixes) != 2 || fixes[0].From != "/a%23b.md" || fixes[1].From != "/my%20dir/n.md#s" {
		t.Errorf("fixes %+v, want both, in document order", fixes)
	}
}

// dump rewrites only a link that resolves: one to nothing keeps its spelling,
// %0A and all, and an external one is never touched.
func TestDumpKeepsLinksToNothing(t *testing.T) {
	body := "[ref](notes%0A%0A%23%20System…%0A.md) [far](//evil.example/x.md) [w](my%20b%5C.md)\n"
	idx := build(t, map[string]string{"a.md": "---\ntype: note\n---\n" + body})
	e, _ := idx.Resolve("/a.md")
	if got := dump(t, idx, []*Entry{e}, false); got != "<!-- /a.md -->\n"+body {
		t.Errorf("dump = %q", got)
	}
}

// An outside link's Target is its climbing path, and `ignore` names it the
// same way: ../PRD.md is silenced however it is written, ../Q.md isn't.
func TestIgnoreSilencesAnOutsideLink(t *testing.T) {
	idx := build(t, map[string]string{
		"wiki.toml": "spec = \"0.1\"\nignore = [\"../PRD.md\"]\n",
		"a.md":      "---\ntype: note\n---\n[p](../PRD.md) [p](/../PRD.md) [q](../Q.md)\n",
	})
	var to []string
	for _, r := range idx.OutOfBundle("/a.md") {
		to = append(to, r.To)
	}
	if !slices.Equal(to, []string{"../Q.md"}) {
		t.Errorf("out of bundle %q, want only ../Q.md", to)
	}
}

// check reads a destination by the one rule: a percent-encoded name, a
// ?query and an encoded anchor find what they name, and an external
// destination is never checked.
func TestCheckReadsDestinationsByTheOneRule(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md":   "---\nokf_version: \"0.1\"\n---\n[a](./a.md)\n",
		"a.md":       "---\ntype: note\n---\n[f](my%20file.md) [r](./b.md?raw=1) [c](./b.md#caf%C3%A9) [e](//evil.example/x.md)\n",
		"b.md":       "---\ntype: note\n---\n## Café\n",
		"my file.md": "---\ntype: note\n---\n",
	})
	for _, is := range idx.Check() {
		if is.Entry == "/a.md" {
			t.Errorf("check: %+v", is)
		}
	}
}

// FrontmatterRefs reads every value that names an entry, in file order then
// item order, by the one rule: the anchor cut, outside ones kept, external
// values and empty items not references.
func TestFrontmatterRefs(t *testing.T) {
	e := Read("/d/x.md", "---\ntype: note\nrelated: [./a.md, \"\", /b.md#s]\nparent: ../c.md\nsite: https://e.com/y.md\nfar: ../../z.md\ntitle: Not a ref\n---\n")
	var got []string
	for _, r := range e.FrontmatterRefs() {
		got = append(got, r.Key+"="+r.Value+"->"+r.Path+"#"+r.Anchor)
		if r.Outside != (r.Key == "far") {
			t.Errorf("%s: outside %v", r.Key, r.Outside)
		}
	}
	want := []string{"related=./a.md->/d/a.md#", "related=/b.md#s->/b.md#s", "parent=../c.md->/c.md#", "far=../../z.md->../z.md#"}
	if !slices.Equal(got, want) {
		t.Errorf("refs %q, want %q", got, want)
	}
}

// Title is `title:`, else the first level-1 heading with text, else "".
func TestTitle(t *testing.T) {
	for content, want := range map[string]string{
		"---\ntitle: T\n---\n# H\n":           "T",
		"---\ntype: note\n---\n## Two\n# H\n": "H",
		"---\ntype: note\n---\n#\n# H\n":      "H",
		"```\n# not a heading\n```\n":         "",
		"no headings\n":                       "",
	} {
		if got := Read("/x.md", content).Title(); got != want {
			t.Errorf("Title of %q = %q, want %q", content, got, want)
		}
	}
}
