package parse

import (
	"encoding/json"
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

func TestFrontmatterComments(t *testing.T) {
	fm, _ := Frontmatter("---\ntype: note  # trailing comment\ntitle: \"a # b\"  # real comment\nurl: http://x/p#frag\ntags: [a, b]  # list comment\n---\nbody\n")
	if fm["type"] != "note" {
		t.Errorf("inline comment not stripped: type=%q", fm["type"])
	}
	if fm["title"] != "a # b" {
		t.Errorf("# inside quotes must survive: title=%q", fm["title"])
	}
	if fm["url"] != "http://x/p#frag" {
		t.Errorf("# glued to text (no preceding space) must survive: url=%q", fm["url"])
	}
	if got, ok := fm["tags"].([]string); !ok || len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("flow list with trailing comment: tags=%v", fm["tags"])
	}
}

func TestFrontmatterBlockScalar(t *testing.T) {
	fm, body := Frontmatter("---\ntype: note\ndescription: |\n  line one\n  line two\ntitle: After\n---\nbody\n")
	if fm["description"] != "line one\nline two" {
		t.Errorf("block scalar = %q", fm["description"])
	}
	if fm["title"] != "After" {
		t.Errorf("key after block scalar lost: title=%q", fm["title"])
	}
	if body != "body\n" {
		t.Errorf("body = %q", body)
	}
}

func TestFrontmatterNestedMapGraceful(t *testing.T) {
	// A nested map reads as its lines, de-indented, and doesn't touch sibling keys.
	fm, _ := Frontmatter("---\ntype: note\nmeta:\n  a: 1\n  b: 2\ntitle: Kept\n---\nbody\n")
	if fm["type"] != "note" || fm["title"] != "Kept" || fm["meta"] != "a: 1\nb: 2" {
		t.Errorf("type=%q title=%q meta=%q", fm["type"], fm["title"], fm["meta"])
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
	if got, _ := listViews(list(`[a, "b c" ,  , d]`)); !reflect.DeepEqual(got, []string{"a", "b c", "d"}) {
		t.Errorf("list = %#v", got)
	}
	if got, _ := listViews(list("[]")); got == nil || len(got) != 0 {
		t.Errorf("empty list = %#v, want []string{}", got)
	}
	// A comma inside quotes separates nothing. Splitting on every comma turned
	// one item into two broken halves ("a and b").
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{`["a,b", c]`, []string{"a,b", "c"}},
		{`['x,y', 'z']`, []string{"x,y", "z"}},
		{`["a: b", "0.1", "[x]"]`, []string{"a: b", "0.1", "[x]"}},
		{`["only one"]`, []string{"only one"}},
	} {
		if got, _ := listViews(list(tc.in)); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("list(%s) = %#v, want %#v", tc.in, got, tc.want)
		}
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

func TestCheckboxes(t *testing.T) {
	body := "- [ ] a\n* [x] b\n+ [ ] c\n  - [ ] d\n- [e](/x.md) not a task\n```\n- [ ] fenced\n```\n"
	ts := Checkboxes(body)
	if len(ts) != 4 {
		t.Fatalf("want 4 tasks (-, *, +, indented), got %d: %+v", len(ts), ts)
	}
	if ts[0].Text != "a" || ts[0].Done || !ts[1].Done || ts[3].Text != "d" {
		t.Errorf("tasks = %+v", ts)
	}
}

// Inline code in a task's text is content, not code to strip: it must survive
// verbatim (a checkbox is line-anchored, so a span can never fake a task).
func TestCheckboxesKeepInlineCode(t *testing.T) {
	body := "- [ ] Scaffold `wiki.toml`, `index.md`\n"
	ts := Checkboxes(body)
	if len(ts) != 1 || ts[0].Text != "Scaffold `wiki.toml`, `index.md`" {
		t.Errorf("task text = %+v, want inline code preserved", ts)
	}
}

func TestHeadings(t *testing.T) {
	body := "# A\n## B\nnot a heading\n#nospace\n###### F\n```\n# fenced\n```\n"
	hs := Headings(body)
	if len(hs) != 3 || hs[0].Level != 1 || hs[1].Level != 2 || hs[2].Level != 6 {
		t.Fatalf("headings = %+v (no-space and fenced excluded)", hs)
	}
}

// Heading text keeps inline code verbatim, for the same reason as tasks.
func TestHeadingsKeepInlineCode(t *testing.T) {
	hs := Headings("## The `wiki checkboxes` command\n")
	if len(hs) != 1 || hs[0].Text != "The `wiki checkboxes` command" {
		t.Errorf("heading text = %+v, want inline code preserved", hs)
	}
}

func TestTables(t *testing.T) {
	body := "intro\n\n| Date | Amount |\n|------|-------:|\n| 2026-01 | 100 |\n| 2026-02 | 200 |\n\nafter\n"
	got := Tables(body)
	if len(got) != 1 {
		t.Fatalf("want 1 table, got %d: %+v", len(got), got)
	}
	if !reflect.DeepEqual(got[0].Header, []string{"Date", "Amount"}) {
		t.Errorf("header = %v", got[0].Header)
	}
	if want := [][]string{{"2026-01", "100"}, {"2026-02", "200"}}; !reflect.DeepEqual(got[0].Rows, want) {
		t.Errorf("rows = %v, want %v", got[0].Rows, want)
	}
}

func TestTablesEdges(t *testing.T) {
	// no outer pipes; an escaped pipe stays literal in its cell; rows fitted to header
	body := "a | b | c\n--- | --- | ---\nx\\|y | 2\n1 | 2 | 3 | 4\n"
	got := Tables(body)
	if len(got) != 1 {
		t.Fatalf("want 1 table, got %d", len(got))
	}
	if !reflect.DeepEqual(got[0].Header, []string{"a", "b", "c"}) {
		t.Errorf("header = %v", got[0].Header)
	}
	if !reflect.DeepEqual(got[0].Rows[0], []string{"x|y", "2", ""}) { // \| literal; short row padded
		t.Errorf("row0 = %q", got[0].Rows[0])
	}
	if !reflect.DeepEqual(got[0].Rows[1], []string{"1", "2", "3"}) { // surplus 4th cell dropped
		t.Errorf("row1 = %q", got[0].Rows[1])
	}
}

func TestTablesFencedAndMultiple(t *testing.T) {
	body := "```\n| not | a |\n|---|---|\n| x | y |\n```\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\ntext\n\n| C | D |\n|---|---|\n| 3 | 4 |\n"
	got := Tables(body)
	if len(got) != 2 { // the fenced table is ignored
		t.Fatalf("want 2 tables, got %d: %+v", len(got), got)
	}
	if got[0].Header[0] != "A" || got[1].Header[0] != "C" {
		t.Errorf("headers = %v / %v", got[0].Header, got[1].Header)
	}
}

func TestTablesNone(t *testing.T) {
	if got := Tables("# Heading\n\njust prose, no pipes.\n"); len(got) != 0 {
		t.Errorf("want no tables, got %+v", got)
	}
	if got := Tables("| a | b |\nnot a delimiter row\n"); len(got) != 0 {
		t.Errorf("a pipe line without a delimiter row is not a table, got %+v", got)
	}
}

func TestLinkTextKeepsInlineCode(t *testing.T) {
	// The text is read from the line as written: inline code inside the brackets
	// is part of the link's text, not erased with the code a target might hold.
	set := Links("[`wiki` docs](/w.md)")
	if len(set.Absolute) != 1 || set.Absolute[0].Text != "`wiki` docs" {
		t.Errorf("Absolute = %+v, want text \"`wiki` docs\"", set.Absolute)
	}
}

func TestMapInternalLinks(t *testing.T) {
	up := func(t string) string { return "/R/" + t }
	cases := []struct{ name, in, want string }{
		{"relative and absolute", "[a](x.md) and [b](/y.md)", "[a](/R/x.md) and [b](/R//y.md)"},
		{"anchor kept in what fn sees", "[a](x.md#h)", "[a](/R/x.md#h)"},
		{"title untouched", `[a](x.md "the title")`, `[a](/R/x.md "the title")`},
		{"image", "![alt](img.png)", "![alt](/R/img.png)"},
		{"url untouched", "[a](https://e.com/x.md) [m](mailto:a@b.c)", "[a](https://e.com/x.md) [m](mailto:a@b.c)"},
		{"self anchor untouched", "[top](#top)", "[top](#top)"},
		{"empty target untouched", "[a]( )", "[a]( )"},
		{"inline code untouched", "`[c](x.md)` [a](x.md)", "`[c](x.md)` [a](/R/x.md)"},
		{"fenced code untouched", "```\n[c](x.md)\n```\n~~~\n[d](x.md)\n~~~\n[a](x.md)", "```\n[c](x.md)\n```\n~~~\n[d](x.md)\n~~~\n[a](/R/x.md)"},
		{"several per line, varying length", "[a](x.md) [b](longer/y.md) [c](z.md)", "[a](/R/x.md) [b](/R/longer/y.md) [c](/R/z.md)"},
		{"angle brackets kept", "[a](<p q.md> \"t\")", "[a](</R/p q.md> \"t\")"},
		{"leading space before target", "[a](  x.md)", "[a](  /R/x.md)"},
		{"line count and CRLF preserved", "one\r\n[a](x.md)\r\nthree", "one\r\n[a](/R/x.md)\r\nthree"},
	}
	for _, c := range cases {
		if got := MapInternalLinks(c.in, up); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestMapInternalLinksSpacesGetBrackets(t *testing.T) {
	// A bare destination cannot hold a space, so a replacement that has one is
	// bracketed; an already-bracketed one is not double-wrapped.
	sp := func(string) string { return "/my dir/x.md" }
	if got := MapInternalLinks("[a](x.md)", sp); got != "[a](</my dir/x.md>)" {
		t.Errorf("bare -> %q", got)
	}
	if got := MapInternalLinks("[a](<x y.md>)", sp); got != "[a](</my dir/x.md>)" {
		t.Errorf("angled -> %q", got)
	}
}

func TestMapInternalLinksIdentityIsByteExact(t *testing.T) {
	body := "# T\n[a](x.md \"t\") `[c](y.md)`\n```\n[d](z.md)\n```\n![i](<p q.png>)\n"
	if got := MapInternalLinks(body, func(t string) string { return t }); got != body {
		t.Errorf("identity map changed bytes:\n got %q\nwant %q", got, body)
	}
}

// TestTypedFrontmatterRuleT has one row per row of rule T, plus every token that
// looks typed in some YAML but stays a string here. Each value is tried bare, as
// a flow-list element and as a block-list item.
func TestTypedFrontmatterRuleT(t *testing.T) {
	for _, tc := range []struct {
		token string
		want  any
	}{
		// null
		{"~", nil},
		{"null", nil},
		{"Null", nil},
		{"NULL", nil},
		// booleans
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		// numbers, byte for byte as written
		{"0", json.Number("0")},
		{"10", json.Number("10")},
		{"-3", json.Number("-3")},
		{"-0", json.Number("-0")},
		{"0.2", json.Number("0.2")},
		{"1.10", json.Number("1.10")},
		{"1e5", json.Number("1e5")},
		{"2.5E-3", json.Number("2.5E-3")},
		{"-1.5e+10", json.Number("-1.5e+10")},
		{"123456789012345678901234567890", json.Number("123456789012345678901234567890")},
		// stays a string: YAML numbers that are not JSON numbers
		{"+1", "+1"},
		{"007", "007"},
		{".5", ".5"},
		{"1.", "1."},
		{"0x1F", "0x1F"},
		{"0o17", "0o17"},
		{".inf", ".inf"},
		{"-.inf", "-.inf"},
		{".nan", ".nan"},
		{"1_000", "1_000"},
		// stays a string: YAML 1.1 booleans, other spellings, dates, text
		{"yes", "yes"},
		{"no", "no"},
		{"on", "on"},
		{"off", "off"},
		{"tRUE", "tRUE"},
		{"nULL", "nULL"},
		{"2026-06-30", "2026-06-30"},
		{"10 apples", "10 apples"},
		{"text", "text"},
		// stays a string: quoted, whatever it holds
		{`"10"`, "10"},
		{`'10'`, "10"},
		{`"true"`, "true"},
		{`'null'`, "null"},
		{`"~"`, "~"},
	} {
		for name, content := range map[string]string{
			"scalar": "---\nk: " + tc.token + "\n---\n",
			"flow":   "---\nk: [" + tc.token + "]\n---\n",
			"block":  "---\nk:\n  - " + tc.token + "\n---\n",
		} {
			fm, _ := TypedFrontmatter(content)
			got := fm["k"]
			want := tc.want
			if name != "scalar" {
				want = []any{tc.want}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s %s: typed = %#v, want %#v", name, tc.token, got, want)
			}
		}
	}
}

// TestTypedFrontmatterScalars covers the values that are never typed by rule T:
// empty values, block scalars, nested and flow maps.
func TestTypedFrontmatterScalars(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		str, typed    any
	}{
		{"empty value", "k:\n", "", nil},
		{"comment only value", "k: # nothing yet\n", "", nil},
		{"tilde", "k: ~\n", "~", nil},
		{"null", "k: null\n", "null", nil},
		{"quoted empty", "k: \"\"\n", "", ""},
		{"number with comment", "k: 10 # ten\n", "10", json.Number("10")},
		{"empty flow list", "k: []\n", []string{}, []any{}},
		{"mixed flow list", "k: [1, 2.5, x, \"7\"]\n", []string{"1", "2.5", "x", "7"}, []any{json.Number("1"), json.Number("2.5"), "x", "7"}},
		{"literal block scalar", "k: |\n  10\n", "10", "10"},
		{"folded block scalar", "k: >\n  true\n", "true", "true"},
		{"nested map", "k:\n  a: 1\n", "a: 1", "a: 1"},
		{"flow map", "k: {a: 1}\n", "{a: 1}", "{a: 1}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := "---\n" + tc.content + "---\n"
			strs, _ := Frontmatter(content)
			typed, _ := TypedFrontmatter(content)
			if !reflect.DeepEqual(strs["k"], tc.str) {
				t.Errorf("string view = %#v, want %#v", strs["k"], tc.str)
			}
			if !reflect.DeepEqual(typed["k"], tc.typed) {
				t.Errorf("typed view = %#v, want %#v", typed["k"], tc.typed)
			}
		})
	}
}

// TestFrontmatterEmptyListItems: an empty item is dropped in both views, and the
// list goes on after it. Empty is tested after unquoting; a null token is kept.
func TestFrontmatterEmptyListItems(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		str, typed    any
	}{
		{"flow empty item", "k: [a, , b]\n", []string{"a", "b"}, []any{"a", "b"}},
		{"flow quoted empty", "k: [\"\"]\n", []string{}, []any{}},
		{"flow null kept", "k: [~, 1]\n", []string{"~", "1"}, []any{nil, json.Number("1")}},
		{"block bare dash", "k:\n  - a\n  -\n  - b\n", []string{"a", "b"}, []any{"a", "b"}},
		{"block quoted empty", "k:\n  - a\n  - \"\"\n  - b\n", []string{"a", "b"}, []any{"a", "b"}},
		{"block single quoted empty", "k:\n  - ''\n  - b\n", []string{"b"}, []any{"b"}},
		{"block comment only", "k:\n  - a\n  - # c\n  - b\n", []string{"a", "b"}, []any{"a", "b"}},
		{"block null kept", "k:\n  - ~\n  - 1\n", []string{"~", "1"}, []any{nil, json.Number("1")}},
		{"block all empty", "k:\n  -\n  - \"\"\n  - # c\n", []string{}, []any{}},
		{"block bare dash only", "k:\n  -\n", []string{}, []any{}},
		{"block bare dash at end", "k:\n  - a\n  -\nnext: x\n", []string{"a"}, []any{"a"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := "---\n" + tc.content + "---\n"
			strs, _ := Frontmatter(content)
			typed, _ := TypedFrontmatter(content)
			if !reflect.DeepEqual(strs["k"], tc.str) {
				t.Errorf("string view = %#v, want %#v", strs["k"], tc.str)
			}
			if !reflect.DeepEqual(typed["k"], tc.typed) {
				t.Errorf("typed view = %#v, want %#v", typed["k"], tc.typed)
			}
		})
	}
	// The key after a list ending in a bare dash is still read.
	fm, _ := Frontmatter("---\nk:\n  - a\n  -\nnext: x\n---\n")
	if fm["next"] != "x" {
		t.Errorf("next = %#v, want \"x\"", fm["next"])
	}
}

// TestTypedFrontmatterBodyAndKeys: both views come from one header, with the
// same keys and the same body.
func TestTypedFrontmatterBodyAndKeys(t *testing.T) {
	content := "---\ntype: note\nn: 10\ntags: [a, b]\n---\n# Body\n"
	strs, sb := Frontmatter(content)
	typed, tb := TypedFrontmatter(content)
	if sb != "# Body\n" || tb != sb {
		t.Errorf("bodies %q and %q, want %q", sb, tb, "# Body\n")
	}
	if len(strs) != 3 || len(typed) != 3 {
		t.Errorf("keys: string view %v, typed view %v", strs, typed)
	}
	for _, content := range []string{"# No header\n", "---\nk: 1\n"} {
		typed, body := TypedFrontmatter(content)
		if len(typed) != 0 || body != content {
			t.Errorf("%q: typed = %v, body = %q", content, typed, body)
		}
	}
}

func TestIsBlockItem(t *testing.T) {
	for line, want := range map[string]bool{
		"  - a":   true,
		"- a":     true,
		"  -":     true,
		"  -\r":   true,
		"-":       true,
		"  -a":    false,
		"  --- x": false,
		"k: v":    false,
		"":        false,
	} {
		if got := isBlockItem(line); got != want {
			t.Errorf("isBlockItem(%q) = %v, want %v", line, got, want)
		}
	}
}

// TypedRows types each table cell by rule T. A cell is quoted only when the
// same quote starts and ends it and appears nowhere else in it.
func TestTableTypedRows(t *testing.T) {
	for _, tc := range []struct {
		cell string
		want any
	}{
		{"12.5", json.Number("12.5")},
		{"", nil},
		{`"7372"`, "7372"},
		{`'7372'`, "7372"},
		{`"Yes," she said, "no"`, `"Yes," she said, "no"`},
		{"~", nil},
		{"null", nil},
		{"true", true},
		{"False", false},
		{"-", "-"},
		{"n/a", "n/a"},
		{"2026-06-30", "2026-06-30"},
		{"12345678901234567890", json.Number("12345678901234567890")},
		{"007", "007"},
		{`" padded "`, " padded "},
		{`""`, ""},
	} {
		tb := Table{Header: []string{"c"}, Rows: [][]string{{tc.cell}}}
		if got := tb.TypedRows()[0][0]; !reflect.DeepEqual(got, tc.want) {
			t.Errorf("TypedRows(%q) = %#v, want %#v", tc.cell, got, tc.want)
		}
	}
	if got := (Table{Header: []string{"a"}}).TypedRows(); got == nil || len(got) != 0 {
		t.Errorf("no rows: %#v, want an empty slice", got)
	}
}
