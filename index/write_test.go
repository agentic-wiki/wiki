package index

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

// index.md and log.md carry no frontmatter by definition, so giving one a block
// would break the format. The bundle-root index.md is the documented exception.
func TestSetFieldAndReservedFiles(t *testing.T) {
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
	if err := sub.SetField("status", "todo"); err != ErrNoFrontmatter {
		t.Errorf("SetField on a folder index.md: err=%v, want ErrNoFrontmatter", err)
	}
	if raw, _ := sub.Raw(); raw != "# a folder board\n" {
		t.Errorf("a refused write changed the file: %q", raw)
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
	for _, key := range []string{"_path", "", "a: b"} {
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
	// Not every system models POSIX permission bits; where it does not, Chmod
	// only toggles read-only and Perm() reports a fixed value, so asserting 0600
	// would be testing the platform rather than the code.
	if runtime.GOOS != "windows" {
		fi, _ := os.Stat(abs)
		if fi.Mode().Perm() != 0o600 {
			t.Errorf("mode=%v, want 0600: a rename must not reset the user's permissions", fi.Mode().Perm())
		}
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
	// The risk measured here is a torn read, which only exists where a replace
	// can happen while a file is open. Where it cannot, the write fails instead —
	// that is what rename's retry is for — and a reader looping this tightly
	// would hold the target open throughout, measuring contention rather than
	// atomicity.
	if runtime.GOOS == "windows" {
		t.Skip("replace-while-open is refused here rather than tearing; see rename()")
	}
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
	// Each rule names the one function that owns it and the file that function
	// lives in. os.CreateTemp is writeFile's own tool, so the pattern matches
	// the bare os.Create( only.
	rules := []struct {
		banned  *regexp.Regexp
		homeIn  string // the one file allowed to use it; empty means nowhere
		use     string
		because string
	}{
		{
			// No home: writeFile builds on os.CreateTemp, which these patterns
			// deliberately do not match, so nothing in the package needs these.
			regexp.MustCompile(`os\.(WriteFile|Create|OpenFile|Truncate)\(`),
			"", "writeFile",
			"it renames a temp file into place, so a reader never sees a partial entry " +
				"and a process killed mid-write cannot truncate one",
		},
		{
			// The reason this needs enforcing: os.Rename works on every developer
			// machine that does not refuse to touch open files, so a raw call
			// passes review and CI, and only fails for users elsewhere.
			regexp.MustCompile(`os\.Rename\(`),
			"rename.go", "rename",
			"it retries while the failure is contention, which a raw os.Rename " +
				"silently drops on systems that refuse to move an open file",
		},
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked, enforced := 0, map[string]bool{}
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
			for _, r := range rules {
				m := r.banned.FindString(line)
				if m == "" {
					continue
				}
				if r.homeIn != "" && name == r.homeIn {
					enforced[r.homeIn] = true // the sanctioned use
					continue
				}
				t.Errorf("%s:%d uses %s — go through %s instead, because %s",
					name, i+1, m, r.use, r.because)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no source files scanned, so this proved nothing")
	}
	// A rule with a sanctioned home must find the call there. If it moves or goes
	// away, the rule is guarding nothing and would pass silently forever.
	for _, r := range rules {
		if r.homeIn != "" && !enforced[r.homeIn] {
			t.Errorf("%s no longer contains the call %s guards; the rule is dead", r.homeIn, r.use)
		}
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

// The retry exists for systems that refuse to replace a file another handle has
// open. It cannot be provoked where a replace does not care, so what is pinned
// here is the contract: it succeeds normally, and a non-contention error is
// returned at once rather than retried.
func TestRenameRetriesThenGivesUp(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rename(src, filepath.Join(dir, "dst")); err != nil {
		t.Fatalf("a plain rename should succeed on the first attempt: %v", err)
	}

	// An impossible destination: the error is returned, and bounded.
	start := time.Now()
	err := rename(filepath.Join(dir, "dst"), filepath.Join(dir, "no-such-dir", "x"))
	if err == nil {
		t.Fatal("renaming into a missing directory should fail")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("gave up after %v; the retry must stay bounded", elapsed)
	}
}

// A refused write names the file, and the file is left as it was.
func TestWriteRefusalNamesTheFile(t *testing.T) {
	uneditable := "---\n{title: x, type: note}\n---\nbody\n" // a flow-map header
	breaking := "---\ntype: note\nbase: &b {form: 10-K}\ncopy: *b\n---\nbody\n"
	idx := build(t, map[string]string{"x.md": uneditable, "y.md": breaking})
	for _, tc := range []struct {
		path  string
		write func(e *Entry) error
		want  string
	}{
		{"/x.md", func(e *Entry) error { return e.SetField("type", "task") },
			"frontmatter of /x.md can't be edited safely: line 2 is not a key on its own line at column 0; fix the file first (wiki check reports it)"},
		{"/x.md", func(e *Entry) error { return e.UnsetField("type") },
			"frontmatter of /x.md can't be edited safely: line 2 is not a key on its own line at column 0; fix the file first (wiki check reports it)"},
		{"/y.md", func(e *Entry) error { return e.UnsetField("base") },
			"frontmatter of /y.md would not read after this write (unknown anchor 'b' referenced); nothing written"},
	} {
		e, err := idx.Resolve(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		before, _ := e.Raw()
		if err := tc.write(e); err == nil || err.Error() != tc.want {
			t.Errorf("%s: %v\nwant %s", tc.path, err, tc.want)
		}
		if after, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, tc.path)); string(after) != before {
			t.Errorf("%s changed: %q", tc.path, after)
		}
	}
}

// An unterminated header, and check --fix on a root index.md it can't edit,
// are refused with the file named.
func TestWriteRefusalNamesTheFileElsewhere(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\n{okf_version: \"0.2\", title: Home}\n---\nhome\n",
		"u.md":     "---\ntype: note\nbody without a closing fence\n",
	})
	e, err := idx.Resolve("/u.md")
	if err != nil {
		t.Fatal(err)
	}
	want := "frontmatter of /u.md not read (line 1: unterminated frontmatter (missing closing --- fence)); fix the file first"
	if err := e.SetField("type", "task"); err == nil || err.Error() != want {
		t.Errorf("unterminated: %v\nwant %s", err, want)
	}
	_, err = idx.Fix(false)
	want = "frontmatter of /index.md can't be edited safely: line 2 is not a key on its own line at column 0; fix the file first (wiki check reports it)"
	if err == nil || err.Error() != want {
		t.Errorf("check --fix: %v\nwant %s", err, want)
	}
}

// A quoted key is set in place, by its decoded text: the file keeps one key.
func TestSetQuotedKeyEndToEnd(t *testing.T) {
	idx := build(t, map[string]string{"x.md": "---\ntype: note\n\"my key\": v # kept\n---\nbody\n"})
	e, err := idx.Resolve("/x.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SetField("my key", "w"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(idx.Bundle.Dir, "x.md"))
	if want := "---\ntype: note\nmy key: w # kept\n---\nbody\n"; string(got) != want {
		t.Errorf("file %q, want %q", got, want)
	}
	if e.Field("my key") != "w" || len(e.Frontmatter()) != 2 {
		t.Errorf("read back: %#v", e.Frontmatter())
	}
}

// An entry whose header doesn't read stays in the index with no fields, says
// why, and keeps the body after its closing fence.
func TestFrontmatterErr(t *testing.T) {
	idx := build(t, map[string]string{"bad.md": "---\ntitle: Fix: x\ntype: note\n---\n# Body\n", "ok.md": "---\ntype: note\n---\n"})
	bad, err := idx.Resolve("/bad.md")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := bad.Body()
	if bad.FrontmatterErr() == nil || bad.Type != "" || len(bad.Frontmatter()) != 0 || body != "# Body\n" {
		t.Errorf("err %v, type %q, fields %v, body %q", bad.FrontmatterErr(), bad.Type, bad.Frontmatter(), body)
	}
	if got, _ := json.Marshal(bad); string(got) != `{"_path":"/bad.md"}` {
		t.Errorf("json %s", got)
	}
	if ok, _ := idx.Resolve("/ok.md"); ok.FrontmatterErr() != nil {
		t.Errorf("ok.md: %v", ok.FrontmatterErr())
	}
}
