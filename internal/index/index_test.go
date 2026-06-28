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
		"log.md":   "# Log\n## 2026-01-01\nentry\n", // reserved + unlinked: must be exempt
	})
	if got := idx.Broken(); len(got) != 1 || got[0].Target != "/nope.md" {
		t.Errorf("broken = %+v", got)
	}
	if orph := idx.Orphans(); len(orph) != 1 || orph[0].Path != "/b.md" {
		t.Errorf("orphans = %+v (b unlinked; index.md and log.md exempt; a linked)", orph)
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

func TestRelativeLinkCannotEscapeBundle(t *testing.T) {
	idx := build(t, map[string]string{
		"sub/page.md": "---\ntype: note\n---\n[esc](../../../../../../etc/passwd)\n",
	})
	// path.Join caps at the bundle root, so the target stays in-bundle (/etc/passwd
	// here, relative to the bundle, not the host) and is simply broken.
	bl := idx.Broken()
	if len(bl) != 1 || bl[0].Target != "/etc/passwd" {
		t.Errorf("relative escape should cap at the bundle root, got %+v", bl)
	}
}

func TestRelativeLinkCountsForOrphans(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\n---\n[a](./a.md)\n", // relative edge
		"a.md":     "---\ntype: note\n---\nx\n",
	})
	for _, o := range idx.Orphans() {
		if o.Path == "/a.md" {
			t.Fatalf("/a.md is linked relatively and must not be an orphan; orphans=%+v", idx.Orphans())
		}
	}
}

func TestReservedFilesExemptFromType(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n[a](/a.md)\n", // reserved: okf_version only
		"log.md":   "# Log\n## 2026-01-01\nnote\n",                 // reserved: no frontmatter
		"a.md":     "---\ntype: note\n---\nx\n",
	})
	// reserved files are exempt from the type requirement -> fully clean
	if got := idx.Check(); len(got) != 0 {
		t.Errorf("reserved files without a type should be clean, got %+v", got)
	}
}

func TestReservedFileFrontmatterLinted(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\nokf_version: \"0.1\"\n---\nhome\n", // type is disallowed here
	})
	issues := idx.Check()
	if !hasWarning(issues, "/index.md", "no frontmatter") {
		t.Errorf("reserved file carrying frontmatter should warn, got %+v", issues)
	}
	for _, is := range issues {
		if is.Level == "error" {
			t.Errorf("reserved file must not produce an error: %+v", is)
		}
	}
}

func TestCheckTimestamp(t *testing.T) {
	for _, ok := range []string{"2026-06-24", "\"2026-06-24T10:00:00Z\""} {
		idx := build(t, map[string]string{"a.md": "---\ntype: note\ntimestamp: " + ok + "\n---\nx\n"})
		if got := idx.Check(); len(got) != 0 {
			t.Errorf("valid timestamp %s should be clean, got %+v", ok, got)
		}
	}
	// absent is fine (optional in OKF)
	if got := build(t, map[string]string{"a.md": "---\ntype: note\n---\nx\n"}).Check(); len(got) != 0 {
		t.Errorf("missing timestamp must not be flagged, got %+v", got)
	}
	// present but empty or non-ISO -> error
	for _, bad := range []string{"\"\"", "not-a-date", "\"2026/06/24\""} {
		idx := build(t, map[string]string{"a.md": "---\ntype: note\ntimestamp: " + bad + "\n---\nx\n"})
		err := false
		for _, is := range idx.Check() {
			if is.Level == "error" && strings.Contains(is.Msg, "timestamp") {
				err = true
			}
		}
		if !err {
			t.Errorf("timestamp %s should error, got %+v", bad, idx.Check())
		}
	}
}

func TestCheckLogDateHeadings(t *testing.T) {
	bad := build(t, map[string]string{"log.md": "# Log\n## 01/02/2026\nnote\n## Notes\nfine\n"})
	if !hasWarning(bad.Check(), "/log.md", "ISO YYYY-MM-DD") {
		t.Errorf("non-ISO log date heading should warn, got %+v", bad.Check())
	}
	clean := build(t, map[string]string{"log.md": "# Log\n## 2026-01-02\nnote\n## Notes\nfine\n"})
	for _, is := range clean.Check() {
		if strings.Contains(is.Msg, "ISO") {
			t.Errorf("ISO date heading must not warn: %+v", is)
		}
	}
}

func TestNormalizeLink(t *testing.T) {
	cases := []struct{ from, target, want string }{
		{"/index.md", "/a.md", "/a.md"},            // absolute passes through
		{"/index.md", "/a.md#sec", "/a.md#sec"},    // anchor preserved
		{"/index.md", "a.md", "/a.md"},             // bare, from root
		{"/index.md", "./a.md", "/a.md"},           // explicit current dir
		{"/sub/x.md", "a.md", "/sub/a.md"},         // bare, from a subdir
		{"/sub/x.md", "../a.md", "/a.md"},          // up one
		{"/sub/deep/x.md", "../../a.md", "/a.md"},  // up two
		{"/sub/x.md", "../a.md#sec", "/a.md#sec"},  // relative + anchor
		{"/sub/x.md", "../../../../a.md", "/a.md"}, // cannot climb above the root
		{"/a/b.md", "c/d.md", "/a/c/d.md"},         // nested relative
	}
	for _, c := range cases {
		if got := normalizeLink(c.from, c.target); got != c.want {
			t.Errorf("normalizeLink(%q, %q) = %q, want %q", c.from, c.target, got, c.want)
		}
	}
}

func TestLinksToSameTargetAcrossForms(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\n---\n[a](/hello.md)\n",    // absolute
		"sub/b.md": "---\ntype: note\n---\n[b](../hello.md)\n",   // relative
		"sub/c.md": "---\ntype: note\n---\n[c](../hello.md#x)\n", // relative + anchor
		"hello.md": "---\ntype: note\n---\nhi\n",
	})
	// all three forms resolve to /hello.md -> three backlinks (one per source)
	if bl := idx.Backlinks("/hello.md"); len(bl) != 3 {
		t.Fatalf("want 3 backlinks across forms, got %+v", bl)
	}
	// NormalizeLinks rewrites the two relatives; the absolute one is left as-is
	changes, _ := idx.NormalizeLinks(true)
	if len(changes) != 2 {
		t.Errorf("want 2 normalized links (the relatives), got %+v", changes)
	}
	b, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "sub/b.md"))
	c, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "sub/c.md"))
	if !strings.Contains(string(b), "[b](/hello.md)") || !strings.Contains(string(c), "[c](/hello.md#x)") {
		t.Errorf("relatives not normalized:\n%s%s", b, c)
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
		"index.md": "---\nokf_version: \"0.1\"\n---\n[a](/a.md)\n",
		"a.md":     "---\ntype: note\n---\nok\n",
	})
	if got := idx.Check(); len(got) != 0 {
		t.Errorf("clean bundle should report nothing, got %+v", got)
	}
}

func TestCheckSeverity(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md":     "---\nokf_version: \"0.1\"\n---\n[a](/a.md)\n", // valid
		"a.md":         "---\ntype: note\n---\nok\n",                   // valid, linked
		"notype.md":    "---\ntitle: x\n---\nbody\n",                   // ERROR: missing type
		"weird.md":     "---\ntype: bogus\n---\n[x](/gone.md)\n",       // WARNING unknown type + WARNING broken link
		"sub/index.md": "---\ntype: note\n---\nbody\n",                 // WARNING: reserved file carries frontmatter
		"a/b/c/d/e.md": "---\ntype: note\n---\nbody\n",                 // WARNING: depth > 3
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
	if errs != 1 {
		t.Errorf("errors = %d, want 1 (missing type only; a broken link is a warning, not an error)", errs)
	}
	if warns != 4 {
		t.Errorf("warnings = %d, want 4 (unknown type, broken link, reserved-file frontmatter, depth)", warns)
	}
	// A broken link is a warning per OKF (not-yet-written knowledge), so it never fails the lint.
	if !hasWarning(idx.Check(), "/weird.md", "broken link") {
		t.Errorf("broken link should be a warning, got %+v", idx.Check())
	}
}

func TestCheckFilenameSpace(t *testing.T) {
	idx := build(t, map[string]string{"a b.md": "---\ntype: note\n---\nbody\n"})
	if !hasWarning(idx.Check(), "/a b.md", "space") {
		t.Errorf("a filename with a space should warn, got %+v", idx.Check())
	}
}

func TestAngleBracketLinkResolvesButNotNormalized(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\n---\n[s](<../a b.md>)\n",
		"a b.md":   "---\ntype: note\n---\nx\n",
	})
	// the <...> link parses cleanly and resolves into the graph (not garbage)
	if len(idx.Backlinks("/a b.md")) == 0 {
		t.Errorf("angle-bracket relative link should resolve to a backlink, got %+v", idx.Backlinks("/a b.md"))
	}
	// the space surfaces as the filename warning, not a broken link
	if !hasWarning(idx.Check(), "/a b.md", "space") {
		t.Errorf("spaced filename should warn, got %+v", idx.Check())
	}
	// NormalizeLinks leaves space targets alone (rename the file instead)
	if c, _ := idx.NormalizeLinks(false); len(c) != 0 {
		t.Errorf("NormalizeLinks should skip space targets, got %+v", c)
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
	missing := build(t, map[string]string{"index.md": "home\n"})
	if !hasWarning(missing.Check(), "/index.md", "okf_version") {
		t.Errorf("missing okf_version should warn, got %+v", missing.Check())
	}
	stale := build(t, map[string]string{"index.md": "---\nokf_version: \"0.2\"\n---\nhome\n"})
	if !hasWarning(stale.Check(), "/index.md", "okf_version") {
		t.Errorf("stale okf_version should warn, got %+v", stale.Check())
	}
	synced := build(t, map[string]string{"index.md": "---\nokf_version: \"0.1\"\n---\nhome\n"})
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
	drift := build(t, map[string]string{"index.md": "---\nokf_version: \"0.2\"\n---\nhome\n"})
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
	missing := build(t, map[string]string{"index.md": "home\n"})
	if f, _ := missing.Fix(true); len(f) != 1 || f[0].From != "" || f[0].To != "0.1" {
		t.Errorf("missing fix = %+v", f)
	}
	if !strings.Contains(readRoot(missing), `okf_version: "0.1"`) {
		t.Errorf("okf_version not inserted:\n%s", readRoot(missing))
	}

	// In sync: no-op.
	synced := build(t, map[string]string{"index.md": "---\nokf_version: \"0.1\"\n---\nhome\n"})
	if f, _ := synced.Fix(true); len(f) != 0 {
		t.Errorf("synced needs no fix, got %+v", f)
	}

	// Dry run: reports the change but does not touch disk.
	dry := build(t, map[string]string{"index.md": "---\nokf_version: \"0.2\"\n---\nhome\n"})
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
		{"create", "no frontmatter here\n", "---\nokf_version: \"0.1\"\n---\nno frontmatter here\n"},
	}
	for _, c := range cases {
		got, err := setFrontmatterValue(c.in, "okf_version", "0.1")
		if err != nil || got != c.want {
			t.Errorf("%s: got %q err=%v, want %q", c.name, got, err, c.want)
		}
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

	// backlinks: every occurrence (a links to /b.md twice -> both rows), sorted by
	// source then line, so a.md:4, a.md:5, then index.md.
	if bl := idx.Backlinks("/b.md"); len(bl) != 3 || bl[0].From != "/a.md" || bl[0].Line != 4 || bl[1].From != "/a.md" || bl[1].Line != 5 || bl[2].From != "/index.md" {
		t.Errorf("backlinks /b.md = %+v (want a.md:4, a.md:5, index.md)", bl)
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

func TestMoveRewritesRelativeLinks(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\n---\n[abs](/a.md) [rel](a.md)\n", // both resolve to /a.md
		"sub/x.md": "---\ntype: note\n---\n[up](../a.md#sec)\n",         // relative + anchor
		"a.md":     "---\ntype: note\n---\nhi\n",
	})
	if _, err := idx.Move("/a.md", "/b.md", false); err != nil {
		t.Fatal(err)
	}
	root, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "index.md"))
	if !strings.Contains(string(root), "[abs](/b.md)") || !strings.Contains(string(root), "[rel](/b.md)") {
		t.Errorf("both absolute and relative links to the moved file should be rewritten:\n%s", root)
	}
	sub, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "sub/x.md"))
	if !strings.Contains(string(sub), "[up](/b.md#sec)") {
		t.Errorf("relative+anchor link should rewrite to absolute dest + anchor:\n%s", sub)
	}
}

func TestSlugify(t *testing.T) {
	idx := build(t, map[string]string{
		"my note.md": "---\ntype: note\n---\nx\n",
		"index.md":   "---\nokf_version: \"0.1\"\n---\n[n](</my note.md>)\n", // angle-bracketed inbound link
	})
	changes, err := idx.Slugify(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0].To != "/my-note.md" {
		t.Fatalf("slugify = %+v, want one rename to /my-note.md", changes)
	}
	if _, err := os.Stat(filepath.Join(idx.Bundle.Dir, "my-note.md")); err != nil {
		t.Errorf("file not renamed to my-note.md: %v", err)
	}
	if _, err := os.Stat(filepath.Join(idx.Bundle.Dir, "my note.md")); err == nil {
		t.Errorf("old spaced file should be gone")
	}
	// the angle-bracketed inbound link is rewritten to the slug (space gone -> bare)
	root, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "index.md"))
	if !strings.Contains(string(root), "[n](/my-note.md)") {
		t.Errorf("inbound <…> link not rewritten:\n%s", root)
	}
}

func TestMoveAcrossLinkForms(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\n---\n[a](/hello.md)\n",    // absolute
		"sub/b.md": "---\ntype: note\n---\n[b](../hello.md)\n",   // relative
		"sub/c.md": "---\ntype: note\n---\n[c](../hello.md#x)\n", // relative + anchor
		"hello.md": "---\ntype: note\n---\nhi\n",
	})
	if _, err := idx.Move("/hello.md", "/world.md", false); err != nil {
		t.Fatal(err)
	}
	for f, want := range map[string]string{
		"index.md": "[a](/world.md)",
		"sub/b.md": "[b](/world.md)",
		"sub/c.md": "[c](/world.md#x)", // anchor preserved through the rewrite
	} {
		got, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, filepath.FromSlash(f)))
		if !strings.Contains(string(got), want) {
			t.Errorf("%s: want %q in\n%s", f, want, got)
		}
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

func TestRelativeLinksResolveAndNormalize(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md":    "---\ntype: index\nokf_version: \"0.1\"\n---\n[up](sub/page.md) [bad](nope.md)\n[anc](sub/page.md#sec) [tit](sub/page.md \"hi\")\n",
		"sub/page.md": "---\ntype: note\n---\nhi\n",
	})

	// Relative links are valid (OKF) and resolved into the graph: sub/page.md is
	// a real backlink target, not orphaned, and check does not flag the form.
	if bl := idx.Backlinks("/sub/page.md"); len(bl) == 0 {
		t.Errorf("relative link should resolve to a backlink edge, got %+v", bl)
	}
	if hasWarning(idx.Check(), "/index.md", "not root-absolute") {
		t.Errorf("relative links are valid; check must not flag them: %+v", idx.Check())
	}
	// The one that resolves nowhere is still reported broken (check audits).
	broken := false
	for _, b := range idx.Broken() {
		if b.From == "/index.md" && b.Target == "/nope.md" {
			broken = true
		}
	}
	if !broken {
		t.Errorf("unresolvable relative link should be broken (-> /nope.md), got %+v", idx.Broken())
	}

	// NormalizeLinks canonicalizes every relative link (anchor + title preserved),
	// including the unresolvable one (the absolute form is deterministic).
	changes, err := idx.NormalizeLinks(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 4 { // up, bad, anc, tit
		t.Errorf("expected 4 normalized links, got %+v", changes)
	}
	raw, err := os.ReadFile(filepath.Join(idx.Bundle.Dir, "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{"[up](/sub/page.md)", "[bad](/nope.md)", "[anc](/sub/page.md#sec)", `[tit](/sub/page.md "hi")`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}

	// Once everything is absolute, there is nothing left to normalize.
	rebuilt, err := Build(idx.Bundle)
	if err != nil {
		t.Fatal(err)
	}
	if dry, _ := rebuilt.NormalizeLinks(false); len(dry) != 0 {
		t.Errorf("nothing should remain to normalize, got %+v", dry)
	}
}

func TestCounts(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n[a](/a.md)\n", // reserved root: okf_version only, no type
		"a.md":     "---\ntype: note\nstatus: open\ntags: [x, y]\n---\nA\n",
		"b.md":     "---\ntype: note\nstatus: done\ntags: [y, y]\n---\nB\n", // duplicate tag y in one entry
		"sub/c.md": "---\ntype: concept\nstatus: open\ntags: [z]\n---\nC\n",
	})

	// TagCounts: y is in a and b (twice in b, but an entry counts once).
	tags := idx.TagCounts("")
	if len(tags) != 3 || tags["x"] != 1 || tags["y"] != 2 || tags["z"] != 1 {
		t.Errorf("TagCounts = %v, want {x:1, y:2, z:1}", tags)
	}
	if sub := idx.TagCounts("sub/"); len(sub) != 1 || sub["z"] != 1 {
		t.Errorf("TagCounts(sub/) = %v, want {z:1}", sub)
	}

	// PropertyKeyCounts: index.md contributes only okf_version (reserved, no type).
	keys := idx.PropertyKeyCounts("")
	for key, want := range map[string]int{"okf_version": 1, "type": 3, "status": 3, "tags": 3} {
		if keys[key] != want {
			t.Errorf("PropertyKeyCounts[%q] = %d, want %d (all: %v)", key, keys[key], want, keys)
		}
	}

	// PropertyValueCounts: scalar key (status, type) and list key (tags), with prefix.
	if st := idx.PropertyValueCounts("status", ""); st["open"] != 2 || st["done"] != 1 {
		t.Errorf("PropertyValueCounts(status) = %v, want {open:2, done:1}", st)
	}
	if ty := idx.PropertyValueCounts("type", ""); ty["note"] != 2 || ty["concept"] != 1 {
		t.Errorf("PropertyValueCounts(type) = %v, want {note:2, concept:1}", ty)
	}
	if tg := idx.PropertyValueCounts("tags", ""); tg["x"] != 1 || tg["y"] != 2 || tg["z"] != 1 {
		t.Errorf("PropertyValueCounts(tags) = %v, want {x:1, y:2, z:1}", tg)
	}
	if st := idx.PropertyValueCounts("status", "sub/"); len(st) != 1 || st["open"] != 1 {
		t.Errorf("PropertyValueCounts(status, sub/) = %v, want {open:1}", st)
	}
	if got := idx.PropertyValueCounts("nope", ""); len(got) != 0 {
		t.Errorf("PropertyValueCounts(nope) = %v, want empty", got)
	}
}
