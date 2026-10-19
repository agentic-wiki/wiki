package parse

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ValidateFrontmatter checks the syntax of a leading YAML header without
// changing its contents or the scalar/string-list interpretation in Frontmatter.
// Files without a header are valid here; requiring a type is the index's job.
// YAML errors retain file-relative line numbers by including the opening fence.
func ValidateFrontmatter(content string) error {
	header, _, err := splitFrontmatter(content)
	if err != nil || header == "" {
		return err
	}
	decoder := yaml.NewDecoder(strings.NewReader(header))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return err
	}
	node := document.Content[0]
	// An empty or comment-only header is allowed, like an absent header.
	if node.Kind != yaml.MappingNode && !(node.Tag == "!!null" && node.Value == "") {
		return fmt.Errorf("line %d: frontmatter must be a YAML mapping", node.Line)
	}
	if err := duplicateFrontmatterKeys(&document); err != nil {
		return err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return errors.New("frontmatter must contain a single YAML mapping")
	}
	return nil
}

// duplicateFrontmatterKeys checks scalar mapping keys throughout the node tree.
// Complex keys are outside extraction's subset but remain valid YAML. Walking
// Content rather than resolving aliases also permits recursive YAML anchors.
func duplicateFrontmatterKeys(node *yaml.Node) error {
	if node.Kind == yaml.MappingNode {
		type key struct{ tag, value string }
		seen := map[key]int{}
		for i := 0; i < len(node.Content); i += 2 {
			k := node.Content[i]
			if k.Kind != yaml.ScalarNode {
				continue
			}
			id := key{k.Tag, k.Value}
			if line, exists := seen[id]; exists {
				return fmt.Errorf("line %d: mapping key %q already defined at line %d", k.Line, k.Value, line)
			}
			seen[id] = k.Line
		}
	}
	for _, child := range node.Content {
		if err := duplicateFrontmatterKeys(child); err != nil {
			return err
		}
	}
	return nil
}

// splitFrontmatter shares fence recognition between extraction and validation.
// header includes the opener, excludes the closer, and is empty when absent.
// Only a complete --- line closes the header, never a ---prefix or indented text.
func splitFrontmatter(content string) (header, body string, err error) {
	first, _, _ := strings.Cut(content, "\n")
	if strings.TrimSuffix(first, "\r") != "---" {
		return "", content, nil
	}
	for start := len(first) + 1; start <= len(content); {
		line, _, newline := strings.Cut(content[start:], "\n")
		end := start + len(line)
		if newline {
			end++
		}
		if strings.TrimSuffix(line, "\r") == "---" {
			return content[:start], content[end:], nil
		}
		if !newline {
			break
		}
		start = end
	}
	return "", content, errors.New("line 1: unterminated frontmatter (missing closing --- fence)")
}
