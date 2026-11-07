package main

import (
	"errors"
	"flag"
	"fmt"
	"maps"
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
	force := fs.Bool("force", false, "write into a directory that isn't empty")
	shape := fs.String("shape", "", "the parts to start with, comma-separated (see above); none gives the minimal starter")
	format := formatFlag(fs)
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "usage: wiki init [dir] [--shape part,part] [--force]\n\n"+
			"Writes a new wiki in dir (default: the current directory).\n\n"+
			"Parts (one fills the wiki; two or more get a folder each):\n%s\n", scaffold.PartsTable())
		fs.PrintDefaults()
	}
	fs.Parse(args)
	if rootDir != "" {
		// --root finds an existing bundle; init makes one, so honoring it would
		// mean guessing, and ignoring it would scaffold into the wrong directory.
		fmt.Fprintln(os.Stderr, "wiki: init takes the directory as its argument: wiki init <dir>")
		return 2
	}
	dir := "."
	if fs.NArg() >= 1 {
		dir = fs.Arg(0)
		fs.Parse(fs.Args()[1:]) // pick up flags that followed the path
	}
	var parts []string
	if *shape != "" {
		for _, p := range strings.Split(*shape, ",") {
			parts = append(parts, strings.TrimSpace(p))
		}
	}
	written, err := scaffold.Write(dir, parts, *force)
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
	}
	out := struct {
		Dir   string   `json:"dir"`
		Files []string `json:"files"`
	}{dir, written}
	lines := []string{"wrote a new wiki in " + dir}
	for _, f := range written {
		lines = append(lines, "  "+f)
	}
	return emit(*format, lines, out)
}

func cmdStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	format := formatFlag(fs)
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
		len(idx.Entries), links, len(idx.TagCounts("", nil)), checkboxes,
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
	return emit(*format, lines, s)
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
	format := formatFlag(fs)
	prefix := fs.String("prefix", "", "filter to a path prefix")
	var where whereFilters
	fs.Var(&where, "where", "filter by frontmatter key=value or key!=value (repeatable = AND; e.g. type=note, status!=done)")
	sortBy := fs.String("sort", "path", "sort order: path|timestamp (timestamp is newest-first)")
	reverse := fs.Bool("reverse", false, "reverse the sort order")
	fields := fieldsFlag(fs, format)
	fs.Parse(args)
	keys, err := fields()
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
	}
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
	return emitEntries(*format, lines, entries, keys)
}

// parseFields reads --fields: keys split on "," with the spaces around each
// trimmed. It returns an error for an empty key, a key named twice, _path
// (always the first column) and a format other than csv or tsv.
func parseFields(s, format string) ([]string, error) {
	if format != "csv" && format != "tsv" {
		return nil, errors.New("--fields needs --format csv or --format tsv")
	}
	keys := strings.Split(s, ",")
	seen := map[string]bool{}
	for i, k := range keys {
		k = strings.TrimSpace(k)
		switch {
		case k == "":
			return nil, fmt.Errorf("--fields has an empty key at position %d (use --fields a,b,c)", i+1)
		case k == "_path":
			return nil, errors.New("--fields can't name _path: it is always the first column")
		case seen[k]:
			return nil, fmt.Errorf("--fields names %q twice", k)
		}
		seen[k] = true
		keys[i] = k
	}
	return keys, nil
}

// fieldsFlag registers --fields, the csv/tsv columns after _path, on fs. After
// fs.Parse, the returned function gives the chosen keys: nil without the flag,
// or the error to report. An explicit --fields "" is an error while no --fields
// means every column, so whether the flag was given matters, not only its value.
func fieldsFlag(fs *flag.FlagSet, format *string) func() ([]string, error) {
	fields := fs.String("fields", "", "csv/tsv columns after _path, in order (e.g. --fields roe,listed)")
	return func() (keys []string, err error) {
		fs.Visit(func(f *flag.Flag) {
			if f.Name == "fields" {
				keys, err = parseFields(*fields, *format)
			}
		})
		return keys, err
	}
}

// emitEntries prints a list of entries, as list and orphans do: text lines as
// given, json the entries with their typed frontmatter, and csv/tsv one row per
// entry with frontmatter columns (entryRows).
func emitEntries(format string, lines []string, entries []*index.Entry, keys []string) int {
	var rows output.Rows
	if output.Tabular(format) {
		rows = entryRows(entries, keys)
	}
	return emitRows(format, lines, entries, rows)
}

// entryRows builds the csv/tsv rows of a list of entries from their typed
// frontmatter:
// _path, then keys in order, or when keys is nil, type and every other key any
// entry uses, in byte order. A key in keys that no entry has is still a column,
// with one warning on stderr per key.
func entryRows(entries []*index.Entry, keys []string) output.Rows {
	fms := make([]map[string]any, len(entries)) // a lookup per entry, so many keys cost their number
	used := map[string]bool{}
	for i, e := range entries {
		fms[i] = map[string]any{}
		for _, p := range e.TypedFrontmatter() {
			fms[i][p.Key] = p.Value
			used[p.Key] = true
		}
	}
	if keys == nil {
		delete(used, "_path") // the entry's path wins over a frontmatter key, as in json
		delete(used, "type")
		keys = append([]string{"type"}, slices.Sorted(maps.Keys(used))...)
	} else if len(entries) > 0 {
		for _, k := range keys {
			if !used[k] {
				fmt.Fprintf(os.Stderr, "wiki: warning: --fields: no listed entry has %q\n", k)
			}
		}
	}
	rows := output.Rows{Header: append([]string{"_path"}, keys...)}
	for i, e := range entries {
		row := []any{e.Path}
		for _, k := range keys {
			v, ok := fms[i][k]
			if !ok {
				v = output.Missing // a key the entry doesn't have: left out of its jsonl row
			}
			row = append(row, v)
		}
		rows.Values = append(rows.Values, row)
	}
	return rows
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
	return emit(format, lines, rows)
}

// countFlags registers the flags shared by tags/properties/property.
func countFlags(fs *flag.FlagSet, unit string) (format, sortBy, prefix *string, counts *bool, where *whereFilters) {
	format = formatFlag(fs)
	counts = fs.Bool("counts", false, "show entry count per "+unit)
	sortBy = fs.String("sort", "name", "sort order: name|count")
	prefix = fs.String("prefix", "", "filter to a path prefix")
	// The vocabulary commands report what a set of entries uses, so they narrow
	// that set the same two ways everything else does: --prefix for where,
	// --where for what.
	where = &whereFilters{}
	fs.Var(where, "where", "filter entries by frontmatter: key=value or key!=value (repeatable)")
	return
}

func cmdTags(args []string) int {
	fs := flag.NewFlagSet("tags", flag.ExitOnError)
	format, sortBy, prefix, counts, where := countFlags(fs, "tag")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}
	return emitCounts(*format, sortedCounts(idx.TagCounts(*prefix, *where), *sortBy), *counts)
}

func cmdProperties(args []string) int {
	fs := flag.NewFlagSet("properties", flag.ExitOnError)
	format, sortBy, prefix, counts, where := countFlags(fs, "property")
	fs.Parse(args)
	idx, code := loadIndex()
	if code != 0 {
		return code
	}
	return emitCounts(*format, sortedCounts(idx.PropertyKeyCounts(*prefix, *where), *sortBy), *counts)
}

func cmdProperty(args []string) int {
	fs := flag.NewFlagSet("property", flag.ExitOnError)
	format, sortBy, prefix, counts, where := countFlags(fs, "value")
	name, ok := parseWithArg(fs, args)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: wiki property <name> [--counts --sort=name|count --prefix]")
		return 2
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}
	return emitCounts(*format, sortedCounts(idx.PropertyValueCounts(name, *prefix, *where), *sortBy), *counts)
}

func cmdCheckboxes(args []string) int {
	fs := flag.NewFlagSet("checkboxes", flag.ExitOnError)
	format := formatFlag(fs)
	all := fs.Bool("all", false, "include done tasks")
	done := fs.Bool("done", false, "only done tasks")
	prefix := fs.String("prefix", "", "filter to a path prefix")
	where := &whereFilters{}
	fs.Var(where, "where", "filter entries by frontmatter: key=value or key!=value (repeatable)")
	fs.Parse(args)
	// Optional [file]: scope to a single entry's own checklist (its subtasks),
	// with flags allowed on either side of the positional (like read/outline).
	target := ""
	if fs.NArg() >= 1 {
		target = fs.Arg(0)
		fs.Parse(fs.Args()[1:])
		if fs.NArg() != 0 {
			fmt.Fprintln(os.Stderr, "usage: wiki checkboxes [file] [--all --done --prefix --where key=value]")
			return 2
		}
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	// A named [file] is explicit, so the filters do not also apply to it.
	entries := idx.Filter(*prefix, *where)
	if target != "" {
		e, err := idx.Resolve(target)
		if err != nil {
			fmt.Fprintln(os.Stderr, "wiki:", err)
			return 2
		}
		entries = []*index.Entry{e}
	}

	// `entry`, matching check's rows: both describe an entry rather than merging
	// its frontmatter, so neither needs the reserved `_path`, and they should not
	// disagree on what to call the same thing.
	type row struct {
		Entry string `json:"entry"`
		Line  int    `json:"line"`
		Done  bool   `json:"done"`
		Text  string `json:"text"`
	}
	var rows []row
	var lines []string
	for _, e := range entries {
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
	return emit(*format, lines, rows)
}

func cmdUnresolved(args []string) int {
	fs := flag.NewFlagSet("unresolved", flag.ExitOnError)
	format := formatFlag(fs)
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
	return emit(*format, lines, broken)
}

func cmdOrphans(args []string) int {
	fs := flag.NewFlagSet("orphans", flag.ExitOnError)
	format := formatFlag(fs)
	fields := fieldsFlag(fs, format)
	fs.Parse(args)
	keys, err := fields()
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
	}
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
	return emitEntries(*format, lines, orph, keys)
}

func cmdCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	format := formatFlag(fs)
	fix := fs.Bool("fix", false, "apply safe repairs, e.g. sync okf_version (writes files)")
	fs.Parse(args)
	idx, code := buildIndex() // check reports unreadable headers as errors: no warnings
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
			if rebuilt, c := buildIndex(); c == 0 {
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
		// Rows: the fixes, then the issues, in one table. A fix has level
		// "fixed" and no msg; an issue has no field, from or to (Missing).
		rows := output.Rows{Header: []string{"level", "entry", "field", "from", "to", "msg"}}
		for _, fx := range fixes {
			rows.Values = append(rows.Values, []any{"fixed", fx.Entry, fx.Field, fx.From, fx.To, output.Missing})
		}
		for _, is := range issues {
			rows.Values = append(rows.Values, []any{is.Level, is.Entry, output.Missing, output.Missing, output.Missing, is.Msg})
		}
		code = emitRows(*format, lines, struct {
			Fixed  []index.Fix   `json:"fixed"`
			Issues []index.Issue `json:"issues"`
		}{fixes, issues}, rows)
	} else {
		code = emit(*format, lines, issues)
	}
	if code != 0 {
		return code
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
	format := formatFlag(fs)
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
	return emit(*format, lines, changes)
}

func cmdRead(args []string) int {
	fs := flag.NewFlagSet("read", flag.ExitOnError)
	format := formatFlag(fs)
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
	return emit(*format, lines, out)
}

func cmdOutline(args []string) int {
	fs := flag.NewFlagSet("outline", flag.ExitOnError)
	format := formatFlag(fs)
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
	rows := output.Rows{Header: []string{"_path", "level", "text", "line"}}
	for _, h := range heads {
		rows.Values = append(rows.Values, []any{e.Path, h.Level, h.Text, h.Line})
	}
	return emitRows(*format, lines, out, rows)
}

func cmdSearch(args []string) int {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	format := formatFlag(fs)
	prefix := fs.String("prefix", "", "filter to a path prefix")
	var where whereFilters
	fs.Var(&where, "where", "filter by frontmatter key=value or key!=value (repeatable = AND; e.g. type=note, status!=done)")
	showLines := fs.Bool("lines", false, "show matching lines instead of entries")
	anyWord := fs.Bool("any", false, "match lines containing any word of the query (default: every word)")
	exact := fs.Bool("exact", false, "match the query verbatim as one phrase (default: every word)")
	query, ok := parseWithArg(fs, args)
	if !ok || strings.TrimSpace(query) == "" {
		fmt.Fprintln(os.Stderr, "usage: wiki search <query> [--any|--exact --where key=value --prefix --lines]  (every word by default; quote a multi-word query)")
		return 2
	}
	if *anyWord && *exact {
		fmt.Fprintln(os.Stderr, "wiki search: --any and --exact are mutually exclusive")
		return 2
	}
	// Default is all-words (AND); --any broadens to OR, --exact narrows to the verbatim phrase.
	mode := index.SearchAll
	switch {
	case *exact:
		mode = index.SearchExact
	case *anyWord:
		mode = index.SearchAny
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	hits := idx.Search(query, *prefix, where, mode)
	var lines []string
	rows := output.Rows{Header: []string{"_path", "type", "matches"}}
	if *showLines {
		// One row per matching line, so a script reads lines, not lists of them.
		rows.Header = []string{"_path", "type", "line", "text"}
		for _, h := range hits {
			for _, ln := range h.Lines {
				text := strings.TrimRight(ln.Text, " \t\r")
				lines = append(lines, fmt.Sprintf("%s:%d: %s", h.Path, ln.Line, text))
				rows.Values = append(rows.Values, []any{h.Path, h.Type, ln.Line, text})
			}
		}
	} else {
		for i, h := range hits {
			lines = append(lines, fmt.Sprintf("%-44s %s", h.Path, h.Type))
			rows.Values = append(rows.Values, []any{h.Path, h.Type, h.Matches})
			hits[i].Lines = nil // json carries per-line detail only with --lines
		}
	}
	if code := emitRows(*format, lines, hits, rows); code != 0 {
		return code
	}
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
	format := formatFlag(fs)
	n := fs.Int("n", 0, "which table to extract when a file has several (1-based)")
	target, ok := parseWithArg(fs, args)
	if !ok {
		fmt.Fprintln(os.Stderr, "usage: wiki table <file> [--n N] [--format text|json|jsonl|csv|tsv]")
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
	if err := output.Table(os.Stdout, *format, tb.Header, tb.Rows, tb.TypedRows()); err != nil {
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
	format := formatFlag(fs)
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
	if res == nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
	}
	for _, w := range res.Warnings {
		fmt.Fprintln(os.Stderr, "wiki: warning:", w)
	}
	code = emitMove(res, *dryRun, *format)
	if err != nil {
		// The file moved, but some files weren't rewritten: one line each.
		errs := []error{err}
		if joined, ok := err.(interface{ Unwrap() []error }); ok {
			errs = joined.Unwrap()
		}
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "wiki:", e)
		}
		return 2
	}
	return code
}

func emitMove(res *index.MoveResult, dryRun bool, format string) int {
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
	rows, _ := output.RowsOf(res.Rewrites) // a slice of structs always has rows
	return emitRows(format, lines, res, rows)
}

func cmdLinks(args []string) int {
	fs := flag.NewFlagSet("links", flag.ExitOnError)
	format := formatFlag(fs)
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
	// Unique targets: this view answers "what does this page point to", and it
	// prints a bare path, so the same target twice would read as a bug rather
	// than as two mentions. The engine returns every occurrence; deciding to
	// collapse them is presentation, which is this layer's job.
	// (`backlinks` prints file:line, so it shows every occurrence.)
	seen := map[string]bool{}
	var refs []index.LinkRef
	var lines []string
	for _, r := range idx.Links(e.Path) {
		if seen[r.To] {
			continue
		}
		seen[r.To] = true
		refs = append(refs, r)
		lines = append(lines, r.To)
	}
	return emit(*format, lines, refs)
}

func cmdBacklinks(args []string) int {
	fs := flag.NewFlagSet("backlinks", flag.ExitOnError)
	format := formatFlag(fs)
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
	return emit(*format, lines, refs)
}

// cmdDump prints the entries as one markdown stream (see index.Dump), in reading
// order (index.ReadingOrder). The filters decide inclusion only: the order comes
// from the whole bundle, so an index.md filtered out of the dump still orders
// its folder. Out-of-bundle links are left as written and listed on stderr once
// the dump is out, since nothing at a published URL can resolve them. A log.md
// is history, not content, so it is left out unless --logs, whatever the filters
// match; stderr then names every link to one and says when only logs matched,
// so neither a dangling link nor an empty dump goes unexplained.
func cmdDump(args []string) int {
	fs := flag.NewFlagSet("dump", flag.ExitOnError)
	prefix := fs.String("prefix", "", "filter to a path prefix")
	where := &whereFilters{}
	fs.Var(where, "where", "filter entries by frontmatter: key=value or key!=value (repeatable)")
	withFrontmatter := fs.Bool("frontmatter", false, "keep each entry's frontmatter")
	logs := fs.Bool("logs", false, "include log.md entries")
	fs.Parse(args)
	if fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "usage: wiki dump [--prefix <path>] [--where key=value] [--frontmatter] [--logs]")
		return 2
	}
	idx, code := loadIndex()
	if code != 0 {
		return code
	}

	keep := map[*index.Entry]bool{}
	for _, e := range idx.Filter(*prefix, *where) {
		keep[e] = true
	}
	// droppedLogs holds the logs the filters matched but --logs is needed for, so
	// the warnings below name only what --logs would actually bring back.
	var entries []*index.Entry
	droppedLogs := map[string]bool{}
	for _, e := range idx.ReadingOrder() {
		switch {
		case !keep[e]:
		case e.IsLog() && !*logs:
			droppedLogs[e.Path] = true
		default:
			entries = append(entries, e)
		}
	}
	if err := idx.Dump(os.Stdout, entries, *withFrontmatter); err != nil {
		fmt.Fprintln(os.Stderr, "wiki:", err)
		return 2
	}
	for _, e := range entries {
		for _, r := range idx.OutOfBundle(e.Path) {
			fmt.Fprintf(os.Stderr, "wiki: warning: %s:%d out-of-bundle link -> %s\n", r.From, r.Line, r.To)
		}
		// The link is still made root-absolute, so it names a section the dump lacks.
		for _, l := range e.Links {
			if droppedLogs[l.Target] {
				fmt.Fprintf(os.Stderr, "wiki: warning: %s:%d link to a left-out log -> %s (--logs includes it)\n", e.Path, l.Line, l.Target)
			}
		}
	}
	if len(entries) == 0 && len(droppedLogs) > 0 {
		fmt.Fprintf(os.Stderr, "wiki: warning: only log.md entries matched (%d), left out without --logs\n", len(droppedLogs))
	}
	return 0
}
