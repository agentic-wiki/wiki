// Package project locates an agentic-wiki project and reads its config.
//
// A project root holds wiki.toml plus scratch (.wiki/, inbox/); its wiki/
// subfolder is the content root (the portable OKF bundle).
package project

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/agentic-wiki/wiki/internal/parse"
)

// Project is a located wiki project on disk.
type Project struct {
	RootDir    string   // project root (contains wiki.toml)
	ContentDir string   // <RootDir>/wiki — the content tree
	Spec       string   // spec version from wiki.toml
	Types      []string // content types declared in wiki.toml
}

// reservedTypes are recognized regardless of wiki.toml.
var reservedTypes = []string{"index", "log"}

// ErrNotFound is returned when no wiki.toml is found walking up from start.
var ErrNotFound = errors.New("no wiki.toml found (not inside a wiki project)")

// Discover walks up from start until it finds a directory containing wiki.toml.
func Discover(start string) (*Project, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	for {
		cfg := filepath.Join(dir, "wiki.toml")
		if fi, err := os.Stat(cfg); err == nil && !fi.IsDir() {
			return load(dir, cfg)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, ErrNotFound
		}
		dir = parent
	}
}

func load(root, cfg string) (*Project, error) {
	data, err := os.ReadFile(cfg)
	if err != nil {
		return nil, err
	}
	spec, types := parseConfig(string(data))
	return &Project{
		RootDir:    root,
		ContentDir: filepath.Join(root, "wiki"),
		Spec:       spec,
		Types:      types,
	}, nil
}

// KnownType reports whether t is a reserved or declared content type.
func (p *Project) KnownType(t string) bool {
	return slices.Contains(reservedTypes, t) || slices.Contains(p.Types, t)
}

// parseConfig reads the tiny wiki.toml we define: a `spec` string and a
// `types` array on one line. Deliberately minimal — no TOML dependency.
func parseConfig(s string) (spec string, types []string) {
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "spec":
			spec = parse.Unquote(val)
		case "types":
			types = parse.List(val)
		}
	}
	return spec, types
}
