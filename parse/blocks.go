package parse

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// Block is one unit of a page's body: a heading, a paragraph, a code block, a
// rule, a quote, an HTML block, a list item, a table's head or one of its rows.
// The blocks plus the gaps between them (blank lines and line breaks) are the
// body, byte for byte.
//
// The model decides only the unit of a merge and of a conflict, never the
// bytes, which always come from the spans. So it reads lines, never CommonMark
// structure: deterministic and simple, not a Markdown parser.
type Block struct {
	Kind       string `json:"kind"` // paragraph | heading | code | rule | quote | html | item | table-head | row
	Start, End int    `json:"-"`    // byte offsets in the body: the block's lines, without the line break after its last one
	Text       string `json:"text"` // body[Start:End]
	Hash       string `json:"hash"` // BlockHash(Text): the block's version
}

// BlockHash is a block's version: the hex of the first 16 bytes of the SHA-256
// of its text.
func BlockHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:16])
}

var (
	blankRe  = regexp.MustCompile(`^[ \t]*$`)
	fenceRe  = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	atxRe    = regexp.MustCompile(`^ {0,3}#{1,6}([ \t]|$)`)
	setextRe = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)
	itemRe   = regexp.MustCompile(`^( *)([-*+]|\d{1,9}[.)])([ \t]+|$)`)
	quoteRe  = regexp.MustCompile(`^ {0,3}>`)
	htmlRe   = regexp.MustCompile(`^ {0,3}<(/?[A-Za-z][A-Za-z0-9-]*[\s/>]|!--)`)
	delimRe  = regexp.MustCompile(`^ {0,3}\|?[ \t]*:?-+:?[ \t]*(\|[ \t]*:?-+:?[ \t]*)*\|?[ \t]*$`)
	indentRe = regexp.MustCompile(`^ *`)
)

// isRule reports a thematic break: up to three spaces, then three or more of
// one of - * _, with only spaces and tabs between them. It wins over a list
// item (`- - -` is a rule).
func isRule(l string) bool {
	t := strings.TrimLeft(l, " ")
	if len(l)-len(t) > 3 || t == "" || !strings.ContainsRune("-*_", rune(t[0])) {
		return false
	}
	c, n := t[0], 0
	for i := 0; i < len(t); i++ {
		switch t[i] {
		case c:
			n++
		case ' ', '\t':
		default:
			return false
		}
	}
	return n >= 3
}

// isClosingFence reports a line closing a fence of n characters c, after any
// indentation the opening allowed (up to 3 spaces, or any inside an item).
func isClosingFence(l string, c byte, n int, anyIndent bool) bool {
	t := strings.TrimLeft(l, " \t")
	if !anyIndent && len(l)-len(strings.TrimLeft(l, " ")) > 3 {
		return false
	}
	m := 0
	for m < len(t) && t[m] == c {
		m++
	}
	return m >= n && strings.Trim(t[m:], " \t") == ""
}

func isTableHead(l string, next *string) bool {
	return strings.Contains(l, "|") && next != nil && delimRe.MatchString(*next) &&
		strings.Contains(*next, "-") && (strings.Contains(*next, "|") || strings.HasPrefix(strings.TrimSpace(l), "|"))
}

// startsBlock reports a line that ends a paragraph or an item's continuation by
// starting another kind of block.
func startsBlock(l string) bool {
	return fenceRe.MatchString(l) || atxRe.MatchString(l) || isRule(l) || itemRe.MatchString(l) || quoteRe.MatchString(l)
}

// Blocks splits a page's body (what follows the frontmatter) into blocks:
//
//	code        a fence (``` or ~~~, up to 3 spaces in) to its closing fence or
//	            the end; or a paragraph indented 4 spaces
//	heading     # to ###### and a space, one line; or a paragraph with a
//	            === or --- line under it
//	rule        ---, *** or ___ alone
//	table-head  a line with a pipe and the delimiter row under it, as one block
//	row         each following non-blank line with a pipe
//	item        a list marker (-, *, +, 1., 1)) and a space, at any indent, to
//	            the next item (nested or not) or a blank line; its continuation
//	            lines, and what follows a blank line indented to its content,
//	            stay in it, and a nested item is its own block
//	quote       > to a blank line
//	html        an HTML tag or <!-- to a blank line
//	paragraph   anything else, to a blank line or a line starting another kind
//
// A line's trailing \r belongs to its line break, so a page with CRLF line
// endings splits the same way.
func Blocks(body string) []Block {
	var lines []string
	var offs []int
	for at := 0; at < len(body); {
		nl := strings.IndexByte(body[at:], '\n')
		end := len(body)
		if nl >= 0 {
			end = at + nl
		}
		lines = append(lines, strings.TrimSuffix(body[at:end], "\r"))
		offs = append(offs, at)
		if nl < 0 {
			break
		}
		at = end + 1
	}
	line := func(i int) *string {
		if i < len(lines) {
			return &lines[i]
		}
		return nil
	}
	blank := func(i int) bool { return blankRe.MatchString(lines[i]) }

	var out []Block
	push := func(kind string, a, b int) { // lines [a, b)
		start, end := offs[a], offs[b-1]+len(lines[b-1])
		text := body[start:end]
		out = append(out, Block{Kind: kind, Start: start, End: end, Text: text, Hash: BlockHash(text)})
	}
	for i := 0; i < len(lines); {
		l := lines[i]
		if blank(i) {
			i++
			continue
		}
		j := i + 1
		if f := fenceRe.FindStringSubmatch(l); f != nil {
			for j < len(lines) && !isClosingFence(lines[j], f[1][0], len(f[1]), false) {
				j++
			}
			end := min(j+1, len(lines))
			push("code", i, end)
			i = end
			continue
		}
		if atxRe.MatchString(l) {
			push("heading", i, j)
			i = j
			continue
		}
		if isRule(l) {
			push("rule", i, j)
			i = j
			continue
		}
		if isTableHead(l, line(i+1)) {
			push("table-head", i, i+2)
			i += 2
			for i < len(lines) && !blank(i) && strings.Contains(lines[i], "|") {
				push("row", i, i+1)
				i++
			}
			continue
		}
		if m := itemRe.FindStringSubmatch(l); m != nil {
			col := len(m[1]) + len(m[2]) + max(1, min(len(m[3]), 4))
			for j < len(lines) {
				if blank(j) {
					// What follows blank lines stays in the item when it is
					// indented to its content and isn't an item itself.
					k := j
					for k < len(lines) && blank(k) {
						k++
					}
					if k < len(lines) && len(indentRe.FindString(lines[k])) >= col && !itemRe.MatchString(lines[k]) {
						j = k
						continue
					}
					break
				}
				if itemRe.MatchString(lines[j]) {
					break // the next item, nested or not, is its own block
				}
				ind := len(indentRe.FindString(lines[j]))
				if f := fenceRe.FindStringSubmatch(lines[j][min(ind, col):]); f != nil && ind >= col {
					// A fence inside the item: taken whole.
					j++
					for j < len(lines) && !isClosingFence(lines[j], f[1][0], len(f[1]), true) {
						j++
					}
					j++
					continue
				}
				if ind < col && startsBlock(lines[j]) {
					break
				}
				j++
			}
			end := min(j, len(lines))
			push("item", i, end)
			i = end
			continue
		}
		if quoteRe.MatchString(l) || htmlRe.MatchString(l) {
			kind := "quote"
			if !quoteRe.MatchString(l) {
				kind = "html"
			}
			for j < len(lines) && !blank(j) {
				j++
			}
			push(kind, i, j)
			i = j
			continue
		}
		// A paragraph, or indented code: to a blank line or a line starting
		// another block. A setext underline belongs to it, and makes it a heading.
		kind := "paragraph"
		if strings.HasPrefix(l, "    ") {
			kind = "code"
		}
		for j < len(lines) && !blank(j) {
			if kind == "paragraph" && setextRe.MatchString(lines[j]) {
				kind = "heading"
				j++
				break
			}
			if startsBlock(lines[j]) || isTableHead(lines[j], line(j+1)) {
				break
			}
			j++
		}
		push(kind, i, j)
		i = j
	}
	return out
}
