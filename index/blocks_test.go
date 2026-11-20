package index

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/bundle"
	"github.com/agentic-wiki/wiki/internal/scaffold"
	"github.com/agentic-wiki/wiki/parse"
)

const page = "---\ntype: note\nstatus: todo\n---\n\n# Plan\n\nIntro.\n\n- [ ] one\n- [ ] two\n\nOutro.\n"

func entryOf(t *testing.T, idx *Index, p string) *Entry {
	t.Helper()
	e, err := idx.Resolve(p)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func fileOf(t *testing.T, idx *Index, p string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(idx.Bundle.Dir, filepath.FromSlash(strings.TrimPrefix(p, "/"))))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// state is what an app holds after reading an entry: its version and blocks.
func state(t *testing.T, e *Entry) (string, []parse.Block) {
	t.Helper()
	v, err := e.Version()
	if err != nil {
		t.Fatal(err)
	}
	bs, err := e.Blocks()
	if err != nil {
		t.Fatal(err)
	}
	return v, bs
}

func hashesOf(bs []parse.Block) []string { return blockHashes(bs) }

func TestFileVersionIs128Bits(t *testing.T) {
	if v := FileVersion([]byte(page)); len(v) != 32 || v == FileVersion([]byte(page+" ")) {
		t.Errorf("version %q", v)
	}
}

// A write replaces only the changed block: the header and every other byte
// stay, the version moves, and the entry reads the result back.
func TestApplyBlocksReplacesOnlyTheChangedBlock(t *testing.T) {
	idx := build(t, map[string]string{"plan.md": page})
	e := entryOf(t, idx, "/plan.md")
	v, bs := state(t, e)
	tick := parse.Op{Op: "replace", At: 2, Hash: bs[2].Hash, Text: "- [x] one"}
	got, err := e.ApplyBlocks(v, hashesOf(bs), []parse.Op{tick})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(page, "- [ ] one", "- [x] one", 1)
	if fileOf(t, idx, "/plan.md") != want || got.Version != FileVersion([]byte(want)) || len(got.Conflicts) != 0 {
		t.Errorf("file %q, applied %+v", fileOf(t, idx, "/plan.md"), got)
	}
	if len(got.Ops) != 1 || got.Ops[0].Op != "replace" || got.Ops[0].At != 2 {
		t.Errorf("ops from base: %+v", got.Ops)
	}
	if len(e.Checkboxes) != 2 || !e.Checkboxes[0].Done {
		t.Errorf("the entry wasn't read back: %+v", e.Checkboxes)
	}
}

// A stale write merges with what the disk changed meanwhile, and a block both
// changed comes back as a conflict, the disk's text kept.
func TestApplyBlocksOnAStaleBase(t *testing.T) {
	idx := build(t, map[string]string{"plan.md": page})
	e := entryOf(t, idx, "/plan.md")
	v, bs := state(t, e)
	// An agent edits the page after the app read it.
	agent := strings.Replace(strings.Replace(page, "Intro.", "Intro, by an agent.", 1), "- [ ] two", "- [ ] two, by an agent", 1)
	os.WriteFile(filepath.Join(idx.Bundle.Dir, "plan.md"), []byte(agent), 0o644)

	ops := []parse.Op{
		{Op: "replace", At: 2, Hash: bs[2].Hash, Text: "- [x] one"},
		{Op: "replace", At: 3, Hash: bs[3].Hash, Text: "- [x] two"},
	}
	got, err := e.ApplyBlocks(v, hashesOf(bs), ops)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(agent, "- [ ] one", "- [x] one", 1)
	if fileOf(t, idx, "/plan.md") != want {
		t.Errorf("file:\n%s\nwant:\n%s", fileOf(t, idx, "/plan.md"), want)
	}
	if len(got.Conflicts) != 1 || got.Conflicts[0].At != 3 || got.Conflicts[0].Case != "both-changed" || got.Conflicts[0].Theirs[0].Text != "- [ ] two, by an agent" {
		t.Errorf("conflicts %+v", got.Conflicts)
	}
}

// A write that changes nothing writes nothing: the file isn't touched.
func TestApplyBlocksWithNoChangeWritesNothing(t *testing.T) {
	idx := build(t, map[string]string{"plan.md": page})
	e := entryOf(t, idx, "/plan.md")
	path := filepath.Join(idx.Bundle.Dir, "plan.md")
	before, _ := os.Stat(path)
	v, bs := state(t, e)
	got, err := e.ApplyBlocks(v, hashesOf(bs), []parse.Op{{Op: "replace", At: 1, Hash: bs[1].Hash, Text: "Intro."}})
	if err != nil || got.Version != v {
		t.Fatalf("applied %+v err %v", got, err)
	}
	if after, _ := os.Stat(path); !after.ModTime().Equal(before.ModTime()) {
		t.Error("a write with no change touched the file")
	}
}

func TestApplyBlocksRefusals(t *testing.T) {
	idx := build(t, map[string]string{"plan.md": page, "bare.md": "just text\n"})
	e := entryOf(t, idx, "/plan.md")
	v, bs := state(t, e)
	stale := slices.Clone(hashesOf(bs))
	stale[0] = parse.BlockHash("something else")
	if _, err := e.ApplyBlocks(v, stale, nil); !errors.Is(err, parse.ErrRefused) {
		t.Errorf("baseBlocks that aren't the page's at its version: %v", err)
	}
	if _, err := e.ApplyBlocks(v, hashesOf(bs), []parse.Op{{Op: "delete", At: 9, Hash: "x"}}); !errors.Is(err, parse.ErrRefused) {
		t.Errorf("an operation out of range: %v", err)
	}
	// A page with no header never gets one through its body.
	bare := entryOf(t, idx, "/bare.md")
	bv, bbs := state(t, bare)
	for _, text := range []string{"---\ntype: note\n---", "\ufeff---\ntype: note\n---"} {
		fence := parse.Op{Op: "insert", After: -1, Text: text}
		if _, err := bare.ApplyBlocks(bv, hashesOf(bbs), []parse.Op{fence}); !errors.Is(err, parse.ErrRefused) {
			t.Errorf("a frontmatter fence (%q) at the start of a page with no header: %v", text[:4], err)
		}
	}
	// Nor does a save make the page bigger than a merge reads.
	huge := parse.Op{Op: "insert", After: 0, Hash: bs[0].Hash, Text: strings.Repeat("- x\n", parse.MaxBlocks)}
	if _, err := e.ApplyBlocks(v, hashesOf(bs), []parse.Op{huge}); !errors.Is(err, parse.ErrRefused) {
		t.Errorf("a save making the page over %d blocks: %v", parse.MaxBlocks, err)
	}
	if fileOf(t, idx, "/plan.md") != page || fileOf(t, idx, "/bare.md") != "just text\n" {
		t.Error("a refused write changed a file")
	}
}

// A reserved file's body is a body like any other.
func TestApplyBlocksOnAReservedFile(t *testing.T) {
	idx := build(t, map[string]string{"notes/index.md": "# Notes\n\n- [a](a.md)\n"})
	e := entryOf(t, idx, "/notes/index.md")
	v, bs := state(t, e)
	if _, err := e.ApplyBlocks(v, hashesOf(bs), []parse.Op{{Op: "insert", After: 1, Hash: bs[1].Hash, Text: "- [b](b.md)"}}); err != nil {
		t.Fatal(err)
	}
	if got := fileOf(t, idx, "/notes/index.md"); got != "# Notes\n\n- [a](a.md)\n- [b](b.md)\n" {
		t.Errorf("file %q", got)
	}
}

// The new writes follow v0.20.0's rule: an entry whose file has become a link
// out of the bundle is refused, and the file outside stays as it was.
func TestTheNewWritesStayInTheBundle(t *testing.T) {
	idx := build(t, map[string]string{"plan.md": page})
	e := entryOf(t, idx, "/plan.md")
	v, bs := state(t, e)
	outside := filepath.Join(t.TempDir(), "secret.md")
	os.WriteFile(outside, []byte(page), 0o644)
	link := filepath.Join(idx.Bundle.Dir, "plan.md")
	os.Remove(link)
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := e.ApplyBlocks(v, hashesOf(bs), []parse.Op{{Op: "replace", At: 1, Hash: bs[1].Hash, Text: "x"}}); !errors.Is(err, ErrOutsideBundle) {
		t.Errorf("ApplyBlocks: %v, want ErrOutsideBundle", err)
	}
	if _, err := e.SetFieldsIf(map[string]any{"status": "todo"}, map[string]any{"status": "done"}, nil); !errors.Is(err, ErrOutsideBundle) {
		t.Errorf("SetFieldsIf: %v, want ErrOutsideBundle", err)
	}
	if got, _ := os.ReadFile(outside); string(got) != page {
		t.Errorf("the file outside the bundle changed:\n%s", got)
	}
}

// Every page of `wiki init --full`, through ApplyBlocks: no operation leaves
// it byte-identical, and changing its last block changes nothing else.
func TestApplyBlocksRoundTripOverABundle(t *testing.T) {
	dir := t.TempDir()
	if _, err := scaffold.Write(dir, scaffold.Full(), false); err != nil {
		t.Fatal(err)
	}
	b, err := bundle.Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Build(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range idx.Entries {
		before, _ := e.Raw()
		v, bs := state(t, e)
		if _, err := e.ApplyBlocks(v, hashesOf(bs), nil); err != nil {
			t.Fatalf("%s: %v", e.Path, err)
		}
		if after, _ := e.Raw(); after != before {
			t.Fatalf("%s changed with no operation", e.Path)
		}
		if len(bs) == 0 {
			continue
		}
		last := bs[len(bs)-1]
		if _, err := e.ApplyBlocks(v, hashesOf(bs), []parse.Op{{Op: "replace", At: len(bs) - 1, Hash: last.Hash, Text: last.Text + " edited"}}); err != nil {
			t.Fatalf("%s: %v", e.Path, err)
		}
		body := parse.Body(before)
		head := before[:len(before)-len(body)]
		want := head + body[:last.End] + " edited" + body[last.End:]
		if after, _ := e.Raw(); after != want {
			t.Fatalf("%s:\n%q\nwant:\n%q", e.Path, after, want)
		}
	}
}

// Each key is written only if it still has the value the caller read; a key
// changed on disk comes back with its value there, and the others are written.
func TestSetFieldsIf(t *testing.T) {
	idx := build(t, map[string]string{"plan.md": "---\ntype: note\nstatus: todo\nowner: ana\ntags: [a, b]\n---\nbody\n"})
	e := entryOf(t, idx, "/plan.md")
	// The disk moved owner on since the app read it.
	path := filepath.Join(idx.Bundle.Dir, "plan.md")
	os.WriteFile(path, []byte("---\ntype: note\nstatus: todo\nowner: bo\ntags: [a, b]\n---\nbody\n"), 0o644)

	got, err := e.SetFieldsIf(
		map[string]any{"status": "todo", "owner": "ana", "tags": []string{"a", "b"}, "due": nil},
		map[string]any{"status": "done", "owner": "cy", "due": "2026-12-01"},
		[]string{"tags"},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "---\ntype: note\nstatus: done\nowner: bo\ndue: 2026-12-01\n---\nbody\n"
	if raw, _ := os.ReadFile(path); string(raw) != want {
		t.Errorf("file:\n%s\nwant:\n%s", raw, want)
	}
	if len(got.Fields) != 1 || got.Fields[0].Key != "owner" || got.Fields[0].Theirs != "bo" || got.Version != FileVersion([]byte(want)) {
		t.Errorf("applied %+v", got)
	}
	if e.Field("status") != "done" {
		t.Errorf("the entry wasn't read back: status %q", e.Field("status"))
	}
	for name, call := range map[string]func() error{
		"a number":       func() error { _, err := e.SetFieldsIf(nil, map[string]any{"n": 3}, nil); return err },
		"set and unset":  func() error { _, err := e.SetFieldsIf(nil, map[string]any{"a": "x"}, []string{"a"}); return err },
		"nothing to set": func() error { _, err := e.SetFieldsIf(nil, nil, nil); return err },
	} {
		if call() == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestCreate(t *testing.T) {
	idx := build(t, map[string]string{"wiki.toml": "spec = \"0.1\"\nignore = [\"drafts/**\"]\n", "a.md": "---\ntype: note\n---\n"})
	content := []byte("---\ntype: note\n---\n# New\n")
	e, err := idx.Create("/notes/deep/new.md", content)
	if err != nil {
		t.Fatal(err)
	}
	if e.Path != "/notes/deep/new.md" || e.Type != "note" || fileOf(t, idx, "/notes/deep/new.md") != string(content) {
		t.Errorf("entry %+v, file %q", e, fileOf(t, idx, "/notes/deep/new.md"))
	}
	if _, err := idx.Create("/a.md", content); !errors.Is(err, ErrExists) {
		t.Errorf("over a file: %v, want ErrExists", err)
	}
	if fileOf(t, idx, "/a.md") != "---\ntype: note\n---\n" {
		t.Error("Create overwrote a file")
	}
	for _, p := range []string{"/../x.md", "/notes/../x.md", "/a//x.md", "/.git/x.md", "/notes/.x.md", "/x.txt", "/.md", "/drafts/x.md",
		`/a\b.md`, "/a:b.md", "/notes/x.md:stream.md", "/a\x01b.md", "/a\nb.md"} {
		if _, err := idx.Create(p, content); !errors.Is(err, ErrBadPath) {
			t.Errorf("%s: %v, want ErrBadPath", p, err)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(idx.Bundle.Dir, "ext")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := idx.Create("/ext/deep/x.md", content); !errors.Is(err, ErrOutsideBundle) {
		t.Errorf("through a folder linked out: %v, want ErrOutsideBundle", err)
	}
	filepath.WalkDir(outside, func(p string, d fs.DirEntry, err error) error {
		if p != outside {
			t.Errorf("Create made %s outside the bundle", p)
		}
		return nil
	})
}

// CheckNewEntry says whether Create would take a path, before anything is
// written: the same rules, ignore and where the file would land included,
// and whether a file is already there.
func TestCheckNewEntry(t *testing.T) {
	idx := build(t, map[string]string{"wiki.toml": "spec = \"0.1\"\nignore = [\"drafts/**\"]\n", "a.md": "---\ntype: note\n---\n"})
	if err := idx.CheckNewEntry("/notes/deep/new.md"); err != nil {
		t.Errorf("a new path: %v", err)
	}
	if err := idx.CheckNewEntry("/a.md"); !errors.Is(err, ErrExists) {
		t.Errorf("a taken path: %v, want ErrExists", err)
	}
	for _, p := range []string{"/../x.md", "/a//x.md", "/.git/x.md", "/notes/.x.md", "/x.txt", "/drafts/x.md", "/a:b.md"} {
		if err := idx.CheckNewEntry(p); !errors.Is(err, ErrBadPath) {
			t.Errorf("%s: %v, want ErrBadPath", p, err)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(idx.Bundle.Dir, "ext")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := idx.CheckNewEntry("/ext/x.md"); !errors.Is(err, ErrOutsideBundle) {
		t.Errorf("through a folder linked out: %v, want ErrOutsideBundle", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(idx.Bundle.Dir, "notes")); entries != nil {
		t.Error("the check made a folder")
	}
}

// WriteIf replaces a page only while it still has the version the caller
// read: all of the new bytes or nothing. It writes a page the index leaves
// out too, and keeps the file's permissions.
func TestWriteIf(t *testing.T) {
	idx := build(t, map[string]string{"wiki.toml": "spec = \"0.1\"\nignore = [\"drafts/**\"]\n", "a.md": "---\ntype: note\n---\nshown\n", "drafts/d.md": "draft\n"})
	shown := FileVersion([]byte("---\ntype: note\n---\nshown\n"))
	abs := filepath.Join(idx.Bundle.Dir, "a.md")
	os.Chmod(abs, 0o600)
	if err := idx.WriteIf("/a.md", shown, []byte("---\ntype: note\n---\nundone\n")); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(abs); fileOf(t, idx, "/a.md") != "---\ntype: note\n---\nundone\n" || fi.Mode().Perm() != 0o600 {
		t.Errorf("file %q, mode %v", fileOf(t, idx, "/a.md"), fi.Mode())
	}
	if err := idx.WriteIf("/a.md", shown, []byte("again\n")); !errors.Is(err, ErrChanged) {
		t.Errorf("at a stale version: %v, want ErrChanged", err)
	}
	if fileOf(t, idx, "/a.md") != "---\ntype: note\n---\nundone\n" {
		t.Error("written at a stale version")
	}
	if err := idx.WriteIf("/gone.md", shown, []byte("x\n")); !errors.Is(err, ErrChanged) {
		t.Errorf("a missing file: %v, want ErrChanged", err)
	}
	if err := idx.WriteIf("/drafts/d.md", FileVersion([]byte("draft\n")), []byte("draft 2\n")); err != nil || fileOf(t, idx, "/drafts/d.md") != "draft 2\n" {
		t.Errorf("a page ignore leaves out: %v", err)
	}
	for _, p := range []string{"/../x.md", "/a//x.md", "/.git/x.md", "/x.txt", "/a:b.md"} {
		if err := idx.WriteIf(p, shown, []byte("x\n")); !errors.Is(err, ErrBadPath) {
			t.Errorf("%s: %v, want ErrBadPath", p, err)
		}
	}
	if err := idx.WriteIf("/a.md", FileVersion([]byte("---\ntype: note\n---\nundone\n")), []byte(strings.Repeat("- x\n", parse.MaxBlocks+1))); !errors.Is(err, parse.ErrRefused) {
		t.Errorf("over the cap: %v, want ErrRefused", err)
	}
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "x.md"), []byte("outside\n"), 0o644)
	if err := os.Symlink(outside, filepath.Join(idx.Bundle.Dir, "ext")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := idx.WriteIf("/ext/x.md", FileVersion([]byte("outside\n")), []byte("x\n")); !errors.Is(err, ErrOutsideBundle) {
		t.Errorf("through a folder linked out: %v, want ErrOutsideBundle", err)
	}
}

// On a filesystem without hard links (exFAT, some FUSE and SMB mounts), Create
// still writes the entry and still never overwrites, and leaves no temp file.
func TestCreateWithoutHardLinks(t *testing.T) {
	for _, refusal := range []error{errors.ErrUnsupported, fs.ErrPermission} {
		saved := link
		link = func(oldname, newname string) error {
			return &os.LinkError{Op: "link", Old: oldname, New: newname, Err: refusal}
		}
		t.Cleanup(func() { link = saved })

		idx := build(t, map[string]string{"a.md": "---\ntype: note\n---\n"})
		content := []byte("---\ntype: note\n---\n# New\n")
		if _, err := idx.Create("/notes/new.md", content); err != nil {
			t.Fatalf("%v: %v", refusal, err)
		}
		if fileOf(t, idx, "/notes/new.md") != string(content) {
			t.Errorf("%v: file %q", refusal, fileOf(t, idx, "/notes/new.md"))
		}
		if _, err := idx.Create("/a.md", content); !errors.Is(err, ErrExists) {
			t.Errorf("%v: over a file: %v, want ErrExists", refusal, err)
		}
		if left, _ := filepath.Glob(filepath.Join(idx.Bundle.Dir, "notes", ".wiki-*.tmp")); len(left) > 0 {
			t.Errorf("%v: temp files left: %v", refusal, left)
		}
		link = saved
	}
}

// A page Create would write over the block cap is refused, nothing written.
func TestCreateRefusesAPageOverTheCap(t *testing.T) {
	idx := build(t, nil)
	if _, err := idx.Create("/big.md", []byte(strings.Repeat("- x\n", parse.MaxBlocks+1))); !errors.Is(err, parse.ErrRefused) {
		t.Errorf("err = %v, want ErrRefused", err)
	}
	if _, err := os.Stat(filepath.Join(idx.Bundle.Dir, "big.md")); !os.IsNotExist(err) {
		t.Errorf("the refused page was written: %v", err)
	}
}

// A list from JSON arrives as []any: SetFieldsIf takes it as a []string, in
// old and in set alike.
func TestSetFieldsIfTakesJSONLists(t *testing.T) {
	idx := build(t, map[string]string{"plan.md": "---\ntype: note\ntags: [a, b]\n---\nbody\n"})
	e := entryOf(t, idx, "/plan.md")
	got, err := e.SetFieldsIf(map[string]any{"tags": []any{"a", "b"}}, map[string]any{"tags": []any{"b", "c"}}, nil)
	if err != nil || len(got.Fields) != 0 {
		t.Fatalf("applied %+v err %v", got, err)
	}
	if !slices.Equal(e.FieldList("tags"), []string{"b", "c"}) {
		t.Errorf("tags %v", e.FieldList("tags"))
	}
	if _, err := e.SetFieldsIf(nil, map[string]any{"tags": []any{"a", 3}}, nil); err == nil {
		t.Error("a list holding a number: no error")
	}
}
