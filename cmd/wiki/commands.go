package main

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/agentic-wiki/wiki/index"
	"github.com/agentic-wiki/wiki/internal/output"
	"github.com/agentic-wiki/wiki/internal/scaffold"
	"github.com/agentic-wiki/wiki/parse"
)

func cmdInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	force := fs.Bool("force", false, "write into a non-empty directory")
	workflow := fs.String("workflow", "", fmt.Sprintf("starter workflow: %s (default: %s)", strings.Join(scaffold.Workflows(), ", "), scaffold.DefaultWorkflow))
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
	fs.Parse(args)
	dir := "."
	if fs.NArg() >= 1 {
		dir = fs.Arg(0)
		fs.Parse(fs.Args()[1:]) // pick up flags that followed the path
	}
	if *workflow == "" {
		*workflow = scaffold.DefaultWorkflow
		fmt.Fprintf(os.Stderr, "Using '%s' workflow.\n", *workflow)
	}
	written, err := scaffold.Write(dir, *workflow, *force)
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
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	links, checkboxes := 0, 0
	for _, e := range idx.Entries {
		links += len(e.Links)
		checkboxes += len(e.Checkboxes)
	}
	s := struct {
		Dir        string `json:"dir"`
		Spec       string `json:"spec"`
		Entries    int    `json:"entries"`
		Links      int    `json:"links"`
		Tags       int    `json:"tags"`
		Checkboxes int    `json:"checkboxes"` // GFM `- [ ]` items, not type:task entries (use `list --where type=task`)
		Broken     int    `json:"broken"`
		Orphans    int    `json:"orphans"`
	}{
		idx.Bundle.Dir, idx.Bundle.Spec,
		len(idx.Entries), links, len(idx.TagCounts("")), checkboxes,
		len(idx.Broken()), len(idx.Orphans()),
	}
	lines := []string{
		"Dir:        " + s.Dir,
		"Spec:       " + s.Spec,
		fmt.Sprintf("Entries:    %d", s.Entries),
		fmt.Sprintf("Links:      %d", s.Links),
		fmt.Sprintf("Tags:       %d", s.Tags),
		fmt.Sprintf("Checkboxes: %d", s.Checkboxes),
		fmt.Sprintf("Broken:     %d", s.Broken),
		fmt.Sprintf("Orphans:    %d", s.Orphans),
	}
	output.Emit(os.Stdout, *format, lines, s)
	return 0
}

// whereFilters collects repeated `--where key=value` flags into the property
// filters Filter/Search apply; an entry must match every one (AND). It implements
// flag.Value so `--where` can be given more than once. `type` and `tags` are
// ordinary frontmatter fields, matched as `--where type=note` / `--where tags=bug`.
type whereFilters []index.PropFilter

func (w *whereFilters) String() string { return "" }

func (w *whereFilters) Set(s string) error {
	f, err := index.ParseFilter(s)
	if err != nil {
		// The CLI keeps its own wording, which names the flag the user typed;
		// the library cannot, since it does not know it was reached from a flag.
		return fmt.Errorf("--where must be key=value or key!=value, got %q", s)
	}
	*w = append(*w, f)
	return nil
}

func cmdList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
	prefix := fs.String("prefix", "", "filter to a path prefix")
	var where whereFilters
	fs.Var(&where, "where", "filter by frontmatter key=value or key!=value (repeatable = AND; e.g. type=note, status!=done)")
	sortBy := fs.String("sort", "path", "sort order: path|timestamp (timestamp is newest-first)")
	reverse := fs.Bool("reverse", false, "reverse the sort order")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	entries := idx.Filter(*prefix, where)
	switch *sortBy {
	case "path":
		sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	case "timestamp":
		// Compute each entry's sort time once (the mtime fallback stats, so never
		// inside the comparator), then order newest-first with path breaking ties.
		type keyed struct {
			e *index.Entry
			t time.Time
		}
		ks := make([]keyed, len(entries))
		for i, e := range entries {
			ks[i] = keyed{e, e.SortTime()}
		}
		sort.SliceStable(ks, func(i, j int) bool {
			if ks[i].t.Equal(ks[j].t) {
				return ks[i].e.Path < ks[j].e.Path
			}
			return ks[i].t.After(ks[j].t)
		})
		for i, k := range ks {
			entries[i] = k.e
		}
	default:
		fmt.Fprintf(os.Stderr, "wiki: unknown --sort %q (use path|timestamp)\n", *sortBy)
		return 2
	}
	if *reverse {
		slices.Reverse(entries)
	}
	var lines []string
	for _, e := range entries {
		lines = append(lines, fmt.Sprintf("%-44s %s", e.Path, e.Type))
	}
	output.Emit(os.Stdout, *format, lines, entries)
	return 0
}

// countRow is one name with its entry count: the row shape for tags/properties.
type countRow struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// sortedCounts turns a name->count map into rows sorted by name (default) or by
// count descending with name breaking ties (sortBy=="count").
func sortedCounts(counts map[string]int, sortBy string) []countRow {
	rows := make([]countRow, 0, len(counts))
	for n, c := range counts {
		rows = append(rows, countRow{n, c})
	}
	sort.Slice(rows, func(i, j int) bool {
		if sortBy == "count" && rows[i].Count != rows[j].Count {
			return rows[i].Count > rows[j].Count
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

// emitCounts renders count rows: text shows the name alone, or "name  count"
// with --counts; json always carries the count. An empty listing is still exit 0.
func emitCounts(format string, rows []countRow, withCounts bool) int {
	var lines []string
	for _, r := range rows {
		if withCounts {
			lines = append(lines, fmt.Sprintf("%-30s %d", r.Name, r.Count))
		} else {
			lines = append(lines, r.Name)
		}
	}
	output.Emit(os.Stdout, format, lines, rows)
	return 0
}

// countFlags registers the flags shared by tags/properties/property.
func countFlags(fs *flag.FlagSet, unit string) (format, sortBy, prefix *string, counts *bool) {
	format = fs.String("format", "text", "output format: text|json|csv|tsv")
	counts = fs.Bool("counts", false, "show entry count per "+unit)
	sortBy = fs.String("sort", "name", "sort order: name|count")
	prefix = fs.String("prefix", "", "filter to a path prefix")
	return
}

func cmdTags(args []string) int {
	fs := flag.NewFlagSet("tags", flag.ExitOnError)
	format, sortBy, prefix, counts := countFlags(fs, "tag")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}
	return emitCounts(*format, sortedCounts(idx.TagCounts(*prefix), *sortBy), *counts)
}

func cmdProperties(args []string) int {
	fs := flag.NewFlagSet("properties", flag.ExitOnError)
	format, sortBy, prefix, counts := countFlags(fs, "property")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}
	return emitCounts(*format, sortedCounts(idx.PropertyKeyCounts(*prefix), *sortBy), *counts)
}

func cmdProperty(args []string) int {
	fs := flag.NewFlagSet("property", flag.ExitOnError)
	format, sortBy, prefix, counts := countFlags(fs, "value")
	name, ok := parseWithArg(fs, args)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: wiki property <name> [--counts --sort=name|count --prefix]")
		return 2
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}
	return emitCounts(*format, sortedCounts(idx.PropertyValueCounts(name, *prefix), *sortBy), *counts)
}

func cmdCheckboxes(args []string) int {
	fs := flag.NewFlagSet("checkboxes", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
	all := fs.Bool("all", false, "include done tasks")
	done := fs.Bool("done", false, "only done tasks")
	prefix := fs.String("prefix", "", "filter to a path prefix")
	fs.Parse(args)
	// Optional [file]: scope to a single entry's own checklist (its subtasks),
	// with flags allowed on either side of the positional (like read/outline).
	target := ""
	if fs.NArg() >= 1 {
		target = fs.Arg(0)
		fs.Parse(fs.Args()[1:])
		if fs.NArg() != 0 {
			fmt.Fprintln(os.Stderr, "usage: wiki checkboxes [file] [--all --done --prefix]")
			return 2
		}
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	entries := idx.Entries
	if target != "" {
		e, err := idx.Resolve(target)
		if err != nil {
			fmt.Fprintln(os.Stderr, "wiki:", err)
			return 2
		}
		entries = []*index.Entry{e}
	}

	type row struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Done bool   `json:"done"`
		Text string `json:"text"`
	}
	var rows []row
	var lines []string
	for _, e := range entries {
		if target == "" && *prefix != "" && !strings.HasPrefix(strings.TrimPrefix(e.Path, "/"), strings.TrimPrefix(*prefix, "/")) {
			continue
		}
		for _, t := range e.Checkboxes {
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
	return 0
}

func cmdUnresolved(args []string) int {
	fs := flag.NewFlagSet("unresolved", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
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
	return 0
}

func cmdOrphans(args []string) int {
	fs := flag.NewFlagSet("orphans", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
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
	return 0
}

func cmdCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
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

// cmdTidy canonicalizes an already-valid bundle. Bare (no category flag) it
// previews every category and writes nothing; a category flag applies just that
// category. Non-interactive: no prompts, and no --dry-run since the bare command
// is the preview.
func cmdTidy(args []string) int {
	fs := flag.NewFlagSet("tidy", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
	links := fs.Bool("links", false, "normalize links to relative (the canonical on-disk form)")
	slug := fs.Bool("slug", false, "rename spaced filenames to hyphenated slugs (rewriting inbound links)")
	wikilinks := fs.Bool("wikilinks", false, "convert [[wikilinks]] to standard markdown links")
	all := fs.Bool("all", false, "apply every category")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}
	noScope := !*links && !*slug && !*wikilinks && !*all // bare command previews every category, writes nothing
	apply := !noScope
	doLinks := *all || *links || noScope
	doSlug := *all || *slug || noScope
	doWikilinks := *all || *wikilinks || noScope

	var changes []index.Fix
	if doLinks {
		c, err := idx.NormalizeLinks(apply)
		if err != nil {
			fmt.Fprintln(os.Stderr, "wiki:", err)
			return 2
		}
		changes = append(changes, c...)
	}
	if doSlug {
		c, err := idx.Slugify(apply)
		if err != nil {
			fmt.Fprintln(os.Stderr, "wiki:", err)
			return 2
		}
		changes = append(changes, c...)
	}
	if doWikilinks {
		c, err := idx.ConvertWikilinks(apply)
		if err != nil {
			fmt.Fprintln(os.Stderr, "wiki:", err)
			return 2
		}
		changes = append(changes, c...)
	}

	prefix := ""
	if !apply {
		prefix = "would "
	}
	var lines []string
	for _, c := range changes {
		switch c.Field {
		case "rename":
			lines = append(lines, fmt.Sprintf("%srename %q -> %q", prefix, c.From, c.To))
		case "wikilink":
			lines = append(lines, fmt.Sprintf("%sconvert wikilink %s: %q -> %q", prefix, c.Entry, c.From, c.To))
		case "wikilink-skip":
			lines = append(lines, fmt.Sprintf("skip wikilink %s: %q (unresolvable, left as-is)", c.Entry, c.From))
		default:
			lines = append(lines, fmt.Sprintf("%slink %s: %q -> %q", prefix, c.Entry, c.From, c.To))
		}
	}
	switch {
	case len(changes) == 0:
		lines = []string{"ok: nothing to tidy"}
	case apply:
		// already applied; the lines above list what changed
	default:
		lines = append(lines, "run `wiki tidy --links --slug` (or --all) to apply")
	}
	output.Emit(os.Stdout, *format, lines, changes)
	return 0
}

func cmdRead(args []string) int {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
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
		Path string `json:"_path"`
		Type string `json:"type"`
		Body string `json:"body"`
	}{e.Path, e.Type, body}
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	output.Emit(os.Stdout, *format, lines, out)
	return 0
}

func cmdOutline(args []string) int {
	fs := flag.NewFlagSet("outline", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
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
		Path     string          `json:"_path"`
		Headings []parse.Heading `json:"headings"`
	}{e.Path, heads}
	output.Emit(os.Stdout, *format, lines, out)
	return 0
}

func cmdSearch(args []string) int {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
	prefix := fs.String("prefix", "", "filter to a path prefix")
	var where whereFilters
	fs.Var(&where, "where", "filter by frontmatter key=value or key!=value (repeatable = AND; e.g. type=note, status!=done)")
	showLines := fs.Bool("lines", false, "show matching lines instead of entries")
	any := fs.Bool("any", false, "match lines containing any word of the query (default: every word)")
	exact := fs.Bool("exact", false, "match the query verbatim as one phrase (default: every word)")
	query, ok := parseWithArg(fs, args)
	if !ok || strings.TrimSpace(query) == "" {
		fmt.Fprintln(os.Stderr, "usage: wiki search <query> [--any|--exact --where key=value --prefix --lines]  (every word by default; quote a multi-word query)")
		return 2
	}
	if *any && *exact {
		fmt.Fprintln(os.Stderr, "wiki search: --any and --exact are mutually exclusive")
		return 2
	}
	// Default is all-words (AND); --any broadens to OR, --exact narrows to the verbatim phrase.
	mode := index.SearchAll
	switch {
	case *exact:
		mode = index.SearchExact
	case *any:
		mode = index.SearchAny
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	hits := idx.Search(query, *prefix, where, mode)
	var lines []string
	if *showLines {
		for _, h := range hits {
			for _, ln := range h.Lines {
				lines = append(lines, fmt.Sprintf("%s:%d: %s", h.Path, ln.Line, strings.TrimRight(ln.Text, " \t\r")))
			}
		}
	} else {
		for _, h := range hits {
			lines = append(lines, fmt.Sprintf("%-44s %s", h.Path, h.Type))
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

// cmdTable extracts a markdown table from an entry. The dataset convention is one
// table per file, so a lone table needs no flag; with several, it lists them and
// asks for --n (1-based) rather than silently guessing.
func cmdTable(args []string) int {
	fs := flag.NewFlagSet("table", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
	n := fs.Int("n", 0, "which table to extract when a file has several (1-based)")
	target, ok := parseWithArg(fs, args)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: wiki table <file> [--n N] [--format text|json|csv|tsv]")
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
	tables := parse.Tables(body)
	switch {
	case len(tables) == 0:
		fmt.Fprintf(os.Stderr, "wiki: no table in %s\n", e.Path)
		return 1 // no table: a no-match, like search
	case len(tables) > 1 && *n == 0:
		fmt.Fprintf(os.Stderr, "wiki: %s has %d tables; choose one with --n\n", e.Path, len(tables))
		for i, tb := range tables {
			fmt.Fprintf(os.Stderr, "  %d: %s\n", i+1, strings.Join(tb.Header, " | "))
		}
		return 2 // ambiguous request: there are tables, pick one
	}
	sel := *n
	if sel == 0 {
		sel = 1 // default to the lone table
	}
	if sel < 1 {
		fmt.Fprintln(os.Stderr, "wiki: --n must be 1 or greater")
		return 2 // malformed argument
	}
	if sel > len(tables) {
		fmt.Fprintf(os.Stderr, "wiki: %s has no table %d (it has %d)\n", e.Path, sel, len(tables))
		return 1 // no such table, like search with no match
	}
	tb := tables[sel-1]
	if err := output.Table(os.Stdout, *format, tb.Header, tb.Rows); err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
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
	includeFM := fs.Bool("include-frontmatter", false, "also rewrite frontmatter *.md values referencing the moved path (opt-in)")
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
	src, dest, ok := parseWith2Args(fs, args)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: wiki move <src> <dest> [--dry-run] [--include-frontmatter]")
		return 2
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}
	res, err := idx.Move(src, dest, *dryRun, *includeFM)
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
		what := fmt.Sprintf("%d link(s)", rw.Links)
		if rw.FrontmatterRefs > 0 {
			what += fmt.Sprintf(" + %d frontmatter ref(s)", rw.FrontmatterRefs)
		}
		lines = append(lines, fmt.Sprintf("  %s in %s", what, rw.Path))
	}
	output.Emit(os.Stdout, format, lines, res)
}

func cmdLinks(args []string) int {
	fs := flag.NewFlagSet("links", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
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
	return 0
}

func cmdBacklinks(args []string) int {
	fs := flag.NewFlagSet("backlinks", flag.ExitOnError)
	format := fs.String("format", "text", "output format: text|json|csv|tsv")
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
	return 0
}
