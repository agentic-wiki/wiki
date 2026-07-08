package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/internal/bundle"
	"github.com/agentic-wiki/wiki/internal/parse"
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
	// type is a materialized field; title/tags are read from frontmatter on demand
	if e.Path != "/a.md" || e.Type != "note" || parse.String(e.fm, "title") != "A" {
		t.Errorf("entry = %+v", e)
	}
	if tags := parse.Strings(e.fm, "tags"); len(tags) != 2 || len(e.Links) != 1 || len(e.Checkboxes) != 1 || len(e.Headings) != 1 {
		t.Errorf("counts: tags=%v links=%v checkboxes=%v headings=%v", tags, e.Links, e.Checkboxes, e.Headings)
	}
	// line numbers are file-relative: 5 frontmatter lines, then # H, link, task
	if e.Headings[0].Line != 6 || e.Links[0].Line != 7 || e.Checkboxes[0].Line != 8 {
		t.Errorf("file-relative lines: heading=%d link=%d checkbox=%d (want 6/7/8)", e.Headings[0].Line, e.Links[0].Line, e.Checkboxes[0].Line)
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

func TestEscapingLinkIsOutOfBundleNotBroken(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\ntype: index\n---\n[esc](/../../../../../../../../etc/hosts)\n",
	})
	// A link that climbs above the bundle root points outside the self-contained
	// bundle: it is not an internal edge, so it is not indexed and not reported
	// broken — but check surfaces it as its own out-of-bundle advisory.
	if got := idx.Entries[0].Links; len(got) != 0 {
		t.Errorf("out-of-bundle link must not become an edge, got %+v", got)
	}
	if got := idx.Broken(); len(got) != 0 {
		t.Errorf("out-of-bundle link must not be reported broken, got %+v", got)
	}
	if !hasWarning(idx.Check(), "/index.md", "out-of-bundle link") {
		t.Errorf("escaping link should warn as out-of-bundle, got %+v", idx.Check())
	}
	// The security guard still stands for direct callers (e.g. Move's dest check):
	// an escaping target must never resolve, even if it exists on the host.
	if idx.FileExists("/../../../../../../../../etc/hosts") {
		t.Errorf("FileExists must refuse a target that escapes the bundle")
	}
}

// A bundle nested in a repo often references sibling files (a repo-root PRD): the
// link is legitimate but outside the bundle, so check must flag it as out-of-bundle
// (an advisory warning), never as a broken link or an error.
func TestOutOfBundleReferenceWarns(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md":    "---\ntype: index\n---\n[prd](../PRD.md) and [b](/b.md)\n",
		"sub/page.md": "---\ntype: note\n---\n[prd](../../PRD.md)\n",
		"b.md":        "---\ntype: note\n---\nx\n",
	})
	if got := idx.Broken(); len(got) != 0 {
		t.Errorf("out-of-bundle refs must not be broken; only in-bundle links count, got %+v", got)
	}
	issues := idx.Check()
	if !hasWarning(issues, "/index.md", "out-of-bundle link -> ../PRD.md") ||
		!hasWarning(issues, "/sub/page.md", "out-of-bundle link -> ../../PRD.md") {
		t.Errorf("both out-of-bundle refs should warn, got %+v", issues)
	}
	for _, is := range issues {
		if is.Level == "error" {
			t.Errorf("out-of-bundle reference must not be an error: %+v", is)
		}
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
	// Resolution is pure path math against the root, so any real dir works as root.
	root := t.TempDir()
	cases := []struct {
		from, target, want string
		escapes            bool
	}{
		{"/index.md", "/a.md", "/a.md", false},                 // absolute
		{"/index.md", "/a.md#sec", "/a.md#sec", false},         // anchor preserved
		{"/index.md", "a.md", "/a.md", false},                  // bare, from root
		{"/index.md", "./a.md", "/a.md", false},                // explicit current dir
		{"/sub/x.md", "a.md", "/sub/a.md", false},              // bare, from a subdir
		{"/sub/x.md", "../a.md", "/a.md", false},               // up one
		{"/sub/deep/x.md", "../../a.md", "/a.md", false},       // up two
		{"/sub/x.md", "../a.md#sec", "/a.md#sec", false},       // relative + anchor
		{"/a/b.md", "c/d.md", "/a/c/d.md", false},              // nested relative
		{"/a/x.md", "/a/../b.md", "/b.md", false},              // absolute interior .. canonicalized
		{"/index.md", "../PRD.md", "", true},                   // relative climbs above root
		{"/sub/x.md", "../../../../a.md", "", true},            // relative climbs above root
		{"/index.md", "/something/../../../this.md", "", true}, // absolute climbs above root
	}
	for _, c := range cases {
		abs, escapes := normalizeLink(root, c.from, c.target)
		if escapes != c.escapes || (!escapes && abs != c.want) {
			t.Errorf("normalizeLink(%q, %q) = (%q, %v), want (%q, %v)", c.from, c.target, abs, escapes, c.want, c.escapes)
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
	if got := len(idx.Filter("", []PropFilter{{Key: "type", Value: "note"}})); got != 2 {
		t.Errorf("type=note = %d, want 2", got)
	}
	if got := len(idx.Filter("", []PropFilter{{Key: "tags", Value: "eu"}})); got != 2 {
		t.Errorf("tags=eu = %d, want 2", got)
	}
	if got := len(idx.Filter("finance/", nil)); got != 2 {
		t.Errorf("--prefix finance/ = %d, want 2", got)
	}
	// prefix + two ANDed property filters
	if got := idx.Filter("finance/", []PropFilter{{Key: "type", Value: "note"}, {Key: "tags", Value: "eu"}}); len(got) != 1 || got[0].Path != "/finance/income/a.md" {
		t.Errorf("combined filter = %+v", got)
	}
	// a missing key never matches an equality filter
	if got := idx.Filter("", []PropFilter{{Key: "nope", Value: "x"}}); len(got) != 0 {
		t.Errorf("unknown key = %d, want 0", len(got))
	}

	// negation: everything whose type is not note
	if got := idx.Filter("", []PropFilter{{Key: "type", Value: "note", Negate: true}}); len(got) != 1 || got[0].Path != "/finance/b.md" {
		t.Errorf("type!=note = %+v, want only /finance/b.md", got)
	}
	// a missing key matches a negation filter (it is not equal to the value)
	if got := len(idx.Filter("", []PropFilter{{Key: "nope", Value: "x", Negate: true}})); got != 3 {
		t.Errorf("nope!=x = %d, want 3 (missing key is not equal)", got)
	}
	// negation composes with equality under AND: notes that are not tagged go
	if got := idx.Filter("", []PropFilter{{Key: "type", Value: "note"}, {Key: "tags", Value: "eu", Negate: true}}); len(got) != 1 || got[0].Path != "/tech/c.md" {
		t.Errorf("type=note AND tags!=eu = %+v, want only /tech/c.md", got)
	}
}

// TestFilterEmptyValue covers comparing against the empty string, which tests
// emptiness rather than a literal value: `key=` matches when the key has no
// non-empty value (absent, blank, or empty list), so `key!=` is a presence
// filter (present and non-empty).
func TestFilterEmptyValue(t *testing.T) {
	idx := build(t, map[string]string{
		"present.md": "---\ntype: task\nassignee: ana\ntags: [x]\n---\n",
		"empty.md":   "---\ntype: task\nassignee: \"\"\ntags: []\n---\n",
		"bare.md":    "---\ntype: task\nassignee:\n---\n",
		"absent.md":  "---\ntype: task\n---\n",
	})
	paths := func(es []*Entry) []string {
		out := make([]string, len(es))
		for i, e := range es {
			out[i] = e.Path
		}
		return out
	}
	// key!= is present-and-non-empty
	if got := paths(idx.Filter("", []PropFilter{{Key: "assignee", Value: "", Negate: true}})); len(got) != 1 || got[0] != "/present.md" {
		t.Errorf("assignee!= = %v, want only /present.md", got)
	}
	// key= is the complement: absent, blank, or empty
	if got := len(idx.Filter("", []PropFilter{{Key: "assignee", Value: ""}})); got != 3 {
		t.Errorf("assignee= = %d, want 3 (empty/bare/absent)", got)
	}
	// an empty list counts as no value too: tags!= keeps only the tagged entry
	if got := paths(idx.Filter("", []PropFilter{{Key: "tags", Value: "", Negate: true}})); len(got) != 1 || got[0] != "/present.md" {
		t.Errorf("tags!= = %v, want only /present.md", got)
	}
}

func TestMoveIncludeFrontmatter(t *testing.T) {
	files := map[string]string{
		"index.md":      "---\nokf_version: \"0.1\"\n---\n[e](/epics/auth.md)\n",
		"epics/auth.md": "---\ntype: note\n---\nepic\n",
		"login.md":      "---\ntype: note\nepic: /epics/auth.md\n---\nlogin [see](/epics/auth.md)\n",
		"oauth.md":      "---\ntype: note\nblocked_by: [/epics/auth.md, /login.md]\n---\nx\n",
		"note.md":       "---\ntype: note\norigin: /epics/auth.md\n---\nprose mentions /epics/auth.md too\n",
		"deps.md":       "---\ntype: note\nrelated:\n  - /epics/auth.md\n  - /login.md\n---\nx\n",
	}
	read := func(idx *Index, rel string) string {
		b, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, filepath.FromSlash(rel)))
		return string(b)
	}

	// Default (no flag): body links move, frontmatter is left untouched.
	base := build(t, files)
	if _, err := base.Move("/epics/auth.md", "/epics/authn.md", false, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(base, "login.md"), "[see](/epics/authn.md)") {
		t.Errorf("body link should move: %q", read(base, "login.md"))
	}
	if !strings.Contains(read(base, "login.md"), "epic: /epics/auth.md") {
		t.Errorf("default move must NOT touch the epic: field: %q", read(base, "login.md"))
	}

	// --include-frontmatter: frontmatter values equal to the path move too...
	idx := build(t, files)
	res, err := idx.Move("/epics/auth.md", "/epics/authn.md", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(read(idx, "login.md"), "epic: /epics/authn.md") {
		t.Errorf("scalar field should move: %q", read(idx, "login.md"))
	}
	if !strings.Contains(read(idx, "oauth.md"), "blocked_by: [/epics/authn.md, /login.md]") {
		t.Errorf("flow-list element should move, others untouched: %q", read(idx, "oauth.md"))
	}
	if d := read(idx, "deps.md"); !strings.Contains(d, "- /epics/authn.md") || !strings.Contains(d, "- /login.md") {
		t.Errorf("block-list element should move, others untouched: %q", d)
	}
	if !strings.Contains(read(idx, "note.md"), "origin: /epics/authn.md") {
		t.Errorf("origin field should move: %q", read(idx, "note.md"))
	}
	// ...but a bare path in prose (not a link, not frontmatter) is left alone.
	if !strings.Contains(read(idx, "note.md"), "prose mentions /epics/auth.md too") {
		t.Errorf("prose path must NOT be rewritten: %q", read(idx, "note.md"))
	}
	// the result reports the frontmatter rewrites
	fm := 0
	for _, rw := range res.Rewrites {
		fm += rw.FrontmatterRefs
	}
	if fm != 4 {
		t.Errorf("expected 4 frontmatter rewrites (scalar + flow elem + origin + block elem), got %d (%+v)", fm, res.Rewrites)
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
	// the missing-type error points at the fix, so a non-entry file (a PROMPT.md,
	// a README) leads the user to wiki.toml `ignore` instead of a dead end.
	var typeErr string
	for _, is := range idx.Check() {
		if is.Level == "error" && is.Entry == "/notype.md" {
			typeErr = is.Msg
		}
	}
	if !strings.Contains(typeErr, "ignore") {
		t.Errorf("missing-type error should hint at wiki.toml `ignore`, got %q", typeErr)
	}
}

func TestCheckUnknownConfigKey(t *testing.T) {
	// An unrecognized wiki.toml key (here the pre-rename `skip`) is inert but must
	// be surfaced as a warning, not silently ignored.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "wiki.toml"),
		[]byte("spec=\"0.1\"\ntypes=[\"note\"]\nskip=[\"AGENTS.md\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("---\ntype: note\n---\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := bundle.Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := Build(b)
	if err != nil {
		t.Fatal(err)
	}
	if !hasWarning(idx.Check(), "wiki.toml", "unknown wiki.toml key: skip") {
		t.Errorf("unknown wiki.toml key should warn, got %+v", idx.Check())
	}
}

func TestCheckFilenameSpace(t *testing.T) {
	idx := build(t, map[string]string{"a b.md": "---\ntype: note\n---\nbody\n"})
	if !hasWarning(idx.Check(), "/a b.md", "space") {
		t.Errorf("a filename with a space should warn, got %+v", idx.Check())
	}
}

func TestAngleBracketLinkResolvesButNotNormalized(t *testing.T) {
	// Source in a subdir so the relative `../` stays in-bundle (a `../` from the
	// bundle root would now correctly resolve outside the bundle and be skipped).
	idx := build(t, map[string]string{
		"sub/page.md": "---\ntype: note\n---\n[s](<../a b.md>)\n",
		"a b.md":      "---\ntype: note\n---\nx\n",
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

func TestWikilinkGraph(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n[a](/a.md)\n",
		"a.md":     "---\ntype: note\n---\nSee [[b]] and [[sub/c|the C]].\n",
		"b.md":     "---\ntype: note\n---\nplain\n",
		"sub/c.md": "---\ntype: note\n---\nplain\n",
	})
	// a resolved wikilink is a real backlink (basename and path-qualified both)
	if bl := idx.Backlinks("/b.md"); len(bl) != 1 || bl[0].From != "/a.md" {
		t.Errorf("backlinks /b.md = %+v, want one from /a.md", bl)
	}
	if bl := idx.Backlinks("/sub/c.md"); len(bl) != 1 || bl[0].Text != "the C" {
		t.Errorf("backlinks /sub/c.md = %+v, want one with display 'the C'", bl)
	}
	// so wiki-linked targets are not orphans
	for _, o := range idx.Orphans() {
		if o.Path == "/b.md" || o.Path == "/sub/c.md" {
			t.Errorf("%s is wiki-linked, should not be an orphan", o.Path)
		}
	}
	// check flags the file once, with a count (both wikilinks in a.md)
	if !hasWarning(idx.Check(), "/a.md", "2 wikilink") {
		t.Errorf("check should warn once with a count of 2: %+v", idx.Check())
	}
	// an unresolvable wikilink is not a graph edge (no fake target)
	idx2 := build(t, map[string]string{"a.md": "---\ntype: note\n---\n[[ghost]]\n"})
	if len(idx2.Broken()) != 0 {
		t.Errorf("unresolved wikilink should not appear as a broken edge: %+v", idx2.Broken())
	}
	if !hasWarning(idx2.Check(), "/a.md", "1 wikilink") {
		t.Error("check should still warn on an unresolvable wikilink")
	}
}

// TestWikilinkAliasesAndEmbeds proves Build wires the two things asserted but not
// yet covered: `aliases:` frontmatter drives resolution end-to-end, and an
// embed `![[x]]` is treated as a graph edge (and flagged).
func TestWikilinkAliasesAndEmbeds(t *testing.T) {
	idx := build(t, map[string]string{
		"a.md":     "---\ntype: note\n---\n[[Nickname]] and ![[b]]\n",
		"people.md": "---\ntype: note\naliases: [Nickname, Nick]\n---\nreal file has no such basename\n",
		"b.md":     "---\ntype: note\n---\nplain\n",
	})
	// [[Nickname]] resolves via people.md's aliases frontmatter, not its filename
	if bl := idx.Backlinks("/people.md"); len(bl) != 1 || bl[0].From != "/a.md" {
		t.Errorf("alias backlink /people.md = %+v, want one from /a.md", bl)
	}
	// an embed ![[b]] is a real edge too
	if bl := idx.Backlinks("/b.md"); len(bl) != 1 || bl[0].From != "/a.md" {
		t.Errorf("embed backlink /b.md = %+v, want one from /a.md", bl)
	}
	// both (the alias link and the embed) are counted in the one per-file warning
	if !hasWarning(idx.Check(), "/a.md", "2 wikilink") {
		t.Errorf("check should warn once with a count of 2 (alias link + embed): %+v", idx.Check())
	}
}

// TestWikilinkSurvivesMove proves a relocation needs no wikilink rewrite: Move
// leaves the [[…]] text alone, and the link re-resolves to the moved file by
// basename on the next build.
func TestWikilinkSurvivesMove(t *testing.T) {
	idx := build(t, map[string]string{
		"a.md": "---\ntype: note\n---\nSee [[b]].\n",
		"b.md": "---\ntype: note\n---\nplain\n",
	})
	if _, err := idx.Move("/b.md", "/sub/b.md", false, false); err != nil {
		t.Fatalf("move: %v", err)
	}
	// Move must not have rewritten the [[b]] text in a.md.
	raw, err := os.ReadFile(filepath.Join(idx.Bundle.Dir, "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "[[b]]") {
		t.Errorf("move rewrote the wikilink; a.md = %q", raw)
	}
	// Re-scanning resolves [[b]] to the moved file by basename.
	b, err := bundle.Discover(idx.Bundle.Dir)
	if err != nil {
		t.Fatal(err)
	}
	idx2, err := Build(b)
	if err != nil {
		t.Fatal(err)
	}
	if bl := idx2.Backlinks("/sub/b.md"); len(bl) != 1 || bl[0].From != "/a.md" {
		t.Errorf("after move, backlinks /sub/b.md = %+v, want one from /a.md", bl)
	}
}

func TestConvertWikilinks(t *testing.T) {
	files := map[string]string{
		"a.md":     "---\ntype: note\n---\nSee [[b]], [[sub/c|the C]], [[b#Setup]], ![[b]], and [[ghost]].\n",
		"b.md":     "---\ntype: note\n---\n.\n",
		"sub/c.md": "---\ntype: note\n---\n.\n",
	}
	// dry run reports but writes nothing
	idx := build(t, files)
	before, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "a.md"))
	if fixes, err := idx.ConvertWikilinks(false); err != nil || len(fixes) == 0 {
		t.Fatalf("dry-run fixes=%d err=%v", len(fixes), err)
	}
	if after, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "a.md")); string(after) != string(before) {
		t.Error("dry run must not write")
	}

	// apply converts each form and leaves the unresolvable one
	idx = build(t, files)
	fixes, err := idx.ConvertWikilinks(true)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "a.md"))
	for _, want := range []string{"[b](/b.md)", "[the C](/sub/c.md)", "[b](/b.md#Setup)", "[[ghost]]"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("converted a.md missing %q:\n%s", want, got)
		}
	}
	// an embed becomes a plain reference, never a markdown image
	if strings.Contains(string(got), "![b](/b.md)") {
		t.Errorf("embed must convert to a plain link, not an image:\n%s", got)
	}
	// the unresolvable link is reported as a skip
	skip := false
	for _, f := range fixes {
		if f.Field == "wikilink-skip" && f.From == "[[ghost]]" {
			skip = true
		}
	}
	if !skip {
		t.Errorf("expected a wikilink-skip for [[ghost]]: %+v", fixes)
	}
	// re-scanning finds only the leftover [[ghost]] still flagged
	b, _ := bundle.Discover(idx.Bundle.Dir)
	idx2, _ := Build(b)
	if !hasWarning(idx2.Check(), "/a.md", "1 wikilink") {
		t.Errorf("after convert, only [[ghost]] should remain flagged: %+v", idx2.Check())
	}
}

func TestScalarTagCoercion(t *testing.T) {
	// a scalar tags: value is read as a one-element list (the coercion lives in
	// parse.Strings, the on-demand accessor that filtering and TagCounts share)
	idx := build(t, map[string]string{"a.md": "---\ntype: note\ntags: solo\n---\nbody\n"})
	if got := parse.Strings(idx.Entries[0].fm, "tags"); len(got) != 1 || got[0] != "solo" {
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

func hasError(issues []Issue, entry, substr string) bool {
	for _, is := range issues {
		if is.Level == "error" && is.Entry == entry && strings.Contains(is.Msg, substr) {
			return true
		}
	}
	return false
}

// A wiki.toml `ignore` entry excludes a meta file from the content index: not an
// entry (absent from list/search/graph), no conformance issue — yet a link *to*
// it still resolves on disk, so it is not broken.
func TestIgnoreExcludesFromIndex(t *testing.T) {
	idx := build(t, map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\nignore=[\"AGENTS.md\"]\n",
		"index.md":  "---\nokf_version: \"0.1\"\n---\n[a](/a.md) and the [manual](/AGENTS.md)\n",
		"a.md":      "---\ntype: note\n---\nx\n",
		"AGENTS.md": "# How to operate\n", // no type
	})
	if _, ok := idx.byPath["/AGENTS.md"]; ok {
		t.Errorf("ignored file must not be indexed as an entry")
	}
	if got := idx.Check(); len(got) != 0 {
		t.Errorf("ignored file must generate no conformance issues, got %+v", got)
	}
	// the link to /AGENTS.md must still resolve (FileExists stats disk), not broken
	for _, b := range idx.Broken() {
		if b.Target == "/AGENTS.md" {
			t.Errorf("a link to an ignored file must not be reported broken: %+v", b)
		}
	}
}

// Control: the same file WITHOUT ignore is flagged (missing type) and orphaned —
// proving the exemption is what silences it.
func TestIgnoreAbsentStillFlags(t *testing.T) {
	idx := build(t, map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\n",
		"index.md":  "---\nokf_version: \"0.1\"\n---\n[a](/a.md)\n",
		"a.md":      "---\ntype: note\n---\nx\n",
		"AGENTS.md": "# How to operate\n",
	})
	if !hasError(idx.Check(), "/AGENTS.md", "type") {
		t.Errorf("without ignore, AGENTS.md should error on missing type, got %+v", idx.Check())
	}
	orphaned := false
	for _, o := range idx.Orphans() {
		if o.Path == "/AGENTS.md" {
			orphaned = true
		}
	}
	if !orphaned {
		t.Errorf("without ignore, AGENTS.md should be an orphan, got %+v", idx.Orphans())
	}
}

// An `ignore` entry resolving outside the bundle silences the out-of-bundle advisory
// for every spelling that resolves to the same external file.
func TestIgnoreOutOfBundleAdvisory(t *testing.T) {
	idx := build(t, map[string]string{
		"wiki.toml":   "spec=\"0.1\"\ntypes=[\"note\"]\nignore=[\"../PRD.md\"]\n",
		"index.md":    "---\nokf_version: \"0.1\"\n---\n[prd](../PRD.md) [b](/b.md)\n", // ../PRD.md from root
		"sub/page.md": "---\ntype: note\n---\n[prd](../../PRD.md)\n",                   // ../../PRD.md from a subdir: same file
		"b.md":        "---\ntype: note\n---\nx\n",
	})
	for _, is := range idx.Check() {
		if strings.Contains(is.Msg, "out-of-bundle") {
			t.Errorf("ignore should silence the out-of-bundle advisory for both spellings, got %+v", is)
		}
	}
}

// wiki.toml `ignore_orphans` keeps parked/retired entries out of the orphan report
// while they stay indexed; a directory subtree covers everything under it.
func TestIgnoreOrphans(t *testing.T) {
	files := map[string]string{
		"index.md":        "---\nokf_version: \"0.1\"\n---\n# Board\n",
		"backlog/idea.md": "---\ntype: note\n---\nparked, nothing links here\n",
	}
	// Without ignore_orphans, the unlinked backlog entry is an orphan.
	if base := build(t, files); !orphanHas(base.Orphans(), "/backlog/idea.md") {
		t.Errorf("without ignore_orphans, /backlog/idea.md should be an orphan; got %+v", base.Orphans())
	}
	// With ignore_orphans covering backlog/**, it is not reported, but still indexed.
	files["wiki.toml"] = "spec=\"0.1\"\ntypes=[\"note\"]\nignore_orphans=[\"backlog/**\"]\n"
	idx := build(t, files)
	if orphanHas(idx.Orphans(), "/backlog/idea.md") {
		t.Errorf("ignore_orphans should exempt /backlog/idea.md; got %+v", idx.Orphans())
	}
	if _, ok := idx.byPath["/backlog/idea.md"]; !ok {
		t.Errorf("an ignore_orphans entry must still be indexed")
	}
}

// A wiki.toml `ignore` pattern may be a glob, not just an exact filename: it
// excludes every matching file from the index, while a plain name still matches
// one file.
func TestIgnoreGlob(t *testing.T) {
	idx := build(t, map[string]string{
		"wiki.toml":       "spec=\"0.1\"\ntypes=[\"note\"]\nignore=[\"drafts/**\", \"*.tmp.md\"]\n",
		"index.md":        "---\nokf_version: \"0.1\"\n---\n[a](/a.md)\n",
		"a.md":            "---\ntype: note\n---\nx\n",
		"scratch.tmp.md":  "no type, ignored by *.tmp.md\n",
		"drafts/one.md":   "no type, ignored by drafts/**\n",
		"drafts/sub/x.md": "no type, ignored by drafts/**\n",
	})
	for _, p := range []string{"/scratch.tmp.md", "/drafts/one.md", "/drafts/sub/x.md"} {
		if _, ok := idx.byPath[p]; ok {
			t.Errorf("%s should be excluded from the index by an ignore glob", p)
		}
	}
	if _, ok := idx.byPath["/a.md"]; !ok {
		t.Errorf("a non-ignored entry must stay indexed")
	}
	if got := idx.Check(); len(got) != 0 {
		t.Errorf("ignored files must generate no conformance issues, got %+v", got)
	}
}

// ignore_orphans accepts arbitrary globs, not only dir/** subtrees.
func TestIgnoreOrphansGlob(t *testing.T) {
	files := map[string]string{
		"wiki.toml":      "spec=\"0.1\"\ntypes=[\"note\"]\nignore_orphans=[\"**/scratch/*.md\"]\n",
		"index.md":       "---\nokf_version: \"0.1\"\n---\n# Board\n",
		"a/scratch/n.md": "---\ntype: note\n---\nparked deep\n",
		"loose.md":       "---\ntype: note\n---\nunlinked, a real orphan\n",
	}
	idx := build(t, files)
	if orphanHas(idx.Orphans(), "/a/scratch/n.md") {
		t.Errorf("a glob-matched entry should be exempt from orphans; got %+v", idx.Orphans())
	}
	if !orphanHas(idx.Orphans(), "/loose.md") {
		t.Errorf("an unmatched unlinked entry should still be an orphan; got %+v", idx.Orphans())
	}
}

func orphanHas(orphans []*Entry, path string) bool {
	for _, e := range orphans {
		if e.Path == path {
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
	hits := idx.Search("income", "", nil)
	if len(hits) != 2 || hits[0].Path != "/finance/income.md" || hits[1].Path != "/tech/notes.md" {
		t.Fatalf("hits = %+v", hits)
	}
	// income.md matches the title line + two body lines
	if hits[0].Matches != 3 {
		t.Errorf("income.md matches = %d want 3: %+v", hits[0].Matches, hits[0].Lines)
	}

	// filters narrow the candidate set
	if got := idx.Search("income", "", []PropFilter{{Key: "type", Value: "note"}}); len(got) != 2 {
		t.Errorf("type=note = %d want 2", len(got))
	}
	if got := idx.Search("income", "", []PropFilter{{Key: "type", Value: "concept"}}); len(got) != 0 {
		t.Errorf("type=concept = %d want 0", len(got))
	}
	if got := idx.Search("income", "finance/", nil); len(got) != 1 {
		t.Errorf("--prefix finance/ = %d want 1", len(got))
	}

	// frontmatter value (a tag) is searchable
	if got := idx.Search("money", "", nil); len(got) != 1 || got[0].Path != "/finance/income.md" {
		t.Errorf("tag-value search = %+v", got)
	}
	// no match
	if got := idx.Search("zzzznope", "", nil); len(got) != 0 {
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

	res, err := idx.Move("/a.md", "/sub/a.md", false, false)
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
	if _, err := idx.Move("/a.md", "/b.md", false, false); err != nil {
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
	if _, err := idx.Move("/hello.md", "/world.md", false, false); err != nil {
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
	res, err := idx.Move("/a.md", "/z.md", true, false)
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
		if _, err := idx.Move(tc.src, tc.dest, false, false); err == nil {
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
	if _, err := idx.Move("/a.md", "/b.md", false, false); err != nil {
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
