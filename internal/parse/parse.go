// Package parse provides targeted extractors for wiki markdown: frontmatter,
// links, tasks, and headings. Intentionally minimal — no full markdown AST.
package parse

import (
	"regexp"
	"strings"
)

// Link is a root-absolute internal markdown link [Text](/target.md).
type Link struct {
	Text   string `json:"text"`
	Target string `json:"target"` // anchor stripped, e.g. /finance/income/x.md
	Line   int    `json:"line"`
}

// Task is a GFM checkbox item.
type Task struct {
	Done bool   `json:"done"`
	Text string `json:"text"`
	Line int    `json:"line"`
}

// Heading is an ATX heading.
type Heading struct {
	Level int    `json:"level"`
	Text  string `json:"text"`
	Line  int    `json:"line"`
}

// Frontmatter splits a leading `---` block from the body and parses the small
// YAML subset we use: scalars and string lists. Values are string or []string.
func Frontmatter(content string) (fm map[string]any, body string) {
	fm = map[string]any{}
	if !strings.HasPrefix(content, "---\n") {
		return fm, content
	}
	rest := content[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return fm, content
	}
	block := rest[:end]
	after := rest[end+1:] // starts at the closing "---"
	if nl := strings.IndexByte(after, '\n'); nl >= 0 {
		body = after[nl+1:]
	}
	parseYAMLSubset(block, fm)
	return fm, body
}

func parseYAMLSubset(block string, fm map[string]any) {
	lines := strings.Split(block, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r")
		if line == "" || line[0] == ' ' || line[0] == '\t' || line[0] == '-' {
			continue // blank or nested; nested is consumed by the block-list scan below
		}
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch {
		case strings.HasPrefix(val, "["):
			fm[key] = List(val)
		case val == "":
			var list []string
			for j := i + 1; j < len(lines); j++ {
				t := strings.TrimSpace(strings.TrimRight(lines[j], "\r"))
				if strings.HasPrefix(t, "- ") {
					list = append(list, Unquote(t[2:]))
					i = j
					continue
				}
				break
			}
			if list != nil {
				fm[key] = list
			} else {
				fm[key] = ""
			}
		default:
			fm[key] = Unquote(val)
		}
	}
}

// List parses a bracketed, comma-separated list of quoted or bare tokens,
// e.g. `[a, "b c", d]` into ["a", "b c", "d"]. Empties are dropped.
func List(val string) []string {
	val = strings.TrimSpace(val)
	val = strings.TrimPrefix(val, "[")
	val = strings.TrimSuffix(val, "]")
	var out []string
	for p := range strings.SplitSeq(val, ",") {
		if p = Unquote(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// Unquote trims surrounding whitespace and a matching pair of quotes.
func Unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		s = s[1 : len(s)-1]
	}
	return strings.TrimSpace(s)
}

// String returns frontmatter key as a string ("" if absent or not a string).
func String(fm map[string]any, key string) string {
	s, _ := fm[key].(string)
	return s
}

// Strings returns frontmatter key as a string slice. A lone scalar
// (e.g. `tags: foo`) is treated as a single-element list.
func Strings(fm map[string]any, key string) []string {
	switch x := fm[key].(type) {
	case []string:
		return x
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	}
	return nil
}

var (
	linkRe       = regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
	taskRe       = regexp.MustCompile(`^\s*[-*+] \[([ xX])\]\s+(.*)$`)
	headingRe    = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	inlineCodeRe = regexp.MustCompile("`[^`]*`")
)

// InternalLinks returns root-absolute markdown links from body, ignoring
// fenced and inline code.
func InternalLinks(body string) []Link {
	var links []Link
	for i, line := range maskedLines(body) {
		for _, m := range linkRe.FindAllStringSubmatch(line, -1) {
			target := strings.TrimSpace(m[2])
			if !strings.HasPrefix(target, "/") {
				continue
			}
			if h := strings.IndexByte(target, '#'); h >= 0 {
				target = target[:h]
			}
			if target != "" {
				links = append(links, Link{Text: m[1], Target: target, Line: i + 1})
			}
		}
	}
	return links
}

// Tasks returns checkbox items, ignoring fenced code.
func Tasks(body string) []Task {
	var tasks []Task
	for i, line := range maskedLines(body) {
		if m := taskRe.FindStringSubmatch(line); m != nil {
			tasks = append(tasks, Task{
				Done: m[1] == "x" || m[1] == "X",
				Text: strings.TrimSpace(m[2]),
				Line: i + 1,
			})
		}
	}
	return tasks
}

// Headings returns ATX headings, ignoring fenced code.
func Headings(body string) []Heading {
	var hs []Heading
	for i, line := range maskedLines(body) {
		if m := headingRe.FindStringSubmatch(line); m != nil {
			hs = append(hs, Heading{Level: len(m[1]), Text: strings.TrimSpace(m[2]), Line: i + 1})
		}
	}
	return hs
}

// maskedLines returns body split into lines with fenced code blocks and inline
// code spans blanked, preserving line numbers.
func maskedLines(body string) []string {
	lines := strings.Split(body, "\n")
	inFence := false
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			inFence = !inFence
			lines[i] = ""
			continue
		}
		if inFence {
			lines[i] = ""
			continue
		}
		lines[i] = inlineCodeRe.ReplaceAllString(line, "")
	}
	return lines
}
