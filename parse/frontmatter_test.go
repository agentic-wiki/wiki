package parse

import (
	"strings"
	"testing"
)

func TestValidateFrontmatter(t *testing.T) {
	valid := map[string]string{
		"absent":                        "# Body\n\npurpose: \"This is prose, not YAML\n",
		"empty":                         "---\n---\n# Body\n",
		"comments only":                 "---\n# Metadata will follow later.\n---\n",
		"quoted values":                 "---\ntitle: \"A: B # C\" # comment\ntags: [one, \"two, three\"]\n---\n",
		"embedded quote in bare scalar": "---\ntitle: An unpaired \"quote inside plain text\n---\n",
		"escaped quotes":                "---\ntitle: \"A \\\"quote\\\" # kept\" # comment\n---\n",
		"single quote escape":           "---\ntitle: 'It''s valid'\n---\n",
		"multiline quoted scalar":       "---\ntitle: \"First line\n  second line\"\n---\n",
		"block values":                  "---\ntags:\n  - one\n  - two\nliteral: |\n  --- is text\n  another line\nfolded: >\n  first\n  second\n---\n",
		"nested maps and aliases":       "---\nmeta: &meta\n  key: value\ncopy: *meta\n---\n",
		"complex nested keys":           "---\nmeta:\n  ? [a, b]\n  : value\n---\n",
		"recursive alias":               "---\nmeta: &meta {self: *meta}\n---\n",
		"numbers and booleans":          "---\nversion: 0.1\nenabled: true\n---\n",
		"crlf":                          "---\r\ntitle: \"Hello\"\r\n---\r\n# Body\r\n",
		"closer at eof":                 "---\ntype: note\n---",
	}
	for name, content := range valid {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFrontmatter(content); err != nil {
				t.Fatalf("valid frontmatter rejected: %v", err)
			}
		})
	}
	invalid := map[string]string{
		"unclosed double quote":        "---\ntype: note\ntitle: \"Unclosed\n---\n",
		"unclosed single quote":        "---\ntitle: 'Unclosed\n---\n",
		"unclosed list":                "---\ntags: [one, two\n---\n",
		"unclosed flow mapping":        "---\nmeta: {key: value\n---\n",
		"duplicate key":                "---\ntitle: First\ntitle: Second\n---\n",
		"nested duplicate key":         "---\nmeta:\n  key: one\n  key: two\n---\n",
		"bad indentation":              "---\nmeta:\n  key: value\n other: value\n---\n",
		"sequence header":              "---\n- one\n- two\n---\n",
		"scalar header":                "---\njust text\n---\n",
		"null header":                  "---\nnull\n---\n",
		"missing closer":               "---\ntype: note\n",
		"opener only":                  "---",
		"closer prefix is not a fence": "---\ntype: note\n---suffix\n",
		"content after document end":   "---\ntype: note\n...\ntitle: Second\n---\n",
	}
	for name, content := range invalid {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFrontmatter(content); err == nil {
				t.Fatal("malformed frontmatter accepted")
			}
		})
	}
}

func TestValidateFrontmatterFileLineNumbers(t *testing.T) {
	const content = "---\ntype: note\ntitle: First\ntitle: Second\n---\n"
	err := ValidateFrontmatter(content)
	if err == nil || !strings.Contains(err.Error(), "line 4") || !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("want file-relative lines for both duplicate keys, got %v", err)
	}
}

func TestFrontmatterSharedFences(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		for _, content := range []string{
			"---\n---\n# Body\n",
			"---\ntype: note\n---\n# Body\n",
			"---\ntext: |\n  --- indented content\n---\n# Body\n",
		} {
			content = strings.ReplaceAll(content, "\n", newline)
			_, body := Frontmatter(content)
			if body != "# Body"+newline {
				t.Errorf("body = %q for %q", body, content)
			}
		}
	}
	content := "---\ntype: note\n---suffix\n# Body\n"
	fm, body := Frontmatter(content)
	if len(fm) != 0 || body != content {
		t.Fatalf("a prefix must not close the header: fm=%v body=%q", fm, body)
	}
	fm, _ = Frontmatter("---\nversion: 0.1\nenabled: true\n---\n")
	if fm["version"] != "0.1" || fm["enabled"] != "true" {
		t.Fatalf("metadata interpretation changed: %v", fm)
	}
}
