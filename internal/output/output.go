// Package output renders command results as text, JSON, JSON Lines, CSV, or TSV.
//
// text prints the command's lines and json its value. jsonl, csv and tsv print
// its rows: one per item of a list result, or a single row for any other
// result. csv and tsv start with a header and render each value by Cell; jsonl
// is one compact object per row, with the row's own keys.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/agentic-wiki/wiki/internal/sheet"
	"github.com/agentic-wiki/wiki/parse"
)

// Rows is a result as a table: the column names and one row of values per
// item, aligned with them. A value is anything Cell renders, or Missing.
type Rows struct {
	Header []string
	Values [][]any
}

// Missing marks a column a row doesn't have: an empty cell in csv and tsv, and
// a key left out in jsonl. It is not null: a null value is nil.
var Missing any = missing{}

type missing struct{}

// Tabular reports whether a format prints rows (jsonl, csv, tsv).
func Tabular(format string) bool {
	return format == "jsonl" || format == "csv" || format == "tsv"
}

// Emit prints a result: lines for text, v for json, and for jsonl, csv and tsv
// the rows RowsOf derives from v.
func Emit(w io.Writer, format string, lines []string, v any) error {
	var rows Rows
	if Tabular(format) {
		var err error
		if rows, err = RowsOf(v); err != nil {
			return err
		}
	}
	return EmitRows(w, format, lines, v, rows)
}

// EmitRows is Emit with the rows given, for a result whose rows are not its
// JSON value's elements: a list's frontmatter columns, search's matching
// lines, a move's rewritten files.
func EmitRows(w io.Writer, format string, lines []string, v any, rows Rows) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false) // results are data, not HTML: as jsonl writes them
		enc.SetIndent("", "  ")
		return enc.Encode(jsonable(v))
	case "jsonl":
		return rows.writeJSONL(w)
	case "csv":
		return rows.write(w, ',')
	case "tsv":
		return rows.write(w, '\t')
	}
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
	return nil
}

// jsonable swaps a nil slice for an empty one so empty results render as [] in
// JSON rather than null.
func jsonable(v any) any {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Slice && rv.IsNil() {
		return reflect.MakeSlice(rv.Type(), 0, 0).Interface()
	}
	return v
}

// RowsOf derives a result's rows from its JSON shape: a slice of structs gives
// one row per element, and a struct a single row. The columns are the fields'
// json names, in field order (a `json:"-"` field is left out), and a zero
// `omitempty` field is Missing, as json leaves it out. Any other value has no
// rows: an error, never a quiet fallback to text.
func RowsOf(v any) (Rows, error) {
	rv := reflect.ValueOf(v)
	t := rv.Type()
	if rv.Kind() == reflect.Slice {
		t = t.Elem()
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return Rows{}, fmt.Errorf("output: %T has no rows", v)
	}
	cols := structColumns(t)
	rows := Rows{Header: make([]string, len(cols))}
	for i, c := range cols {
		rows.Header[i] = c.name
	}
	add := func(ev reflect.Value) {
		if ev = reflect.Indirect(ev); !ev.IsValid() {
			return
		}
		row := make([]any, len(cols))
		for j, c := range cols {
			f := ev.FieldByIndex(c.index)
			if c.omitEmpty && omitted(f) {
				row[j] = Missing
			} else {
				row[j] = f.Interface()
			}
		}
		rows.Values = append(rows.Values, row)
	}
	if rv.Kind() == reflect.Slice {
		for i := 0; i < rv.Len(); i++ {
			add(rv.Index(i))
		}
	} else {
		add(rv)
	}
	return rows, nil
}

// omitted reports whether json's omitempty leaves the field f out: false, 0, a
// nil pointer or interface, or an empty array, slice, map or string. json never
// leaves out a struct.
func omitted(f reflect.Value) bool {
	switch f.Kind() {
	case reflect.Array, reflect.Slice, reflect.Map, reflect.String:
		return f.Len() == 0
	case reflect.Struct:
		return false
	}
	return f.IsZero()
}

// Cell is the one rule for a csv or tsv cell, for the values rows carry:
// frontmatter values (a string, json.Number, bool, nil, []any or parse.Map),
// string lists, and a command's own numbers. Missing, and a value for which
// parse.Empty holds (nil, "", an empty list or map, a list of empty items), is
// an empty cell. A string is itself. A list or map is one line of YAML in flow
// style (parse.Flow), so JSON appears only in json and jsonl output. A number
// is as written, a bool true or false.
func Cell(v any) string {
	switch x := v.(type) {
	case missing:
		return ""
	case string:
		return x
	case []string:
		items := make([]any, len(x))
		for i, s := range x {
			items[i] = s
		}
		v = items
	}
	if parse.Empty(v) {
		return ""
	}
	switch x := v.(type) {
	case []any, parse.Map:
		return parse.Flow(x)
	case json.Number:
		return string(x)
	}
	return fmt.Sprint(v) // a bool, or a command's int (a line, a count)
}

// compactJSON encodes v as one line of JSON, without HTML escaping: rows are
// data, not HTML, so < > & stay as written.
func compactJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

// write prints the rows as csv or tsv through sheet.Write: every CSV field
// quoted, and a cell a spreadsheet would run as a formula with a leading '.
// With no rows, the header alone is printed.
func (r Rows) write(w io.Writer, comma rune) error {
	rows := [][]string{r.Header}
	for _, row := range r.Values {
		cells := make([]string, len(row))
		for i, v := range row {
			cells[i] = Cell(v)
		}
		rows = append(rows, cells)
	}
	return sheet.Write(w, comma, rows)
}

// writeJSONL prints one compact JSON object per row, its keys in header order,
// leaving out the Missing ones. With no rows, it prints nothing.
func (r Rows) writeJSONL(w io.Writer) error {
	for _, row := range r.Values {
		var b bytes.Buffer
		b.WriteByte('{')
		for i, v := range row {
			if _, absent := v.(missing); absent {
				continue
			}
			k, err := compactJSON(r.Header[i])
			if err != nil {
				return err
			}
			val, err := compactJSON(v)
			if err != nil {
				return err
			}
			if b.Len() > 1 {
				b.WriteByte(',')
			}
			b.Write(k)
			b.WriteByte(':')
			b.Write(val)
		}
		b.WriteString("}\n")
		if _, err := w.Write(b.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

type column struct {
	name      string
	index     []int
	omitEmpty bool
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
		name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		switch name {
		case "-":
			continue
		case "":
			name = f.Name
		}
		cols = append(cols, column{name, f.Index, strings.Contains(opts, "omitempty")})
	}
	return cols
}

// Table renders a dataset's extracted table, whose columns are dynamic (named
// by the header). text prints the cells as written in aligned columns. csv,
// tsv and json are the table's own writers (parse.Table.WriteCSV, WriteTSV and
// WriteJSON), which apps call too, so they give the CLI's bytes. jsonl gives
// one typed object per line.
func Table(w io.Writer, format string, t parse.Table) error {
	switch format {
	case "json":
		return t.WriteJSON(w)
	case "csv":
		return t.WriteCSV(w)
	case "jsonl":
		return Rows{t.Header, t.TypedRows()}.writeJSONL(w)
	case "tsv":
		return t.WriteTSV(w)
	default:
		return alignedTable(w, t.Header, t.Rows)
	}
}

// alignedTable prints the table as space-padded columns, for human reading.
func alignedTable(w io.Writer, header []string, rows [][]string) error {
	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = len(h)
	}
	for _, r := range rows {
		for i := 0; i < len(widths) && i < len(r); i++ {
			if len(r[i]) > widths[i] {
				widths[i] = len(r[i])
			}
		}
	}
	put := func(cells []string) {
		var b strings.Builder
		for i := range widths {
			c := ""
			if i < len(cells) {
				c = cells[i]
			}
			if i == len(widths)-1 {
				b.WriteString(c)
			} else {
				b.WriteString(c + strings.Repeat(" ", widths[i]-len(c)+2))
			}
		}
		fmt.Fprintln(w, b.String())
	}
	put(header)
	for _, r := range rows {
		put(r)
	}
	return nil
}
