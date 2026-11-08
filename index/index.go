// Package index scans a bundle's content tree and builds a queryable model.
package index

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/agentic-wiki/wiki/bundle"
	"github.com/agentic-wiki/wiki/internal/wikilink"
	"github.com/agentic-wiki/wiki/parse"
)

// Link is an internal link as indexed: Raw as written; Target and Anchor are
// ResolveDest's Path and Anchor (Target is the climbing path for an outside
// link). Queries and the graph match on Target.
type Link struct {
	Text     string
	Raw      string
	Target   string
	Anchor   string
	Line     int
	Wikilink bool // true if this edge came from an Obsidian [[wikilink]] (Raw is its [[…]] form)
}

// Entry is one markdown file in the content tree. Its canonical, always-present
// metadata is the path (identity) and type (the one required, validated field).
// Every other frontmatter field (title, tags, status, …) lives only in fm, and
// its YAML-typed copy in typed, both from the same parser, and is read
// on demand, like `timestamp`. Links/Checkboxes/Headings are parsed from the
// body, not frontmatter, so they are materialized.
type Entry struct {
	Path        string           `json:"_path"` // root-absolute, e.g. /finance/income/index.md
	Type        string           `json:"type"`
	Links       []Link           `json:"-"` // internal links, resolved to root-absolute: the graph edges
	SelfAnchors []Link           `json:"-"` // pure #anchor links (Target = this entry); not graph edges, checked against own headings
	Outside     []Link           `json:"-"` // links climbing above the root (Raw kept; Target = the root-relative climbing path, ../PRD.md, for ignore matching)
	Checkboxes  []parse.Checkbox `json:"-"`
	Headings    []parse.Heading  `json:"-"`
	wikilinks   []wikilink.Link  // [[wikilinks]] parsed from the body (compat); resolved into Links (Wikilink) in Build
	abs         string           // the file, or "" for an entry Read outside a bundle
	root        string           // the bundle dir, so an entry can re-parse itself after a write
	content     string           // the text of an entry Read outside a bundle
	fm          map[string]any
	typed       parse.Map // fm with YAML types (parse.ReadFrontmatter): what MarshalJSON emits
	fmErr       error     // why the header wasn't read (parse.ReadFrontmatter), or nil
}

// inBundle reports whether the entry is a file of a bundle, not one from Read.
func (e *Entry) inBundle() bool { return e.abs != "" }

// base is the entry's filename (basename of Path), computed on demand: the path
// is the single source of truth for identity, so the basename is not stored.
func (e *Entry) base() string { return path.Base(e.Path) }

// IsLog reports whether the entry is a folder's log.md (OKF §7): the side
// narrative of how a folder changed, not its content. Matched by exact base
// name, case-sensitive, like every reserved filename.
func (e *Entry) IsLog() bool { return e.base() == "log.md" }

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
	if e.inBundle() {
		if fi, err := os.Stat(e.abs); err == nil {
			return fi.ModTime()
		}
	}
	return time.Time{}
}

// Raw returns the entry's full file content, read fresh from disk. The index
// holds only metadata, so bodies are re-read on demand. An entry Read outside
// a bundle returns the text it was read from.
func (e *Entry) Raw() (string, error) {
	if !e.inBundle() {
		return e.content, nil
	}
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
	return parse.Body(raw), nil
}

// MarshalJSON renders the entry as its typed frontmatter (TypedFrontmatter) plus
// its file path under the reserved key `_path`, first, then the keys in file
// order, maps and lists to any depth. The underscore prefix keeps
// `_path` off the frontmatter namespace, so a user's own `name:`/`path:` field
// round-trips untouched (a person's `name:` is their name, not the filename); the
// basename is just `basename(_path)`, so it is not emitted separately.
//
// Values carry their YAML types, so a script can query them without casts: an
// unquoted number is a JSON number as written (`10`, `0.2`), `true`/`false` a
// boolean, `~`/`null`/an empty value null; a quoted value, a date and anything
// else stay strings. No key is treated specially, `tags` included: a scalar
// stays a scalar. So `list --format json` exposes the full frontmatter (status,
// assignee, epic, …) that structured tooling needs.
func (e *Entry) MarshalJSON() ([]byte, error) {
	m := make(parse.Map, 0, len(e.typed)+1)
	m = append(m, parse.Pair{Key: "_path", Value: e.Path})
	for _, p := range e.typed {
		if p.Key != "_path" { // the entry's path wins over a frontmatter key
			m = append(m, p)
		}
	}
	return json.Marshal(m)
}

// MatchProperty reports whether the entry's frontmatter key holds value, on the
// string view: a scalar matches on equal text, a list when any item's text
// equals value (a list or map item compares as its Flow), and a map only on its
// whole Flow, keys in file order. Comparing against the empty string tests
// emptiness: `key=` matches when parse.Empty holds for the key's typed value
// (absent, null, "", [], {}, or a list of empty items), so its negation `key!=`
// matches when the key has a value. Backs the `--where` filter.
func (e *Entry) MatchProperty(key, value string) bool {
	if value == "" {
		v, _ := e.typed.Get(key)
		return parse.Empty(v)
	}
	return slices.Contains(parse.Strings(e.fm, key), value)
}

// Index is the built model of a bundle.
type Index struct {
	Bundle      *bundle.Bundle
	Entries     []*Entry
	byPath      map[string]*Entry
	ignoreIn    []string        // wiki.toml `ignore` patterns resolving inside: bundle-path globs; a match is not indexed as an entry
	ignoreOut   map[string]bool // wiki.toml `ignore` entries climbing above the root, by climbing path (../PRD.md): silence that out-of-bundle advisory
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
		e, err := parseEntry(b.Dir, path)
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
	resolve := idx.wikilinkResolver()
	for _, e := range idx.Entries {
		for _, wl := range e.wikilinks {
			target, anchor, display := wl.Split()
			resolved := resolve(target, e.Path)
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

// wikilinkResolver returns the Obsidian-style resolver over the bundle's entries
// and their `aliases`: a wikilink target (already split from #anchor and
// |display), as written in the entry at from, to its root-absolute path, or ""
// when nothing matches. Build, tidy and dump all resolve through it.
func (idx *Index) wikilinkResolver() func(target, from string) string {
	paths := make([]string, len(idx.Entries))
	aliasesPerEntry := map[string][]string{}
	for i, e := range idx.Entries {
		paths[i] = e.Path
		if a := parse.Strings(e.fm, "aliases"); len(a) > 0 {
			aliasesPerEntry[e.Path] = a
		}
	}
	aliases := wikilink.AliasMap(aliasesPerEntry)
	return func(target, from string) string {
		return wikilink.Resolve(target, from, paths, aliases)
	}
}

// resolveIgnore resolves the bundle's wiki.toml `ignore` and `ignore_orphans`
// lists into glob patterns matched by matchGlob. An `ignore` pattern resolving
// inside the bundle becomes a bundle-path glob (ignoreIn): a matching file is not
// indexed as an entry. One climbing above the root (ignoreOut, keyed by the
// root-relative climbing path, ../PRD.md, as an outside Link.Target is)
// acknowledges an out-of-bundle link so its advisory is silenced; these are
// exact (globbing external files is out of scope). `ignore_orphans` patterns
// become bundle-path globs (orphanGlobs). A pattern with no wildcard is an exact
// path (a single file), so `ignore = ["AGENTS.md"]` still works. Classification
// is pure lexical arithmetic, so an outside path is never stat'd or read.
func (idx *Index) resolveIgnore() {
	idx.ignoreOut = map[string]bool{}
	for _, s := range idx.Bundle.Ignore {
		rel := path.Clean(strings.TrimPrefix(filepath.ToSlash(s), "/"))
		if rel == ".." || strings.HasPrefix(rel, "../") {
			idx.ignoreOut[rel] = true
		} else {
			idx.ignoreIn = append(idx.ignoreIn, "/"+rel)
		}
	}
	for _, pat := range idx.Bundle.IgnoreOrphans {
		idx.orphanGlobs = append(idx.orphanGlobs, "/"+strings.TrimPrefix(filepath.ToSlash(pat), "/"))
	}
}

func parseEntry(root, abs string) (*Entry, error) {
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	rel, _ := filepath.Rel(root, abs)
	// rel is the file's path under the bundle root ("finance/income.md");
	// the entry's Path is its root-absolute bundle id ("/finance/income.md").
	e, body, offset := read("/"+filepath.ToSlash(rel), string(data))
	e.abs, e.root = abs, root
	e.wikilinks = wikilink.Parse(body) // resolved into graph edges in Build (needs all entries)
	for i := range e.wikilinks {
		e.wikilinks[i].Line += offset
	}
	return e, nil
}

// Read returns the entry that content would be at path p, read by the same
// rules as an entry of an index, for a file that isn't in one (a file read
// from git, say). p is cleaned and made root-absolute (docs/plan.md is
// /docs/plan.md). It has everything an indexed entry has, except wikilinks
// (which resolve against an index). Its SortTime is its `timestamp` alone,
// and any write to it is refused with ErrNotInBundle.
func Read(p, content string) *Entry {
	e, _, _ := read(path.Clean("/"+p), content)
	e.content = content
	return e
}

// read returns the entry that content would be at path p, without what needs
// a bundle (its file, its wikilinks), with its body and how many lines precede
// the body in the file.
func read(p, content string) (e *Entry, body string, offset int) {
	fm, typed, body, fmErr := parse.ReadFrontmatter(content) // one decode for both views
	// Checkboxes and headings are parsed from the frontmatter-stripped body, so
	// their line numbers are body-relative; offset by the frontmatter's length
	// to make them file-relative (what `unresolved`/`backlinks`/`tasks` report).
	offset = strings.Count(content[:len(content)-len(body)], "\n")
	checkboxes, heads := parse.Checkboxes(body), parse.Headings(body)
	for i := range checkboxes {
		checkboxes[i].Line += offset
	}
	for i := range heads {
		heads[i].Line += offset
	}
	links, selfAnchors, outside := resolveLinks(parse.Links(body), p, offset)
	return &Entry{
		Path:        p,
		Type:        parse.String(fm, "type"),
		Links:       links,
		SelfAnchors: selfAnchors,
		Outside:     outside,
		Checkboxes:  checkboxes,
		Headings:    heads,
		fm:          fm,
		typed:       typed,
		fmErr:       fmErr,
	}, body, offset
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

// resolveLinks reads each internal link of a body by ResolveDest; lines are
// shifted by offset to be file-relative, and a link climbing above the root
// goes to outside.
func resolveLinks(set parse.LinkSet, entryPath string, offset int) (links, selfAnchors, outside []Link) {
	links = make([]Link, 0, len(set.Absolute)+len(set.Relative))
	for _, bucket := range [][]parse.Link{set.Absolute, set.Relative} {
		for _, l := range bucket {
			d := ResolveDest(entryPath, l.Target)
			link := Link{Text: l.Text, Raw: l.Target, Target: d.Path, Anchor: d.Anchor, Line: l.Line + offset}
			if d.Outside {
				outside = append(outside, link)
				continue
			}
			links = append(links, link)
		}
	}
	// A pure #anchor points at a heading in this same entry: not a graph edge (it
	// would be a self-backlink), so it is kept apart, with Target set to the entry
	// itself, only for check to validate against the entry's own headings.
	for _, l := range set.SelfAnchors {
		selfAnchors = append(selfAnchors, Link{Text: l.Text, Raw: l.Target, Target: entryPath, Anchor: decoded(strings.TrimPrefix(l.Target, "#")), Line: l.Line + offset})
	}
	return links, selfAnchors, outside
}

// hasHeadingSlug reports whether the entry has a heading whose ID is anchor,
// read as a slug (parse.Slug), so a markdown fragment (`#my-heading`) and an
// Obsidian wikilink one (`#My Heading`) both find the same heading, and a
// repeat is reached by its "-N" (`#notes-1`).
func (e *Entry) hasHeadingSlug(anchor string) bool {
	anchor = parse.Slug(anchor)
	return slices.ContainsFunc(e.Headings, func(h parse.Heading) bool { return h.ID == anchor })
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

// TagCounts returns every tag in the bundle (optionally narrowed by path prefix
// and property filters, the same pair Filter takes)
// with the number of entries carrying it.
func (idx *Index) TagCounts(pathPrefix string, props []PropFilter) map[string]int {
	counts := map[string]int{}
	for _, e := range idx.Filter(pathPrefix, props) {
		for _, t := range dedupe(e.values("tags")) {
			counts[t]++
		}
	}
	return counts
}

// PropertyKeyCounts returns every frontmatter key in use (optionally narrowed by
// path prefix and property filters) with the number of entries that set it.
func (idx *Index) PropertyKeyCounts(pathPrefix string, props []PropFilter) map[string]int {
	counts := map[string]int{}
	for _, e := range idx.Filter(pathPrefix, props) {
		for k := range e.fm {
			counts[k]++
		}
	}
	return counts
}

// PropertyValueCounts returns the distinct values of frontmatter key (optionally
// narrowed by path prefix and property filters) with the number of entries
// holding each, on the string view. A list contributes each item (a list or
// map item as its Flow), counted once per entry; a map is its Flow. A value or
// item for which parse.Empty holds (null, "", [], {}) is skipped.
func (idx *Index) PropertyValueCounts(key, pathPrefix string, props []PropFilter) map[string]int {
	counts := map[string]int{}
	for _, e := range idx.Filter(pathPrefix, props) {
		for _, v := range dedupe(e.values(key)) {
			counts[v]++
		}
	}
	return counts
}

// values is the string view of key's value or items, without the empty ones,
// in a new slice.
func (e *Entry) values(key string) []string {
	strs := parse.Strings(e.fm, key)
	typed, _ := e.typed.Get(key)
	items, isList := typed.([]any)
	if !isList {
		if parse.Empty(typed) {
			return nil
		}
		return slices.Clone(strs)
	}
	var out []string
	for i, item := range items {
		if !parse.Empty(item) {
			out = append(out, strs[i])
		}
	}
	return out
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
//
// The json keys match LinkRef's: a broken link is a link row like any other, and
// naming its ends differently only made them harder to join.
type BrokenLink struct {
	From   string `json:"from"`
	Target string `json:"to"`
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
		if e.base() == "index.md" || e.IsLog() || idx.orphanExempt(e.Path) {
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

// Links returns the internal links written in the entry at path, one LinkRef per
// occurrence, in document order. The mirror of Backlinks, which answers the same
// question from the other end.
//
// Occurrences, not unique targets: an entry may link the same target from two
// places, and each has its own line. Collapsing them here would make Line the
// first of several, silently, which is a presentation choice the engine has no
// business making — `wiki links` shows bare targets and so dedupes, `wiki
// backlinks` shows file:line and so does not.
//
// An unknown path yields nil. Whether a target resolves is a health question for
// `check` / `unresolved`, not this navigational view.
func (idx *Index) Links(path string) []LinkRef {
	e, ok := idx.byPath[path]
	if !ok {
		return nil
	}
	refs := make([]LinkRef, 0, len(e.Links))
	for _, l := range e.Links {
		refs = append(refs, LinkRef{From: e.Path, To: l.Target, Text: l.Text, Line: l.Line})
	}
	return refs
}

// OutOfBundle returns the links written in the entry at path whose target
// resolves outside the bundle, one LinkRef per occurrence with To as written
// (`../PRD.md`), minus those acknowledged in wiki.toml `ignore`. They are not
// graph edges and cannot be verified from inside the bundle, which is why check
// and dump report them. An unknown path yields nil.
func (idx *Index) OutOfBundle(path string) []LinkRef {
	e, ok := idx.byPath[path]
	if !ok {
		return nil
	}
	var refs []LinkRef
	for _, l := range e.Outside {
		if idx.ignoreOut[l.Target] {
			continue // acknowledged in wiki.toml `ignore`
		}
		refs = append(refs, LinkRef{From: e.Path, To: l.Raw, Text: l.Text, Line: l.Line})
	}
	return refs
}

// Backlinks returns every internal link that points to target, one LinkRef per
// occurrence (a source that links several times appears once per link), sorted
// by source path then line. Relative links count, they resolve to the same target.
//
// This scans every edge in the bundle, which is the right shape for one target
// (sub-millisecond on a 5k-entry bundle) and the wrong one for all of them: a
// loop over every entry rescans the whole graph each time, ~2s where one pass
// would be ~10ms. Nothing needs all of them yet; when something does, it wants
// a single pass over Entries building a map keyed by Link.Target, not this.
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
	FrontmatterRefs int    `json:"frontmatter_refs"` // frontmatter values rewritten under --include-frontmatter (0 without it)
}

// MoveResult describes a (possibly dry-run) move: the relocation and the
// per-entry link rewrites it entails.
type MoveResult struct {
	From     string        `json:"from"`
	To       string        `json:"to"`
	DryRun   bool          `json:"dry_run"`
	Rewrites []FileRewrite `json:"rewrites"`
	// Warnings name the files whose frontmatter references --include-frontmatter
	// left as they were: a header wiki can't edit safely. The CLI prints them on
	// stderr; they are not part of the result's json.
	Warnings []string `json:"-"`
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
// With dryRun it computes the plan without writing. Otherwise it renames first
// and rewrites after, so a failed rename leaves the wiki as it was. A rewrite
// that fails after the rename leaves that file's references at the old path:
// the others are still written, and the error names each file, for
// `unresolved` to surface what is left.
//
// So a nil result with an error means nothing changed; a result with an error
// means the file moved, and the error joins one error per file not rewritten.
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

	// Plan the frontmatter rewrites first, before any file is written (dry run
	// included). parse.RewriteValues follows the rules every write does; one
	// rule here: a file it refuses (a header wiki can't edit safely, a key
	// written twice, a result that wouldn't read back as planned) keeps its
	// frontmatter as it is, with a warning naming the file and the reason, and
	// the move goes on. Only a file with a planned rewrite is checked.
	type fmPlan struct {
		text string // the file with its frontmatter rewritten: the text that was checked
		refs int    // values rewritten
	}
	plans := map[*Entry]fmPlan{}
	if includeFrontmatter {
		for _, e := range idx.Entries {
			if e.fmErr != nil {
				continue // an unreadable header has no fields to rewrite; loading the index warned
			}
			raw, err := e.Raw()
			if err != nil {
				return nil, err
			}
			text, refs, err := parse.RewriteValues(raw, idx.frontmatterRewrites(e, src.Path, dest))
			var refused *parse.EditError
			switch {
			case errors.As(err, &refused):
				res.Warnings = append(res.Warnings, fmt.Sprintf("%v; its frontmatter references are left as they are", e.named(err)))
				continue
			case err != nil:
				return nil, err
			}
			if refs == 0 {
				continue
			}
			plans[e] = fmPlan{text, refs}
		}
	}

	// Rewrite links through the one rule: every link whose destination
	// resolves to src is respelled to point at dest (relative from the linking
	// file), and the moved file's own outgoing links are respelled relative
	// from its new folder (its folder changed, so its relative links would
	// otherwise dangle). Only the path changes: the suffix (?query#anchor), the
	// title and a <…> form stay as written. Wikilinks resolve by basename each
	// run, so a relocation needs no rewrite. Nothing is written yet: each file's
	// new text is kept until the rename has succeeded.
	type write struct {
		e    *Entry
		text string
	}
	var writes []write
	for _, e := range idx.Entries {
		moved := e == src
		plan, rewritesFM := plans[e]
		if !moved && !rewritesFM && !slices.ContainsFunc(e.Links, func(l Link) bool { return l.Target == src.Path }) {
			continue
		}
		raw := plan.text // the checked frontmatter rewrite: it changes no line count
		if !rewritesFM {
			var err error
			if raw, err = e.Raw(); err != nil {
				return nil, err
			}
		}
		body := parse.Body(raw)
		head := raw[:len(raw)-len(body)]
		from := e.Path
		if moved {
			from = dest // e's place after the move (only src moves)
		}
		n := 0
		body = parse.MapInternalLinks(body, func(target string) string {
			d := ResolveDest(e.Path, target)
			switch {
			case d.Outside:
				return target // not in the bundle: left as written
			case d.Path == src.Path:
				d.Path = dest // a link to the moved file
			case !moved:
				return target
			} // else the moved file's own link to an unmoved target
			repl := spell(RelativeLink(from, d.Path), true) + suffixOf(target)
			if repl == target {
				return target // a same-folder rename leaves most spellings unchanged
			}
			n++
			return repl
		})
		fields := plan.refs
		if n == 0 && fields == 0 {
			continue
		}
		res.Rewrites = append(res.Rewrites, FileRewrite{Path: e.Path, Links: n, FrontmatterRefs: fields})
		writes = append(writes, write{e, head + body})
	}
	if dryRun {
		return res, nil
	}

	// Rename first, then rewrite: a move whose rename fails leaves the wiki as
	// it was. The moved file's own text goes to its new path, or, when it is a
	// symlink, to the file it points to, which doesn't move.
	movedAbs := destAbs
	if fi, err := os.Lstat(src.abs); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if real, err := filepath.EvalSymlinks(src.abs); err == nil {
			movedAbs = real
		}
	}
	made := missingDirs(filepath.Dir(destAbs))
	err = os.MkdirAll(filepath.Dir(destAbs), 0o755)
	if err == nil {
		// Through rename, not os.Rename: a move needs delete access to the
		// source, so it meets the same contention a replace does even though
		// the destination is known not to exist.
		err = rename(src.abs, destAbs)
	}
	if err != nil {
		for _, d := range made {
			os.Remove(d) // only an empty folder goes, so nothing else is lost
		}
		return nil, err
	}

	// The file has moved. A rewrite that fails now leaves links pointing at the
	// old path; the others are still written, and the error names each file
	// left as it was, for `wiki unresolved` to find its links.
	var errs []error
	for _, w := range writes {
		abs := w.e.abs
		if w.e == src {
			abs = movedAbs
		}
		if err := writeFile(abs, []byte(w.text)); err != nil {
			errs = append(errs, fmt.Errorf("moved %s to %s, but %s was not rewritten (run wiki unresolved): %w", src.Path, dest, w.e.Path, err))
		}
	}
	return res, errors.Join(errs...)
}

// missingDirs is dir and each of its parents that doesn't exist yet, deepest
// first: the folders os.MkdirAll(dir) would make.
func missingDirs(dir string) []string {
	var missing []string
	for d := dir; d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Lstat(d); err == nil {
			break
		}
		missing = append(missing, d)
	}
	return missing
}

// frontmatterRewrites is the callback parse.RewriteValues asks about every
// frontmatter value of entry e that ref reads as a reference: a value naming
// src is repointed at dest, and the moved file's own relative refs are
// normalized (its folder changes). It always writes the root-absolute form: a
// frontmatter ref is never rendered as a link, it must be a stable key, so
// `--where blockers=/active/x.md` finds every referrer by its text.
func (idx *Index) frontmatterRewrites(e *Entry, srcPath, dest string) func(key, v string) (string, bool) {
	return func(_, v string) (string, bool) {
		d, ok := ref(e.Path, v)
		switch {
		case !ok || d.Outside:
			return "", false
		case d.Path == srcPath:
			return spell(dest, false) + suffixOf(v), true
		case e.Path == srcPath:
			return spell(d.Path, false) + suffixOf(v), true // root-absolute: the canonical frontmatter form
		}
		return "", false // only the moved file normalizes its refs to unmoved targets
	}
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
	var root *Entry
	// An unrecognized wiki.toml key is silently inert otherwise (a typo, or a
	// renamed field like the old `skip`), so surface it rather than let the author
	// assume it took effect.
	for _, k := range idx.Bundle.Unknown {
		issues = append(issues, Issue{"warning", "wiki.toml", "unknown wiki.toml key: " + k})
	}
	for _, e := range idx.Entries {
		raw, err := e.Raw()
		if err != nil {
			issues = append(issues, Issue{"error", e.Path, "cannot read entry: " + err.Error()})
			continue
		}
		syntax, shape := parse.CheckFrontmatter(raw)
		if syntax != nil {
			issues = append(issues, Issue{"error", e.Path, "invalid frontmatter: " + syntax.Error()})
			continue // metadata-derived diagnostics would obscure the syntax error
		}
		// A shape issue sits next to the entry's other diagnostics, never instead of them.
		for _, err := range shape {
			issues = append(issues, Issue{"error", e.Path, err.Error()})
		}
		if e.Path == "/index.md" {
			root = e
		}
		if e.base() == "index.md" || e.IsLog() {
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
			if e.IsLog() {
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
		for _, r := range idx.OutOfBundle(e.Path) {
			issues = append(issues, Issue{"warning", e.Path, "out-of-bundle link -> " + r.To})
		}
	}
	// The bundle-root index.md carries OKF's okf_version badge; wiki.toml `spec`
	// is the source of truth, and the tool flags any drift between them.
	if root != nil {
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
	if err := parse.ValidateFrontmatter(string(raw)); err != nil {
		return nil, nil // Check reports the error; --fix must not rewrite a malformed header
	}
	updated, err := parse.SetField(string(raw), "okf_version", want)
	if err != nil {
		return nil, root.named(err)
	}
	if apply {
		if err := writeFile(root.abs, []byte(updated)); err != nil {
			return nil, err
		}
	}
	return &Fix{root.Path, "okf_version", got, want}, nil
}

// NormalizeLinks rewrites root-absolute links to their canonical relative form
// across the bundle. Root-absolute links are valid per OKF, so this is an opt-in
// normalization, not a repair: it canonicalizes their on-disk spelling and never
// changes the graph (every edge is resolved to root-absolute in memory). With
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
func (idx *Index) ConvertWikilinks(apply bool) ([]Fix, error) {
	resolve := idx.wikilinkResolver()
	var changes []Fix
	for _, e := range idx.Entries {
		if len(e.wikilinks) == 0 {
			continue
		}
		rws := e.wikilinkRewrites(resolve, func(abs string) string {
			return RelativeLink(e.Path, abs) // canonical relative on-disk form
		})
		for _, rw := range rws {
			if rw.To == "" {
				changes = append(changes, Fix{Entry: e.Path, Field: "wikilink-skip", From: rw.From})
			} else {
				changes = append(changes, Fix{Entry: e.Path, Field: "wikilink", From: rw.From, To: rw.To})
			}
		}
		if !apply {
			continue
		}
		raw, err := e.Raw()
		if err != nil {
			return nil, err
		}
		lines := strings.Split(raw, "\n")
		if applyWikilinkRewrites(lines, rws) {
			if err := writeFile(e.abs, []byte(strings.Join(lines, "\n"))); err != nil {
				return nil, err
			}
		}
	}
	return changes, nil
}

// wikilinkRewrite is one [[wikilink]] on a (file-relative) line and the markdown
// link replacing it. To is "" when the wikilink does not resolve: it stays as
// written.
type wikilinkRewrite struct {
	Line     int
	From, To string
}

// wikilinkRewrites converts the entry's [[wikilinks]] to markdown links: the
// path that form makes of the resolved root-absolute path, and the anchor,
// both written by spell, so tidy can write the relative on-disk form and
// dump the root-absolute one from the same conversion, and each reads back as
// it was meant. One rewrite per unique [[…]] on a line, embeds first:
// `[[x]]` is a substring of `![[x]]`, so replacing the bare form first would
// mangle an embed with the same target on that line.
func (e *Entry) wikilinkRewrites(resolve func(target, from string) string, form func(abs string) string) []wikilinkRewrite {
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
	seen := map[string]bool{}
	var rws []wikilinkRewrite
	for _, wl := range wls {
		key := fmt.Sprintf("%d:%s", wl.Line, wl.Full())
		if seen[key] {
			continue
		}
		seen[key] = true
		target, anchor, display := wl.Split()
		resolved := resolve(target, e.Path)
		if resolved == "" {
			rws = append(rws, wikilinkRewrite{Line: wl.Line, From: wl.Full()})
			continue
		}
		suffix := ""
		if anchor != "" {
			suffix = "#" + spell(anchor, true) // [[x#My Heading]] reads back as the anchor "My Heading"
		}
		url := spell(form(resolved), true) + suffix
		text := display
		if text == "" {
			text = target
		}
		rws = append(rws, wikilinkRewrite{Line: wl.Line, From: wl.Full(), To: "[" + text + "](" + url + ")"})
	}
	return rws
}

// applyWikilinkRewrites replaces each resolved rewrite on its line of lines (the
// entry's raw content split on "\n") and reports whether anything changed.
//
// Replacement is line-scoped by the parsed line number, but matches the [[…]] text
// on that line, so the rare case of an identical wikilink token appearing in an
// inline-code span on the same line as a real one would also be converted.
func applyWikilinkRewrites(lines []string, rws []wikilinkRewrite) bool {
	changed := false
	for _, rw := range rws {
		ln := rw.Line - 1
		if rw.To == "" || ln < 0 || ln >= len(lines) {
			continue
		}
		lines[ln] = strings.ReplaceAll(lines[ln], rw.From, rw.To)
		changed = true
	}
	return changed
}

// normalizeEntryLinks rewrites each root-absolute link in one entry to its
// canonical relative form, through the one rule (ResolveDest, then spell):
// only the path changes, the suffix (?query#anchor), the title and a <…> form
// stay as written. The relative form is path arithmetic, so it applies whether
// or not the target exists. A link climbing above the root is left exactly as
// authored, never clamped. Returns one Fix per link, in document order, and
// writes the file once.
func (idx *Index) normalizeEntryLinks(e *Entry, apply bool) ([]Fix, error) {
	raw, err := e.Raw()
	if err != nil {
		return nil, err
	}
	body := parse.Body(raw)
	head := raw[:len(raw)-len(body)]
	var changes []Fix
	body = parse.MapInternalLinks(body, func(target string) string {
		if !strings.HasPrefix(target, "/") {
			return target
		}
		d := ResolveDest(e.Path, target)
		if d.Outside {
			return target
		}
		repl := spell(RelativeLink(e.Path, d.Path), true) + suffixOf(target)
		changes = append(changes, Fix{e.Path, "link", target, repl})
		return repl
	})
	slices.Reverse(changes) // MapInternalLinks visits the links last to first
	if apply && len(changes) > 0 {
		if err := writeFile(e.abs, []byte(head+body)); err != nil {
			return nil, err
		}
	}
	return changes, nil
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

// Field returns a frontmatter value as a scalar ("" when absent or list-valued).
func (e *Entry) Field(key string) string { return parse.String(e.fm, key) }

// FieldList returns a frontmatter value as a list, treating a lone scalar as a
// one-element list, without its empty items (parse.Empty: "", null, [], {}),
// which wiki treats as no value everywhere (`property`, `tags`, `key=`).
//
// The list is new on every call: a caller may sort or filter it without
// changing the index, which every reader shares.
func (e *Entry) FieldList(key string) []string { return e.values(key) }

// Frontmatter returns the entry's frontmatter in the string view
// (parse.Frontmatter): each value is its text as YAML reads it, a list its
// items, a map its Flow, with no per-field coercion.
//
// The map is a copy, lists included: the index holds the original, and a caller
// that could mutate it would be editing the engine's state rather than reading it.
func (e *Entry) Frontmatter() map[string]any {
	out := make(map[string]any, len(e.fm))
	for k, v := range e.fm {
		if list, ok := v.([]string); ok {
			out[k] = slices.Clone(list)
			continue
		}
		out[k] = v
	}
	return out
}

// TypedFrontmatter is Frontmatter with YAML types (parse.ReadFrontmatter): a
// value is a string, a json.Number, a bool, nil, a []any or a parse.Map, keys
// in file order. It is what `list --format json` emits. It is a deep copy
// (Map.Clone): the index holds the original.
func (e *Entry) TypedFrontmatter() parse.Map {
	return e.typed.Clone()
}

// FrontmatterErr is why the entry's header couldn't be read (not valid YAML,
// not a mapping, unterminated, or over a limit), or nil. Such an entry stays
// in the index with no fields: its type is "", and its body is what follows
// the closing fence. `wiki check` reports it; every other command warns.
func (e *Entry) FrontmatterErr() error { return e.fmErr }

// Title is the entry's title by the scaffolded AGENTS.md's rule: its `title:`
// value, else the text of its first level-1 heading that has text, else "". A
// readable file name is the caller's fallback (presentation).
func (e *Entry) Title() string {
	if t := e.Field("title"); t != "" {
		return t
	}
	for _, h := range e.Headings {
		if h.Level == 1 && h.Text != "" {
			return h.Text
		}
	}
	return ""
}
