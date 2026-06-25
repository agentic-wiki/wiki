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
