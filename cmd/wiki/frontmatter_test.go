package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/index"
)

func TestCmdCheckRejectsMalformedFrontmatter(t *testing.T) {
	cases := map[string]string{
		"unclosed purpose quote": "---\ntype: note\npurpose: \"Keep historical choices.\n---\n# Body\n",
		"unclosed title quote":   "---\ntype: note\ntitle: \"Unclosed\n---\n# Body\n",
		"unclosed list":          "---\ntype: note\ntags: [history\n---\n# Body\n",
		"duplicate key":          "---\ntype: note\ntitle: First\ntitle: Second\n---\n# Body\n",
		"missing closing fence":  "---\ntype: note\ntitle: Unclosed\n# Body\n",
		"nonmapping header":      "---\n- one\n- two\n---\n# Body\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			t.Chdir(writeBundle(t))
			if err := os.WriteFile("guide.md", []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{nil, {"--format", "json"}, {"--fix"}} {
				out, code := capture(t, func() int { return cmdCheck(args) })
				if code != 1 || !strings.Contains(out, "invalid frontmatter") || !strings.Contains(out, "/guide.md") {
					t.Errorf("check %v: exit=%d output=%s", args, code, out)
				}
			}
			got, err := os.ReadFile("guide.md")
			if err != nil || string(got) != content {
				t.Fatalf("check rewrote malformed input: %q, %v", got, err)
			}
		})
	}
}

func TestCmdCheckReportsEveryMalformedHeader(t *testing.T) {
	t.Chdir(writeBundle(t))
	for _, name := range []string{"guide.md", "flat.md", "index.md"} {
		if err := os.WriteFile(name, []byte("---\ntype: note\ntitle: \"Unclosed\n---\n# Body\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, code := capture(t, func() int { return cmdCheck([]string{"--format", "json"}) })
	if code != 1 {
		t.Fatalf("exit=%d, want 1: %s", code, out)
	}
	var issues []index.Issue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, issue := range issues {
		if issue.Level == "error" && strings.Contains(issue.Msg, "invalid frontmatter") {
			seen[issue.Entry] = true
		}
	}
	if len(seen) != 3 {
		t.Fatalf("want all 3 malformed files, got %+v", issues)
	}
}

func TestCmdCheckFixLeavesMalformedRootHeader(t *testing.T) {
	t.Chdir(writeBundle(t))
	const content = "---\nokf_version: \"Unclosed\n---\n# Home\n"
	if err := os.WriteFile("index.md", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := capture(t, func() int { return cmdCheck([]string{"--fix"}) })
	if code != 1 || !strings.Contains(out, "/index.md: invalid frontmatter") {
		t.Fatalf("exit=%d output=%s", code, out)
	}
	got, err := os.ReadFile("index.md")
	if err != nil || string(got) != content {
		t.Fatalf("check --fix changed malformed root: %q, %v", got, err)
	}
}

func TestCmdCheckAcceptsValidFrontmatter(t *testing.T) {
	t.Chdir(writeBundle(t))
	const content = "---\r\ntype: note\r\ntitle: An embedded \"quote in plain text\r\nquoted: \"A \\\"quote\\\" # kept\" # comment\r\ntags:\r\n  - one\r\n  - two\r\nsummary: >\r\n  First line\r\n  second line\r\nmeta: &meta\r\n  key: value\r\ncopy: *meta\r\n---\r\n# Body\r\n"
	if err := os.WriteFile("guide.md", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := capture(t, func() int { return cmdCheck(nil) })
	if code != 0 || out != "ok: no issues found\n" {
		t.Fatalf("valid YAML rejected: exit=%d output=%s", code, out)
	}
}
