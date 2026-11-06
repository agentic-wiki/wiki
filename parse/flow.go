package parse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Map is a YAML mapping with its keys in file order. It is the typed value of
// every mapping, the top level included, and it marshals to a JSON object in
// that order. A key written twice keeps its last value at its first position.
type Map []Pair

// Pair is one key of a Map and its typed value.
type Pair struct {
	Key   string
	Value any
}

// Get returns the value of key, and whether the map has it.
func (m Map) Get(key string) (any, bool) {
	for _, p := range m {
		if p.Key == key {
			return p.Value, true
		}
	}
	return nil, false
}

// mapBuilder builds a Map in file order: a key written twice keeps its last
// value at its first position. pos finds a key's position, so building is
// linear in the number of keys.
type mapBuilder struct {
	m   Map
	pos map[string]int
}

func newMapBuilder() *mapBuilder { return &mapBuilder{m: Map{}, pos: map[string]int{}} }

func (b *mapBuilder) set(key string, v any) {
	if i, ok := b.pos[key]; ok {
		b.m[i].Value = v
		return
	}
	b.pos[key] = len(b.m)
	b.m = append(b.m, Pair{key, v})
}

// MarshalJSON writes the map as a JSON object, keys in file order.
func (m Map) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, p := range m {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := json.Marshal(p.Key)
		if err != nil {
			return nil, err
		}
		v, err := json.Marshal(p.Value)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// Empty is the one "no value" test, on a typed value (ReadFrontmatter): true
// for nil (an absent key or null), "", an empty list, an empty map, and a list
// whose items are all empty. 0 and false are values.
func Empty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case Map:
		return len(x) == 0
	case []any:
		for _, item := range x {
			if !Empty(item) {
				return false
			}
		}
		return true
	}
	return false
}

// Flow writes a typed value as ReadFrontmatter builds it (string, json.Number,
// bool, nil, []any, Map) as YAML on one line, in flow style. Read back by
// ReadFrontmatter, the result gives the same typed value; read by any YAML 1.1
// or 1.2 reader, its strings stay strings, dates aside. It is one recursive
// writer into a single strings.Builder: no node builds or keeps its own text.
//
// A list is [a, b], a map {k: v} in its key order, a number as written, a bool
// true or false, and null null. A string or a key is bare when it is flow-safe
// and no YAML reader would type it, and otherwise double-quoted with
// strconv.Quote, whose escapes YAML reads the same way. Comments, anchors,
// quote styles and spellings such as True are not kept. Any other type is
// written as its text, quoted as a string would be: best effort.
func Flow(v any) string {
	var b strings.Builder
	writeFlow(&b, v)
	return b.String()
}

func writeFlow(b *strings.Builder, v any) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(x))
	case json.Number:
		b.WriteString(string(x))
	case string:
		writeFlowString(b, x)
	case []any:
		b.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				b.WriteString(", ")
			}
			writeFlow(b, item)
		}
		b.WriteByte(']')
	case Map:
		b.WriteByte('{')
		for i, p := range x {
			if i > 0 {
				b.WriteString(", ")
			}
			if p.Key == "<<" { // bare, it would read as a merge key
				b.WriteString(`"<<"`)
			} else {
				writeFlowString(b, p.Key)
			}
			b.WriteString(": ")
			writeFlow(b, p.Value)
		}
		b.WriteByte('}')
	default: // not a typed value: best effort, written as its text
		writeFlowString(b, fmt.Sprint(x))
	}
}

func writeFlowString(b *strings.Builder, s string) {
	if flowSafe(s) && !typedWord(s) {
		b.WriteString(s)
		return
	}
	b.WriteString(strconv.Quote(s))
}

// special holds the characters that make a value need quotes, both in Flow
// (flowSafe) and in what SetField and SetFieldList write (needsQuote). ? is
// here on purpose, anywhere in the string: yaml.v3 can't read it bare inside a
// flow list or map (`[Why?, b]` doesn't read), though the YAML spec allows it.
const special = ",[]{}#&*!|>?'\"%@`"

// flowSafe reports whether a string reads back as itself, bare, inside a flow
// list or map. strconv.IsPrint already refuses the controls, DEL, NEL, LS, PS,
// U+FEFF, U+FFFE and every space but U+0020, so a bare string is valid YAML on
// one line.
func flowSafe(s string) bool {
	if s == "" || s[0] == ' ' || s[len(s)-1] == ' ' || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if !strconv.IsPrint(r) {
			return false
		}
	}
	if strings.ContainsAny(s, special) || strings.Contains(s, ": ") || strings.HasSuffix(s, ":") {
		return false
	}
	return s[0] != '-' && s[0] != ':'
}

// typedWord reports whether some YAML 1.1 or 1.2 reader would type the whole
// string as a bool, null or number. Flow quotes it, so it stays a string there
// too. Dates are left out: wiki reads them as strings, and they stay bare.
func typedWord(s string) bool {
	// Cheap tests first, so most strings skip the regexp: every typed word
	// starts with one of these, and one that starts with a digit is 0x, 0o or
	// 0b, or holds nothing but a decimal's or a base-60 number's characters
	// (so a hash or a date-time doesn't need the regexp).
	switch c := s[0]; {
	case !strings.ContainsRune("yYnNtTfFoO~+-.0123456789", rune(c)):
		return false
	case c >= '0' && c <= '9' && !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "0o") && !strings.HasPrefix(s, "0b") &&
		strings.Trim(s, "0123456789_.eE+-:") != "":
		return false
	}
	return typedWordRe.MatchString(s)
}

var typedWordRe = regexp.MustCompile(`^(?:` +
	`y|Y|yes|Yes|YES|n|N|no|No|NO|true|True|TRUE|false|False|FALSE|on|On|ON|off|Off|OFF|~|null|Null|NULL` +
	`|[-+]?(?:\.[0-9]+|[0-9][0-9_]*(?:\.[0-9_]*)?)(?:[eE][-+]?[0-9]+)?` + // decimal
	`|[-+]?0x[0-9a-fA-F_]+` + // hex
	`|0o[0-7]+` + // octal
	`|[-+]?0b[01_]+` + // binary
	`|[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+(?:\.[0-9_]*)?` + // base 60
	`|[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN)` +
	`)$`)

// Clone is a deep copy of the map: every []any and Map in it is copied, to
// any depth. Strings, numbers, bools and nil are values already.
func (m Map) Clone() Map { return deepCopy(m).(Map) }

// deepCopy is Clone for any typed value.
func deepCopy(v any) any {
	switch x := v.(type) {
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = deepCopy(item)
		}
		return out
	case Map:
		out := make(Map, len(x))
		for i, p := range x {
			out[i] = Pair{p.Key, deepCopy(p.Value)}
		}
		return out
	}
	return v
}
