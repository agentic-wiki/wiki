//go:build unix

package index

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/agentic-wiki/wiki/bundle"
)

// buildAndCheck builds the bundle at root and returns the index and what
// check says, by entry.
func buildAndCheck(t *testing.T, root string) (*Index, map[string]string) {
	t.Helper()
	b, err := bundle.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Build(b)
	if err != nil {
		t.Fatalf("the whole build failed: %v", err)
	}
	said := map[string]string{}
	for _, i := range idx.Check() {
		said[i.Entry] = i.Msg
	}
	return idx, said
}

// A .md that can't be read, a folder that can't be read, or a .md that is a
// link to a folder (a cloned repo can ship one) is left out and named by
// check, never failing the build of the whole wiki.
func TestBuildSkipsWhatItCantRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any file")
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "wiki.toml"), []byte("spec = \"0.1\"\n"), 0o644)
	os.WriteFile(filepath.Join(root, "a.md"), []byte("---\ntype: note\n---\n"), 0o644)
	os.WriteFile(filepath.Join(root, "locked.md"), []byte("---\ntype: note\n---\n"), 0o000)
	os.MkdirAll(filepath.Join(root, "shut"), 0o755)
	os.WriteFile(filepath.Join(root, "shut", "x.md"), []byte("---\ntype: note\n---\n"), 0o644)
	os.Chmod(filepath.Join(root, "shut"), 0o000)
	t.Cleanup(func() { os.Chmod(filepath.Join(root, "shut"), 0o755) })
	os.MkdirAll(filepath.Join(root, "somedir"), 0o755)
	if err := os.Symlink("somedir", filepath.Join(root, "dir.md")); err != nil {
		t.Fatal(err)
	}

	idx, said := buildAndCheck(t, root)
	if _, err := idx.Resolve("/a.md"); err != nil {
		t.Errorf("a.md is missing: %v", err)
	}
	for entry, want := range map[string]string{
		"/locked.md": "can't be read: not indexed",
		"/shut":      "is a folder that can't be read: not indexed",
		"/dir.md":    "is a symlink to a folder, not a file: not indexed",
	} {
		if said[entry] != want {
			t.Errorf("check on %s: %q, want %q", entry, said[entry], want)
		}
	}
}

// A .md that is itself a FIFO (no link) is left out and named: opening it
// would block Build forever.
func TestBuildSkipsAFIFO(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "wiki.toml"), []byte("spec = \"0.1\"\n"), 0o644)
	os.WriteFile(filepath.Join(root, "a.md"), []byte("---\ntype: note\n---\n"), 0o644)
	if err := syscall.Mkfifo(filepath.Join(root, "pipe.md"), 0o644); err != nil {
		t.Skipf("no FIFO here: %v", err)
	}
	b, err := bundle.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	type built struct {
		idx *Index
		err error
	}
	done := make(chan built, 1)
	go func() {
		idx, err := Build(b)
		done <- built{idx, err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("the whole build failed: %v", r.err)
		}
		said := map[string]string{}
		for _, i := range r.idx.Check() {
			said[i.Entry] = i.Msg
		}
		if want := "isn't a file: not indexed"; said["/pipe.md"] != want {
			t.Errorf("check on /pipe.md: %q, want %q", said["/pipe.md"], want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Build blocked on the FIFO")
	}
}
