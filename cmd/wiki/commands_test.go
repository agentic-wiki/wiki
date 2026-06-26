package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	write("index.md", "---\ntype: index\nokf_version: \"0.1\"\n---\n# Home\n[g](/guide.md)\n")
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
	if out, _ := capture(t, func() int { return cmdSearch([]string{"--type", "index", "Home"}) }); !strings.Contains(out, "/index.md") {
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
	// guide.md has no outgoing links -> exit 1
	if _, code := capture(t, func() int { return cmdLinks([]string{"guide.md"}) }); code != 1 {
		t.Errorf("links none exit=%d want 1", code)
	}
	// backlinks: guide.md <- index.md
	if out, code := capture(t, func() int { return cmdBacklinks([]string{"guide.md"}) }); code != 0 || !strings.Contains(out, "/index.md") {
		t.Errorf("backlinks: %q (%d)", out, code)
	}
	// flat.md has no backlinks -> exit 1
	if _, code := capture(t, func() int { return cmdBacklinks([]string{"flat.md"}) }); code != 1 {
		t.Errorf("backlinks none exit=%d want 1", code)
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
	t.Chdir(writeBundle(t))
	cases := []struct {
		name string
		run  func() int
		want int
	}{
		{"status", func() int { return cmdStatus(nil) }, 0},
		{"list all", func() int { return cmdList(nil) }, 0},
		{"list empty filter", func() int { return cmdList([]string{"--type", "nope"}) }, 1},
		{"tasks", func() int { return cmdTasks(nil) }, 0},                   // guide.md has an open checkbox
		{"unresolved (clean)", func() int { return cmdUnresolved(nil) }, 1}, // no broken links
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
