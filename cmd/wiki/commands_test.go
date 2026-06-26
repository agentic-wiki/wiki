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
	write("guide.md", "---\ntype: note\ntitle: Guide\n---\n# Guide\nintro text\n## Setup\nstep one\n### Detail\nfine print\n## Usage\nrun it\n")
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
