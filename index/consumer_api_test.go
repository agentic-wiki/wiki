package index

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/parse"
)

// The accessors below exist only for library consumers: no command calls them,
// so nothing else would notice if they broke.

// The whole point of the copy is that a consumer reading frontmatter cannot
// edit the engine's state by accident.
func TestFrontmatterIsACopy(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"a.md":     "---\ntype: task\ntitle: A\ntags: [ui, api]\n---\nbody\n",
	})
	e, err := idx.Resolve("/a.md")
	if err != nil {
		t.Fatal(err)
	}

	fm := e.Frontmatter()
	if got := fm["title"]; got != "A" {
		t.Errorf("title=%v", got)
	}
	if got, ok := fm["tags"].([]string); !ok || !slices.Equal(got, []string{"ui", "api"}) {
		t.Fatalf("tags=%#v", fm["tags"])
	}

	// Mutating the returned map, and the slices inside it, must not reach the entry.
	fm["title"] = "clobbered"
	delete(fm, "type")
	fm["tags"].([]string)[0] = "clobbered"

	if e.Field("title") != "A" {
		t.Errorf("mutating the copy changed the entry: title=%q", e.Field("title"))
	}
	if e.Type != "task" {
		t.Errorf("deleting from the copy changed the entry: type=%q", e.Type)
	}
	if got := e.FieldList("tags"); !slices.Equal(got, []string{"ui", "api"}) {
		t.Errorf("mutating a copied list reached the entry: %#v", got)
	}
	// A second call is unaffected by what was done to the first.
	if e.Frontmatter()["title"] != "A" {
		t.Error("a later call returned the mutated value")
	}
}

// A body link is written relative and the graph is keyed root-absolute, so a
// consumer rendering a body needs both directions. These are inverses.
func TestResolveAndRelativeLinkRoundTrip(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md":      "---\nokf_version: \"0.1\"\n---\n",
		"a.md":          "---\ntype: note\n---\n",
		"sub/b.md":      "---\ntype: note\n---\n",
		"sub/deep/c.md": "---\ntype: note\n---\n",
	})

	cases := []struct{ from, written, want string }{
		{"/sub/b.md", "./deep/c.md", "/sub/deep/c.md"},
		{"/sub/b.md", "../a.md", "/a.md"},
		{"/sub/deep/c.md", "../../a.md", "/a.md"},
		{"/a.md", "./sub/b.md", "/sub/b.md"},
		// A root-absolute link still resolves; it is valid, just not canonical.
		{"/sub/b.md", "/a.md", "/a.md"},
		// An anchor rides along with the target.
		{"/sub/b.md", "../a.md#heading", "/a.md#heading"},
	}
	for _, tc := range cases {
		got, outside := idx.ResolveLink(tc.from, tc.written)
		if outside {
			t.Errorf("ResolveLink(%q, %q) reported outside", tc.from, tc.written)
			continue
		}
		if got != tc.want {
			t.Errorf("ResolveLink(%q, %q) = %q, want %q", tc.from, tc.written, got, tc.want)
		}
	}

	// RelativeLink is the inverse: re-spelling a resolved target from the same
	// file must resolve back to it.
	for _, tc := range cases {
		target, _, _ := strings.Cut(tc.want, "#")
		rel := RelativeLink(tc.from, target)
		back, _ := idx.ResolveLink(tc.from, rel)
		if back != target {
			t.Errorf("RelativeLink(%q, %q) = %q, which resolves back to %q", tc.from, target, rel, back)
		}
	}
}

// A target above the bundle root is neither an edge nor broken, and a consumer
// must be able to tell, or it will try to navigate out of the bundle.
func TestResolveLinkReportsOutside(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"sub/b.md": "---\ntype: note\n---\n",
	})
	if _, outside := idx.ResolveLink("/sub/b.md", "../../escape.md"); !outside {
		t.Error("a target above the bundle root should report outside")
	}
	if _, outside := idx.ResolveLink("/sub/b.md", "../index.md"); outside {
		t.Error("a target inside the bundle should not report outside")
	}
}

// The query syntax is part of the query contract, so its parse belongs to the
// library. The subtleties are easy to regress: != is matched before =, so a
// value may contain =, and values are unquoted the way frontmatter is.
func TestParseFilter(t *testing.T) {
	for _, tc := range []struct {
		in      string
		key     string
		value   string
		negate  bool
		wantErr bool
	}{
		{in: "type=task", key: "type", value: "task"},
		{in: "status!=done", key: "status", value: "done", negate: true},
		{in: "status=", key: "status", value: ""},
		{in: "status!=", key: "status", value: "", negate: true},
		// != is matched first, so this is a negation, not a key named "a!".
		{in: "a!=b", key: "a", value: "b", negate: true},
		// A value may contain =, because the split takes the first operator only.
		{in: "url=a=b", key: "url", value: "a=b"},
		{in: `title="quoted value"`, key: "title", value: "quoted value"},
		{in: "nope", wantErr: true},
		{in: "", wantErr: true},
	} {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseFilter(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParseFilter(%q) should fail, got %+v", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseFilter(%q): %v", tc.in, err)
			}
			if got.Key != tc.key || got.Value != tc.value || got.Negate != tc.negate {
				t.Errorf("ParseFilter(%q) = %+v, want {%q %q %v}", tc.in, got, tc.key, tc.value, tc.negate)
			}
		})
	}
}

// TypedFrontmatter returns a copy, like Frontmatter, with YAML types.
func TestTypedFrontmatterIsACopy(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"a.md":     "---\ntype: task\nn: 10\ns: '10'\nl: [1, x]\nm: {k: [1]}\n---\nbody\n",
	})
	e, err := idx.Resolve("/a.md")
	if err != nil {
		t.Fatal(err)
	}

	fm := e.TypedFrontmatter()
	get := func(m parse.Map, k string) any { v, _ := m.Get(k); return v }
	if get(fm, "n") != json.Number("10") || get(fm, "s") != "10" || get(fm, "type") != "task" {
		t.Errorf("typed frontmatter = %#v", fm)
	}
	if got, ok := get(fm, "l").([]any); !ok || !reflect.DeepEqual(got, []any{json.Number("1"), "x"}) {
		t.Fatalf("l=%#v", get(fm, "l"))
	}

	for i := range fm {
		switch fm[i].Key {
		case "n":
			fm[i].Value = "clobbered"
		case "l":
			fm[i].Value.([]any)[0] = "clobbered"
		case "m":
			k, _ := fm[i].Value.(parse.Map).Get("k")
			k.([]any)[0] = "clobbered"
		}
	}
	again := e.TypedFrontmatter()
	deep, _ := get(again, "m").(parse.Map).Get("k")
	if get(again, "n") != json.Number("10") || get(again, "l").([]any)[0] != json.Number("1") || deep.([]any)[0] != json.Number("1") {
		t.Errorf("mutating the copy reached the entry: %#v", again)
	}
}

// A write re-parses the entry, so the typed view follows it like the string view.
func TestTypedFrontmatterFollowsWrites(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"a.md":     "---\ntype: task\ndone: false\n---\nbody\n",
	})
	e, err := idx.Resolve("/a.md")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SetField("status", "todo"); err != nil {
		t.Fatal(err)
	}
	if err := e.UnsetField("done"); err != nil {
		t.Fatal(err)
	}
	fm := e.TypedFrontmatter()
	status, _ := fm.Get("status")
	if _, ok := fm.Get("done"); ok || status != "todo" {
		t.Errorf("typed frontmatter after writes = %#v", fm)
	}
}

// An entry marshals with _path first, then its keys in file order at every
// depth; a frontmatter key named _path doesn't replace the entry's path.
func TestMapJSONFileOrder(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"a.md":     "---\ntype: note\nz: 1\n_path: /elsewhere.md\nm: {y: [b, {d: 1, c: 2}], x: true}\n---\nbody\n",
	})
	e, err := idx.Resolve("/a.md")
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(e)
	if want := `{"_path":"/a.md","type":"note","z":1,"m":{"y":["b",{"d":1,"c":2}],"x":true}}`; err != nil || string(got) != want {
		t.Errorf("got %s, %v\nwant %s", got, err, want)
	}
}

// FieldList leaves out empty items, which wiki treats as no value, and is a
// new slice on every call: a caller can change it without reaching the index.
func TestFieldListDropsEmptyItemsAndIsACopy(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"a.md":     "---\ntype: note\ntags: [a, \"\", b]\nblockers:\n  -\n  - /n.md\nowner: ana\n---\nbody\n",
	})
	e, err := idx.Resolve("/a.md")
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string][]string{"tags": {"a", "b"}, "blockers": {"/n.md"}, "owner": {"ana"}} {
		got := e.FieldList(key)
		if !slices.Equal(got, want) {
			t.Errorf("%s: %q, want %q", key, got, want)
		}
		got[0] = "clobbered"
		slices.Sort(got)
		if again := e.FieldList(key); !slices.Equal(again, want) {
			t.Errorf("%s: changing the result reached the entry: %q", key, again)
		}
	}
	// The string view keeps every item, as written.
	if got := e.Frontmatter()["tags"]; !reflect.DeepEqual(got, []string{"a", "", "b"}) {
		t.Errorf("Frontmatter tags = %#v", got)
	}
}

// The reason a header wasn't read is the text a user should see, with no
// "yaml: " prefix.
func TestFrontmatterErrHasNoYAMLPrefix(t *testing.T) {
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n",
		"bad.md":   "---\ntitle: a: b\n---\nbody\n",
	})
	e, err := idx.Resolve("/bad.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := e.FrontmatterErr(); got == nil || got.Error() != "line 2: mapping values are not allowed in this context" {
		t.Errorf("FrontmatterErr = %v", got)
	}
}

// check tests an anchor against the headings' IDs, the same rule parse
// gives them: every ID is found, a repeat by its "-N", and a slug that isn't
// an ID warns.
func TestCheckAnchorsUseHeadingIDs(t *testing.T) {
	texts := []string{"Simple", "Two Words", "With-Hyphen and_underscore", "Punctuation: removed, right?",
		"MiXeD CaSe", "Numbers 123", "Trailing underscore_", "__dunder__", "Notes", "Notes"}
	var body strings.Builder
	for _, h := range texts {
		body.WriteString("## " + h + "\n\n")
	}
	for _, h := range parse.Headings(body.String()) {
		body.WriteString("[to](#" + h.ID + ")\n\n")
	}
	body.WriteString("[to](#notes-2)\n")
	idx := build(t, map[string]string{
		"index.md": "---\nokf_version: \"0.1\"\n---\n[a](./a.md)\n",
		"a.md":     "---\ntype: note\n---\n" + body.String(),
	})
	var anchors []string
	for _, is := range idx.Check() {
		if strings.Contains(is.Msg, "anchor") {
			anchors = append(anchors, is.Msg)
		}
	}
	if !slices.Equal(anchors, []string{"link anchor not found -> #notes-2"}) {
		t.Errorf("anchor warnings %q, want only #notes-2", anchors)
	}
}
