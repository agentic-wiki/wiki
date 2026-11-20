# Agentic Wiki CLI

> Your agent can finally own your whole knowledge base.

Karpathy's LLM-wiki note got a lot of us excited: a knowledge base an agent could actually live in. The idea immediately resonated; however, a full implementation never shipped. A lone skill file won't fly long.

It takes a full stack: plain **Markdown** for your notes, docs, datasets, and tasks; a **tool** that reads the folder as a queryable, linked graph; and the **skills** that teach an agent to drive it. **Agentic wiki** provides the latter two, the tool and the skills. Your agent owns the upkeep (capturing, filing, linking, refining, grooming) while you own the files.

Underneath, it simply answers like a database. `wiki` CLI reads your markdown folder and tells you what links to what, what has no home, every entry of a given kind and tag, and every open task, then refactors the whole base without breaking a single link. Built for your agent to call, and just as runnable by you.

There's pleasantly little to it: no app to launch, no database, no daemon, no sync service, nothing that can lock you in. Just one static binary handling plain files that are yours, and that you will still be able to open in thirty years.

```sh
wiki list --where type=dataset --where tags=finance  # query entries by type and topic
wiki search "quarterly revenue"             # full-text search
wiki backlinks /finance/income.md           # everything that points here
wiki checkboxes                             # every open checkbox, gathered
```

## Why this exists

Markdown in a folder is the only knowledge store that is human-readable, portable, LLM-native, git-versioned, and standard all at once. Your editor, Obsidian, this CLI, and any language model open the very same files, and nothing is trapped inside someone's product.

The catch is that a plain text folder is dumb: `grep` cannot tell you what links to a page, which notes have no home, or move a file and fix the links behind it. `wiki` adds exactly that structure while leaving the files as ordinary Markdown, so you get the freedom of plain text and the power of a database at the same time. Markdown files stay the single source of truth; the index is derived and disposable.

## Install

Homebrew, on macOS or Linux:

```sh
brew install agentic-wiki/tap/wiki
```

Or grab a binary directly:

```sh
# macOS
curl -L https://github.com/agentic-wiki/wiki/releases/latest/download/wiki_darwin_arm64.tar.gz | tar xz
sudo mv wiki /usr/local/bin/

# Linux or WSL (amd64)
curl -L https://github.com/agentic-wiki/wiki/releases/latest/download/wiki_linux_amd64.tar.gz | tar xz
sudo mv wiki /usr/local/bin/
```

All other platforms (windows/amd64, darwin/amd64, linux/arm64, windows/arm64) are on the [releases page](https://github.com/agentic-wiki/wiki/releases).

With a Go toolchain on any OS: `go install github.com/agentic-wiki/wiki/cmd/wiki@latest`.

## Sixty seconds

```sh
wiki init --shape specs,kanban  # a product map and its backlog
wiki status                     # what is here
wiki check                      # is it healthy? (links resolve, entries typed)
wiki list                       # every entry
```

`wiki init` alone writes the minimal starter: `index.md`, `wiki.toml` and `AGENTS.md`, with `CLAUDE.md` and `.gitignore`. Add `--shape` to start from parts, or `--full` for all of them: see [Starter shapes](#starter-shapes).

That's the whole thing: a folder of plain Markdown. Open it in any editor, commit it to git, point Obsidian at it, point an agent at it. `wiki` simply makes it queryable and keeps it honest.

> [!NOTE]
> If you use Obsidian, set it to write standard markdown links: Files and links → turn off **Use [[Wikilinks]]**, set **New link format** to *Relative path to file* (the canonical on-disk form, so links navigate in any renderer), turn on **Automatically update internal links**. (`wiki` recognizes `[[wikilinks]]` for compatibility but flags them in `wiki check`, and `wiki tidy --wikilinks` converts them to standard links). See the [format spec](https://github.com/agentic-wiki/spec#links).

## What you can ask it

Everything is a fast, scriptable query over the folder.

**Find things**

```sh
wiki list --where type=concept --where tags=crypto --prefix tech/   # by kind, topic, and subtree
wiki list --where type=task --where status!=done        # key!=value negates (here: still-open work)
wiki list --where type=note --sort=timestamp            # most-recently-changed first (--reverse: oldest first)
wiki search "language model" --lines                    # every word by default (--any: broaden to any word; --exact: verbatim phrase)
wiki read /tech/infra/hetzner.md                        # an entry's body, frontmatter stripped
wiki outline /tech/infra/hetzner.md                     # its headings
wiki table /finance/expenses.md --format csv            # a dataset's table as rows (csv/json), for jq/duckdb
wiki list --where type=company --format csv --fields ticker,roe,listed   # chosen frontmatter keys as columns
```

`list --format json` keeps YAML types: an unquoted number, `true`/`false` or `null` is a JSON value, and a quoted value is a string. DuckDB can then query it without casts:

```sh
wiki list --where type=company --format json > companies.json
duckdb -c "select ticker from read_json('companies.json') where roe >= 0.2"
```

Two filters narrow any set of entries: **`--prefix` for where, `--where` for what.** Both work on `list`, `search`, `dump`, `checkboxes`, `tags`, `properties`, and `property` — so a folder that mixes kinds can still be asked a narrow question. `--where key=value` compares text: a list matches when any item equals the value, and a list or map item, or a whole map, compares as its one line of YAML (copy it from `property` or a csv cell, since that line fixes the spelling). `--where key=` matches a key with no value: absent, null, `""`, `[]`, `{}`, or a list of empty items.

**Follow the graph** (the part `grep` cannot do)

```sh
wiki links /index.md               # what this page points to
wiki backlinks /finance/income.md  # what points back at it
wiki orphans                       # entries nothing links to (lost knowledge)
wiki unresolved                    # links to pages not yet written
```

That last one is quietly the favorite: a broken link is not an error here, it is a piece of knowledge you have promised yourself and not written yet. `wiki unresolved` is a to-write list that keeps itself.

**See your work and your vocabulary**

```sh
wiki checkboxes                    # every open - [ ] checkbox, across the whole base
wiki list --where type=task        # list task entries (detailed entries)
wiki tags --counts --sort=count    # what you write about most
wiki property status --counts      # how many in each status: todo vs done, planned vs built
wiki property status --counts --where type=task   # …only the tasks', if the folder mixes kinds
```

**Reshape it safely**

```sh
wiki move /a.md /archive/a.md   # relocate or rename, rewriting every link to it
wiki tidy                       # canonicalize links and filenames (run it bare to preview)
wiki check --fix                # health report, and repair the safe issues
```

**Publish it whole**

```sh
wiki dump > llms-full.txt                   # every entry as one markdown file: the whole base at one URL, for agents
wiki dump --prefix docs/ --frontmatter      # a subtree only, keeping each entry's frontmatter
wiki dump --logs                            # log.md files too: they are left out by default
```

Entries come in the order the indexes give them (each folder's `index.md`, then what it links, then the rest by path), each headed by `<!-- /its/path.md -->`. Every `log.md` is left out unless `--logs`: it is the history of a folder, not its content (links to one, now pointing at nothing, are warned about on stderr). Links are made root-absolute so they still point somewhere in one file. Your files are never touched, and the same base always dumps the same bytes.

Every command except `dump` (plain markdown) speaks `--format text|json|jsonl|csv|tsv` and returns conventional exit codes (`0` ok; `1` no match (`search`/`table`) or `check` errors; `2` error), so it drops straight into scripts and pipelines:

```sh
wiki search "needs review" >/dev/null && echo "matches found" || echo "none"  # search is grep-like: 1 on no match
[ -n "$(wiki unresolved)" ] && echo "broken links exist"                    # listings exit 0 even when empty; test the output
```

In `jsonl`, `csv` and `tsv`, every command gives rows: a list result is one row per item, and any other result is a single row. jsonl holds the same rows as csv, not the json elements: one compact JSON object per line, with the row's own keys and its values in their JSON types, and nothing at all for an empty result. csv and tsv always start with a header; a list or map in a cell is one line of YAML in flow style (`[a, b]`, `{income: 2025-12-31}`, with strings another YAML reader would type, such as `yes` or `"10"`, quoted), and a value with no value (absent, null, `""`, `[]`, `{}`, or a list of empty items) is an empty cell. JSON appears only in `json` and `jsonl`. `text` and `json` are unchanged.

| Command | One row per | Columns |
| --- | --- | --- |
| `list`, `ls`, `orphans` | entry | `_path,type`, then every other frontmatter key in use, or `_path` and the `--fields` you name |
| `search` | hit | `_path,type,matches` |
| `search --lines` | matching line | `_path,type,line,text` |
| `checkboxes` | item | `entry,line,done,text` |
| `tags`, `properties`, `property` | name | `name,count` |
| `unresolved` | broken link | `from,to,line` |
| `links`, `backlinks` | link | `from,to,text,line` |
| `table` | table row | the table's header; cells as written (json and jsonl type them: `12.5` is a number, an empty cell or `~` is null) |
| `tidy` | change | `entry,field,from,to` |
| `check` | issue | `level,entry,msg` |
| `check --fix` | fix, then issue | `level,entry,field,from,to,msg` (a fix has level `fixed` and no `msg`) |
| `move`, `mv` | rewritten file | `_path,links,frontmatter_refs` |
| `outline` | heading | `_path,level,text,line,id` |
| `status` | (one row) | `dir,spec,entries,links,tags,checkboxes,broken,orphans` |
| `init` | (one row) | `dir,files` (`files` is a JSON list) |
| `read` | (one row) | `_path,type,body` (`body` is one cell over several lines) |

## How a base is organized

Three axes, kept separate, are what keep it friction-free:

- **Folder is one stable home.** Group by domain, one place per thing. Moving is `wiki move`, and the links follow.
- **`type` is what an entry is:** `note`, `concept`, `dataset`, `task`, `source`, and so on, one per entry, declared in `wiki.toml`.
- **Tags are everything cross-cutting:** `tech`, `2026`, `needs-review`. If something wants to live in two folders at once, that is a tag.

Frontmatter is YAML: values can be text, numbers, `true`/`false`, null, lists and nested maps. `--format json` gives lists and maps as JSON, and it is the exact view. Everywhere else, a list or map is one line of YAML in flow style (`[a, b]`, `{k: v}`), and types are lost: the text `"{a: 1}"` and the map `{a: 1}` both read `{a: 1}` there. `wiki check` reports what wiki can't read or edit safely.

**Limits.** A header can be up to 1 MiB, with any number of values written in it: an index of thousands of records reads. What aliases (`&a` ... `*a`) produce is limited to 10,000 values and 1 MiB of text, each use counted again, so a file can't expand without bound. Beyond these limits a header is refused: `check` reports it, and other commands warn and read the file without its frontmatter. Known limit: a 1 MiB header of about 500,000 tiny values takes about 25 MB of memory once read, for as long as it is held (wikiview's index holds every file).

Entries link with relative Markdown links, `[Income](../finance/income.md)`, so they work in any Markdown viewer. A link from the root (`/finance/income.md`) resolves too, and `wiki tidy --links` rewrites it relative. A link's path is percent-decoded once and its `?query` dropped when it is read, so `my%20file.md` names `my file.md`; `move`, `tidy` and `dump` write a path back the same way, and a link starting `//` names another site, never an entry. Go programs get the same rule from the library: `index.ResolveDest`, `Entry.FrontmatterRefs` and `Entry.Title`. Folders can keep an `index.md` listing what is inside, so you (and an agent) read top down and follow links instead of grepping in the dark.

```
my-wiki/
├── wiki.toml             # marks the base, bundle settings
├── index.md              # home: links into each area
├── finance/
│   ├── index.md
│   ├── income.md         (type: dataset)
│   └── budget.md         (type: note)
└── tech/infra/
    ├── index.md
    └── wiki-cli.md        (type: tool)
```

## Starter shapes

With no `--shape`, `wiki init` writes the minimal starter: `index.md`, `wiki.toml` and `AGENTS.md`, with `CLAUDE.md` and `.gitignore`. `AGENTS.md` is the rules and commands an agent works by, and `CLAUDE.md` holds one line, `@AGENTS.md`, so Claude Code reads the same file.

`wiki init --shape a,b` adds parts. One part fills the wiki's root; two or more each get their own folder, linked from the root `index.md`. Each part adds its section to `AGENTS.md` and its types to `wiki.toml`. `wiki init --full` writes every part.

| Part | Folder | What it holds |
|---|---|---|
| `knowledge` | `topics/` | what we know, filed by topic: `people/`, `processes/` and `decisions/` to start, and an `inbox/` |
| `kanban` | `backlog/` | the work, one task per file |
| `data` | `data/` | one example of each form of data: records in `companies/`, a dataset in `expenses.md` |
| `specs` | `product/` | what the product is, one spec per concept |

Each part's section in `AGENTS.md` and its lines in `wiki.toml` are yours to edit.

## Run by an agent

The `wiki` CLI is built for an AI agent to operate the whole base, end to end. An agent is probabilistic; your knowledge base must not be. `wiki` is the deterministic engine in between, so every query, move, and check is exact and repeatable: an agent runs your base with the reliability of a database, not the guesswork of a prompt.

You capture a rough thought on the go, and the agent puts it in the inbox. Later it reads the inbox back to you, asks the one or two questions that sharpen each thought, then files it as a real entry in the right place, linked to what it relates to. It keeps the indexes current, finds the orphans, grooms the links, and tells you what is still unwritten. You think and decide; it does the filing.

The loop, in the tool's own verbs: capture, refine, promote, index, retrieve, maintain. Two skills live in the [skills repo](https://github.com/agentic-wiki/skills):

- **agentic-wiki**, the operating manual for a knowledge base.
- **agentic-backlog**, for a task backlog that is itself a wiki bundle.

Because the agent drives the same commands you do, you are never locked out of your own knowledge, and it is never locked out of helping.

## The stack: three layers

What you have just met is a stack, the modern kind: three layers, and only the bottom one is required.

1. **Markdown.** Plain files in a folder, with a light convention (a `type` in frontmatter, links between entries) we keep OKF-compatible. Complete and navigable on its own, by a human, an LLM, or any tool. This is all you strictly need; everything above is leverage.
2. **Tool.** `wiki` CLI: a neutral, deterministic engine that indexes the folder and answers structured queries like a database. Built for agents to call, runnable by anyone. Its packages (`parse`, `index`) are also a Go library, which wikiview imports: a heading's anchor (`Heading.ID`), why a header wasn't read (`Entry.FrontmatterErr`) and a list's items (`Entry.FieldList`, a new slice without empty items) come with the same rules the CLI uses. `index.Read` reads one file's text by those rules without a bundle, for a file that isn't in one.
3. **Skill.** The manual your agent follows to drive the tool, where the workflow opinion lives, and yours to customize. Maintained in the [skills repo](https://github.com/agentic-wiki/skills).

The Markdown is data at rest and stands alone; the tool is a swappable engine over it; the skills are where the opinion lives. We provide the top two; the foundation is just your own files, and you can keep only those.

## Everything it does

| Command | What it does |
|---|---|
| `init [dir]` | Write a new wiki: the minimal starter, `--shape` parts or `--full` (`--force` to write into a non-empty dir) |
| `status` | Bundle counts: entries, links, tags, checkboxes, broken links, orphans |
| `list` | Entries, filtered by `--where key=value` (or `key!=value`) / `--prefix` |
| `search <q>` | Full-text over frontmatter and body; every word by default (`--any` any word, `--exact` verbatim phrase, `--lines` for file:line) |
| `read <path>` | An entry's body, frontmatter stripped |
| `dump` | Every entry as one markdown stream in index order, links made root-absolute, e.g. `llms-full.txt` (`--frontmatter` keeps frontmatter; `--logs` includes `log.md` files, left out by default; out-of-bundle links are warned about on stderr) |
| `outline <path>` | An entry's heading hierarchy |
| `table <path>` | Extract a dataset's markdown table as text/csv/json (`--n` to pick one) |
| `checkboxes` | Open `- [ ]` items, not `type: task` entries (`--all`, `--done`; `[file]` scopes to one entry) |
| `tags` / `properties` / `property <key>` | The base's vocabulary (`--counts`, `--sort=name\|count`) |
| `links <path>` / `backlinks <path>` | Outgoing / incoming links |
| `unresolved` / `orphans` | Broken links / entries with nothing linking in |
| `move <src> <dst>` | Relocate or rename, rewriting every link to it (`--include-frontmatter` also rewrites matching frontmatter values) |
| `tidy` | Canonicalize the base: `--links`, `--slug`, `--wikilinks`, `--all` (bare previews) |
| `check` | Health lint, including YAML syntax, duplicate frontmatter keys and the shape of values (`--fix` repairs safe issues, e.g. version drift) |
| `version` | Print the version |

File arguments are **bundle paths**, not filesystem paths: a root-absolute `/finance/income.md`, or a bare `income.md` when it is unambiguous. They mean the same thing wherever you run from, and whatever `--root <dir>` points at (a global flag, accepted before or after the command: `wiki --root kb dump` and `wiki dump --root kb` are the same). `--prefix` takes the same form to scope a listing to a subtree.

Two everyday commands have short aliases: `wiki ls` for `list`, and `wiki mv` for `move`.

## The backlog itself is a wiki

There is no issue tracker and no "TASKS.md" here: this repo's own backlog lives in [`backlog/`](backlog/index.md), and it is itself an agentic-wiki bundle, managed by `wiki`. The tool runs on its own to-do list, with the same commands as above:

```sh
cd backlog
wiki checkboxes                      # what is left to build
wiki list --where type=task --where tags=feature  # new features
wiki check                           # the backlog stays conformant
```

(The cross-repo roadmap, spanning the spec and the skills, lives in the [spec repo](https://github.com/agentic-wiki/spec).)

## Design

- **Standalone first:** agents call `wiki` directly, no server in the way.
- **Minimal on purpose:** two focused dependencies (TOML configuration parsing and YAML frontmatter validation), a single static binary, native on macOS, Linux, and Windows. Git is recommended but entirely optional.
- **Files are truth:** the index is derived from disk and fully disposable.

## Develop

Requires Go 1.24+ and [just](https://github.com/casey/just).

```sh
just            # list all recipes
just check      # vet + lint + test
just test-all   # unit + smoke
just smoke      # end-to-end smoke test against a temp bundle
just build      # build the binary to ./bin/wiki
```

---

Built on Google's [Open Knowledge Format](https://cloud.google.com/blog/products/data-analytics/how-the-open-knowledge-format-can-improve-data-sharing), delivering the LLM-wiki idea Karpathy sketched.
