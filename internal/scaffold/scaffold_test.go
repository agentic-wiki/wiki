package scaffold

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/bundle"
	"github.com/agentic-wiki/wiki/index"
	"github.com/agentic-wiki/wiki/parse"
)

// shapes is every set of parts, each in parts order: the empty one and the 15
// non-empty ones.
func shapes() [][]string {
	var out [][]string
	for mask := range 1 << len(parts) {
		var shape []string
		for i, p := range parts {
			if mask&(1<<i) != 0 {
				shape = append(shape, p.name)
			}
		}
		out = append(out, shape)
	}
	return out
}

// examples are where each part's frontmatter template is written as an entry,
// under the part's folder.
var examples = map[string]string{"specs": "example.md", "kanban": "example-task.md", "data": "companies/example.md", "knowledge": "inbox/example.md"}

// writeTemplates writes each part's frontmatter template (the first markdown
// block of its AGENTS.md section) as an entry, and links the specs one from
// the product map.
func writeTemplates(t *testing.T, dir string, shape []string) {
	t.Helper()
	for _, name := range shape {
		body, err := files.ReadFile("files/parts/" + name + "/AGENTS.md")
		if err != nil {
			t.Fatal(err)
		}
		_, template, _ := strings.Cut(string(body), "```markdown\n")
		template, _, _ = strings.Cut(template, "```\n")
		p := parts[slices.IndexFunc(parts, func(p part) bool { return p.name == name })]
		at := filepath.Join(dir, filepath.FromSlash(prefix(p, len(shape))))
		write(t, filepath.Join(at, examples[name]), template)
		if name == "specs" {
			index, _ := os.ReadFile(filepath.Join(at, "index.md"))
			write(t, filepath.Join(at, "index.md"), string(index)+"- [Example](example.md)\n")
		}
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Every set of parts writes a wiki that passes check with no error or warning
// and has no orphan, with each part's frontmatter template written as an entry.
func TestShapesPassCheck(t *testing.T) {
	for _, shape := range shapes() {
		t.Run(strings.Join(append([]string{"shape"}, shape...), "-"), func(t *testing.T) {
			dir := t.TempDir()
			if _, err := Write(dir, shape, false); err != nil {
				t.Fatal(err)
			}
			writeTemplates(t, dir, shape)
			b, err := bundle.Discover(dir)
			if err != nil {
				t.Fatal(err)
			}
			idx, err := index.Build(b)
			if err != nil {
				t.Fatal(err)
			}
			if issues := idx.Check(); len(issues) != 0 {
				t.Errorf("check: %+v", issues)
			}
			if orphans := idx.Orphans(); len(orphans) != 0 {
				t.Errorf("orphans: %v", orphans[0].Path)
			}
		})
	}
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// One part fills the root; two or more each get their folder, linked from the
// root index.md, with their AGENTS.md sections and wiki.toml lists. There is no
// WORKFLOW.md, and CLAUDE.md holds one line, @AGENTS.md.
func TestShapeLayout(t *testing.T) {
	dir := t.TempDir()
	if _, err := Write(dir, []string{"kanban"}, false); err != nil {
		t.Fatal(err)
	}
	if got := read(t, dir, "index.md"); !strings.HasPrefix(got, "---\nokf_version: \"0.1\"\n---\n\n# Backlog\n\n") {
		t.Errorf("kanban index.md: %q", got)
	}
	read(t, dir, "archive/index.md")
	if got := read(t, dir, "AGENTS.md"); !strings.Contains(got, "\n## What this wiki holds\n") || !strings.Contains(got, "\n### Backlog\n\n") {
		t.Errorf("kanban AGENTS.md has no Backlog section:\n%s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "backlog")); err == nil {
		t.Error("kanban alone wrote backlog/")
	}
	if got := read(t, dir, "wiki.toml"); !strings.Contains(got, "Here that is everything") || !strings.Contains(got, "# path    = \"/\"\n") {
		t.Errorf("kanban wiki.toml:\n%s", got)
	}

	dir = t.TempDir()
	if _, err := Write(dir, []string{"specs", "kanban"}, false); err != nil {
		t.Fatal(err)
	}
	root := read(t, dir, "index.md")
	product, backlog := strings.Index(root, "- [Product](product/index.md)"), strings.Index(root, "- [Backlog](backlog/index.md)")
	if product < 0 || backlog < product {
		t.Errorf("root index.md: %q", root)
	}
	if got := read(t, dir, "product/index.md"); !strings.HasPrefix(got, "# Product\n\n") {
		t.Errorf("product/index.md: %q", got)
	}
	read(t, dir, "backlog/archive/index.md")
	agents := read(t, dir, "AGENTS.md")
	shared := strings.Index(agents, "# Operating this wiki")
	product, backlog = strings.Index(agents, "\n### Product in `product/`\n"), strings.Index(agents, "\n### Backlog in `backlog/`\n")
	holds := strings.Index(agents, "\n## What this wiki holds\n")
	if shared != 0 || holds < 0 || holds > product || product < 0 || backlog < product || strings.Contains(agents, "{dir}") {
		t.Errorf("AGENTS.md:\n%s", agents)
	}
	toml := read(t, dir, "wiki.toml")
	for _, want := range []string{`types = ["spec", "note", "task"]`, `ignore_orphans = ["backlog/**"]`, `# path    = "/backlog"`} {
		if !strings.Contains(toml, want) {
			t.Errorf("wiki.toml lacks %s:\n%s", want, toml)
		}
	}

	dir = t.TempDir()
	if _, err := Write(dir, nil, false); err != nil {
		t.Fatal(err)
	}
	sharedText, _ := files.ReadFile("files/AGENTS.md")
	if got := read(t, dir, "AGENTS.md"); got != string(sharedText) {
		t.Errorf("the minimal starter's AGENTS.md is not the shared one alone:\n%s", got)
	}
	if got := read(t, dir, "wiki.toml"); !strings.Contains(got, "# types = [\"note\"]\n") || strings.Contains(got, "ignore_orphans") {
		t.Errorf("minimal wiki.toml:\n%s", got)
	}
}

// A part's name that is empty, unknown or repeated is refused, and nothing is
// written.
func TestShapeErrors(t *testing.T) {
	for _, tc := range []struct {
		shape []string
		want  string
	}{
		{[]string{"nope"}, `unknown part "nope" in --shape (use knowledge, kanban, data or specs)`},
		{[]string{"kanban", "kanban"}, `part "kanban" is listed twice in --shape`},
		{[]string{"kanban", ""}, "empty part in --shape"},
	} {
		dir := t.TempDir()
		_, err := Write(dir, tc.shape, false)
		if err == nil || err.Error() != tc.want {
			t.Errorf("%q: %v, want %q", tc.shape, err, tc.want)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("%q: wrote %d files", tc.shape, len(entries))
		}
	}
}

func TestWrite(t *testing.T) {
	dir := t.TempDir()
	written, err := Write(dir, []string{"specs"}, false)
	if err != nil {
		t.Fatal(err)
	}
	// the ignore file lands as .gitignore, never a bare "gitignore"
	if slices.Contains(written, "gitignore") || !slices.Contains(written, ".gitignore") {
		t.Errorf("written = %v (want .gitignore, not gitignore)", written)
	}
	for _, f := range []string{"wiki.toml", ".gitignore", "index.md", "AGENTS.md", "CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	// CLAUDE.md is a plain file whose one line imports AGENTS.md for Claude Code.
	if info, err := os.Lstat(filepath.Join(dir, "CLAUDE.md")); err != nil || !info.Mode().IsRegular() {
		t.Errorf("CLAUDE.md: %v, %v; want a plain file", info, err)
	}
	if got := read(t, dir, "CLAUDE.md"); got != "@AGENTS.md\n" {
		t.Errorf("CLAUDE.md = %q, want %q", got, "@AGENTS.md\n")
	}
	if _, err := os.Stat(filepath.Join(dir, "WORKFLOW.md")); err == nil {
		t.Error("a WORKFLOW.md was written")
	}
	// a non-empty target is refused without force, allowed with it
	if _, err := Write(dir, nil, false); err == nil {
		t.Errorf("re-write without force should error")
	}
	if _, err := Write(dir, nil, true); err != nil {
		t.Errorf("force re-write: %v", err)
	}
}

// A forced init over a wiki from an earlier init replaces its CLAUDE.md
// symlink with the one-line file, without writing through it to AGENTS.md, and
// leaves the user's old WORKFLOW.md untouched.
func TestWriteForceOverEarlierInit(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "AGENTS.md"), "old manual\n")
	write(t, filepath.Join(dir, "WORKFLOW.md"), "old workflow\n")
	if err := os.Symlink("AGENTS.md", filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Skip("cannot make a symlink here (Windows may refuse it):", err)
	}
	if _, err := Write(dir, nil, true); err != nil {
		t.Fatal(err)
	}
	shared, _ := files.ReadFile("files/AGENTS.md")
	if got := read(t, dir, "AGENTS.md"); got != string(shared) {
		t.Errorf("AGENTS.md is not the manual:\n%s", got)
	}
	if info, err := os.Lstat(filepath.Join(dir, "CLAUDE.md")); err != nil || !info.Mode().IsRegular() {
		t.Errorf("CLAUDE.md: %v, %v; want a plain file", info, err)
	}
	if got := read(t, dir, "CLAUDE.md"); got != "@AGENTS.md\n" {
		t.Errorf("CLAUDE.md = %q, want %q", got, "@AGENTS.md\n")
	}
	if got := read(t, dir, "WORKFLOW.md"); got != "old workflow\n" {
		t.Errorf("WORKFLOW.md = %q, want it untouched", got)
	}
}

// A lone .git directory does not make the target "non-empty": a freshly
// `git init`'d (or empty-cloned) repo is a normal init target.
func TestWriteToleratesGitDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir, nil, false); err != nil {
		t.Errorf("init into a .git-only dir should succeed without force: %v", err)
	}

	// .git alongside real content still requires --force
	dir2 := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir2, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "notes.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(dir2, nil, false); err == nil {
		t.Errorf("init into a dir with .git + content should still require force")
	}
}

// TestScaffoldIsOKFConformant locks the OKF v0.1 MUST-level rules on the bundle
// `wiki init` emits, independently of `wiki check` (which is an opt-in lint, not
// an OKF gate). It runs for every set of parts, so a part that breaks
// conformance fails here.
func TestScaffoldIsOKFConformant(t *testing.T) {
	for _, shape := range shapes() {
		t.Run(strings.Join(append([]string{"shape"}, shape...), "-"), func(t *testing.T) { assertOKFConformant(t, shape) })
	}
}

func assertOKFConformant(t *testing.T, shape []string) {
	dir := t.TempDir()
	if _, err := Write(dir, shape, false); err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".md") {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		fm, _ := parse.Frontmatter(string(raw))
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		switch d.Name() {
		case "index.md", "log.md":
			// Reserved filenames carry no frontmatter, except the bundle-root
			// index.md, which may carry okf_version and nothing else.
			if rel == "index.md" {
				if v := parse.String(fm, "okf_version"); v != "0.1" {
					t.Errorf("root index.md okf_version = %q, want \"0.1\"", v)
				}
				if len(fm) != 1 {
					t.Errorf("root index.md frontmatter must carry only okf_version, got %v", fm)
				}
			} else if len(fm) != 0 {
				t.Errorf("%s is reserved and must have no frontmatter, got %v", rel, fm)
			}
		case "AGENTS.md", "CLAUDE.md":
			// declared non-entries (wiki.toml `ignore`): operating docs, no type
		default:
			// Every other entry MUST declare a non-empty type.
			if parse.String(fm, "type") == "" {
				t.Errorf("%s: missing required non-empty type", rel)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
