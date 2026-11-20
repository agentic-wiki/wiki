package parse

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/internal/scaffold"
)

// merged runs mine's change from base (as an app sends it) against theirs.
func merged(t *testing.T, base, mine, theirs string) (string, []string, bool) {
	t.Helper()
	b, ops := opsBetween(base, mine)
	body, applied, conflicts, rewritten, err := MergeBlocks(b, ops, theirs)
	if err != nil {
		t.Fatal(err)
	}
	var cs []string
	for _, c := range conflicts {
		s := fmt.Sprintf("%s@%d theirs=%d", c.Case, c.At, len(c.Theirs))
		if len(c.Also) > 0 {
			s += fmt.Sprintf(" also=%v", c.Also)
		}
		cs = append(cs, s)
	}
	if len(applied)+len(conflicts) != len(ops) && !rewritten {
		t.Errorf("%d applied and %d conflicts for %d operations", len(applied), len(conflicts), len(ops))
	}
	return body, cs, rewritten
}

// The merge table of the editor ADR, a row per case.
func TestMergeBlocksTable(t *testing.T) {
	const base = "A\n\nB\n\nC\n"
	for _, c := range []struct {
		name, mine, theirs, want string
		conflicts                []string
	}{
		{"mine changed X, theirs left it", "A\n\nB2\n\nC\n", "A2\n\nB\n\nC\n", "A2\n\nB2\n\nC\n", nil},
		{"theirs changed X, mine left it", "A1\n\nB\n\nC\n", "A\n\nB3\n\nC\n", "A1\n\nB3\n\nC\n", nil},
		{"theirs deleted X, mine left it", "A1\n\nB\n\nC\n", "A\n\nC\n", "A1\n\nC\n", nil},
		{"theirs split X, mine left it", "A1\n\nB\n\nC\n", "A\n\nB1\n\nB2\n\nC\n", "A1\n\nB1\n\nB2\n\nC\n", nil},
		{"both changed X the same way", "A\n\nBx\n\nC\n", "A\n\nBx\n\nC2\n", "A\n\nBx\n\nC2\n", nil},
		{"both changed X differently", "A\n\nBm\n\nC\n", "A\n\nBt\n\nC\n", "A\n\nBt\n\nC\n", []string{"both-changed@1 theirs=1"}},
		{"mine changed X, theirs split it", "A\n\nBm\n\nC\n", "A\n\nB1\n\nB2\n\nC\n", "A\n\nB1\n\nB2\n\nC\n", []string{"both-changed@1 theirs=2"}},
		{"mine changed X, theirs joined it with a neighbour", "A\n\nBm\n\nC\n", "A\n\nBC\n", "A\n\nBC\n", []string{"both-changed@1 theirs=1 also=[2]"}},
		{"mine changed X, theirs deleted it", "A\n\nBm\n\nC\n", "A\n\nC\n", "A\n\nC\n", []string{"deleted-outside@1 theirs=0"}},
		{"mine deleted X, theirs changed it", "A\n\nC\n", "A\n\nBt\n\nC\n", "A\n\nBt\n\nC\n", []string{"changed-outside@1 theirs=1"}},
		{"both deleted X", "A\n\nC\n", "A\n\nC\n", "A\n\nC\n", nil},
		{"mine deleted X, theirs left it", "A\n\nC\n", "A2\n\nB\n\nC\n", "A2\n\nC\n", nil},
		{"mine changed X, theirs moved it", "A\n\nBm\n\nC\n", "A\n\nC\n\nB\n", "A\n\nC\n\nBm\n", nil},
		{"mine deleted X, theirs moved it", "A\n\nC\n", "A\n\nC\n\nB\n", "A\n\nC\n", nil},
		{"theirs joined X with a neighbour, mine left it", "A1\n\nB\n\nC\n", "A\n\nBC\n", "A1\n\nBC\n", nil},
		{"both inserted after X", "A\n\nM\n\nB\n\nC\n", "A\n\nT\n\nB\n\nC\n", "A\n\nM\n\nT\n\nB\n\nC\n", nil},
		{"both inserted the same block after X", "A\n\nX\n\nB\n\nC\n", "A\n\nX\n\nB\n\nC\n", "A\n\nX\n\nB\n\nC\n", nil},
		{"mine inserted after X, theirs deleted it", "A\n\nB\n\nM\n\nC\n", "A\n\nC\n", "A\n\nM\n\nC\n", nil},
		{"mine inserted after X, theirs deleted it and the next", "A\n\nB\n\nM\n\nC\n", "A\n", "A\n", []string{"deleted-outside@1 theirs=0"}},
		{"mine inserted after X, theirs replaced it", "A\n\nB\n\nM\n\nC\n", "A\n\nBt\n\nC\n", "A\n\nBt\n\nM\n\nC\n", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			body, conflicts, _ := merged(t, base, c.mine, c.theirs)
			if body != c.want || strings.Join(conflicts, "; ") != strings.Join(c.conflicts, "; ") {
				t.Errorf("got %q %v, want %q %v", body, conflicts, c.want, c.conflicts)
			}
		})
	}
}

// An inserted block keeps the gap mine had before it; a deleted block takes
// the gap before it with it, or the one after it at the start of the page.
func TestMergeBlocksGaps(t *testing.T) {
	for _, c := range []struct{ name, base, mine, theirs, want string }{
		{"an item in a tight list", "- a\n- b\n", "- a\n- x\n- b\n", "- a\n- b2\n", "- a\n- x\n- b2\n"},
		{"a row in a table", "| h |\n|---|\n| 1 |\n", "| h |\n|---|\n| 1 |\n| 2 |\n", "| h |\n|---|\n| 1 |\n", "| h |\n|---|\n| 1 |\n| 2 |\n"},
		{"the first block deleted", "A\n\nB\n\nC\n", "B\n\nC\n", "A\n\nB\n\nC2\n", "B\n\nC2\n"},
		{"the last block deleted", "A\n\nB\n\nC\n", "A\n\nB\n", "A2\n\nB\n\nC\n", "A2\n\nB\n"},
		{"inserted at the start", "A\n\nB\n", "Z\n\nA\n\nB\n", "A\n\nB2\n", "Z\n\nA\n\nB2\n"},
		{"inserted into an empty page", "", "Z\n", "", "Z\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if body, conflicts, _ := merged(t, c.base, c.mine, c.theirs); body != c.want || conflicts != nil {
				t.Errorf("got %q %v, want %q", body, conflicts, c.want)
			}
		})
	}
}

// An item inserted right after an item, or a row right after a row, with no
// gap given, continues that list or table with the gap it already uses; the
// page's gap after the anchor moves to after the inserted run. (The editor's
// Enter at the end of a list, and its new row.)
func TestAnInsertContinuesItsListOrTable(t *testing.T) {
	for _, c := range []struct {
		name, page string
		after      int
		texts      []string
		want       string
	}{
		{"an item at the end of a tight list", "- a\n- b\n\nPara\n", 1, []string{"- new"}, "- a\n- b\n- new\n\nPara\n"},
		{"a row at the end of a table", "| h |\n|---|\n| 1 |\n\nPara\n", 1, []string{"| 2 |"}, "| h |\n|---|\n| 1 |\n| 2 |\n\nPara\n"},
		{"two items, one operation each", "- a\n- b\n\nPara\n", 1, []string{"- n1", "- n2"}, "- a\n- b\n- n1\n- n2\n\nPara\n"},
		{"two items in one operation", "- a\n- b\n\nPara\n", 1, []string{"- n1\n- n2"}, "- a\n- b\n- n1\n- n2\n\nPara\n"},
		{"a loose list keeps its blank lines", "- a\n\n- b\n\nPara\n", 1, []string{"- new"}, "- a\n\n- b\n\n- new\n\nPara\n"},
		{"two items in a loose list", "- a\n\n- b\n\nPara\n", 1, []string{"- n1", "- n2"}, "- a\n\n- b\n\n- n1\n\n- n2\n\nPara\n"},
		{"a list of one item", "Intro\n\n- a\n\nPara\n", 1, []string{"- b"}, "Intro\n\n- a\n- b\n\nPara\n"},
		{"the first row of a table", "| h |\n|---|\n\nPara\n", 0, []string{"| 1 |"}, "| h |\n|---|\n| 1 |\n\nPara\n"},
		{"after a loose list's first item", "- a\n\n- b\n", 0, []string{"- new"}, "- a\n\n- new\n\n- b\n"},
		{"after a loose list's first item, the list after a paragraph", "Para\n\n- a\n\n- b\n", 1, []string{"- new"}, "Para\n\n- a\n\n- new\n\n- b\n"},
		{"another list's gap doesn't count", "- a\n\n1. x\n\nPara\n", 1, []string{"2. y"}, "- a\n\n1. x\n2. y\n\nPara\n"},
		{"a paragraph after an item keeps the page's gap", "- a\n- b\n\nPara\n", 1, []string{"New"}, "- a\n- b\n\nNew\n\nPara\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			bs := Blocks(c.page)
			var ops []Op
			for _, text := range c.texts {
				ops = append(ops, Op{Op: "insert", After: c.after, Hash: bs[c.after].Hash, Text: text})
			}
			got, _, conflicts, _, err := MergeBlocks(hashes(bs), ops, c.page)
			if err != nil || len(conflicts) > 0 || got != c.want {
				t.Errorf("got %q %v %v, want %q", got, conflicts, err, c.want)
			}
		})
	}
}

// A replacement keeps the page's gaps unless its window, re-split, would
// change the blocks around it: an item replaced by an item, or a row by a
// row, changes no byte outside it, while new text that would fuse with a
// neighbour (a paragraph after an item, an item that can't interrupt a
// paragraph) gets a blank line on that side. The text comes as an editor
// sends it, with no line breaks around it.
func TestAReplacementKeepsItsGapsUnlessItWouldFuse(t *testing.T) {
	for _, c := range []struct {
		name, page string
		at         int
		text, want string
	}{
		{"an item after a paragraph", "**Acceptance:**\n- one\n- two\n\nAfter\n", 1, "- one, edited",
			"**Acceptance:**\n- one, edited\n- two\n\nAfter\n"},
		{"a row after a row", "| h |\n|---|\n| 1 |\n| 2 |\n\nAfter\n", 1, "| 1, edited |",
			"| h |\n|---|\n| 1, edited |\n| 2 |\n\nAfter\n"},
		// The blank line keeps it out of the item before; the item after
		// can interrupt it, so that gap stays.
		{"an item becomes a paragraph", "- one\n- two\n- three\n", 1, "Not an item",
			"- one\n\nNot an item\n- three\n"},
		{"a paragraph becomes an item", "Intro\n\nPara\n\nAfter\n", 1, "- an item",
			"Intro\n\n- an item\n\nAfter\n"},
		// A list item interrupts a paragraph only when it isn't empty and, if
		// ordered, starts at 1 (CommonMark): otherwise it needs a blank line.
		{"an item emptied after a paragraph", "**Acceptance:**\n- one\n- two\n", 1, "-",
			"**Acceptance:**\n\n-\n- two\n"},
		{"an ordered item renumbered after a paragraph", "Steps:\n1. a\n2. b\n", 1, "2. a",
			"Steps:\n\n2. a\n2. b\n"},
		{"Archy's empty item", "Para\n- one\n", 1, "-", "Para\n\n-\n"},
		{"Archy's renumbered item", "Para\n- one\n", 1, "2. one", "Para\n\n2. one\n"},
		{"an ordered item from 1 after a paragraph", "Steps:\n1) a\n2) b\n", 1, "1) a, edited",
			"Steps:\n1) a, edited\n2) b\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			bs := Blocks(c.page)
			got, _, conflicts, _, err := MergeBlocks(hashes(bs), []Op{{Op: "replace", At: c.at, Hash: bs[c.at].Hash, Text: c.text}}, c.page)
			if err != nil || len(conflicts) > 0 || got != c.want {
				t.Errorf("got %q %v %v, want %q", got, conflicts, err, c.want)
			}
		})
	}
}

// When the page is still the base, nothing can conflict: the operations
// apply as sent, and the result is exactly mine.
func TestMergeBlocksOntoTheBaseIsMine(t *testing.T) {
	base := "# T\n\nintro\n\n- a\n- b\n  more\n- c\n\n| h |\n|---|\n| 1 |\n\n```\ncode\n```\n"
	for _, mine := range []string{
		base,
		"# T\n\nintro changed\n\n- a\n- b\n  more\n- c\n\n| h |\n|---|\n| 1 |\n\n```\ncode\n```\n",
		"# T\n\n- a\n- new\n- b\n  more\n\n| h |\n|---|\n| 1 |\n| 2 |\n\n```\ncode\n```\n\nend\n",
		"intro\n\n- c\n",
	} {
		if body, conflicts, _ := merged(t, base, mine, base); body != mine || conflicts != nil {
			t.Errorf("onto the base:\n%q\nwant mine:\n%q (%v)", body, mine, conflicts)
		}
	}
}

// A page with no block of the base left was rewritten: nothing is applied,
// inserts included, and every operation is a conflict.
func TestMergeBlocksOnARewrittenPage(t *testing.T) {
	b, ops := opsBetween("A\n\nB\n", "A\n\nB2\n\nNew\n")
	body, applied, conflicts, rewritten, err := MergeBlocks(b, ops, "X\n\nY\n")
	if err != nil || !rewritten || body != "X\n\nY\n" || len(applied) != 0 || len(conflicts) != len(ops) {
		t.Fatalf("body %q applied %v conflicts %v rewritten %v err %v", body, applied, conflicts, rewritten, err)
	}
	for _, c := range conflicts {
		if c.Case != "both-changed" {
			t.Errorf("conflict %+v, want both-changed", c)
		}
	}
}

// Identical blocks are told apart by position: a change to the second of two
// identical rules lands on the second, after the disk inserted before both.
func TestMergeBlocksIdenticalBlocksTakePosition(t *testing.T) {
	base := "a\n\n---\n\nmid\n\n---\n\nend\n"
	body, conflicts, _ := merged(t, base, "a\n\n---\n\nmid\n\n***\n\nend\n", "new\n\na\n\n---\n\nmid\n\n---\n\nend\n")
	if want := "new\n\na\n\n---\n\nmid\n\n***\n\nend\n"; body != want || conflicts != nil {
		t.Errorf("got %q %v, want %q", body, conflicts, want)
	}
}

// Several changes in one save keep the page's tight gaps between them, as
// one change does: an editor's save often carries several units.
func TestSeveralChangesInOneSaveStayTight(t *testing.T) {
	for _, c := range []struct {
		name, page string
		ops        func(bs []Block) []Op
		want       string
	}{
		{"two adjacent items replaced", "- a\n- b\n- c\n", func(bs []Block) []Op {
			return []Op{{Op: "replace", At: 0, Hash: bs[0].Hash, Text: "- a2"}, {Op: "replace", At: 1, Hash: bs[1].Hash, Text: "- b2"}}
		}, "- a2\n- b2\n- c\n"},
		{"two adjacent rows replaced", "| h |\n|---|\n| 1 |\n| 2 |\n| 3 |\n", func(bs []Block) []Op {
			return []Op{{Op: "replace", At: 1, Hash: bs[1].Hash, Text: "| 1b |"}, {Op: "replace", At: 2, Hash: bs[2].Hash, Text: "| 2b |"}}
		}, "| h |\n|---|\n| 1b |\n| 2b |\n| 3 |\n"},
		{"an item replaced and another inserted", "- a\n- b\n- c\n", func(bs []Block) []Op {
			return []Op{{Op: "replace", At: 0, Hash: bs[0].Hash, Text: "- a2"}, {Op: "insert", After: 0, Hash: bs[0].Hash, Text: "- new"}}
		}, "- a2\n- new\n- b\n- c\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			bs := Blocks(c.page)
			got, _, conflicts, _, err := MergeBlocks(hashes(bs), c.ops(bs), c.page)
			if err != nil || len(conflicts) > 0 || got != c.want {
				t.Errorf("got %q %v %v, want %q", got, conflicts, err, c.want)
			}
		})
	}
}

// When no blank line can keep new text apart from its neighbour (indented,
// it continues the item before it), the save is refused, nothing written,
// rather than written fused.
func TestMergeBlocksRefusesTextThatWouldJoinItsNeighbour(t *testing.T) {
	const page = "- a\n\nPara\n"
	bs := Blocks(page)
	for _, text := range []string{"    x", "  x"} {
		_, _, _, _, err := MergeBlocks(hashes(bs), []Op{{Op: "replace", At: 1, Hash: bs[1].Hash, Text: text}}, page)
		if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "operation 0: this block would join its neighbour") {
			t.Errorf("%q: err = %v", text, err)
		}
	}
}

// An operation whose text opens a block it doesn't close (a fence, an HTML
// comment, a <script> or <pre>) would swallow the rest of the page: refused,
// nothing written. Closed, it is accepted.
func TestMergeBlocksRefusesAnUnclosedBlock(t *testing.T) {
	const page = "A\n- b\n"
	bs := Blocks(page)
	for _, text := range []string{"```", "~~~~\ncode\n~~~", "<!-- a note", "<script>\nalert(1)", "- item\n\n  ```\n  code",
		"<?php echo 1;", "<!DOCTYPE html", "<![CDATA[ data"} {
		_, _, _, _, err := MergeBlocks(hashes(bs), []Op{{Op: "replace", At: 0, Hash: bs[0].Hash, Text: text}}, page)
		if !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "this block isn't closed: it would swallow the rest of the page") {
			t.Errorf("%q: err = %v", text, err)
		}
	}
	for text, want := range map[string]string{
		"```\ncode\n```":                             "```\ncode\n```\n- b\n",
		"<!-- a note -->":                            "<!-- a note -->\n\n- b\n",
		"<pre>\nx\n</pre>":                           "<pre>\nx\n</pre>\n\n- b\n",
		"<?php echo 1; ?>":                           "<?php echo 1; ?>\n- b\n",
		"<![CDATA[ data ]]>":                         "<![CDATA[ data ]]>\n- b\n",
		"    ```\n    indented code showing a fence": "    ```\n    indented code showing a fence\n- b\n",
	} {
		got, _, _, _, err := MergeBlocks(hashes(bs), []Op{{Op: "replace", At: 0, Hash: bs[0].Hash, Text: text}}, page)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", text, got, err, want)
		}
	}
}

func TestMergeBlocksRefusals(t *testing.T) {
	bs := Blocks("A\n\nB\n\nC\n")
	base := hashes(bs)
	many := make([]Op, MaxOps+1)
	for i := range many {
		many[i] = Op{Op: "insert", After: -1, Text: "x"}
	}
	for name, ops := range map[string][]Op{
		"out of range":           {{Op: "replace", At: 3, Hash: base[0], Text: "x"}},
		"wrong hash":             {{Op: "replace", At: 1, Hash: base[0], Text: "x"}},
		"start with a hash":      {{Op: "insert", After: -1, Hash: base[0], Text: "x"}},
		"out of order":           {{Op: "insert", After: 2, Hash: base[2], Text: "x"}, {Op: "replace", At: 0, Hash: base[0], Text: "y"}},
		"insert before replace":  {{Op: "insert", After: 1, Hash: base[1], Text: "x"}, {Op: "replace", At: 1, Hash: base[1], Text: "y"}},
		"two changes to a block": {{Op: "replace", At: 1, Hash: base[1], Text: "x"}, {Op: "delete", At: 1, Hash: base[1]}},
		"not UTF-8":              {{Op: "replace", At: 1, Hash: base[1], Text: "x\xff"}},
		"no text":                {{Op: "replace", At: 1, Hash: base[1], Text: "\n\n"}},
		"a move":                 {{Op: "move", At: 1, Hash: base[1], After: 2}},
		"too many operations":    many,
	} {
		if _, _, _, _, err := MergeBlocks(base, ops, "A\n\nB\n\nC\n"); !errors.Is(err, ErrRefused) {
			t.Errorf("%s: err = %v, want ErrRefused", name, err)
		}
	}
	big := make([]string, MaxBlocks+1)
	if _, _, _, _, err := MergeBlocks(big, nil, ""); !errors.Is(err, ErrRefused) {
		t.Errorf("a page over %d blocks: err = %v, want ErrRefused", MaxBlocks, err)
	}
	// Nor does a save make a page that big: one insert of many items.
	huge := []Op{{Op: "insert", After: 2, Hash: base[2], Text: strings.Repeat("- x\n", MaxBlocks)}}
	if _, _, _, _, err := MergeBlocks(base, huge, "A\n\nB\n\nC\n"); !errors.Is(err, ErrRefused) {
		t.Errorf("a save making a page over %d blocks: err = %v, want ErrRefused", MaxBlocks, err)
	}
	// Several inserts after one block, in their order, are fine.
	ok := []Op{{Op: "replace", At: 1, Hash: base[1], Text: "B2"}, {Op: "insert", After: 1, Hash: base[1], Text: "x"}, {Op: "insert", After: 1, Hash: base[1], Text: "y"}}
	if body, _, _, _, err := MergeBlocks(base, ok, "A\n\nB\n\nC\n"); err != nil || body != "A\n\nB2\n\nx\n\ny\n\nC\n" {
		t.Errorf("body %q err %v", body, err)
	}
}

// DiffBlocks reports what changed, by block, anchored to the old version: a
// move for a block that only changed place.
func TestDiffBlocks(t *testing.T) {
	diff := func(a, b string) string {
		var got []string
		for _, o := range DiffBlocks(Blocks(a), Blocks(b)) {
			s := o.Op + fmt.Sprintf(" at=%d after=%d", o.At, o.After)
			for _, x := range o.Blocks {
				s += " " + x.Text
			}
			got = append(got, s)
		}
		return strings.Join(got, "; ")
	}
	for _, c := range []struct{ a, b, want string }{
		{"A\n\nB\n\nC\n", "A\n\nB2\n\nC\n\nD\n", "replace at=1 after=0 B2; insert at=0 after=2 D"},
		{"A\n\nB\n\nC\n", "Z\n\nA\n\nC\n", "insert at=0 after=-1 Z; delete at=1 after=0"},
		{"A\n\nB\n\nC\n", "A\n\nB1\n\nB2\n\nC\n", "replace at=1 after=0 B1 B2"},
		{"A\n\nB\n\nC\n\nD\n\nE\n", "A\n\nC\n\nD\n\nE\n\nB\n", "move at=1 after=4"},
		{"A\n", "A\n", ""},
	} {
		if got := diff(c.a, c.b); got != c.want {
			t.Errorf("%q -> %q:\n%s\nwant:\n%s", c.a, c.b, got, c.want)
		}
	}
}

// An operation travels as JSON with only the fields its kind has.
func TestOpJSON(t *testing.T) {
	for _, c := range []struct {
		op   Op
		want string
	}{
		{Op{Op: "replace", At: 12, Hash: "h", Text: "- [x] new"}, `{"at":12,"hash":"h","op":"replace","text":"- [x] new"}`},
		{Op{Op: "insert", After: -1, Text: "start"}, `{"after":-1,"hash":"","op":"insert","text":"start"}`},
		{Op{Op: "delete", At: 3, Hash: "h"}, `{"at":3,"hash":"h","op":"delete"}`},
		{Op{Op: "move", At: 7, Hash: "h", After: 15}, `{"after":15,"at":7,"hash":"h","op":"move"}`},
		{Op{Op: "insert", After: 2, Hash: "h", Blocks: []Block{{Kind: "paragraph", Text: "x", Hash: "hx"}}}, `{"after":2,"blocks":[{"kind":"paragraph","text":"x","hash":"hx"}],"hash":"h","op":"insert"}`},
	} {
		b, err := json.Marshal(c.op)
		if err != nil || string(b) != c.want {
			t.Errorf("%s, want %s (%v)", b, c.want, err)
		}
		var back Op
		if err := json.Unmarshal(b, &back); err != nil || back.Op != c.op.Op || back.At != c.op.At || back.After != c.op.After || back.Text != c.op.Text {
			t.Errorf("round trip of %s: %+v %v", b, back, err)
		}
	}
}

// A page at the block cap diffs in linear space: no table of n×m, which for
// 20,000 blocks would hold 400 million cells.
func TestDiffInLinearSpace(t *testing.T) {
	a := make([]string, MaxBlocks)
	for i := range a {
		a[i] = fmt.Sprint(i)
	}
	b := append([]string(nil), a...)
	for i := 0; i < len(b); i += 997 {
		b[i] = "changed " + b[i]
	}
	b = append(b[:5000], append([]string{"new"}, b[5000:]...)...)
	// Bytes allocated, not time, which depends on the machine: a table would
	// take gigabytes, linear space a few megabytes.
	allocated := func(f func()) uint64 {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		f()
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}
	var pairs [][2]int
	if n := allocated(func() { pairs = lcsPairs(a, b) }); len(pairs) != MaxBlocks-21 || n > 64<<20 {
		t.Errorf("%d pairs, %d bytes allocated", len(pairs), n)
	}
	other := make([]string, MaxBlocks)
	for i := range other {
		other[i] = "x" + fmt.Sprint(i)
	}
	if n := allocated(func() { pairs = lcsPairs(a, other) }); len(pairs) != 0 || n > 8<<20 {
		t.Errorf("two pages with nothing in common: %d pairs, %d bytes allocated", len(pairs), n)
	}
}

// corpus is every committed page: what `wiki init --full` writes, and the
// kept page of each recorded merge.
func corpus(t *testing.T) map[string]string {
	t.Helper()
	pages := map[string]string{}
	dir := t.TempDir()
	if _, err := scaffold.Write(dir, scaffold.Full(), false); err != nil {
		t.Fatal(err)
	}
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(p, ".md") {
			raw, _ := os.ReadFile(p)
			pages[p] = Body(string(raw))
		}
		return nil
	})
	cases, _ := filepath.Glob(filepath.Join("testdata", "merges", "*", "kept.md"))
	for _, p := range cases {
		raw, _ := os.ReadFile(p)
		pages[p] = string(raw)
	}
	return pages
}

// New text never fuses with a neighbour: a paragraph inserted after any block,
// or put in place of any list item, with no gap given, is its own block, and
// every other block stays as it was. A table can't hold a paragraph, so the
// places between two of its rows are left out.
func TestNewTextNeverFusesWithANeighbour(t *testing.T) {
	const para = "A new paragraph, never part of its neighbours."
	n := 0
	for name, body := range corpus(t) {
		bs := Blocks(body)
		base := hashes(bs)
		check := func(what string, ops []Op, want []string) {
			got, _, conflicts, _, err := MergeBlocks(base, ops, body)
			if err != nil || len(conflicts) > 0 {
				t.Fatalf("%s, %s: %v %v", name, what, err, conflicts)
			}
			if h := hashes(Blocks(got)); strings.Join(h, " ") != strings.Join(want, " ") {
				t.Fatalf("%s, %s: the blocks changed:\n%q", name, what, got)
			}
			n++
		}
		for i, b := range bs {
			inTable := i+1 < len(bs) && bs[i+1].Kind == "row" && (b.Kind == "row" || b.Kind == "table-head")
			if !inTable {
				want := append(append(slices.Clone(base[:i+1]), BlockHash(para)), base[i+1:]...)
				check(fmt.Sprintf("a paragraph after block %d", i), []Op{{Op: "insert", After: i, Hash: b.Hash, Text: para}}, want)
			}
			if b.Kind == "item" {
				want := slices.Clone(base)
				want[i] = BlockHash(para)
				check(fmt.Sprintf("item %d replaced by a paragraph", i), []Op{{Op: "replace", At: i, Hash: b.Hash, Text: para}}, want)
			}
		}
	}
	if n == 0 {
		t.Fatal("the corpus held no blocks")
	}
	t.Logf("%d writes, none fused", n)
}
