package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/agentic-wiki/wiki/internal/index"
	"github.com/agentic-wiki/wiki/internal/output"
	"github.com/agentic-wiki/wiki/internal/parse"
	"github.com/agentic-wiki/wiki/internal/scaffold"
)

func cmdInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	force := fs.Bool("force", false, "write into a non-empty directory")
	format := fs.String("format", "text", "output format: text|json")
	fs.Parse(args)
	dir := "."
	if fs.NArg() >= 1 {
		dir = fs.Arg(0)
		fs.Parse(fs.Args()[1:]) // pick up flags that followed the path
	}
	written, err := scaffold.Write(dir, *force)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
	}
	out := struct {
		Dir   string   `json:"dir"`
		Files []string `json:"files"`
	}{dir, written}
	lines := []string{"initialized agentic-wiki bundle in " + dir}
	for _, f := range written {
		lines = append(lines, "  "+f)
	}
	output.Emit(os.Stdout, *format, lines, out)
	return 0
}

func cmdStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	links, tasks := 0, 0
	tagset := map[string]bool{}
	for _, e := range idx.Entries {
		links += len(e.Links)
		tasks += len(e.Tasks)
		for _, t := range e.Tags {
			tagset[t] = true
		}
	}
	s := struct {
		Dir     string `json:"dir"`
		Spec    string `json:"spec"`
		Entries int    `json:"entries"`
		Links   int    `json:"links"`
		Tags    int    `json:"tags"`
		Tasks   int    `json:"tasks"`
		Broken  int    `json:"broken"`
		Orphans int    `json:"orphans"`
	}{
		idx.Bundle.Dir, idx.Bundle.Spec,
		len(idx.Entries), links, len(tagset), tasks,
		len(idx.Broken()), len(idx.Orphans()),
	}
	lines := []string{
		"Dir:      " + s.Dir,
		"Spec:     " + s.Spec,
		fmt.Sprintf("Entries:  %d", s.Entries),
		fmt.Sprintf("Links:    %d", s.Links),
		fmt.Sprintf("Tags:     %d", s.Tags),
		fmt.Sprintf("Tasks:    %d", s.Tasks),
		fmt.Sprintf("Broken:   %d", s.Broken),
		fmt.Sprintf("Orphans:  %d", s.Orphans),
	}
	output.Emit(os.Stdout, *format, lines, s)
	return 0
}

func cmdList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json")
	typ := fs.String("type", "", "filter by type")
	tag := fs.String("tag", "", "filter by tag")
	prefix := fs.String("prefix", "", "filter to a path prefix")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	entries := idx.Filter(*typ, *tag, *prefix)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	var lines []string
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("%-44s %-9s %s", e.Path, e.Type, e.Title))
	}
	output.Emit(os.Stdout, *format, lines, entries)
	if len(entries) == 0 {
		return 1
	}
	return 0
}

func cmdTasks(args []string) int {
	fs := flag.NewFlagSet("tasks", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json")
	all := fs.Bool("all", false, "include done tasks")
	done := fs.Bool("done", false, "only done tasks")
	prefix := fs.String("prefix", "", "filter to a path prefix")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	type row struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Done bool   `json:"done"`
		Text string `json:"text"`
	}
	var rows []row
	var lines []string
	for _, e := range idx.Entries {
		if *prefix != "" && !strings.HasPrefix(strings.TrimPrefix(e.Path, "/"), strings.TrimPrefix(*prefix, "/")) {
			continue
		}
		for _, t := range e.Tasks {
			switch {
			case *done && !t.Done:
				continue
			case !*all && !*done && t.Done:
				continue
			}
			rows = append(rows, row{e.Path, t.Line, t.Done, t.Text})
			box := " "
			if t.Done {
				box = "x"
			}
			lines = append(lines, fmt.Sprintf("[%s] %s:%d  %s", box, e.Path, t.Line, t.Text))
		}
	}
	output.Emit(os.Stdout, *format, lines, rows)
	if len(rows) == 0 {
		return 1
	}
	return 0
}

func cmdUnresolved(args []string) int {
	fs := flag.NewFlagSet("unresolved", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	broken := idx.Broken()
	var lines []string
	for _, b := range broken {
		lines = append(lines, fmt.Sprintf("%s:%d -> %s", b.From, b.Line, b.Target))
	}
	output.Emit(os.Stdout, *format, lines, broken)
	if len(broken) == 0 {
		return 1
	}
	return 0
}

func cmdOrphans(args []string) int {
	fs := flag.NewFlagSet("orphans", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	orph := idx.Orphans()
	sort.Slice(orph, func(i, j int) bool { return orph[i].Path < orph[j].Path })
	var lines []string
	for _, e := range orph {
		lines = append(lines, e.Path)
	}
	output.Emit(os.Stdout, *format, lines, orph)
	if len(orph) == 0 {
		return 1
	}
	return 0
}

func cmdCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json")
	fix := fs.Bool("fix", false, "apply safe repairs, e.g. sync okf_version (writes files)")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	var fixes []index.Fix
	if *fix {
		applied, err := idx.Fix(true)
		if err != nil {
			fmt.Fprintln(os.Stderr, "wiki:", err)
			return 2
		}
		fixes = applied
		if len(applied) > 0 { // re-read so remaining issues reflect the writes
			if rebuilt, c := loadIndex(); c == 0 {
				idx = rebuilt
			}
		}
	}

	issues := idx.Check()
	errs := 0
	var lines []string
	for _, fx := range fixes {
		lines = append(lines, fmt.Sprintf("fixed   %s: %s %q -> %q", fx.Entry, fx.Field, fx.From, fx.To))
	}
	for _, is := range issues {
		if is.Level == "error" {
			errs++
		}
		lines = append(lines, fmt.Sprintf("%-7s %s: %s", is.Level, is.Entry, is.Msg))
	}
	if len(lines) == 0 {
		lines = []string{"ok: no issues found"}
	}
	if *fix {
		if fixes == nil {
			fixes = []index.Fix{}
		}
		if issues == nil {
			issues = []index.Issue{}
		}
		output.Emit(os.Stdout, *format, lines, struct {
			Fixed  []index.Fix   `json:"fixed"`
			Issues []index.Issue `json:"issues"`
		}{fixes, issues})
	} else {
		output.Emit(os.Stdout, *format, lines, issues)
	}
	if errs > 0 {
		return 1
	}
	return 0
}

func cmdRead(args []string) int {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json")
	target, ok := parseWithArg(fs, args)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: wiki read <file>")
		return 2
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}
	e, err := idx.Resolve(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
	}
	body, err := e.Body()
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
	}
	out := struct {
		Path  string `json:"path"`
		Type  string `json:"type"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}{e.Path, e.Type, e.Title, body}
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	output.Emit(os.Stdout, *format, lines, out)
	return 0
}

func cmdOutline(args []string) int {
	fs := flag.NewFlagSet("outline", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json")
	target, ok := parseWithArg(fs, args)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: wiki outline <file>")
		return 2
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}
	e, err := idx.Resolve(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
	}
	var lines []string
	for _, h := range e.Headings {
		lines = append(lines, strings.Repeat("  ", h.Level-1)+h.Text)
	}
	heads := e.Headings
	if heads == nil {
		heads = []parse.Heading{}
	}
	out := struct {
		Path     string          `json:"path"`
		Headings []parse.Heading `json:"headings"`
	}{e.Path, heads}
	output.Emit(os.Stdout, *format, lines, out)
	return 0
}

func cmdSearch(args []string) int {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json")
	typ := fs.String("type", "", "filter by type")
	tag := fs.String("tag", "", "filter by tag")
	prefix := fs.String("prefix", "", "filter to a path prefix")
	showLines := fs.Bool("lines", false, "show matching lines instead of entries")
	query, ok := parseWithArg(fs, args)
	if !ok || strings.TrimSpace(query) == "" {
		fmt.Fprintln(os.Stderr, "usage: wiki search <query> [--type --tag --prefix --lines]  (quote a multi-word query)")
		return 2
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	hits := idx.Search(query, *typ, *tag, *prefix)
	var lines []string
	if *showLines {
		for _, h := range hits {
			for _, ln := range h.Lines {
				lines = append(lines, fmt.Sprintf("%s:%d: %s", h.Path, ln.Line, strings.TrimRight(ln.Text, " \t\r")))
			}
		}
	} else {
		for _, h := range hits {
			lines = append(lines, fmt.Sprintf("%-44s %-9s %s", h.Path, h.Type, h.Title))
		}
		for i := range hits {
			hits[i].Lines = nil // json carries per-line detail only with --lines
		}
	}
	output.Emit(os.Stdout, *format, lines, hits)
	if len(hits) == 0 {
		return 1
	}
	return 0
}

// parseWithArg parses fs allowing flags on either side of a single positional
// argument, which it returns. ok is false unless there is exactly one positional.
func parseWithArg(fs *flag.FlagSet, args []string) (string, bool) {
	fs.Parse(args)
	if fs.NArg() == 0 {
		return "", false
	}
	arg := fs.Arg(0)
	fs.Parse(fs.Args()[1:]) // pick up flags that followed the positional
	if fs.NArg() != 0 {
		return "", false
	}
	return arg, true
}

// parseWith2Args is parseWithArg for commands taking exactly two positionals.
func parseWith2Args(fs *flag.FlagSet, args []string) (string, string, bool) {
	fs.Parse(args)
	if fs.NArg() < 2 {
		return "", "", false
	}
	a, b := fs.Arg(0), fs.Arg(1)
	fs.Parse(fs.Args()[2:]) // pick up flags that followed the positionals
	if fs.NArg() != 0 {
		return "", "", false
	}
	return a, b, true
}

func cmdMove(args []string) int {
	fs := flag.NewFlagSet("move", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "preview the move without writing")
	format := fs.String("format", "text", "output format: text|json")
	src, dest, ok := parseWith2Args(fs, args)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: wiki move <src> <dest> [--dry-run]")
		return 2
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}
	res, err := idx.Move(src, dest, *dryRun)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
	}
	emitMove(res, *dryRun, *format)
	return 0
}

func emitMove(res *index.MoveResult, dryRun bool, format string) {
	verb := "moved"
	if dryRun {
		verb = "would move"
	}
	lines := []string{fmt.Sprintf("%s %s -> %s", verb, res.From, res.To)}
	for _, rw := range res.Rewrites {
		lines = append(lines, fmt.Sprintf("  %d link(s) in %s", rw.Links, rw.Path))
	}
	output.Emit(os.Stdout, format, lines, res)
}

func cmdLinks(args []string) int {
	fs := flag.NewFlagSet("links", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json")
	target, ok := parseWithArg(fs, args)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: wiki links <file>")
		return 2
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}
	e, err := idx.Resolve(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
	}
	refs := idx.OutLinks(e)
	var lines []string
	for _, r := range refs {
		lines = append(lines, r.To)
	}
	output.Emit(os.Stdout, *format, lines, refs)
	if len(refs) == 0 {
		return 1
	}
	return 0
}

func cmdBacklinks(args []string) int {
	fs := flag.NewFlagSet("backlinks", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json")
	target, ok := parseWithArg(fs, args)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: wiki backlinks <file>")
		return 2
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}
	e, err := idx.Resolve(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
	}
	refs := idx.Backlinks(e.Path)
	var lines []string
	for _, r := range refs {
		lines = append(lines, fmt.Sprintf("%s:%d  %s", r.From, r.Line, r.Text))
	}
	output.Emit(os.Stdout, *format, lines, refs)
	if len(refs) == 0 {
		return 1
	}
	return 0
}
