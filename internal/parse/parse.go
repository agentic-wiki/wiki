// Package parse provides targeted extractors for wiki markdown: frontmatter,
// links, tasks, and headings. Intentionally minimal — no full markdown AST.
package parse

import (
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

// Frontmatter splits a leading `---` block from the body and parses the small
// YAML subset we use: scalars and string lists. Values are string or []string.
func Frontmatter(content string) (fm map[string]any, body string) {
	fm = map[string]any{}
	rest, ok := strings.CutPrefix(content, "---\n")
	if !ok {
		if rest, ok = strings.CutPrefix(content, "---\r\n"); !ok {
			return fm, content // no frontmatter (handles LF and CRLF openers)
		}
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return fm, content
	}
	block := rest[:end]
	after := rest[end+1:] // starts at the closing "---"
	if nl := strings.IndexByte(after, '\n'); nl >= 0 {
		body = after[nl+1:]
	}
	parseYAMLSubset(block, fm)
	return fm, body
}

// parseYAMLSubset fills fm from a frontmatter block. Supported: `key: value`
// scalars (quoted or bare, internal spaces kept), flow lists `[a, "b c"]`, block
// lists (`- item`), block scalars (`|`/`>`), and full-line and inline `#`
// comments. Out of scope and skipped without corrupting other keys: nested maps,
// anchors, multi-document streams. Numbers and booleans are kept as strings.
func parseYAMLSubset(block string, fm map[string]any) {
	lines := strings.Split(block, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '-' {
			continue // blank or nested; nested is consumed by the scans below
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = stripComment(strings.TrimSpace(val))
		switch {
		case strings.HasPrefix(val, "["):
			fm[key] = List(val)
		case strings.HasPrefix(val, "|") || strings.HasPrefix(val, ">"):
			// block scalar: gather the following more-indented lines
			var blk []string
			for j := i + 1; j < len(lines); j++ {
				raw := strings.TrimRight(lines[j], "\r")
				if raw != "" && raw[0] != ' ' && raw[0] != '\t' {
					break // dedent ends the block
				}
				blk = append(blk, strings.TrimSpace(raw))
				i = j
			}
			fm[key] = strings.TrimSpace(strings.Join(blk, "\n"))
		case val == "":
			var list []string
			for j := i + 1; j < len(lines); j++ {
				t := strings.TrimSpace(strings.TrimRight(lines[j], "\r"))
				if strings.HasPrefix(t, "- ") {
					list = append(list, Unquote(stripComment(t[2:])))
					i = j
					continue
				}
				break
			}
			if list != nil {
				fm[key] = list
			} else {
				fm[key] = ""
			}
		default:
			fm[key] = Unquote(val)
		}
	}
}

// stripComment removes a trailing inline YAML comment — a `#` at the start or
// preceded by whitespace — that is not inside single/double quotes. A `#` glued
// to the preceding character (URLs, anchors) is kept.
func stripComment(s string) string {
	inSingle, inDouble := false, false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
				return strings.TrimRight(s[:i], " \t")
			}
		}
	}
	return s
}

// List parses a bracketed, comma-separated list of quoted or bare tokens,
// e.g. `[a, "b c", d]` into ["a", "b c", "d"]. Empties are dropped.
func List(val string) []string {
	val = strings.TrimSpace(val)
	val = strings.TrimPrefix(val, "[")
	val = strings.TrimSuffix(val, "]")
	var out []string
	for p := range strings.SplitSeq(val, ",") {
		if p = Unquote(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Unquote trims surrounding whitespace and a matching pair of quotes.
func Unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		s = s[1 : len(s)-1]
	}
	return strings.TrimSpace(s)
}

// String returns frontmatter key as a string ("" if absent or not a string).
func String(fm map[string]any, key string) string {
	s, _ := fm[key].(string)
	return s
}

// Strings returns frontmatter key as a string slice. A lone scalar
// (e.g. `tags: foo`) is treated as a single-element list.
func Strings(fm map[string]any, key string) []string {
	switch x := fm[key].(type) {
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

// scanLinks returns every markdown link in body (outside fenced/inline code),
// with the optional title stripped but the anchor kept.
func scanLinks(body string) []Link {
	var out []Link
	for i, line := range maskedLines(body) {
		// A link written inside an inline code span is documentation, not an edge,
		// so blank spans on this line before matching. (Tasks/Headings keep their
		// inline code as text, so this strip lives here, not in maskedLines.)
		line = inlineCodeRe.ReplaceAllString(line, "")
		for _, m := range linkRe.FindAllStringSubmatch(line, -1) {
			target := strings.TrimSpace(m[2])
			if strings.HasPrefix(target, "<") {
				// Angle-bracketed destination, the markdown way to allow spaces:
				// take everything up to the closing '>', ignoring a title after it.
				if gt := strings.IndexByte(target, '>'); gt >= 0 {
					target = target[1:gt]
				} else {
					target = target[1:] // unterminated; drop the '<' rather than leak it
				}
			} else if sp := strings.IndexAny(target, " \t"); sp >= 0 {
				target = target[:sp] // bare destination: a space begins the title
			}
			out = append(out, Link{Text: m[1], Target: target, Line: i + 1})
		}
	}
	return out
}

// LinkSet holds a body's internal markdown links, classified by target form.
// Targets are as written (title stripped, anchor kept); resolving them to
// canonical root-absolute form is the index's job.
type LinkSet struct {
	Absolute []Link // root-absolute, e.g. /finance/income.md
	Relative []Link // relative or bare, e.g. ../x.md, ./x.md, sibling.md
	External []Link // carries a URL scheme, e.g. https://, mailto:
}

// Links classifies every markdown link in body (outside code) by target form.
// Empty targets and pure #anchors are omitted (they point at no entry). Both
// Absolute and Relative are valid internal links per OKF; External is the
// leftover bucket, returned for completeness, no consumer requires it.
func Links(body string) LinkSet {
	var set LinkSet
	for _, l := range scanLinks(body) {
		switch t := l.Target; {
		case t == "" || strings.HasPrefix(t, "#"):
			// intra-document anchor or empty: not a link to another entry
		case hasURLScheme(t):
			set.External = append(set.External, l)
		case strings.HasPrefix(t, "/"):
			set.Absolute = append(set.Absolute, l)
		default:
			set.Relative = append(set.Relative, l)
		}
	}
	return set
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
