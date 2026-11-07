// Package scaffold writes a new agentic-wiki bundle: the minimal starter, or
// one composed from parts (`wiki init --shape`).
//
// The shared files (the AGENTS.md operating manual, the .gitignore) are under
// files/, and each part's files under files/parts/<name>/. One part fills the
// wiki's root; two or more each get their own folder, linked from a generated
// root index.md. AGENTS.md (the shared manual, then a section per part) and
// wiki.toml are built from the parts.
package scaffold

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed files
var files embed.FS

// part is one composable piece of a wiki: its files are under files/parts/<name>/.
type part struct {
	name   string   // the --shape name
	folder string   // where it goes when the wiki has two or more parts
	title  string   // its index.md heading, AGENTS.md section heading and root index.md link
	about  string   // one line: the root index.md link and `init -h`
	types  []string // wiki.toml types
	orphan []string // wiki.toml ignore_orphans, relative to its folder
}

// parts, in the order `init -h` lists them.
var parts = []part{
	{"knowledge", "topics", "Topics", "what we know, filed by topic and linked", []string{"note", "source"}, []string{"inbox/**"}},
	{"kanban", "backlog", "Backlog", "the work, one task per file", []string{"task"}, []string{"**"}},
	{"data", "data", "Data", "records and datasets, with an example of each", []string{"company", "dataset"}, []string{"*/**"}},
	{"specs", "product", "Product", "what the product is, one spec per concept", []string{"spec", "note"}, nil},
}

// Full is every part's name, in order: the shape `init --full` writes.
func Full() []string {
	names := make([]string, len(parts))
	for i, p := range parts {
		names[i] = p.name
	}
	return names
}

// PartsTable is the parts as `init -h` lists them: name, folder, and what each holds.
func PartsTable() string {
	var b strings.Builder
	for _, p := range parts {
		fmt.Fprintf(&b, "  %-11s %-9s %s\n", p.name, p.folder+"/", p.about)
	}
	return b.String()
}

// Write creates a new bundle in dir from the parts named in shape, in that
// order, or the minimal starter when shape is empty. It writes AGENTS.md (the
// shared manual, then a section per part), the .gitignore, and a CLAUDE.md that
// holds `@AGENTS.md`, then each part's files, the root index.md and wiki.toml.
// A part's name that is empty, unknown or repeated is an error naming --shape,
// and a non-empty dir is refused unless force is set (a lone .git directory
// doesn't count); nothing is written then. Returns the slash-separated
// relative paths written.
func Write(dir string, shape []string, force bool) ([]string, error) {
	chosen, err := choose(shape)
	if err != nil {
		return nil, err
	}
	if !force {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, e := range entries {
				if e.Name() != ".git" { // a version-control dir alone is not content
					return nil, fmt.Errorf("target %q is not empty (use --force)", dir)
				}
			}
		}
	}
	var written []string
	emit := func(rel string, data []byte) error {
		dest := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		// Remove what is there first: an earlier init wrote CLAUDE.md as a
		// symlink to AGENTS.md, and writing through it would overwrite AGENTS.md.
		if err := os.Remove(dest); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			return err
		}
		written = append(written, rel)
		return nil
	}

	agents, err := agentsMD(chosen)
	if err != nil {
		return written, err
	}
	gitignore, err := files.ReadFile("files/gitignore")
	if err != nil {
		return written, err
	}
	for _, f := range []struct {
		rel  string
		data []byte
	}{
		{"AGENTS.md", agents},
		{".gitignore", gitignore},
		{"CLAUDE.md", []byte("@AGENTS.md\n")}, // Claude Code reads AGENTS.md through it
	} {
		if err := emit(f.rel, f.data); err != nil {
			return written, err
		}
	}

	const frontmatter = "---\nokf_version: \"0.1\"\n---\n\n"
	for _, p := range chosen {
		root := "files/parts/" + p.name
		err := fs.WalkDir(files, root, func(path string, d fs.DirEntry, err error) error {
			rel := strings.TrimPrefix(path, root+"/")
			if err != nil || d.IsDir() || rel == "AGENTS.md" || rel == "board.toml" { // read by agentsMD and wikiTOML
				return err
			}
			data, err := files.ReadFile(path)
			if err != nil {
				return err
			}
			if rel == "index.md" { // the part's top index.md: its heading is the title
				data = append([]byte("# "+p.title+"\n\n"), data...)
				if len(chosen) == 1 { // it is the wiki's root index.md
					data = append([]byte(frontmatter), data...)
				}
			}
			return emit(prefix(p, len(chosen))+rel, data)
		})
		if err != nil {
			return written, err
		}
	}
	if len(chosen) != 1 {
		var b strings.Builder
		b.WriteString(frontmatter + "# Wiki\n\n")
		for _, p := range chosen {
			fmt.Fprintf(&b, "- [%s](%s/index.md): %s.\n", p.title, p.folder, p.about)
		}
		if len(chosen) == 0 {
			b.WriteString("Link each top-level folder's `index.md` here.\n")
		}
		if err := emit("index.md", []byte(b.String())); err != nil {
			return written, err
		}
	}
	toml, err := wikiTOML(chosen)
	if err != nil {
		return written, err
	}
	if err := emit("wiki.toml", toml); err != nil {
		return written, err
	}
	return written, nil
}

// choose looks up the named parts, in the order given. Its errors name the
// --shape flag, which is where the names come from.
func choose(shape []string) ([]part, error) {
	var chosen []part
	for _, name := range shape {
		i := slices.IndexFunc(parts, func(p part) bool { return p.name == name })
		switch {
		case name == "":
			return nil, errors.New("empty part in --shape")
		case i < 0:
			names := Full()
			list := strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
			return nil, fmt.Errorf("unknown part %q in --shape (use %s)", name, list)
		case slices.ContainsFunc(chosen, func(p part) bool { return p.name == name }):
			return nil, fmt.Errorf("part %q is listed twice in --shape", name)
		}
		chosen = append(chosen, parts[i])
	}
	return chosen, nil
}

// prefix is where a part's files go: its folder when the wiki has two or more
// parts, else the root.
func prefix(p part, n int) string {
	if n < 2 {
		return ""
	}
	return p.folder + "/"
}

// agentsMD is AGENTS.md: the shared manual, then, when there are parts, the
// `## What this wiki holds` section (its intro in parts-intro.md) with each
// part's section under `### <title>` (with its folder, when it has one),
// {dir} replaced.
func agentsMD(chosen []part) ([]byte, error) {
	shared, err := files.ReadFile("files/AGENTS.md")
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.Write(shared)
	if len(chosen) > 0 {
		intro, err := files.ReadFile("files/parts-intro.md")
		if err != nil {
			return nil, err
		}
		b.WriteString("\n")
		b.Write(intro)
	}
	for _, p := range chosen {
		body, err := files.ReadFile("files/parts/" + p.name + "/AGENTS.md")
		if err != nil {
			return nil, err
		}
		dir := prefix(p, len(chosen))
		b.WriteString("\n### " + p.title)
		if dir != "" {
			b.WriteString(" in `" + dir + "`")
		}
		b.WriteString("\n\n" + strings.ReplaceAll(string(body), "{dir}", dir))
	}
	return []byte(b.String()), nil
}

// wikiTOML is wiki.toml: the parts' types and ignore_orphans, each list a
// union in part order, and the kanban part's wikiview board view (its
// board.toml, already commented out). With no parts, types is commented out
// too, so types stay free-form.
func wikiTOML(chosen []part) ([]byte, error) {
	var types, orphans []string
	var board []byte
	for _, p := range chosen {
		for _, t := range p.types {
			if !slices.Contains(types, t) {
				types = append(types, t)
			}
		}
		for _, o := range p.orphan {
			orphans = append(orphans, prefix(p, len(chosen))+o)
		}
		if p.name == "kanban" {
			data, err := files.ReadFile("files/parts/kanban/board.toml")
			if err != nil {
				return nil, err
			}
			board = []byte(strings.ReplaceAll(string(data), "{dir}", strings.TrimSuffix(prefix(p, len(chosen)), "/")))
		}
	}
	var b strings.Builder
	b.WriteString("spec = \"0.1\"\n\n# The entry types this wiki uses: `wiki check` reports any other.\n")
	if len(types) == 0 {
		b.WriteString("# types = [\"note\"]\n")
	} else {
		b.WriteString("types = " + list(types) + "\n")
	}
	b.WriteString("\n# Operating docs, not entries.\nignore = [\"AGENTS.md\", \"CLAUDE.md\"]\n")
	if len(orphans) > 0 {
		if slices.Equal(orphans, []string{"**"}) {
			b.WriteString("\n# Entries nothing has to link to. Here that is everything: tasks are found by\n# query, so `wiki orphans` reports nothing. Narrow it if you add other entries.\n")
		} else {
			b.WriteString("\n# Entries nothing has to link to.\n")
		}
		b.WriteString("ignore_orphans = " + list(orphans) + "\n")
	}
	if board != nil {
		b.WriteString("\n" + string(board))
	}
	return []byte(b.String()), nil
}

// list writes strings as a TOML array.
func list(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
