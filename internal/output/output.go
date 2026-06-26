// Package output renders command results as text, JSON, CSV, or TSV.
package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

// Emit prints lines (text) or marshals v to the chosen format. CSV/TSV derive a
// header row and columns from v's struct json tags (see delimited); non-tabular
// commands fall back to the text lines.
func Emit(w io.Writer, format string, lines []string, v any) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(jsonable(v))
	case "csv":
		return delimited(w, v, ',', lines)
	case "tsv":
		return delimited(w, v, '\t', lines)
	default:
		for _, l := range lines {
			fmt.Fprintln(w, l)
		}
		return nil
	}
}

// jsonable swaps a nil slice for an empty one so empty results render as [] in
// JSON rather than null.
func jsonable(v any) any {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Slice && rv.IsNil() {
		return reflect.MakeSlice(rv.Type(), 0, 0).Interface()
	}
	return v
}

// delimited writes v as CSV or TSV when it is a slice: a header row from the
// element struct's json field tags, then one row per element (a list-valued
// field like tags joins with "; "). A slice of scalars becomes a single "value"
// column. Anything else (a non-slice result) falls back to the text lines, so
// `--format csv` on a non-tabular command degrades gracefully instead of
// emitting nothing.
func delimited(w io.Writer, v any, comma rune, lines []string) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		for _, l := range lines {
			fmt.Fprintln(w, l)
		}
		return nil
	}
	cw := csv.NewWriter(w)
	cw.Comma = comma

	elem := rv.Type().Elem()
	for elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if elem.Kind() == reflect.Struct {
		cols := structColumns(elem)
		header := make([]string, len(cols))
		for i, c := range cols {
			header[i] = c.name
		}
		cw.Write(header)
		for i := 0; i < rv.Len(); i++ {
			ev := reflect.Indirect(rv.Index(i))
			if !ev.IsValid() {
				continue
			}
			row := make([]string, len(cols))
			for j, c := range cols {
				row[j] = cell(ev.FieldByIndex(c.index))
			}
			cw.Write(row)
		}
	} else {
		cw.Write([]string{"value"})
		for i := 0; i < rv.Len(); i++ {
			cw.Write([]string{cell(rv.Index(i))})
		}
	}
	cw.Flush()
	return cw.Error()
}

type column struct {
	name  string
	index []int
}

// structColumns lists a struct's exported fields as columns, named by their
// json tag (a `json:"-"` field is skipped, an untagged field uses its Go name).
func structColumns(t reflect.Type) []column {
	var cols []column
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" { // unexported
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		switch name {
		case "-":
			continue
		case "":
			name = f.Name
		}
		cols = append(cols, column{name, f.Index})
	}
	return cols
}

// cell renders one field value as a string cell. A string slice joins with
// "; " so list fields (e.g. tags) stay in a single column.
func cell(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Slice:
		parts := make([]string, v.Len())
		for i := range parts {
			parts[i] = cell(v.Index(i))
		}
		return strings.Join(parts, "; ")
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}
