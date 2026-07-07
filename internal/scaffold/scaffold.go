// Package scaffold writes a fresh agentic-wiki bundle from an embedded starter.
//
// The starter is split into shared files (the AGENTS.md operating manual, the
// .gitignore) and per-workflow files (wiki.toml, index.md, WORKFLOW.md) under
// files/workflows/<name>/. `wiki init --workflow <name>` picks one; the default
// is `generic`. Sourcing workflows from a separate repo is deferred (see backlog).
package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed files
var files embed.FS

// DefaultWorkflow is the starter used when --workflow is not given.
const DefaultWorkflow = "default"

// Workflows lists the embedded starter workflows available to `init`.
func Workflows() []string {
	entries, _ := fs.ReadDir(files, "files/workflows")
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	return out
}

// Write creates a fresh conformant bundle in dir from the named workflow's
// starter. It writes the shared files (AGENTS.md, and the ignore file, which
// ships as `gitignore` and lands as `.gitignore`), then the workflow's files
// (wiki.toml, index.md, WORKFLOW.md) flattened to the bundle root, then a
// CLAUDE.md symlink -> AGENTS.md so Claude Code reads the same manual (a one-line
// pointer file when the OS refuses a symlink, e.g. Windows without privilege).
// The scaffolded wiki.toml lists AGENTS.md/CLAUDE.md/WORKFLOW.md in `skip`, so
// they are not indexed as entries. A non-empty dir is refused unless force is
// set; a lone .git directory does not count as content. Returns the
// slash-separated relative paths written.
func Write(dir, workflow string, force bool) ([]string, error) {
	if workflow == "" {
		workflow = DefaultWorkflow
	}
	if !slices.Contains(Workflows(), workflow) {
		return nil, fmt.Errorf("unknown workflow %q (available: %s)", workflow, strings.Join(Workflows(), ", "))
	}
	if !force {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if e.Name() == ".git" {
					continue // a version-control dir alone is not "content"
				}
				return nil, fmt.Errorf("target %q is not empty (use --force)", dir)
			}
		}
	}

	var written []string
	emit := func(src, rel string) error {
		dest := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		data, err := files.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
		written = append(written, rel)
		return nil
	}

	// Shared files: everything directly under files/ (the workflows/ tree is next).
	shared, err := fs.ReadDir(files, "files")
	if err != nil {
		return nil, err
	}
	for _, e := range shared {
		if e.IsDir() {
			continue
		}
		rel := e.Name()
		if rel == "gitignore" {
			rel = ".gitignore" // dotfiles are awkward to embed
		}
		if err := emit("files/"+e.Name(), rel); err != nil {
			return written, err
		}
	}

	// Workflow files, flattened from files/workflows/<name>/ to the bundle root.
	wfRoot := "files/workflows/" + workflow
	err = fs.WalkDir(files, wfRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		return emit(p, strings.TrimPrefix(p, wfRoot+"/"))
	})
	if err != nil {
		return written, err
	}

	// CLAUDE.md -> AGENTS.md (relative), so Claude Code reads the same manual.
	claude := filepath.Join(dir, "CLAUDE.md")
	os.Remove(claude) // a --force re-init may leave an old one
	if err := os.Symlink("AGENTS.md", claude); err != nil {
		// The OS refused a symlink (e.g. Windows without privilege): leave a
		// one-line pointer so CLAUDE.md still leads to the manual.
		if err := os.WriteFile(claude, []byte("See [AGENTS.md](AGENTS.md).\n"), 0o644); err != nil {
			return written, err
		}
	}
	written = append(written, "CLAUDE.md")
	return written, nil
}
