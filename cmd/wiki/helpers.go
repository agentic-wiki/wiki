package main

import (
	"fmt"
	"os"

	"github.com/agentic-wiki/wiki/internal/bundle"
	"github.com/agentic-wiki/wiki/internal/index"
)

// loadIndex discovers the bundle from the current directory and builds the
// index. On failure it prints to stderr and returns a non-zero exit code.
func loadIndex() (*index.Index, int) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return nil, 2
	}
	b, err := bundle.Discover(cwd)
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
