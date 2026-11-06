package parse

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// RewriteValues returns content with frontmatter values respelled, item by
// item, and how many it respelled. It calls rewrite for every scalar value and
// list item of every top-level occurrence of every key (a key written twice
// is asked about in each; rewrite may be called more than once for a value
// and must give the same answer each time), and writes each new value it
// returns in place of the old, where yaml.v3 says it is; every other byte
// stays as it is. A new value keeps the old token's quote character when it
// fits, and is otherwise quoted only when it needs it, the way SetField writes
// (so `/x: y.md` is written quoted). A header with nothing to rewrite is
// returned as it is.
//
// Like SetField, it refuses with an *EditError, returning nothing to write: a
// header that doesn't read or that wiki can't edit safely, a key written twice
// that has a rewrite in any occurrence, and a result that wouldn't read back
// with exactly the planned change (every other key keeping its value). A value
// it can't respell in place (written with escapes, or over several lines) is
// left as it is, and so refused by that last check.
func RewriteValues(content string, rewrite func(key, value string) (string, bool)) (out string, refs int, err error) {
	h, d, unreadable, err := scanDecoded(content)
	switch {
	case err != nil:
		return "", 0, notRead(err)
	case unreadable != nil:
		return "", 0, notRead(unreadable)
	}
	// Ask about every occurrence of every key first: a header with nothing to
	// rewrite is returned as it is, whatever else is wrong with it.
	want := map[string][]string{} // a rewritten key's items, as they must read after
	count := map[string]int{}     // a key's occurrences
	var planned []int             // the fields with a rewrite
	for i, f := range h.fields {
		count[f.key]++
		var items []string
		rewritten := false
		for _, v := range f.values() {
			if nv, ok := rewrite(f.key, v); ok && nv != v {
				v, rewritten = nv, true
				refs++
			}
			items = append(items, v)
		}
		if rewritten {
			planned = append(planned, i)
			want[f.key] = items
		}
	}
	if refs == 0 {
		return content, 0, nil
	}
	if err := editable(d, unreadable); err != nil {
		return "", 0, err
	}
	for _, i := range planned {
		if key := h.fields[i].key; count[key] > 1 {
			var at []int
			for _, f := range h.fields {
				if f.key == key {
					at = append(at, f.line+1)
				}
			}
			return "", 0, &EditError{Reason: fmt.Sprintf("has key %q twice (lines %d and %d); fix the file first (wiki check reports it)", key, at[0], at[1])}
		}
	}
	lines := strings.Split(content, "\n")
	cols := newColumns(lines)
	var edits []respelling
	for _, i := range planned {
		key := h.fields[i].key
		for _, n := range scalars(d.keys[i].vNode) { // fields and keys are one to one
			if nv, ok := rewrite(key, n.Value); ok && nv != n.Value {
				if e, found := respellAt(cols, n, nv); found {
					edits = append(edits, e)
				}
			}
		}
	}
	apply(lines, edits)
	out = strings.Join(lines, "\n")
	now, err := readBack(d, out, slices.Collect(maps.Keys(want))...)
	if err != nil {
		return "", 0, err
	}
	got := map[string]any{}
	for _, k := range now.keys {
		got[k.key] = k.str
	}
	for k, items := range want {
		if !slices.Equal(stringsOf(got[k]), items) {
			return "", 0, &EditError{Reason: fmt.Sprintf("would not rewrite %s as planned (it would read %q); nothing written", k, stringsOf(got[k]))}
		}
	}
	return out, refs, nil
}

// scalars are the scalar nodes a value is written as: the value itself, or
// each item of a list. An alias is written elsewhere, so it has none here.
func scalars(v *yaml.Node) []*yaml.Node {
	switch v.Kind {
	case yaml.ScalarNode:
		return []*yaml.Node{v}
	case yaml.SequenceNode:
		var out []*yaml.Node
		for _, item := range v.Content {
			if item.Kind == yaml.ScalarNode {
				out = append(out, item)
			}
		}
		return out
	}
	return nil
}

// respelling replaces the bytes [start, end) of one line (0-based) with text.
type respelling struct {
	line, start, end int
	text             string
}

// respellAt finds a scalar node in the source (cols), past any anchor or tag (yaml.v3's
// column points at those), and gives the respelling that writes v in its place.
// It finds none when the source there isn't the value as written: quote, value,
// quote, or the bare value (an escape, or a value over several lines).
func respellAt(cols *columns, n *yaml.Node, v string) (respelling, bool) {
	if n.Line < 1 || n.Line > len(cols.lines) {
		return respelling{}, false
	}
	line := cols.lines[n.Line-1]
	start := cols.offset(n.Line-1, n.Column)
	// A tag or anchor runs to the next space; no byte of a multi-byte rune is one.
	for start < len(line) && (line[start] == '&' || line[start] == '!') {
		for start < len(line) && line[start] != ' ' && line[start] != '\t' {
			start++
		}
		for start < len(line) && (line[start] == ' ' || line[start] == '\t') {
			start++
		}
	}
	q := ""
	switch {
	case n.Style&yaml.DoubleQuotedStyle != 0:
		q = `"`
	case n.Style&yaml.SingleQuotedStyle != 0:
		q = "'"
	}
	written := q + n.Value + q
	if !strings.HasPrefix(line[start:], written) {
		return respelling{}, false
	}
	return respelling{n.Line - 1, start, start + len(written), token(v, q)}, true
}

// token writes a new value in place of one quoted with q ("" when bare): in
// the same quote character when it fits, else quoted only when it needs it.
func token(v, q string) string {
	switch {
	case q != "" && fitsQuote(v, q[0]):
		return q + v + q
	case needsQuote(v):
		return strconv.Quote(v)
	}
	return v
}

// apply writes the respellings into lines, each changed line built once, left
// to right, so many respellings on one long line cost its length, not that
// times their number. Offsets are into the lines as they were.
func apply(lines []string, edits []respelling) {
	slices.SortFunc(edits, func(a, b respelling) int { return cmp.Or(cmp.Compare(a.line, b.line), cmp.Compare(a.start, b.start)) })
	for from := 0; from < len(edits); {
		to := from
		for to < len(edits) && edits[to].line == edits[from].line {
			to++
		}
		line := lines[edits[from].line]
		var b strings.Builder
		at := 0
		for _, e := range edits[from:to] {
			b.WriteString(line[at:e.start])
			b.WriteString(e.text)
			at = e.end
		}
		b.WriteString(line[at:])
		lines[edits[from].line] = b.String()
		from = to
	}
}
