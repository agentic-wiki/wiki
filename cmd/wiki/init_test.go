package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runChild runs `wiki args…` in a child copy of the test binary, in dir: every
// command's flag set is flag.ExitOnError, so a bad flag ends the process. The
// arguments go across as JSON.
func runChild(t *testing.T, dir string, args []string) (stdout, stderr string, code int) {
	t.Helper()
	data, _ := json.Marshal(args)
	cmd := exec.Command(os.Args[0], "-test.run=^TestChildRun$")
	cmd.Env = append(os.Environ(), "WIKI_TEST_RUN="+string(data))
	cmd.Dir = dir
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		code = exit.ExitCode()
	case err != nil:
		t.Fatal(err)
	}
	return out.String(), errOut.String(), code
}

// TestChildRun is the child's side of runChild: it runs the command and exits.
func TestChildRun(t *testing.T) {
	data := os.Getenv("WIKI_TEST_RUN")
	if data == "" {
		t.Skip("run by runChild")
	}
	var args []string
	if err := json.Unmarshal([]byte(data), &args); err != nil {
		os.Exit(3)
	}
	os.Exit(run(args))
}

// init --shape writes the parts and lists what it wrote.
func TestInitShape(t *testing.T) {
	dir := t.TempDir()
	out, code := capture(t, func() int { return cmdInit([]string{dir, "--shape", "specs, kanban"}) })
	if code != 0 {
		t.Fatalf("exit %d\n%s", code, out)
	}
	for _, f := range []string{"product/index.md", "product/technical-debt.md", "backlog/index.md", "backlog/archive/index.md", "index.md", "AGENTS.md", "wiki.toml"} {
		if !strings.Contains(out, "  "+f+"\n") {
			t.Errorf("not listed: %s\n%s", f, out)
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			t.Errorf("not written: %s", f)
		}
	}
}

// A bad --shape, or the removed --workflow, exits 2 with its message, and
// nothing is written.
func TestInitShapeErrors(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"--shape", "nope"}, `wiki: unknown part "nope" in --shape (use notes, kanban, records or specs)`},
		{[]string{"--shape", "kanban,"}, "wiki: empty part in --shape"},
		{[]string{"--shape", "kanban,kanban"}, `wiki: part "kanban" is listed twice in --shape`},
		{[]string{"--workflow", "default"}, "flag provided but not defined: -workflow"},
	} {
		dir := t.TempDir()
		_, stderr, code := runChild(t, dir, append([]string{"init"}, tc.args...))
		if code != 2 || !strings.Contains(stderr, tc.want) {
			t.Errorf("init %q: exit %d, stderr %q; want 2 and %q", tc.args, code, stderr, tc.want)
		}
		if entries, _ := os.ReadDir(dir); len(entries) != 0 {
			t.Errorf("init %q wrote %d files", tc.args, len(entries))
		}
	}
}

// docLines are the `wiki …` lines of the sh blocks in a scaffolded doc.
func docLines(doc string) []string {
	var lines []string
	inSh := false
	for _, line := range strings.Split(doc, "\n") {
		switch {
		case strings.HasPrefix(line, "```"):
			inSh = line == "```sh"
		case inSh && strings.HasPrefix(line, "wiki "):
			lines = append(lines, line)
		}
	}
	return lines
}

// docArgs splits a doc line into arguments, honouring "…" and '…', and cutting
// at an unquoted | (a pipe to jq) or an unquoted " #" (the comment). The first
// argument, wiki, is dropped.
func docArgs(line string) []string {
	var args []string
	var arg strings.Builder
	inArg := false
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0 && c == quote:
			quote = 0
		case quote != 0:
			arg.WriteByte(c)
		case c == '"' || c == '\'':
			quote, inArg = c, true
		case c == '|' || c == '#' && i > 0 && line[i-1] == ' ':
			i = len(line)
		case c == ' ' || c == '\t':
			if inArg {
				args = append(args, arg.String())
				arg.Reset()
				inArg = false
			}
		default:
			arg.WriteByte(c)
			inArg = true
		}
	}
	if inArg {
		args = append(args, arg.String())
	}
	return args[1:]
}

// Every `wiki …` line the scaffolded AGENTS.md shows runs: its command exists
// and its flags parse. Each runs in the wiki it was written for (the minimal
// starter, each part alone, all four in folders). A missing entry is fine (the
// docs' example paths don't exist in a new wiki); an unknown command or flag,
// a bad value, or a usage line (a wrong number of arguments) is the drift this
// test exists for.
func TestDocCommandsRun(t *testing.T) {
	shapes := [][]string{nil, {"notes"}, {"kanban"}, {"records"}, {"specs"}, {"notes", "kanban", "records", "specs"}}
	ran := 0
	for _, shape := range shapes {
		dir := t.TempDir()
		args := []string{dir}
		if shape != nil {
			args = append(args, "--shape", strings.Join(shape, ","))
		}
		if _, code := capture(t, func() int { return cmdInit(args) }); code != 0 {
			t.Fatalf("init %v: exit %d", shape, code)
		}
		text, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
		for _, line := range docLines(string(text)) {
			ran++
			_, stderr, code := runChild(t, dir, docArgs(line))
			for _, marker := range []string{"unknown command", "flag provided but not defined", "invalid value", "usage: wiki"} {
				if code == 2 && strings.Contains(stderr, marker) {
					t.Errorf("%v: %q: %s", shape, line, strings.TrimSpace(stderr))
					break
				}
			}
		}
	}
	if ran == 0 {
		t.Fatal("no doc command found")
	}
	t.Logf("%d doc command lines run", ran)
}
