// Package bundle locates an agentic-wiki bundle and reads its config.
//
// A bundle is a directory containing wiki.toml; the markdown content lives
// directly in that directory, with .wiki/ as a hidden, disposable cache.
package bundle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
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
	// Tool holds the [tool.<name>] tables, keyed by tool name. wiki never reads
	// inside them: they are space granted to other tools over the same bundle, so
	// one file describes the directory instead of a satellite config per tool.
	// Handed over raw so no consumer writes a second wiki.toml parser.
	Tool map[string]toml.Primitive
	// Unknown holds wiki.toml keys the tool does not recognize (a typo, or a
	// renamed field), as dotted paths. They are inert; `check` surfaces them so
	// they aren't silently ignored.
	Unknown []string

	md toml.MetaData // retained so Tool tables can be decoded on demand
}

// DecodeTool unmarshals the [tool.<name>] table into v, which is any struct or
// map the caller defines. Reports whether the table was present.
//
// The decoding happens here rather than in the consumer because the alternative
// is every tool parsing wiki.toml again: the same rule with two homes, which is
// the drift this repo keeps refusing.
func (b *Bundle) DecodeTool(name string, v any) (bool, error) {
	p, ok := b.Tool[name]
	if !ok {
		return false, nil
	}
	if err := b.md.PrimitiveDecode(p, v); err != nil {
		return true, fmt.Errorf("wiki.toml [tool.%s]: %w", name, err)
	}
	return true, nil
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

// config is the whole of wiki.toml the tool interprets. Anything else is either
// a [tool.*] table, which is deliberately opaque, or an unrecognized key.
type config struct {
	Spec          string                    `toml:"spec"`
	Types         []string                  `toml:"types"`
	Ignore        []string                  `toml:"ignore"`
	IgnoreOrphans []string                  `toml:"ignore_orphans"`
	Tool          map[string]toml.Primitive `toml:"tool"`
}

func load(root, cfg string) (*Bundle, error) {
	var c config
	// A malformed wiki.toml is an error rather than a shrug. The config decides
	// what is an entry and which types are valid, so reading half of it and
	// carrying on produces confidently wrong answers.
	md, err := toml.DecodeFile(cfg, &c)
	if err != nil {
		return nil, fmt.Errorf("wiki.toml: %w", err)
	}
	// Undecoded keys are the ones no field claimed. The [tool.*] subtree is
	// skipped rather than reported: reserving the namespace means wiki never has
	// an opinion about what is inside it. (Its keys show as undecoded because
	// toml.Primitive defers decoding, so the filter is explicit.)
	var unknown []string
	for _, k := range md.Undecoded() {
		if len(k) > 0 && k[0] == "tool" {
			continue
		}
		// Report the shallowest unrecognized key only: once [nested] is flagged,
		// listing every key inside it adds noise, not information.
		s := k.String()
		if slices.ContainsFunc(unknown, func(u string) bool { return strings.HasPrefix(s, u+".") }) {
			continue
		}
		unknown = append(unknown, s)
	}
	return &Bundle{
		Dir:           root,
		Spec:          c.Spec,
		Types:         c.Types,
		Ignore:        c.Ignore,
		IgnoreOrphans: c.IgnoreOrphans,
		Tool:          c.Tool,
		Unknown:       unknown,
		md:            md,
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
