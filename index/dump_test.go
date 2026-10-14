package index

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/bundle"
)

func orderOf(idx *Index) []string {
	var out []string
	for _, e := range idx.ReadingOrder() {
		out = append(out, e.Path)
	}
	return out
}

func TestReadingOrderFollowsIndexes(t *testing.T) {
	idx := build(t, map[string]string{
		// The root index lists zeta before alpha, and the guides folder (by its
		// index) before both; a grandchild link and an external one order nothing.
		"index.md":        "# Home\n- [Guides](./guides/index.md)\n- [Zeta](./zeta.md)\n- [deep](./guides/deep/x.md)\n- [web](https://e.com)\n- [Alpha](./alpha.md)\n- [Zeta again](./zeta.md)\n",
		"zeta.md":         "z\n",
		"alpha.md":        "a\n",
		"unlisted.md":     "nothing links here\n",
		"aaa-first.md":    "path order would put this first\n",
		"guides/index.md": "- [Two](two.md)\n- [Back home](../index.md)\n- [Sibling](../alpha.md)\n- [Self](./index.md)\n",
		"guides/one.md":   "1\n",
		"guides/two.md":   "2\n",
		// No index here: path order.
		"guides/deep/x.md": "x\n",
		"guides/deep/b.md": "b\n",
		// A folder the root index never mentions comes with the unlisted, by path.
		"notes/n.md": "n\n",
	})
	want := []string{
		"/index.md",
		"/guides/index.md", "/guides/two.md", "/guides/deep/b.md", "/guides/deep/x.md", "/guides/one.md",
		"/zeta.md",
		"/alpha.md",
		"/aaa-first.md", "/notes/n.md", "/unlisted.md",
	}
	if got := orderOf(idx); !reflect.DeepEqual(got, want) {
		t.Errorf("order:\n got %v\nwant %v", got, want)
	}
}

func TestReadingOrderEveryEntryOnce(t *testing.T) {
	// Indexes that link each other in a cycle, link a folder by its path, and
	// link entries repeatedly must still yield each entry exactly once.
	idx := build(t, map[string]string{
		"index.md":     "[b](./b/) [a](./a/index.md) [a again](./a/index.md)\n",
		"a/index.md":   "[b](../b/index.md) [home](../index.md)\n",
		"a/x.md":       "x\n",
		"b/index.md":   "[a](../a/index.md)\n",
		"b/c/index.md": "[up](../index.md)\n",
		"b/c/y.md":     "y\n",
	})
	want := []string{"/index.md", "/b/index.md", "/b/c/index.md", "/b/c/y.md", "/a/index.md", "/a/x.md"}
	if got := orderOf(idx); !reflect.DeepEqual(got, want) {
		t.Errorf("order:\n got %v\nwant %v", got, want)
	}
	if len(idx.ReadingOrder()) != len(idx.Entries) {
		t.Errorf("ReadingOrder has %d entries, index has %d", len(idx.ReadingOrder()), len(idx.Entries))
	}
}

func TestReadingOrderWithoutIndexIsPathOrder(t *testing.T) {
	// Byte-wise: uppercase sorts before lowercase ("C" before "a", where a
	// case-insensitive sort would put it last), and "b" (a folder) before "b.md".
	// No two names differ only by case: on a case-insensitive filesystem (macOS,
	// Windows) they would be one file.
	idx := build(t, map[string]string{
		"b.md":   "",
		"a.md":   "",
		"C.md":   "",
		"b/z.md": "",
	})
	want := []string{"/C.md", "/a.md", "/b/z.md", "/b.md"}
	if got := orderOf(idx); !reflect.DeepEqual(got, want) {
		t.Errorf("order: got %v want %v", got, want)
	}
}

func TestReadingOrderUsesLineOrderAcrossLinkForms(t *testing.T) {
	// The index stores absolute links before relative ones and wikilinks last;
	// ordering must follow the lines they were written on.
	idx := build(t, map[string]string{
		"index.md": "[[c]]\n[b](./b.md)\n[a](/a.md)\n",
		"a.md":     "", "b.md": "", "c.md": "",
	})
	want := []string{"/index.md", "/c.md", "/b.md", "/a.md"}
	if got := orderOf(idx); !reflect.DeepEqual(got, want) {
		t.Errorf("order: got %v want %v", got, want)
	}
}

func dump(t *testing.T, idx *Index, entries []*Entry, withFrontmatter bool) string {
	t.Helper()
	var buf bytes.Buffer
	if err := idx.Dump(&buf, entries, withFrontmatter); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestDumpTransformsLinks(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "# Home\n",
		"docs/guide.md": "---\ntype: note\n---\n# Guide\n" +
			"[sib](./setup.md#install) [up](../index.md) [bare](setup.md) [abs](/docs/setup.md)\n" +
			"[todo](./missing.md) [web](https://e.com/a.md) [self](#guide) [out](../../PRD.md)\n" +
			"[[setup]] and [[setup#Install|install it]] and [[nowhere]] and ![[setup]]\n" +
			"`[code](./setup.md)` ![img](./pic.png \"cap\")\n" +
			"```\n[fenced](./setup.md)\n```\n",
		"docs/setup.md": "---\ntype: note\n---\n# Setup\n## Install\n",
	})
	e, _ := idx.Resolve("/docs/guide.md")
	got := dump(t, idx, []*Entry{e}, false)
	want := "<!-- /docs/guide.md -->\n# Guide\n" +
		"[sib](/docs/setup.md#install) [up](/index.md) [bare](/docs/setup.md) [abs](/docs/setup.md)\n" +
		"[todo](/docs/missing.md) [web](https://e.com/a.md) [self](#guide) [out](../../PRD.md)\n" +
		"[setup](/docs/setup.md) and [install it](/docs/setup.md#Install) and [[nowhere]] and [setup](/docs/setup.md)\n" +
		"`[code](./setup.md)` ![img](/docs/pic.png \"cap\")\n" +
		"```\n[fenced](./setup.md)\n```\n"
	if got != want {
		t.Errorf("dump:\n got %q\nwant %q", got, want)
	}
}

func TestDumpFrontmatterAndLayout(t *testing.T) {
	idx := build(t, map[string]string{
		"a.md":     "---\ntype: note\ntags: [x]\n---\n\n\n# A\n[b](./b.md)\n\n\n",
		"b.md":     "no trailing newline",
		"empty.md": "",
		"fm.md":    "---\ntype: note\n---\n",
	})
	entries := idx.ReadingOrder()

	want := "<!-- /a.md -->\n# A\n[b](/b.md)\n\n" +
		"<!-- /b.md -->\nno trailing newline\n\n" +
		"<!-- /empty.md -->\n\n" +
		"<!-- /fm.md -->\n"
	if got := dump(t, idx, entries, false); got != want {
		t.Errorf("without frontmatter:\n got %q\nwant %q", got, want)
	}

	// Frontmatter verbatim between its fences; its own content is not rewritten.
	want = "<!-- /a.md -->\n---\ntype: note\ntags: [x]\n---\n\n\n# A\n[b](/b.md)\n\n" +
		"<!-- /b.md -->\nno trailing newline\n\n" +
		"<!-- /empty.md -->\n\n" +
		"<!-- /fm.md -->\n---\ntype: note\n---\n"
	if got := dump(t, idx, entries, true); got != want {
		t.Errorf("with frontmatter:\n got %q\nwant %q", got, want)
	}

	if got := dump(t, idx, nil, false); got != "" {
		t.Errorf("no entries should write nothing, got %q", got)
	}
}

func TestDumpNormalizesLineEndingsAndBOM(t *testing.T) {
	idx := build(t, map[string]string{
		"crlf.md": "\uFEFF---\r\ntype: note\r\n---\r\n# C\r\n[[lf]] [l](./lf.md)\r\n",
		"lf.md":   "---\ntype: note\n---\n# L\n",
	})
	got := dump(t, idx, idx.ReadingOrder(), false)
	want := "<!-- /crlf.md -->\n# C\n[lf](/lf.md) [l](/lf.md)\n\n<!-- /lf.md -->\n# L\n"
	if got != want {
		t.Errorf("dump:\n got %q\nwant %q", got, want)
	}
}

func TestDumpIsReadOnlyAndDeterministic(t *testing.T) {
	files := map[string]string{
		"index.md": "[a](./a.md) [[b]]\r\n",
		"a.md":     "---\ntype: note\n---\n[b](b.md)\n",
		"b.md":     "\uFEFF# B\n",
		"sub/c.md": "[up](../a.md)\n",
	}
	idx := build(t, files)
	first := dump(t, idx, idx.ReadingOrder(), true)
	if second := dump(t, idx, idx.ReadingOrder(), true); second != first {
		t.Errorf("two dumps differ:\n%q\n%q", first, second)
	}
	for rel, content := range files {
		got, err := os.ReadFile(filepath.Join(idx.Bundle.Dir, rel))
		if err != nil || string(got) != content {
			t.Errorf("%s changed on disk: %q (err %v)", rel, got, err)
		}
	}
}

func TestOutOfBundle(t *testing.T) {
	root := t.TempDir()
	bundleDir := filepath.Join(root, "kb")
	write := func(rel, content string) {
		p := filepath.Join(bundleDir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("wiki.toml", "spec=\"0.1\"\nignore=[\"../NOTES.md\"]\n")
	write("a.md", "---\ntype: note\n---\n[prd](../PRD.md)\n[notes](../NOTES.md)\n[in](./b.md)\n")
	write("b.md", "---\ntype: note\n---\n")
	b, err := bundle.Discover(bundleDir)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Build(b)
	if err != nil {
		t.Fatal(err)
	}

	got := idx.OutOfBundle("/a.md")
	want := []LinkRef{{From: "/a.md", To: "../PRD.md", Text: "prd", Line: 4}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("OutOfBundle = %+v, want %+v (../NOTES.md is acknowledged in ignore)", got, want)
	}
	if idx.OutOfBundle("/b.md") != nil || idx.OutOfBundle("/nope.md") != nil {
		t.Error("an entry without out-of-bundle links, or an unknown path, should yield nil")
	}
	// check reports the same set, so the two can never disagree.
	var warned []string
	for _, is := range idx.Check() {
		if strings.HasPrefix(is.Msg, "out-of-bundle link") {
			warned = append(warned, is.Msg)
		}
	}
	if !reflect.DeepEqual(warned, []string{"out-of-bundle link -> ../PRD.md"}) {
		t.Errorf("check warned %v", warned)
	}
}
