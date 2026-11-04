package parse

import (
	"errors"
	"strings"
)

// Header is a frontmatter header as it sits in the file. Line numbers are
// 0-based indexes into strings.Split(content, "\n").
type Header struct {
	Open, Close int     // the two --- lines; both -1 when the file has no header
	Fields      []Field // top-level keys, in file order; a key written twice is here twice
}

// Field is one top-level key and the lines it owns.
type Field struct {
	Key       string
	Line, End int // the key's line and the last line it owns (End >= Line)

	kind       kind
	raw        string // the text after the colon, comment removed
	comment    string // the key line's comment with the spacing before it, e.g. " # ten"
	str, typed any    // the value as Frontmatter and TypedFrontmatter read it
}

// Values returns this one occurrence of the key the way Strings returns a
// key from Frontmatter: a list gives its items, a scalar a one-element list,
// and an empty value nil. A key written twice has a Field, and Values, for each.
func (f Field) Values() []string { return stringsOf(f.str) }

// kind is what a key holds, from how it is written.
type kind int

const (
	kindScalar kind = iota // a value after the colon, or nothing and no owned content (a null placeholder)
	kindList               // [ after the colon, or nothing and a block item first
	kindText               // | or > after the colon
	kindObject             // { after the colon, or nothing and other content first
)

func (k kind) String() string {
	return [...]string{"Scalar", "List", "Text", "Object"}[k]
}

var errUnterminated = errors.New("line 1: unterminated frontmatter (missing closing --- fence)")

// Scan reads a frontmatter header: a first line that is exactly ---, up to the
// next line that is exactly --- (a trailing \r is allowed on both). A file
// whose first line isn't --- has no header. A header with no closing fence is
// an error.
//
// A comment line starts with # after any indent, or, at column 0, has a space
// or tab followed by # before its first colon (see commentLine).
//
// A key line is a line at column 0 that is not blank, not a block item (`-` or
// `- …`) and not a comment line. It is split at the first colon; the key is
// the trimmed text before it, and the value is the text after it, with its
// comment removed. A line with no colon or an empty key is no key.
//
// A key owns its own line, then every following line that is indented, or a
// block item at any indent, or blank or a comment line when an owned line
// follows. The first other line, or the closing fence, ends the span. Lines
// before the first key belong to no key.
//
// What the key holds comes from the text after its colon; when that is empty,
// from the first owned line that isn't blank or a comment line:
//   - a scalar: a single-line value (rule T in TypedFrontmatter);
//   - a list: a flow list [a, "b c"], or block items (`- a`); empty items are
//     dropped, and an owned line that is not an item continues the item before
//     it (joined with \n);
//   - text, after | or >: every owned line as written, except a comment line
//     at column 0, de-indented by the smallest indent;
//   - an object (a nested map, or a flow map {…}): a multi-line string, the
//     value after the colon and the owned lines without comment lines,
//     de-indented and joined with \n;
//   - a scalar or flow list that continues on owned lines: a multi-line string
//     the same way, with inline comments removed too.
func Scan(content string) (Header, error) {
	h, _, _, err := scan(content)
	return h, err
}

// scan is Scan, plus the header's text (the opening fence and the lines up to
// the closing one, which yaml.v3 validates) and the body after the closing fence.
func scan(content string) (h Header, header, body string, err error) {
	h = Header{Open: -1, Close: -1}
	first, _, _ := strings.Cut(content, "\n")
	if strings.TrimSuffix(first, "\r") != "---" {
		return h, "", content, nil
	}
	var lines []string // the header lines between the fences
	for start := len(first) + 1; start <= len(content); {
		line, _, newline := strings.Cut(content[start:], "\n")
		end := start + len(line)
		if newline {
			end++
		}
		if strings.TrimSuffix(line, "\r") == "---" {
			h.Open, h.Close = 0, len(lines)+1
			h.Fields = fields(lines)
			return h, content[:start], content[end:], nil
		}
		if !newline {
			break
		}
		lines = append(lines, strings.TrimSuffix(line, "\r"))
		start = end
	}
	return Header{Open: -1, Close: -1}, "", content, errUnterminated
}

// fields finds the keys in the header lines; line i is file line i+1.
func fields(lines []string) []Field {
	var fs []Field
	for i := 0; i < len(lines); i++ {
		key, val, comment, ok := keyLine(lines[i])
		if !ok {
			continue
		}
		end := spanEnd(lines, i)
		f := Field{Key: key, Line: i + 1, End: end + 1, raw: val, comment: comment}
		f.kind, f.str, f.typed = value(val, lines[i+1:end+1])
		fs = append(fs, f)
		i = end
	}
	return fs
}

// keyLine splits a key line into its key, its value (comment removed) and its
// comment, with the spacing before the #.
func keyLine(line string) (key, val, comment string, ok bool) {
	if blank(line) || line[0] == ' ' || line[0] == '\t' || isBlockItem(line) || commentLine(line) {
		return "", "", "", false
	}
	key, after, ok := strings.Cut(line, ":")
	key = strings.TrimSpace(key)
	if !ok || key == "" {
		return "", "", "", false
	}
	val = stripComment(strings.TrimSpace(after))
	if val != "" {
		after = after[len(after)-len(strings.TrimLeft(after, " \t"))+len(val):]
	}
	return key, val, strings.TrimRight(after, " \t"), true
}

// spanEnd returns the last line the key on line i owns.
func spanEnd(lines []string, i int) int {
	end := i
	for j := i + 1; j < len(lines); j++ {
		l := lines[j]
		switch {
		case blank(l):
			// owned only if an owned line follows
		case l[0] == ' ' || l[0] == '\t' || isBlockItem(l):
			end = j
		case commentLine(l):
			// like a blank line: owned only if an owned line follows
		default:
			return end
		}
	}
	return end
}

// How multiline treats # comments in owned lines.
type comments int

const (
	keepComments     comments = iota // text (| or >): every line as written, except a column-0 comment line
	dropCommentLines                 // an object: comment lines left out, the rest as written
	stripComments                    // a plain value: comment lines left out, inline comments removed
)

// value reads what a key holds, from the text after its colon and its owned lines.
func value(val string, owned []string) (k kind, str, typed any) {
	if strings.HasPrefix(val, "|") || strings.HasPrefix(val, ">") {
		s := multiline("", owned, keepComments)
		return kindText, s, s
	}
	if val == "" {
		first := firstContent(owned)
		switch {
		case first == "":
			return kindScalar, "", nil
		case isBlockItem(first):
			strs, typed := listViews(blockItems(owned))
			return kindList, strs, typed
		}
		s := multiline("", owned, dropCommentLines)
		return kindObject, s, s
	}
	switch val[0] {
	case '{':
		s := multiline(val, owned, dropCommentLines)
		return kindObject, s, s
	case '[':
		k = kindList
	}
	if firstContent(owned) != "" { // a scalar or flow list that continues on owned lines
		s := multiline(val, owned, stripComments)
		return k, s, s
	}
	if k == kindList {
		strs, typed := listViews(list(val))
		return k, strs, typed
	}
	return k, Unquote(val), typedScalar(val)
}

// firstContent returns the first owned line that isn't blank or a comment, or "".
func firstContent(owned []string) string {
	for _, l := range owned {
		if !blank(l) && !commentLine(l) {
			return l
		}
	}
	return ""
}

// blockItems returns a block list's raw item tokens, comments removed. A line
// that is not an item continues the item before it.
func blockItems(owned []string) []string {
	var tokens []string
	for _, l := range owned {
		t := strings.TrimSpace(l)
		switch {
		case t == "" || commentLine(l):
		case isBlockItem(t):
			tokens = append(tokens, stripComment(strings.TrimSpace(t[1:]))) // a bare - gives an empty token
		case tokens[len(tokens)-1] == "": // the list's first content line is an item, so there is one
			tokens[len(tokens)-1] = stripComment(t)
		default:
			tokens[len(tokens)-1] += "\n" + stripComment(t)
		}
	}
	return tokens
}

// multiline joins first (when not empty) and the owned lines, de-indented by
// their smallest indent, with \n. c says what happens to # comments; in text,
// a column-0 # line is not part of the text. Blank lines inside are kept and
// trailing ones dropped.
func multiline(first string, owned []string, c comments) string {
	var lines []string
	indent := -1
	for _, l := range owned {
		if commentLine(l) && (c != keepComments || l[0] != ' ' && l[0] != '\t') {
			continue
		}
		if c == stripComments {
			l = stripComment(l)
		}
		lines = append(lines, l)
		if !blank(l) {
			if n := len(l) - len(strings.TrimLeft(l, " \t")); indent < 0 || n < indent {
				indent = n
			}
		}
	}
	var out []string
	if first != "" {
		out = append(out, first)
	}
	for _, l := range lines {
		if blank(l) {
			out = append(out, "")
		} else {
			out = append(out, l[indent:])
		}
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

// blank reports whether a line holds only spaces and tabs.
func blank(line string) bool { return strings.TrimSpace(line) == "" }

// commentLine reports whether a line is only a comment: it starts with # after
// any indent, or, at column 0 and not a block item, a space or tab followed by
// # comes before its first colon (`a #b: x` is a comment, as in YAML). The
// scanner uses this one test for key lines, spans and values alike.
func commentLine(line string) bool {
	if strings.HasPrefix(strings.TrimSpace(line), "#") {
		return true
	}
	if line == "" || line[0] == ' ' || line[0] == '\t' || isBlockItem(line) {
		return false
	}
	before, _, _ := strings.Cut(line, ":")
	return strings.Contains(before, " #") || strings.Contains(before, "\t#")
}
