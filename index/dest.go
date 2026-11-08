package index

import (
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/agentic-wiki/wiki/parse"
)

// Dest is what a link destination names, read by the wiki's one rule.
type Dest struct {
	// Path is root-absolute and decoded once ("/docs/my plan.md"; "/" for the
	// root). Never unescape it again: "%252e" is the literal name "%2e". When
	// the destination climbs above the root, it is the root-relative climbing
	// path ("../README.md"), and Outside is set.
	Path    string
	Anchor  string // after the first '#', decoded, without the '#'; "" if none
	Outside bool
}

// ResolveDest reads dest, a link destination written in the entry at from
// (root-absolute, e.g. "/docs/a.md"). Call it only for an internal destination
// (parse.Links: absolute or relative):
//  1. cut the fragment at the first '#' (the anchor), then the query at the
//     first '?' (dropped);
//  2. percent-decode the path, once; "%2F" decodes to "/", since a file name
//     can't hold one. A path that isn't valid encoding, or that decodes to a
//     NUL or to invalid UTF-8, is taken as written;
//  3. join it to the wiki's root when it starts with '/', else to from's folder;
//  4. path.Clean;
//  5. outside when the result is ".." or starts with "../" (segment-exact).
//
// It reads no file: existence is the caller's to check.
func ResolveDest(from, dest string) Dest {
	var d Dest
	p, anchor, hasAnchor := strings.Cut(dest, "#")
	if hasAnchor {
		d.Anchor = decoded(anchor)
	}
	p, _, _ = strings.Cut(p, "?")
	p = decoded(p)
	// Root-relative, so a climb above the root stays visible: path.Join on a
	// rooted path would clamp it at "/".
	base := strings.TrimPrefix(path.Dir(from), "/")
	if strings.HasPrefix(p, "/") {
		base = ""
	}
	rel := path.Clean(path.Join(base, strings.TrimPrefix(p, "/")))
	switch {
	case rel == ".." || strings.HasPrefix(rel, "../"):
		d.Path, d.Outside = rel, true
	case rel == ".":
		d.Path = "/"
	default:
		d.Path = "/" + rel
	}
	return d
}

// decoded is s percent-decoded once, or s as written when it isn't valid
// encoding or decodes to a NUL or to invalid UTF-8.
func decoded(s string) string {
	d, err := url.PathUnescape(s)
	if err != nil || strings.ContainsRune(d, 0) || !utf8.ValidString(d) {
		return s
	}
	return d
}

// suffixOf is what a rewrite copies byte for byte from a destination as
// written: from its first '?' or '#' on ("?raw=1#x"), or "".
func suffixOf(dest string) string {
	if i := strings.IndexAny(dest, "?#"); i >= 0 {
		return dest[i:]
	}
	return ""
}

// spell writes the path p so that it reads back through ResolveDest as itself.
// Both modes percent-encode %, #, ?, \, the controls and every rune
// strconv.IsPrint refuses. A link (link true) also encodes a space, (, ), <
// and >, so the path never needs <…> and can't end the link or the line: a
// destination written <…> keeps its brackets, since writers replace only what
// is inside them. A frontmatter reference keeps the rest as readable text
// that `--where` matches as written; YAML quoting is RewriteValues' job.
// Invalid UTF-8 is encoded byte by byte. A writer adds the suffix as written.
func spell(p string, link bool) string {
	var b strings.Builder
	for i := 0; i < len(p); {
		r, size := utf8.DecodeRuneInString(p[i:])
		unsafe := r == '%' || r == '#' || r == '?' || r == '\\' || !strconv.IsPrint(r) ||
			link && (r == ' ' || r == '(' || r == ')' || r == '<' || r == '>') // IsPrint allows the ASCII space
		if r == utf8.RuneError && size == 1 || unsafe {
			for _, c := range []byte(p[i : i+size]) {
				fmt.Fprintf(&b, "%%%02X", c)
			}
		} else {
			b.WriteString(p[i : i+size])
		}
		i += size
	}
	return b.String()
}

// RelativeLink is the root-absolute path p (no anchor: it never cuts '#') as
// written from the entry at fromPath: slash-separated, and always starting
// with "./" or "../", so it reads as relative and never as a scheme
// ("./C:/x.md"). It isn't spelled: pass it to spell to write a link.
func RelativeLink(fromPath, p string) string {
	from, to := splitSlash(path.Dir(fromPath)), splitSlash(p)
	i := 0
	for i < len(from) && i < len(to) && from[i] == to[i] {
		i++
	}
	parts := make([]string, 0, len(from)-i+len(to)-i)
	for range from[i:] {
		parts = append(parts, "..")
	}
	joined := strings.Join(append(parts, to[i:]...), "/")
	switch {
	case joined == "":
		return "." // p is fromPath's folder (degenerate for a file)
	case joined == ".." || strings.HasPrefix(joined, "../"):
		return joined
	}
	return "./" + joined
}

// Ref is a frontmatter value that names an entry (see ref).
type Ref struct {
	Key   string // the frontmatter key
	Value string // the value as the string view reads it
	Dest
}

// FrontmatterRefs returns every frontmatter value of the entry that names an
// entry, in the typed Map's key order (file order), then item order; outside
// ones included. Empty items are left out (FieldList). Existence is the
// caller's: idx.Resolve(r.Path).
func (e *Entry) FrontmatterRefs() []Ref {
	var out []Ref
	for _, p := range e.typed {
		for _, v := range e.FieldList(p.Key) {
			if d, ok := ref(e.Path, v); ok {
				out = append(out, Ref{Key: p.Key, Value: v, Dest: d})
			}
		}
	}
	return out
}

// ref reads v, a frontmatter value of the entry at from, as a reference: one
// whose path (before any '#') ends in ".md", and which isn't external (a
// scheme, or //, /\ or \\ as a link would be). That suffix is the whole
// heuristic, and what keeps ordinary metadata (`title: Some Note`) from being
// read as a path. FrontmatterRefs and move's rewrite share it.
func ref(from, v string) (Dest, bool) {
	if p, _, _ := strings.Cut(v, "#"); !strings.HasSuffix(p, ".md") || parse.IsExternal(v) {
		return Dest{}, false
	}
	return ResolveDest(from, v), true
}
