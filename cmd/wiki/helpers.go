package main

import (
	"fmt"
	"os"

	"github.com/agentic-wiki/wiki/internal/index"
	"github.com/agentic-wiki/wiki/internal/project"
)

// loadIndex discovers the bundle from the current directory and builds the
// index. On failure it prints to stderr and returns a non-zero exit code.
func loadIndex() (*index.Index, int) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return nil, 2
	}
	proj, err := project.Discover(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return nil, 2
	}
	idx, err := index.Build(proj)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return nil, 2
	}
	return idx, 0
}
