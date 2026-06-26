package parse

import (
	"reflect"
	"testing"
)

func TestFrontmatterCRLF(t *testing.T) {
	fm, body := Frontmatter("---\r\ntype: note\r\ntitle: A\r\n---\r\nbody line\r\n")
	if fm["type"] != "note" || fm["title"] != "A" {
		t.Errorf("CRLF frontmatter: type=%v title=%v", fm["type"], fm["title"])
	}
	if body != "body line\r\n" {
		t.Errorf("CRLF body = %q", body)
	}
}

func TestLinksAngleBrackets(t *testing.T) {
	// <...> is markdown's way to allow spaces in a destination; we strip the
	// brackets (and any title after '>') and keep the inner target verbatim.
	set := Links("[a](</x y.md>) [b](<../p q.md>) [c](<plain.md> \"t\")")
	if len(set.Absolute) != 1 || set.Absolute[0].Target != "/x y.md" {
		t.Errorf("Absolute = %+v, want one /x y.md", set.Absolute)
	}
	if len(set.Relative) != 2 || set.Relative[0].Target != "../p q.md" || set.Relative[1].Target != "plain.md" {
		t.Errorf("Relative = %+v, want ../p q.md and plain.md", set.Relative)
	}
}

func TestLinksStripTitle(t *testing.T) {
	set := Links(`see [x](/a.md "a title") and [y](/b.md#sec 'y')`)
	if len(set.Absolute) != 2 || set.Absolute[0].Target != "/a.md" || set.Absolute[1].Target != "/b.md#sec" {
		t.Errorf("titled links = %+v (want /a.md, /b.md#sec; title stripped, anchor kept)", set.Absolute)
	}
}

func TestFrontmatter(t *testing.T) {
	content := "---\ntype: concept\ntitle: \"A: B\"\ntags: [x, y, z]\n---\n\n# Body\ntext\n"
	fm, body := Frontmatter(content)
	if fm["type"] != "concept" {
		t.Errorf("type = %v", fm["type"])
	}
	if fm["title"] != "A: B" {
		t.Errorf("title = %v (colon inside quotes should survive)", fm["title"])
	}
	if got := fm["tags"]; !reflect.DeepEqual(got, []string{"x", "y", "z"}) {
		t.Errorf("tags = %#v", got)
	}
	if body != "\n# Body\ntext\n" {
		t.Errorf("body = %q", body)
	}
}

func TestFrontmatterNone(t *testing.T) {
	content := "# No frontmatter\n"
	fm, body := Frontmatter(content)
	if len(fm) != 0 || body != content {
		t.Errorf("fm=%v body=%q", fm, body)
	}
}

func TestFrontmatterUnclosed(t *testing.T) {
	content := "---\ntype: note\nno closing fence\n"
	fm, body := Frontmatter(content)
	if len(fm) != 0 || body != content {
		t.Errorf("unclosed frontmatter should be left intact: fm=%v body=%q", fm, body)
	}
}

func TestFrontmatterBlockList(t *testing.T) {
	fm, _ := Frontmatter("---\ntags:\n  - a\n  - b\n---\nbody\n")
	if got := fm["tags"]; !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("block-list tags = %#v", got)
	}
}

func TestFrontmatterTagsTrimmed(t *testing.T) {
	// Quoted and unquoted alike trim surrounding whitespace; internal spaces stay.
	fm, _ := Frontmatter("---\ntags: [ \"hi \", ho  , \"x y\" ]\n---\nbody\n")
	want := []string{"hi", "ho", "x y"}
	if got := fm["tags"]; !reflect.DeepEqual(got, want) {
		t.Errorf("tags = %#v, want %#v", got, want)
	}
}

func TestUnquote(t *testing.T) {
	for in, want := range map[string]string{
		`"x"`:    "x",
		`'y'`:    "y",
		"  z  ":  "z",
		`"a: b"`: "a: b",
		"bare":   "bare",
		`""`:     "",
	} {
		if got := Unquote(in); got != want {
			t.Errorf("Unquote(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestList(t *testing.T) {
	if got := List(`[a, "b c" ,  , d]`); !reflect.DeepEqual(got, []string{"a", "b c", "d"}) {
		t.Errorf("List = %#v", got)
	}
	if got := List("[]"); got != nil {
		t.Errorf("empty list = %#v, want nil", got)
	}
}

func TestStringStrings(t *testing.T) {
	fm := map[string]any{"s": "v", "l": []string{"a", "b"}, "scalar": "solo"}
	if String(fm, "s") != "v" || String(fm, "missing") != "" || String(fm, "l") != "" {
		t.Errorf("String wrong")
	}
	if got := Strings(fm, "l"); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("Strings(list) = %#v", got)
	}
	if got := Strings(fm, "scalar"); !reflect.DeepEqual(got, []string{"solo"}) {
		t.Errorf("Strings(scalar) = %#v, want [solo]", got)
	}
	if Strings(fm, "missing") != nil {
		t.Errorf("Strings(missing) should be nil")
	}
}

func TestLinks(t *testing.T) {
	body := "[a](/x.md) [ext](https://e.com) [anc](/y.md#h) [rel](../r.md) [bare](r.md)\n" +
		"line2 [b](/z.md) ![img](/p.png)\n" +
		"```\n[code](/c.md)\n```\n" +
		"inline `[ic](/q.md)` x\n" +
		"[m](mailto:a@b.c) [top](#h) [t](sub/p.md \"title\")\n"
	set := Links(body)
	targets := func(ls []Link) []string {
		out := []string{}
		for _, l := range ls {
			out = append(out, l.Target)
		}
		return out
	}
	// Absolute keeps anchors (the index strips them for the graph key); embeds
	// count; fenced/inline code is ignored.
	if got, want := targets(set.Absolute), []string{"/x.md", "/y.md#h", "/z.md", "/p.png"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Absolute = %v, want %v", got, want)
	}
	// Relative keeps the anchor, drops the title.
	if got, want := targets(set.Relative), []string{"../r.md", "r.md", "sub/p.md"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Relative = %v, want %v", got, want)
	}
	if got, want := targets(set.External), []string{"https://e.com", "mailto:a@b.c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("External = %v, want %v", got, want)
	}
	// pure #anchor and code links land in no bucket; line numbers are preserved
	if set.Absolute[0].Line != 1 || set.Absolute[2].Line != 2 {
		t.Errorf("line numbers wrong: %+v", set.Absolute)
	}
}

func TestLinkSchemeEdges(t *testing.T) {
	set := Links("[a](weird:thing) [b](path/to:file.md) [c](mailto:x@y.z)")
	// a scheme is letters then ':' before any '/': weird: and mailto: are schemes
	// (external); a ':' after a '/' is just a path char.
	if len(set.External) != 2 || set.External[0].Target != "weird:thing" || set.External[1].Target != "mailto:x@y.z" {
		t.Errorf("External = %+v, want weird:thing and mailto:x@y.z", set.External)
	}
	if len(set.Relative) != 1 || set.Relative[0].Target != "path/to:file.md" {
		t.Errorf("Relative = %+v, want path/to:file.md (colon after slash is not a scheme)", set.Relative)
	}
}

func TestTasks(t *testing.T) {
	body := "- [ ] a\n* [x] b\n+ [ ] c\n  - [ ] d\n- [e](/x.md) not a task\n```\n- [ ] fenced\n```\n"
	ts := Tasks(body)
	if len(ts) != 4 {
		t.Fatalf("want 4 tasks (-, *, +, indented), got %d: %+v", len(ts), ts)
	}
	if ts[0].Text != "a" || ts[0].Done || !ts[1].Done || ts[3].Text != "d" {
		t.Errorf("tasks = %+v", ts)
	}
}

func TestHeadings(t *testing.T) {
	body := "# A\n## B\nnot a heading\n#nospace\n###### F\n```\n# fenced\n```\n"
	hs := Headings(body)
	if len(hs) != 3 || hs[0].Level != 1 || hs[1].Level != 2 || hs[2].Level != 6 {
		t.Fatalf("headings = %+v (no-space and fenced excluded)", hs)
	}
}
