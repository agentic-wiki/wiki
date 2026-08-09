package bundle

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestOKFVersion(t *testing.T) {
	if v, ok := (&Bundle{Spec: "0.1"}).OKFVersion(); !ok || v != "0.1" {
		t.Errorf("OKFVersion(0.1) = %q, %v; want 0.1, true", v, ok)
	}
	if _, ok := (&Bundle{Spec: "9.9"}).OKFVersion(); ok {
		t.Errorf("unknown spec should not embed OKF")
	}
}

// loadConfig writes a wiki.toml and loads it, returning the bundle or the error.
func loadConfig(t *testing.T, toml string) (*Bundle, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "wiki.toml"), []byte(toml), 0o644); err != nil {
		t.Fatal(err)
	}
	return Discover(dir)
}

func mustLoad(t *testing.T, toml string) *Bundle {
	t.Helper()
	b, err := loadConfig(t, toml)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLoadConfig(t *testing.T) {
	b := mustLoad(t, `
spec = "0.1"
types = ["note", "concept"]   # a trailing comment
ignore = ["AGENTS.md", "../PRD.md"]
ignore_orphans = ["backlog/**"]
`)
	if b.Spec != "0.1" {
		t.Errorf("spec=%q", b.Spec)
	}
	if !reflect.DeepEqual(b.Types, []string{"note", "concept"}) {
		t.Errorf("types=%#v", b.Types)
	}
	if !reflect.DeepEqual(b.Ignore, []string{"AGENTS.md", "../PRD.md"}) {
		t.Errorf("ignore=%#v", b.Ignore)
	}
	if !reflect.DeepEqual(b.IgnoreOrphans, []string{"backlog/**"}) {
		t.Errorf("ignore_orphans=%#v", b.IgnoreOrphans)
	}
	if b.Unknown != nil {
		t.Errorf("a clean config should have no unknown keys, got %#v", b.Unknown)
	}
}

// A line-based reader saw `types = [` and produced an empty list, which reads as
// "no vocabulary declared" and so allowed every type — the opposite of what the
// author wrote, with nothing reported.
func TestMultilineArray(t *testing.T) {
	b := mustLoad(t, `
spec = "0.1"
types = [
  "task",
  "note",
]
`)
	if !reflect.DeepEqual(b.Types, []string{"task", "note"}) {
		t.Errorf("types=%#v, want the two declared types", b.Types)
	}
	if b.KnownType("bogus") {
		t.Error("a declared vocabulary must reject an undeclared type")
	}
}

// The namespace is space granted to other tools: never validated, never warned
// about, and never mistaken for bundle config.
func TestToolNamespaceIsOpaque(t *testing.T) {
	b := mustLoad(t, `
spec = "0.1"
types = ["task", "note"]

[tool.wikiview]
default_board = "/backlog"

[[tool.wikiview.board]]
path = "/backlog"
columns = ["todo", "done"]

[tool.other]
types = ["this", "must", "not", "leak"]
`)
	if len(b.Unknown) != 0 {
		t.Errorf("[tool.*] must not warn, got %#v", b.Unknown)
	}
	// A line reader treated every key as top-level, so a tool's `types` silently
	// replaced the bundle's vocabulary.
	if !reflect.DeepEqual(b.Types, []string{"task", "note"}) {
		t.Errorf("a tool table leaked into bundle config: types=%#v", b.Types)
	}
	if _, ok := b.Tool["wikiview"]; !ok {
		t.Errorf("Tool should carry the wikiview table, got keys %v", b.Tool)
	}
}

func TestDecodeTool(t *testing.T) {
	b := mustLoad(t, `
spec = "0.1"

[tool.wikiview]
default_board = "/backlog"

[[tool.wikiview.board]]
path = "/backlog"
where = ["type=task"]
columns = ["todo", "done"]
`)
	var cfg struct {
		DefaultBoard string `toml:"default_board"`
		Board        []struct {
			Path    string   `toml:"path"`
			Where   []string `toml:"where"`
			Columns []string `toml:"columns"`
		} `toml:"board"`
	}
	found, err := b.DecodeTool("wikiview", &cfg)
	if err != nil || !found {
		t.Fatalf("DecodeTool: found=%v err=%v", found, err)
	}
	if cfg.DefaultBoard != "/backlog" || len(cfg.Board) != 1 {
		t.Fatalf("decoded %+v", cfg)
	}
	if got := cfg.Board[0]; got.Path != "/backlog" ||
		!reflect.DeepEqual(got.Where, []string{"type=task"}) ||
		!reflect.DeepEqual(got.Columns, []string{"todo", "done"}) {
		t.Errorf("board = %+v", got)
	}
	// An absent table is not an error; it is a bundle that does not use the tool.
	if found, err := b.DecodeTool("absent", &cfg); found || err != nil {
		t.Errorf("absent tool: found=%v err=%v", found, err)
	}
}

func TestUnknownKeysReportFullPath(t *testing.T) {
	// A renamed field (the old `skip`) or a typo is inert, so surface it rather
	// than let the author assume it took effect.
	b := mustLoad(t, `
spec = "0.1"
skip = ["AGENTS.md"]
tpyes = ["note"]

[nested]
key = "value"
`)
	// The shallowest unrecognized key only: [nested] is flagged, its contents are
	// implied.
	if want := []string{"skip", "tpyes", "nested"}; !reflect.DeepEqual(b.Unknown, want) {
		t.Errorf("unknown=%#v, want %#v", b.Unknown, want)
	}

	// A key nested under a *recognized* table still reports its full path, since
	// a bare "key" cannot be found in a file with several tables.
	b = mustLoad(t, "spec = \"0.1\"\n\n[tool.x]\nfine = 1\n\n[nested]\na = 1\nb = 2\n")
	if !slices.Contains(b.Unknown, "nested") || slices.Contains(b.Unknown, "nested.a") {
		t.Errorf("unknown=%#v", b.Unknown)
	}
}

// Reading half a config and carrying on produces confidently wrong answers: the
// config decides what is an entry and which types are valid.
func TestMalformedConfigIsAnError(t *testing.T) {
	for _, tc := range []struct{ name, toml string }{
		{"unterminated array", "spec = \"0.1\"\ntypes = [\"a\", \n"},
		{"bare token", "spec = \"0.1\"\ntypes = [note, concept]\n"},
		{"missing value", "spec =\n"},
		{"junk line", "spec = \"0.1\"\nthis is not toml\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := loadConfig(t, tc.toml); err == nil {
				t.Error("malformed wiki.toml should not load silently")
			}
		})
	}
}

func TestKnownType(t *testing.T) {
	b := &Bundle{Types: []string{"note", "concept"}}
	for _, ty := range []string{"note", "concept"} {
		if !b.KnownType(ty) {
			t.Errorf("%q should be known", ty)
		}
	}
	// With a declared vocabulary, an undeclared type is unknown.
	for _, ty := range []string{"index", "log", "bogus"} {
		if b.KnownType(ty) {
			t.Errorf("%q is not a declared content type", ty)
		}
	}
	// No declared vocabulary (opt-in): every type is allowed.
	none := &Bundle{}
	for _, ty := range []string{"note", "anything", "made-up"} {
		if !none.KnownType(ty) {
			t.Errorf("%q should be allowed when no vocabulary is declared", ty)
		}
	}
}

func TestDiscoverWalksUp(t *testing.T) {
	root := t.TempDir()
	writeTOML(t, root)
	deep := filepath.Join(root, "a", "b") // content lives at the bundle root, no wiki/ subfolder
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	p := mustDiscover(t, deep)
	if realpath(t, p.Dir) != realpath(t, root) {
		t.Errorf("Dir=%q want %q", p.Dir, root)
	}
	if p.Spec != "0.1" {
		t.Errorf("spec=%q", p.Spec)
	}
}

func TestDiscoverExactDir(t *testing.T) {
	root := t.TempDir()
	writeTOML(t, root)
	p := mustDiscover(t, root) // wiki.toml is right here, no walking
	if realpath(t, p.Dir) != realpath(t, root) {
		t.Errorf("Dir=%q want %q", p.Dir, root)
	}
}

func TestDiscoverNotFound(t *testing.T) {
	if _, err := Discover(t.TempDir()); err != ErrNotFound {
		t.Errorf("err=%v want ErrNotFound", err)
	}
}

func writeTOML(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "wiki.toml"), []byte("spec=\"0.1\"\ntypes=[\"note\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustDiscover(t *testing.T, dir string) *Bundle {
	t.Helper()
	p, err := Discover(dir)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func realpath(t *testing.T, p string) string {
	t.Helper()
	r, _ := filepath.EvalSymlinks(p)
	return r
}
