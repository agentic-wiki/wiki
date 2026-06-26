package scaffold

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestWrite(t *testing.T) {
	dir := t.TempDir()
	written, err := Write(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	// the ignore file lands as .gitignore, never a bare "gitignore"
	if slices.Contains(written, "gitignore") || !slices.Contains(written, ".gitignore") {
		t.Errorf("written = %v (want .gitignore, not gitignore)", written)
	}
	for _, f := range []string{"wiki.toml", ".gitignore", "index.md", "notes/welcome.md"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	// a non-empty target is refused without force, allowed with it
	if _, err := Write(dir, false); err == nil {
		t.Errorf("re-write without force should error")
	}
	if _, err := Write(dir, true); err != nil {
		t.Errorf("force re-write: %v", err)
	}
}
