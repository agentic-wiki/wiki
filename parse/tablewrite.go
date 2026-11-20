package parse

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/agentic-wiki/wiki/internal/sheet"
)

// WriteCSV writes the table as CSV: the header, then each row's cells, every
// field quoted (internal/sheet), so a `;` can't split a cell either. It is
// what `wiki table --format csv` prints, so an app that exports a dataset
// gives the same bytes as the CLI.
//
// A cell a spreadsheet would run as a formula gets a leading `'`, the rule
// every CSV and TSV of wiki follows (internal/sheet): CSV is opened in Excel
// and Sheets, and a dataset may hold text an agent copied from an email or a
// web page. WriteJSON is the exact form, with every value as written.
func (t Table) WriteCSV(w io.Writer) error { return t.writeDelimited(w, ',') }

// WriteTSV writes the table as WriteCSV does, separated by tabs: what
// `wiki table --format tsv` prints, with formulas disarmed the same way.
func (t Table) WriteTSV(w io.Writer) error { return t.writeDelimited(w, '\t') }

func (t Table) writeDelimited(w io.Writer, comma rune) error {
	return sheet.Write(w, comma, append([][]string{t.Header}, t.Rows...))
}

// WriteJSON writes the table as a JSON array with one object per row, its keys
// the header in column order and its values typed by TypedRows, indented by two
// spaces. It is what `wiki table --format json` prints, so an app that exports
// a dataset gives the same bytes as the CLI.
func (t Table) WriteJSON(w io.Writer) error {
	values := t.TypedRows()
	objs := make([]orderedObj, len(values))
	for i, r := range values {
		objs[i] = orderedObj{t.Header, r}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(objs)
}

// orderedObj marshals to a JSON object preserving key order (a table's column
// order), which encoding/json's sorted-key map handling would not.
type orderedObj struct {
	keys []string
	vals []any
}

func (o orderedObj) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		var val any
		if i < len(o.vals) {
			val = o.vals[i]
		}
		kb, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		vb, err := json.Marshal(val)
		if err != nil {
			return nil, err
		}
		b.Write(kb)
		b.WriteByte(':')
		b.Write(vb)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}
