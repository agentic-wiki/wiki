// Package index scans a bundle's content tree and builds a queryable model.
package index

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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

// Raw returns the entry's full file content, read fresh from disk. The index
// holds only metadata, so bodies are re-read on demand.
func (e *Entry) Raw() (string, error) {
	data, err := os.ReadFile(e.abs)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Body returns the entry's markdown body with the frontmatter block stripped.
func (e *Entry) Body() (string, error) {
	raw, err := e.Raw()
	if err != nil {
		return "", err
	}
	_, body := parse.Frontmatter(raw)
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
	content := string(data)
	fm, body := parse.Frontmatter(content)
	// Links/tasks/headings are parsed from the frontmatter-stripped body, so
	// their line numbers are body-relative; offset by the frontmatter's length
	// to make them file-relative (what `unresolved`/`backlinks`/`tasks` report).
	offset := strings.Count(content[:len(content)-len(body)], "\n")
	links, tasks, heads := parse.InternalLinks(body), parse.Tasks(body), parse.Headings(body)
	for i := range links {
		links[i].Line += offset
	}
	for i := range tasks {
		tasks[i].Line += offset
	}
	for i := range heads {
		heads[i].Line += offset
	}
	return &Entry{
		Path:     "/" + filepath.ToSlash(rel),
		Name:     filepath.Base(abs),
		Type:     parse.String(fm, "type"),
		Title:    parse.String(fm, "title"),
		Tags:     parse.Strings(fm, "tags"),
		Links:    links,
		Tasks:    tasks,
		Headings: heads,
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
	if !withinDir(idx.Bundle.Dir, p) {
		return false // a target that escapes the bundle is not a valid in-bundle file
	}
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

// SearchLine is one matching line within an entry (1-indexed, file-relative).
type SearchLine struct {
	Line int    `json:"line"`
	Text string `json:"text"`
}

// SearchHit is an entry with at least one line matching a search query.
type SearchHit struct {
	Path    string       `json:"path"`
	Type    string       `json:"type"`
	Title   string       `json:"title"`
	Matches int          `json:"matches"`
	Lines   []SearchLine `json:"lines,omitempty"`
}

// Search returns entries whose file (frontmatter + body) contains query,
// case-insensitively, after applying the type/tag/path filters. Each hit carries
// its matching lines, sorted by path. Unreadable files are skipped.
func (idx *Index) Search(query, typ, tag, pathPrefix string) []SearchHit {
	q := strings.ToLower(query)
	var hits []SearchHit
	for _, e := range idx.Filter(typ, tag, pathPrefix) {
		raw, err := e.Raw()
		if err != nil {
			continue
		}
		var lines []SearchLine
		for i, line := range strings.Split(raw, "\n") {
			if strings.Contains(strings.ToLower(line), q) {
				lines = append(lines, SearchLine{i + 1, line})
			}
		}
		if len(lines) > 0 {
			hits = append(hits, SearchHit{e.Path, e.Type, e.Title, len(lines), lines})
		}
	}
	slices.SortFunc(hits, func(a, b SearchHit) int { return strings.Compare(a.Path, b.Path) })
	return hits
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

// LinkRef is a directed internal link between two entries.
type LinkRef struct {
	From string `json:"from"`
	To   string `json:"to"`
	Text string `json:"text"`
	Line int    `json:"line"`
}

// OutLinks returns the entry's outgoing internal links as unique targets, in
// first-seen order. (Whether a target resolves is a health question for `check`
// / `unresolved`, not this navigational view.)
func (idx *Index) OutLinks(e *Entry) []LinkRef {
	seen := map[string]bool{}
	var refs []LinkRef
	for _, l := range e.Links {
		if seen[l.Target] {
			continue
		}
		seen[l.Target] = true
		refs = append(refs, LinkRef{From: e.Path, To: l.Target, Text: l.Text, Line: l.Line})
	}
	return refs
}

// Backlinks returns the unique entries that link to the given root-absolute path
// (the first link from each source), sorted by source path.
func (idx *Index) Backlinks(target string) []LinkRef {
	seen := map[string]bool{}
	var refs []LinkRef
	for _, e := range idx.Entries {
		for _, l := range e.Links {
			if l.Target == target && !seen[e.Path] {
				seen[e.Path] = true
				refs = append(refs, LinkRef{From: e.Path, To: target, Text: l.Text, Line: l.Line})
			}
		}
	}
	slices.SortFunc(refs, func(a, b LinkRef) int { return strings.Compare(a.From, b.From) })
	return refs
}

// FileRewrite records how many links were rewritten in one entry during a move.
type FileRewrite struct {
	Path  string `json:"path"`
	Links int    `json:"links"`
}

// MoveResult describes a (possibly dry-run) move: the relocation and the
// per-entry link rewrites it entails.
type MoveResult struct {
	From     string        `json:"from"`
	To       string        `json:"to"`
	DryRun   bool          `json:"dry_run"`
	Rewrites []FileRewrite `json:"rewrites"`
}

// Move relocates the entry at srcArg to dest (a root-absolute path) and rewrites
// every internal link that targets it across the bundle. Links are root-absolute,
// so only links *to* src need rewriting; the moved file's own links stay valid.
// With dryRun it computes the plan without writing. There is no rollback: on a
// mid-way write error it returns what was already done so `unresolved` can surface
// any leftovers.
func (idx *Index) Move(srcArg, dest string, dryRun bool) (*MoveResult, error) {
	src, err := idx.Resolve(srcArg)
	if err != nil {
		return nil, err
	}
	dest = "/" + filepath.ToSlash(strings.TrimPrefix(dest, "/"))
	destAbs := filepath.Join(idx.Bundle.Dir, filepath.FromSlash(strings.TrimPrefix(dest, "/")))
	switch {
	case !strings.HasSuffix(dest, ".md"):
		return nil, fmt.Errorf("destination must be a .md path: %s", dest)
	case dest == src.Path:
		return nil, fmt.Errorf("destination equals source: %s", dest)
	case !withinDir(idx.Bundle.Dir, destAbs):
		return nil, fmt.Errorf("destination escapes the bundle: %s", dest)
	case idx.FileExists(dest):
		return nil, fmt.Errorf("destination already exists: %s", dest)
	}
	res := &MoveResult{From: src.Path, To: dest, DryRun: dryRun}

	// rewrite incoming links, anchored to the lines the parser found real links on
	re := regexp.MustCompile(`\]\(` + regexp.QuoteMeta(src.Path) + `(#[^)\s]*)?(\s[^)]*)?\)`)
	for _, e := range idx.Entries {
		onLine := map[int]bool{}
		for _, l := range e.Links {
			if l.Target == src.Path {
				onLine[l.Line] = true
			}
		}
		if len(onLine) == 0 {
			continue
		}
		raw, err := e.Raw()
		if err != nil {
			return res, err
		}
		// Link lines are file-relative, so match directly against the raw file.
		lines := strings.Split(raw, "\n")
		n := 0
		for i := range lines {
			if !onLine[i+1] {
				continue
			}
			lines[i] = re.ReplaceAllStringFunc(lines[i], func(m string) string {
				n++
				sub := re.FindStringSubmatch(m) // [1]=#anchor, [2]=" title"
				return "](" + dest + sub[1] + sub[2] + ")"
			})
		}
		if n == 0 {
			continue
		}
		res.Rewrites = append(res.Rewrites, FileRewrite{Path: e.Path, Links: n})
		if !dryRun {
			if err := os.WriteFile(e.abs, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
				return res, err
			}
		}
	}

	if !dryRun {
		if err := os.MkdirAll(filepath.Dir(destAbs), 0o755); err != nil {
			return res, err
		}
		if err := os.Rename(src.abs, destAbs); err != nil {
			return res, err
		}
	}
	return res, nil
}

// withinDir reports whether p is dir itself or lexically inside it, guarding
// against `..` escapes.
func withinDir(dir, p string) bool {
	rel, err := filepath.Rel(dir, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
