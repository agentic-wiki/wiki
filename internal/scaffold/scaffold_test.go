package scaffold

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/parse"
)

func TestWrite(t *testing.T) {
	dir := t.TempDir()
	written, err := Write(dir, "default", false)
	if err != nil {
		t.Fatal(err)
	}
	// the ignore file lands as .gitignore, never a bare "gitignore"
	if slices.Contains(written, "gitignore") || !slices.Contains(written, ".gitignore") {
		t.Errorf("written = %v (want .gitignore, not gitignore)", written)
	}
	for _, f := range []string{"wiki.toml", ".gitignore", "index.md", "AGENTS.md", "WORKFLOW.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	// the meta files are declared non-entries in the scaffolded wiki.toml
	toml, _ := os.ReadFile(filepath.Join(dir, "wiki.toml"))
	if !strings.Contains(string(toml), "ignore") {
		t.Errorf("scaffolded wiki.toml should list an ignore set:\n%s", toml)
	}
	// CLAUDE.md leads to AGENTS.md: a symlink mirrors its content, a stub points to it
	claude, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	agents, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if string(claude) != string(agents) && !strings.Contains(string(claude), "AGENTS.md") {
		t.Errorf("CLAUDE.md should mirror or point to AGENTS.md, got:\n%s", claude)
	}
	// a non-empty target is refused without force, allowed with it
	if _, err := Write(dir, "default", false); err == nil {
		t.Errorf("re-write without force should error")
	}
	if _, err := Write(dir, "default", true); err != nil {
		t.Errorf("force re-write: %v", err)
	}
}

func TestWriteUnknownWorkflow(t *testing.T) {
	if _, err := Write(t.TempDir(), "nope", false); err == nil {
		t.Errorf("unknown workflow should error")
	}
}

// A lone .git directory does not make the target "non-empty": a freshly
// `git init`'d (or empty-cloned) repo is a normal init target.
func TestWriteToleratesGitDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, "default", false); err != nil {
		t.Errorf("init into a .git-only dir should succeed without force: %v", err)
	}

	// .git alongside real content still requires --force
	dir2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir2, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "notes.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir2, "default", false); err == nil {
		t.Errorf("init into a dir with .git + content should still require force")
	}
}

// TestScaffoldIsOKFConformant locks the OKF v0.1 MUST-level rules on the bundle
// `wiki init` emits, independently of `wiki check` (which is an opt-in lint, not
// an OKF gate). It runs for every workflow, so a new starter that breaks
// conformance fails here.
func TestScaffoldIsOKFConformant(t *testing.T) {
	for _, wf := range Workflows() {
		t.Run(wf, func(t *testing.T) { assertOKFConformant(t, wf) })
	}
}

func assertOKFConformant(t *testing.T, workflow string) {
	dir := t.TempDir()
	if _, err := Write(dir, workflow, false); err != nil {
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
		case "AGENTS.md", "CLAUDE.md", "WORKFLOW.md":
			// declared non-entries (wiki.toml `ignore`): operating docs, no type
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
