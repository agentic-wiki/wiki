package bundle

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestParseConfig(t *testing.T) {
	spec, types, _, _ := parseConfig("spec = \"0.1\"\ntypes = [\"note\", \"concept\"]\n# a comment\n")
	if spec != "0.1" {
		t.Errorf("spec=%q", spec)
	}
	if !reflect.DeepEqual(types, []string{"note", "concept"}) {
		t.Errorf("types=%#v", types)
	}
}

func TestParseConfigMessy(t *testing.T) {
	// Spaces, bare + quoted tokens; internal space preserved; no spec line.
	if _, types, _, _ := parseConfig("types = [ \"a\" , b ,  \"c d\" ]\n"); !reflect.DeepEqual(types, []string{"a", "b", "c d"}) {
		t.Errorf("types=%#v", types)
	}
	if spec, empty, _, _ := parseConfig("types = []\n"); spec != "" || empty != nil {
		t.Errorf("spec=%q types=%#v, want empty", spec, empty)
	}
}

func TestParseConfigIgnore(t *testing.T) {
	_, _, ignore, orphans := parseConfig("spec=\"0.1\"\ntypes=[\"note\"]\nignore=[\"AGENTS.md\", \"../PRD.md\"]\nignore_orphans=[\"backlog/**\"]\n")
	if !reflect.DeepEqual(ignore, []string{"AGENTS.md", "../PRD.md"}) {
		t.Errorf("ignore=%#v", ignore)
	}
	if !reflect.DeepEqual(orphans, []string{"backlog/**"}) {
		t.Errorf("ignore_orphans=%#v", orphans)
	}
}

func TestKnownType(t *testing.T) {
	b := &Bundle{Types: []string{"note", "concept"}}
	for _, ty := range []string{"note", "concept"} {
		if !b.KnownType(ty) {
			t.Errorf("%q should be known", ty)
		}
	}
	// index/log are reserved filenames, not content types; bogus is undeclared.
	for _, ty := range []string{"index", "log", "bogus"} {
		if b.KnownType(ty) {
			t.Errorf("%q is not a declared content type", ty)
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
