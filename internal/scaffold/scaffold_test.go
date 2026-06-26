package scaffold

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/internal/parse"
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

// TestScaffoldIsOKFConformant locks the OKF v0.1 MUST-level rules on the bundle
// `wiki init` emits, independently of `wiki check` (which is an opt-in lint, not
// an OKF gate). A future edit to files/ that breaks conformance fails here.
func TestScaffoldIsOKFConformant(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, false); err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fm, _ := parse.Frontmatter(string(raw))
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		switch d.Name() {
		case "index.md", "log.md":
			// Reserved filenames carry no frontmatter, except the bundle-root
			// index.md, which may carry okf_version and nothing else.
			if rel == "index.md" {
				if v := parse.String(fm, "okf_version"); v != "0.1" {
					t.Errorf("root index.md okf_version = %q, want \"0.1\"", v)
				}
				if len(fm) != 1 {
					t.Errorf("root index.md frontmatter must carry only okf_version, got %v", fm)
				}
			} else if len(fm) != 0 {
				t.Errorf("%s is reserved and must have no frontmatter, got %v", rel, fm)
			}
		default:
			// Every other entry MUST declare a non-empty type.
			if parse.String(fm, "type") == "" {
				t.Errorf("%s: missing required non-empty type", rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
