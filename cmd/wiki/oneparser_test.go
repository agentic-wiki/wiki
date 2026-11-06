package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/parse"
)

// writeFiles writes a bundle of files into a fresh directory and enters it.
func writeFiles(t *testing.T, files map[string]string) {
	t.Helper()
	t.Chdir(t.TempDir())
	for name, content := range files {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const unreadableWarning = "wiki: warning: /bad.md: frontmatter not read (line 2: mapping values are not allowed in this context); run wiki check\n"

// An entry whose header doesn't read gets one warning on stderr, from every
// command but check, even when the command's filters leave the entry out; the
// exit code doesn't change.
func TestWarnsOnUnreadableHeader(t *testing.T) {
	writeFiles(t, map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\n",
		"bad.md":    "---\ntitle: Fix: x\ntype: note\n---\nbody\n",
		"good.md":   "---\ntype: note\ntags: [a]\n---\nbody\n",
	})
	for name, run := range map[string]func() int{
		"property":           func() int { return cmdProperty([]string{"tags"}) },
		"list --where":       func() int { return cmdList([]string{"--where", "tags=a"}) },
		"list --format json": func() int { return cmdList([]string{"--format", "json"}) },
	} {
		out, errOut, code := captureAll(t, run)
		if code != 0 || errOut != unreadableWarning {
			t.Errorf("%s: exit %d, stderr %q; want exit 0 and %q", name, code, errOut, unreadableWarning)
		}
		if name == "list --where" && (strings.Contains(out, "/bad.md") || !strings.Contains(out, "/good.md")) {
			t.Errorf("%s: %q", name, out)
		}
	}
}

// check reports an unreadable header as an error, and doesn't also warn.
func TestCheckDoesNotWarn(t *testing.T) {
	writeFiles(t, map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\n",
		"bad.md":    "---\ntitle: Fix: x\ntype: note\n---\nbody\n",
	})
	out, errOut, code := captureAll(t, func() int { return cmdCheck(nil) })
	if code != 1 || errOut != "" || !strings.Contains(out, "/bad.md: invalid frontmatter: yaml: line 2: mapping values are not allowed in this context") {
		t.Errorf("exit %d, stderr %q\n%s", code, errOut, out)
	}
	// check --fix applies a fix (a stale okf_version), rebuilds, and still
	// doesn't warn.
	if err := os.WriteFile("index.md", []byte("---\nokf_version: \"0.0\"\n---\n# Home\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code = captureAll(t, func() int { return cmdCheck([]string{"--fix"}) })
	fixed, _ := os.ReadFile("index.md")
	if code != 1 || errOut != "" || !strings.Contains(out, "okf_version") || !strings.Contains(string(fixed), `okf_version: "0.1"`) || !strings.Contains(out, "/bad.md: invalid frontmatter") {
		t.Errorf("check --fix: exit %d, stderr %q, index.md %q\n%s", code, errOut, fixed, out)
	}
}

// listPaths runs list with args and returns the paths it prints.
func listPaths(t *testing.T, args ...string) string {
	t.Helper()
	out, code := capture(t, func() int { return cmdList(args) })
	if code != 0 {
		t.Fatalf("list %q: exit %d", args, code)
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if f := strings.Fields(line); len(f) > 0 {
			paths = append(paths, f[0])
		}
	}
	return strings.Join(paths, " ")
}

// --where matches an item of a list of maps by its Flow, and a map only by its
// whole Flow, keys in file order.
func TestWhereNested(t *testing.T) {
	writeFiles(t, map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"dataset\"]\n",
		"a.md":      "---\ntype: dataset\nraw:\n  - path: a.pdf\n    pages: 40\n  - path: b.pdf\ndata_as_of:\n  income: 2025-12-31\n  balance: 2025-09-30\n---\n",
		"b.md":      "---\ntype: dataset\nraw:\n  - {path: c.pdf}\ndata_as_of: {income: 2026-01-31}\n---\n",
	})
	for where, want := range map[string]string{
		"raw={path: a.pdf, pages: 40}": "/a.md",
		"raw={path: c.pdf}":            "/b.md",
		"raw=a.pdf":                    "",
		"data_as_of={income: 2025-12-31, balance: 2025-09-30}": "/a.md",
		"data_as_of={balance: 2025-09-30, income: 2025-12-31}": "", // another key order
		"data_as_of={income: 2026-01-31}":                      "/b.md",
	} {
		if got := listPaths(t, "--where", where); got != want {
			t.Errorf("--where %s: %q, want %q", where, got, want)
		}
	}
}

// One "no value" test: for each of these, --where k= matches and the csv cell
// is empty; for [a], neither.
func TestNoValueAgrees(t *testing.T) {
	cases := map[string]string{
		"list-of-empty.md": "k: [\"\"]",
		"list-of-null.md":  "k: [~]",
		"bare-dash.md":     "k:\n  -",
		"empty-map.md":     "k: {}",
		"empty-list.md":    "k: []",
		"null.md":          "k: ~",
		"absent.md":        "other: x",
		"value.md":         "k: [a]",
	}
	files := map[string]string{"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\n"}
	for name, k := range cases {
		files[name] = "---\ntype: note\n" + k + "\n---\n"
	}
	writeFiles(t, files)
	matched := " " + listPaths(t, "--where", "k=") + " "
	out, code := capture(t, func() int { return cmdList([]string{"--format", "csv", "--fields", "k"}) })
	if code != 0 {
		t.Fatalf("csv: exit %d", code)
	}
	for name := range cases {
		empty := name != "value.md"
		if got := strings.Contains(matched, " /"+name+" "); got != empty {
			t.Errorf("%s: --where k= matched %v, want %v", name, got, empty)
		}
		if got := strings.Contains(out, "/"+name+",\n"); got != empty {
			t.Errorf("%s: empty csv cell %v, want %v\n%s", name, got, empty, out)
		}
	}
}

// property counts items, once per entry, skipping empty ones; a list of maps
// counts item by item, each as its Flow.
func TestPropertyCountsItems(t *testing.T) {
	writeFiles(t, map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\n",
		"a.md":      "---\ntype: note\ntags: [x, \"\", y, x, ~]\nraw:\n  - {path: a.pdf}\n  - {path: b.pdf}\n---\n",
		"b.md":      "---\ntype: note\ntags: [x]\nraw:\n  - {path: a.pdf}\n  - {}\n---\n",
	})
	out, code := capture(t, func() int { return cmdProperty([]string{"tags", "--counts", "--format", "csv"}) })
	if want := "name,count\nx,2\ny,1\n"; code != 0 || out != want {
		t.Errorf("property tags:\n got %q\nwant %q", out, want)
	}
	out, code = capture(t, func() int { return cmdProperty([]string{"raw", "--counts", "--format", "csv"}) })
	if want := "name,count\n{path: a.pdf},2\n{path: b.pdf},1\n"; code != 0 || out != want {
		t.Errorf("property raw:\n got %q\nwant %q", out, want)
	}
}

// nestedBundle is the requester's nested shapes, for the golden outputs.
var nestedBundle = map[string]string{
	"wiki.toml": "spec=\"0.1\"\ntypes=[\"dataset\"]\n",
	"acme.md": "---\ntype: dataset\ntitle: 'Acme''s filings'\ndata_as_of:\n  income: 2025-12-31\n  balance: 2025-09-30\n" +
		"raw:\n  - path: a.pdf\n    pages: 40\n    meta: {form: 10-K, year: 2015}\ntags: [finance, \"10-K\", \"yes\"]\n---\n",
}

// list --format jsonl gives maps as objects and lists as arrays, nested keys
// in file order; the top level follows the csv columns (_path, type, then byte
// order).
func TestListJSONNested(t *testing.T) {
	writeFiles(t, nestedBundle)
	out, code := capture(t, func() int { return cmdList([]string{"--format", "jsonl"}) })
	want := `{"_path":"/acme.md","type":"dataset","data_as_of":{"income":"2025-12-31","balance":"2025-09-30"},` +
		`"raw":[{"path":"a.pdf","pages":40,"meta":{"form":"10-K","year":2015}}],"tags":["finance","10-K","yes"],"title":"Acme's filings"}` + "\n"
	if code != 0 || out != want {
		t.Errorf("jsonl:\n got %s\nwant %s", out, want)
	}
	// json is in file order at every depth, the top level included.
	out, code = capture(t, func() int { return cmdList([]string{"--format", "json"}) })
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(out)); err != nil {
		t.Fatal(err)
	}
	want = `[{"_path":"/acme.md","type":"dataset","title":"Acme's filings","data_as_of":{"income":"2025-12-31","balance":"2025-09-30"},` +
		`"raw":[{"path":"a.pdf","pages":40,"meta":{"form":"10-K","year":2015}}],"tags":["finance","10-K","yes"]}]`
	if code != 0 || compact.String() != want {
		t.Errorf("json:\n got %s\nwant %s", compact.String(), want)
	}
}

// list --format csv gives a map or a list as one line of YAML in flow style.
func TestListCSVNested(t *testing.T) {
	writeFiles(t, nestedBundle)
	out, code := capture(t, func() int { return cmdList([]string{"--format", "csv"}) })
	want := "_path,type,data_as_of,raw,tags,title\n" +
		`/acme.md,dataset,"{income: 2025-12-31, balance: 2025-09-30}","[{path: a.pdf, pages: 40, meta: {form: 10-K, year: 2015}}]","[finance, 10-K, ""yes""]",Acme's filings` + "\n"
	if code != 0 || out != want {
		t.Errorf("csv:\n got %q\nwant %q", out, want)
	}
}

// Every csv/tsv list cell is one line of YAML, never JSON: a frontmatter list
// and map, init's files, and table cells (scalars, which stay as they are).
func TestCSVListCellIsFlowYAML(t *testing.T) {
	writeFiles(t, map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\n",
		"n.md":      "---\ntype: note\nl: [a, \"b, c\", 10]\nm: {k: v}\n---\n| x | y |\n|---|---|\n| yes | [a] |\n",
	})
	out, code := capture(t, func() int { return cmdList([]string{"--format", "tsv", "--fields", "l,m"}) })
	if want := "_path\tl\tm\n/n.md\t\"[a, \"\"b, c\"\", 10]\"\t{k: v}\n"; code != 0 || out != want {
		t.Errorf("list tsv:\n got %q\nwant %q", out, want)
	}
	out, code = capture(t, func() int { return cmdTable([]string{"n.md", "--format", "csv"}) })
	if want := "x,y\nyes,[a]\n"; code != 0 || out != want {
		t.Errorf("table csv:\n got %q\nwant %q", out, want)
	}
	t.Chdir(t.TempDir())
	out, code = capture(t, func() int { return cmdInit([]string{"--format", "tsv"}) })
	if want := "dir\tfiles\n.\t[AGENTS.md, .gitignore, WORKFLOW.md, index.md, wiki.toml, CLAUDE.md]\n"; code != 0 || out != want {
		t.Errorf("init tsv:\n got %q\nwant %q", out, want)
	}
}

// move --include-frontmatter checks, and warns, only where it plans a rewrite.
// Shade's cases A and B (the "reference" is text inside another value, as YAML
// reads it) have no reference to the moved file: no warning, frontmatter as it
// is. E1, a quoted value continued at column 0, with a real reference, is
// rewritten as YAML reads it. A header wiki can't edit safely with a real
// reference (a flow-map header) keeps its frontmatter, with a warning naming
// the file and the reason. Every file still moves its body links.
func TestMoveFrontmatterFollowsWriteRules(t *testing.T) {
	files := map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\n",
		"a.md":      "---\ntype: note\n---\n# A\n",
		"case-a.md": "---\ntitle: \"first\nrelated: /a.md\"\ntype: note\n---\n[a](/a.md)\n",
		"case-b.md": "---\ntags: [x,\nrelated: /a.md]\ntype: note\n---\n[a](/a.md)\n",
		"e1.md":     "---\ntitle: \"Ship it\nsoon\"\nrelated: /a.md\n---\n[a](/a.md)\n",
		"flow.md":   "---\n{type: note, related: /a.md}\n---\n[a](/a.md)\n",
		"clean.md":  "---\ntype: note\nrelated: [/a.md]\n---\n[a](/a.md)\n",
	}
	writeFiles(t, files)
	out, errOut, code := captureAll(t, func() int { return cmdMove([]string{"a.md", "b.md", "--include-frontmatter"}) })
	want := "wiki: warning: frontmatter of /flow.md can't be edited safely: line 2 is not a key on its own line at column 0; " +
		"fix the file first (wiki check reports it); its frontmatter references are left as they are\n"
	if code != 0 || errOut != want {
		t.Fatalf("exit %d, stderr %q\nwant %q\n%s", code, errOut, want, out)
	}
	for _, f := range []string{"case-a.md", "case-b.md", "flow.md"} {
		got, _ := os.ReadFile(f)
		if wantFile := strings.Replace(files[f], "[a](/a.md)", "[a](./b.md)", 1); string(got) != wantFile {
			t.Errorf("%s: %q, want only its body link rewritten: %q", f, got, wantFile)
		}
	}
	for f, want := range map[string]string{
		"e1.md":    "---\ntitle: \"Ship it\nsoon\"\nrelated: /b.md\n---\n[a](./b.md)\n",
		"clean.md": "---\ntype: note\nrelated: [/b.md]\n---\n[a](./b.md)\n",
	} {
		if got, _ := os.ReadFile(f); string(got) != want {
			t.Errorf("%s: %q, want %q", f, got, want)
		}
	}
	if _, err := os.Stat("b.md"); err != nil {
		t.Errorf("not moved: %v", err)
	}
}

// A new value that needs quotes is written quoted, and reads back: here a
// destination with ": " in its name.
func TestMoveFrontmatterReadsBack(t *testing.T) {
	writeFiles(t, map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\n",
		"a.md":      "---\ntype: note\n---\n# A\n",
		"ref.md":    "---\ntype: note\nrelated: /a.md\n---\n# Ref\n",
	})
	_, errOut, code := captureAll(t, func() int { return cmdMove([]string{"a.md", "x: y.md", "--include-frontmatter"}) })
	got, _ := os.ReadFile("ref.md")
	if code != 0 || errOut != "" || string(got) != "---\ntype: note\nrelated: \"/x: y.md\"\n---\n# Ref\n" {
		t.Fatalf("exit %d, stderr %q, ref.md %q", code, errOut, got)
	}
	if fm, _ := parse.Frontmatter(string(got)); fm["related"] != "/x: y.md" {
		t.Errorf("reads back %q", fm["related"])
	}
}

// References are planned from YAML's reading of a header, so headers the
// scanner can't see keys in still reach the write rules: a flow-map header
// (case C) and a `?` key (case D) are refused there, and so is a rewrite that
// would change a key that isn't a reference through an anchor (D1, through a
// map). Each file keeps its frontmatter, with one warning naming it, and the
// move happens.
func TestMoveFrontmatterPlansFromYAML(t *testing.T) {
	files := map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\n",
		"a.md":      "---\ntype: note\n---\n# A\n",
		"c.md":      "---\n{type: note, related: /a.md}\n---\nc\n",
		"d.md":      "---\n? related\n: /a.md\ntype: note\n---\nd\n",
		"d1.md":     "---\nx: &r 1\nrelated: &r /a.md\nother: *r\n---\nd1\n",
		"d1-map.md": "---\nrelated: &r /a.md\nmeta: {ref: *r}\n---\nd1\n",
	}
	writeFiles(t, files)
	_, errOut, code := captureAll(t, func() int { return cmdMove([]string{"a.md", "b.md", "--include-frontmatter"}) })
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	for file, why := range map[string]string{
		"c.md":      "can't be edited safely: line 2 is not a key on its own line at column 0",
		"d.md":      "can't be edited safely: line 2 is not a key on its own line at column 0",
		"d1-map.md": "would change meta through an anchor; nothing written",
	} {
		want := "wiki: warning: frontmatter of /" + file + " " + why
		if strings.Count(errOut, "/"+file+" ") != 1 || !strings.Contains(errOut, want) {
			t.Errorf("%s: want one warning %q in\n%s", file, want, errOut)
		}
		if got, _ := os.ReadFile(file); string(got) != files[file] {
			t.Errorf("%s changed: %q", file, got)
		}
	}
	// D1 as written: other reads /a.md through the anchor, so it is a planned
	// reference too, and the rewrite of related's token moves both, exactly as
	// planned. Only related's line changes; x keeps its value.
	got, _ := os.ReadFile("d1.md")
	fm, _ := parse.Frontmatter(string(got))
	if string(got) != "---\nx: &r 1\nrelated: &r /b.md\nother: *r\n---\nd1\n" || fm["other"] != "/b.md" || fm["x"] != "1" || strings.Contains(errOut, "/d1.md") {
		t.Errorf("d1.md: %q, %v", got, fm)
	}
	if _, err := os.Stat("b.md"); err != nil {
		t.Errorf("not moved: %v", err)
	}
}
