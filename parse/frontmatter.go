package parse

import (
	"fmt"
	"sort"
)

// CheckFrontmatter returns the header's syntax error or, when there is none,
// its shape issues, each starting "line N: " (N counts from the file's first
// line), in line order. One decode serves both.
//
// The syntax errors are exactly the headers ReadFrontmatter refuses, with the
// same messages (not valid YAML, not a mapping, unterminated, over a limit),
// plus one the read accepts: a key written twice, at any depth, identified by
// its decoded text (the read keeps its last value).
//
// The shape issues are what wiki can't read or edit safely: a key that is a
// list or a map, a key with no text (`~: x`), a merge key (`<<`), and a key
// written with escapes, at any depth; and a header whose keys aren't each a
// scalar key at column 0 (a flow-map header, a `?` key).
// An alias is never followed for these. A file without a header has neither.
//
//lint:ignore ST1008 the syntax error comes first on purpose: shape issues exist only when there is none (the ADR's signature)
func CheckFrontmatter(content string) (syntax error, shape []error) {
	_, d, unreadable, err := scanDecoded(content)
	switch {
	case err != nil:
		return err, nil
	case unreadable != nil:
		return unreadable, nil
	case d.duplicate != nil:
		return d.duplicate, nil
	}
	issues := d.issues
	if line := uneditable(d); line > 0 {
		issues = append(issues, issue{line, "wiki can't edit this header safely: write each key on its own line at column 0"})
	}
	sort.SliceStable(issues, func(i, j int) bool { return issues[i].line < issues[j].line })
	for _, is := range issues {
		shape = append(shape, fmt.Errorf("line %d: %s", is.line, is.msg))
	}
	return nil, shape
}

// ValidateFrontmatter is CheckFrontmatter's syntax error alone. Files without
// a header are valid here; requiring a type is the index's job.
func ValidateFrontmatter(content string) error {
	syntax, _ := CheckFrontmatter(content)
	return syntax
}
