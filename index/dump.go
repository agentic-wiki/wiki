package index

import (
	"bufio"
	"io"
	"path"
	"slices"
	"strings"

	"github.com/agentic-wiki/wiki/parse"
)

// ReadingOrder returns every entry once, in the order a reader following the
// bundle's indexes top-down would meet them. Starting at the root, each folder
// contributes:
//
//  1. its index.md, if present;
//  2. its direct children in the order index.md first links them, a subfolder
//     placed by a link to its index.md (or to the folder itself);
//  3. its remaining children, files and subfolders alike, by plain byte-wise path.
//
// A subfolder is emitted whole, recursively by the same rule, where it lands. A
// folder without index.md is therefore in path order, so a bundle with no indexes
// reads in path order. Links only order, never include: an entry nothing links to
// still appears. Only links to direct children count, so links to grandchildren,
// siblings or parents (and cycles between indexes) never move anything.
//
// Links are taken in line order. Two links of different forms on one line (say
// a root-absolute and a relative one) keep the index's form-grouped order, since
// links carry no column.
func (idx *Index) ReadingOrder() []*Entry {
	// children maps each folder ("/", "/a") to its direct children: entry paths
	// for files, folder paths for subfolders holding at least one entry. Each is
	// recorded once: entry paths are unique, and a folder is added to its parent
	// only the first time it is seen.
	children := map[string][]string{}
	isDir := map[string]bool{"/": true}
	for _, e := range idx.Entries {
		child := e.Path
		for dir := path.Dir(child); ; dir = path.Dir(dir) {
			children[dir] = append(children[dir], child)
			if isDir[dir] {
				break // the rest of the chain is already recorded
			}
			isDir[dir] = true
			child = dir
		}
	}

	out := make([]*Entry, 0, len(idx.Entries))
	var walk func(dir string)
	walk = func(dir string) {
		kids := children[dir]
		slices.Sort(kids) // byte-wise, never locale- or case-aware, so every OS agrees
		var order []string
		placed := map[string]bool{}
		place := func(c string) {
			if !placed[c] {
				placed[c] = true
				order = append(order, c)
			}
		}
		if ix, ok := idx.byPath[path.Join(dir, "index.md")]; ok {
			place(ix.Path)
			links := slices.Clone(ix.Links)
			slices.SortStableFunc(links, func(a, b Link) int { return a.Line - b.Line })
			for _, l := range links {
				c := l.Target
				if path.Base(c) == "index.md" && isDir[path.Dir(c)] && path.Dir(c) != dir {
					c = path.Dir(c) // a subfolder's index stands for the subfolder
				}
				if _, ok := slices.BinarySearch(kids, c); ok {
					place(c)
				}
			}
		}
		for _, c := range kids {
			place(c)
		}
		for _, c := range order {
			if isDir[c] {
				walk(c)
			} else {
				out = append(out, idx.byPath[c])
			}
		}
	}
	walk("/")
	return out
}

// Dump writes entries, in the order given, as one markdown stream: each entry is
// a delimiter line naming its root-absolute path,
//
//	<!-- /finance/income.md -->
//
// followed by its body, entries separated by one blank line. It is the bundle
// made portable to a single URL (llms-full.txt), so each body is transformed
// in memory, never on disk:
//
//   - internal links, relative on disk, become root-absolute, so they still mean
//     something in one file and match the delimiter of the entry they name
//     (anchors kept; an unresolved link gets the path it would have);
//   - resolved [[wikilinks]] become markdown links to that same root-absolute path;
//   - URLs, out-of-bundle links and unresolvable wikilinks are left as written;
//   - frontmatter is dropped unless withFrontmatter, which keeps it verbatim.
//
// The bytes depend only on the source: line endings are normalized to "\n", a
// UTF-8 BOM is dropped, leading and trailing blank lines are trimmed, and the
// stream ends with exactly one newline (an empty slice writes nothing).
//
// A body line identical to a delimiter cannot be told from a real one, and an
// unterminated "<!--" in a body hides what follows it when the dump is rendered
// as HTML; bodies are the author's own, so neither is escaped.
func (idx *Index) Dump(w io.Writer, entries []*Entry, withFrontmatter bool) error {
	resolve := idx.wikilinkResolver()
	bw := bufio.NewWriter(w)
	for i, e := range entries {
		text, err := idx.dumpText(e, resolve, withFrontmatter)
		if err != nil {
			return err
		}
		if i > 0 {
			bw.WriteString("\n")
		}
		bw.WriteString("<!-- " + e.Path + " -->\n")
		if text != "" {
			bw.WriteString(text + "\n")
		}
	}
	return bw.Flush()
}

// dumpText is one entry's dumped content: see Dump.
func (idx *Index) dumpText(e *Entry, resolve func(target, from string) string, withFrontmatter bool) (string, error) {
	raw, err := e.Raw()
	if err != nil {
		return "", err
	}
	// Wikilink rewrites address file-relative lines, so they go first, while the
	// raw content still has the lines they were parsed from.
	lines := strings.Split(raw, "\n")
	applyWikilinkRewrites(lines, e.wikilinkRewrites(resolve, func(abs string) string { return abs }))
	raw = strings.Join(lines, "\n")
	raw = strings.ReplaceAll(strings.TrimPrefix(raw, "\uFEFF"), "\r\n", "\n")

	_, body := parse.Frontmatter(raw)
	head := raw[:len(raw)-len(body)]
	body = parse.MapInternalLinks(body, func(target string) string {
		abs, escapes := normalizeLink(idx.Bundle.Dir, e.Path, target)
		if escapes {
			return target // out of the bundle: there is no root-absolute path to give it
		}
		return abs
	})
	if withFrontmatter {
		body = head + body
	}
	return strings.Trim(body, "\n"), nil
}
