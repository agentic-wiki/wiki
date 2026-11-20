package bundle

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
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

[tool.wikishell]
default_board = "/backlog"

[[tool.wikishell.board]]
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
	if got := b.Tools(); !reflect.DeepEqual(got, []string{"other", "wikishell"}) {
		t.Errorf("Tools() = %v", got)
	}
}

func TestDecodeTool(t *testing.T) {
	b := mustLoad(t, `
spec = "0.1"

[tool.wikishell]
default_board = "/backlog"

[[tool.wikishell.board]]
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
	found, err := b.DecodeTool("wikishell", &cfg)
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

// [[board]] and [[graph]] belong to the apps that show them. wiki sets them
// aside, as it does [tool.*]: no warning, and nothing leaks into its own config.
func TestBoardAndGraphAreOpaque(t *testing.T) {
	b := mustLoad(t, `
spec = "0.1"
types = ["task"]

[[board]]
id = "backlog"
path = "/backlog"
types = ["this", "must", "not", "leak"]

[[graph]]
id = "map"
path = "/"
neighbours = true
`)
	if len(b.Unknown) != 0 {
		t.Errorf("[[board]] and [[graph]] must not warn, got %#v", b.Unknown)
	}
	if !reflect.DeepEqual(b.Types, []string{"task"}) {
		t.Errorf("a board table leaked into bundle config: types=%#v", b.Types)
	}
}

// A wrong type is the reading app's problem to report. wiki loads the bundle
// and stays quiet; the decode is where the error shows.
func TestBoardOfWrongTypeIsNotWikisProblem(t *testing.T) {
	b := mustLoad(t, `
spec = "0.1"
board = "x"
graph = 1
`)
	if len(b.Unknown) != 0 {
		t.Errorf("a mistyped board must not warn, got %#v", b.Unknown)
	}
	var boards []struct {
		ID string `toml:"id"`
	}
	found, err := b.DecodeBoards(&boards)
	if !found || err == nil || !strings.Contains(err.Error(), "wiki.toml [[board]]") {
		t.Errorf("DecodeBoards: found=%v err=%v, want true and an error naming [[board]]", found, err)
	}
	var graphs []struct {
		ID string `toml:"id"`
	}
	found, err = b.DecodeGraphs(&graphs)
	if !found || err == nil || !strings.Contains(err.Error(), "wiki.toml [[graph]]") {
		t.Errorf("DecodeGraphs: found=%v err=%v, want true and an error naming [[graph]]", found, err)
	}
}

func TestDecodeBoardsAndGraphs(t *testing.T) {
	b := mustLoad(t, `
spec = "0.1"

[[board]]
id = "backlog"
path = "/backlog"
where = ["type=task"]

[[board]]
id = "bugs"
path = "/backlog"
lane = "priority"

[[graph]]
id = "map"
path = "/"
neighbours = true
`)
	var boards []struct {
		ID    string   `toml:"id"`
		Path  string   `toml:"path"`
		Where []string `toml:"where"`
		Lane  string   `toml:"lane"`
	}
	found, err := b.DecodeBoards(&boards)
	if err != nil || !found {
		t.Fatalf("DecodeBoards: found=%v err=%v", found, err)
	}
	if len(boards) != 2 || boards[0].ID != "backlog" || boards[1].Lane != "priority" ||
		!reflect.DeepEqual(boards[0].Where, []string{"type=task"}) {
		t.Errorf("boards = %+v", boards)
	}

	// A slice of maps works too, for an app that wants to see every key written.
	var graphs []map[string]any
	found, err = b.DecodeGraphs(&graphs)
	if err != nil || !found {
		t.Fatalf("DecodeGraphs: found=%v err=%v", found, err)
	}
	if len(graphs) != 1 || graphs[0]["id"] != "map" || graphs[0]["neighbours"] != true {
		t.Errorf("graphs = %#v", graphs)
	}
}

// No tables is not an error: it is a bundle that declares no views.
func TestDecodeBoardsAndGraphsAbsent(t *testing.T) {
	b := mustLoad(t, "spec = \"0.1\"\n")
	var v []map[string]any
	if found, err := b.DecodeBoards(&v); found || err != nil {
		t.Errorf("DecodeBoards: found=%v err=%v", found, err)
	}
	if found, err := b.DecodeGraphs(&v); found || err != nil {
		t.Errorf("DecodeGraphs: found=%v err=%v", found, err)
	}
}

// A decode writes to the metadata it reads from. Without the lock, two decodes
// at once on one bundle are a concurrent map write, which ends the process.
// Apps share one bundle across requests, so every decode must be safe to run
// in parallel. Run with -race to see a missing lock.
func TestDecodesRunInParallel(t *testing.T) {
	b := mustLoad(t, `
spec = "0.1"

[[board]]
id = "backlog"
path = "/backlog"

[[graph]]
id = "map"
path = "/"

[tool.app]
name = "x"
`)
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			var v []map[string]any
			if _, err := b.DecodeBoards(&v); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			var v []map[string]any
			if _, err := b.DecodeGraphs(&v); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			var v map[string]any
			if _, err := b.DecodeTool("app", &v); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
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
