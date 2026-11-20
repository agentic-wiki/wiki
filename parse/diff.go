package parse

import (
	"encoding/json"
)

// Op is one change to a page's blocks, anchored to a version of the page by a
// block's index in that version's list and its hash. The same vocabulary flows
// both ways: an app sends replace, insert and delete to MergeBlocks, and
// DiffBlocks reports those plus move.
//
//	replace  At, Hash: the block; Text (or Blocks): what takes its place,
//	         one block or several
//	insert   After, Hash: the block it goes after (-1 and "" for the very
//	         start); Text (or Blocks): one block or several. Line breaks at
//	         the start of Text are the gap before it; without them it gets the
//	         gap the page has there
//	delete   At, Hash: the block
//	move     At, Hash: the block, unchanged; After: the block it now follows
//
// Operations are in key order: (At, 0) for a replace or delete, (After, 1) for
// an insert, so at one block its replace or delete comes first, then what is
// inserted after it. A block takes at most one replace or delete; several
// inserts after one block keep their order.
type Op struct {
	Op     string  // replace | insert | delete | move
	At     int     // replace, delete, move
	After  int     // insert, move
	Hash   string  // the anchoring block's hash ("" for an insert at the very start)
	Text   string  // replace, insert: Markdown, as an app sends it
	Blocks []Block // replace, insert: already split, as DiffBlocks reports it
}

// MarshalJSON writes only the fields the kind of operation has.
func (o Op) MarshalJSON() ([]byte, error) {
	m := map[string]any{"op": o.Op, "hash": o.Hash}
	switch o.Op {
	case "insert":
		m["after"] = o.After
	case "move":
		m["at"], m["after"] = o.At, o.After
	default:
		m["at"] = o.At
	}
	if o.Op == "replace" || o.Op == "insert" {
		if o.Blocks != nil {
			m["blocks"] = o.Blocks
		} else {
			m["text"] = o.Text
		}
	}
	return json.Marshal(m)
}

// UnmarshalJSON reads an operation as an app sends it: `{"op": "insert",
// "after": 12, "hash": "…", "text": "…"}`.
func (o *Op) UnmarshalJSON(b []byte) error {
	var w struct {
		Op     string  `json:"op"`
		At     int     `json:"at"`
		After  int     `json:"after"`
		Hash   string  `json:"hash"`
		Text   string  `json:"text"`
		Blocks []Block `json:"blocks"`
	}
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	*o = Op{Op: w.Op, At: w.At, After: w.After, Hash: w.Hash, Text: w.Text, Blocks: w.Blocks}
	return nil
}

// key is the operation's place in the order: its block, then 0 for a replace
// or delete and 1 for an insert.
func (o Op) key() (int, int) {
	if o.Op == "insert" {
		return o.After, 1
	}
	return o.At, 0
}

// Conflict is an operation MergeBlocks didn't apply, because the page on disk
// changed the same block: At is the block's index in the base, and Theirs the
// disk's blocks in its place (none when it was deleted there).
//
//	both-changed     both sides changed the block (or the disk split or
//	                 joined it): Theirs is what took its place, and Also the
//	                 other base blocks joined into it
//	deleted-outside  the block was changed here and deleted on disk
//	changed-outside  the block was deleted here and changed on disk
type Conflict struct {
	At     int     `json:"at"`
	Case   string  `json:"case"`
	Theirs []Block `json:"theirs"`
	Also   []int   `json:"also,omitempty"`
}

// DiffBlocks lists the operations that turn the blocks of a into those of b,
// anchored to a: replaces, inserts and deletes between the blocks the two
// share, and a move for a block deleted once and inserted once elsewhere,
// unchanged. Only the hashes of a are read, so a can be a version known only
// by its hashes.
//
// Blocks are matched by hash with Myers' diff in linear space, which pairs
// identical blocks in order: between two blocks of one content, position
// decides.
func DiffBlocks(a, b []Block) []Op {
	ah, bh := hashes(a), hashes(b)
	pairs := lcsPairs(ah, bh)
	var ops []Op
	anchor := func(i int) string {
		if i < 0 {
			return ""
		}
		return ah[i]
	}
	pi, pj := 0, 0
	for _, p := range append(pairs, [2]int{len(a), len(b)}) {
		f, t, jf, jt := pi, p[0], pj, p[1]
		k, m := t-f, jt-jf
		switch {
		case k == 0 && m > 0:
			ops = append(ops, Op{Op: "insert", After: f - 1, Hash: anchor(f - 1), Blocks: b[jf:jt]})
		case k > 0:
			// Paired one to one, the last of a taking what is left of b; what
			// is left of a goes.
			for x := 0; x < k; x++ {
				switch {
				case x < m && (x < k-1 || m == k):
					ops = append(ops, Op{Op: "replace", At: f + x, Hash: ah[f+x], Blocks: b[jf+x : jf+x+1]})
				case x < m:
					ops = append(ops, Op{Op: "replace", At: f + x, Hash: ah[f+x], Blocks: b[jf+x : jt]})
				default:
					ops = append(ops, Op{Op: "delete", At: f + x, Hash: ah[f+x]})
				}
			}
		}
		pi, pj = p[0]+1, p[1]+1
	}
	return withMoves(ops)
}

// withMoves turns a delete and a one-block insert of the same block, each of
// that content the only one, into a move.
func withMoves(ops []Op) []Op {
	del, ins := map[string][]int{}, map[string][]int{}
	for i, o := range ops {
		switch {
		case o.Op == "delete":
			del[o.Hash] = append(del[o.Hash], i)
		case o.Op == "insert" && len(o.Blocks) == 1:
			ins[o.Blocks[0].Hash] = append(ins[o.Blocks[0].Hash], i)
		}
	}
	drop := map[int]bool{}
	for h, d := range del {
		if len(d) == 1 && len(ins[h]) == 1 {
			ops[d[0]] = Op{Op: "move", At: ops[d[0]].At, Hash: h, After: ops[ins[h][0]].After}
			drop[ins[h][0]] = true
		}
	}
	out := ops[:0]
	for i, o := range ops {
		if !drop[i] {
			out = append(out, o)
		}
	}
	return out
}

func hashes(bs []Block) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Hash
	}
	return out
}

// lcsPairs matches a and b: the index pairs of a longest common subsequence,
// in order. Myers' diff in linear space (the bisection of diff-match-patch):
// O((n+m)·d) time for d differences, with no n×m table.
func lcsPairs(a, b []string) [][2]int {
	in := make(map[string]bool, len(a))
	for _, s := range a {
		in[s] = true
	}
	shared := false
	for _, s := range b {
		if in[s] {
			shared = true
			break
		}
	}
	var out [][2]int
	if shared {
		lcsInto(a, b, 0, 0, &out)
	}
	return out
}

func lcsInto(a, b []string, ao, bo int, out *[][2]int) {
	p := 0
	for p < len(a) && p < len(b) && a[p] == b[p] {
		*out = append(*out, [2]int{ao + p, bo + p})
		p++
	}
	a, b, ao, bo = a[p:], b[p:], ao+p, bo+p
	s := 0
	for s < len(a) && s < len(b) && a[len(a)-1-s] == b[len(b)-1-s] {
		s++
	}
	ma, mb := a[:len(a)-s], b[:len(b)-s]
	if len(ma) > 0 && len(mb) > 0 {
		if x, y, ok := bisect(ma, mb); ok {
			lcsInto(ma[:x], mb[:y], ao, bo, out)
			lcsInto(ma[x:], mb[y:], ao+x, bo+y, out)
		}
	}
	for i := 0; i < s; i++ {
		*out = append(*out, [2]int{ao + len(ma) + i, bo + len(mb) + i})
	}
}

// bisect finds the middle of an edit path between a and b, where the forward
// and backward searches meet, splitting the problem in two. ok is false when
// they never meet: a and b share nothing.
func bisect(a, b []string) (x, y int, ok bool) {
	n, m := len(a), len(b)
	maxD := (n + m + 1) / 2
	vOff, vLen := maxD, 2*maxD+2
	v1, v2 := make([]int, vLen), make([]int, vLen)
	for i := range v1 {
		v1[i], v2[i] = -1, -1
	}
	v1[vOff+1], v2[vOff+1] = 0, 0
	delta := n - m
	front := delta%2 != 0
	k1start, k1end, k2start, k2end := 0, 0, 0, 0
	for d := 0; d < maxD; d++ {
		for k1 := -d + k1start; k1 <= d-k1end; k1 += 2 {
			k1Off := vOff + k1
			var x1 int
			if k1 == -d || (k1 != d && v1[k1Off-1] < v1[k1Off+1]) {
				x1 = v1[k1Off+1]
			} else {
				x1 = v1[k1Off-1] + 1
			}
			y1 := x1 - k1
			for x1 < n && y1 < m && a[x1] == b[y1] {
				x1++
				y1++
			}
			v1[k1Off] = x1
			switch {
			case x1 > n:
				k1end += 2
			case y1 > m:
				k1start += 2
			case front:
				if k2Off := vOff + delta - k1; k2Off >= 0 && k2Off < vLen && v2[k2Off] != -1 {
					if x1 >= n-v2[k2Off] {
						return x1, y1, true
					}
				}
			}
		}
		for k2 := -d + k2start; k2 <= d-k2end; k2 += 2 {
			k2Off := vOff + k2
			var x2 int
			if k2 == -d || (k2 != d && v2[k2Off-1] < v2[k2Off+1]) {
				x2 = v2[k2Off+1]
			} else {
				x2 = v2[k2Off-1] + 1
			}
			y2 := x2 - k2
			for x2 < n && y2 < m && a[n-x2-1] == b[m-y2-1] {
				x2++
				y2++
			}
			v2[k2Off] = x2
			switch {
			case x2 > n:
				k2end += 2
			case y2 > m:
				k2start += 2
			case !front:
				if k1Off := vOff + delta - k2; k1Off >= 0 && k1Off < vLen && v1[k1Off] != -1 {
					x1 := v1[k1Off]
					y1 := vOff + x1 - k1Off
					if x1 >= n-x2 {
						return x1, y1, true
					}
				}
			}
		}
	}
	return 0, 0, false
}
