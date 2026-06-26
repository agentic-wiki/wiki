// Package index scans a bundle's content tree and builds a queryable model.
package index

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/agentic-wiki/wiki/internal/bundle"
	"github.com/agentic-wiki/wiki/internal/parse"
)

// Link is an internal link as indexed: its on-disk form (Raw, anchor kept) and
// its resolved root-absolute graph key (Target, anchor stripped). Queries and
// the graph match on Target; rewrites (Move, NormalizeLinks) match Raw on disk.
type Link struct {
	Text   string
	Raw    string
	Target string
	Line   int
}

// Entry is one markdown file in the content tree.
type Entry struct {
	Path     string          `json:"path"` // root-absolute, e.g. /finance/income/index.md
	Name     string          `json:"name"` // base name, e.g. index.md
	Type     string          `json:"type"`
	Title    string          `json:"title"`
	Tags     []string        `json:"tags"`
	Links    []Link          `json:"-"` // internal links, resolved to root-absolute: the graph edges
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
	// rel is the file's path under the bundle root ("finance/income.md");
	// entryPath is its root-absolute bundle id ("/finance/income.md").
	entryPath := "/" + filepath.ToSlash(rel)
	links := resolveLinks(parse.Links(body), entryPath, offset)
	tasks, heads := parse.Tasks(body), parse.Headings(body)
	for i := range tasks {
		tasks[i].Line += offset
	}
	for i := range heads {
		heads[i].Line += offset
	}
	return &Entry{
		Path:     entryPath,
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

// resolveLinks turns a body's parsed (as-written) internal links into indexed
// links: each keeps its on-disk Raw form and gains a resolved root-absolute
// Target (the graph key, anchor stripped). Lines are shifted by offset to be
// file-relative. Absolute and relative links both resolve via normalizeLink.
func resolveLinks(set parse.LinkSet, entryPath string, offset int) []Link {
	links := make([]Link, 0, len(set.Absolute)+len(set.Relative))
	for _, bucket := range [][]parse.Link{set.Absolute, set.Relative} {
		for _, l := range bucket {
			target := normalizeLink(entryPath, l.Target)
			if h := strings.IndexByte(target, '#'); h >= 0 {
				target = target[:h] // the graph edge is the file, not the anchor
			}
			links = append(links, Link{Text: l.Text, Raw: l.Target, Target: target, Line: l.Line + offset})
		}
	}
	return links
}

// normalizeLink resolves a link target, as written in the entry at fromPath, to
// its canonical root-absolute spelling (anchor preserved): the single standard
// representation of an internal link. Absolute targets pass through; relative
// ones join against the entry's directory (path.Join cleans `.`/`..` and cannot
// climb above the bundle root). The index uses it for the graph key (after
// dropping the anchor) and NormalizeLinks uses it for the on-disk rewrite.
func normalizeLink(fromPath, target string) string {
	anchor := ""
	if h := strings.IndexByte(target, '#'); h >= 0 {
		anchor, target = target[h:], target[:h]
	}
	if !strings.HasPrefix(target, "/") {
		target = path.Join(path.Dir(fromPath), target)
	}
	return target + anchor
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
// Backlinks returns every internal link that points to target, one LinkRef per
// occurrence (a source that links several times appears once per link), sorted
// by source path then line. Relative links count, they resolve to the same target.
func (idx *Index) Backlinks(target string) []LinkRef {
	var refs []LinkRef
	for _, e := range idx.Entries {
		for _, l := range e.Links {
			if l.Target == target {
				refs = append(refs, LinkRef{From: e.Path, To: target, Text: l.Text, Line: l.Line})
			}
		}
	}
	slices.SortStableFunc(refs, func(a, b LinkRef) int {
		if a.From != b.From {
			return strings.Compare(a.From, b.From)
		}
		return a.Line - b.Line
	})
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
// every internal link that targets it across the bundle, relative and
// root-absolute alike: links are matched by their resolved target and rewritten
// by their on-disk form. The moved file's own outgoing links stay valid. With
// dryRun it computes the plan without writing. There is no rollback: on a mid-way
// write error it returns what was already done so `unresolved` can surface any
// leftovers.
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

	// Rewrite every link whose resolved target is src, matching each by its
	// on-disk form (Raw) so relative and root-absolute links are both handled.
	for _, e := range idx.Entries {
		var hits []Link
		for _, l := range e.Links {
			if l.Target == src.Path {
				hits = append(hits, l)
			}
		}
		if len(hits) == 0 {
			continue
		}
		raw, err := e.Raw()
		if err != nil {
			return res, err
		}
		lines := strings.Split(raw, "\n") // link lines are file-relative
		n := 0
		for _, l := range hits {
			if l.Line-1 < 0 || l.Line-1 >= len(lines) {
				continue
			}
			anchor := ""
			if h := strings.IndexByte(l.Raw, '#'); h >= 0 {
				anchor = l.Raw[h:]
			}
			// Match the on-disk form, which may be angle-bracketed (`<…>`) if the
			// old target had a space. Re-wrap only if the new target still has one.
			re := regexp.MustCompile(`\]\(<?` + regexp.QuoteMeta(l.Raw) + `>?(\s[^)]*)?\)`)
			lines[l.Line-1] = re.ReplaceAllStringFunc(lines[l.Line-1], func(m string) string {
				n++
				nt := dest + anchor
				if strings.ContainsAny(nt, " \t") {
					nt = "<" + nt + ">"
				}
				return "](" + nt + re.FindStringSubmatch(m)[1] + ")" // keep anchor + title
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
		if e.Name == "index.md" || e.Name == "log.md" {
			// Reserved files (OKF §6/§7) are not concept documents: they carry no
			// frontmatter (the bundle-root index.md may carry okf_version) and are
			// exempt from the type requirement.
			for k := range e.fm {
				if e.Path == "/index.md" && k == "okf_version" {
					continue
				}
				issues = append(issues, Issue{"warning", e.Path, "reserved file should carry no frontmatter"})
				break
			}
			if e.Name == "log.md" {
				// OKF §7: log date headings use ISO YYYY-MM-DD.
				for _, h := range e.Headings {
					if looksLikeNonISODate(h.Text) {
						issues = append(issues, Issue{"warning", e.Path, "log date heading should be ISO YYYY-MM-DD: " + h.Text})
					}
				}
			}
		} else {
			switch {
			case e.Type == "":
				issues = append(issues, Issue{"error", e.Path, "missing required `type`"})
			case !idx.Bundle.KnownType(e.Type):
				issues = append(issues, Issue{"warning", e.Path, "type not in vocabulary: " + e.Type})
			}
			// timestamp is optional, but if present it must be valid ISO 8601.
			if _, ok := e.fm["timestamp"]; ok {
				if ts := parse.String(e.fm, "timestamp"); !validTimestamp(ts) {
					issues = append(issues, Issue{"error", e.Path, "timestamp not ISO 8601: " + ts})
				}
			}
		}
		if e.Depth() > 3 {
			issues = append(issues, Issue{"warning", e.Path, "deeper than 3 folders"})
		}
		if strings.Contains(e.Path, " ") {
			issues = append(issues, Issue{"warning", e.Path, "path contains a space; use a hyphenated slug"})
		}
	}
	// Broken links are reported as errors. Relative links are valid per OKF and
	// are resolved into the graph at build, so Broken covers them too (a relative
	// link that resolves nowhere shows up here); no separate relative-link check.
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

// Fix is a single change reported by `check --fix` or `tidy`: an entry's
// field (or a link) rewritten From -> To.
type Fix struct {
	Entry string `json:"entry"`
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// Fix applies the bundle's safe-to-write conformance repairs. Currently that is
// syncing the bundle-root index.md okf_version badge; the change is validated
// before any write, and written only when apply is true. (Relative-link
// normalization is not a repair, see NormalizeLinks.)
func (idx *Index) Fix(apply bool) ([]Fix, error) {
	okf, err := idx.fixOKFVersion(apply)
	if err != nil {
		return nil, err
	}
	if okf != nil {
		return []Fix{*okf}, nil
	}
	return nil, nil
}

// fixOKFVersion syncs the bundle-root index.md okf_version badge to the value
// the declared spec embeds. It returns the change, or nil when there is no root
// index, the spec embeds no OKF version, or the badge is already in sync.
func (idx *Index) fixOKFVersion(apply bool) (*Fix, error) {
	root, ok := idx.byPath["/index.md"]
	if !ok {
		return nil, nil
	}
	want, embedsOKF := idx.Bundle.OKFVersion()
	if !embedsOKF {
		return nil, nil
	}
	got := parse.String(root.fm, "okf_version")
	if got == want {
		return nil, nil
	}
	raw, err := os.ReadFile(root.abs)
	if err != nil {
		return nil, err
	}
	updated, err := setFrontmatterValue(string(raw), "okf_version", want)
	if err != nil {
		return nil, err
	}
	if apply {
		if err := os.WriteFile(root.abs, []byte(updated), 0o644); err != nil {
			return nil, err
		}
	}
	return &Fix{root.Path, "okf_version", got, want}, nil
}

// NormalizeLinks rewrites relative links to their canonical root-absolute form
// across the bundle. Relative links are valid per OKF, so this is an opt-in
// normalization, not a repair: it canonicalizes their on-disk spelling and never
// changes the graph (the resolved edge is already absolute in memory). With
// apply=false it reports the changes without writing.
func (idx *Index) NormalizeLinks(apply bool) ([]Fix, error) {
	var changes []Fix
	for _, e := range idx.Entries {
		c, err := idx.normalizeEntryLinks(e, apply)
		if err != nil {
			return nil, err
		}
		changes = append(changes, c...)
	}
	return changes, nil
}

// Slugify renames entry files whose basename contains a space to a hyphenated
// slug (e.g. "a b.md" -> "a-b.md", case preserved), rewriting inbound links via
// Move. Name collisions are skipped (Move refuses to clobber); spaced directories
// are out of scope. Returns one Fix per renamed file.
func (idx *Index) Slugify(apply bool) ([]Fix, error) {
	var changes []Fix
	for _, e := range idx.Entries {
		if !strings.Contains(e.Name, " ") {
			continue
		}
		dest := path.Join(path.Dir(e.Path), slugifyName(e.Name))
		if _, err := idx.Move(e.Path, dest, !apply); err != nil {
			continue // collision or unmovable: leave it (check still flags the space)
		}
		changes = append(changes, Fix{Entry: e.Path, Field: "rename", From: e.Path, To: dest})
	}
	return changes, nil
}

// slugifyName collapses whitespace runs in a filename to single hyphens.
func slugifyName(name string) string {
	return strings.Join(strings.Fields(name), "-")
}

// normalizeEntryLinks rewrites each relative link in one entry to its absolute
// form (anchor and title preserved), writing the file once. The absolute form is
// deterministic (path arithmetic), so it applies whether or not the target
// exists. Returns one Fix per link.
func (idx *Index) normalizeEntryLinks(e *Entry, apply bool) ([]Fix, error) {
	raw, err := e.Raw()
	if err != nil {
		return nil, err
	}
	_, body := parse.Frontmatter(raw)
	offset := strings.Count(raw[:len(raw)-len(body)], "\n")
	rels := parse.Links(body).Relative
	if len(rels) == 0 {
		return nil, nil
	}
	var changes []Fix
	lines := strings.Split(raw, "\n") // link lines are file-relative
	for _, l := range rels {
		abs := normalizeLink(e.Path, l.Target)
		if strings.ContainsAny(abs, " \t") {
			continue // a space target is flagged by check; the fix is renaming the file, not normalizing the link
		}
		changes = append(changes, Fix{e.Path, "link", l.Target, abs})
		ln := l.Line + offset - 1
		if !apply || ln < 0 || ln >= len(lines) {
			continue
		}
		re := regexp.MustCompile(`\]\(` + regexp.QuoteMeta(l.Target) + `(\s[^)]*)?\)`)
		lines[ln] = re.ReplaceAllStringFunc(lines[ln], func(m string) string {
			return "](" + abs + re.FindStringSubmatch(m)[1] + ")" // keep any title
		})
	}
	if apply {
		if err := os.WriteFile(e.abs, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
			return nil, err
		}
	}
	return changes, nil
}

// setFrontmatterValue returns content with the frontmatter `key` set to a quoted
// value, updating an existing key in place or inserting it as the last
// frontmatter line. It errors if there is no frontmatter block, and preserves
// the file's existing line endings.
func setFrontmatterValue(content, key, value string) (string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimRight(lines[0], "\r") != "---" {
		// No frontmatter block yet: create one carrying just this key.
		return fmt.Sprintf("---\n%s: \"%s\"\n---\n%s", key, value, content), nil
	}
	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimRight(lines[i], "\r") == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return "", fmt.Errorf("unterminated frontmatter")
	}
	cr := ""
	if strings.HasSuffix(lines[0], "\r") {
		cr = "\r"
	}
	newLine := fmt.Sprintf(`%s: "%s"`, key, value) + cr
	for i := 1; i < closeIdx; i++ {
		if k, _, ok := strings.Cut(strings.TrimRight(lines[i], "\r"), ":"); ok && strings.TrimSpace(k) == key {
			lines[i] = newLine
			return strings.Join(lines, "\n"), nil
		}
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:closeIdx]...)
	out = append(out, newLine)
	out = append(out, lines[closeIdx:]...)
	return strings.Join(out, "\n"), nil
}

// validTimestamp reports whether s is a non-empty ISO 8601 timestamp: an
// RFC 3339 datetime or a bare YYYY-MM-DD date.
func validTimestamp(s string) bool {
	if s == "" {
		return false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if _, err := time.Parse(layout, s); err == nil {
			return true
		}
	}
	return false
}

// nonISODateLayouts are common date spellings the log lint flags in favor of ISO.
var nonISODateLayouts = []string{"2006/01/02", "01/02/2006", "2006-1-2", "January 2, 2006", "Jan 2, 2006", "2 Jan 2006"}

// looksLikeNonISODate reports whether heading parses as a date in a common
// non-ISO format (and not already as ISO YYYY-MM-DD).
func looksLikeNonISODate(heading string) bool {
	h := strings.TrimSpace(heading)
	if _, err := time.Parse("2006-01-02", h); err == nil {
		return false // already ISO
	}
	for _, l := range nonISODateLayouts {
		if _, err := time.Parse(l, h); err == nil {
			return true
		}
	}
	return false
}

func hasPathPrefix(path, prefix string) bool {
	return strings.HasPrefix(strings.TrimPrefix(path, "/"), strings.TrimPrefix(prefix, "/"))
}
