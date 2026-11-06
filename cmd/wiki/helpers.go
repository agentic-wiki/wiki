package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/agentic-wiki/wiki/bundle"
	"github.com/agentic-wiki/wiki/index"
	"github.com/agentic-wiki/wiki/internal/output"
)

// loadIndex discovers the bundle (from --root if given, else the current
// directory) and builds the index. On failure it prints to stderr and returns
// a non-zero exit code.
//
// It is the one place that warns about unreadable headers: one stderr line per
// entry whose frontmatter couldn't be read, whether or not the command shows
// that entry, and with no change to the exit code. check, which reports these
// entries as errors, calls buildIndex instead.
func loadIndex() (*index.Index, int) {
	idx, code := buildIndex()
	if idx == nil {
		return nil, code
	}
	for _, e := range idx.Entries {
		if err := e.FrontmatterErr(); err != nil {
			fmt.Fprintf(os.Stderr, "wiki: warning: %s: frontmatter not read (%s); run wiki check\n", e.Path, strings.TrimPrefix(err.Error(), "yaml: "))
		}
	}
	return idx, code
}

// buildIndex is loadIndex without the warnings.
func buildIndex() (*index.Index, int) {
	start := rootDir
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintln(os.Stderr, "wiki:", err)
			return nil, 2
		}
		start = cwd
	}
	b, err := bundle.Discover(start)
	if errors.Is(err, bundle.ErrNotFound) && rootDir == "" {
		// The library cannot name a CLI flag, so the way out is added here.
		fmt.Fprintf(os.Stderr, "wiki: %v\nrun it inside a bundle, or point at one with --root <dir>\n", err)
		return nil, 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return nil, 2
	}
	idx, err := index.Build(b)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return nil, 2
	}
	return idx, 0
}

// formatValue is the --format flag. Only text, json, jsonl, csv and tsv are
// accepted, so a format wiki lacks is a flag error (exit 2) rather than text
// output.
type formatValue string

func (f *formatValue) String() string { return string(*f) }

func (f *formatValue) Set(s string) error {
	switch s {
	case "text", "json", "jsonl", "csv", "tsv":
		*f = formatValue(s)
		return nil
	}
	return fmt.Errorf("unknown --format %q (use text|json|jsonl|csv|tsv)", s)
}

// formatFlag registers --format on fs, defaulting to text: every command that
// takes --format declares it through here.
func formatFlag(fs *flag.FlagSet) *string {
	f := formatValue("text")
	fs.Var(&f, "format", "output `format`: text|json|jsonl|csv|tsv")
	return (*string)(&f)
}

// emit prints a command's result (output.Emit) and returns the exit code: 0,
// or 2 when it can't be printed, such as a result with no rows for csv, so a
// command never prints nothing and exits 0.
func emit(format string, lines []string, v any) int {
	return exitOn(output.Emit(os.Stdout, format, lines, v))
}

// emitRows is emit with the csv/tsv rows given (output.EmitRows).
func emitRows(format string, lines []string, v any, rows output.Rows) int {
	return exitOn(output.EmitRows(os.Stdout, format, lines, v, rows))
}

// exitOn prints err, if any, as `wiki: <err>` and returns the exit code.
func exitOn(err error) int {
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
	}
	return 0
}
