package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeBundle creates a minimal bundle in a temp dir and returns its path.
func writeBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("wiki.toml", "spec=\"0.1\"\ntypes=[\"note\"]\n")
	write("index.md", "---\nokf_version: \"0.1\"\n---\n# Home\n[g](/guide.md)\n")
	write("guide.md", "---\ntype: note\ntitle: Guide\n---\n# Guide\nintro text\n## Setup\nstep one\n### Detail\nfine print\n## Usage\nrun it\n- [ ] try the CLI\n")
	write("flat.md", "---\ntype: note\n---\nno headings here\n")
	return dir
}

// capture runs f with os.Stdout redirected and returns its stdout and exit code.
func capture(t *testing.T, f func() int) (string, int) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	code := f()
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out), code
}

func TestCmdRead(t *testing.T) {
	t.Chdir(writeBundle(t))

	out, code := capture(t, func() int { return cmdRead([]string{"guide.md"}) })
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !strings.Contains(out, "intro text") || !strings.Contains(out, "## Setup") {
		t.Errorf("body missing content: %q", out)
	}
	if strings.Contains(out, "title: Guide") || strings.Contains(out, "type: note") {
		t.Errorf("frontmatter leaked into read: %q", out)
	}

	if _, code := capture(t, func() int { return cmdRead([]string{"/guide.md"}) }); code != 0 {
		t.Errorf("read by absolute path exit=%d", code)
	}
	if _, code := capture(t, func() int { return cmdRead([]string{"nope.md"}) }); code != 2 {
		t.Errorf("read missing exit=%d want 2", code)
	}
	if _, code := capture(t, func() int { return cmdRead(nil) }); code != 2 {
		t.Errorf("read no-arg exit=%d want 2", code)
	}

	out, code = capture(t, func() int { return cmdRead([]string{"--format", "json", "guide.md"}) })
	if code != 0 || !strings.Contains(out, `"body"`) || !strings.Contains(out, `"path": "/guide.md"`) {
		t.Errorf("json read: %q (code %d)", out, code)
	}

	// flag AFTER the positional must still take effect (parseWithArg)
	out, code = capture(t, func() int { return cmdRead([]string{"guide.md", "--format", "json"}) })
	if code != 0 || !strings.Contains(out, `"body"`) {
		t.Errorf("flag-after-file read: %q (code %d)", out, code)
	}
}

func TestCmdOutline(t *testing.T) {
	t.Chdir(writeBundle(t))

	out, code := capture(t, func() int { return cmdOutline([]string{"guide.md"}) })
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if want := "Guide\n  Setup\n    Detail\n  Usage\n"; out != want {
		t.Errorf("outline=\n%q\nwant\n%q", out, want)
	}

	out, code = capture(t, func() int { return cmdOutline([]string{"--format", "json", "guide.md"}) })
	if code != 0 || !strings.Contains(out, `"level": 2`) || !strings.Contains(out, `"text": "Setup"`) {
		t.Errorf("json outline: %q", out)
	}

	// no headings: empty text, exit 0, and json shows [] (not null)
	if out, code := capture(t, func() int { return cmdOutline([]string{"flat.md"}) }); out != "" || code != 0 {
		t.Errorf("flat outline=%q code=%d, want empty/0", out, code)
	}
	if out, _ := capture(t, func() int { return cmdOutline([]string{"--format", "json", "flat.md"}) }); !strings.Contains(out, `"headings": []`) {
		t.Errorf("flat json should carry an empty headings array: %q", out)
	}

	if _, code := capture(t, func() int { return cmdOutline([]string{"nope.md"}) }); code != 2 {
		t.Errorf("outline missing exit=%d want 2", code)
	}
}

func TestCmdSearch(t *testing.T) {
	t.Chdir(writeBundle(t))

	// default: lists matching entries
	out, code := capture(t, func() int { return cmdSearch([]string{"setup"}) })
	if code != 0 || !strings.Contains(out, "/guide.md") {
		t.Errorf("search setup: %q (code %d)", out, code)
	}

	// --lines: grep-style line with a file-relative line number
	out, code = capture(t, func() int { return cmdSearch([]string{"--lines", "step"}) })
	if code != 0 || !strings.Contains(out, "/guide.md:8: step one") {
		t.Errorf("search --lines: %q", out)
	}

	// flag AFTER the query must still take effect (parseWithArg)
	out, code = capture(t, func() int { return cmdSearch([]string{"step", "--lines"}) })
	if code != 0 || !strings.Contains(out, "step one") {
		t.Errorf("flag-after-query search: %q", out)
	}

	// --type filter
	if out, _ := capture(t, func() int { return cmdSearch([]string{"--type", "note", "Setup"}) }); !strings.Contains(out, "/guide.md") {
		t.Errorf("type-filter search: %q", out)
	}

	// no match -> exit 1; no query -> exit 2
	if _, code := capture(t, func() int { return cmdSearch([]string{"zzzznope"}) }); code != 1 {
		t.Errorf("no-match exit=%d want 1", code)
	}
	if _, code := capture(t, func() int { return cmdSearch(nil) }); code != 2 {
		t.Errorf("no-query exit=%d want 2", code)
	}

	// json carries the match count
	out, _ = capture(t, func() int { return cmdSearch([]string{"--format", "json", "setup"}) })
	if !strings.Contains(out, `"matches"`) {
		t.Errorf("json search: %q", out)
	}
}

func TestCmdLinkGraph(t *testing.T) {
	t.Chdir(writeBundle(t))

	// links: index.md -> /guide.md
	if out, code := capture(t, func() int { return cmdLinks([]string{"/index.md"}) }); code != 0 || !strings.Contains(out, "/guide.md") {
		t.Errorf("links: %q (%d)", out, code)
	}
	// guide.md has no outgoing links: an empty listing, exit 0 (like ls)
	if _, code := capture(t, func() int { return cmdLinks([]string{"guide.md"}) }); code != 0 {
		t.Errorf("links none exit=%d want 0", code)
	}
	// backlinks: guide.md <- index.md
	if out, code := capture(t, func() int { return cmdBacklinks([]string{"guide.md"}) }); code != 0 || !strings.Contains(out, "/index.md") {
		t.Errorf("backlinks: %q (%d)", out, code)
	}
	// flat.md has no backlinks: an empty listing, exit 0 (like ls)
	if _, code := capture(t, func() int { return cmdBacklinks([]string{"flat.md"}) }); code != 0 {
		t.Errorf("backlinks none exit=%d want 0", code)
	}
	// missing target -> exit 2
	if _, code := capture(t, func() int { return cmdLinks([]string{"nope.md"}) }); code != 2 {
		t.Errorf("links missing exit=%d want 2", code)
	}
}

func TestCmdMove(t *testing.T) {
	t.Chdir(writeBundle(t))

	// dry-run previews and writes nothing
	if out, code := capture(t, func() int { return cmdMove([]string{"--dry-run", "/guide.md", "/docs/guide.md"}) }); code != 0 || !strings.Contains(out, "would move") {
		t.Errorf("dry-run move: %q (%d)", out, code)
	}
	if _, err := os.Stat("docs/guide.md"); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote the file")
	}

	// real move relocates the file and rewrites the incoming link
	if _, code := capture(t, func() int { return cmdMove([]string{"/guide.md", "/docs/guide.md"}) }); code != 0 {
		t.Fatalf("move exit %d", code)
	}
	if _, err := os.Stat("docs/guide.md"); err != nil {
		t.Errorf("file not moved: %v", err)
	}
	if out, _ := capture(t, func() int { return cmdRead([]string{"/index.md"}) }); !strings.Contains(out, "/docs/guide.md") {
		t.Errorf("incoming link not rewritten: %q", out)
	}

	// move also renames (same dir, new basename) — no separate command needed
	if _, code := capture(t, func() int { return cmdMove([]string{"/docs/guide.md", "/docs/manual.md"}) }); code != 0 {
		t.Errorf("rename-via-move exit %d", code)
	}
	if _, err := os.Stat("docs/manual.md"); err != nil {
		t.Errorf("rename-via-move failed: %v", err)
	}

	// errors: missing src, one arg
	if _, code := capture(t, func() int { return cmdMove([]string{"/nope.md", "/x.md"}) }); code != 2 {
		t.Errorf("move missing src exit=%d want 2", code)
	}
	if _, code := capture(t, func() int { return cmdMove([]string{"/index.md"}) }); code != 2 {
		t.Errorf("move one-arg exit=%d want 2", code)
	}
	// refuse overwriting an existing destination
	if _, code := capture(t, func() int { return cmdMove([]string{"/index.md", "/flat.md"}) }); code != 2 {
		t.Errorf("move onto existing dest exit=%d want 2", code)
	}
}

func TestCmdInit(t *testing.T) {
	t.Chdir(t.TempDir())

	if _, code := capture(t, func() int { return cmdInit(nil) }); code != 0 {
		t.Fatalf("init exit %d", code)
	}
	for _, f := range []string{"wiki.toml", ".gitignore", "index.md", "notes/welcome.md"} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	if _, err := os.Stat("gitignore"); !os.IsNotExist(err) {
		t.Errorf("scaffold leaked a bare 'gitignore'")
	}
	// a freshly-init'd bundle must pass check
	if _, code := capture(t, func() int { return cmdCheck(nil) }); code != 0 {
		t.Errorf("fresh bundle should pass check, exit=%d", code)
	}
	// re-init into the now-non-empty dir is refused without --force
	if _, code := capture(t, func() int { return cmdInit(nil) }); code != 2 {
		t.Errorf("re-init without --force exit=%d want 2", code)
	}
	if _, code := capture(t, func() int { return cmdInit([]string{"--force"}) }); code != 0 {
		t.Errorf("init --force exit=%d want 0", code)
	}
}

func TestQueryCommands(t *testing.T) {
	// Enumeration and diagnostic commands return 0 even on an empty result, like
	// ls/find; only search (grep) and check (lint) use exit 1. See TestCmdSearch.
	t.Chdir(writeBundle(t))
	cases := []struct {
		name string
		run  func() int
		want int
	}{
		{"status", func() int { return cmdStatus(nil) }, 0},
		{"list all", func() int { return cmdList(nil) }, 0},
		{"list empty filter", func() int { return cmdList([]string{"--type", "nope"}) }, 0}, // empty listing is still exit 0
		{"tasks", func() int { return cmdTasks(nil) }, 0},                   // guide.md has an open checkbox
		{"unresolved (clean)", func() int { return cmdUnresolved(nil) }, 0}, // no broken links: a clean diagnostic, exit 0
		{"orphans", func() int { return cmdOrphans(nil) }, 0},               // flat.md is an orphan
		{"check (clean)", func() int { return cmdCheck(nil) }, 0},
	}
	for _, tc := range cases {
		if _, code := capture(t, tc.run); code != tc.want {
			t.Errorf("%s: exit=%d want %d", tc.name, code, tc.want)
		}
	}
}

func TestRun(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"no args", nil, 2},
		{"unknown command", []string{"frobnicate"}, 2},
		{"help", []string{"help"}, 0},
		{"version", []string{"version"}, 0},
	} {
		if _, code := capture(t, func() int { return run(tc.args) }); code != tc.want {
			t.Errorf("run(%v) = %d, want %d", tc.args, code, tc.want)
		}
	}
}

func TestCmdCheckFix(t *testing.T) {
	dir := writeBundle(t)
	// Introduce okf_version drift on the bundle-root index.md.
	if err := os.WriteFile(filepath.Join(dir, "index.md"),
		[]byte("---\nokf_version: \"0.2\"\n---\n# Home\n[g](/guide.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	// Plain check flags the drift (a warning, so exit 0).
	if out, _ := capture(t, func() int { return cmdCheck(nil) }); !strings.Contains(out, "okf_version") {
		t.Fatalf("check should flag drift: %q", out)
	}

	// check --fix repairs it and reports what changed.
	out, code := capture(t, func() int { return cmdCheck([]string{"--fix"}) })
	if code != 0 {
		t.Errorf("check --fix exit=%d want 0", code)
	}
	if !strings.Contains(out, "fixed") || !strings.Contains(out, "okf_version") {
		t.Errorf("check --fix should report the fix: %q", out)
	}

	// The bundle is now clean.
	if out, _ := capture(t, func() int { return cmdCheck(nil) }); !strings.Contains(out, "ok: no issues found") {
		t.Errorf("post-fix check not clean: %q", out)
	}

	// JSON --fix surfaces a fixed[] key even when there is nothing left to fix.
	if out, _ := capture(t, func() int { return cmdCheck([]string{"--fix", "--format", "json"}) }); !strings.Contains(out, `"fixed"`) {
		t.Errorf("json --fix missing fixed key: %q", out)
	}
}

func TestCmdTidy(t *testing.T) {
	dir := writeBundle(t)
	// a relative link in index.md, and a spaced filename to slug
	if err := os.WriteFile(filepath.Join(dir, "index.md"),
		[]byte("---\nokf_version: \"0.1\"\n---\n# Home\n[g](guide.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a note.md"), []byte("---\ntype: note\n---\nx\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	// bare = preview both categories, write nothing
	out, code := capture(t, func() int { return cmdTidy(nil) })
	if code != 0 || !strings.Contains(out, "would") || !strings.Contains(out, "/guide.md") || !strings.Contains(out, "a-note.md") {
		t.Errorf("bare tidy preview: %q (code %d)", out, code)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "index.md")); strings.Contains(string(b), "(/guide.md)") {
		t.Errorf("bare tidy must not write")
	}
	if _, err := os.Stat(filepath.Join(dir, "a note.md")); err != nil {
		t.Errorf("bare tidy must not rename")
	}

	// --slug applies the rename
	if _, code := capture(t, func() int { return cmdTidy([]string{"--slug"}) }); code != 0 {
		t.Errorf("tidy --slug exit=%d", code)
	}
	if _, err := os.Stat(filepath.Join(dir, "a-note.md")); err != nil {
		t.Errorf("--slug should rename a note.md -> a-note.md")
	}

	// --links applies link normalization
	if _, code := capture(t, func() int { return cmdTidy([]string{"--links"}) }); code != 0 {
		t.Errorf("tidy --links exit=%d", code)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "index.md")); !strings.Contains(string(b), "[g](/guide.md)") {
		t.Errorf("--links should normalize: %s", b)
	}

	// nothing left -> ok message; json is an array
	if out, _ := capture(t, func() int { return cmdTidy(nil) }); !strings.Contains(out, "nothing to tidy") {
		t.Errorf("no-op tidy: %q", out)
	}
	if out, _ := capture(t, func() int { return cmdTidy([]string{"--format", "json"}) }); !strings.Contains(out, "[") {
		t.Errorf("json tidy missing array: %q", out)
	}
}

func TestCmdTagsProperties(t *testing.T) {
	dir := writeBundle(t)
	// give the entries tags/status so there is something to introspect
	if err := os.WriteFile(filepath.Join(dir, "guide.md"),
		[]byte("---\ntype: note\ntitle: Guide\nstatus: open\ntags: [docs, x]\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "flat.md"),
		[]byte("---\ntype: note\nstatus: done\ntags: [x]\n---\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	// tags --counts --sort=count: x (2) sorts before docs (1)
	out, code := capture(t, func() int { return cmdTags([]string{"--counts", "--sort=count"}) })
	if code != 0 {
		t.Fatalf("tags exit=%d", code)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "x") {
		t.Errorf("tags --sort=count: x (count 2) should be first, got %q", out)
	}

	// tags bare: just names, no counts
	out, _ = capture(t, func() int { return cmdTags(nil) })
	if !strings.Contains(out, "docs") || strings.ContainsAny(out, "0123456789") {
		t.Errorf("bare tags should be names only: %q", out)
	}

	// properties --counts includes the reserved-root okf_version and the shared keys
	out, _ = capture(t, func() int { return cmdProperties([]string{"--counts"}) })
	for _, k := range []string{"okf_version", "status", "type", "tags"} {
		if !strings.Contains(out, k) {
			t.Errorf("properties missing %q: %q", k, out)
		}
	}

	// property status: open and done, one each
	out, _ = capture(t, func() int { return cmdProperty([]string{"status", "--counts"}) })
	if !strings.Contains(out, "open") || !strings.Contains(out, "done") {
		t.Errorf("property status: %q", out)
	}

	// property type as json: note appears twice
	out, _ = capture(t, func() int { return cmdProperty([]string{"type", "--format", "json"}) })
	if !strings.Contains(out, `"name": "note"`) || !strings.Contains(out, `"count": 2`) {
		t.Errorf("property type json: %q", out)
	}

	// unknown key -> no values -> exit 0 (an empty listing, like ls)
	if _, code := capture(t, func() int { return cmdProperty([]string{"zzz"}) }); code != 0 {
		t.Errorf("property zzz exit=%d, want 0", code)
	}
	// missing name -> usage, exit 2
	if _, code := capture(t, func() int { return cmdProperty(nil) }); code != 2 {
		t.Errorf("property (no name) exit=%d, want 2", code)
	}
	// prefix with no entries -> exit 0 (an empty listing, like ls)
	if _, code := capture(t, func() int { return cmdTags([]string{"--prefix", "sub/"}) }); code != 0 {
		t.Errorf("tags --prefix sub/ exit=%d, want 0", code)
	}
}

func TestGlobalRoot(t *testing.T) {
	t.Cleanup(func() { rootDir = "" }) // global flag state; keep other tests independent
	t.Chdir(t.TempDir())               // cwd has no bundle
	cwd, _ := os.Getwd()
	b := writeBundle(t)

	// --root operates on <dir> even though cwd has no bundle...
	if _, code := capture(t, func() int { return run([]string{"--root", b, "status"}) }); code != 0 {
		t.Errorf("--root status exit=%d want 0", code)
	}
	// ...and unlike git -C it does not change the working directory.
	if now, _ := os.Getwd(); now != cwd {
		t.Errorf("--root changed cwd to %q, want %q (no chdir)", now, cwd)
	}
	if _, code := capture(t, func() int { return run([]string{"--root=" + b, "check"}) }); code != 0 {
		t.Errorf("--root= check exit=%d want 0", code)
	}
	// a dir with no bundle above it, and a missing arg, both error
	if _, code := capture(t, func() int { return run([]string{"--root", t.TempDir(), "status"}) }); code != 2 {
		t.Errorf("--root no-bundle exit=%d want 2", code)
	}
	if _, code := capture(t, func() int { return run([]string{"--root"}) }); code != 2 {
		t.Errorf("--root no-arg exit=%d want 2", code)
	}
}

// inOrder reports whether subs each appear in out, in the given order.
func inOrder(out string, subs ...string) bool {
	last := -1
	for _, s := range subs {
		i := strings.Index(out, s)
		if i <= last {
			return false
		}
		last = i
	}
	return true
}

func TestCmdListSort(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("wiki.toml", "spec=\"0.1\"\ntypes=[\"note\"]\n")
	write("old.md", "---\ntype: note\ntimestamp: 2024-01-01\n---\nx\n")
	write("new.md", "---\ntype: note\ntimestamp: 2026-06-01\n---\nx\n")
	write("mid.md", "---\ntype: note\ntimestamp: 2025-03-15\n---\nx\n")
	t.Chdir(dir)

	// --sort=timestamp orders newest-first
	out, code := capture(t, func() int { return cmdList([]string{"--sort=timestamp"}) })
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !inOrder(out, "/new.md", "/mid.md", "/old.md") {
		t.Errorf("timestamp sort should be newest-first, got:\n%s", out)
	}

	// --reverse flips it to oldest-first (the grooming pass)
	if out, _ := capture(t, func() int { return cmdList([]string{"--sort=timestamp", "--reverse"}) }); !inOrder(out, "/old.md", "/mid.md", "/new.md") {
		t.Errorf("reversed timestamp sort should be oldest-first, got:\n%s", out)
	}

	// default sort is by path (alphabetical)
	if out, _ := capture(t, func() int { return cmdList(nil) }); !inOrder(out, "/mid.md", "/new.md", "/old.md") {
		t.Errorf("default sort should be by path, got:\n%s", out)
	}

	// an unknown --sort value is a usage error
	if _, code := capture(t, func() int { return cmdList([]string{"--sort=bogus"}) }); code != 2 {
		t.Errorf("unknown --sort exit=%d want 2", code)
	}
}

func TestCmdListSortMtimeFallback(t *testing.T) {
	dir := t.TempDir()
	writeAt := func(rel, content string, mod time.Time) {
		p := filepath.Join(dir, rel)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if !mod.IsZero() {
			if err := os.Chtimes(p, mod, mod); err != nil {
				t.Fatal(err)
			}
		}
	}
	writeAt("wiki.toml", "spec=\"0.1\"\ntypes=[\"note\"]\n", time.Time{})
	// neither carries a frontmatter timestamp, so ordering falls back to mtime
	writeAt("older.md", "---\ntype: note\n---\nx\n", time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
	writeAt("newer.md", "---\ntype: note\n---\nx\n", time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC))
	t.Chdir(dir)

	out, code := capture(t, func() int { return cmdList([]string{"--sort=timestamp"}) })
	if code != 0 {
		t.Fatalf("exit=%d", code)
	}
	if !inOrder(out, "/newer.md", "/older.md") {
		t.Errorf("mtime fallback should put the newer mtime first, got:\n%s", out)
	}
}

func TestCmdTable(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		if err := os.WriteFile(filepath.Join(dir, rel), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("wiki.toml", "spec=\"0.1\"\ntypes=[\"note\",\"dataset\"]\n")
	write("one.md", "---\ntype: dataset\n---\n| date | amt |\n|---|---|\n| 2026-01 | 100 |\n")
	write("multi.md", "---\ntype: note\n---\n| a | b |\n|---|---|\n| 1 | 2 |\n\ntext\n\n| c | d |\n|---|---|\n| 3 | 4 |\n")
	write("none.md", "---\ntype: note\n---\njust prose\n")
	t.Chdir(dir)

	// a lone table needs no flag: csv
	out, code := capture(t, func() int { return cmdTable([]string{"--format", "csv", "one.md"}) })
	if code != 0 || !strings.Contains(out, "date,amt") || !strings.Contains(out, "2026-01,100") {
		t.Errorf("csv table: %q (code %d)", out, code)
	}

	// json carries the rows
	if out, _ := capture(t, func() int { return cmdTable([]string{"--format", "json", "one.md"}) }); !strings.Contains(out, `"date"`) || !strings.Contains(out, "2026-01") {
		t.Errorf("json table: %q", out)
	}

	// several tables without --n: refuse rather than guess (exit 2)
	if _, code := capture(t, func() int { return cmdTable([]string{"multi.md"}) }); code != 2 {
		t.Errorf("multi-table without --n should exit 2, got %d", code)
	}

	// --n selects (opt-in)
	out, code = capture(t, func() int { return cmdTable([]string{"--n", "2", "--format", "csv", "multi.md"}) })
	if code != 0 || !strings.Contains(out, "c,d") || !strings.Contains(out, "3,4") {
		t.Errorf("--n 2: %q (code %d)", out, code)
	}

	// --n out of range, no table, and missing file are all errors
	if _, code := capture(t, func() int { return cmdTable([]string{"--n", "9", "multi.md"}) }); code != 2 {
		t.Errorf("--n out of range should exit 2, got %d", code)
	}
	if _, code := capture(t, func() int { return cmdTable([]string{"none.md"}) }); code != 2 {
		t.Errorf("no table should exit 2, got %d", code)
	}
	if _, code := capture(t, func() int { return cmdTable([]string{"nope.md"}) }); code != 2 {
		t.Errorf("missing file should exit 2, got %d", code)
	}
}
