// Package scaffold writes a fresh agentic-wiki bundle from an embedded starter.
//
// v1 embeds the starter in this repo (the files/ directory). Sourcing it from
// the separate agentic-wiki/template repo as a go-module dependency, plus
// template selection, is the scaffold-registry work (see tasks/).
package scaffold

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed files
var files embed.FS

// Write creates a fresh conformant bundle in dir from the embedded starter. The
// ignore file ships as `gitignore` and is written out as `.gitignore` (dotfiles
// are awkward to embed). A non-empty dir is refused unless force is set. Returns
// the slash-separated relative paths written.
func Write(dir string, force bool) ([]string, error) {
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 && !force {
		return nil, fmt.Errorf("target %q is not empty (use --force)", dir)
	}
	var written []string
	err := fs.WalkDir(files, "files", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, "files/")
		if rel == "gitignore" {
			rel = ".gitignore"
		}
		dest := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		data, err := files.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
		written = append(written, rel)
		return nil
	})
	return written, err
}
