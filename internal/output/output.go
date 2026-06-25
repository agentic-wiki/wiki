// Package output renders command results as text or JSON.
package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// Emit prints lines (text) or marshals v (json) to w.
func Emit(w io.Writer, format string, lines []string, v any) error {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	}
	for _, l := range lines {
		fmt.Fprintln(w, l)
	}
	return nil
}
