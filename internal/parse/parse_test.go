package parse

import (
	"reflect"
	"testing"
)

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

func TestInternalLinks(t *testing.T) {
	body := "[a](/x.md), [ext](https://e.com), [anc](/y.md#h), [rel](../r.md), [bare](r.md)\n" +
		"second [b](/z.md) line\n" +
		"```\n[code](/c.md)\n```\n" +
		"inline `[ic](/q.md)` end\n"
	links := InternalLinks(body)
	var got []string
	for _, l := range links {
		got = append(got, l.Target)
	}
	want := []string{"/x.md", "/y.md", "/z.md"} // external, relative, and code ignored; anchor stripped
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("targets = %#v, want %#v", got, want)
	}
	if links[0].Line != 1 || links[2].Line != 2 {
		t.Errorf("line numbers wrong: %+v", links)
	}
}

func TestInternalLinksIgnoresRelative(t *testing.T) {
	// Documents current behavior: relative links are not captured at all
	// (the format mandates root-absolute). A future `check` lint should warn.
	if got := InternalLinks("[x](../up.md) and [y](sibling.md)\n"); got != nil {
		t.Errorf("relative links should be ignored, got %+v", got)
	}
}

func TestInternalLinksCapturesEmbeds(t *testing.T) {
	if got := InternalLinks("![alt](/p.png)\n"); len(got) != 1 || got[0].Target != "/p.png" {
		t.Errorf("embed = %+v, want one edge to /p.png", got)
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
