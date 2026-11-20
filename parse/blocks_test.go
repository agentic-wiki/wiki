package parse

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/internal/scaffold"
)

// kinds is Blocks' kinds and texts for a body.
func kinds(body string) []string {
	var out []string
	for _, b := range Blocks(body) {
		out = append(out, b.Kind+": "+b.Text)
	}
	return out
}

func TestBlocksKinds(t *testing.T) {
	body := "# Title\n\nA paragraph\nover two lines.\n\nSetext\n======\n\n---\n\n" +
		"```go\nx := 1\n\n```\n\n" +
		"| a | b |\n|---|---|\n| 1 | 2 |\n| 3 | 4 |\n\n" +
		"- one\n  continued\n- two\n  - nested\n\n  still two's\n- [ ] three\n\n" +
		"1. first\n2) second\n\n" +
		"> quoted\n> more\n\n<div>\nhtml\n</div>\n\n    indented code\n\n- - -\n"
	want := []string{
		"heading: # Title",
		"paragraph: A paragraph\nover two lines.",
		"heading: Setext\n======",
		"rule: ---",
		"code: ```go\nx := 1\n\n```",
		"table-head: | a | b |\n|---|---|",
		"row: | 1 | 2 |",
		"row: | 3 | 4 |",
		"item: - one\n  continued",
		"item: - two",
		"item:   - nested",
		"paragraph:   still two's", // under the outer item, whose block ended at the nested one
		"item: - [ ] three",
		"item: 1. first",
		"item: 2) second",
		"quote: > quoted\n> more",
		"html: <div>\nhtml\n</div>",
		"code:     indented code",
		"rule: - - -",
	}
	got := kinds(body)
	if strings.Join(got, "\n·\n") != strings.Join(want, "\n·\n") {
		t.Errorf("blocks:\n%s\nwant:\n%s", strings.Join(got, "\n·\n"), strings.Join(want, "\n·\n"))
	}
}

// Blocks plus the gaps between them are the body, byte for byte, and a gap
// holds only line breaks and blanks: no text is ever outside a block.
func checkRebuild(t *testing.T, name, body string) {
	t.Helper()
	bs := Blocks(body)
	pos := 0
	for i, b := range bs {
		if b.Start < pos || b.End < b.Start || body[b.Start:b.End] != b.Text || b.Hash != BlockHash(b.Text) {
			t.Fatalf("%s: block %d is not its span: %+v", name, i, b)
		}
		if gap := body[pos:b.Start]; strings.Trim(gap, " \t\r\n") != "" {
			t.Fatalf("%s: text outside a block before block %d: %q", name, i, gap)
		}
		pos = b.End
	}
	if rest := body[pos:]; strings.Trim(rest, " \t\r\n") != "" {
		t.Fatalf("%s: text after the last block: %q", name, rest)
	}
}

// A list item interrupts a paragraph only when it isn't empty and, if
// ordered, starts at 1 (CommonMark); otherwise its line is paragraph text
// (an empty `-` under a paragraph is a setext underline).
func TestAnItemInterruptsAParagraphOnlyByCommonMarksRule(t *testing.T) {
	for body, want := range map[string]string{
		"Para\n2. one\n":   "paragraph",
		"Para\n1. one\n":   "paragraph item",
		"Para\n01) one\n":  "paragraph item",
		"Para\n- one\n":    "paragraph item",
		"Para\n-\n":        "heading",
		"Para\n*\n":        "paragraph",
		"Para\n\n2. one\n": "paragraph item",
	} {
		var kinds []string
		for _, b := range Blocks(body) {
			kinds = append(kinds, b.Kind)
		}
		if got := strings.Join(kinds, " "); got != want {
			t.Errorf("%q: %s, want %s", body, got, want)
		}
	}
}

func TestBlocksRebuildThePage(t *testing.T) {
	for name, body := range map[string]string{
		"empty":       "",
		"blank":       "\n\n",
		"no newline":  "just text",
		"crlf":        "# T\r\n\r\npara\r\n\r\n- a\r\n- b\r\n",
		"open fence":  "```\ncode never closed\n",
		"leading gap": "\n\n# T\n",
		"table alone": "| a |\n|---|\n",
		"item gap":    "- a\n\n\n- b\n",
	} {
		checkRebuild(t, name, body)
	}
	// CRLF splits as LF does: the \r belongs to the line break.
	if got := kinds("# T\r\n\r\npara\r\n"); len(got) != 2 || got[1] != "paragraph: para" {
		t.Errorf("crlf blocks: %q", got)
	}
}

func TestBlockHashIs128Bits(t *testing.T) {
	if h := BlockHash("- [x] done"); len(h) != 32 || h == BlockHash("- [ ] done") {
		t.Errorf("hash %q", h)
	}
}

// Every page `wiki init --full` writes, and, when WIKI_BLOCKS_REPOS names
// repos, every .md file they hold, rebuilds from its blocks.
func TestBlocksRebuildEveryPage(t *testing.T) {
	dir := t.TempDir()
	if _, err := scaffold.Write(dir, scaffold.Full(), false); err != nil {
		t.Fatal(err)
	}
	pages := 0
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".md") {
			raw, _ := os.ReadFile(p)
			checkRebuild(t, p, Body(string(raw)))
			pages++
		}
		return nil
	})
	if pages == 0 {
		t.Fatal("init --full wrote no pages")
	}
	repos := os.Getenv("WIKI_BLOCKS_REPOS")
	if repos == "" {
		return
	}
	for _, repo := range strings.Split(repos, ",") {
		git := func(args ...string) ([]byte, error) {
			return exec.Command("git", append([]string{"-c", "safe.directory=*", "-C", repo}, args...)...).Output()
		}
		files, err := git("ls-tree", "-r", "--name-only", "HEAD")
		if err != nil {
			t.Fatalf("%s: %v", repo, err)
		}
		for _, f := range strings.Fields(string(files)) {
			if !strings.HasSuffix(f, ".md") {
				continue
			}
			raw, err := git("show", "HEAD:"+f)
			if err != nil {
				continue
			}
			checkRebuild(t, f, Body(string(raw)))
			pages++
		}
	}
	t.Logf("%d pages rebuilt from their blocks", pages)
}
