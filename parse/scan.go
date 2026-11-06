package parse

import (
	"errors"
	"strings"
	"unicode/utf8"

	"go.yaml.in/yaml/v3"
)

// layout is a frontmatter header as it sits in the file, for writes. Line
// numbers are 0-based indexes into strings.Split(content, "\n").
type layout struct {
	open, close int     // the two --- lines; both -1 when the file has no header
	fields      []field // top-level keys, in file order; a key written twice is here twice
}

// field is one top-level key and the lines it owns, from yaml.v3's reading.
type field struct {
	key       string // the key as YAML reads it ("my key" for `"my key": v`)
	line, end int    // the key's line and the last line it owns (end >= line)

	kind       kind
	block      bool   // a block list (`- a` items), not a flow one
	quote      byte   // the quote of a quoted scalar value, or 0
	comment    string // the key line's comment with the spacing before it, e.g. " # ten"
	str, typed any    // this occurrence's value in the string and typed views
}

// values returns this one occurrence of the key the way Strings returns a key
// from Frontmatter: a list gives its items, a scalar a one-element list, and
// an empty value nil.
func (f field) values() []string { return stringsOf(f.str) }

// kind is what a key holds, which decides how a write spells it.
type kind int

const (
	kindScalar kind = iota // a scalar, or null
	kindList               // a list, flow ([a, b]) or block (- a)
	kindText               // a | or > block
	kindObject             // a map
)

func (k kind) String() string {
	return [...]string{"Scalar", "List", "Text", "Object"}[k]
}

var errUnterminated = errors.New("line 1: unterminated frontmatter (missing closing --- fence)")

// scanDecoded finds a frontmatter header and decodes it once: the header's
// layout (its fences, and each top-level key's lines from yaml.v3's key
// positions), the decode, and why the header can't be read (decode's error,
// with no fields then). A header is a first line that is exactly ---, up to
// the next line that is exactly --- (a trailing \r is allowed on both). A file
// whose first line isn't --- has none, and a header with no closing fence is
// errUnterminated.
func scanDecoded(content string) (h layout, d header, unreadable, err error) {
	lines, headerText, _, err := split(content)
	h = layout{open: -1, close: -1}
	if err != nil || headerText == "" {
		return h, header{}, nil, err
	}
	h.open, h.close = 0, len(lines)+1
	d, unreadable = decode(headerText)
	if unreadable == nil {
		h.fields = yamlFields(d, lines)
	}
	return h, d, unreadable, nil
}

// yamlFields gives each top-level key its lines: from its key's line to the
// line before the next key's, or to the closing fence, less the blank lines
// and column-0 # lines at the end. Such a # line is a comment, except inside
// a quoted scalar (`k: "a` / `# b"`, or an item of a flow list), so trimming
// never goes above the line where the value's last quoted scalar ends
// (quotedEnd). A block scalar's lines are indented, and anywhere else a # at a
// line's start is a comment.
// lines are the header lines between the fences: line i is file line i+1.
func yamlFields(d header, lines []string) []field {
	cols := newColumns(lines)
	fs := make([]field, 0, len(d.keys))
	for i, k := range d.keys {
		start, end := k.line-1, len(lines) // 0-based file lines; the fence is line len(lines)+1
		if i+1 < len(d.keys) {
			end = d.keys[i+1].line - 2
		}
		last := start // the value's last line: a quoted one may end on a line starting with #
		if k.scalarKey {
			last = max(last, quotedEnd(cols, k.vNode))
		}
		for end > last && (blank(lines[end-1]) || strings.HasPrefix(lines[end-1], "#")) {
			end--
		}
		end = max(end, start)
		f := field{key: k.key, line: start, end: end, str: k.str, typed: k.typed}
		if k.scalarKey {
			f.kind, f.block, f.quote = shape(k.vNode)
			f.comment = keyLineComment(lines[start-1], k.kNode, k.vNode)
		}
		fs = append(fs, f)
	}
	return fs
}

// shape is how a value node is written: its kind, whether a list is a block
// list, and the quote of a quoted scalar.
func shape(v *yaml.Node) (k kind, block bool, quote byte) {
	switch {
	case v.Kind == yaml.SequenceNode:
		return kindList, v.Style&yaml.FlowStyle == 0, 0
	case v.Kind == yaml.MappingNode:
		return kindObject, false, 0
	case v.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0:
		return kindText, false, 0
	case v.Style&yaml.DoubleQuotedStyle != 0:
		return kindScalar, false, '"'
	case v.Style&yaml.SingleQuotedStyle != 0:
		return kindScalar, false, '\''
	}
	return kindScalar, false, 0
}

// keyLineComment is the comment that ends a key's line (yaml.v3's line comment
// on the value or the key, which keeps any spaces after it), with the spaces
// or tabs before it as written, or "" when the line doesn't end with it.
func keyLineComment(line string, k, v *yaml.Node) string {
	c := v.LineComment
	if c == "" {
		c = k.LineComment
	}
	line = strings.TrimSuffix(line, "\r")
	if c == "" || !strings.HasSuffix(line, c) {
		return ""
	}
	return line[len(strings.TrimRight(line[:len(line)-len(c)], " \t")):]
}

// split cuts content at its header's fences: the header lines between them
// (a trailing \r removed), the header's text (the opening fence and the lines
// up to the closing one, which yaml.v3 decodes), and the body. A file with no
// header has no lines and is all body; so is one with an unterminated header,
// which is errUnterminated.
func split(content string) (lines []string, headerText, body string, err error) {
	first, _, _ := strings.Cut(content, "\n")
	if strings.TrimSuffix(first, "\r") != "---" {
		return nil, "", content, nil
	}
	for start := len(first) + 1; start <= len(content); {
		line, _, newline := strings.Cut(content[start:], "\n")
		end := start + len(line)
		if newline {
			end++
		}
		if strings.TrimSuffix(line, "\r") == "---" {
			return lines, content[:start], content[end:], nil
		}
		if !newline {
			break
		}
		lines = append(lines, strings.TrimSuffix(line, "\r"))
		start = end
	}
	return nil, "", content, errUnterminated
}

// blank reports whether a line holds only spaces and tabs.
func blank(line string) bool { return strings.TrimSpace(line) == "" }

// quotedEnd is the 0-based file line where the last quoted scalar of a value
// ends (its closing quote), at any depth: the value itself, or an item or key
// of a flow list or map in it. It is found in the source from each opening
// quote, and is 0 when the value holds none. Aliases are not followed: their
// text is elsewhere. cols holds the header lines between the fences: line i is
// file line i+1. Inside double quotes a backslash escapes the next character;
// inside single quotes two single quotes are one.
func quotedEnd(cols *columns, v *yaml.Node) int {
	lines := cols.lines
	end := 0
	for _, child := range v.Content {
		end = max(end, quotedEnd(cols, child))
	}
	if v.Kind != yaml.ScalarNode || v.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) == 0 || v.Line < 2 || v.Line-1 > len(lines) {
		return end
	}
	q := byte('"')
	if v.Style&yaml.SingleQuotedStyle != 0 {
		q = '\''
	}
	i := v.Line - 2
	start := cols.offset(i, v.Column)
	j := start + strings.IndexByte(lines[i][start:], q) + 1 // past the opening quote, after any anchor or tag
	for ; i < len(lines); i, j = i+1, 0 {
		for l := lines[i]; j < len(l); j++ {
			switch {
			case q == '"' && l[j] == '\\':
				j++
			case q == '\'' && l[j] == '\'' && j+1 < len(l) && l[j+1] == '\'':
				j++
			case l[j] == q:
				return max(end, i+1)
			}
		}
	}
	return end
}

// columns turns yaml.v3's columns, which count runes from 1, into byte offsets
// in a set of lines. Each line is converted at most once, and an ASCII line
// not at all, so a long line with many scalars on it costs no more than its
// own length.
type columns struct {
	lines []string
	runes map[int][]int // a non-ASCII line's byte offset of each rune; nil for an ASCII line
}

func newColumns(lines []string) *columns {
	return &columns{lines: lines, runes: map[int][]int{}}
}

// offset is the byte offset in line i of the rune at column col (1-based),
// clamped to the line.
func (c *columns) offset(i, col int) int {
	line := c.lines[i]
	offs, seen := c.runes[i]
	if !seen {
		for b := 0; b < len(line); b++ {
			if line[b] >= utf8.RuneSelf {
				for at := range line {
					offs = append(offs, at)
				}
				break
			}
		}
		c.runes[i] = offs
	}
	n := max(col-1, 0)
	switch {
	case offs == nil:
		return min(n, len(line))
	case n < len(offs):
		return offs[n]
	}
	return len(line)
}
