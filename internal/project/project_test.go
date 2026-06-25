package project

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseConfig(t *testing.T) {
	spec, types := parseConfig("spec = \"0.1\"\ntypes = [\"note\", \"concept\"]\n# a comment\n")
	if spec != "0.1" {
		t.Errorf("spec=%q", spec)
	}
	if !reflect.DeepEqual(types, []string{"note", "concept"}) {
		t.Errorf("types=%#v", types)
	}
}

func TestParseConfigMessy(t *testing.T) {
	// Spaces, bare + quoted tokens; internal space preserved; no spec line.
	if _, types := parseConfig("types = [ \"a\" , b ,  \"c d\" ]\n"); !reflect.DeepEqual(types, []string{"a", "b", "c d"}) {
		t.Errorf("types=%#v", types)
	}
	if spec, empty := parseConfig("types = []\n"); spec != "" || empty != nil {
		t.Errorf("spec=%q types=%#v, want empty", spec, empty)
	}
}

func TestKnownType(t *testing.T) {
	p := &Project{Types: []string{"note"}}
	for _, ty := range []string{"note", "index", "log"} {
		if !p.KnownType(ty) {
			t.Errorf("%q should be known", ty)
		}
	}
	if p.KnownType("bogus") {
		t.Errorf("bogus should be unknown")
	}
}

func TestDiscoverWalksUp(t *testing.T) {
	root := t.TempDir()
	writeTOML(t, root)
	deep := filepath.Join(root, "wiki", "a", "b")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	p := mustDiscover(t, deep)
	if realpath(t, p.RootDir) != realpath(t, root) {
		t.Errorf("RootDir=%q want %q", p.RootDir, root)
	}
	if p.ContentDir != filepath.Join(p.RootDir, "wiki") {
		t.Errorf("ContentDir=%q", p.ContentDir)
	}
	if p.Spec != "0.1" {
		t.Errorf("spec=%q", p.Spec)
	}
}

func TestDiscoverExactDir(t *testing.T) {
	root := t.TempDir()
	writeTOML(t, root)
	p := mustDiscover(t, root) // wiki.toml is right here, no walking
	if realpath(t, p.RootDir) != realpath(t, root) {
		t.Errorf("RootDir=%q want %q", p.RootDir, root)
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

func mustDiscover(t *testing.T, dir string) *Project {
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
