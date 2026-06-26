// Package output renders command results as text or JSON.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

// Emit prints lines (text) or marshals v (json) to w.
func Emit(w io.Writer, format string, lines []string, v any) error {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(jsonable(v))
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
