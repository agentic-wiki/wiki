// Package bundle locates an agentic-wiki bundle and reads its config.
//
// A bundle is a directory containing wiki.toml; the markdown content lives
// directly in that directory, with .wiki/ as a hidden, disposable cache.
package bundle

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/agentic-wiki/wiki/parse"
)

// Bundle is a located agentic-wiki bundle: a directory containing wiki.toml,
// with the markdown content living directly inside it.
type Bundle struct {
	Dir    string   // the bundle directory (holds wiki.toml); also the content root
	Spec   string   // spec version the bundle conforms to (from wiki.toml)
	Types  []string // content types declared in wiki.toml
	Ignore []string // paths (relative to Dir) wiki disregards: an in-bundle path is not indexed (not an entry); an out-of-bundle path silences that link's advisory
	// IgnoreOrphans lists paths (relative to Dir) whose entries stay indexed but are
	// not reported by `wiki orphans`: a directory subtree or an exact path.
	IgnoreOrphans []string
	// Unknown holds wiki.toml keys the tool does not recognize (a typo, or a
	// renamed field). They are inert; `check` surfaces them so they aren't
	// silently ignored.
	Unknown []string
}

// ErrNotFound is returned when no wiki.toml is found walking up from start.
var ErrNotFound = errors.New("no wiki.toml found (not inside a wiki bundle)")

// Discover walks up from start until it finds a directory containing wiki.toml.
func Discover(start string) (*Bundle, error) {
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

func load(root, cfg string) (*Bundle, error) {
	data, err := os.ReadFile(cfg)
	if err != nil {
		return nil, err
	}
	spec, types, ignore, ignoreOrphans, unknown := parseConfig(string(data))
	return &Bundle{
		Dir:           root,
		Spec:          spec,
		Types:         types,
		Ignore:        ignore,
		IgnoreOrphans: ignoreOrphans,
		Unknown:       unknown,
	}, nil
}

// KnownType reports whether t is an allowed content type. A declared vocabulary
// (`types` in wiki.toml) is opt-in: when none is declared (empty list), every
// type is allowed and this returns true; when one is declared, t must be in it.
func (b *Bundle) KnownType(t string) bool {
	return len(b.Types) == 0 || slices.Contains(b.Types, t)
}

// okfVersions maps an agentic-wiki spec version to the OKF version it embeds.
// Our spec is its own thing; OKF is one ingredient, declared to OKF consumers
// via okf_version in the bundle-root index.md. A future spec may embed a
// different OKF version, or none.
var okfVersionMap = map[string]string{"0.1": "0.1"}

// OKFVersion returns the OKF version this bundle's spec embeds, and whether the
// spec embeds OKF at all (false for an unknown or non-OKF spec).
func (b *Bundle) OKFVersion() (string, bool) {
	v, ok := okfVersionMap[b.Spec]
	return v, ok
}

// parseConfig reads the tiny wiki.toml we define: a `spec` string, and `types`,
// `ignore`, and `ignore_orphans` arrays, each on one line. Deliberately minimal
// (no TOML dependency). Any other key is collected in unknown so `check` can flag
// it rather than let a typo or a renamed field pass unnoticed.
func parseConfig(s string) (spec string, types, ignore, ignoreOrphans, unknown []string) {
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch k := strings.TrimSpace(key); k {
		case "spec":
			spec = parse.Unquote(val)
		case "types":
			types = parse.List(val)
		case "ignore":
			ignore = parse.List(val)
		case "ignore_orphans":
			ignoreOrphans = parse.List(val)
		default:
			unknown = append(unknown, k)
		}
	}
	return spec, types, ignore, ignoreOrphans, unknown
}
