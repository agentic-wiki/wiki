// Package index scans a bundle's content tree and builds a queryable model.
package index

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/agentic-wiki/wiki/internal/bundle"
	"github.com/agentic-wiki/wiki/internal/parse"
)

// Entry is one markdown file in the content tree.
type Entry struct {
	Path     string          `json:"path"` // root-absolute, e.g. /finance/income/index.md
	Name     string          `json:"name"` // base name, e.g. index.md
	Type     string          `json:"type"`
	Title    string          `json:"title"`
	Tags     []string        `json:"tags"`
	Links    []parse.Link    `json:"-"`
	Tasks    []parse.Task    `json:"-"`
	Headings []parse.Heading `json:"-"`
	abs      string
	fm       map[string]any
}

// Depth is the number of folders below the content root (a top-level file is 0).
func (e *Entry) Depth() int {
	return strings.Count(strings.Trim(e.Path, "/"), "/")
}

// Body returns the entry's markdown body with the frontmatter block stripped,
// read fresh from disk. The index never holds bodies in memory, so this re-reads
// the one file on demand.
func (e *Entry) Body() (string, error) {
	data, err := os.ReadFile(e.abs)
	if err != nil {
		return "", err
	}
	_, body := parse.Frontmatter(string(data))
	return body, nil
}

// Index is the built model of a bundle.
type Index struct {
	Bundle  *bundle.Bundle
	Entries []*Entry
	byPath  map[string]*Entry
}

// Build scans the bundle directory and parses every .md file.
func Build(b *bundle.Bundle) (*Index, error) {
	idx := &Index{Bundle: b, byPath: map[string]*Entry{}}
	err := filepath.WalkDir(b.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != b.Dir && strings.HasPrefix(d.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		e, err := parseEntry(b, path)
		if err != nil {
			return err
		}
		idx.Entries = append(idx.Entries, e)
		idx.byPath[e.Path] = e
		return nil
	})
	if err != nil {
		return nil, err
	}
	return idx, nil
}

func parseEntry(b *bundle.Bundle, abs string) (*Entry, error) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	rel, _ := filepath.Rel(b.Dir, abs)
	fm, body := parse.Frontmatter(string(data))
	return &Entry{
		Path:     "/" + filepath.ToSlash(rel),
		Name:     filepath.Base(abs),
		Type:     parse.String(fm, "type"),
		Title:    parse.String(fm, "title"),
		Tags:     parse.Strings(fm, "tags"),
		Links:    parse.InternalLinks(body),
		Tasks:    parse.Tasks(body),
		Headings: parse.Headings(body),
		abs:      abs,
		fm:       fm,
	}, nil
}

// FileExists reports whether a root-absolute target resolves to a real file.
func (idx *Index) FileExists(target string) bool {
	if _, ok := idx.byPath[target]; ok {
		return true
	}
	p := filepath.Join(idx.Bundle.Dir, filepath.FromSlash(strings.TrimPrefix(target, "/")))
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

// Resolve finds a single entry by a "/"-containing path (matched exactly,
// root-absolute, leading slash optional) or a bare basename (matched across the
// tree). It errors if nothing matches or a basename is ambiguous. This is the
// shared resolver for commands that target one entry (read, outline, ...).
func (idx *Index) Resolve(arg string) (*Entry, error) {
	if strings.Contains(arg, "/") {
		target := "/" + strings.TrimPrefix(arg, "/")
		if e, ok := idx.byPath[target]; ok {
			return e, nil
		}
		return nil, fmt.Errorf("no entry at %s", target)
	}
	var matches []*Entry
	for _, e := range idx.Entries {
		if e.Name == arg {
			matches = append(matches, e)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return nil, fmt.Errorf("no entry named %q", arg)
	default:
		paths := make([]string, len(matches))
		for i, e := range matches {
			paths[i] = e.Path
		}
		slices.Sort(paths)
		return nil, fmt.Errorf("%q is ambiguous: %s", arg, strings.Join(paths, ", "))
	}
}

// Filter returns entries matching the given type, tag, and path prefix (any of
// which may be empty to skip).
func (idx *Index) Filter(typ, tag, pathPrefix string) []*Entry {
	var out []*Entry
	for _, e := range idx.Entries {
		if typ != "" && e.Type != typ {
			continue
		}
		if tag != "" && !slices.Contains(e.Tags, tag) {
			continue
		}
		if pathPrefix != "" && !hasPathPrefix(e.Path, pathPrefix) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// BrokenLink is an internal link with no target file.
type BrokenLink struct {
	From   string `json:"from"`
	Target string `json:"target"`
	Line   int    `json:"line"`
}

// Broken returns all internal links that do not resolve.
func (idx *Index) Broken() []BrokenLink {
	var out []BrokenLink
	for _, e := range idx.Entries {
		for _, l := range e.Links {
			if !idx.FileExists(l.Target) {
				out = append(out, BrokenLink{From: e.Path, Target: l.Target, Line: l.Line})
			}
		}
	}
	return out
}

// Orphans returns entries with no incoming internal links, excluding index.md
// (navigation entry points).
func (idx *Index) Orphans() []*Entry {
	incoming := map[string]int{}
	for _, e := range idx.Entries {
		for _, l := range e.Links {
			incoming[l.Target]++
		}
	}
	var out []*Entry
	for _, e := range idx.Entries {
		if e.Name == "index.md" {
			continue
		}
		if incoming[e.Path] == 0 {
			out = append(out, e)
		}
	}
	return out
}

// Issue is a validation finding.
type Issue struct {
	Level string `json:"level"` // error | warning
	Entry string `json:"entry"`
	Msg   string `json:"msg"`
}

// Check reports conformance and health issues. The one hard rule (a present
// `type`) and broken links are errors; everything else (undeclared type,
// reserved-file/type agreement, folder depth) is an advisory warning.
func (idx *Index) Check() []Issue {
	var issues []Issue
	for _, e := range idx.Entries {
		switch {
		case e.Type == "":
			issues = append(issues, Issue{"error", e.Path, "missing required `type`"})
		case !idx.Bundle.KnownType(e.Type):
			issues = append(issues, Issue{"warning", e.Path, "type not in vocabulary: " + e.Type})
		}
		if e.Name == "index.md" && e.Type != "" && e.Type != "index" {
			issues = append(issues, Issue{"warning", e.Path, "index.md should be type: index"})
		}
		if e.Name == "log.md" && e.Type != "" && e.Type != "log" {
			issues = append(issues, Issue{"warning", e.Path, "log.md should be type: log"})
		}
		if e.Depth() > 3 {
			issues = append(issues, Issue{"warning", e.Path, "deeper than 3 folders"})
		}
	}
	for _, b := range idx.Broken() {
		issues = append(issues, Issue{"error", b.From, "broken link -> " + b.Target})
	}
	// The bundle-root index.md carries OKF's okf_version badge; wiki.toml `spec`
	// is the source of truth, and the tool flags any drift between them.
	if root, ok := idx.byPath["/index.md"]; ok {
		if want, embedsOKF := idx.Bundle.OKFVersion(); embedsOKF {
			switch got := parse.String(root.fm, "okf_version"); got {
			case want:
				// in sync
			case "":
				issues = append(issues, Issue{"warning", root.Path, "missing okf_version (spec " + idx.Bundle.Spec + " embeds OKF " + want + ")"})
			default:
				issues = append(issues, Issue{"warning", root.Path, "okf_version " + got + " out of sync (spec " + idx.Bundle.Spec + " embeds OKF " + want + ")"})
			}
		}
	}
	return issues
}

func hasPathPrefix(path, prefix string) bool {
	return strings.HasPrefix(strings.TrimPrefix(path, "/"), strings.TrimPrefix(prefix, "/"))
}
