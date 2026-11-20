// Package sheet keeps what wiki writes as CSV or TSV safe to open in a
// spreadsheet: one rule, for every command that prints either.
package sheet

import (
	"regexp"
	"strings"
)

// formulaStart holds the first characters on which a spreadsheet reads a cell
// as a formula.
const formulaStart = "=+-@\t\r"

// skipped are what a spreadsheet strips before it reads a cell's first
// character: a byte order mark (U+FEFF), spaces, no-break spaces. A BOM
// before `=1+1` still runs it.
const skipped = "\ufeff \u00a0"

// plainNumber is a decimal number with an optional sign, fraction and
// exponent: `-3`, `+1.5`, `-0.5e3`. A spreadsheet reads it as a number, never
// as a formula, so it keeps its sign.
var plainNumber = regexp.MustCompile(`^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$`)

// Safe returns the cells, with a `'` before each one that would start like a
// formula once a spreadsheet strips its leading BOM and spaces, and isn't a
// plain number: `=1`, `-3+A1`, `@SUM(A1)`, ` =x`. The `'` makes the
// spreadsheet show the cell as text; the cell itself is kept as written.
func Safe(cells []string) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		lead := strings.TrimLeft(c, skipped)
		if lead != "" && strings.ContainsRune(formulaStart, rune(lead[0])) && !plainNumber.MatchString(lead) {
			c = "'" + c
		}
		out[i] = c
	}
	return out
}
