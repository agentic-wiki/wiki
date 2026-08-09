// Package wikilink is a compatibility shim for Obsidian-style [[wikilinks]].
// wiki does not officially support them (markdown links are the graph); this
// package only recognizes them so the graph stays correct and `tidy` can
// convert them to canonical markdown, resolving the way Obsidian would, with
// zero promises. It is deliberately quarantined here so the standard markdown
// link parsing (parse) and the graph (index) stay clean and only call in.
// Ported from the obsy predecessor; pure and dependency-free.
//
// All paths are wiki bundle paths: root-absolute and slash-separated (e.g.
// /finance/income.md), so this package uses the slash-only `path` package, never
// `path/filepath`.
package wikilink

import (
	"path"
	"sort"
	"strings"
)

// Link is one [[wikilink]] found in a body.
type Link struct {
	Raw     string // content between the brackets, e.g. "note#heading|display"
	IsEmbed bool   // true for ![[...]]
	Line    int    // 1-based line within the body
}

// Full returns the link's on-disk text, e.g. "[[note]]" or "![[note]]", so a
// rewrite (tidy) can find and replace the exact span.
func (l Link) Full() string {
	if l.IsEmbed {
		return "![[" + l.Raw + "]]"
	}
	return "[[" + l.Raw + "]]"
}

// Split breaks Raw into its parts: "note#heading|shown" -> target "note",
// anchor "heading", display "shown". A pipe escaped as \| (how Obsidian writes a
// pipe inside a table cell) still separates the display; the backslash is dropped.
func (l Link) Split() (target, anchor, display string) {
	s := strings.TrimSpace(l.Raw)
	if i := strings.IndexByte(s, '|'); i >= 0 {
		display = strings.TrimSpace(s[i+1:])
		s = strings.TrimSuffix(s[:i], "\\")
	}
	if i := strings.IndexByte(s, '#'); i >= 0 {
		anchor = strings.TrimSpace(s[i+1:])
		s = s[:i]
	}
	return strings.TrimSpace(s), anchor, display
}

// Parse extracts every wikilink in body, skipping fenced code blocks and inline
// code spans. body is the markdown after frontmatter has been stripped.
func Parse(body string) []Link {
	var links []Link
	inFence := false
	for n, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		links = append(links, parseLine(line, n+1)...)
	}
	return links
}

func parseLine(line string, lineNum int) []Link {
	var links []Link
	for i := 0; i < len(line); {
		// Skip an inline-code span: a wikilink inside backticks is documentation.
		if line[i] == '`' {
			if end := strings.IndexByte(line[i+1:], '`'); end >= 0 {
				i += end + 2
				continue
			}
		}
		isEmbed := false
		start := -1
		switch {
		case i+2 < len(line) && line[i] == '!' && line[i+1] == '[' && line[i+2] == '[':
			isEmbed, start, i = true, i+3, i+3
		case i+1 < len(line) && line[i] == '[' && line[i+1] == '[':
			start, i = i+2, i+2
		default:
			i++
			continue
		}
		end := strings.Index(line[start:], "]]")
		if end < 0 {
			continue
		}
		if raw := line[start : start+end]; raw != "" {
			links = append(links, Link{Raw: raw, IsEmbed: isEmbed, Line: lineNum})
		}
		i = start + end + 2
	}
	return links
}

// Resolve maps a wikilink target (already split off from #anchor and |display) to
// a bundle path, the way Obsidian would, or "" if nothing matches. paths is every
// entry's root-absolute path; fromPath is the entry holding the link (for
// tiebreaking); aliases maps a frontmatter alias to its entry path.
//
//   - a target containing "/" is an exact vault-relative path ([[folder/note]]);
//   - a bare target matches by basename anywhere, so [[note]] resolves even in a
//     sibling or parent folder, tiebroken by fewest path segments, then the same
//     folder as fromPath, then alphabetical (Obsidian's shortest-path rule).
func Resolve(target, fromPath string, paths []string, aliases map[string]string) string {
	if target == "" {
		return ""
	}
	withExt := target
	if !strings.HasSuffix(withExt, ".md") {
		withExt += ".md"
	}
	if strings.Contains(target, "/") {
		want := "/" + strings.TrimPrefix(withExt, "/")
		for _, p := range paths {
			if p == want {
				return p
			}
		}
		return ""
	}
	if p, ok := aliases[target]; ok {
		return p
	}
	base := path.Base(withExt)
	var candidates []string
	for _, p := range paths {
		if path.Base(p) == base {
			candidates = append(candidates, p)
		}
	}
	switch len(candidates) {
	case 0:
		return ""
	case 1:
		return candidates[0]
	}
	fromDir := path.Dir(fromPath)
	sort.Slice(candidates, func(i, j int) bool {
		if di, dj := strings.Count(candidates[i], "/"), strings.Count(candidates[j], "/"); di != dj {
			return di < dj
		}
		si, sj := path.Dir(candidates[i]) == fromDir, path.Dir(candidates[j]) == fromDir
		if si != sj {
			return si
		}
		return candidates[i] < candidates[j]
	})
	return candidates[0]
}

// AliasMap flattens per-entry aliases (path -> its frontmatter aliases) into an
// alias -> path lookup for Resolve. This is the one place the compat layer reads
// a frontmatter field; it stays inside this package.
func AliasMap(perEntry map[string][]string) map[string]string {
	m := make(map[string]string, len(perEntry))
	for p, aliases := range perEntry {
		for _, a := range aliases {
			if a = strings.TrimSpace(a); a != "" {
				m[a] = p
			}
		}
	}
	return m
}
