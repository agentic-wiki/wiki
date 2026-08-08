// Package index scans a bundle's content tree and builds a queryable model.
package index

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/agentic-wiki/wiki/bundle"
	"github.com/agentic-wiki/wiki/internal/wikilink"
	"github.com/agentic-wiki/wiki/parse"
)

// Link is an internal link as indexed: its on-disk form (Raw, anchor kept) and
// its resolved root-absolute graph key (Target, anchor stripped). Queries and
// the graph match on Target; rewrites (Move, NormalizeLinks) match Raw on disk.
type Link struct {
	Text     string
	Raw      string
	Target   string // resolved root-absolute path, anchor stripped (the graph edge is the file)
	Anchor   string // the "#fragment" without the '#', parsed from both markdown and wikilink forms ("" if none)
	Line     int
	Wikilink bool // true if this edge came from an Obsidian [[wikilink]] (Raw is its [[…]] form)
}

// anchorSuffix reconstructs the "#fragment" of a link (or "" if it has none), the
// single source for re-spelling an on-disk target with its anchor.
func (l Link) anchorSuffix() string {
	if l.Anchor == "" {
		return ""
	}
	return "#" + l.Anchor
}

// Entry is one markdown file in the content tree. Its canonical, always-present
// metadata is the path (identity) and type (the one required, validated field);
// those are the columns of tabular output. Every other frontmatter field (title,
// tags, status, …) lives only in fm and is read on demand, like `timestamp`, so
// there is a single source of truth. Links/Checkboxes/Headings are parsed from
// the body, not frontmatter, so they are materialized.
type Entry struct {
	Path        string           `json:"_path"` // root-absolute, e.g. /finance/income/index.md
	Type        string           `json:"type"`
	Links       []Link           `json:"-"` // internal links, resolved to root-absolute: the graph edges
	SelfAnchors []Link           `json:"-"` // pure #anchor links (Target = this entry); not graph edges, checked against own headings
	Outside     []Link           `json:"-"` // links resolving outside the bundle (Raw kept; Target = resolved abs fs path, for ignore matching)
	Checkboxes  []parse.Checkbox `json:"-"`
	Headings    []parse.Heading  `json:"-"`
	wikilinks   []wikilink.Link  // [[wikilinks]] parsed from the body (compat); resolved into Links (Wikilink) in Build
	abs         string
	fm          map[string]any
}

// base is the entry's filename (basename of Path), computed on demand: the path
// is the single source of truth for identity, so the basename is not stored.
func (e *Entry) base() string { return path.Base(e.Path) }

// Depth is the number of folders below the content root (a top-level file is 0).
func (e *Entry) Depth() int {
	return strings.Count(strings.Trim(e.Path, "/"), "/")
}

// SortTime is the entry's effective time for ordering: its curated frontmatter
// `timestamp` when present and valid, else the file's mtime, read on demand so
// only timestamp-less entries pay the stat.
func (e *Entry) SortTime() time.Time {
	if t, ok := parseTimestamp(parse.String(e.fm, "timestamp")); ok {
		return t
	}
	if fi, err := os.Stat(e.abs); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
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

// MarshalJSON renders the entry as its frontmatter verbatim plus its file path
// under the reserved key `_path`. The underscore prefix keeps `_path` off the
// frontmatter namespace, so a user's own `name:`/`path:` field round-trips
// untouched (a person's `name:` is their name, not the filename); the basename
// is just `basename(_path)`, so it is not emitted separately. Frontmatter is
// left exactly as written, with no per-field coercion, `tags` included: the tool
// is opaque about field meaning, so it can't single out one field to normalize.
// (Matching still treats a scalar and a one-element list alike, but that lives in
// the query layer, not here.) So `list --format json` exposes the full
// frontmatter (status, assignee, epic, …) that structured tooling needs. CSV/TSV
// keep the fixed canonical columns (the struct's json tags), since arbitrary
// keys don't fit a fixed column set.
func (e *Entry) MarshalJSON() ([]byte, error) {
	m := make(map[string]any, len(e.fm)+1)
	for k, v := range e.fm {
		m[k] = v
	}
	m["_path"] = e.Path
	return json.Marshal(m)
}

// MatchProperty reports whether the entry's frontmatter key holds value: a
// list-valued key (e.g. tags) matches when any element equals value, a scalar
// matches on equality. A missing key never matches a non-empty value. Comparing
// against the empty string tests emptiness: `key=` matches when the key has no
// non-empty value (absent, blank, or an empty list), so its negation `key!=`
// matches when the key is present and non-empty. Backs the `--where` filter.
func (e *Entry) MatchProperty(key, value string) bool {
	vals := parse.Strings(e.fm, key)
	if value == "" {
		return len(vals) == 0
	}
	return slices.Contains(vals, value)
}

// Index is the built model of a bundle.
type Index struct {
	Bundle      *bundle.Bundle
	Entries     []*Entry
	byPath      map[string]*Entry
	ignoreIn    []string        // wiki.toml `ignore` patterns resolving inside: bundle-path globs; a match is not indexed as an entry
	ignoreOut   map[string]bool // wiki.toml `ignore` entries resolving outside: absolute fs path -> silence that out-of-bundle advisory
	orphanGlobs []string        // wiki.toml `ignore_orphans` as bundle-path globs: matched entries stay indexed but are not reported as orphans
}

// Build scans the bundle directory and parses every .md file.
func Build(b *bundle.Bundle) (*Index, error) {
	idx := &Index{Bundle: b, byPath: map[string]*Entry{}}
	idx.resolveIgnore()
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
		rel, _ := filepath.Rel(b.Dir, path)
		if matchAnyGlob(idx.ignoreIn, "/"+filepath.ToSlash(rel)) {
			return nil // wiki.toml `ignore`: a declared non-entry, excluded from the content index
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
	idx.resolveWikilinks()
	return idx, nil
}

// resolveWikilinks is the second Build pass: it resolves each entry's parsed
// [[wikilinks]] against the full entry set (Obsidian-style, via internal/wikilink)
// and adds the resolved ones to the graph as edges marked Wikilink, so backlinks/
// orphans see them while Move/tidy/check can tell them from real markdown links.
// An unresolvable wikilink is left off the graph and surfaced only by check.
func (idx *Index) resolveWikilinks() {
	paths := make([]string, len(idx.Entries))
	aliasesPerEntry := map[string][]string{}
	for i, e := range idx.Entries {
		paths[i] = e.Path
		if a := parse.Strings(e.fm, "aliases"); len(a) > 0 {
			aliasesPerEntry[e.Path] = a
		}
	}
	aliases := wikilink.AliasMap(aliasesPerEntry)
	for _, e := range idx.Entries {
		for _, wl := range e.wikilinks {
			target, anchor, display := wl.Split()
			resolved := wikilink.Resolve(target, e.Path, paths, aliases)
			if resolved == "" {
				continue
			}
			text := display
			if text == "" {
				text = target
			}
			e.Links = append(e.Links, Link{Text: text, Raw: wl.Full(), Target: resolved, Anchor: anchor, Line: wl.Line, Wikilink: true})
		}
	}
}

// resolveIgnore resolves the bundle's wiki.toml `ignore` and `ignore_orphans`
// lists into glob patterns matched by matchGlob. An `ignore` pattern resolving
// inside the bundle becomes a bundle-path glob (ignoreIn): a matching file is not
// indexed as an entry. One resolving outside (ignoreOut, keyed by absolute fs
// path) acknowledges an out-of-bundle link so its advisory is silenced; these are
// exact (globbing external files is out of scope). `ignore_orphans` patterns
// become bundle-path globs (orphanGlobs). A pattern with no wildcard is an exact
// path (a single file), so `ignore = ["AGENTS.md"]` still works. Classification
// is pure lexical arithmetic, so an outside path is never stat'd or read.
func (idx *Index) resolveIgnore() {
	idx.ignoreOut = map[string]bool{}
	root := idx.Bundle.Dir
	for _, s := range idx.Bundle.Ignore {
		s = strings.TrimPrefix(filepath.ToSlash(s), "/")
		p := filepath.Join(root, filepath.FromSlash(s))
		if withinDir(root, p) {
			rel, _ := filepath.Rel(root, p)
			idx.ignoreIn = append(idx.ignoreIn, "/"+filepath.ToSlash(rel))
		} else {
			idx.ignoreOut[p] = true
		}
	}
	for _, pat := range idx.Bundle.IgnoreOrphans {
		idx.orphanGlobs = append(idx.orphanGlobs, "/"+strings.TrimPrefix(filepath.ToSlash(pat), "/"))
	}
}

func parseEntry(b *bundle.Bundle, abs string) (*Entry, error) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	rel, _ := filepath.Rel(b.Dir, abs)
	content := string(data)
	fm, body := parse.Frontmatter(content)
	// Links/checkboxes/headings are parsed from the frontmatter-stripped body, so
	// their line numbers are body-relative; offset by the frontmatter's length
	// to make them file-relative (what `unresolved`/`backlinks`/`tasks` report).
	offset := strings.Count(content[:len(content)-len(body)], "\n")
	// rel is the file's path under the bundle root ("finance/income.md");
	// entryPath is its root-absolute bundle id ("/finance/income.md").
	entryPath := "/" + filepath.ToSlash(rel)
	links, selfAnchors, outside := resolveLinks(b.Dir, parse.Links(body), entryPath, offset)
	checkboxes, heads := parse.Checkboxes(body), parse.Headings(body)
	for i := range checkboxes {
		checkboxes[i].Line += offset
	}
	for i := range heads {
		heads[i].Line += offset
	}
	wikilinks := wikilink.Parse(body) // resolved into graph edges in Build (needs all entries)
	for i := range wikilinks {
		wikilinks[i].Line += offset
	}
	return &Entry{
		Path:        entryPath,
		Type:        parse.String(fm, "type"),
		Links:       links,
		SelfAnchors: selfAnchors,
		Outside:     outside,
		Checkboxes:  checkboxes,
		Headings:    heads,
		wikilinks:   wikilinks,
		abs:         abs,
		fm:          fm,
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
// file-relative. Links whose target resolves outside the bundle are returned
// separately in outside (Raw kept, no resolved Target): they are not graph edges
// and the escaping path is never resolved on disk, but check reports them so the
// author knows the tool cannot verify them.
func resolveLinks(bundleRoot string, set parse.LinkSet, entryPath string, offset int) (links, selfAnchors, outside []Link) {
	links = make([]Link, 0, len(set.Absolute)+len(set.Relative))
	for _, bucket := range [][]parse.Link{set.Absolute, set.Relative} {
		for _, l := range bucket {
			target, escapes := normalizeLink(bundleRoot, entryPath, l.Target)
			if escapes {
				outside = append(outside, Link{Text: l.Text, Raw: l.Target, Target: target, Line: l.Line + offset})
				continue
			}
			anchor := ""
			if h := strings.IndexByte(target, '#'); h >= 0 {
				anchor = target[h+1:] // the graph edge is the file; the anchor is kept for check/rewrite
				target = target[:h]
			}
			links = append(links, Link{Text: l.Text, Raw: l.Target, Target: target, Anchor: anchor, Line: l.Line + offset})
		}
	}
	// A pure #anchor points at a heading in this same entry: not a graph edge (it
	// would be a self-backlink), so it is kept apart, with Target set to the entry
	// itself, only for check to validate against the entry's own headings.
	for _, l := range set.SelfAnchors {
		selfAnchors = append(selfAnchors, Link{Text: l.Text, Raw: l.Target, Target: entryPath, Anchor: strings.TrimPrefix(l.Target, "#"), Line: l.Line + offset})
	}
	return links, selfAnchors, outside
}

// normalizeLink resolves a link target, as written in the entry at fromPath, to
// its canonical root-absolute spelling within the bundle rooted at bundleRoot
// (anchor preserved): the single standard representation of an internal link. An
// absolute target resolves from the bundle root, a relative one from the entry's
// directory; the result is the final on-disk path with all `.`/`..` applied.
// escapes is true when that path lands outside the bundle — decided by withinDir,
// the same containment guard FileExists uses, so a `..` climb above the root
// (relative or absolute) is caught once, here, and never resolved on disk. Such a
// link points outside the self-contained bundle, so it is not an internal edge and
// callers skip it. The index uses the resolved form as the graph key (anchor
// dropped); NormalizeLinks uses it for the on-disk rewrite.
func normalizeLink(bundleRoot, fromPath, target string) (abs string, escapes bool) {
	anchor := ""
	if h := strings.IndexByte(target, '#'); h >= 0 {
		anchor, target = target[h:], target[:h]
	}
	base := bundleRoot
	if !strings.HasPrefix(target, "/") {
		base = filepath.Join(bundleRoot, filepath.FromSlash(strings.TrimPrefix(path.Dir(fromPath), "/")))
	}
	p := filepath.Join(base, filepath.FromSlash(strings.TrimPrefix(target, "/")))
	if !withinDir(bundleRoot, p) {
		return p, true // outside the bundle: return the resolved fs path (for ignore matching); never resolved on disk
	}
	rel, _ := filepath.Rel(bundleRoot, p) // no error: withinDir confirmed p is under bundleRoot
	if rel == "." {
		return "/" + anchor, false // the bundle root itself
	}
	return "/" + filepath.ToSlash(rel) + anchor, false
}

// relativeLink is the inverse of normalizeLink for an in-bundle target: it returns
// the on-disk relative spelling of a resolved root-absolute target (anchor kept),
// as written from the entry at fromPath. The result is slash-separated and always
// carries a leading "./" or "../", so it is unambiguously relative and resolves back
// to target from fromPath's directory. This is the canonical on-disk link form.
func relativeLink(fromPath, target string) string {
	anchor := ""
	if h := strings.IndexByte(target, '#'); h >= 0 {
		anchor, target = target[h:], target[:h]
	}
	return relPath(path.Dir(fromPath), target) + anchor
}

// relPath returns the relative slash-path from directory fromDir to target, both
// root-absolute bundle paths ("/…"). The result is prefixed with "./" (target at or
// below fromDir) or a run of "../" (target above it).
func relPath(fromDir, target string) string {
	from, to := splitSlash(fromDir), splitSlash(target)
	i := 0
	for i < len(from) && i < len(to) && from[i] == to[i] {
		i++
	}
	parts := make([]string, 0, len(from)-i+len(to)-i)
	for range from[i:] {
		parts = append(parts, "..")
	}
	parts = append(parts, to[i:]...)
	joined := strings.Join(parts, "/")
	if joined == "" {
		return "." // target is fromDir itself (degenerate for a file target)
	}
	if !strings.HasPrefix(joined, "../") && joined != ".." {
		joined = "./" + joined
	}
	return joined
}

// headingSlug renders heading text to a GitHub-style anchor slug: lowercased,
// punctuation dropped, spaces turned to hyphens (underscores and existing hyphens
// kept). It is text-based, not a full markdown render, so a heading with inline
// markup slugs its literal characters (a rare edge, documented for `check`).
func headingSlug(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// hasHeadingSlug reports whether the entry has a heading matching anchor. Both the
// anchor and each heading are reduced to a GitHub-style slug before comparing, so a
// markdown fragment (`#my-heading`) and an Obsidian wikilink one (`#My Heading`)
// both resolve to the same heading. Repeated headings are disambiguated as GitHub
// does: the Nth occurrence of a slug gains a "-N" suffix, the first stays bare (so a
// second "## Notes" is reachable as `#notes-1`).
func (e *Entry) hasHeadingSlug(anchor string) bool {
	anchor = headingSlug(anchor)
	seen := map[string]int{}
	for _, h := range e.Headings {
		s := headingSlug(h.Text)
		final := s
		if n := seen[s]; n > 0 {
			final = fmt.Sprintf("%s-%d", s, n)
		}
		seen[s]++
		if final == anchor {
			return true
		}
	}
	return false
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
		if e.base() == arg {
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

// PropFilter is one frontmatter key/value test for Filter/Search (the `--where`
// flag): an entry matches when its frontmatter key holds value (Entry.MatchProperty),
// or, with Negate (`key!=value`), when it does not, a missing key counts as "does
// not hold", so `key!=value` matches entries lacking the key entirely.
type PropFilter struct {
	Key    string
	Value  string
	Negate bool
}

// ParseFilter reads one `key=value` / `key!=value` expression.
//
// The spelling is part of the query contract, not of the CLI's argument
// handling, so it lives here: a consumer that accepted the same syntax and
// parsed it itself would be a second implementation of a rule with one correct
// home, and the two would drift on exactly the details that are easy to miss.
// `!=` is matched before `=` so a value may itself contain `=`, and the value is
// unquoted the way frontmatter is, so a quote surviving the shell
// (`k="v"`) still compares equal to `k: "v"`.
func ParseFilter(s string) (PropFilter, error) {
	neg := false
	kRaw, vRaw, ok := strings.Cut(s, "!=")
	if ok {
		neg = true
	} else {
		kRaw, vRaw, ok = strings.Cut(s, "=")
	}
	key := parse.Unquote(kRaw)
	if !ok || key == "" {
		return PropFilter{}, fmt.Errorf("%q is not a filter: expected key=value or key!=value", s)
	}
	return PropFilter{Key: key, Value: parse.Unquote(vRaw), Negate: neg}, nil
}

// Filter returns entries under pathPrefix (empty = the whole bundle) that satisfy
// every property filter (nil = no property constraint). props are ANDed; a
// list-valued field matches when it includes the value, a scalar when it equals
// it. type and tags are ordinary fields here (`type=note`, `tags=bug`), so one
// filter covers every frontmatter axis.
func (idx *Index) Filter(pathPrefix string, props []PropFilter) []*Entry {
	var out []*Entry
	for _, e := range idx.Entries {
		if pathPrefix != "" && !hasPathPrefix(e.Path, pathPrefix) {
			continue
		}
		if !e.matchesAll(props) {
			continue
		}
		out = append(out, e)
	}
	return out
}

// matchesAll reports whether the entry satisfies every property filter (AND). A
// negated filter (`key!=value`) passes when the entry does not hold that value,
// including when the key is absent.
func (e *Entry) matchesAll(props []PropFilter) bool {
	for _, p := range props {
		if e.MatchProperty(p.Key, p.Value) == p.Negate {
			return false
		}
	}
	return true
}

// TagCounts returns every tag in the bundle (optionally within a path prefix)
// with the number of entries carrying it.
func (idx *Index) TagCounts(pathPrefix string) map[string]int {
	counts := map[string]int{}
	for _, e := range idx.Filter(pathPrefix, nil) {
		for _, t := range dedupe(parse.Strings(e.fm, "tags")) {
			counts[t]++
		}
	}
	return counts
}

// PropertyKeyCounts returns every frontmatter key in use (optionally within a
// path prefix) with the number of entries that set it.
func (idx *Index) PropertyKeyCounts(pathPrefix string) map[string]int {
	counts := map[string]int{}
	for _, e := range idx.Filter(pathPrefix, nil) {
		for k := range e.fm {
			counts[k]++
		}
	}
	return counts
}

// PropertyValueCounts returns the distinct values of frontmatter key (optionally
// within a path prefix) with the number of entries holding each. A list-valued
// key (e.g. tags) contributes each element.
func (idx *Index) PropertyValueCounts(key, pathPrefix string) map[string]int {
	counts := map[string]int{}
	for _, e := range idx.Filter(pathPrefix, nil) {
		for _, v := range dedupe(parse.Strings(e.fm, key)) {
			counts[v]++
		}
	}
	return counts
}

// dedupe drops repeated strings, preserving first-seen order, so a value counts
// an entry once even if it appears twice in the same list.
func dedupe(items []string) []string {
	if len(items) < 2 {
		return items
	}
	var out []string
	seen := map[string]bool{}
	for _, it := range items {
		if !seen[it] {
			seen[it] = true
			out = append(out, it)
		}
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
	Path    string       `json:"_path"`
	Type    string       `json:"type"`
	Matches int          `json:"matches"`
	Lines   []SearchLine `json:"lines,omitempty"`
}

// SearchMode selects how a multi-word query matches a line. A single-word query
// behaves identically in all three modes.
type SearchMode int

const (
	SearchAll   SearchMode = iota // AND: the line contains every query word (any order) — the default (zero value)
	SearchAny                     // OR: the line contains any query word
	SearchExact                   // the line contains the query verbatim as one substring
)

// Search returns entries whose file (frontmatter + body) contains query,
// case-insensitively, after applying the path-prefix and property filters. mode
// selects all-words (default), any-word, or exact-phrase matching. Each hit
// carries its matching lines, sorted by path. Unreadable files are skipped.
func (idx *Index) Search(query, pathPrefix string, props []PropFilter, mode SearchMode) []SearchHit {
	q := strings.ToLower(query)
	var words []string
	if mode != SearchExact {
		words = strings.Fields(q)
	}
	var hits []SearchHit
	for _, e := range idx.Filter(pathPrefix, props) {
		raw, err := e.Raw()
		if err != nil {
			continue
		}
		var lines []SearchLine
		for i, line := range strings.Split(raw, "\n") {
			if matchLine(strings.ToLower(line), q, words, mode) {
				lines = append(lines, SearchLine{i + 1, line})
			}
		}
		if len(lines) > 0 {
			hits = append(hits, SearchHit{e.Path, e.Type, len(lines), lines})
		}
	}
	slices.SortFunc(hits, func(a, b SearchHit) int { return strings.Compare(a.Path, b.Path) })
	return hits
}

// matchLine reports whether a (lowercased) line satisfies the query under mode.
// SearchExact tests the whole query as one substring; the word modes test each
// whitespace-split token (SearchAny = any token, SearchAll = every token).
func matchLine(line, q string, words []string, mode SearchMode) bool {
	switch mode {
	case SearchExact:
		return strings.Contains(line, q)
	case SearchAny:
		for _, w := range words {
			if strings.Contains(line, w) {
				return true
			}
		}
		return false
	default: // SearchAll (the default)
		for _, w := range words {
			if !strings.Contains(line, w) {
				return false
			}
		}
		return true
	}
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

// Orphans returns entries with no incoming internal links, excluding the reserved
// files index.md (navigation entry points) and log.md (side narrative), and any
// path covered by wiki.toml `ignore_orphans` (parked or retired work kept out of
// the report).
func (idx *Index) Orphans() []*Entry {
	incoming := map[string]int{}
	for _, e := range idx.Entries {
		for _, l := range e.Links {
			incoming[l.Target]++
		}
	}
	var out []*Entry
	for _, e := range idx.Entries {
		if b := e.base(); b == "index.md" || b == "log.md" || idx.orphanExempt(e.Path) {
			continue
		}
		if incoming[e.Path] == 0 {
			out = append(out, e)
		}
	}
	return out
}

// orphanExempt reports whether p is covered by a wiki.toml `ignore_orphans`
// glob (see matchGlob): an exact path, a `dir/**` subtree, or any `*`/`?`/`**`
// pattern.
func (idx *Index) orphanExempt(p string) bool {
	return matchAnyGlob(idx.orphanGlobs, p)
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

// FileRewrite records what a move rewrote in one entry: body links, and, when
// --include-frontmatter is set, frontmatter values referencing the moved path.
type FileRewrite struct {
	Path            string `json:"_path"`
	Links           int    `json:"links"`
	FrontmatterRefs int    `json:"frontmatter_refs,omitempty"` // frontmatter values rewritten under --include-frontmatter
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
// links to the canonical relative on-disk form: every internal link that targets
// src (relative or root-absolute alike, matched by resolved target) is respelled
// relative from the linking file to dest, and the moved file's own outgoing links
// are respelled relative from its new directory (its dir changed, so its relative
// links would otherwise dangle). With includeFrontmatter it treats `*.md`-suffixed
// frontmatter values as references and keeps them valid too, resolving them the same
// way but writing them root-absolute, the stable-key form a metadata field needs
// (an opt-in: frontmatter is otherwise opaque, since the tool cannot know a
// path-shaped value is a reference rather than a snapshot).
// With dryRun it computes the plan without writing. There is no rollback: on
// a mid-way write error it returns what was already done so `unresolved` can surface
// any leftovers.
func (idx *Index) Move(srcArg, dest string, dryRun, includeFrontmatter bool) (*MoveResult, error) {
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

	// Rewrite links as relative (the canonical on-disk form), matching each by its
	// on-disk form (Raw) so relative and root-absolute links are both handled. Two
	// jobs: every link whose resolved target is src is respelled to point at dest
	// (relative from the *linking* file), and the moved file's own outgoing links
	// are respelled relative from its new directory (its dir changed, so its
	// relative links would otherwise dangle).
	for _, e := range idx.Entries {
		epath := e.Path
		if e == src {
			epath = dest // e's location after the move (only src moves)
		}
		type rewrite struct {
			line           int
			oldRaw, newRaw string
		}
		var rws []rewrite
		for _, l := range e.Links {
			// Wikilinks resolve by basename each run, so a relocation needs no
			// rewrite; and their [[…]] form isn't a markdown `](…)` target anyway.
			if l.Wikilink {
				continue
			}
			switch {
			case l.Target == src.Path: // a link to the moved file
				rws = append(rws, rewrite{l.Line, l.Raw, relativeLink(epath, dest+l.anchorSuffix())})
			case e == src: // the moved file's own outgoing link to an unmoved target
				rws = append(rws, rewrite{l.Line, l.Raw, relativeLink(dest, l.Target+l.anchorSuffix())})
			}
		}
		// A same-directory rename leaves most spellings unchanged; drop the no-ops.
		rws = slices.DeleteFunc(rws, func(r rewrite) bool { return r.newRaw == r.oldRaw })
		var fmRWs map[string][]fmRewrite
		if includeFrontmatter {
			fmRWs = idx.frontmatterRewrites(e, src.Path, dest)
		}
		if len(rws) == 0 && len(fmRWs) == 0 {
			continue
		}
		raw, err := e.Raw()
		if err != nil {
			return res, err
		}
		lines := strings.Split(raw, "\n") // link lines are file-relative
		n := 0
		for _, r := range rws {
			if r.line-1 < 0 || r.line-1 >= len(lines) {
				continue
			}
			// Match the on-disk form, which may be angle-bracketed (`<…>`) if the
			// old target had a space. Re-wrap only if the new target still has one.
			re := regexp.MustCompile(`\]\(<?` + regexp.QuoteMeta(r.oldRaw) + `>?(\s[^)]*)?\)`)
			lines[r.line-1] = re.ReplaceAllStringFunc(lines[r.line-1], func(m string) string {
				n++
				nt := r.newRaw
				if strings.ContainsAny(nt, " \t") {
					nt = "<" + nt + ">"
				}
				return "](" + nt + re.FindStringSubmatch(m)[1] + ")" // keep anchor + title
			})
		}
		fields := 0
		if len(fmRWs) > 0 {
			_, body := parse.Frontmatter(raw)
			fmEnd := strings.Count(raw[:len(raw)-len(body)], "\n")
			fields = rewriteFrontmatterRefs(lines, fmEnd, fmRWs)
		}
		if n == 0 && fields == 0 {
			continue
		}
		res.Rewrites = append(res.Rewrites, FileRewrite{Path: e.Path, Links: n, FrontmatterRefs: fields})
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

// fmRewrite is one frontmatter value respelling: the value exactly as written on
// disk, what it becomes, and the token-boundary pattern that finds it in a line.
type fmRewrite struct {
	oldRaw, newRaw string
	tok            *regexp.Regexp
}

// frontmatterRewrites plans the frontmatter respellings a move entails for entry
// e, keyed by frontmatter key. It is the frontmatter twin of the body-link pass in
// Move: a value referencing src is repointed at dest, and (only for the moved file
// itself, whose directory changes, so its relative refs would otherwise dangle) a
// relative value referencing anything else is normalized in place.
//
// Matching is by resolved target, not by spelling, so a relative ref is found as
// readily as a root-absolute one. The rewrite always emits the **root-absolute**
// form, which is where frontmatter deliberately parts company with body links:
// a body link is relative because it must navigate in a renderer, while a
// frontmatter ref is never rendered as a link and instead must be a *stable key*.
// Only one spelling per target keeps `--where blockers=/active/x.md` finding every
// referrer; relative values spell the same target differently from each directory,
// so no single query can match them all.
//
// A frontmatter value is considered a reference only when it ends in `.md` (after
// any `#anchor`). That suffix is the whole heuristic, and it is what keeps the
// pass from mangling ordinary metadata: an arbitrary value like `title: Some Note`
// would otherwise resolve to a perfectly valid in-bundle path and be rewritten as
// one. Values resolving outside the bundle are left alone, as body links are.
func (idx *Index) frontmatterRewrites(e *Entry, srcPath, dest string) map[string][]fmRewrite {
	var out map[string][]fmRewrite
	for k := range e.fm {
		var seen map[string]bool
		for _, v := range parse.Strings(e.fm, k) {
			if p, _, _ := strings.Cut(v, "#"); !strings.HasSuffix(p, ".md") {
				continue
			}
			abs, escapes := normalizeLink(idx.Bundle.Dir, e.Path, v)
			if escapes {
				continue
			}
			newRaw := abs // already root-absolute: the canonical frontmatter form
			switch {
			case stripAnchor(abs) == srcPath:
				newRaw = dest + anchorOf(abs)
			case e.Path != srcPath:
				continue // only the moved file normalizes its refs to unmoved targets
			}
			if newRaw == v || seen[v] {
				continue
			}
			if seen == nil {
				seen = map[string]bool{}
			}
			seen[v] = true
			if out == nil {
				out = map[string][]fmRewrite{}
			}
			out[k] = append(out[k], fmRewrite{
				oldRaw: v,
				newRaw: newRaw,
				tok:    regexp.MustCompile(`([:\[,\s"'])` + regexp.QuoteMeta(v) + `([\],\s"']|$)`),
			})
		}
		// Longest first, so a value that is a suffix-substring of another is never
		// matched inside it.
		slices.SortFunc(out[k], func(a, b fmRewrite) int { return len(b.oldRaw) - len(a.oldRaw) })
	}
	return out
}

func stripAnchor(p string) string { p, _, _ = strings.Cut(p, "#"); return p }

func anchorOf(p string) string {
	if _, a, ok := strings.Cut(p, "#"); ok {
		return "#" + a
	}
	return ""
}

// rewriteFrontmatterRefs applies planned respellings within the first fmEnd lines
// (an entry's frontmatter block). Keying by frontmatter key gates the rewrite, so a
// path that appears only as prose inside some other value is never touched; the
// token boundaries keep it to whole values (scalar, flow, or block list). Returns
// the number of values rewritten.
func rewriteFrontmatterRefs(lines []string, fmEnd int, rewrites map[string][]fmRewrite) int {
	n, lastKey := 0, ""
	for i := 0; i < fmEnd && i < len(lines); i++ {
		t := strings.TrimRight(lines[i], "\r")
		cr := lines[i][len(t):]
		if t == "---" {
			continue
		}
		owner := lastKey
		if !strings.HasPrefix(strings.TrimSpace(t), "- ") { // a "key: value" line, not a block item
			key, _, ok := strings.Cut(t, ":")
			if !ok {
				continue
			}
			owner = strings.TrimSpace(key)
			lastKey = owner
		}
		c := 0
		for _, rw := range rewrites[owner] {
			t = rw.tok.ReplaceAllStringFunc(t, func(m string) string {
				c++
				sub := rw.tok.FindStringSubmatch(m)
				return sub[1] + rw.newRaw + sub[2]
			})
		}
		if c > 0 {
			lines[i] = t + cr
			n += c
		}
	}
	return n
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

// Check reports conformance and health issues. Errors are genuine malformations
// (a missing `type`, a present-but-invalid timestamp); everything else is an
// advisory warning, including broken links — per OKF a broken link may be
// not-yet-written knowledge, so it does not fail the lint (`unresolved` lists them).
func (idx *Index) Check() []Issue {
	var issues []Issue
	// An unrecognized wiki.toml key is silently inert otherwise (a typo, or a
	// renamed field like the old `skip`), so surface it rather than let the author
	// assume it took effect.
	for _, k := range idx.Bundle.Unknown {
		issues = append(issues, Issue{"warning", "wiki.toml", "unknown wiki.toml key: " + k})
	}
	for _, e := range idx.Entries {
		b := e.base()
		if b == "index.md" || b == "log.md" {
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
			if b == "log.md" {
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
				issues = append(issues, Issue{"error", e.Path, "missing required `type` (or add this file to `ignore` in wiki.toml if it is not a bundle entry)"})
			case !idx.Bundle.KnownType(e.Type):
				// A vocabulary is opt-in (KnownType is true when none is declared), so
				// reaching here means wiki.toml `types` is set and this type is not in it.
				issues = append(issues, Issue{"error", e.Path, "type `" + e.Type + "` not in wiki.toml `types`; add it there or fix the entry"})
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
		// Wikilinks aren't part of the format (markdown links are the graph). They
		// are resolved as a courtesy but flagged once per file (the file, not each
		// link, is the unit a fix addresses).
		if n := len(e.wikilinks); n > 0 {
			issues = append(issues, Issue{"warning", e.Path, fmt.Sprintf("%d wikilink(s), not standard links; run `wiki tidy --wikilinks` to convert", n)})
		}
	}
	// Broken links are warnings, not errors: per OKF a broken link may be
	// not-yet-written knowledge, so a bundle with one still passes check
	// (`unresolved` is the to-write surface). Relative links are valid per OKF
	// and resolved into the graph at build, so Broken covers them too (a relative
	// link that resolves nowhere shows up here); no separate relative-link check.
	for _, b := range idx.Broken() {
		issues = append(issues, Issue{"warning", b.From, "broken link -> " + b.Target})
	}
	// A link's #anchor should point at a real heading in its target. The target
	// file itself is verified above (Broken); this additionally checks the fragment,
	// so a link to an existing file with a missing #section is caught. A miss is a
	// warning, like a broken link: a dangling reference, not a malformation. Covers
	// markdown and wikilink forms (both carry Anchor) and pure-#anchor self-links
	// (SelfAnchors, resolved against the entry's own headings). Obsidian block refs
	// (`#^id`) are left alone: they address a block, not a heading.
	for _, e := range idx.Entries {
		for _, l := range e.Links {
			if l.Anchor == "" || strings.HasPrefix(l.Anchor, "^") {
				continue
			}
			if t, ok := idx.byPath[l.Target]; ok && !t.hasHeadingSlug(l.Anchor) {
				issues = append(issues, Issue{"warning", e.Path, "link anchor not found -> " + l.Raw})
			}
		}
		for _, l := range e.SelfAnchors {
			if strings.HasPrefix(l.Anchor, "^") {
				continue
			}
			if !e.hasHeadingSlug(l.Anchor) {
				issues = append(issues, Issue{"warning", e.Path, "link anchor not found -> " + l.Raw})
			}
		}
	}
	// A link that resolves outside the bundle is not broken (the file lives
	// elsewhere) and not a graph edge, but it cannot be verified from within the
	// self-contained bundle, so it is surfaced as its own advisory. To silence it,
	// reference the out-of-bundle file as a code span or a full URL, not a link.
	for _, e := range idx.Entries {
		for _, l := range e.Outside {
			if idx.ignoreOut[l.Target] {
				continue // acknowledged in wiki.toml `ignore`
			}
			issues = append(issues, Issue{"warning", e.Path, "out-of-bundle link -> " + l.Raw})
		}
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
		b := e.base()
		if !strings.Contains(b, " ") {
			continue
		}
		dest := path.Join(path.Dir(e.Path), slugifyName(b))
		if _, err := idx.Move(e.Path, dest, !apply, false); err != nil {
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

// ConvertWikilinks rewrites each entry's [[wikilinks]] to canonical relative
// markdown, resolving Obsidian-style (internal/wikilink): `[[t]]` -> `[t](./t.md)`,
// `[[t|d]]` -> `[d](…)`, `[[t#h]]` -> `[t](…#h)`, and an embed `![[e]]` -> `[e](…)`
// (wiki has no transclusion, so an embed is just a reference). An unresolvable
// wikilink is left as written and reported (Field "wikilink-skip"; check flags it
// too). One Fix per unique link on a line (Field "wikilink" for conversions).
//
// Replacement is line-scoped by the parsed line number, but matches the [[…]] text
// on that line, so the rare case of an identical wikilink token appearing in an
// inline-code span on the same line as a real one would also be converted.
func (idx *Index) ConvertWikilinks(apply bool) ([]Fix, error) {
	paths := make([]string, len(idx.Entries))
	aliasesPerEntry := map[string][]string{}
	for i, e := range idx.Entries {
		paths[i] = e.Path
		if a := parse.Strings(e.fm, "aliases"); len(a) > 0 {
			aliasesPerEntry[e.Path] = a
		}
	}
	aliases := wikilink.AliasMap(aliasesPerEntry)
	var changes []Fix
	for _, e := range idx.Entries {
		if len(e.wikilinks) == 0 {
			continue
		}
		raw, err := e.Raw()
		if err != nil {
			return nil, err
		}
		lines := strings.Split(raw, "\n")
		seen := map[string]bool{} // one Fix per (line, [[…]]) even if it repeats
		changed := false
		// Convert embeds first: `[[x]]` is a substring of `![[x]]`, so replacing the
		// bare form first would mangle an embed with the same target on that line.
		wls := slices.Clone(e.wikilinks)
		slices.SortStableFunc(wls, func(a, b wikilink.Link) int {
			if a.IsEmbed == b.IsEmbed {
				return 0
			}
			if a.IsEmbed {
				return -1
			}
			return 1
		})
		for _, wl := range wls {
			key := fmt.Sprintf("%d:%s", wl.Line, wl.Full())
			if seen[key] {
				continue
			}
			seen[key] = true
			target, anchor, display := wl.Split()
			resolved := wikilink.Resolve(target, e.Path, paths, aliases)
			if resolved == "" {
				changes = append(changes, Fix{Entry: e.Path, Field: "wikilink-skip", From: wl.Full()})
				continue
			}
			abs := resolved
			if anchor != "" {
				abs += "#" + anchor
			}
			url := relativeLink(e.Path, abs) // canonical relative on-disk form
			if strings.ContainsAny(url, " \t") {
				url = "<" + url + ">"
			}
			text := display
			if text == "" {
				text = target
			}
			md := "[" + text + "](" + url + ")"
			changes = append(changes, Fix{Entry: e.Path, Field: "wikilink", From: wl.Full(), To: md})
			ln := wl.Line - 1
			if !apply || ln < 0 || ln >= len(lines) {
				continue
			}
			lines[ln] = strings.ReplaceAll(lines[ln], wl.Full(), md)
			changed = true
		}
		if apply && changed {
			if err := os.WriteFile(e.abs, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
				return nil, err
			}
		}
	}
	return changes, nil
}

// normalizeEntryLinks rewrites each root-absolute link in one entry to its
// canonical relative form (anchor and title preserved), writing the file once.
// The relative form is deterministic (path arithmetic), so it applies whether or
// not the target exists. Returns one Fix per link.
func (idx *Index) normalizeEntryLinks(e *Entry, apply bool) ([]Fix, error) {
	raw, err := e.Raw()
	if err != nil {
		return nil, err
	}
	_, body := parse.Frontmatter(raw)
	offset := strings.Count(raw[:len(raw)-len(body)], "\n")
	abs := parse.Links(body).Absolute
	if len(abs) == 0 {
		return nil, nil
	}
	var changes []Fix
	lines := strings.Split(raw, "\n") // link lines are file-relative
	for _, l := range abs {
		target, escapes := normalizeLink(idx.Bundle.Dir, e.Path, l.Target)
		if escapes {
			continue // out-of-bundle link: leave it exactly as authored, never clamp it
		}
		rel := relativeLink(e.Path, target)
		if strings.ContainsAny(rel, " \t") {
			continue // a space target is flagged by check; the fix is renaming the file, not normalizing the link
		}
		changes = append(changes, Fix{e.Path, "link", l.Target, rel})
		ln := l.Line + offset - 1
		if !apply || ln < 0 || ln >= len(lines) {
			continue
		}
		re := regexp.MustCompile(`\]\(` + regexp.QuoteMeta(l.Target) + `(\s[^)]*)?\)`)
		lines[ln] = re.ReplaceAllStringFunc(lines[ln], func(m string) string {
			return "](" + rel + re.FindStringSubmatch(m)[1] + ")" // keep any title
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

// parseTimestamp parses an ISO 8601 timestamp in the two forms the spec allows:
// an RFC 3339 datetime or a bare YYYY-MM-DD date. ok is false if neither matches.
func parseTimestamp(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// validTimestamp reports whether s is a non-empty ISO 8601 timestamp (see parseTimestamp).
func validTimestamp(s string) bool {
	if s == "" {
		return false
	}
	_, ok := parseTimestamp(s)
	return ok
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
