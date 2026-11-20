// Real concurrent edits, as tests of MergeBlocks and Blocks.
//
// Committed (testdata/merges): every page both parents of a merge commit
// changed in wiki-cli and wiki-shell, before 2026-11-20 02:25 UTC, as base,
// mine, theirs and what the merge kept, each distinct case once.
// TestMergeBlocksOnRecordedEdits always runs on them.
//
// Opt in, over local repos: WIKI_BLOCKS_REPOS=<repo>,<repo>,… re-runs the
// merge (TestMergeBlocksOnRealConcurrentEdits) and the block model
// (TestBlocksRebuildEveryPage) on any git repos on this machine. Given
// foreman, wiki-cli and wiki-shell, it asserts the editor ADR's counts: 75
// cases, 13 with conflicts, 55 of the 60 git merged cleanly identical to what
// was kept. `just test-corpus <repos>` runs it.
package parse

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// opsBetween is what an app sends for the change from base to mine: per run
// of changed blocks, replaces paired one to one (the last taking the rest),
// deletes for what is left of base, and an insert carrying mine's gap before
// it. The editor builds the same operations from its own document.
func opsBetween(base, mine string) ([]string, []Op) {
	b, m := Blocks(base), Blocks(mine)
	bh := hashes(b)
	var ops []Op
	pi, pj := 0, 0
	for _, p := range append(lcsPairs(bh, hashes(m)), [2]int{len(b), len(m)}) {
		f, t, jf, jt := pi, p[0], pj, p[1]
		span := func(x, y int) string { return mine[m[x].Start:m[y-1].End] }
		k, n := t-f, jt-jf
		switch {
		case k == 0 && n > 0:
			gap := ""
			if jf > 0 {
				gap = mine[m[jf-1].End:m[jf].Start]
			}
			hash := ""
			if f > 0 {
				hash = bh[f-1]
			}
			ops = append(ops, Op{Op: "insert", After: f - 1, Hash: hash, Text: gap + span(jf, jt)})
		case k > 0:
			for x := 0; x < k; x++ {
				switch {
				case x < n && (x < k-1 || n == k):
					ops = append(ops, Op{Op: "replace", At: f + x, Hash: bh[f+x], Text: span(jf+x, jf+x+1)})
				case x < n:
					ops = append(ops, Op{Op: "replace", At: f + x, Hash: bh[f+x], Text: span(jf+x, jt)})
				default:
					ops = append(ops, Op{Op: "delete", At: f + x, Hash: bh[f+x]})
				}
			}
		}
		pi, pj = p[0]+1, p[1]+1
	}
	return bh, ops
}

// mergeCase is one real concurrent edit: a page both parents of a merge
// commit changed. base is the merge base, mine parent 1, theirs parent 2, and
// kept what the merge commit holds. Bodies only: the header has its own write.
type mergeCase struct {
	name                     string
	base, mine, theirs, kept string
}

// repoMerges finds the real concurrent edits in a git repo: every merge commit
// made before until where both parents changed the same .md page.
func repoMerges(t *testing.T, repo, until string) []mergeCase {
	t.Helper()
	git := func(args ...string) (string, error) {
		out, err := exec.Command("git", append([]string{"-c", "safe.directory=*", "-C", repo}, args...)...).Output()
		return string(out), err
	}
	list, err := git("rev-list", "--merges", "--all", "--before="+until)
	if err != nil {
		t.Fatalf("%s: %v", repo, err)
	}
	var cases []mergeCase
	for _, m := range strings.Fields(list) {
		p1, e1 := git("rev-parse", m+"^1")
		p2, e2 := git("rev-parse", m+"^2")
		if e1 != nil || e2 != nil {
			continue
		}
		p1, p2 = strings.TrimSpace(p1), strings.TrimSpace(p2)
		b, err := git("merge-base", p1, p2)
		if err != nil {
			continue
		}
		b = strings.TrimSpace(b)
		changed := func(rev string) map[string]bool {
			out, _ := git("diff", "--name-only", b, rev, "--", "*.md")
			set := map[string]bool{}
			for _, f := range strings.Fields(out) {
				set[f] = true
			}
			return set
		}
		c1, c2 := changed(p1), changed(p2)
		for path := range c1 {
			if !c2[path] {
				continue
			}
			var texts [4]string
			ok := true
			for i, rev := range []string{b, p1, p2, m} {
				out, err := git("show", rev+":"+path)
				if err != nil {
					ok = false
					break
				}
				texts[i] = Body(out)
			}
			if ok {
				cases = append(cases, mergeCase{filepath.Base(repo) + " " + m[:7] + " " + path, texts[0], texts[1], texts[2], texts[3]})
			}
		}
	}
	return cases
}

// The committed real edits merge as they were measured: each case by name
// identical to what the merge kept, with a conflict, or different (a person
// resolved the merge by hand), and none refused.
func TestMergeBlocksOnRecordedEdits(t *testing.T) {
	notKept := map[string]string{
		"01-wiki-1e07194-changelog-md":       "differs",
		"02-wiki-1e07194-readme-md":          "conflict",
		"10-wiki-f4996a6-changelog-md":       "differs",
		"16-wiki-3119084-changelog-md":       "differs",
		"31-wiki-shell-23466b0-changelog-md": "differs",
	}
	dirs, err := os.ReadDir(filepath.Join("testdata", "merges"))
	if err != nil {
		t.Fatal(err)
	}
	kept := 0
	for _, d := range dirs {
		read := func(name string) string {
			b, err := os.ReadFile(filepath.Join("testdata", "merges", d.Name(), name))
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		}
		base, ops := opsBetween(read("base.md"), read("mine.md"))
		body, _, conflicts, _, err := MergeBlocks(base, ops, read("theirs.md"))
		got := "kept"
		switch {
		case err != nil:
			t.Errorf("%s: %v", d.Name(), err)
			continue
		case len(conflicts) > 0:
			got = "conflict"
		case body != read("kept.md"):
			got = "differs"
		default:
			kept++
		}
		want := notKept[d.Name()]
		if want == "" {
			want = "kept"
		}
		if got != want {
			t.Errorf("%s: %s, want %s", d.Name(), got, want)
		}
	}
	if len(dirs) != 23 || kept != 18 {
		t.Errorf("%d cases, %d identical to what was kept; want 23 and 18", len(dirs), kept)
	}
}

// The real concurrent edits of the editor ADR, re-run when WIKI_BLOCKS_REPOS
// names the repos (foreman, wiki-cli and wiki-shell, comma-separated). With
// all three, the counts are the ADR's: 75 cases, 13 with conflicts, and 55 of
// the 60 that git merged cleanly identical to what was kept.
func TestMergeBlocksOnRealConcurrentEdits(t *testing.T) {
	repos := os.Getenv("WIKI_BLOCKS_REPOS")
	if repos == "" {
		t.Skip("WIKI_BLOCKS_REPOS names no repos")
	}
	var cases []mergeCase
	for _, r := range strings.Split(repos, ",") {
		cases = append(cases, repoMerges(t, r, "2026-11-20T02:25:30Z")...)
	}
	tmp := t.TempDir()
	gitClean := func(c mergeCase) bool {
		w := func(n, s string) string {
			p := filepath.Join(tmp, n)
			os.WriteFile(p, []byte(s), 0o644)
			return p
		}
		return exec.Command("git", "merge-file", "-p", w("m", c.mine), w("b", c.base), w("t", c.theirs)).Run() == nil
	}
	var withConflicts, clean, identical int
	for _, c := range cases {
		base, ops := opsBetween(c.base, c.mine)
		body, _, conflicts, _, err := MergeBlocks(base, ops, c.theirs)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if len(conflicts) > 0 {
			withConflicts++
			continue
		}
		if gitClean(c) {
			clean++
			if body == c.kept {
				identical++
			} else if testing.Verbose() {
				t.Logf("%s differs from what was kept", c.name)
			}
		}
	}
	t.Logf("%d cases, %d with conflicts, %d of the clean git merges identical to what was kept (%d clean in both)", len(cases), withConflicts, identical, clean)
	if len(strings.Split(repos, ",")) == 3 && (len(cases) != 75 || withConflicts != 13 || identical != 55) {
		t.Errorf("want 75 cases, 13 with conflicts, 55 identical")
	}
}
