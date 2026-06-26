package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/internal/bundle"
)

func build(t *testing.T, files map[string]string) *Index {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "wiki.toml"), []byte("spec=\"0.1\"\ntypes=[\"note\", \"concept\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, err := bundle.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Build(b)
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func TestBuildParsesEntry(t *testing.T) {
	idx := build(t, map[string]string{
		"a.md": "---\ntype: note\ntitle: A\ntags: [x, y]\n---\n# H\n[l](/b.md)\n- [ ] todo\n",
	})
	if len(idx.Entries) != 1 {
		t.Fatalf("entries = %d", len(idx.Entries))
	}
	e := idx.Entries[0]
	if e.Path != "/a.md" || e.Type != "note" || e.Title != "A" {
		t.Errorf("entry = %+v", e)
	}
	if len(e.Tags) != 2 || len(e.Links) != 1 || len(e.Tasks) != 1 || len(e.Headings) != 1 {
		t.Errorf("counts: tags=%v links=%v tasks=%v headings=%v", e.Tags, e.Links, e.Tasks, e.Headings)
	}
	// line numbers are file-relative: 5 frontmatter lines, then # H, link, task
	if e.Headings[0].Line != 6 || e.Links[0].Line != 7 || e.Tasks[0].Line != 8 {
		t.Errorf("file-relative lines: heading=%d link=%d task=%d (want 6/7/8)", e.Headings[0].Line, e.Links[0].Line, e.Tasks[0].Line)
	}
}

func TestBrokenAndOrphans(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\n---\n[a](/a.md)\n",
		"a.md":     "---\ntype: note\n---\n[missing](/nope.md)\n",
		"b.md":     "---\ntype: note\n---\nlonely\n",
	})
	if got := idx.Broken(); len(got) != 1 || got[0].Target != "/nope.md" {
		t.Errorf("broken = %+v", got)
	}
	if orph := idx.Orphans(); len(orph) != 1 || orph[0].Path != "/b.md" {
		t.Errorf("orphans = %+v (b unlinked; index.md exempt; a linked)", orph)
	}
}

func TestEscapingLinkIsBroken(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\n---\n[esc](/../../../../../../../../etc/hosts)\n",
	})
	if idx.FileExists("/../../../../../../../../etc/hosts") {
		t.Errorf("a target escaping the bundle must not resolve, even if it exists on the host")
	}
	if got := idx.Broken(); len(got) != 1 {
		t.Errorf("escaping link should be reported broken, got %+v", got)
	}
}

func TestFilter(t *testing.T) {
	idx := build(t, map[string]string{
		"finance/income/a.md": "---\ntype: note\ntags: [eu]\n---\n",
		"finance/b.md":        "---\ntype: concept\ntags: [eu]\n---\n",
		"tech/c.md":           "---\ntype: note\ntags: [go]\n---\n",
	})
	if got := len(idx.Filter("note", "", "")); got != 2 {
		t.Errorf("--type note = %d, want 2", got)
	}
	if got := len(idx.Filter("", "eu", "")); got != 2 {
		t.Errorf("--tag eu = %d, want 2", got)
	}
	if got := len(idx.Filter("", "", "finance/")); got != 2 {
		t.Errorf("--prefix finance/ = %d, want 2", got)
	}
	if got := idx.Filter("note", "eu", "finance/"); len(got) != 1 || got[0].Path != "/finance/income/a.md" {
		t.Errorf("combined filter = %+v", got)
	}
}

func TestCheckClean(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\nokf_version: \"0.1\"\n---\n[a](/a.md)\n",
		"a.md":     "---\ntype: note\n---\nok\n",
	})
	if got := idx.Check(); len(got) != 0 {
		t.Errorf("clean bundle should report nothing, got %+v", got)
	}
}

func TestCheckSeverity(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md":     "---\ntype: index\nokf_version: \"0.1\"\n---\n[a](/a.md)\n", // valid
		"a.md":         "---\ntype: note\n---\nok\n",                                // valid, linked
		"notype.md":    "---\ntitle: x\n---\nbody\n",                                // ERROR: missing type
		"weird.md":     "---\ntype: bogus\n---\n[x](/gone.md)\n",                    // WARNING unknown + ERROR broken link
		"sub/index.md": "---\ntype: note\n---\nbody\n",                              // WARNING: index.md should be type index
		"a/b/c/d/e.md": "---\ntype: note\n---\nbody\n",                              // WARNING: depth > 3
	})
	errs, warns := 0, 0
	for _, is := range idx.Check() {
		switch is.Level {
		case "error":
			errs++
		case "warning":
			warns++
		}
	}
	if errs != 2 {
		t.Errorf("errors = %d, want 2 (missing type, broken link)", errs)
	}
	if warns != 3 {
		t.Errorf("warnings = %d, want 3 (unknown type, index.md type, depth)", warns)
	}
}

func TestDepth(t *testing.T) {
	for p, want := range map[string]int{"/index.md": 0, "/a/x.md": 1, "/a/b/x.md": 2, "/a/b/c/x.md": 3} {
		if got := (&Entry{Path: p}).Depth(); got != want {
			t.Errorf("Depth(%q)=%d want %d", p, got, want)
		}
	}
}

func TestScalarTagCoercion(t *testing.T) {
	idx := build(t, map[string]string{"a.md": "---\ntype: note\ntags: solo\n---\nbody\n"})
	if got := idx.Entries[0].Tags; len(got) != 1 || got[0] != "solo" {
		t.Errorf("tags = %#v, want [solo]", got)
	}
}

func TestCheckOKFVersionSync(t *testing.T) {
	// build's wiki.toml declares spec 0.1, which embeds OKF 0.1.
	missing := build(t, map[string]string{"index.md": "---\ntype: index\n---\nhome\n"})
	if !hasWarning(missing.Check(), "/index.md", "okf_version") {
		t.Errorf("missing okf_version should warn, got %+v", missing.Check())
	}
	stale := build(t, map[string]string{"index.md": "---\ntype: index\nokf_version: \"0.2\"\n---\nhome\n"})
	if !hasWarning(stale.Check(), "/index.md", "okf_version") {
		t.Errorf("stale okf_version should warn, got %+v", stale.Check())
	}
	synced := build(t, map[string]string{"index.md": "---\ntype: index\nokf_version: \"0.1\"\n---\nhome\n"})
	if got := synced.Check(); len(got) != 0 {
		t.Errorf("synced bundle should be clean, got %+v", got)
	}
}

func hasWarning(issues []Issue, entry, substr string) bool {
	for _, is := range issues {
		if is.Level == "warning" && is.Entry == entry && strings.Contains(is.Msg, substr) {
			return true
		}
	}
	return false
}

func TestFix(t *testing.T) {
	readRoot := func(idx *Index) string {
		b, err := os.ReadFile(filepath.Join(idx.Bundle.Dir, "index.md"))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	// Drift: the fix is applied, the file rewritten, and a rebuild is clean.
	drift := build(t, map[string]string{"index.md": "---\ntype: index\nokf_version: \"0.2\"\n---\nhome\n"})
	fixes, err := drift.Fix(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fixes) != 1 || fixes[0].Field != "okf_version" || fixes[0].From != "0.2" || fixes[0].To != "0.1" {
		t.Fatalf("drift fix = %+v", fixes)
	}
	if !strings.Contains(readRoot(drift), `okf_version: "0.1"`) {
		t.Errorf("file not synced:\n%s", readRoot(drift))
	}
	rebuilt, err := Build(drift.Bundle)
	if err != nil {
		t.Fatal(err)
	}
	if got := rebuilt.Check(); len(got) != 0 {
		t.Errorf("post-fix check not clean: %+v", got)
	}

	// Missing: okf_version is inserted (From is empty).
	missing := build(t, map[string]string{"index.md": "---\ntype: index\n---\nhome\n"})
	if f, _ := missing.Fix(true); len(f) != 1 || f[0].From != "" || f[0].To != "0.1" {
		t.Errorf("missing fix = %+v", f)
	}
	if !strings.Contains(readRoot(missing), `okf_version: "0.1"`) {
		t.Errorf("okf_version not inserted:\n%s", readRoot(missing))
	}

	// In sync: no-op.
	synced := build(t, map[string]string{"index.md": "---\ntype: index\nokf_version: \"0.1\"\n---\nhome\n"})
	if f, _ := synced.Fix(true); len(f) != 0 {
		t.Errorf("synced needs no fix, got %+v", f)
	}

	// Dry run: reports the change but does not touch disk.
	dry := build(t, map[string]string{"index.md": "---\ntype: index\nokf_version: \"0.2\"\n---\nhome\n"})
	before := readRoot(dry)
	if f, _ := dry.Fix(false); len(f) != 1 {
		t.Errorf("dry run should report 1 fix, got %+v", f)
	}
	if readRoot(dry) != before {
		t.Errorf("dry run wrote to disk")
	}
}

func TestSetFrontmatterValue(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"update", "---\ntype: index\nokf_version: \"0.2\"\n---\nbody\n", "---\ntype: index\nokf_version: \"0.1\"\n---\nbody\n"},
		{"insert", "---\ntype: index\n---\nbody\n", "---\ntype: index\nokf_version: \"0.1\"\n---\nbody\n"},
		{"crlf", "---\r\ntype: index\r\n---\r\nbody\r\n", "---\r\ntype: index\r\nokf_version: \"0.1\"\r\n---\r\nbody\r\n"},
	}
	for _, c := range cases {
		got, err := setFrontmatterValue(c.in, "okf_version", "0.1")
		if err != nil || got != c.want {
			t.Errorf("%s: got %q err=%v, want %q", c.name, got, err, c.want)
		}
	}
	if _, err := setFrontmatterValue("no frontmatter here\n", "okf_version", "0.1"); err == nil {
		t.Errorf("expected an error when there is no frontmatter")
	}
}

func TestResolve(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md":          "---\ntype: index\n---\nhome\n",
		"finance/index.md":  "---\ntype: index\n---\n",
		"finance/income.md": "---\ntype: note\n---\nbody\n",
		"tech/index.md":     "---\ntype: index\n---\n",
	})
	// path (leading slash optional) and unique basename all reach the same entry
	for _, arg := range []string{"/finance/income.md", "finance/income.md", "income.md"} {
		if e, err := idx.Resolve(arg); err != nil || e.Path != "/finance/income.md" {
			t.Errorf("Resolve(%q) = %v, %v", arg, e, err)
		}
	}
	// ambiguous basename (3 index.md) and both missing forms error
	for _, arg := range []string{"index.md", "/nope.md", "nope.md"} {
		if _, err := idx.Resolve(arg); err == nil {
			t.Errorf("Resolve(%q) should error", arg)
		}
	}
}

func TestBody(t *testing.T) {
	idx := build(t, map[string]string{
		"a.md": "---\ntype: note\ntitle: A\n---\n# Heading\nbody line\n",
		"b.md": "no frontmatter here\njust text\n",
	})
	a, _ := idx.Resolve("a.md")
	body, err := a.Body()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "type: note") || strings.Contains(body, "---") {
		t.Errorf("frontmatter not stripped: %q", body)
	}
	if !strings.Contains(body, "# Heading") || !strings.Contains(body, "body line") {
		t.Errorf("body content missing: %q", body)
	}
	b, _ := idx.Resolve("b.md")
	if bb, _ := b.Body(); !strings.Contains(bb, "no frontmatter here") {
		t.Errorf("no-frontmatter body wrong: %q", bb)
	}
}

func TestSearch(t *testing.T) {
	idx := build(t, map[string]string{
		"finance/income.md":   "---\ntype: note\ntitle: Income\ntags: [money]\n---\nmonthly income tracking\nINCOME spikes in Q4\n",
		"finance/expenses.md": "---\ntype: concept\ntitle: Expenses\n---\nrent and groceries\n",
		"tech/notes.md":       "---\ntype: note\n---\nincome is mentioned here\n",
	})

	// case-insensitive, across files, sorted by path
	hits := idx.Search("income", "", "", "")
	if len(hits) != 2 || hits[0].Path != "/finance/income.md" || hits[1].Path != "/tech/notes.md" {
		t.Fatalf("hits = %+v", hits)
	}
	// income.md matches the title line + two body lines
	if hits[0].Matches != 3 {
		t.Errorf("income.md matches = %d want 3: %+v", hits[0].Matches, hits[0].Lines)
	}

	// filters narrow the candidate set
	if got := idx.Search("income", "note", "", ""); len(got) != 2 {
		t.Errorf("--type note = %d want 2", len(got))
	}
	if got := idx.Search("income", "concept", "", ""); len(got) != 0 {
		t.Errorf("--type concept = %d want 0", len(got))
	}
	if got := idx.Search("income", "", "", "finance/"); len(got) != 1 {
		t.Errorf("--prefix finance/ = %d want 1", len(got))
	}

	// frontmatter value (a tag) is searchable
	if got := idx.Search("money", "", "", ""); len(got) != 1 || got[0].Path != "/finance/income.md" {
		t.Errorf("tag-value search = %+v", got)
	}
	// no match
	if got := idx.Search("zzzznope", "", "", ""); len(got) != 0 {
		t.Errorf("no-match = %d want 0", len(got))
	}
}

func TestLinkGraph(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\n---\n[a](/a.md)\n[b](/b.md)\n",
		"a.md":     "---\ntype: note\n---\n[b](/b.md)\n[b again](/b.md)\n[gone](/nope.md)\n",
		"b.md":     "---\ntype: note\n---\nleaf, no links\n",
	})
	a, _ := idx.Resolve("a.md")

	// outgoing links deduped by target (a links to /b.md twice)
	if out := idx.OutLinks(a); len(out) != 2 || out[0].To != "/b.md" || out[1].To != "/nope.md" {
		t.Fatalf("a out-links = %+v want [/b.md /nope.md] (deduped)", out)
	}

	// backlinks deduped by source, sorted by path (a links to /b.md twice, counts once)
	if bl := idx.Backlinks("/b.md"); len(bl) != 2 || bl[0].From != "/a.md" || bl[1].From != "/index.md" {
		t.Errorf("backlinks /b.md = %+v (want a.md then index.md, deduped)", bl)
	}
	if bl := idx.Backlinks("/nope.md"); len(bl) != 1 || bl[0].From != "/a.md" {
		t.Errorf("backlinks /nope.md = %+v (a.md links to it)", bl)
	}
}

func TestMove(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\n---\n[a](/a.md)\n",
		"b.md":     "---\ntype: note\n---\nsee [a](/a.md#intro) and again [a](/a.md)\n",
		"c.md":     "---\ntype: note\n---\nprose /a.md not a link\n```\n[code](/a.md)\n```\n",
		"a.md":     "---\ntype: note\n---\nthe target\n",
	})
	dir := idx.Bundle.Dir
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(b)
	}

	res, err := idx.Move("/a.md", "/sub/a.md", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.From != "/a.md" || res.To != "/sub/a.md" {
		t.Errorf("res = %+v", res)
	}
	// rewrites in index.md (1) and b.md (2 on one line); NOT c.md (prose + fenced code)
	got := map[string]int{}
	for _, rw := range res.Rewrites {
		got[rw.Path] = rw.Links
	}
	if len(res.Rewrites) != 2 || got["/index.md"] != 1 || got["/b.md"] != 2 {
		t.Errorf("rewrites = %+v", res.Rewrites)
	}

	// the file moved, content intact
	if !strings.Contains(read("sub/a.md"), "the target") {
		t.Errorf("moved content wrong: %q", read("sub/a.md"))
	}
	if _, err := os.Stat(filepath.Join(dir, "a.md")); !os.IsNotExist(err) {
		t.Errorf("source still present after move")
	}
	// links rewritten; anchor preserved
	if !strings.Contains(read("index.md"), "[a](/sub/a.md)") {
		t.Errorf("index.md not rewritten: %q", read("index.md"))
	}
	if !strings.Contains(read("b.md"), "[a](/sub/a.md#intro)") || !strings.Contains(read("b.md"), "again [a](/sub/a.md)") {
		t.Errorf("b.md not rewritten/anchor lost: %q", read("b.md"))
	}
	// c.md untouched: prose mention + fenced-code link stay as /a.md
	if c := read("c.md"); strings.Contains(c, "/sub/a.md") || !strings.Contains(c, "/a.md") {
		t.Errorf("c.md should be untouched: %q", c)
	}
}

func TestMoveValidate(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\n---\n[a](/a.md)\n",
		"a.md":     "---\ntype: note\n---\nx\n",
		"b.md":     "---\ntype: note\n---\ny\n",
	})
	dir := idx.Bundle.Dir

	// dry-run: plan computed, nothing written
	res, err := idx.Move("/a.md", "/z.md", true)
	if err != nil || !res.DryRun || len(res.Rewrites) != 1 {
		t.Fatalf("dry-run res=%+v err=%v", res, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "z.md")); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote the destination")
	}
	if _, err := os.Stat(filepath.Join(dir, "a.md")); err != nil {
		t.Errorf("dry-run removed the source")
	}

	for _, tc := range []struct {
		name, src, dest string
	}{
		{"overwrite", "/a.md", "/b.md"},
		{"non-md dest", "/a.md", "/c"},
		{"missing src", "/nope.md", "/x.md"},
		{"dest equals src", "/a.md", "/a.md"},
		{"escapes bundle", "/a.md", "/../escape.md"},
		{"escapes via subdir", "/a.md", "/sub/../../escape.md"},
	} {
		if _, err := idx.Move(tc.src, tc.dest, false); err == nil {
			t.Errorf("%s: expected error", tc.name)
		}
	}
}

func TestMoveTitledLink(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\n---\n[t](/a.md \"keep me\")\n[u](/a.md#sec 'k')\n",
		"a.md":     "---\ntype: note\n---\nx\n",
	})
	dir := idx.Bundle.Dir
	if _, err := idx.Move("/a.md", "/b.md", false); err != nil {
		t.Fatal(err)
	}
	s, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	// the title and the anchor are both preserved through the rewrite
	if !strings.Contains(string(s), `[t](/b.md "keep me")`) || !strings.Contains(string(s), `[u](/b.md#sec 'k')`) {
		t.Errorf("title/anchor not preserved on rewrite: %q", s)
	}
}
