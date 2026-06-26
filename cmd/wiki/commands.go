package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/agentic-wiki/wiki/internal/output"
	"github.com/agentic-wiki/wiki/internal/parse"
)

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
	path := fs.String("path", "", "filter by path prefix")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	entries := idx.Filter(*typ, *tag, *path)
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
	path := fs.String("path", "", "filter by path prefix")
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
		if *path != "" && !strings.HasPrefix(strings.TrimPrefix(e.Path, "/"), strings.TrimPrefix(*path, "/")) {
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
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	issues := idx.Check()
	errs := 0
	var lines []string
	for _, is := range issues {
		if is.Level == "error" {
			errs++
		}
		lines = append(lines, fmt.Sprintf("%-7s %s: %s", is.Level, is.Entry, is.Msg))
	}
	if len(issues) == 0 {
		lines = []string{"ok: no issues found"}
	}
	output.Emit(os.Stdout, *format, lines, issues)
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
	path := fs.String("path", "", "filter by path prefix")
	showLines := fs.Bool("lines", false, "show matching lines instead of entries")
	query, ok := parseWithArg(fs, args)
	if !ok || strings.TrimSpace(query) == "" {
		fmt.Fprintln(os.Stderr, "usage: wiki search <query> [--type --tag --path --lines]  (quote a multi-word query)")
		return 2
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	hits := idx.Search(query, *typ, *tag, *path)
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
