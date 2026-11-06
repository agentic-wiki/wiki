package parse

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestSetFieldShapes(t *testing.T) {
	tests := []struct{ name, doc, key, value, want string }{
		{"bare stays bare", "---\nstatus: todo\n---\nb\n", "status", "done", "---\nstatus: done\n---\nb\n"},
		{"author's quoting kept", "---\nstatus: \"todo\"\n---\nb\n", "status", "done", "---\nstatus: \"done\"\n---\nb\n"},
		{"a value needing quotes gets them", "---\ns: a\n---\nb\n", "s", "in: progress", "---\ns: \"in: progress\"\n---\nb\n"},
		// A bare `true` would parse as a boolean and a bare 0.1 as a float, so
		// neither would still be the string that was set.
		{"yaml keyword quoted", "---\nk: a\n---\nb\n", "k", "true", "---\nk: \"true\"\n---\nb\n"},
		{"number quoted", "---\nk: a\n---\nb\n", "k", "0.1", "---\nk: \"0.1\"\n---\nb\n"},
		// A bare number or boolean stays bare when the new value is one too, so
		// its type (list --format json) survives the write.
		{"bare number stays bare", "---\nn: 10\n---\nb\n", "n", "11", "---\nn: 11\n---\nb\n"},
		{"bare number with comment", "---\nn: 10 # ten\n---\nb\n", "n", "0.5", "---\nn: 0.5 # ten\n---\nb\n"},
		{"bare bool stays bare", "---\ndone: false\n---\nb\n", "done", "true", "---\ndone: true\n---\nb\n"},
		{"quoted number stays quoted", "---\nv: \"1.0\"\n---\nb\n", "v", "1.1", "---\nv: \"1.1\"\n---\nb\n"},
		{"new number key quoted", "---\ntype: task\n---\nb\n", "x", "5", "---\ntype: task\nx: \"5\"\n---\nb\n"},
		{"number to bool quoted", "---\nn: 10\n---\nb\n", "n", "true", "---\nn: \"true\"\n---\nb\n"},
		{"number to text", "---\nn: 10\n---\nb\n", "n", "ten", "---\nn: ten\n---\nb\n"},
		{"non-json number quoted", "---\nn: 10\n---\nb\n", "n", "007", "---\nn: \"007\"\n---\nb\n"},
		{"negative number stays bare", "---\nn: -1\n---\nb\n", "n", "-2", "---\nn: -2\n---\nb\n"},
		{"bool spelled True stays bare", "---\ndone: false\n---\nb\n", "done", "True", "---\ndone: True\n---\nb\n"},
		{"list of numbers to number bare", "---\nn: [1, 2]\n---\nb\n", "n", "5", "---\nn: 5\n---\nb\n"},
		{"block list of numbers to number bare", "---\nn:\n  - 1\n---\nb\n", "n", "5", "---\nn: 5\n---\nb\n"},
		{"list of strings to number quoted", "---\nn: [a]\n---\nb\n", "n", "5", "---\nn: \"5\"\n---\nb\n"},
		{"number with a comment quoted", "---\nn: 10\n---\nb\n", "n", "11 # x", "---\nn: \"11 # x\"\n---\nb\n"},
		// A null placeholder takes a typed value bare, so the column keeps the
		// type it was waiting for; a quoted placeholder is a string and stays one.
		{"empty placeholder to number bare", "---\nepoch:\n---\nb\n", "epoch", "5", "---\nepoch: 5\n---\nb\n"},
		{"tilde placeholder to bool bare", "---\ndone: ~\n---\nb\n", "done", "true", "---\ndone: true\n---\nb\n"},
		{"null placeholder to number bare", "---\nn: null # later\n---\nb\n", "n", "0.2", "---\nn: 0.2 # later\n---\nb\n"},
		{"quoted empty placeholder stays quoted", "---\nversion: \"\"\n---\nb\n", "version", "1.10", "---\nversion: \"1.10\"\n---\nb\n"},
		{"null placeholder to text unchanged", "---\nz: ~\n---\nb\n", "z", "soon", "---\nz: soon\n---\nb\n"},
		// A comment after the colon doesn't make the key a scalar: its items still
		// belong to it, so they go with it.
		{"commented block list replaced whole", "---\nn: # scores\n  - 1\n  - 2\ntype: task\n---\nb\n", "n", "5", "---\nn: 5 # scores\ntype: task\n---\nb\n"},
		{"empty block list to number bare", "---\nn:\n  -\n---\nb\n", "n", "5", "---\nn: 5\n---\nb\n"},
		{"null stays quoted", "---\nz: ~\n---\nb\n", "z", "null", "---\nz: \"null\"\n---\nb\n"},
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
			got, err := SetField(tc.doc, tc.key, tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tc.want)
			}
		})
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
			got, err := SetFieldList(tc.doc, tc.key, []string{"ui", "api", "new"})
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
			got, err := SetFieldList("---\nk: old\n---\n", "k", tc.in)
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

// A bare `-` is an (empty) block-list item to the parser, so the writer must
// treat it as part of the list too: a write replaces the whole list, and the
// parser reads back exactly what was written, with valid YAML around it.
func TestWritesOnBlockListWithBareDash(t *testing.T) {
	const doc = "---\nl:\n  - a\n  -\n  - b\ntype: task\n---\nb\n"
	check := func(name, got string, want any) {
		t.Helper()
		if wantDoc := map[string]string{
			"SetField":     "---\nl: x\ntype: task\n---\nb\n",
			"SetFieldList": "---\nl:\n  - y\ntype: task\n---\nb\n",
			"UnsetField":   "---\ntype: task\n---\nb\n",
		}[name]; got != wantDoc {
			t.Errorf("%s wrote %q, want %q", name, got, wantDoc)
		}
		if err := ValidateFrontmatter(got); err != nil {
			t.Errorf("%s: invalid YAML %v:\n%s", name, err, got)
		}
		fm, _ := Frontmatter(got)
		if !reflect.DeepEqual(fm["l"], want) || fm["type"] != "task" {
			t.Errorf("%s: read back l=%#v type=%#v, want l=%#v:\n%s", name, fm["l"], fm["type"], want, got)
		}
	}

	got, err := SetField(doc, "l", "x")
	if err != nil {
		t.Fatal(err)
	}
	check("SetField", got, "x")

	got, err = SetFieldList(doc, "l", []string{"y"})
	if err != nil {
		t.Fatal(err)
	}
	check("SetFieldList", got, []string{"y"})

	if got, err = UnsetField(doc, "l"); err != nil {
		t.Fatal(err)
	}
	check("UnsetField", got, nil)
}

// A block-list key with a comment after its colon (`n: # scores`) is a list to
// the parser, so every write takes its items along.
func TestWritesOnCommentedBlockListKey(t *testing.T) {
	const doc = "---\nn: # scores\n  - 1\n  - 2\ntype: task\n---\nb\n"
	list, err := SetFieldList(doc, "n", []string{"c"})
	if err != nil {
		t.Fatal(err)
	}
	unset, err := UnsetField(doc, "n")
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct{ got, want string }{
		"SetFieldList": {list, "---\nn: # scores\n  - c\ntype: task\n---\nb\n"},
		"UnsetField":   {unset, "---\ntype: task\n---\nb\n"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s wrote %q, want %q", name, tc.got, tc.want)
		}
		if err := ValidateFrontmatter(tc.got); err != nil {
			t.Errorf("%s: invalid YAML: %v", name, err)
		}
	}
}

// Setting or removing a key replaces or removes exactly the lines it owns, on
// every span-table shape, and reads back as what was set. A key written twice
// refuses both writes.
func TestWriteEachSpanShape(t *testing.T) {
	for _, row := range spanRows {
		if row.read == "" {
			continue
		}
		content := "---\n" + strings.Join(row.lines, "\n") + "\n---\nb\n"
		h, _ := scanLayout(content)
		var owned []field
		for _, f := range h.fields {
			if f.key == row.read {
				owned = append(owned, f)
			}
		}
		set, setErr := SetField(content, row.read, "z")
		unset, unsetErr := UnsetField(content, row.read)
		if len(owned) > 1 {
			if setErr == nil || unsetErr == nil {
				t.Errorf("%q: a key written twice must refuse both writes: %v, %v", row.lines, setErr, unsetErr)
			}
			continue
		}
		if setErr != nil || unsetErr != nil {
			t.Fatalf("%q: %v, %v", row.lines, setErr, unsetErr)
		}
		f := owned[0]
		lines := strings.Split(content, "\n")
		before, after := strings.Join(lines[:f.line], "\n"), strings.Join(lines[f.end+1:], "\n")
		// The span becomes the one key line; how the value is quoted is TestSetFieldShapes' job.
		if !strings.HasPrefix(set, before+"\n"+row.read+": ") || !strings.HasSuffix(set, "\n"+after) || strings.Count(set, "\n") != strings.Count(before+"\nk\n"+after, "\n") {
			t.Errorf("%q: SetField wrote %q, want %q, one key line, then %q", row.lines, set, before, after)
		}
		if want := before + "\n" + after; unset != want {
			t.Errorf("%q: UnsetField wrote %q, want %q", row.lines, unset, want)
		}
		if fm, _ := Frontmatter(set); fm[row.read] != "z" {
			t.Errorf("%q: read back %#v after SetField", row.lines, fm[row.read])
		}
		if fm, _ := Frontmatter(unset); fm[row.read] != nil {
			t.Errorf("%q: still read %#v after UnsetField", row.lines, fm[row.read])
		}
	}
}

// A nested map or a | text goes whole, and a nested map isn't a null
// placeholder, so a number written over it is quoted.
func TestWriteReplacesWholeSpans(t *testing.T) {
	got, err := SetField("---\nm:\n  a: 1\n  b: x\nt: y\n---\n", "m", "5")
	if want := "---\nm: \"5\"\nt: y\n---\n"; err != nil || got != want {
		t.Errorf("SetField over a nested map: %q, %v; want %q", got, err, want)
	}
	got, err = UnsetField("---\nd: |\n  text\n\n  more\nt: y\n---\n", "d")
	if want := "---\nt: y\n---\n"; err != nil || got != want {
		t.Errorf("UnsetField of a | text: %q, %v; want %q", got, err, want)
	}
	got, err = SetFieldList("---\nk: a long\n  value\nt: y\n---\n", "k", []string{"c"})
	if want := "---\nk: [c]\nt: y\n---\n"; err != nil || got != want {
		t.Errorf("SetFieldList over a continued scalar: %q, %v; want %q", got, err, want)
	}
}

// A key is valid when the scanner reads `key: x` back as exactly that key, and
// it doesn't start with _ (reserved for _path).
func TestWriteKeyValidity(t *testing.T) {
	invalid := `can't be written: it would not read back as the same key (a key can't start with a space or a YAML indicator such as "-", "#", "[" or "*", or be null or "<<")`
	for key, want := range map[string]string{
		"k":          "",
		"my key":     "",
		"roe_min_5y": "",
		" k":         invalid,
		"- k":        invalid,
		"# k":        invalid,
		"a #b":       invalid,
		"a:b":        "", // YAML reads `a:b: x` as the key a:b
		"a: b":       invalid,
		"k ":         invalid,
		"":           invalid,
		"a\nb":       invalid,
		"[a":         invalid,
		"{a":         invalid,
		"*a":         invalid,
		"'a":         invalid,
		"%a":         invalid,
		"@a":         invalid,
		"|a":         invalid,
		"~":          invalid,
		"null":       invalid,
		"<<":         invalid,
		"why?":       "",
		"_x":         `is reserved (leading underscore)`,
	} {
		for name, write := range map[string]func() (string, error){
			"SetField":     func() (string, error) { return SetField("---\nt: y\n---\n", key, "5") },
			"SetFieldList": func() (string, error) { return SetFieldList("---\nt: y\n---\n", key, []string{"5"}) },
			"UnsetField":   func() (string, error) { return UnsetField("---\nt: y\n---\n", key) },
		} {
			_, err := write()
			switch {
			case want == "" && err != nil:
				t.Errorf("%s(%q): %v", name, key, err)
			case want != "" && (err == nil || err.Error() != fmt.Sprintf("key %q %s", key, want)):
				t.Errorf("%s(%q): err = %v, want key %q %s", name, key, err, key, want)
			}
		}
	}
}

// A key written twice refuses every write to it, naming both lines, and leaves
// other keys writable.
func TestWriteDuplicateKey(t *testing.T) {
	const content = "---\nk: 1\nt: x\nk: 2\n---\n"
	want := `frontmatter has key "k" twice (lines 2 and 4); fix the file first (wiki check reports it)`
	for name, write := range map[string]func() (string, error){
		"SetField":     func() (string, error) { return SetField(content, "k", "3") },
		"SetFieldList": func() (string, error) { return SetFieldList(content, "k", []string{"3"}) },
		"UnsetField":   func() (string, error) { return UnsetField(content, "k") },
	} {
		if got, err := write(); err == nil || err.Error() != want || got != "" {
			t.Errorf("%s: %q, %v; want the error %q", name, got, err, want)
		}
	}
	if got, err := SetField(content, "t", "y"); err != nil || got != "---\nk: 1\nt: y\nk: 2\n---\n" {
		t.Errorf("SetField on another key: %q, %v", got, err)
	}
}

// A header with no closing fence is an error, and nothing is returned to write.
func TestWriteUnterminatedHeader(t *testing.T) {
	for name, write := range map[string]func() (string, error){
		"SetField":     func() (string, error) { return SetField("---\nk: v\n", "k", "w") },
		"SetFieldList": func() (string, error) { return SetFieldList("---\nk: v\n", "k", nil) },
		"UnsetField":   func() (string, error) { return UnsetField("---\nk: v\n", "k") },
	} {
		if got, err := write(); err == nil || got != "" {
			t.Errorf("%s: %q, %v", name, got, err)
		}
	}
}

// okf_version is updated, inserted, or given a header when the file has none.
func TestSetFieldOKFVersion(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"update", "---\ntype: index\nokf_version: \"0.2\"\n---\nbody\n", "---\ntype: index\nokf_version: \"0.1\"\n---\nbody\n"},
		{"insert", "---\ntype: index\n---\nbody\n", "---\ntype: index\nokf_version: \"0.1\"\n---\nbody\n"},
		{"crlf", "---\r\ntype: index\r\n---\r\nbody\r\n", "---\r\ntype: index\r\nokf_version: \"0.1\"\r\n---\r\nbody\r\n"},
		{"create", "no frontmatter here\n", "---\nokf_version: \"0.1\"\n---\nno frontmatter here\n"},
	}
	for _, c := range cases {
		got, err := SetField(c.in, "okf_version", "0.1")
		if err != nil || got != c.want {
			t.Errorf("%s: got %q err=%v, want %q", c.name, got, err, c.want)
		}
	}
}

// Without a header, UnsetField has nothing to remove.
func TestUnsetFieldNoHeader(t *testing.T) {
	if got, err := UnsetField("# body\n", "k"); err != nil || got != "# body\n" {
		t.Errorf("UnsetField without a header: %q, %v", got, err)
	}
}

// Rule W: a new value or list item keeps the field's type and the key line's
// comment. One row per row of the ADR's rule W table.
func TestRuleW(t *testing.T) {
	type write func(content string) (string, error)
	set := func(key, value string) write {
		return func(c string) (string, error) { return SetField(c, key, value) }
	}
	setList := func(key string, values ...string) write {
		return func(c string) (string, error) { return SetFieldList(c, key, values) }
	}
	unset := func(key string) write {
		return func(c string) (string, error) { return UnsetField(c, key) }
	}
	for _, tc := range []struct {
		before []string
		write  write
		after  []string // nil: the write is refused
	}{
		{[]string{"n: 10 # ten"}, set("n", "11"), []string{"n: 11 # ten"}},
		{[]string{"v: 'abc' # note"}, set("v", "xyz"), []string{"v: 'xyz' # note"}},
		{[]string{`v: "1.0"`}, set("v", "1.1"), []string{`v: "1.1"`}},
		// A quoted value takes the author's quote whenever it fits (reads back
		// unchanged: no escapes), even when the value needs quoting; else %q.
		{[]string{"v: '1.0'"}, set("v", "1.1"), []string{"v: '1.1'"}},
		{[]string{"v: 'abc'"}, set("v", "a: b"), []string{"v: 'a: b'"}},
		{[]string{"v: 'a'"}, set("v", "it's"), []string{`v: "it's"`}},
		{[]string{`v: "a"`}, set("v", `say "hi"`), []string{`v: "say \"hi\""`}},
		{[]string{`v: "abc"`}, set("v", "true"), []string{`v: "true"`}},
		{[]string{"v: 'a'"}, set("v", " padded "), []string{`v: " padded "`}}, // spaces at the ends do not fit: the read trims them
		{[]string{`v: "a"`}, set("v", `C:\dir`), []string{`v: "C:\\dir"`}},
		{[]string{"v: 'a' # c"}, set("v", "x\ny"), []string{`v: "x\ny" # c`}},
		{[]string{"k: # todo"}, set("k", "5"), []string{"k: 5 # todo"}},
		{[]string{"k: # todo", "  - a"}, set("k", "5"), []string{`k: "5" # todo`}},
		{[]string{"r: [0.35, 0.36]"}, setList("r", "0.35", "0.37"), []string{"r: [0.35, 0.37]"}},
		{[]string{"r: []"}, setList("r", "1", "true"), []string{"r: [1, true]"}},
		{[]string{"t: [a, b]"}, setList("t", "1", "c"), []string{`t: ["1", c]`}},
		{[]string{"t: [a]"}, setList("t", "x,y"), []string{`t: ["x,y"]`}},
		{[]string{"m:", "  a: 1"}, set("m", "5"), []string{`m: "5"`}},
		{[]string{"d: |", "  text"}, unset("d"), []string{}},
		{[]string{"k: 1", "k: 2"}, set("k", "3"), nil},
		{[]string{"t: y"}, set("_x", "5"), nil},
		{[]string{"t: y"}, set("- k", "5"), nil},
		{[]string{"t: y"}, set("x", "5"), []string{"t: y", `x: "5"`}},
		// Each list item is judged on its own, and the comment stays on the key line.
		{[]string{"r: # rates", "  - 1", "  - 2"}, setList("r", "3", "n/a"), []string{"r: # rates", "  - 3", "  - n/a"}},
		{[]string{"r: [1, a] # mixed"}, setList("r", "2", "b"), []string{`r: ["2", b] # mixed`}},
		{[]string{"r: [~]"}, setList("r", "1"), []string{"r: [1]"}}, // every item null: the old type is null
		{[]string{"ok: [true, false]"}, setList("ok", "false", "1"), []string{`ok: [false, "1"]`}},
		{[]string{"k: a long", "  value # c"}, set("k", "5"), []string{`k: "5"`}},
		{[]string{"n: 10   #  spaced"}, set("n", "12"), []string{"n: 12   #  spaced"}},
		{[]string{"tags: # t", "  - a,b"}, setList("tags", "c,d", "2"), []string{"tags: # t", `  - "c,d"`, `  - "2"`}},
	} {
		content := "---\n" + strings.Join(tc.before, "\n") + "\n---\nb\n"
		got, err := tc.write(content)
		if tc.after == nil {
			if err == nil {
				t.Errorf("%q: wrote %q, want a refusal", tc.before, got)
			}
			continue
		}
		want := "---\n" + strings.Join(append(tc.after, "---", "b", ""), "\n")
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v\nwant %q", tc.before, got, err, want)
		}
	}
}

// Rule W on a list of maps: it counts as a string, so a number set in its
// place is quoted.
func TestOldTypeNested(t *testing.T) {
	content := "---\nraw:\n  - path: a.pdf\n    pages: 40\n---\n"
	got, err := SetField(content, "raw", "5")
	if want := "---\nraw: \"5\"\n---\n"; err != nil || got != want {
		t.Errorf("got %q, %v; want %q", got, err, want)
	}
}

// Headers wiki can't edit safely: every write refuses with the reason, and
// nothing is returned to write. Shade's cases C and D (keys not at column 0),
// and a header that isn't valid YAML.
func TestWriteRefusesUneditable(t *testing.T) {
	const notAKey = "line %d is not a key on its own line at column 0"
	for name, tc := range map[string]struct {
		header, why string
		line        int
	}{
		"C: a flow-map header": {"{title: x, type: note}", notAKey, 2},
		"D: a ? key":           {"? title\n: x\ntype: note", notAKey, 2},
		"not valid YAML":       {"title: Fix: x\ntype: note", "not read (line %d: ", 2},
	} {
		content := "---\n" + tc.header + "\n---\n# Body\n"
		why := fmt.Sprintf(tc.why, tc.line)
		for write, fn := range map[string]func() (string, error){
			"SetField title":   func() (string, error) { return SetField(content, "title", "new") },
			"SetField status":  func() (string, error) { return SetField(content, "status", "done") },
			"SetFieldList":     func() (string, error) { return SetFieldList(content, "tags", []string{"x"}) },
			"UnsetField title": func() (string, error) { return UnsetField(content, "title") },
			"UnsetField type":  func() (string, error) { return UnsetField(content, "type") },
		} {
			got, err := fn()
			var refused *EditError
			if got != "" || !errors.As(err, &refused) || !strings.Contains(err.Error(), why) {
				t.Errorf("%s: %s: %q, %v; want a refusal saying %q", name, write, got, err, why)
			}
		}
	}
	// A real comment line at column 0 between keys doesn't block a write.
	got, err := SetField("---\ntitle: x\n# note\ntype: note\n---\n", "type", "task")
	if want := "---\ntitle: x\n# note\ntype: task\n---\n"; err != nil || got != want {
		t.Errorf("# note between keys: %q, %v", got, err)
	}
}

// A value continued on an unindented line is YAML's own reading, and writes
// edit it as such, byte for byte: Shade's cases A, B, E1, E2 and `foo #bar"`.
// Each key's lines run to the next key's line.
func TestWriteEditsContinuedValues(t *testing.T) {
	for name, tc := range map[string]struct {
		header string
		want   map[string]string // the write, to the header after it
	}{
		"A: a key line inside a quoted scalar": {"title: \"first\nstatus: x\"\ntype: note", map[string]string{
			"set title":  "title: \"new\"\ntype: note",
			"set status": "title: \"first\nstatus: x\"\ntype: note\nstatus: done",
			"unset type": "title: \"first\nstatus: x\"",
		}},
		"B: a key line inside a flow list": {"tags: [a,\nstatus: b]\ntype: note", map[string]string{
			"set tags":   "tags: [x]\ntype: note",
			"set status": "tags: [a,\nstatus: b]\ntype: note\nstatus: done",
			"unset tags": "type: note",
		}},
		"E1: a quoted value continued at column 0": {"title: \"Ship it\nsoon\"\ntype: note", map[string]string{
			"set title":   "title: \"new\"\ntype: note",
			"set status":  "title: \"Ship it\nsoon\"\ntype: note\nstatus: done",
			"unset title": "type: note",
		}},
		"E2: a flow list continued at column 0": {"tags: [a,\nb]\ntype: note", map[string]string{
			"set tags":   "tags: [x]\ntype: note",
			"unset type": "tags: [a,\nb]",
		}},
		"a # inside a continued quoted value": {"title: \"a\nfoo #bar\"\ntype: note", map[string]string{
			"set title":   "title: \"new\"\ntype: note",
			"unset title": "type: note",
		}},
	} {
		content := "---\n" + tc.header + "\n---\n# Body\n"
		for write, want := range tc.want {
			op, key, _ := strings.Cut(write, " ")
			var got string
			var err error
			switch {
			case op == "unset":
				got, err = UnsetField(content, key)
			case key == "tags":
				got, err = SetFieldList(content, key, []string{"x"})
			case key == "title":
				got, err = SetField(content, key, "new")
			default:
				got, err = SetField(content, key, "done")
			}
			if want := "---\n" + want + "\n---\n# Body\n"; err != nil || got != want {
				t.Errorf("%s: %s: %v\n got %q\nwant %q", name, write, err, got, want)
			}
		}
	}
}

// nestedAndQuoted is a header holding the requester's nested shapes and every
// kind of quoted text whose reading changed: rows 1–3, 13 and 15 of the ADR's
// Decision 8.
const nestedAndQuoted = "---\n" +
	"type: dataset\n" +
	"raw:\n  - path: a.pdf # first\n    meta:\n      form: 10-K\n  - &r {path: b.pdf, pages: 2}\n" +
	"data_as_of:\n  income: 2025-12-31\n\n  balance: 2025-09-30\n" +
	"a: 'company''s'\nb: 'text '\nc: \" a \"\nd: \"a \\\"q\\\"\"\ne: \"\\t\\u00e9\"\n" +
	"f: [\"\", a]\ng:\n  - ''\n  -\nh: \"x\n  y\"\n" +
	"status: open # now\n" +
	"---\n# Body\n"

// Setting or removing a key changes no other byte, next to nested values and
// quoted text.
func TestSetFieldKeepsNestedAndQuotedBytes(t *testing.T) {
	got, err := SetField(nestedAndQuoted, "status", "done")
	if want := strings.Replace(nestedAndQuoted, "status: open # now", "status: done # now", 1); err != nil || got != want {
		t.Errorf("set: %v\n got %q\nwant %q", err, got, want)
	}
	got, err = UnsetField(nestedAndQuoted, "status")
	if want := strings.Replace(nestedAndQuoted, "status: open # now\n", "", 1); err != nil || got != want {
		t.Errorf("unset: %v\n got %q\nwant %q", err, got, want)
	}
	got, err = SetField(nestedAndQuoted, "owner", "ana")
	if want := strings.Replace(nestedAndQuoted, "status: open # now\n", "status: open # now\nowner: ana\n", 1); err != nil || got != want {
		t.Errorf("add: %v\n got %q\nwant %q", err, got, want)
	}
}

// Removing a nested value removes all its lines, and nothing else.
func TestUnsetNestedRemovesItsLines(t *testing.T) {
	got, err := UnsetField(nestedAndQuoted, "raw")
	if want := strings.Replace(nestedAndQuoted, "raw:\n  - path: a.pdf # first\n    meta:\n      form: 10-K\n  - &r {path: b.pdf, pages: 2}\n", "", 1); err != nil || got != want {
		t.Errorf("raw: %v\n got %q\nwant %q", err, got, want)
	}
	got, err = UnsetField(nestedAndQuoted, "data_as_of")
	if want := strings.Replace(nestedAndQuoted, "data_as_of:\n  income: 2025-12-31\n\n  balance: 2025-09-30\n", "", 1); err != nil || got != want {
		t.Errorf("data_as_of: %v\n got %q\nwant %q", err, got, want)
	}
}

// Every write reads its result back: one that would leave the header
// unreadable, here an anchor removed while an alias still names it, is refused,
// and nothing is returned to write.
func TestWriteRefusesBreakingTheHeader(t *testing.T) {
	const values = "---\nbase: &b {form: 10-K}\ncopy: *b\n---\n"
	const keys = "---\na: &k status\n*k : open\n---\n"
	for name, write := range map[string]func() (string, error){
		"unset base":                   func() (string, error) { return UnsetField(values, "base") },
		"set base x":                   func() (string, error) { return SetField(values, "base", "x") },
		"set base [x]":                 func() (string, error) { return SetFieldList(values, "base", []string{"x"}) },
		"set the anchored key's value": func() (string, error) { return SetField(keys, "a", "x") },
	} {
		got, err := write()
		var ee *EditError
		if got != "" || !errors.As(err, &ee) || !strings.Contains(err.Error(), "would not read after this write") || !strings.Contains(err.Error(), "unknown anchor") {
			t.Errorf("%s: %q, %v; want a refusal naming the unknown anchor", name, got, err)
		}
	}
	// A write that would make another key change value, through an alias that
	// now names an earlier anchor, is refused too.
	const rebound = "---\na: &s 1\nstatus: &s open\nother: *s\n---\n"
	for name, write := range map[string]func() (string, error){
		"set status":   func() (string, error) { return SetField(rebound, "status", "done") },
		"unset status": func() (string, error) { return UnsetField(rebound, "status") },
	} {
		got, err := write()
		if want := "frontmatter would change other through an anchor; nothing written"; got != "" || err == nil || err.Error() != want {
			t.Errorf("%s: %q, %v; want %q", name, got, err, want)
		}
	}
	// A write that would pass a budget is refused: here the 1 MiB header cap.
	got, err := SetField(values, "big", strings.Repeat("x", 1<<20))
	if want := "frontmatter would not read after this write (header larger than 1 MiB); nothing written"; got != "" || err == nil || err.Error() != want {
		t.Errorf("set a 1 MiB value: %d bytes, %v; want %q", len(got), err, want)
	}
	// Writes that keep the anchor, or that touch the alias, go through.
	if got, err := UnsetField(values, "copy"); err != nil || got != "---\nbase: &b {form: 10-K}\n---\n" {
		t.Errorf("unset copy: %q, %v", got, err)
	}
	if got, err := SetField(values, "status", "done"); err != nil || got != "---\nbase: &b {form: 10-K}\ncopy: *b\nstatus: done\n---\n" {
		t.Errorf("set status: %q, %v", got, err)
	}
}

// A value holding ? is quoted, so a list written with it reads back as written.
func TestSetFieldListQuestionMark(t *testing.T) {
	values := []string{"Why?", "a?b", "a ?", "b"}
	out, err := SetFieldList("---\ntype: note\n---\n", "tags", values)
	if err != nil {
		t.Fatal(err)
	}
	strs, _, _, err := ReadFrontmatter(out)
	if err != nil || !reflect.DeepEqual(strs["tags"], values) {
		t.Errorf("%q reads %#v, %v", out, strs["tags"], err)
	}
	out, err = SetField("---\ntype: note\n---\n", "q", "Why?")
	if strs, _, _, _ := ReadFrontmatter(out); err != nil || strs["q"] != "Why?" {
		t.Errorf("%q: %v", out, err)
	}
}
