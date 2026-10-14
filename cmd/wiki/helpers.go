package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/agentic-wiki/wiki/bundle"
	"github.com/agentic-wiki/wiki/index"
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
