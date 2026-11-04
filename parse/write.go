package parse

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// SetField returns content with key set to the scalar value, replacing only the
// lines the key owns (see Scan), or adding the key (and a header) when it is
// absent. Errors: an invalid key; an unterminated header; a key that appears
// twice.
//
// The value is written by rule W, so it keeps what the author wrote: a number
// or boolean replacing a value of the same type, or a null placeholder, stays
// bare (see bare); a value replacing a quoted one keeps its quote character;
// anything else is quoted only when it needs it. The comment on the key's line
// is kept.
func SetField(content, key, value string) (string, error) {
	return edit(content, key, func(f *Field) []string {
		if f == nil {
			return []string{newField(key, value)}
		}
		return []string{formatField(key, value, f)}
	})
}

// SetFieldList is SetField for a list of scalars, each item written by rule W
// on its own. A key written as a block list stays one; any other key is
// written as a flow list, and an empty list as `key: []`. The comment on the
// key's line is kept; comments on the old items go with them.
func SetFieldList(content, key string, values []string) (string, error) {
	return edit(content, key, func(f *Field) []string {
		old, comment := oldType(f), ""
		if f != nil {
			comment = f.comment
		}
		items := make([]string, len(values))
		for i, v := range values {
			items[i] = listItem(v, old)
		}
		if f != nil && f.kind == kindList && f.raw == "" && len(values) > 0 {
			lines := []string{key + ":" + comment}
			for _, item := range items {
				lines = append(lines, "  - "+item)
			}
			return lines
		}
		return []string{key + ": [" + strings.Join(items, ", ") + "]" + comment}
	})
}

// UnsetField removes key and every line it owns. Content is unchanged when the
// key is absent. Errors: an invalid key; an unterminated header; a key that
// appears twice.
func UnsetField(content, key string) (string, error) {
	return edit(content, key, func(f *Field) []string { return nil })
}

// edit replaces the lines key owns with render(field), or, when the key is
// absent, adds render(nil) at the end of the header (and a header when there
// is none). An absent key that renders no lines leaves content unchanged.
func edit(content, key string, render func(f *Field) []string) (string, error) {
	if err := validKey(key); err != nil {
		return "", err
	}
	h, err := Scan(content)
	if err != nil {
		return "", err
	}
	var f *Field
	for i := range h.Fields {
		if h.Fields[i].Key != key {
			continue
		}
		if f != nil {
			return "", fmt.Errorf("key %q appears twice in frontmatter (lines %d and %d); fix the file first (wiki check reports it)", key, f.Line+1, h.Fields[i].Line+1)
		}
		f = &h.Fields[i]
	}
	repl := render(f)
	if f == nil && len(repl) == 0 {
		return content, nil
	}
	if h.Open < 0 {
		return "---\n" + strings.Join(repl, "\n") + "\n---\n" + content, nil
	}
	lines := strings.Split(content, "\n")
	start, end := h.Close, h.Close-1 // absent: insert before the closing fence
	if f != nil {
		start, end = f.Line, f.End
	}
	cr := carriage(lines[start])
	for i := range repl {
		repl[i] += cr
	}
	out := append(append(append([]string{}, lines[:start]...), repl...), lines[end+1:]...)
	return strings.Join(out, "\n"), nil
}

// validKey reports why key can't be written: it starts with _ (reserved for the
// index's _path), or the scanner wouldn't read `key: x` back as exactly this key.
func validKey(key string) error {
	if strings.HasPrefix(key, "_") {
		return fmt.Errorf("key %q is reserved (leading underscore)", key)
	}
	h, err := Scan("---\n" + key + ": x\n---\n")
	if err != nil || len(h.Fields) != 1 || h.Fields[0].Key != key {
		return fmt.Errorf(`key %q can't be written: it would not read back as the same key (a key can't start with a space, "-" or "#", or contain ":" or " #")`, key)
	}
	return nil
}

// formatField renders a `key: value` line in place of the field f by rule W,
// and keeps the key line's comment:
//   - bare when the value keeps the field's type (see bare);
//   - else, when the old value is a quoted scalar, in the author's quote
//     character whenever the value fits in it (see fitsQuote), even when it
//     needs quoting: `v: '1.0'` set to 1.1 gives `v: '1.1'`;
//   - else quoted only when it needs it, with %q (newField).
func formatField(key, value string, f *Field) string {
	line := newField(key, value)
	switch {
	case bare(value, oldType(f)):
		line = key + ": " + value
	case f.kind == kindScalar && quoted(f.raw) && fitsQuote(value, f.raw[0]):
		line = fmt.Sprintf("%s: %c%s%c", key, f.raw[0], value, f.raw[0]) // keep the author's quoting
	}
	return line + f.comment
}

// fitsQuote reports whether value, wrapped in the quote q, reads back
// unchanged. The subset decodes no escapes and trims spaces inside quotes, so
// single quotes fit a value with no ', \r or \n, double quotes one with no ",
// \, \r or \n, and neither fits one with spaces at either end.
func fitsQuote(value string, q byte) bool {
	unfit := "'\r\n"
	if q == '"' {
		unfit = "\"\\\r\n"
	}
	return !strings.ContainsAny(value, unfit) && value == strings.TrimSpace(value)
}

// newField renders a `key: value` line for a key the file doesn't have yet,
// quoting only when the value needs it.
func newField(key, value string) string {
	if needsQuote(value) {
		return fmt.Sprintf("%s: %q", key, value)
	}
	return key + ": " + value
}

// listItem renders one list item by rule W: bare when it keeps the list's type
// (see bare), and otherwise quoted only when it needs it.
func listItem(v string, old valueType) string {
	if !bare(v, old) && needsQuote(v) {
		return fmt.Sprintf("%q", v)
	}
	return v
}

// valueType is the type rule T gives a value, as rule W compares them.
type valueType int

const (
	typeNone valueType = iota // no old value: the key is new
	typeNull
	typeNumber
	typeBool
	typeString
)

// typeOf is the type of one typed-view value.
func typeOf(v any) valueType {
	switch v.(type) {
	case nil:
		return typeNull
	case json.Number:
		return typeNumber
	case bool:
		return typeBool
	}
	return typeString
}

// oldType is the type a field holds by rule T: a scalar's own type; for a
// list, the type all its items share (mixed types count as string, and an empty
// list as null); text and objects are strings. A missing field has none.
func oldType(f *Field) valueType {
	if f == nil {
		return typeNone
	}
	items, isList := f.typed.([]any)
	switch {
	case f.kind == kindText || f.kind == kindObject:
		return typeString
	case !isList:
		return typeOf(f.typed) // a scalar, or a value read as a multi-line string
	case len(items) == 0:
		return typeNull
	}
	t := typeOf(items[0])
	for _, item := range items[1:] {
		if typeOf(item) != t {
			return typeString
		}
	}
	return t
}

// bare reports whether rule W writes value without quotes: rule T makes it a
// number or a boolean, and the old type is the same type or null, so the field
// keeps the type it has, or was waiting for, in `list --format json`. A quoted
// placeholder (`key: ""`) is a string and stays one. value must be the bare
// token itself: one with spaces or a # would be cut by the comment rule.
func bare(value string, old valueType) bool {
	if strings.ContainsAny(value, " \t\r\n#") {
		return false
	}
	t := typeOf(typedScalar(value))
	return (t == typeNumber || t == typeBool) && (old == t || old == typeNull)
}

// needsQuote reports whether a value would not survive unquoted. Deliberately
// conservative: a wrongly-bare value corrupts the file, while a needlessly
// quoted one is merely noisy. It covers a list item too: a comma or a bracket
// would end a flow-list item.
//
// Numbers are quoted, which matters more than it looks: a bare `0.1` is a float,
// so a version or an identifier written bare would stop being the string it was.
func needsQuote(v string) bool {
	if v == "" || v != strings.TrimSpace(v) {
		return true
	}
	if strings.ContainsAny(v, ":#[]{},&*!|>'\"%@`\n\r\t") {
		return true
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return true
	}
	switch strings.ToLower(v) {
	case "true", "false", "null", "yes", "no", "on", "off", "~":
		return true
	}
	return v[0] == '-' || v[0] == '?'
}

// carriage returns the \r a CRLF line ends with, so a replaced line keeps it.
func carriage(line string) string {
	if strings.HasSuffix(line, "\r") {
		return "\r"
	}
	return ""
}
