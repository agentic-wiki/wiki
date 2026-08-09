package index

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agentic-wiki/wiki/bundle"
)

// The contract the write API exists to keep: setting a field changes that
// field's lines and nothing else. Every frontmatter shape the YAML subset
// supports, and several it deliberately skips, must survive untouched — a
// parse-and-reserialize would quietly drop exactly what it does not model.
func TestSetFieldPreservesEverythingElse(t *testing.T) {
	const doc = `---
type: task            # a trailing comment
title: "Quoted: with a colon"
status: todo
tags: [feature, ui]
blockers:
  - /active/a.md
  - /active/b.md
nested:
  deep:
    key: value
description: |
  a block scalar
  spanning lines
---

# Body

Prose with a [link](./other.md).
`
	idx := build(t, map[string]string{"index.md": "---\nokf_version: \"0.1\"\n---\n", "a.md": doc})
	e, err := idx.Resolve("/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SetField("status", "in-progress"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "a.md"))
	want := strings.Replace(doc, "status: todo", "status: in-progress", 1)
	if string(got) != want {
		t.Errorf("bytes outside the target line changed.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	// The in-memory entry must not still report the old value.
	if e.Field("status") != "in-progress" {
		t.Errorf("entry not refreshed: status=%q", e.Field("status"))
	}
}

func TestSetFieldShapes(t *testing.T) {
	tests := []struct{ name, doc, key, value, want string }{
		{"bare stays bare", "---\nstatus: todo\n---\nb\n", "status", "done", "---\nstatus: done\n---\nb\n"},
		{"author's quoting kept", "---\nstatus: \"todo\"\n---\nb\n", "status", "done", "---\nstatus: \"done\"\n---\nb\n"},
		{"a value needing quotes gets them", "---\ns: a\n---\nb\n", "s", "in: progress", "---\ns: \"in: progress\"\n---\nb\n"},
		// A bare `true` would parse as a boolean and a bare 0.1 as a float, so
		// neither would still be the string that was set.
		{"yaml keyword quoted", "---\nk: a\n---\nb\n", "k", "true", "---\nk: \"true\"\n---\nb\n"},
		{"number quoted", "---\nk: a\n---\nb\n", "k", "0.1", "---\nk: \"0.1\"\n---\nb\n"},
		{"absent key inserted", "---\ntype: task\n---\nb\n", "status", "todo", "---\ntype: task\nstatus: todo\n---\nb\n"},
		// Replacing a list with a scalar must take the list's items with it, or
		// they orphan into the next key.
		{"block list replaced whole", "---\ntags:\n  - a\n  - b\ntype: task\n---\nb\n", "tags", "c", "---\ntags: c\ntype: task\n---\nb\n"},
		{"flow list replaced", "---\ntags: [a, b]\ntype: task\n---\nb\n", "tags", "c", "---\ntags: c\ntype: task\n---\nb\n"},
		{"crlf survives", "---\r\ns: todo\r\n---\r\nb\r\n", "s", "done", "---\r\ns: done\r\n---\r\nb\r\n"},
		{"no frontmatter gets a block", "# Just a body\n", "s", "todo", "---\ns: todo\n---\n# Just a body\n"},
		// A key that is a prefix of another must not match it.
		{"similar key untouched", "---\ns_note: keep\ns: todo\n---\nb\n", "s", "done", "---\ns_note: keep\ns: done\n---\nb\n"},
		// The body may contain something that looks like frontmatter.
		{"only the leading block", "---\ns: todo\n---\nt\n---\ns: decoy\n---\n", "s", "done", "---\ns: done\n---\nt\n---\ns: decoy\n---\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := setFrontmatterValue(tc.doc, tc.key, tc.value, false)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

// index.md and log.md carry no frontmatter by definition, so giving one a block
// would break the format. The bundle-root index.md is the documented exception.
func TestSetFieldAndReservedFiles(t *testing.T) {
	if _, err := setFrontmatterValue("# Board\n", "status", "todo", true); err != ErrNoFrontmatter {
		t.Errorf("err=%v, want ErrNoFrontmatter", err)
	}
	idx := build(t, map[string]string{
		"index.md":     "---\nokf_version: \"0.1\"\n---\nhome\n",
		"sub/index.md": "# a folder board\n",
	})
	root, _ := idx.Resolve("/index.md")
	if root.reserved() {
		t.Error("the bundle-root index.md carries okf_version, so it is not frontmatter-less")
	}
	sub, _ := idx.Resolve("/sub/index.md")
	if !sub.reserved() {
		t.Error("a folder index.md carries no frontmatter")
	}
}

func TestSetFieldsIsOnePassAndDeterministic(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"a.md":     "---\ntype: task\nstatus: todo\nassignee: john\npriority: high\n---\nbody\n",
	})
	e, _ := idx.Resolve("/a.md")
	if err := e.SetFields(map[string]any{"status": "done", "assignee": "mary"}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "a.md"))
	want := "---\ntype: task\nstatus: done\nassignee: mary\npriority: high\n---\nbody\n"
	if string(got) != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}

	// Inserted keys land in a stable order, not one depending on map iteration.
	var first string
	for i := range 8 {
		idx := build(t, map[string]string{
			"index.md": "---\nokf_version: \"0.1\"\n---\n",
			"b.md":     "---\ntype: task\n---\nbody\n",
		})
		e, _ := idx.Resolve("/b.md")
		if err := e.SetFields(map[string]any{"zeta": "1", "alpha": "2", "mid": "3"}); err != nil {
			t.Fatal(err)
		}
		b, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "b.md"))
		if i == 0 {
			first = string(b)
		} else if string(b) != first {
			t.Fatalf("run %d differs:\n%q\nvs\n%q", i, b, first)
		}
	}
}

func TestSetFieldsRejectsReservedKeys(t *testing.T) {
	idx := build(t, map[string]string{"index.md": "---\nokf_version: \"0.1\"\n---\n", "a.md": "---\ntype: task\n---\n"})
	e, _ := idx.Resolve("/a.md")
	for _, key := range []string{"_path", "", "a:b"} {
		if err := e.SetField(key, "v"); err == nil {
			t.Errorf("SetField(%q) should be rejected", key)
		}
	}
	// A rejected batch applies nothing at all.
	if err := e.SetFields(map[string]any{"status": "done", "_path": "/x.md"}); err == nil {
		t.Error("a reserved key should fail the whole batch")
	}
	if raw, _ := e.Raw(); strings.Contains(raw, "done") {
		t.Error("a rejected batch must not partially apply")
	}
}

func TestUnsetField(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"a.md":     "---\ntype: task\ntags:\n  - a\n  - b\nstatus: todo\n---\nbody\n",
	})
	e, _ := idx.Resolve("/a.md")
	if err := e.UnsetField("tags"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "a.md"))
	if want := "---\ntype: task\nstatus: todo\n---\nbody\n"; string(got) != want {
		t.Errorf("got %q want %q", got, want)
	}
	before, _ := e.Raw()
	if err := e.UnsetField("absent"); err != nil {
		t.Fatal(err)
	}
	if after, _ := e.Raw(); after != before {
		t.Error("removing an absent key changed the file")
	}
}

// The format's inline task mechanism, which had no write primitive at all.
func TestSetCheckbox(t *testing.T) {
	const doc = "---\ntype: note\n---\n# Steps\n\n- [ ] first\n- [x] second\n- [ ] first\n\nprose\n"
	idx := build(t, map[string]string{"index.md": "---\nokf_version: \"0.1\"\n---\n", "a.md": doc})
	e, _ := idx.Resolve("/a.md")
	if len(e.Checkboxes) != 3 {
		t.Fatalf("checkboxes=%d, want 3", len(e.Checkboxes))
	}

	// Keyed by line, so the duplicate text stays distinguishable.
	if err := e.SetCheckbox(e.Checkboxes[2].Line, true); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "a.md"))
	want := "---\ntype: note\n---\n# Steps\n\n- [ ] first\n- [x] second\n- [x] first\n\nprose\n"
	if string(got) != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
	if !e.Checkboxes[2].Done || e.Checkboxes[0].Done {
		t.Error("entry not refreshed after toggling")
	}

	// And back again.
	if err := e.SetCheckbox(e.Checkboxes[1].Line, false); err != nil {
		t.Fatal(err)
	}
	if raw, _ := e.Raw(); !strings.Contains(raw, "- [ ] second") {
		t.Errorf("unticking failed:\n%s", raw)
	}
	// A line holding no checkbox is an error, not a silent no-op.
	if err := e.SetCheckbox(1, true); err == nil {
		t.Error("expected an error for a line holding no checkbox")
	}
	if err := e.SetCheckbox(9999, true); err == nil {
		t.Error("expected an error for a line past the end")
	}
}

// A write must not touch the file when nothing changed, or every no-op would
// wake a watcher for a change that did not happen.
func TestNoOpWriteDoesNotTouchFile(t *testing.T) {
	idx := build(t, map[string]string{"index.md": "---\nokf_version: \"0.1\"\n---\n", "a.md": "---\nstatus: todo\n---\nb\n"})
	e, _ := idx.Resolve("/a.md")
	abs := filepath.Join(idx.Bundle.Dir, "a.md")
	before, _ := os.Stat(abs)
	if err := e.SetField("status", "todo"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.Stat(abs)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Error("a no-op write touched the file")
	}
}

// Writes go through a temp file and a rename, so a reader never sees a torn
// file, and the entry's permissions survive.
func TestWriteIsAtomicAndKeepsMode(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "a.md")
	if err := os.WriteFile(abs, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(abs, []byte("new\n")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(abs); string(got) != "new\n" {
		t.Errorf("content=%q", got)
	}
	fi, _ := os.Stat(abs)
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("mode=%v, want 0600: a rename must not reset the user's permissions", fi.Mode().Perm())
	}
	ents, _ := os.ReadDir(dir)
	for _, en := range ents {
		if strings.HasPrefix(en.Name(), ".wiki-") {
			t.Errorf("temp file left behind: %s", en.Name())
		}
	}
}

// Atomicity has to hold for the commands, not just for the helper. This drives a
// real rewrite while a reader watches, and counts any read that matches neither
// the before nor the after state. With os.WriteFile in place of writeFile it
// reports hundreds of torn reads; it is the regression guard for that swap.
func TestCommandRewritesAreAtomic(t *testing.T) {
	filler := strings.Repeat("a line of the entry body\n", 3000)
	before := "---\ntype: note\n---\n[x](/index.md)\n" + filler // absolute: tidy rewrites it
	after := "---\ntype: note\n---\n[x](./index.md)\n" + filler // relative: the canonical form

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "wiki.toml"), []byte("spec = \"0.1\"\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "index.md"), []byte("---\nokf_version: \"0.1\"\n---\nhome\n"), 0o644)
	target := filepath.Join(dir, "a.md")

	// Re-dirtying must not itself be observable as a torn write, or the test
	// would be measuring its own setup.
	reset := func() {
		if err := os.WriteFile(target+".reset", []byte(before), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(target+".reset", target); err != nil {
			t.Fatal(err)
		}
	}
	reset()

	var torn, reads int64
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				if b, err := os.ReadFile(target); err == nil {
					atomic.AddInt64(&reads, 1)
					if s := string(b); s != before && s != after {
						atomic.AddInt64(&torn, 1)
					}
				}
			}
		}
	}()

	rewrites := 0
	for range 50 {
		reset()
		b, err := bundle.Discover(dir)
		if err != nil {
			t.Fatal(err)
		}
		idx, err := Build(b)
		if err != nil {
			t.Fatal(err)
		}
		fixes, err := idx.NormalizeLinks(true)
		if err != nil {
			t.Fatal(err)
		}
		rewrites += len(fixes)
	}
	close(stop)
	<-done

	if rewrites == 0 {
		t.Fatal("nothing was rewritten, so this proved nothing")
	}
	if torn > 0 {
		t.Errorf("%d of %d reads saw a partial file: writes are not atomic", torn, reads)
	}
}

// A rename replaces the name it is given, so an unresolved write would swap a
// symlinked entry for a regular file and fork it into two diverging copies.
// Writing through to the target is what a plain write did.
func TestWriteFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.md")
	link := filepath.Join(dir, "link.md")
	if err := os.WriteFile(real, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.md", link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := writeFile(link, []byte("after\n")); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Error("the symlink was replaced by a regular file")
	}
	if b, _ := os.ReadFile(real); string(b) != "after\n" {
		t.Errorf("target not updated: %q", b)
	}
}

// writeFile is an unexported helper, so nothing stops the next mutating command
// from reaching for os.WriteFile and quietly reintroducing torn writes — which
// is how the engine ended up with four of them. This fails the build instead of
// relying on anyone remembering.
//
// Scoped to this package because it is the only one that rewrites entries a
// reader may hold open. internal/scaffold creates a bundle that does not exist
// yet, so it has no such reader.
func TestNoDirectWritesOutsideWriteFile(t *testing.T) {
	// os.CreateTemp and os.Rename are writeFile's own tools, so the patterns
	// below match the bare calls only.
	banned := regexp.MustCompile(`os\.(WriteFile|Create|OpenFile|Truncate)\(`)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, f := range entries {
		name := f.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		checked++
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if m := banned.FindString(line); m != "" {
				t.Errorf("%s:%d uses %s — entry rewrites must go through writeFile, "+
					"which renames a temp file into place so a concurrent reader "+
					"never sees a partial entry", name, i+1, m)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no source files scanned, so this proved nothing")
	}
}

// A list is not a string that happens to contain brackets: SetField would quote
// it into a scalar, so a consumer had no way to write `tags` or `blockers` — the
// two list fields the format uses most.
func TestSetFieldList(t *testing.T) {
	tests := []struct{ name, doc, key, want string }{
		{
			"flow stays flow",
			"---\ntags: [ui, api]\ntype: task\n---\nb\n", "tags",
			"---\ntags: [ui, api, new]\ntype: task\n---\nb\n",
		},
		{
			// The API was asked to change a value, not to reformat the file.
			"block stays block",
			"---\nblockers:\n  - /x.md\n  - /y.md\ntype: task\n---\nb\n", "blockers",
			"---\nblockers:\n  - ui\n  - api\n  - new\ntype: task\n---\nb\n",
		},
		{
			"absent key is inserted flow-style",
			"---\ntype: task\n---\nb\n", "tags",
			"---\ntype: task\ntags: [ui, api, new]\n---\nb\n",
		},
		{
			// Replacing a scalar with a list must not leave the old quoting.
			"scalar becomes a list",
			"---\ntags: \"just one\"\ntype: task\n---\nb\n", "tags",
			"---\ntags: [ui, api, new]\ntype: task\n---\nb\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := setFrontmatterList(tc.doc, tc.key, []string{"ui", "api", "new"}, false)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tc.want)
			}
		})
	}
}

// Items are quoted more strictly than scalars: a comma or a bracket would end
// the item in flow style, where in a scalar they are harmless.
func TestSetFieldListQuoting(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []string
		want string
	}{
		{"plain items bare", []string{"ui", "api"}, "---\nk: [ui, api]\n---\n"},
		{"comma forces quotes", []string{"a,b", "c"}, "---\nk: [\"a,b\", c]\n---\n"},
		{"brackets force quotes", []string{"[x]"}, "---\nk: [\"[x]\"]\n---\n"},
		{"colon forces quotes", []string{"a: b"}, "---\nk: [\"a: b\"]\n---\n"},
		{"number forces quotes", []string{"0.1"}, "---\nk: [\"0.1\"]\n---\n"},
		{"empty list is explicit", nil, "---\nk: []\n---\n"},
		{"paths stay bare", []string{"/a/b.md"}, "---\nk: [/a/b.md]\n---\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := setFrontmatterList("---\nk: old\n---\n", "k", tc.in, false)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

// The written form must survive a round trip, or the API produces frontmatter
// its own parser misreads.
func TestSetFieldListRoundTrips(t *testing.T) {
	for _, values := range [][]string{
		{"ui", "api"},
		{"a,b", "c: d", "0.1", "[x]"},
		{"/epics/x.md", "/epics/y.md"},
	} {
		idx := build(t, map[string]string{
			"index.md": "---\nokf_version: \"0.1\"\n---\n",
			"a.md":     "---\ntype: task\ntags: [old]\n---\nbody\n",
		})
		e, _ := idx.Resolve("/a.md")
		if err := e.SetFieldList("tags", values); err != nil {
			t.Fatal(err)
		}
		if got := e.FieldList("tags"); !slices.Equal(got, values) {
			raw, _ := e.Raw()
			t.Errorf("wrote %q, read back %#v, want %#v", raw, got, values)
		}
	}
}

// The reason SetFields takes map[string]any: a scalar and a list set together
// are one write. With separate calls they were two, and a failure between them
// left the entry half-updated — the exact thing one pass exists to prevent.
func TestSetFieldsMixesScalarsAndLists(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"a.md":     "---\ntype: task\nstatus: todo\ntags: [old]\nblockers:\n  - /x.md\n---\nbody\n",
	})
	e, _ := idx.Resolve("/a.md")
	if err := e.SetFields(map[string]any{
		"status":   "done",
		"tags":     []string{"ui", "api"},
		"blockers": []string{"/y.md", "/z.md"},
		"assignee": "mary",
	}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "a.md"))
	want := "---\ntype: task\nstatus: done\ntags: [ui, api]\nblockers:\n  - /y.md\n  - /z.md\nassignee: mary\n---\nbody\n"
	if string(got) != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
	// Everything reads back as the type it was written as.
	if e.Field("status") != "done" || e.Field("assignee") != "mary" {
		t.Errorf("scalars: status=%q assignee=%q", e.Field("status"), e.Field("assignee"))
	}
	if !slices.Equal(e.FieldList("tags"), []string{"ui", "api"}) {
		t.Errorf("tags=%#v", e.FieldList("tags"))
	}
	if !slices.Equal(e.FieldList("blockers"), []string{"/y.md", "/z.md"}) {
		t.Errorf("blockers=%#v", e.FieldList("blockers"))
	}
}

// map[string]any moves the type check to runtime, so it has to be a real check
// with a message that names what went wrong — and it must reject before writing.
func TestSetFieldsRejectsUnsupportedTypes(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"a.md":     "---\ntype: task\nstatus: todo\n---\nbody\n",
	})
	e, _ := idx.Resolve("/a.md")
	before, _ := e.Raw()

	for _, v := range []any{42, true, 1.5, nil, []int{1}, map[string]string{"a": "b"}} {
		err := e.SetFields(map[string]any{"status": "done", "bad": v})
		if err == nil {
			t.Errorf("value %#v (%T) should be rejected", v, v)
			continue
		}
		if !strings.Contains(err.Error(), "bad") {
			t.Errorf("error should name the offending key, got %v", err)
		}
	}
	// A rejected batch writes nothing, including the valid keys beside it.
	if after, _ := e.Raw(); after != before {
		t.Errorf("a rejected batch modified the file:\n%q", after)
	}
	// Round-tripping Frontmatter through SetFields is the point of the shape.
	fm := e.Frontmatter()
	fm["status"] = "in-progress"
	if err := e.SetFields(fm); err != nil {
		t.Fatalf("Frontmatter() output should be accepted by SetFields: %v", err)
	}
	if e.Field("status") != "in-progress" {
		t.Errorf("status=%q", e.Field("status"))
	}
}
