package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/agentic-wiki/wiki/index"
)

func TestCmdCheckRejectsMalformedFrontmatter(t *testing.T) {
	cases := map[string]string{
		"unclosed purpose quote": "---\ntype: note\npurpose: \"Keep historical choices.\n---\n# Body\n",
		"unclosed title quote":   "---\ntype: note\ntitle: \"Unclosed\n---\n# Body\n",
		"unclosed list":          "---\ntype: note\ntags: [history\n---\n# Body\n",
		"duplicate key":          "---\ntype: note\ntitle: First\ntitle: Second\n---\n# Body\n",
		"missing closing fence":  "---\ntype: note\ntitle: Unclosed\n# Body\n",
		"nonmapping header":      "---\n- one\n- two\n---\n# Body\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			t.Chdir(writeBundle(t))
			if err := os.WriteFile("guide.md", []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{nil, {"--format", "json"}, {"--fix"}} {
				out, code := capture(t, func() int { return cmdCheck(args) })
				if code != 1 || !strings.Contains(out, "invalid frontmatter") || !strings.Contains(out, "/guide.md") {
					t.Errorf("check %v: exit=%d output=%s", args, code, out)
				}
			}
			got, err := os.ReadFile("guide.md")
			if err != nil || string(got) != content {
				t.Fatalf("check rewrote malformed input: %q, %v", got, err)
			}
		})
	}
}

func TestCmdCheckReportsEveryMalformedHeader(t *testing.T) {
	t.Chdir(writeBundle(t))
	for _, name := range []string{"guide.md", "flat.md", "index.md"} {
		if err := os.WriteFile(name, []byte("---\ntype: note\ntitle: \"Unclosed\n---\n# Body\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, code := capture(t, func() int { return cmdCheck([]string{"--format", "json"}) })
	if code != 1 {
		t.Fatalf("exit=%d, want 1: %s", code, out)
	}
	var issues []index.Issue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, issue := range issues {
		if issue.Level == "error" && strings.Contains(issue.Msg, "invalid frontmatter") {
			seen[issue.Entry] = true
		}
	}
	if len(seen) != 3 {
		t.Fatalf("want all 3 malformed files, got %+v", issues)
	}
}

func TestCmdCheckFixLeavesMalformedRootHeader(t *testing.T) {
	t.Chdir(writeBundle(t))
	const content = "---\nokf_version: \"Unclosed\n---\n# Home\n"
	if err := os.WriteFile("index.md", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := capture(t, func() int { return cmdCheck([]string{"--fix"}) })
	if code != 1 || !strings.Contains(out, "/index.md: invalid frontmatter") {
		t.Fatalf("exit=%d output=%s", code, out)
	}
	got, err := os.ReadFile("index.md")
	if err != nil || string(got) != content {
		t.Fatalf("check --fix changed malformed root: %q, %v", got, err)
	}
}

func TestCmdCheckAcceptsValidFrontmatter(t *testing.T) {
	t.Chdir(writeBundle(t))
	const content = "---\r\ntype: note\r\ntitle: An embedded \"quote in plain text\r\nquoted: \"A \\\"quote\\\" # kept\" # comment\r\ntags:\r\n  - one\r\n  - two\r\nsummary: >\r\n  First line\r\n  second line\r\nbase: &base value\r\ncopy: *base\r\n---\r\n# Body\r\n"
	if err := os.WriteFile("guide.md", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := capture(t, func() int { return cmdCheck(nil) })
	if code != 0 || out != "ok: no issues found\n" {
		t.Fatalf("valid YAML rejected: exit=%d output=%s", code, out)
	}
}

// A nested map, and an alias to it, are valid YAML and pass check. A merge key
// through that alias is reported once, where it is written: the alias is not
// followed.
func TestCmdCheckNestedAnchor(t *testing.T) {
	t.Chdir(writeBundle(t))
	const nested = "---\r\ntype: note\r\nbase: &base\r\n  key: value\r\ncopy: *base\r\n---\r\n# Body\r\n"
	if err := os.WriteFile("guide.md", []byte(nested), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, code := capture(t, func() int { return cmdCheck(nil) }); code != 0 || out != "ok: no issues found\n" {
		t.Errorf("nested map: exit=%d output=%s", code, out)
	}
	merged := strings.Replace(nested, "copy: *base\r\n", "copy:\r\n  <<: *base\r\nmore: *base\r\n", 1)
	if err := os.WriteFile("guide.md", []byte(merged), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := capture(t, func() int { return cmdCheck(nil) })
	want := `/guide.md: line 6: "<<" merges keys, which wiki does not read; write the keys out`
	if code != 1 || strings.Contains(out, "invalid frontmatter") || strings.Count(out, "\n") != 1 || !strings.Contains(out, want) {
		t.Errorf("merge key: exit=%d output=%s", code, out)
	}
}

// list --format json gives unquoted numbers, booleans and null as JSON values,
// numbers byte for byte as written; quoted values and dates stay strings.
func TestCmdListJSONTyped(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for name, content := range map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"entry\"]\n",
		"e.md":      "---\ntype: entry\nn: 10\nx: 0.2\nb: true\nz: null\ns: '7372'\nl: [1, 2.5]\nd: 2026-06-30\n---\nbody\n",
	} {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, code := capture(t, func() int { return cmdList([]string{"--format", "json"}) })
	if code != 0 {
		t.Fatalf("exit=%d: %s", code, out)
	}
	var got []json.RawMessage
	if err := json.Unmarshal([]byte(out), &got); err != nil || len(got) != 1 {
		t.Fatalf("want one entry: %v %s", err, out)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, got[0]); err != nil {
		t.Fatal(err)
	}
	want := `{"_path":"/e.md","type":"entry","n":10,"x":0.2,"b":true,"z":null,"s":"7372","l":[1,2.5],"d":"2026-06-30"}` // keys in file order
	if compact.String() != want {
		t.Errorf("list --format json\n got %s\nwant %s", compact.String(), want)
	}
}

// --where compares text on the string view: a quoted and an unquoted 10 match
// alike. key= matches no value by parse.Empty: empty, null and ~ alike.
func TestCmdListWhereComparesText(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for name, content := range map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"entry\"]\n",
		"bare.md":   "---\ntype: entry\nn: 10\nepoch:\n---\n",
		"quoted.md": "---\ntype: entry\nn: '10'\nepoch: ~\n---\n",
		"other.md":  "---\ntype: entry\nn: 11\nepoch: null\n---\n",
	} {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for where, want := range map[string]string{
		"n=10":   "/bare.md /quoted.md",
		"epoch=": "/bare.md /other.md /quoted.md",
	} {
		out, code := capture(t, func() int { return cmdList([]string{"--where", where}) })
		var paths []string
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			paths = append(paths, strings.Fields(line)[0])
		}
		if code != 0 || strings.Join(paths, " ") != want {
			t.Errorf("--where %s: %q (exit %d), want %s", where, out, code, want)
		}
	}
}

// writeCSVBundle writes two entries with different keys, for list csv/tsv.
func writeCSVBundle(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	for name, content := range map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"entry\"]\n",
		"a.md":      "---\ntype: entry\nn: 10\nx: 0.2\nb: true\ns: '7372'\nl: [1, \"a,b\", \"<i>\"]\nz: ~\n---\n",
		"b.md":      "---\ntype: entry\nn: 7\nd: 2026-06-30\n---\n",
	} {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Without --fields: _path, type, then every other key any listed entry uses, in
// byte order. A missing key and null are empty cells; a list is its Flow.
func TestCmdListCSVColumns(t *testing.T) {
	writeCSVBundle(t)
	out, errOut, code := captureAll(t, func() int { return cmdList([]string{"--format", "csv"}) })
	want := "_path,type,b,d,l,n,s,x,z\n" +
		"/a.md,entry,true,,\"[1, \"\"a,b\"\", \"\"<i>\"\"]\",10,7372,0.2,\n" +
		"/b.md,entry,,2026-06-30,,7,,,\n"
	if code != 0 || out != want || errOut != "" {
		t.Errorf("list --format csv: exit %d, stderr %q\n got %q\nwant %q", code, errOut, out, want)
	}

	out, _, code = captureAll(t, func() int { return cmdList([]string{"--format", "tsv", "--fields", "n,d"}) })
	if want := "_path\tn\td\n/a.md\t10\t\n/b.md\t7\t2026-06-30\n"; code != 0 || out != want {
		t.Errorf("list --format tsv --fields n,d: exit %d\n got %q\nwant %q", code, out, want)
	}
}

// --fields chooses and orders the columns after _path; spaces around keys are
// trimmed, and type may be named like any key.
func TestCmdListCSVFields(t *testing.T) {
	writeCSVBundle(t)
	for _, tc := range []struct {
		fields, want string
	}{
		{"n,x,b", "_path,n,x,b\n/a.md,10,0.2,true\n/b.md,7,,\n"},
		{"x, n", "_path,x,n\n/a.md,0.2,10\n/b.md,,7\n"},
		{"type,n", "_path,type,n\n/a.md,entry,10\n/b.md,entry,7\n"},
	} {
		out, errOut, code := captureAll(t, func() int { return cmdList([]string{"--format", "csv", "--fields", tc.fields}) })
		if code != 0 || out != tc.want || errOut != "" {
			t.Errorf("--fields %q: exit %d, stderr %q\n got %q\nwant %q", tc.fields, code, errOut, out, tc.want)
		}
	}
}

// Each --fields error exits 2, writes nothing to stdout and names the problem.
func TestCmdListFieldsErrors(t *testing.T) {
	writeCSVBundle(t)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--format", "csv", "--fields", "n,,x"}, "wiki: --fields has an empty key at position 2 (use --fields a,b,c)\n"},
		{[]string{"--format", "csv", "--fields", ""}, "wiki: --fields has an empty key at position 1 (use --fields a,b,c)\n"},
		{[]string{"--format", "csv", "--fields", "n, "}, "wiki: --fields has an empty key at position 2 (use --fields a,b,c)\n"},
		{[]string{"--format", "csv", "--fields", "n,x,n"}, "wiki: --fields names \"n\" twice\n"},
		{[]string{"--format", "csv", "--fields", "_path,n"}, "wiki: --fields can't name _path: it is always the first column\n"},
		{[]string{"--fields", "n"}, "wiki: --fields needs --format csv or --format tsv\n"},
		{[]string{"--format", "json", "--fields", "n"}, "wiki: --fields needs --format csv or --format tsv\n"},
	} {
		out, errOut, code := captureAll(t, func() int { return cmdList(tc.args) })
		if code != 2 || out != "" || errOut != tc.want {
			t.Errorf("list %q: exit %d, stdout %q\n stderr %q\n  want %q", tc.args, code, out, errOut, tc.want)
		}
	}
}

// A --fields key no listed entry has is still a column, with one warning line
// per key on stderr, in --fields order, and exit 0. With no entries listed,
// only the header is written and nothing is warned about.
func TestCmdListFieldsMissingKeys(t *testing.T) {
	writeCSVBundle(t)
	out, errOut, code := captureAll(t, func() int { return cmdList([]string{"--format", "csv", "--fields", "roe,n,pe"}) })
	if want := "_path,roe,n,pe\n/a.md,,10,\n/b.md,,7,\n"; code != 0 || out != want {
		t.Errorf("exit %d\n got %q\nwant %q", code, out, want)
	}
	wantErr := "wiki: warning: --fields: no listed entry has \"roe\"\nwiki: warning: --fields: no listed entry has \"pe\"\n"
	if errOut != wantErr {
		t.Errorf("stderr %q, want %q", errOut, wantErr)
	}

	out, errOut, code = captureAll(t, func() int {
		return cmdList([]string{"--format", "csv", "--fields", "roe", "--where", "type=none"})
	})
	if code != 0 || out != "_path,roe\n" || errOut != "" {
		t.Errorf("no entries: exit %d, stdout %q, stderr %q", code, out, errOut)
	}
}

// orphans emits entries like list does, so its json is typed too.
func TestCmdOrphansJSONTyped(t *testing.T) {
	writeCSVBundle(t)
	out, code := capture(t, func() int { return cmdOrphans([]string{"--format", "json"}) })
	if code != 0 || !strings.Contains(out, `"n": 10`) || !strings.Contains(out, `"s": "7372"`) {
		t.Errorf("orphans --format json: exit %d: %s", code, out)
	}
}

// A file whose rewritten key appears twice keeps its frontmatter, with one
// warning naming it, and the move goes on.
func TestCmdMoveSkipsDuplicateKey(t *testing.T) {
	writeFiles(t, map[string]string{
		"wiki.toml": "spec=\"0.1\"\ntypes=[\"note\"]\n",
		"x.md":      "---\ntype: note\n---\n",
		"a.md":      "---\ntype: note\nrelated: /x.md\nrelated: /x.md\n---\n",
	})
	out, errOut, code := captureAll(t, func() int { return cmdMove([]string{"x.md", "/y.md", "--include-frontmatter"}) })
	want := "wiki: warning: frontmatter of /a.md has key \"related\" twice (lines 3 and 4); fix the file first (wiki check reports it); its frontmatter references are left as they are\n"
	if code != 0 || errOut != want || !strings.Contains(out, "moved /x.md -> /y.md") {
		t.Errorf("exit %d, stdout %q\n stderr %q\n   want %q", code, out, errOut, want)
	}
	if got, _ := os.ReadFile("a.md"); string(got) != "---\ntype: note\nrelated: /x.md\nrelated: /x.md\n---\n" {
		t.Errorf("a.md changed: %q", got)
	}
}

// A shape issue is an error next to the entry's other issues, without the
// "invalid frontmatter:" prefix, which stays for syntax errors.
func TestCmdCheckReportsShapeIssues(t *testing.T) {
	t.Chdir(writeBundle(t))
	if err := os.WriteFile("guide.md", []byte("---\ntitle: Guide\nbase: &b {a: 1}\nmeta:\n  <<: *b\n---\n# Guide\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := capture(t, func() int { return cmdCheck([]string{"--format", "json"}) })
	if code != 1 {
		t.Fatalf("exit=%d, want 1: %s", code, out)
	}
	var issues []index.Issue
	if err := json.Unmarshal([]byte(out), &issues); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		`line 5: "<<" merges keys, which wiki does not read; write the keys out`:                          false,
		"missing required `type` (or add this file to `ignore` in wiki.toml if it is not a bundle entry)": false,
	}
	for _, is := range issues {
		if is.Entry != "/guide.md" {
			continue
		}
		if strings.Contains(is.Msg, "invalid frontmatter") {
			t.Errorf("a shape issue has the syntax prefix: %q", is.Msg)
		}
		if _, ok := want[is.Msg]; ok && is.Level == "error" {
			want[is.Msg] = true
		}
	}
	for msg, seen := range want {
		if !seen {
			t.Errorf("missing error %q in %s", msg, out)
		}
	}
}

// check --fix still repairs okf_version on a root index.md whose header has
// a shape issue, and still reports the issue.
func TestCmdCheckFixWithShapeIssue(t *testing.T) {
	dir := writeBundle(t)
	t.Chdir(dir)
	if err := os.WriteFile("index.md", []byte("---\nokf_version: \"0.2\"\n\"m\\x65ta\": 1\n---\n# Home\n[g](/guide.md)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code := capture(t, func() int { return cmdCheck([]string{"--fix"}) })
	if code != 1 || !strings.Contains(out, "okf_version") || !strings.Contains(out, `key "meta" is written with escapes`) {
		t.Errorf("check --fix: exit %d\n%s", code, out)
	}
	if got, _ := os.ReadFile("index.md"); string(got) != "---\nokf_version: \"0.1\"\n\"m\\x65ta\": 1\n---\n# Home\n[g](/guide.md)\n" {
		t.Errorf("index.md = %q", got)
	}
}
