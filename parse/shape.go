package parse

import (
	"fmt"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// ShapeIssues reports frontmatter values that are not scalars or lists of
// scalars, one error per key, each starting "line N: " (N counts from the
// file's first line), in line order:
//   - a nested object, a list of objects, or a list inside a list;
//   - a value that continues on the next line, or a block-list item that does;
//   - a value with no key, or a key that is a list or an object, which a read
//     would otherwise drop without a word.
//
// It assumes the header is valid YAML (call ValidateFrontmatter first) and
// returns nil when it isn't, or when there is no header.
func ShapeIssues(content string) []error {
	m, err := decodeHeader(content)
	if err != nil || m == nil {
		return nil
	}
	type issue struct {
		line int // where to point the author
		msg  string
	}
	byKey := map[int]issue{} // by the key's line: one issue per key
	for i := 0; i+1 < len(m.Content); i += 2 {
		key, val := m.Content[i], m.Content[i+1]
		switch msg := nodeShape(key.Value, val); {
		case key.Kind != yaml.ScalarNode:
			byKey[key.Line] = issue{key.Line, "a key that is a list or an object; write a plain key, e.g. k: x"}
		case key.Tag == "!!null":
			byKey[key.Line] = issue{key.Line, "a value with no key; write a key before the colon"}
		case msg != "":
			byKey[key.Line] = issue{key.Line, msg}
		}
	}
	h, _ := Scan(content) // valid YAML, so the header is terminated
	lines := strings.Split(content, "\n")
	for _, f := range h.Fields {
		if _, found := byKey[f.Line+1]; found {
			continue
		}
		switch line, item := continuation(f, lines); {
		case line < 0:
		case item:
			byKey[f.Line+1] = issue{line + 1, fmt.Sprintf("%q continues an item on the next line; write each item on one line", f.Key)}
		default:
			byKey[f.Line+1] = issue{line + 1, fmt.Sprintf("%q continues on the next line; write the value on one line, or use | or > for text", f.Key)}
		}
	}
	all := make([]issue, 0, len(byKey))
	for _, is := range byKey {
		all = append(all, is)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].line < all[j].line })
	errs := make([]error, len(all))
	for i, is := range all {
		errs[i] = fmt.Errorf("line %d: %s", is.line, is.msg)
	}
	return errs
}

// nodeShape describes why the value of key isn't a scalar or a list of
// scalars, or returns "" when it is one.
func nodeShape(key string, v *yaml.Node) string {
	v = resolve(v)
	switch v.Kind {
	case yaml.MappingNode:
		example := key + "_a: 1"
		if len(v.Content) >= 2 && v.Content[0].Kind == yaml.ScalarNode && v.Content[1].Kind == yaml.ScalarNode {
			example = key + "_" + v.Content[0].Value + ": " + v.Content[1].Value
		}
		return fmt.Sprintf("%q holds a nested object; values are scalars or lists of scalars (flatten it, e.g. %s)", key, example)
	case yaml.SequenceNode:
		for _, item := range v.Content {
			switch resolve(item).Kind {
			case yaml.MappingNode:
				return fmt.Sprintf("%q holds a list of objects; values are scalars or lists of scalars", key)
			case yaml.SequenceNode:
				return fmt.Sprintf("%q holds a list inside a list; values are scalars or lists of scalars", key)
			}
		}
	}
	return ""
}

// resolve follows an alias (`*name`) to the node it names.
func resolve(v *yaml.Node) *yaml.Node {
	if v.Kind == yaml.AliasNode && v.Alias != nil {
		return v.Alias
	}
	return v
}

// continuation returns the 0-based line of f's value that continues on the
// next owned line, with item true when it is a block-list item, or -1 when
// nothing continues. A scalar or a flow list continues when its key owns any
// content line; a block-list item, when a content line that isn't an item
// follows it. Text and objects don't continue: their owned lines are the value.
func continuation(f Field, lines []string) (line int, item bool) {
	owned := lines[f.Line+1 : f.End+1]
	switch {
	case f.kind == kindScalar || f.kind == kindList && f.raw != "":
		for _, l := range owned {
			if l = strings.TrimSuffix(l, "\r"); !blank(l) && !commentLine(l) {
				return f.Line, false
			}
		}
	case f.kind == kindList:
		last := -1
		for j, l := range owned {
			switch l = strings.TrimSuffix(l, "\r"); {
			case blank(l) || commentLine(l):
			case isBlockItem(l):
				last = f.Line + 1 + j
			default:
				return last, true
			}
		}
	}
	return -1, false
}
