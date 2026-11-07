//go:build unix

package main

import (
	"os"
	"strings"
	"testing"
)

// A rewrite that fails after the rename leaves the file moved and the other
// rewrites written. The move result and any warning are still printed, then
// one `wiki:` line per file it couldn't rewrite, and the exit is 2. Here those
// files' folder is read-only, which only Unix enforces for a new file, and
// only for a user other than root.
func TestMoveRewriteFailsAfterRename(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	writeFiles(t, map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\n",
		"a.md":      "---\ntype: note\n---\n# A\n",
		"ref.md":    "---\ntype: note\n---\n# Ref\nsee [a](a.md)\n",
		"twice.md":  "---\ntype: note\nrelated: /a.md\nrelated: /a.md\n---\n# Twice\n",
	})
	locked := map[string]string{
		"locked/one.md": "---\ntype: note\n---\n# One\nsee [a](../a.md)\n",
		"locked/two.md": "---\ntype: note\n---\n# Two\nsee [a](../a.md)\n",
	}
	if err := os.Mkdir("locked", 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range locked {
		if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Chmod("locked", 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod("locked", 0o755) })

	out, errOut, code := captureAll(t, func() int { return cmdMove([]string{"a.md", "b.md", "--include-frontmatter"}) })
	if code != 2 || !strings.HasPrefix(out, "moved /a.md -> /b.md\n") {
		t.Errorf("exit %d, stdout %q; want 2 and the move result", code, out)
	}
	lines := strings.Split(strings.TrimSuffix(errOut, "\n"), "\n")
	wants := []string{
		`wiki: warning: frontmatter of /twice.md has key "related" twice`,
		"wiki: moved /a.md to /b.md, but /locked/one.md was not rewritten",
		"wiki: moved /a.md to /b.md, but /locked/two.md was not rewritten",
	}
	if len(lines) != len(wants) {
		t.Fatalf("stderr has %d lines, want %d:\n%s", len(lines), len(wants), errOut)
	}
	for i, want := range wants {
		if !strings.HasPrefix(lines[i], want) {
			t.Errorf("stderr line %d = %q, want it to start %q", i+1, lines[i], want)
		}
	}
	if _, err := os.Stat("b.md"); err != nil {
		t.Errorf("not moved: %v", err)
	}
	if got, _ := os.ReadFile("ref.md"); string(got) != "---\ntype: note\n---\n# Ref\nsee [a](./b.md)\n" {
		t.Errorf("ref.md = %q, want it rewritten", got)
	}
	for name, text := range locked {
		if got, _ := os.ReadFile(name); string(got) != text {
			t.Errorf("%s = %q, want it as it was", name, got)
		}
	}
}
