// Package parse provides targeted extractors for wiki markdown: frontmatter,
// links, tasks, and headings. Intentionally minimal — no full markdown AST.
package parse

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Link is a root-absolute internal markdown link [Text](/target.md).
type Link struct {
	Text   string `json:"text"`
	Target string `json:"target"` // anchor stripped, e.g. /finance/income/x.md
	Line   int    `json:"line"`
}

// Checkbox is a GFM checkbox item.
type Checkbox struct {
	Done bool   `json:"done"`
	Text string `json:"text"`
	Line int    `json:"line"`
}

// Heading is an ATX heading.
type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	Line  int    `json:"line"`
}

// Table is a parsed GFM table: the header cells and the data rows (each row
// padded or truncated to the header's width).
type Table struct {
	Header []string   `json:"header"`
	Rows   [][]string `json:"rows"`
}

// ReadFrontmatter returns the header's two views:
//   - strs, the string view: a scalar is its text as YAML reads it (a plain
//     scalar keeps its spelling, so `~` is "~" and `k:` is ""), a list is a
//     []string of its items' texts (a list or map item is its Flow), and a
//     map is its Flow;
//   - typed, the typed view: rule T at every plain scalar (see typedBare), lists
//     as []any, and maps as Map, the top level included.
//
// A key written twice keeps its last value. body is what follows the closing
// fence. err is non-nil when the header can't be read: not valid YAML, not a
// mapping, or over a limit (header larger than 1 MiB, more than 10,000 values
// or 1 MiB of text through aliases). Both views are then empty. An
// unterminated header gives err, and body is all of content.
func ReadFrontmatter(content string) (strs map[string]any, typed Map, body string, err error) {
	strs, typed = map[string]any{}, Map{}
	_, headerText, body, err := split(content)
	if err != nil {
		return strs, typed, body, err
	}
	d, err := decode(headerText)
	if err != nil {
		return strs, typed, body, err
	}
	for _, k := range d.keys {
		if k.scalarKey {
			strs[k.key] = k.str
		}
	}
	return strs, d.values(), body, nil
}

// Body returns content without its header: what follows the closing fence, or
// all of content when there is none or it is unterminated. It decodes nothing.
func Body(content string) string {
	_, _, body, _ := split(content)
	return body
}

// Frontmatter is ReadFrontmatter's string view and body: every value is a
// string or a []string. A header that can't be read gives no keys.
func Frontmatter(content string) (fm map[string]any, body string) {
	fm, _, body, _ = ReadFrontmatter(content)
	return fm, body
}

// jsonNumberRe is JSON's number grammar. Every token it matches is also a YAML
// 1.2 core int or float; YAML spellings outside it (+1, 007, .5, 0x1F, .inf)
// stay strings.
var jsonNumberRe = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

// typedScalar types one raw token (trimmed, quotes kept), so tok is "" only
// when nothing follows the colon. The first match wins:
//
//	key: "" or key: ''         the string "" (any quoted token is its string)
//	key:, key: ~, key: null    nil, as in YAML
//	key: true, key: False      a bool (yes/no/on/off stay strings, as in YAML 1.2)
//	key: 10, key: 1.10         a json.Number, byte for byte as written
//	anything else              the token itself; dates are never reformatted
//
// An empty placeholder is null rather than "", so a DuckDB column that is
// still waiting for its numbers keeps its type instead of becoming text.
func typedScalar(tok string) any {
	if quoted(tok) {
		return Unquote(tok)
	}
	return typedBare(tok)
}

// typedBare is rule T for a token that isn't quoted: nil, a bool, a json.Number
// or the token itself.
func typedBare(tok string) any {
	switch tok {
	case "", "~", "null", "Null", "NULL": // "" is nothing written (`key:`, an empty cell), never a quoted ""
		return nil
	case "true", "True", "TRUE":
		return true
	case "false", "False", "FALSE":
		return false
	}
	if jsonNumberRe.MatchString(tok) {
		return json.Number(tok)
	}
	return tok
}

// Unquote trims surrounding whitespace and a matching pair of quotes.
func Unquote(s string) string {
	s = strings.TrimSpace(s)
	if quoted(s) {
		s = s[1 : len(s)-1]
	}
	return strings.TrimSpace(s)
}

// quoted reports whether the trimmed token s is wrapped in '…' or "…".
func quoted(s string) bool {
	return len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0]
}

// String returns frontmatter key as a string ("" if absent or not a string).
func String(fm map[string]any, key string) string {
	s, _ := fm[key].(string)
	return s
}

// Strings returns frontmatter key as a string slice. A lone scalar
// (e.g. `tags: foo`) is treated as a single-element list.
func Strings(fm map[string]any, key string) []string { return stringsOf(fm[key]) }

// stringsOf is Strings for one string-view value: a list is its items, a
// scalar a one-element list, and "" (or anything else) nil.
func stringsOf(v any) []string {
	switch x := v.(type) {
	case []string:
		return x
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	}
	return nil
}

var (
	linkRe       = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
	checkboxRe   = regexp.MustCompile(`^\s*[-*+] \[([ xX])\]\s+(.*)$`)
	headingRe    = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	inlineCodeRe = regexp.MustCompile("`[^`]*`")
	// tableDelimRe matches a GFM table's delimiter row: dash cells with optional
	// alignment colons (e.g. `---|:--:|---`), with optional outer pipes.
	tableDelimRe = regexp.MustCompile(`^\s*\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?\s*$`)
)

// linkMatch is one markdown link found by matchLinks: its text, its target as
// written (title stripped, anchor kept, angle brackets removed), and where that
// target sits on its line, so a rewrite can replace exactly those bytes.
type linkMatch struct {
	text, target string
	line         int  // 0-based line index within the body
	start, end   int  // byte span of target on the original line
	angled       bool // the destination was written <…>, the markdown way to allow spaces
}

// matchLinks returns body's lines and every markdown link on them, outside fenced
// and inline code. It is the one link scanner: Links reads it, MapInternalLinks
// rewrites through it, so what counts as a link never differs between the two.
func matchLinks(body string) (lines []string, ms []linkMatch) {
	lines = strings.Split(body, "\n")
	for i, line := range maskedLines(body) {
		// A link written inside an inline code span is documentation, not an edge,
		// so blank spans on this line before matching. They are blanked to spaces
		// rather than removed so byte offsets still address the original line.
		// (Tasks/Headings keep their inline code as text, so this lives here, not
		// in maskedLines.)
		line = inlineCodeRe.ReplaceAllStringFunc(line, func(s string) string { return strings.Repeat(" ", len(s)) })
		for _, m := range linkRe.FindAllStringSubmatchIndex(line, -1) {
			raw := line[m[4]:m[5]]
			start := m[4] + len(raw) - len(strings.TrimLeft(raw, " \t"))
			dest := strings.TrimSpace(raw)
			lm := linkMatch{text: lines[i][m[2]:m[3]], line: i}
			if strings.HasPrefix(dest, "<") {
				// Angle-bracketed destination: take everything up to the closing '>',
				// ignoring a title after it.
				start++
				dest = dest[1:]
				if gt := strings.IndexByte(dest, '>'); gt >= 0 {
					dest = dest[:gt]
				} // unterminated: keep the rest, dropping the '<' rather than leaking it
				lm.angled = true
			} else if sp := strings.IndexAny(dest, " \t"); sp >= 0 {
				dest = dest[:sp] // bare destination: a space begins the title
			}
			lm.target, lm.start, lm.end = dest, start, start+len(dest)
			ms = append(ms, lm)
		}
	}
	return lines, ms
}

// LinkSet holds a body's internal markdown links, classified by target form.
// Targets are as written (title stripped, anchor kept); resolving them to
// canonical root-absolute form is the index's job.
type LinkSet struct {
	Absolute    []Link // root-absolute, e.g. /finance/income.md
	Relative    []Link // relative or bare, e.g. ../x.md, ./x.md, sibling.md
	External    []Link // carries a URL scheme, e.g. https://, mailto:
	SelfAnchors []Link // pure #anchor (intra-document): not a cross-entry edge, but check validates it against the entry's own headings
}

// linkKind classifies a link target into the LinkSet buckets.
type linkKind int

const (
	kindNone linkKind = iota // empty target: not a link
	kindSelfAnchor
	kindExternal
	kindAbsolute
	kindRelative
)

func classify(t string) linkKind {
	switch {
	case t == "":
		return kindNone
	case strings.HasPrefix(t, "#"):
		return kindSelfAnchor
	case hasURLScheme(t):
		return kindExternal
	case strings.HasPrefix(t, "/"):
		return kindAbsolute
	default:
		return kindRelative
	}
}

// Links classifies every markdown link in body (outside code) by target form.
// Empty targets are omitted; a pure #anchor lands in SelfAnchors (an intra-document
// reference, not a cross-entry edge). Both Absolute and Relative are valid internal
// links per OKF; External is the leftover bucket, returned for completeness.
func Links(body string) LinkSet {
	var set LinkSet
	_, ms := matchLinks(body)
	for _, m := range ms {
		l := Link{Text: m.text, Target: m.target, Line: m.line + 1}
		switch classify(m.target) {
		case kindSelfAnchor:
			set.SelfAnchors = append(set.SelfAnchors, l) // same-page anchor; check validates it against own headings
		case kindExternal:
			set.External = append(set.External, l)
		case kindAbsolute:
			set.Absolute = append(set.Absolute, l)
		case kindRelative:
			set.Relative = append(set.Relative, l)
		}
	}
	return set
}

// MapInternalLinks returns body with the target of every internal (absolute or
// relative) markdown link outside code replaced by fn(target). fn receives the
// target as Links reports it (title stripped, anchor kept) and returns it
// unchanged to leave the link alone. Only the target's bytes change: link text,
// titles, URLs, #anchors and code are left exactly as written. A replacement
// containing whitespace is wrapped in <…> when the original was bare, so the link
// stays a link.
func MapInternalLinks(body string, fn func(target string) string) string {
	lines, ms := matchLinks(body)
	// Replace right to left within a line so earlier offsets stay valid.
	for i := len(ms) - 1; i >= 0; i-- {
		m := ms[i]
		if k := classify(m.target); k != kindAbsolute && k != kindRelative {
			continue
		}
		repl := fn(m.target)
		if repl == m.target {
			continue
		}
		if !m.angled && strings.ContainsAny(repl, " \t") {
			repl = "<" + repl + ">"
		}
		lines[m.line] = lines[m.line][:m.start] + repl + lines[m.line][m.end:]
	}
	return strings.Join(lines, "\n")
}

// hasURLScheme reports whether target begins with an RFC 3986 scheme
// (ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ) ":"), e.g. http: or mailto:.
func hasURLScheme(target string) bool {
	colon := strings.IndexByte(target, ':')
	if colon <= 0 {
		return false
	}
	if slash := strings.IndexByte(target, '/'); slash >= 0 && slash < colon {
		return false // the ':' sits inside a path/anchor, not a scheme
	}
	for i := 0; i < colon; i++ {
		c := target[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		case i > 0 && (c >= '0' && c <= '9' || c == '+' || c == '-' || c == '.'):
		default:
			return false
		}
	}
	return true
}

// Checkboxes returns checkbox items, ignoring fenced code.
func Checkboxes(body string) []Checkbox {
	var checkboxes []Checkbox
	for i, line := range maskedLines(body) {
		if m := checkboxRe.FindStringSubmatch(line); m != nil {
			checkboxes = append(checkboxes, Checkbox{
				Done: m[1] == "x" || m[1] == "X",
				Text: strings.TrimSpace(m[2]),
				Line: i + 1,
			})
		}
	}
	return checkboxes
}

// Headings returns ATX headings, ignoring fenced code.
func Headings(body string) []Heading {
	var hs []Heading
	for i, line := range maskedLines(body) {
		if m := headingRe.FindStringSubmatch(line); m != nil {
			hs = append(hs, Heading{Level: len(m[1]), Text: strings.TrimSpace(m[2]), Line: i + 1})
		}
	}
	return hs
}

// maskedLines returns body split into lines with fenced code blocks blanked
// (line numbers preserved) so block-scanning parsers skip code. Inline code
// spans are left intact: task and heading text keeps them verbatim, and the one
// caller that must ignore inline code (link scanning) blanks it itself.
func maskedLines(body string) []string {
	lines := strings.Split(body, "\n")
	inFence := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			inFence = !inFence
			lines[i] = ""
			continue
		}
		if inFence {
			lines[i] = ""
		}
	}
	return lines
}

// Tables returns the GitHub-flavored markdown tables in body, in document order,
// skipping fenced code. A table is a header row, a delimiter row (e.g. `---|:--:`),
// then the contiguous data rows. Each data row is padded or truncated to the
// header's width so the result is rectangular (GFM ignores surplus cells).
func Tables(body string) []Table {
	lines := strings.Split(body, "\n")
	var tables []Table
	inFence := false
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.Contains(lines[i], "|") {
			continue
		}
		// A header line is only a table if a delimiter row follows it.
		if i+1 >= len(lines) || !tableDelimRe.MatchString(lines[i+1]) {
			continue
		}
		header := splitRow(lines[i])
		var rows [][]string
		j := i + 2
		for ; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "" || !strings.Contains(lines[j], "|") {
				break
			}
			rows = append(rows, fitRow(splitRow(lines[j]), len(header)))
		}
		tables = append(tables, Table{Header: header, Rows: rows})
		i = j - 1
	}
	return tables
}

// splitRow splits a markdown table row into trimmed cells, dropping the optional
// outer pipes. It honors `\|` as a literal pipe; a `|` inside an inline-code span
// is not special-cased (uncommon in dataset tables).
func splitRow(line string) []string {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "|")
	s = strings.TrimSuffix(s, "|")
	var cells []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		switch {
		case s[i] == '\\' && i+1 < len(s) && s[i+1] == '|':
			cur.WriteByte('|')
			i++
		case s[i] == '|':
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(s[i])
		}
	}
	return append(cells, strings.TrimSpace(cur.String()))
}

// fitRow pads (with empty cells) or truncates row to exactly n columns so a
// table's rows stay rectangular against its header.
func fitRow(row []string, n int) []string {
	if len(row) < n {
		return append(row, make([]string, n-len(row))...)
	}
	return row[:n]
}

// TypedRows returns the table's cells typed by rule T, the way ReadFrontmatter
// types a value: an empty cell, ~ or null is nil, true/false a bool, a JSON
// number a json.Number as written, and anything else a string. A cell is a
// quoted string, without its quotes, only when the same quote starts and ends
// it and appears nowhere else in it, so `"Yes," she said, "no"` stays as written.
func (t Table) TypedRows() [][]any {
	rows := make([][]any, len(t.Rows))
	for i, r := range t.Rows {
		rows[i] = make([]any, len(r))
		for j, c := range r {
			if quoted(c) && !strings.ContainsRune(c[1:len(c)-1], rune(c[0])) {
				rows[i][j] = c[1 : len(c)-1]
			} else {
				rows[i][j] = typedBare(c)
			}
		}
	}
	return rows
}
