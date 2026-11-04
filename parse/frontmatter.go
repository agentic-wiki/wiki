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
	_, err := decodeHeader(content)
	return err
}

// decodeHeader decodes a leading header with yaml.v3 and checks it as
// ValidateFrontmatter does, returning its top-level mapping: nil, with no
// error, for a file without a header or with an empty or comment-only one.
func decodeHeader(content string) (*yaml.Node, error) {
	_, header, _, err := scan(content)
	if err != nil || header == "" {
		return nil, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(header))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	node := document.Content[0]
	// An empty or comment-only header is allowed, like an absent header.
	if node.Kind != yaml.MappingNode && !(node.Tag == "!!null" && node.Value == "") {
		return nil, fmt.Errorf("line %d: frontmatter must be a YAML mapping", node.Line)
	}
	if err := duplicateFrontmatterKeys(&document); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("frontmatter must contain a single YAML mapping")
	}
	if node.Kind != yaml.MappingNode {
		return nil, nil
	}
	return node, nil
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
