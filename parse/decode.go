package parse

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"go.yaml.in/yaml/v3"
)

// The limits on one header, which comes from a file anyone may have written
// (wiki/research/one-parser-adr.md, Decision 5).
const (
	maxHeaderBytes = 1 << 20 // raw header text, checked before yaml.v3 sees it
	maxValues      = 10000   // values built, each alias use counted again
	maxTextBytes   = 1 << 20 // scalar and key text built, plus the flow text of the string view
)

var (
	errHeaderTooLarge = errors.New("header larger than 1 MiB")
	errTooManyValues  = fmt.Errorf("more than %d values once aliases are expanded", maxValues)
	errTooManyBytes   = errors.New("more than 1 MiB once aliases are expanded")
)

// header is a decoded frontmatter header: its top-level keys in file order,
// each occurrence of a key written twice included, and what check reports.
type header struct {
	keys []headerKey
	// issues are the shapes check reports (see CheckFrontmatter), found on
	// the walk where the nodes are written, never through an alias.
	issues []issue
	// duplicate is the first mapping key written twice, at any depth, by its
	// decoded text: check refuses it, the read keeps the last value.
	duplicate error
}

// issue is one shape check reports: its 1-based file line and message.
type issue struct {
	line int
	msg  string
}

// headerKey is one top-level key of a header and its value in both views.
type headerKey struct {
	key       string // its decoded text
	line, col int    // where the key starts in the file, both 1-based
	// scalarKey is true when the key is a scalar. A key that is a list or a
	// map (`? [a, b]`) is left out of the read.
	scalarKey bool
	str       any // the string view: a string, or a []string for a list
	typed     any // the typed view: rule T, []any, Map
	// kNode and vNode are the key and value as written, for writes: where the
	// key is, how its value is written, and where each scalar sits.
	kNode, vNode *yaml.Node
}

// decode refuses a header over 1 MiB before yaml.v3 sees it, decodes it once,
// and walks the node tree once, under two budgets: 10,000 values and 1 MiB of
// scalar, key and flow text, aliases counted at every use. headerText is the
// opening fence and the lines up to the closing one, so yaml.v3's line numbers
// are the file's. An empty or comment-only header has no keys.
//
// The same walk notes what check reports: the shape issues and the first key
// written twice. It is the only code in parse that walks a yaml.Node and
// follows an alias.
func decode(headerText string) (header, error) {
	root, err := yamlRoot(headerText)
	if err != nil || root == nil {
		return header{}, err
	}
	w := walker{headerText: headerText}
	var keys []headerKey
	seen := map[string]int{} // a top-level key's text, to the line it is first on
	for i := 0; i+1 < len(root.Content); i += 2 {
		written, v := root.Content[i], root.Content[i+1]
		text, scalarKey := w.key(written, seen)
		hk := headerKey{key: text, line: written.Line, col: written.Column, scalarKey: scalarKey, kNode: written, vNode: v}
		if scalarKey {
			if err := w.text(len(text)); err != nil {
				return header{}, err
			}
			if hk.typed, err = w.value(v); err != nil {
				return header{}, err
			}
			if hk.str, err = w.strs(v, hk.typed); err != nil {
				return header{}, err
			}
		}
		keys = append(keys, hk)
	}
	return header{keys: keys, issues: w.issues, duplicate: w.duplicate}, nil
}

// yamlRoot checks the size cap, decodes the header with yaml.v3, and returns
// its top-level mapping: nil, with no error, for a file without a header or
// with an empty or comment-only one. The header must be one YAML document
// holding a mapping.
func yamlRoot(headerText string) (*yaml.Node, error) {
	if headerText == "" {
		return nil, nil
	}
	if len(headerText) > maxHeaderBytes {
		return nil, errHeaderTooLarge
	}
	decoder := yaml.NewDecoder(strings.NewReader(headerText))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	node := document.Content[0]
	// An empty or comment-only header is allowed, like an absent header.
	if node.Kind != yaml.MappingNode && !(node.Tag == "!!null" && node.Value == "") {
		return nil, fmt.Errorf("line %d: frontmatter must be a YAML mapping", node.Line)
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

// walker builds typed values from nodes, keeps one header's budgets, and
// notes what check reports where the nodes are written.
type walker struct {
	values, bytes int
	headerText    string   // the header, for how a key is written
	cols          *columns // its lines and their columns, built when escaped first needs them
	aliases       int      // how many alias uses the walk is inside: check notes nothing there
	issues        []issue
	duplicate     error
}

// text counts n bytes of text against the budget.
func (w *walker) text(n int) error {
	if w.bytes += n; w.bytes > maxTextBytes {
		return errTooManyBytes
	}
	return nil
}

// key reads one mapping key: its decoded text, and false for a key that is a
// list or a map. Where the key is written (not reached through an alias), it
// notes the key's shape issue and, by seen (a mapping's key texts, to the line
// each is first on), a key written twice.
func (w *walker) key(written *yaml.Node, seen map[string]int) (string, bool) {
	k := resolve(written)
	if w.aliases > 0 {
		return k.Value, k.Kind == yaml.ScalarNode
	}
	note := func(msg string) { w.issues = append(w.issues, issue{written.Line, msg}) }
	switch {
	case k.Kind != yaml.ScalarNode:
		note("a key that is a list or an object; write a plain key, e.g. k: x")
		return "", false
	case k.Tag == "!!null":
		note("a value with no key; write a key before the colon")
	case k.Tag == "!!merge":
		note(`"<<" merges keys, which wiki does not read; write the keys out`)
	case w.escaped(written):
		note(fmt.Sprintf("key %q is written with escapes; write it plainly", k.Value))
	}
	if first, twice := seen[k.Value]; twice && w.duplicate == nil {
		w.duplicate = fmt.Errorf("line %d: mapping key %q already defined at line %d", written.Line, k.Value, first)
	} else if !twice {
		seen[k.Value] = written.Line
	}
	return k.Value, true
}

// escaped reports whether a key node is double-quoted in the file with a \
// before its closing quote.
func (w *walker) escaped(n *yaml.Node) bool {
	if n.Style&yaml.DoubleQuotedStyle == 0 {
		return false
	}
	if w.cols == nil { // only for a header with a double-quoted key
		w.cols = newColumns(strings.Split(w.headerText, "\n"))
	}
	if n.Line < 1 || n.Line > len(w.cols.lines) {
		return false
	}
	line := w.cols.lines[n.Line-1]
	i := w.cols.offset(n.Line-1, n.Column)
	// The node starts at its properties, if any: skip each tag (!x, !!str) or
	// anchor (&a) and the spaces after it, to reach the opening quote. A tag or
	// anchor runs to the next space; no byte of a multi-byte rune is one.
	for i < len(line) && (line[i] == '!' || line[i] == '&') {
		for i < len(line) && line[i] != ' ' && line[i] != '\t' {
			i++
		}
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
	}
	if i >= len(line) || line[i] != '"' {
		return false
	}
	for j := i + 1; j < len(line); j++ {
		switch line[j] {
		case '\\':
			return true
		case '"':
			return false
		}
	}
	return false
}

// value builds the typed value of a node: rule T at a plain scalar, a []any
// for a sequence, a Map for a mapping, and the named node for an alias, built
// again at every use. A key that is a list or a map is left out.
func (w *walker) value(n *yaml.Node) (any, error) {
	if n.Kind == yaml.AliasNode {
		w.aliases++
		v, err := w.value(resolve(n))
		w.aliases--
		return v, err
	}
	if w.values++; w.values > maxValues {
		return nil, errTooManyValues
	}
	switch n.Kind {
	case yaml.SequenceNode:
		items := make([]any, len(n.Content))
		for i, item := range n.Content {
			v, err := w.value(item)
			if err != nil {
				return nil, err
			}
			items[i] = v
		}
		return items, nil
	case yaml.MappingNode:
		m := newMapBuilder()
		var seen map[string]int // only key notes it, and not inside an alias
		if w.aliases == 0 {
			seen = map[string]int{}
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			text, scalarKey := w.key(n.Content[i], seen)
			if !scalarKey {
				continue
			}
			if err := w.text(len(text)); err != nil {
				return nil, err
			}
			v, err := w.value(n.Content[i+1])
			if err != nil {
				return nil, err
			}
			m.set(text, v)
		}
		return m.m, nil
	}
	if err := w.text(len(n.Value)); err != nil {
		return nil, err
	}
	return scalar(n), nil
}

// strs is the string view of a top-level value, from its node and its typed
// value: a scalar's decoded text; a list's items, each a scalar item's text or
// a list or map item's Flow; a map's Flow. The flow text counts against the
// budget.
func (w *walker) strs(n *yaml.Node, typed any) (any, error) {
	n = resolve(n)
	switch n.Kind {
	case yaml.SequenceNode:
		items := typed.([]any)
		out := make([]string, len(items))
		for i, item := range items {
			if it := resolve(n.Content[i]); it.Kind == yaml.ScalarNode {
				out[i] = it.Value
				continue
			}
			out[i] = Flow(item)
			if err := w.text(len(out[i])); err != nil {
				return nil, err
			}
		}
		return out, nil
	case yaml.MappingNode:
		s := Flow(typed)
		return s, w.text(len(s))
	}
	return n.Value, nil
}

// scalar types one scalar node by rule T. A quoted scalar, a | or > block,
// and a scalar tagged !!str are strings; other tags change nothing.
func scalar(n *yaml.Node) any {
	quotedOrBlock := yaml.DoubleQuotedStyle | yaml.SingleQuotedStyle | yaml.LiteralStyle | yaml.FoldedStyle
	if n.Style&quotedOrBlock != 0 || n.Style&yaml.TaggedStyle != 0 && n.ShortTag() == "!!str" {
		return n.Value
	}
	return typedBare(n.Value)
}

// resolve follows aliases (`*name`) to the node they name.
func resolve(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}

// values is the header's typed view: each scalar key with its value, in file
// order, a key written twice keeping its last value at its first position.
func (h header) values() Map {
	m := newMapBuilder()
	for _, k := range h.keys {
		if k.scalarKey {
			m.set(k.key, k.typed)
		}
	}
	return m.m
}
