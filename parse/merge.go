package parse

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

// ErrRefused is why MergeBlocks (and the writes built on it) refused a set of
// operations and wrote nothing: an anchor that isn't the base's, operations out
// of order, a text that isn't UTF-8, or more than the limits allow.
var ErrRefused = errors.New("refused")

// The limits of one merge: a page over MaxBlocks blocks is not merged, and a
// save holds at most MaxOps operations.
const (
	MaxBlocks = 20000
	MaxOps    = 1000
)

func refuse(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrRefused, fmt.Sprintf(format, a...))
}

// MergeBlocks merges an app's operations into the page body on disk, three
// ways, by block. base is the block hashes of the version the operations were
// made against; ops are its changes (replace, insert, delete, in key order);
// theirs is the body on disk now, which may have changed since.
//
// The result is written onto theirs, so every byte on disk that the
// operations didn't change stays. Where both sides changed one block, theirs
// stays and the operation comes back as a conflict:
//
//	mine           theirs                         result
//	changed X      left X                         mine
//	left X         changed, deleted, split X      theirs
//	changed X      changed X the same way         once
//	changed X      changed X otherwise, split or  conflict both-changed
//	               joined it
//	changed X      deleted X                      conflict deleted-outside
//	deleted X      changed X                      conflict changed-outside
//	deleted X      deleted X, or left it          deleted
//	changed X      moved X, unchanged             applied at its new place
//	inserted       inserted after X               both: mine next to X;
//	after X                                       a block both inserted, once
//	inserted       deleted X                      before the block that
//	after X                                       followed X; a conflict
//	                                              deleted-outside if it went too
//	inserted       replaced X                     after theirs' replacement
//	after X
//
// Gaps: an inserted block keeps the gap given at the start of its text, or
// takes the page's gap there; but an item right after an item, or a row
// right after a row, continues that list or table with the gap it already
// uses (one line break for a row, and for a tight list), and the page's gap
// then follows the inserted run. A deleted run between two blocks that stay
// takes the narrower of the gaps around it with it, so the wider one separates
// them now (at the start of the page, the gap after it; at the end, the gap
// before it). And a gap new text meets is never narrower than the two kinds
// there need: a blank line wherever an item or a row meets another kind, so
// new text never fuses with a neighbour. When no block of base is left on disk (rewritten),
// nothing is applied: every operation is a conflict both-changed.
//
// applied lists the operations whose change is in body. err wraps ErrRefused
// when the operations themselves are invalid, and then nothing is merged.
func MergeBlocks(base []string, ops []Op, theirs string) (body string, applied []Op, conflicts []Conflict, rewritten bool, err error) {
	tb := Blocks(theirs)
	if err := checkOps(base, ops, len(tb)); err != nil {
		return "", nil, nil, false, err
	}
	pairs := lcsPairs(base, hashes(tb))
	if len(base) > 0 && len(pairs) == 0 && len(ops) > 0 {
		for _, o := range ops {
			at, _ := o.key()
			conflicts = append(conflicts, Conflict{At: at, Case: "both-changed", Theirs: []Block{}})
		}
		return theirs, nil, conflicts, true, nil
	}
	m := &merger{body: theirs, tb: tb, base: base}
	m.place(pairs)
	work := slices.Clone(ops) // dedupe trims inserts: never the caller's
	m.dedupe(work)
	same := m.sameChanges(work)
	for i, o := range work {
		if same[i] {
			applied = append(applied, ops[i]) // the disk made this change too
			continue
		}
		ok, c := m.apply(o)
		switch {
		case c != nil:
			conflicts = append(conflicts, *c)
		case ok:
			applied = append(applied, ops[i])
		}
	}
	body, err = m.write()
	if err != nil {
		return "", nil, nil, false, err
	}
	if n := len(Blocks(body)); n > MaxBlocks {
		return "", nil, nil, false, refuse("the merged page would have %d blocks, more than %d", n, MaxBlocks)
	}
	return body, applied, conflicts, false, nil
}

// checkOps refuses operations that can't be anchored to base.
func checkOps(base []string, ops []Op, theirBlocks int) error {
	switch {
	case len(ops) > MaxOps:
		return refuse("%d operations, more than %d", len(ops), MaxOps)
	case len(base) > MaxBlocks || theirBlocks > MaxBlocks:
		return refuse("the page has more than %d blocks", MaxBlocks)
	}
	lastAt, lastRank := -2, 1
	for i, o := range ops {
		at, rank := o.key()
		switch o.Op {
		case "replace", "delete":
			if at < 0 || at >= len(base) {
				return refuse("operation %d: block %d is out of range", i, at)
			}
			if o.Hash != base[at] {
				return refuse("operation %d: block %d is not %s in the base", i, at, o.Hash)
			}
		case "insert":
			if at < -1 || at >= len(base) {
				return refuse("operation %d: block %d is out of range", i, at)
			}
			if (at == -1 && o.Hash != "") || (at >= 0 && o.Hash != base[at]) {
				return refuse("operation %d: block %d is not %s in the base", i, at, o.Hash)
			}
		default:
			return refuse("operation %d: %q is not replace, insert or delete", i, o.Op)
		}
		if at < lastAt || (at == lastAt && rank < lastRank) || (at == lastAt && rank == 0 && lastRank == 0) {
			return refuse("operation %d: out of order, or a second change to block %d", i, at)
		}
		lastAt, lastRank = at, rank
		if o.Op != "delete" {
			text := opText(o)
			if !utf8.ValidString(text) {
				return refuse("operation %d: the text is not valid UTF-8", i)
			}
			if strings.TrimSpace(text) == "" {
				return refuse("operation %d: a %s needs text (to remove a block, delete it)", i, o.Op)
			}
		}
	}
	return nil
}

// opText is the Markdown an operation brings: its Text, or its Blocks joined
// by blank lines.
func opText(o Op) string {
	if o.Blocks == nil {
		return o.Text
	}
	parts := make([]string, len(o.Blocks))
	for i, b := range o.Blocks {
		parts[i] = b.Text
	}
	return strings.Join(parts, "\n\n")
}

// splitGap cuts the line breaks at the start of an insert's text (the gap it
// asks for before it) from the text, and trims its trailing ones.
func splitGap(text string) (gap, rest string) {
	rest = strings.TrimLeft(text, "\r\n")
	return text[:len(text)-len(rest)], strings.TrimRight(rest, "\r\n")
}

// where says what became of a base block on disk.
type where struct {
	kept  bool // still there, unchanged (in place or moved): at tb[j]
	j     int
	hunk  *hunk // otherwise: the run of changes it is part of
	moved bool
}

// hunk is a run between blocks base and disk share: base[f:t] became tb[jf:jt].
type hunk struct{ f, t, jf, jt int }

type blockEdit struct {
	start, end int
	text       string
	seq        int
}

type merger struct {
	body  string
	tb    []Block
	base  []string
	at    []where // per base block
	gone  []bool  // per disk block: deleted by an operation
	added map[string]int
	ins   map[int][]string // per disk block: what goes after it, in order
	front []string         // what goes before the first block
	repl  map[int]string   // per disk block: its replacement
	edits []blockEdit
}

// place maps each base block to the disk.
func (m *merger) place(pairs [][2]int) {
	m.at = make([]where, len(m.base))
	m.gone = make([]bool, len(m.tb))
	m.ins, m.repl = map[int][]string{}, map[int]string{}
	var hunks []*hunk
	pi, pj := 0, 0
	for _, p := range append(pairs, [2]int{len(m.base), len(m.tb)}) {
		if p[0] > pi || p[1] > pj {
			hunks = append(hunks, &hunk{pi, p[0], pj, p[1]})
		}
		if p[0] < len(m.base) {
			m.at[p[0]] = where{kept: true, j: p[1]}
		}
		pi, pj = p[0]+1, p[1]+1
	}
	// A block deleted from one place and inserted unchanged at another, the
	// only one of its content each way, moved: it is still there.
	del, ins := map[string][]int{}, map[string][]int{}
	m.added = map[string]int{}
	for _, h := range hunks {
		for i := h.f; i < h.t; i++ {
			m.at[i] = where{hunk: h}
			if h.jf == h.jt {
				del[m.base[i]] = append(del[m.base[i]], i)
			}
		}
		if h.f == h.t {
			for j := h.jf; j < h.jt; j++ {
				ins[m.tb[j].Hash] = append(ins[m.tb[j].Hash], j)
				m.added[m.tb[j].Hash]++
			}
		}
	}
	for h, d := range del {
		if len(d) == 1 && len(ins[h]) == 1 {
			m.at[d[0]] = where{kept: true, j: ins[h][0], moved: true}
		}
	}
	// A run of k blocks the disk replaced by k others is k changes, block by
	// block, as DiffBlocks reports them: each base block is judged against its
	// own replacement. A split or a join stays one change of the run.
	for _, h := range hunks {
		if k := h.t - h.f; k > 1 && k == h.jt-h.jf {
			for x := 0; x < k; x++ {
				m.at[h.f+x] = where{hunk: &hunk{h.f + x, h.f + x + 1, h.jf + x, h.jf + x + 1}}
			}
		}
	}
}

// dedupe drops, from the blocks the operations insert, those the disk inserted
// too: the same change made twice is kept once. A block of base content is
// never dropped.
func (m *merger) dedupe(ops []Op) {
	inBase := map[string]bool{}
	for _, h := range m.base {
		inBase[h] = true
	}
	for i, o := range ops {
		if o.Op != "insert" {
			continue
		}
		gap, text := splitGap(opText(o))
		bs := Blocks(text)
		keep := bs[:0:0]
		for _, b := range bs {
			if c := m.added[b.Hash]; c > 0 && !inBase[b.Hash] {
				m.added[b.Hash] = c - 1
				continue
			}
			keep = append(keep, b)
		}
		if len(keep) == len(bs) {
			continue
		}
		var sb strings.Builder
		for k, b := range keep {
			if k > 0 {
				sb.WriteString(text[keep[k-1].End:b.Start]) // the gap the text had
			}
			sb.WriteString(b.Text)
		}
		if sb.Len() == 0 {
			ops[i].Text, ops[i].Blocks = "", []Block{} // nothing left to insert
			continue
		}
		ops[i].Text, ops[i].Blocks = gap+sb.String(), nil
	}
}

// sameChanges finds the operations that make a change the disk made too: for
// a run of base blocks the disk changed, operations changing every block of
// the run (and inserting between them) into the same blocks, in order.
func (m *merger) sameChanges(ops []Op) map[int]bool {
	same := map[int]bool{}
	seen := map[*hunk]bool{}
	var hunks []*hunk
	for _, w := range m.at {
		if w.hunk != nil && !seen[w.hunk] {
			seen[w.hunk] = true
			hunks = append(hunks, w.hunk)
		}
	}
	for _, h := range hunks {
		if h.f == h.t {
			continue
		}
		var group []int
		var mine []string
		covered := 0
		for i, o := range ops {
			at, _ := o.key()
			switch {
			case (o.Op == "replace" || o.Op == "delete") && at >= h.f && at < h.t:
				covered++
			case o.Op == "insert" && at >= h.f && at < h.t-1:
			default:
				continue
			}
			group = append(group, i)
			if o.Op != "delete" {
				_, text := splitGap(opText(o))
				mine = append(mine, hashes(Blocks(text))...)
			}
		}
		if covered == h.t-h.f && slices.Equal(mine, hashes(m.tb[h.jf:h.jt])) {
			for _, i := range group {
				same[i] = true
			}
		}
	}
	return same
}

// apply applies one operation, or says why it can't be.
func (m *merger) apply(o Op) (bool, *Conflict) {
	if o.Op == "insert" {
		return m.insert(o)
	}
	w := m.at[o.At]
	if w.kept {
		if o.Op == "delete" {
			m.gone[w.j] = true
		} else {
			_, text := splitGap(opText(o))
			m.repl[w.j] = text
		}
		return true, nil
	}
	h := w.hunk
	theirs := slices.Clone(m.tb[h.jf:h.jt])
	if theirs == nil {
		theirs = []Block{}
	}
	var also []int
	for i := h.f; i < h.t; i++ {
		if i != o.At {
			also = append(also, i)
		}
	}
	switch {
	case o.Op == "delete" && len(theirs) == 0:
		return true, nil // deleted on both sides
	case o.Op == "delete":
		return false, &Conflict{At: o.At, Case: "changed-outside", Theirs: theirs, Also: also}
	case len(theirs) == 0:
		return false, &Conflict{At: o.At, Case: "deleted-outside", Theirs: theirs}
	}
	_, text := splitGap(opText(o))
	if h.t-h.f == 1 && text == m.body[theirs[0].Start:theirs[len(theirs)-1].End] {
		return true, nil // changed the same way on both sides
	}
	return false, &Conflict{At: o.At, Case: "both-changed", Theirs: theirs, Also: also}
}

// insert places an insert's text on disk, by where its anchor went.
func (m *merger) insert(o Op) (bool, *Conflict) {
	text := opText(o)
	if o.Blocks != nil && len(o.Blocks) == 0 {
		return true, nil // all of it inserted on disk already
	}
	if o.After == -1 {
		m.front = append(m.front, text)
		return true, nil
	}
	w := m.at[o.After]
	switch {
	case w.kept:
		m.ins[w.j] = append(m.ins[w.j], text)
	case w.hunk.jt > w.hunk.jf:
		m.ins[w.hunk.jt-1] = append(m.ins[w.hunk.jt-1], text) // after theirs' replacement
	default:
		// The anchor was deleted on disk: before the block that followed it.
		next := o.After + 1
		switch {
		case next == len(m.base) && len(m.tb) > 0:
			m.ins[len(m.tb)-1] = append(m.ins[len(m.tb)-1], text)
		case next == len(m.base):
			m.front = append(m.front, text)
		case m.at[next].kept:
			m.before(m.at[next].j, text)
		case m.at[next].hunk.jt > m.at[next].hunk.jf:
			m.before(m.at[next].hunk.jf, text)
		default:
			return false, &Conflict{At: o.After, Case: "deleted-outside", Theirs: []Block{}}
		}
	}
	return true, nil
}

// before puts text before disk block j: after the block before it, or at the
// front.
func (m *merger) before(j int, text string) {
	if j == 0 {
		m.front = append(m.front, text)
		return
	}
	m.ins[j-1] = append(m.ins[j-1], text)
}

// gapAfter is the gap to put between disk block j and what is inserted after
// it: the page's own gap after j, or one fit for the two kinds at the end.
func (m *merger) gapAfter(j int, next string) string {
	if j+1 < len(m.tb) {
		return m.body[m.tb[j].End:m.tb[j+1].Start]
	}
	return defaultGap(m.tb[j].Kind, next)
}

// defaultGap separates two blocks with no page gap to copy: a line break
// between two items or two rows of a table, a blank line otherwise.
func defaultGap(a, b string) string {
	tight := func(k string) string {
		switch k {
		case "item":
			return "item"
		case "row", "table-head":
			return "row"
		}
		return k
	}
	if ta := tight(a); (ta == "item" || ta == "row") && ta == tight(b) && b != "table-head" {
		return "\n"
	}
	return "\n\n"
}

// continues is "item" when a block of kind b after one of kind a continues a
// list, "row" when it continues a table, and "" otherwise.
func continues(a, b string) string {
	switch {
	case a == "item" && b == "item":
		return "item"
	case (a == "row" || a == "table-head") && b == "row":
		return "row"
	}
	return ""
}

// sameList reports two items of one list: the same bullet (-, * or +), or
// both ordered with the same delimiter (. or )).
func sameList(a, b Block) bool {
	return a.Kind == "item" && b.Kind == "item" && listMarker(a.Text) == listMarker(b.Text)
}

// listMarker is what tells an item's list apart: its bullet, or the
// delimiter after its number.
func listMarker(text string) string {
	t := strings.TrimLeft(text, " \t>")
	if t == "" {
		return ""
	}
	if strings.ContainsRune("-*+", rune(t[0])) {
		return t[:1]
	}
	if d := strings.TrimLeft(t, "0123456789"); d != t && d != "" && (d[0] == '.' || d[0] == ')') {
		return d[:1]
	}
	return ""
}

// kindsOf is the kinds of the first and last blocks of text, where it follows
// a block of kind before. Rows are the one kind that depends on where they
// are: after a row or a table's head, lines that all hold a pipe are rows,
// though alone they read as a paragraph.
func kindsOf(text, before string) (first, last string) {
	if before == "row" || before == "table-head" {
		rows := text != ""
		for _, l := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
			rows = rows && strings.Contains(l, "|")
		}
		if rows {
			return "row", "row"
		}
	}
	return firstKind(text), lastKind(text)
}

// firstKind is the kind of the first block of text.
func firstKind(text string) string {
	if bs := Blocks(text); len(bs) > 0 {
		return bs[0].Kind
	}
	return ""
}

func lastKind(text string) string {
	if bs := Blocks(text); len(bs) > 0 {
		return bs[len(bs)-1].Kind
	}
	return ""
}

// write builds the merged body: the disk's body with every edit spliced in.
// Edits never overlap by construction; if two ever did, it is an error and
// nothing is written, rather than an edit dropped.
func (m *merger) write() (string, error) {
	seq := 0
	add := func(start, end int, text string) {
		m.edits = append(m.edits, blockEdit{start, end, text, seq})
		seq++
	}
	// Deleted blocks go in runs, each with the gap before it (after it, for a
	// run at the start), so two runs never claim one gap.
	for s := 0; s < len(m.tb); s++ {
		if !m.gone[s] {
			continue
		}
		e := s
		for e+1 < len(m.tb) && m.gone[e+1] {
			e++
		}
		before, after := s > 0, e+1 < len(m.tb)
		if before && after {
			// Between two blocks that stay: the run takes the narrower gap
			// with it, and the wider one is what separates them now (a blank
			// line from a paragraph, a line break between two items).
			before = len(m.body[m.tb[s-1].End:m.tb[s].Start]) <= len(m.body[m.tb[e].End:m.tb[e+1].Start])
		}
		switch {
		case before:
			add(m.tb[s-1].End, m.tb[e].End, "")
		case after:
			add(m.tb[s].Start, m.tb[e+1].Start, "")
		default:
			add(m.tb[s].Start, m.tb[e].End, "")
		}
		s = e
	}
	// Each insert goes after the last block before it that stays (-1: before
	// every block that stays).
	ins := map[int][]string{}
	for j := range m.tb {
		if len(m.ins[j]) == 0 {
			continue
		}
		a := j
		for a >= 0 && m.gone[a] {
			a--
		}
		ins[a] = append(ins[a], m.ins[j]...)
	}
	prevKept := func(j int) int {
		for j--; j >= 0 && m.gone[j]; j-- {
		}
		return j
	}
	nextKept := func(j int) int {
		for j++; j < len(m.tb) && m.gone[j]; j++ {
		}
		if j < len(m.tb) {
			return j
		}
		return -1
	}
	// The kinds at the edges of a block that stays, once replaced or followed
	// by inserts.
	var replKindsFirst func(j int) string
	firstOf := func(j int) string {
		if _, ok := m.repl[j]; ok {
			return replKindsFirst(j)
		}
		return m.tb[j].Kind
	}
	var lastOf func(j int) string
	replKinds := func(j int) (string, string) {
		before := ""
		if p := prevKept(j); p >= 0 {
			before = lastOf(p)
		}
		return kindsOf(m.repl[j], before)
	}
	lastOf = func(j int) string {
		k := m.tb[j].Kind
		if _, ok := m.repl[j]; ok {
			_, k = replKinds(j)
		}
		for _, t := range ins[j] {
			_, x := splitGap(t)
			_, k = kindsOf(x, k)
		}
		return k
	}
	replKindsFirst = func(j int) string {
		f, _ := replKinds(j)
		return f
	}
	changed := func(j int) bool {
		_, r := m.repl[j]
		return r || len(ins[j]) > 0
	}
	gapAfter := func(j int) string {
		if j+1 < len(m.tb) {
			return m.body[m.tb[j].End:m.tb[j+1].Start]
		}
		return ""
	}
	gapBefore := func(j int) string {
		if j > 0 {
			return m.body[m.tb[j-1].End:m.tb[j].Start]
		}
		return ""
	}
	// widen is what to add to a gap so it is never narrower than the one the
	// two kinds meeting there need: new text never fuses with a neighbour (a
	// paragraph after an item would otherwise continue it).
	nl := "\n"
	if strings.Contains(m.body, "\r\n") {
		nl = "\r\n"
	}
	widen := func(gap, need string) string {
		if d := strings.Count(need, "\n") - strings.Count(gap, "\n"); d > 0 {
			return strings.Repeat(nl, d)
		}
		return ""
	}

	// What goes before every block that stays.
	front := append(slices.Clone(m.front), ins[-1]...)
	if len(front) > 0 {
		first := nextKept(-1)
		var sb strings.Builder
		for i, t := range front {
			_, t = splitGap(t)
			if i > 0 {
				_, p := splitGap(front[i-1])
				sb.WriteString(defaultGap(lastKind(p), firstKind(t)))
			}
			sb.WriteString(t)
		}
		text := sb.String()
		switch {
		case first >= 0:
			add(m.tb[first].Start, m.tb[first].Start, text+defaultGap(lastKind(text), firstOf(first)))
		case len(m.tb) > 0:
			add(m.tb[len(m.tb)-1].End, m.tb[len(m.tb)-1].End, "\n\n"+text)
		default:
			add(len(m.body), len(m.body), text+"\n")
		}
	}
	// Each gap that new text meets is checked once: by the text on its left,
	// or, when that is unchanged, by the replacement on its right.
	for j := range m.tb {
		if m.gone[j] {
			continue
		}
		prevKind := m.tb[j].Kind
		if r, ok := m.repl[j]; ok {
			rf, rl := replKinds(j)
			if p := prevKept(j); p >= 0 && !changed(p) {
				r = widen(gapBefore(j), defaultGap(lastOf(p), rf)) + r
			}
			if k := nextKept(j); k >= 0 && len(ins[j]) == 0 {
				r += widen(gapAfter(j), defaultGap(rl, firstOf(k)))
			}
			add(m.tb[j].Start, m.tb[j].End, r)
			prevKind = rl
		}
		if len(ins[j]) == 0 {
			continue
		}
		// An item after an item, or a row after a row, continues that list or
		// table, with the gap it already uses: the one after the anchor when
		// the next block is an item of the same list, else the one before it
		// when the block before is, else one line break. The page's gap after
		// the anchor then comes after the inserted run.
		listGap := nl
		switch {
		case j+1 < len(m.tb) && !m.gone[j+1] && sameList(m.tb[j], m.tb[j+1]):
			listGap = gapAfter(j)
		case j > 0 && !m.gone[j-1] && sameList(m.tb[j-1], m.tb[j]):
			listGap = gapBefore(j)
		}
		var sb strings.Builder
		for i, t := range ins[j] {
			gap, t := splitGap(t)
			tf, tl := kindsOf(t, prevKind)
			if gap == "" {
				switch family := continues(prevKind, tf); {
				case family == "item":
					gap = listGap
				case family == "row":
					gap = nl
				case i == 0:
					gap = m.gapAfter(j, tf)
				default:
					gap = defaultGap(prevKind, tf)
				}
			}
			sb.WriteString(gap + widen(gap, defaultGap(prevKind, tf)) + t)
			prevKind = tl
		}
		if k := nextKept(j); k >= 0 {
			sb.WriteString(widen(gapAfter(j), defaultGap(prevKind, firstOf(k))))
		}
		add(m.tb[j].End, m.tb[j].End, sb.String())
	}
	sort.SliceStable(m.edits, func(a, b int) bool {
		x, y := m.edits[a], m.edits[b]
		if x.start != y.start {
			return x.start < y.start
		}
		if (x.start == x.end) != (y.start == y.end) {
			return x.start == x.end // an insertion before what starts there
		}
		return x.seq < y.seq
	})
	var out strings.Builder
	pos := 0
	for _, e := range m.edits {
		if e.start < pos {
			return "", fmt.Errorf("merging blocks: two edits overlap at byte %d", e.start)
		}
		out.WriteString(m.body[pos:e.start])
		out.WriteString(e.text)
		pos = e.end
	}
	out.WriteString(m.body[pos:])
	return out.String(), nil
}
