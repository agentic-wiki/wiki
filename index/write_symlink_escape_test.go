package index

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentic-wiki/wiki/bundle"
)

// A cloned repo can ship an entry that is a symlink to a file outside the
// bundle (~/.bashrc, say). Build leaves such a link out, and a write never
// reaches the file outside either: writeFile refuses a file whose real path
// leaves the bundle, even one swapped for a link after the index was built.
// With body writes, whoever edits the wiki would otherwise write chosen
// content to any file the user can write.
func TestWriteRefusesAnEntryOutsideTheBundle(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	target := filepath.Join(outside, "bashrc")
	orig := "---\ntype: note\n---\n# Home\n"
	if err := os.WriteFile(target, []byte(orig), 0o644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "wiki.toml"), []byte("spec = \"0.1\"\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "notes"), 0o755)
	entry := filepath.Join(root, "notes", "setup.md")
	os.WriteFile(entry, []byte(orig), 0o644)
	b, err := bundle.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Build(b)
	if err != nil {
		t.Fatal(err)
	}
	e, err := idx.Resolve("/notes/setup.md")
	if err != nil {
		t.Fatal(err)
	}
	// The entry becomes a link to the outside file after the build.
	os.Remove(entry)
	if err := os.Symlink(target, entry); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := e.SetField("owner", "x"); err == nil {
		t.Error("a write to an entry that resolves outside the bundle succeeded, want a refusal")
	}
	if got, _ := os.ReadFile(target); string(got) != orig {
		t.Errorf("the file outside the bundle was rewritten:\n%s", got)
	}
}

// A bundle whose path runs through a symlink (macOS's /var is one, to
// /private/var) is still inside itself: the bundle's root and the entry's file
// are both compared resolved, so its entries are written as usual.
func TestWriteUnderASymlinkedParent(t *testing.T) {
	real, links := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(real, "kb"), 0o755)
	os.WriteFile(filepath.Join(real, "kb", "wiki.toml"), []byte("spec = \"0.1\"\n"), 0o644)
	os.WriteFile(filepath.Join(real, "kb", "a.md"), []byte("---\ntype: note\n---\n"), 0o644)
	if err := os.Symlink(real, filepath.Join(links, "home")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	b, err := bundle.Discover(filepath.Join(links, "home", "kb"))
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Build(b)
	if err != nil {
		t.Fatal(err)
	}
	e, err := idx.Resolve("/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SetField("owner", "x"); err != nil {
		t.Fatalf("a write inside a bundle under a symlinked folder: %v", err)
	}
}

// A link to a file outside the bundle is never an entry: not listed, not read,
// not searched, so the wiki never shows that file. check names it. A link
// that stays inside the bundle is an entry like any other.
func TestBuildLeavesOutALinkOutsideTheBundle(t *testing.T) {
	top := t.TempDir()
	root := filepath.Join(top, "kb")
	os.MkdirAll(filepath.Join(root, "notes"), 0o755)
	os.WriteFile(filepath.Join(root, "wiki.toml"), []byte("spec = \"0.1\"\n"), 0o644)
	os.WriteFile(filepath.Join(root, "a.md"), []byte("---\ntype: note\n---\ninside\n"), 0o644)
	os.WriteFile(filepath.Join(top, "outside.md"), []byte("---\ntype: note\n---\nsecretword\n"), 0o644)
	if err := os.Symlink("../../outside.md", filepath.Join(root, "notes", "x.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink("../a.md", filepath.Join(root, "notes", "y.md")); err != nil {
		t.Fatal(err)
	}
	b, err := bundle.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Build(b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Resolve("/notes/x.md"); err == nil {
		t.Error("the link outside the bundle is an entry")
	}
	if _, err := idx.Resolve("/notes/y.md"); err != nil {
		t.Errorf("the link inside the bundle is not an entry: %v", err)
	}
	if hits := idx.Search("secretword", "", nil, 0); len(hits) != 0 {
		t.Errorf("the outside file is searchable: %+v", hits)
	}
	var found bool
	for _, i := range idx.Check() {
		if i.Entry == "/notes/x.md" && i.Level == "warning" && i.Msg == "is a symlink to a file outside the bundle: not indexed" {
			found = true
		}
	}
	if !found {
		t.Errorf("check does not name the link: %+v", idx.Check())
	}
}

// A link that stays inside the bundle but reaches a dot path (.git/config) is
// not content either: Build leaves it out, check names it, and a write through
// it is refused, so no field write can set core.hooksPath.
func TestALinkToADotPathIsNeitherReadNorWritten(t *testing.T) {
	root := t.TempDir()
	config := "[core]\n\tbare = false\n"
	os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	os.MkdirAll(filepath.Join(root, "notes"), 0o755)
	os.WriteFile(filepath.Join(root, ".git", "config"), []byte(config), 0o644)
	os.WriteFile(filepath.Join(root, "wiki.toml"), []byte("spec = \"0.1\"\n"), 0o644)
	link := filepath.Join(root, "notes", "x.md")
	if err := os.Symlink("../.git/config", link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	b, err := bundle.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Build(b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := idx.Resolve("/notes/x.md"); err == nil {
		t.Error("a link to .git/config is an entry")
	}
	var named bool
	for _, i := range idx.Check() {
		named = named || (i.Entry == "/notes/x.md" && i.Msg == "is a symlink to a file on a dot path (.git, a dot file): not indexed")
	}
	if !named {
		t.Errorf("check does not name the link: %+v", idx.Check())
	}

	// The write side, for a link that appears after the build.
	os.Remove(link)
	os.WriteFile(link, []byte("---\ntype: note\n---\n"), 0o644)
	if idx, err = Build(b); err != nil {
		t.Fatal(err)
	}
	e, err := idx.Resolve("/notes/x.md")
	if err != nil {
		t.Fatal(err)
	}
	os.Remove(link)
	os.Symlink("../.git/config", link)
	if err := e.SetField("hooksPath", "/tmp/evil"); !errors.Is(err, ErrOutsideBundle) {
		t.Errorf("err = %v, want ErrOutsideBundle", err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, ".git", "config")); string(got) != config {
		t.Errorf(".git/config was rewritten:\n%s", got)
	}
}

// move judges its destination where it would really land: through a folder
// linked out of the bundle it is refused before any folder is made, and so is
// a dot path. A destination with new folders inside the bundle still works.
func TestMoveKeepsItsDestinationInTheBundle(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(root, "wiki.toml"), []byte("spec = \"0.1\"\n"), 0o644)
	os.WriteFile(filepath.Join(root, "a.md"), []byte("---\ntype: note\n---\n"), 0o644)
	if err := os.Symlink(outside, filepath.Join(root, "ext")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	b, err := bundle.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Build(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, dest := range []string{"/ext/a.md", "/ext/new/deep/a.md", "/.drafts/a.md", "/notes/.a.md"} {
		if _, err := idx.Move("/a.md", dest, false, false); !errors.Is(err, ErrOutsideBundle) {
			t.Errorf("move to %s: err = %v, want ErrOutsideBundle", dest, err)
		}
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Errorf("the move reached outside the bundle: %v", entries)
	}
	if _, err := os.Stat(filepath.Join(root, "a.md")); err != nil {
		t.Errorf("a.md left its place: %v", err)
	}
	if _, err := idx.Move("/a.md", "/notes/new/a.md", false, false); err != nil {
		t.Errorf("a move into new folders inside the bundle: %v", err)
	}
}

// A link to nothing does not stop the wiki being built: it is left out, check
// names it, and every other entry is there.
func TestBuildSkipsADanglingLink(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "wiki.toml"), []byte("spec = \"0.1\"\n"), 0o644)
	os.WriteFile(filepath.Join(root, "a.md"), []byte("---\ntype: note\n---\n"), 0o644)
	if err := os.Symlink("missing.md", filepath.Join(root, "gone.md")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	b, err := bundle.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Build(b)
	if err != nil {
		t.Fatalf("one dangling link failed the whole build: %v", err)
	}
	if _, err := idx.Resolve("/a.md"); err != nil {
		t.Errorf("a.md is missing: %v", err)
	}
	var named bool
	for _, i := range idx.Check() {
		named = named || (i.Entry == "/gone.md" && i.Msg == "is a symlink to a file that doesn't exist: not indexed")
	}
	if !named {
		t.Errorf("check does not name the dangling link: %+v", idx.Check())
	}
}
