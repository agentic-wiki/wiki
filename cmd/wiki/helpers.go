package main

import (
	"fmt"
	"os"

	"github.com/agentic-wiki/wiki/internal/bundle"
	"github.com/agentic-wiki/wiki/internal/index"
)

// loadIndex discovers the bundle (from --root if given, else the current
// directory) and builds the index. On failure it prints to stderr and returns
// a non-zero exit code.
func loadIndex() (*index.Index, int) {
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
