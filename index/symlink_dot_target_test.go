package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentic-wiki/wiki/bundle"
)

// A cloned repo can ship `notes/setup.md -> ../.git/config`: a plain name
// whose file is a dot path inside the bundle. It must be neither an entry
// (its content would show in search, the tree and boards) nor writable (a
// body write into .git/config sets core.fsmonitor, which git then runs).
func TestASymlinkOntoADotPathIsNotAnEntry(t *testing.T) {
	root := t.TempDir()
	write := func(p, s string) {
		full := filepath.Join(root, filepath.FromSlash(p))
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("wiki.toml", "spec = \"0.1\"\n")
	config := "[remote \"origin\"]\n\turl = https://u:token@example.com/x\n"
	write(".git/config", config)
	os.MkdirAll(filepath.Join(root, "notes"), 0o755)
	if err := os.Symlink("../.git/config", filepath.Join(root, "notes", "setup.md")); err != nil {
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
	if e, err := idx.Resolve("/notes/setup.md"); err == nil {
		t.Error("a link onto .git/config is an entry")
		e.SetField("x", "y")
	}
	if got, _ := os.ReadFile(filepath.Join(root, ".git", "config")); string(got) != config {
		t.Errorf(".git/config was rewritten:\n%s", got)
	}
	if err := writeFile(root, filepath.Join(root, "notes", "setup.md"), []byte("x")); err == nil {
		t.Error("writeFile wrote through a link onto a dot path")
	}
}
