package index

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/bundle"
)

// BenchmarkBuild2300 builds a bundle of 2,300 entries shaped like the
// requesting team's dataset pages (1.6 KB of frontmatter: six scalars, a map of
// dates, and six raw records of eight fields with a nested map). A benchmark
// only: no test asserts time.
func BenchmarkBuild2300(b *testing.B) {
	root := b.TempDir()
	if err := os.WriteFile(filepath.Join(root, "wiki.toml"), []byte("spec=\"0.1\"\ntypes=[\"dataset\"]\n"), 0o644); err != nil {
		b.Fatal(err)
	}
	for i := range 2300 {
		var h strings.Builder
		fmt.Fprintf(&h, "---\ntype: dataset\ntitle: 'Acme''s filings %d'\ncompany: acme%d\nticker: ACME\ncik: \"0000123456\"\nupdated: 2026-10-01\n", i, i)
		h.WriteString("data_as_of:\n  income: 2025-12-31\n  balance: 2025-09-30\nraw:\n")
		for r := range 6 {
			fmt.Fprintf(&h, "  - path: sources/acme/10-K-%d.pdf\n    sha256: 9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a0%d\n", 2015+r, r)
			fmt.Fprintf(&h, "    fetched: 2026-10-01T12:00:00Z\n    pages: %d\n    bytes: 1048576\n    source: sec\n    meta:\n      form: 10-K\n      year: %d\n", 40+r, 2015+r)
		}
		fmt.Fprintf(&h, "---\n# Acme %d\n\nSee [root](/index.md).\n", i)
		if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("e%d.md", i)), []byte(h.String()), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	bun, err := bundle.Discover(root)
	if err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := Build(bun); err != nil {
			b.Fatal(err)
		}
	}
}
