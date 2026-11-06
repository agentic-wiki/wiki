package parse

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// SetField returns content with key set to the scalar value, replacing only the
// lines the key owns (see yamlFields), or adding the key (and a header) when it is
// absent. Errors: an invalid key; an unterminated header; a key that appears
// twice.
//
// The value is written by rule W, so it keeps what the author wrote: a number
// or boolean replacing a value of the same type, or a null placeholder, stays
// bare (see bare); a value replacing a quoted one keeps its quote character;
// anything else is quoted only when it needs it. The comment on the key's line
// is kept.
func SetField(content, key, value string) (string, error) {
	return edit(content, key, func(f *field) []string {
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
	return edit(content, key, func(f *field) []string {
		old, comment := oldType(f), ""
		if f != nil {
			comment = f.comment
		}
		items := make([]string, len(values))
		for i, v := range values {
			items[i] = listItem(v, old)
		}
		if f != nil && f.kind == kindList && f.block && len(values) > 0 {
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
	return edit(content, key, func(f *field) []string { return nil })
}

// EditError is why a write refused a header. Its message is "frontmatter
// <Reason>", or "frontmatter of <Path> <Reason>" once index.Entry has set the
// file's path.
type EditError struct {
	Path   string // the entry's path, when known
	Reason string
}

func (e *EditError) Error() string {
	if e.Path != "" {
		return "frontmatter of " + e.Path + " " + e.Reason
	}
	return "frontmatter " + e.Reason
}

// notRead is the refusal for a header that doesn't read.
func notRead(err error) *EditError {
	return &EditError{Reason: fmt.Sprintf("not read (%s); fix the file first", strings.TrimPrefix(err.Error(), "yaml: "))}
}

// edit replaces the lines key owns with render(field), or, when the key is
// absent, adds render(nil) at the end of the header (and a header when there
// is none). An absent key that renders no lines leaves content unchanged. It
// writes only into a header that reads and that wiki can edit safely (see
// editable), and then reads its result back: a write is refused when the
// header would no longer read (an anchor removed while an alias still names
// it, a budget passed), or when any key but the edited one would change value
// (an alias that now names another anchor).
func edit(content, key string, render func(f *field) []string) (string, error) {
	out, before, err := splice(content, key, render)
	if err != nil || out == content {
		return out, err
	}
	if _, err := readBack(before, out, key); err != nil {
		return "", err
	}
	return out, nil
}

// readBack is the check every write makes before saving: after must read, and
// every key of before (its decode) but the edited ones must keep its value, so
// no alias silently names another anchor. It returns the decode of after.
func readBack(before header, after string, edited ...string) (header, error) {
	_, headerText, _, err := split(after)
	var now header
	if err == nil {
		now, err = decode(headerText)
	}
	if err != nil {
		return header{}, &EditError{Reason: fmt.Sprintf("would not read after this write (%s); nothing written", strings.TrimPrefix(err.Error(), "yaml: "))}
	}
	isEdited := map[string]bool{} // lookups, so a header of many keys costs no more than their number
	for _, k := range edited {
		isEdited[k] = true
	}
	nowValues := map[string]any{}
	for _, p := range now.values() {
		nowValues[p.Key] = p.Value
	}
	for _, p := range before.values() {
		if v := nowValues[p.Key]; !isEdited[p.Key] && !reflect.DeepEqual(v, p.Value) {
			return header{}, &EditError{Reason: fmt.Sprintf("would change %s through an anchor; nothing written", p.Key)}
		}
	}
	return now, nil
}

// splice is edit without the read-back. It also returns the header's decode
// from before the write, for the read-back to compare against.
func splice(content, key string, render func(f *field) []string) (string, header, error) {
	if err := validKey(key); err != nil {
		return "", header{}, err
	}
	h, d, unreadable, err := scanDecoded(content)
	if err != nil {
		return "", header{}, notRead(err)
	}
	var f *field
	for i := range h.fields {
		if h.fields[i].key != key {
			continue
		}
		if f != nil {
			return "", header{}, &EditError{Reason: fmt.Sprintf("has key %q twice (lines %d and %d); fix the file first (wiki check reports it)", key, f.line+1, h.fields[i].line+1)}
		}
		f = &h.fields[i]
	}
	lines := strings.Split(content, "\n")
	if h.open >= 0 {
		// Before the absent-key case too: a key may be in YAML's reading
		// without a field of its own (a flow-map header).
		if err := editable(d, unreadable); err != nil {
			return "", header{}, err
		}
	}
	repl := render(f)
	if f == nil && len(repl) == 0 {
		return content, d, nil
	}
	if h.open < 0 {
		return "---\n" + strings.Join(repl, "\n") + "\n---\n" + content, d, nil
	}
	start, end := h.close, h.close-1 // absent: insert before the closing fence
	if f != nil {
		start, end = f.line, f.end
	}
	cr := carriage(lines[start])
	for i := range repl {
		repl[i] += cr
	}
	out := append(append(append([]string{}, lines[:start]...), repl...), lines[end+1:]...)
	return strings.Join(out, "\n"), d, nil
}

// editable reports why a write into the header could change something other
// than the key it edits, or nil: the header must read (unreadable is decode's
// error), and every top-level key must be a scalar key at column 0 (see
// uneditable).
func editable(d header, unreadable error) error {
	if unreadable != nil {
		return notRead(unreadable)
	}
	if line := uneditable(d); line > 0 {
		return &EditError{Reason: fmt.Sprintf("can't be edited safely: line %d is not a key on its own line at column 0; fix the file first (wiki check reports it)", line)}
	}
	return nil
}

// uneditable is the 1-based line of the first top-level key that isn't a
// scalar key at column 0 (a flow-map header, a `?` key, a key that is a list
// or a map), or 0. Each key's lines are then exactly its own (yamlFields).
func uneditable(d header) int {
	for _, k := range d.keys {
		if !k.scalarKey || k.col != 1 {
			return k.line
		}
	}
	return 0
}

// validKey reports why key can't be written: it starts with _ (reserved for the
// index's _path), or `key: x` would not read back as exactly this key: YAML
// must read it as one scalar key equal to key, at column 0, neither null (`~`,
// `null`) nor a merge key (`<<`).
func validKey(key string) error {
	if strings.HasPrefix(key, "_") {
		return fmt.Errorf("key %q is reserved (leading underscore)", key)
	}
	d, err := decode("---\n" + key + ": x\n")
	if err != nil || len(d.keys) != 1 || !d.keys[0].scalarKey || d.keys[0].key != key || d.keys[0].col != 1 ||
		typedBare(key) == nil || key == "<<" {
		return fmt.Errorf(`key %q can't be written: it would not read back as the same key (a key can't start with a space or a YAML indicator such as "-", "#", "[" or "*", or be null or "<<")`, key)
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
func formatField(key, value string, f *field) string {
	line := newField(key, value)
	switch {
	case bare(value, oldType(f)):
		line = key + ": " + value
	case f.quote != 0 && fitsQuote(value, f.quote):
		line = fmt.Sprintf("%s: %c%s%c", key, f.quote, value, f.quote) // keep the author's quoting
	}
	return line + f.comment
}

// fitsQuote reports whether value, wrapped in the quote q, reads back
// unchanged with no escape written: single quotes fit a value with no ', \r
// or \n, and double quotes one with no ", \, \r or \n. A value with spaces at
// either end is left to newField, which quotes it with %q.
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
// list, the type all its items share (mixed types count as string, an empty
// list as null, and a null item as null). A map, and a list that holds a list
// or a map, count as string. A missing field has none.
func oldType(f *field) valueType {
	if f == nil {
		return typeNone
	}
	items, isList := f.typed.([]any)
	switch {
	case !isList:
		return typeOf(f.typed)
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
	if strings.ContainsAny(v, ":"+special+"\n\r\t") {
		return true
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return true
	}
	switch strings.ToLower(v) {
	case "true", "false", "null", "yes", "no", "on", "off", "~":
		return true
	}
	return v[0] == '-'
}

// carriage returns the \r a CRLF line ends with, so a replaced line keeps it.
func carriage(line string) string {
	if strings.HasSuffix(line, "\r") {
		return "\r"
	}
	return ""
}
