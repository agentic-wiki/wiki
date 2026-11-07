package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// writeRowsBundle writes the bundle the csv goldens run on: links, a broken
// link, checkboxes, headings, a table, tags and a root-absolute link to tidy.
func writeRowsBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	for name, content := range map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\n",
		"index.md":  "---\nokf_version: \"0.1\"\n---\n# Home\n[g](./guide.md)\n",
		"guide.md":  "---\ntype: note\ntitle: Guide\ntags: [a, b]\nn: 10\n---\n# Guide\nintro text\n## Setup\n- [ ] try it\n- [x] done it\n[flat](./flat.md) [gone](./gone.md)\n",
		"flat.md":   "---\ntype: note\n---\nflat body, see [g](/guide.md)\n",
		"data.md":   "---\ntype: note\n---\n| a | b |\n|---|---|\n| 1 | x |\n",
	} {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Every command gives its rows in jsonl, csv and tsv: one row per item of a
// list result, one row for any other result. csv has a header always; jsonl is
// one compact object per row with the row's own keys, and nothing for an empty
// result. One golden per row of the ADR's table (revision 6, D1).
func TestRowsPerCommand(t *testing.T) {
	dir := writeRowsBundle(t)
	dirJSON, _ := json.Marshal(dir) // a Windows path has backslashes, which JSON escapes
	for _, tc := range []struct {
		name       string
		run        func([]string) int
		args       []string
		csv, jsonl string
		code       int
	}{
		{"list", cmdList, nil,
			"_path,type,n,okf_version,tags,title\n/data.md,note,,,,\n/flat.md,note,,,,\n/guide.md,note,10,,\"[a, b]\",Guide\n/index.md,,,0.1,,\n",
			`{"_path":"/data.md","type":"note"}` + "\n" + `{"_path":"/flat.md","type":"note"}` + "\n" + `{"_path":"/guide.md","type":"note","n":10,"tags":["a","b"],"title":"Guide"}` + "\n" + `{"_path":"/index.md","okf_version":"0.1"}` + "\n", 0},
		{"orphans", cmdOrphans, nil, "_path,type\n/data.md,note\n", `{"_path":"/data.md","type":"note"}` + "\n", 0},
		{"orphans --fields", cmdOrphans, []string{"--fields", "type"}, "_path,type\n/data.md,note\n", "", -1},
		{"search", cmdSearch, []string{"intro"}, "_path,type,matches\n/guide.md,note,1\n", `{"_path":"/guide.md","type":"note","matches":1}` + "\n", 0},
		{"search --lines", cmdSearch, []string{"intro", "--lines"}, "_path,type,line,text\n/guide.md,note,8,intro text\n", `{"_path":"/guide.md","type":"note","line":8,"text":"intro text"}` + "\n", 0},
		{"search, no hits", cmdSearch, []string{"nothing-matches"}, "_path,type,matches\n", "", 1},
		{"search --lines, no hits", cmdSearch, []string{"nothing-matches", "--lines"}, "_path,type,line,text\n", "", 1},
		{"checkboxes", cmdCheckboxes, nil, "entry,line,done,text\n/guide.md,10,false,try it\n", `{"entry":"/guide.md","line":10,"done":false,"text":"try it"}` + "\n", 0},
		{"tags", cmdTags, nil, "name,count\na,1\nb,1\n", `{"name":"a","count":1}` + "\n" + `{"name":"b","count":1}` + "\n", 0},
		{"properties", cmdProperties, nil, "name,count\nn,1\nokf_version,1\ntags,1\ntitle,1\ntype,3\n",
			`{"name":"n","count":1}` + "\n" + `{"name":"okf_version","count":1}` + "\n" + `{"name":"tags","count":1}` + "\n" + `{"name":"title","count":1}` + "\n" + `{"name":"type","count":3}` + "\n", 0},
		{"property", cmdProperty, []string{"type"}, "name,count\nnote,3\n", `{"name":"note","count":3}` + "\n", 0},
		{"unresolved", cmdUnresolved, nil, "from,to,line\n/guide.md,/gone.md,12\n", `{"from":"/guide.md","to":"/gone.md","line":12}` + "\n", 0},
		{"links", cmdLinks, []string{"guide.md"}, "from,to,text,line\n/guide.md,/flat.md,flat,12\n/guide.md,/gone.md,gone,12\n",
			`{"from":"/guide.md","to":"/flat.md","text":"flat","line":12}` + "\n" + `{"from":"/guide.md","to":"/gone.md","text":"gone","line":12}` + "\n", 0},
		{"backlinks", cmdBacklinks, []string{"guide.md"}, "from,to,text,line\n/flat.md,/guide.md,g,4\n/index.md,/guide.md,g,5\n",
			`{"from":"/flat.md","to":"/guide.md","text":"g","line":4}` + "\n" + `{"from":"/index.md","to":"/guide.md","text":"g","line":5}` + "\n", 0},
		{"table", cmdTable, []string{"data.md"}, "a,b\n1,x\n", `{"a":1,"b":"x"}` + "\n", 0},
		{"tidy", cmdTidy, nil, "entry,field,from,to\n/flat.md,link,/guide.md,./guide.md\n", `{"entry":"/flat.md","field":"link","from":"/guide.md","to":"./guide.md"}` + "\n", 0},
		{"check", cmdCheck, nil, "level,entry,msg\nwarning,/guide.md,broken link -> /gone.md\n", `{"level":"warning","entry":"/guide.md","msg":"broken link -> /gone.md"}` + "\n", 0},
		{"move", cmdMove, []string{"flat.md", "/flat2.md", "--dry-run"}, "_path,links,frontmatter_refs\n/flat.md,1,0\n/guide.md,1,0\n",
			`{"_path":"/flat.md","links":1,"frontmatter_refs":0}` + "\n" + `{"_path":"/guide.md","links":1,"frontmatter_refs":0}` + "\n", 0},
		{"outline", cmdOutline, []string{"guide.md"}, "_path,level,text,line\n/guide.md,1,Guide,7\n/guide.md,2,Setup,9\n",
			`{"_path":"/guide.md","level":1,"text":"Guide","line":7}` + "\n" + `{"_path":"/guide.md","level":2,"text":"Setup","line":9}` + "\n", 0},
		{"status", cmdStatus, nil, "dir,spec,entries,links,tags,checkboxes,broken,orphans\n" + dir + ",0.1,4,4,2,2,1,1\n",
			`{"dir":` + string(dirJSON) + `,"spec":"0.1","entries":4,"links":4,"tags":2,"checkboxes":2,"broken":1,"orphans":1}` + "\n", 0},
		{"read", cmdRead, []string{"flat.md"}, "_path,type,body\n/flat.md,note,\"flat body, see [g](/guide.md)\n\"\n",
			`{"_path":"/flat.md","type":"note","body":"flat body, see [g](/guide.md)\n"}` + "\n", 0},
	} {
		for format, want := range map[string]string{"csv": tc.csv, "jsonl": tc.jsonl} {
			if format == "jsonl" && tc.code < 0 {
				continue // --fields is csv/tsv only
			}
			code := max(tc.code, 0)
			out, got := capture(t, func() int { return tc.run(append(tc.args, "--format", format)) })
			if out != want || got != code {
				t.Errorf("%s --format %s: exit %d (want %d)\n got %q\nwant %q", tc.name, format, got, code, out, want)
			}
		}
	}
}

// check --fix gives the fixes, then the issues, as one table; with nothing
// to report, check gives the header alone.
func TestRowsCheckFix(t *testing.T) {
	writeRowsBundle(t)
	if err := os.WriteFile("index.md", []byte("---\nokf_version: \"0.2\"\n---\n# Home\n[g](./guide.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := capture(t, func() int { return cmdCheck([]string{"--fix", "--format", "csv"}) })
	want := "level,entry,field,from,to,msg\nfixed,/index.md,okf_version,0.2,0.1,\nwarning,/guide.md,,,,broken link -> /gone.md\n"
	if out != want || code != 0 {
		t.Errorf("check --fix: exit %d\n got %q\nwant %q", code, out, want)
	}
	// jsonl: each row carries its own keys, so a fix has no msg and an issue no field, from or to.
	if err := os.WriteFile("index.md", []byte("---\nokf_version: \"0.2\"\n---\n# Home\n[g](./guide.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = capture(t, func() int { return cmdCheck([]string{"--fix", "--format", "jsonl"}) })
	want = `{"level":"fixed","entry":"/index.md","field":"okf_version","from":"0.2","to":"0.1"}` + "\n" + `{"level":"warning","entry":"/guide.md","msg":"broken link -> /gone.md"}` + "\n"
	if out != want || code != 0 {
		t.Errorf("check --fix jsonl: exit %d\n got %q\nwant %q", code, out, want)
	}

	t.Chdir(t.TempDir())
	for name, content := range map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\n",
		"index.md":  "---\nokf_version: \"0.1\"\n---\n# Home\n",
	} {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--format", "csv"}, "level,entry,msg\n"},
		{[]string{"--fix", "--format", "csv"}, "level,entry,field,from,to,msg\n"},
		{[]string{"--format", "jsonl"}, ""},
		{[]string{"--fix", "--format", "jsonl"}, ""},
	} {
		args, want := tc.args, tc.want
		out, code := capture(t, func() int { return cmdCheck(args) })
		if out != want || code != 0 {
			t.Errorf("check %v on a clean bundle: exit %d, %q, want %q", args, code, out, want)
		}
	}
}

// init gives one row, its files as a JSON list in one cell.
func TestRowsInit(t *testing.T) {
	t.Chdir(t.TempDir())
	out, code := capture(t, func() int { return cmdInit([]string{"--format", "csv"}) })
	want := "dir,files\n.,\"[AGENTS.md, .gitignore, CLAUDE.md, index.md, wiki.toml]\"\n" // a list is its Flow
	if out != want || code != 0 {
		t.Errorf("init: exit %d\n got %q\nwant %q", code, out, want)
	}
	t.Chdir(t.TempDir())
	out, code = capture(t, func() int { return cmdInit([]string{"--format", "jsonl"}) })
	want = `{"dir":".","files":["AGENTS.md",".gitignore","CLAUDE.md","index.md","wiki.toml"]}` + "\n"
	if out != want || code != 0 {
		t.Errorf("init jsonl: exit %d\n got %q\nwant %q", code, out, want)
	}
}

// orphans takes --fields like list, with the same errors.
func TestCmdOrphansFieldsErrors(t *testing.T) {
	writeRowsBundle(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--format", "csv", "--fields", ""}, "wiki: --fields has an empty key at position 1 (use --fields a,b,c)\n"},
		{[]string{"--fields", "type"}, "wiki: --fields needs --format csv or --format tsv\n"},
	} {
		out, errOut, code := captureAll(t, func() int { return cmdOrphans(tc.args) })
		if code != 2 || out != "" || errOut != tc.want {
			t.Errorf("orphans %q: exit %d, stdout %q, stderr %q, want %q", tc.args, code, out, errOut, tc.want)
		}
	}
}

// A result that can't be printed, here one with no rows for csv, is an error
// on stderr with exit 2, never empty output with exit 0.
func TestEmitErrorExitsTwo(t *testing.T) {
	out, errOut, code := captureAll(t, func() int { return emit("csv", []string{"a line"}, []string{"a", "b"}) })
	if code != 2 || out != "" || errOut != "wiki: output: []string has no rows\n" {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out, errOut)
	}
	if _, _, code := captureAll(t, func() int { return emit("text", []string{"a line"}, nil) }); code != 0 {
		t.Errorf("text: exit %d, want 0", code)
	}
}

// frontmatter_refs is a count, so 0 is a real value: json gives it too.
func TestMoveJSONGivesZeroFrontmatterRefs(t *testing.T) {
	writeRowsBundle(t)
	out, code := capture(t, func() int { return cmdMove([]string{"flat.md", "/flat2.md", "--dry-run", "--format", "json"}) })
	if code != 0 || strings.Count(out, `"frontmatter_refs": 0`) != 2 {
		t.Errorf("move json: exit %d\n%s", code, out)
	}
}

// table types its cells in json and jsonl (rule T; a quoted cell is a string
// without its quotes unless the quote also appears inside), and prints them as
// written in text, csv and tsv.
func TestTableTypedCells(t *testing.T) {
	writeRowsBundle(t)
	const ds = "---\ntype: note\n---\n" +
		"| price | code | quote | empty | nul | na | date | big |\n" +
		"|---|---|---|---|---|---|---|---|\n" +
		"| 12.5 | \"7372\" | \"Yes,\" she said, \"no\" |  | ~ | n/a | 2026-06-30 | 12345678901234567890 |\n"
	if err := os.WriteFile("ds.md", []byte(ds), 0o644); err != nil {
		t.Fatal(err)
	}
	for format, want := range map[string]string{
		"json":  "[\n  {\n    \"price\": 12.5,\n    \"code\": \"7372\",\n    \"quote\": \"\\\"Yes,\\\" she said, \\\"no\\\"\",\n    \"empty\": null,\n    \"nul\": null,\n    \"na\": \"n/a\",\n    \"date\": \"2026-06-30\",\n    \"big\": 12345678901234567890\n  }\n]\n",
		"jsonl": `{"price":12.5,"code":"7372","quote":"\"Yes,\" she said, \"no\"","empty":null,"nul":null,"na":"n/a","date":"2026-06-30","big":12345678901234567890}` + "\n",
		"csv":   "price,code,quote,empty,nul,na,date,big\n12.5,\"\"\"7372\"\"\",\"\"\"Yes,\"\" she said, \"\"no\"\"\",,~,n/a,2026-06-30,12345678901234567890\n",
	} {
		out, code := capture(t, func() int { return cmdTable([]string{"ds.md", "--format", format}) })
		if out != want || code != 0 {
			t.Errorf("table --format %s: exit %d\n got %q\nwant %q", format, code, out, want)
		}
	}
}
