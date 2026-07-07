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
wiki init my-wiki && cd my-wiki   # a bare, ready-to-use knowledge base
wiki status                       # what is here
wiki check                        # is it healthy? (links resolve, entries typed)
wiki list                         # every entry
```

`wiki init` scaffolds from a starter `--workflow` (`wiki init -h` lists them): **`default`** (a general knowledge base with a light backlog), **`project-backlog`** (a plain-Markdown kanban tracker), **`org-wiki`** (an organization's internal knowledge base: projects, clients, products, people), and **`product-docs`** (wiki-first product documentation: a linked concept graph plus an optional guides layer). It defaults to `default`, and each is a starting point you then edit to fit.

That `my-wiki` folder is the whole thing. Open it in any editor, commit it to git, point Obsidian at it, point an agent at it. `wiki` simply makes it queryable and keeps it honest. If you use Obsidian, set it to write standard markdown links: Files and links → turn off **Use [[Wikilinks]]**, set **New link format** to *Absolute path in vault*, turn on **Automatically update internal links**. (`wiki` ignores `[[wikilinks]]`.) See the [format spec](https://github.com/agentic-wiki/spec#links).

## What you can ask it

Everything is a fast, scriptable query over the folder.

**Find things**

```sh
wiki list --where type=concept --where tags=crypto --prefix tech/   # by kind, topic, and subtree
wiki list --where type=note --sort=timestamp            # most-recently-changed first (--reverse: oldest first)
wiki search "language model" --lines                    # full-text over frontmatter + body
wiki read /tech/infra/hetzner.md                        # an entry's body, frontmatter stripped
wiki outline /tech/infra/hetzner.md                     # its heading map
wiki table /finance/expenses.md --format csv            # a dataset's table as rows (csv/json), for jq/duckdb
```

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
wiki property status --counts      # how many open vs done, draft vs final
```

**Reshape it safely**

```sh
wiki move /a.md /archive/a.md   # relocate or rename, rewriting every link to it
wiki tidy                       # canonicalize links and filenames (run it bare to preview)
wiki check --fix                # health report, and repair the safe issues
```

Every command speaks `--format text|json|csv|tsv` and returns conventional exit codes (`0` ok; `1` no match (`search`/`table`) or `check` errors; `2` error), so it drops straight into scripts and pipelines:

```sh
wiki search needs-review >/dev/null && echo "matches found" || echo "none"  # search is grep-like: 1 on no match
[ -n "$(wiki unresolved)" ] && echo "broken links exist"                    # listings exit 0 even when empty; test the output
```

## How a base is organized

Three axes, kept separate, are what keep it friction-free:

- **Folder is one stable home.** Group by domain, one place per thing. Moving is `wiki move`, and the links follow.
- **`type` is what an entry is:** `note`, `concept`, `dataset`, `task`, `source`, and so on, one per entry, declared in `wiki.toml`.
- **Tags are everything cross-cutting:** `tech`, `2026`, `needs-review`. If something wants to live in two folders at once, that is a tag.

Entries link with ordinary Markdown links from the base root, `[Income](/finance/income.md)`. Folders can keep an `index.md` listing what is inside, so you (and an agent) read top down and follow links instead of grepping in the dark.

```
my-wiki/
├── wiki.toml             # marks the base, declares your types
├── index.md              # home: links into each area
├── finance/
│   ├── index.md
│   ├── income.md         (type: dataset)
│   └── budget.md         (type: note)
└── tech/infra/
    ├── index.md
    └── hetzner.md        (type: tool)
```

## Run by an agent

The `wiki` CLI is built for an AI agent to operate the whole base, end to end. An agent is probabilistic; your knowledge base must not be. `wiki` is the deterministic engine in between, so every query, move, and check is exact and repeatable: an agent runs your base with the reliability of a database, not the guesswork of a prompt.

You capture a rough thought on the go, and the agent files it as a `draft`. Later it reads your drafts back to you, asks the one or two questions that sharpen each, then files it into a real entry in the right place, linked to what it relates to. It keeps the indexes current, finds the orphans, grooms the links, and tells you what is still unwritten. You think and decide; it does the filing.

The loop, in the tool's own verbs: capture, refine, promote, index, retrieve, maintain. Two skills live in the [skills repo](https://github.com/agentic-wiki/skills):

- **agentic-wiki**, the operating manual for a knowledge base.
- **agentic-backlog**, for a task backlog that is itself a wiki bundle.

Because the agent drives the same commands you do, you are never locked out of your own knowledge, and it is never locked out of helping.

## The stack: three layers

What you have just met is a stack, the modern kind: three layers, and only the bottom one is required.

1. **Markdown.** Plain files in a folder, with a light convention (a `type` in frontmatter, links between entries) we keep OKF-compatible. Complete and navigable on its own, by a human, an LLM, or any tool. This is all you strictly need; everything above is leverage.
2. **Tool.** `wiki` CLI: a neutral, deterministic engine that indexes the folder and answers structured queries like a database. Built for agents to call, runnable by anyone.
3. **Skill.** The manual your agent follows to drive the tool, where the workflow opinion lives, and yours to customize. Maintained in the [skills repo](https://github.com/agentic-wiki/skills).

The Markdown is data at rest and stands alone; the tool is a swappable engine over it; the skills are where the opinion lives. We provide the top two; the foundation is just your own files, and you can keep only those.

## Everything it does

| Command | What it does |
|---|---|
| `init [dir]` | Scaffold a new bundle from a `--workflow` starter (`--force` to write into a non-empty dir) |
| `status` | Bundle counts: entries, links, tags, checkboxes, broken links, orphans |
| `list` | Entries, filtered by `--where key=value` / `--prefix` |
| `search <q>` | Full-text over frontmatter and body (`--lines` for file:line) |
| `read <path>` | An entry's body, frontmatter stripped |
| `outline <path>` | An entry's heading hierarchy |
| `table <path>` | Extract a dataset's markdown table as text/csv/json (`--n` to pick one) |
| `checkboxes` | Open `- [ ]` items, not `type: task` entries (`--all`, `--done`; `[file]` scopes to one entry) |
| `tags` / `properties` / `property <key>` | The base's vocabulary (`--counts`, `--sort=name\|count`) |
| `links <path>` / `backlinks <path>` | Outgoing / incoming links |
| `unresolved` / `orphans` | Broken links / entries with nothing linking in |
| `move <src> <dst>` | Relocate or rename, rewriting every link to it |
| `tidy` | Canonicalize the base: `--links`, `--slug`, `--all` (bare previews) |
| `check` | Health lint (`--fix` repairs the safe issues, e.g. version drift) |
| `version` | Print the version |

File arguments are **bundle paths**, not filesystem paths: a root-absolute `/finance/income.md`, or a bare `income.md` when it is unambiguous. They mean the same thing wherever you run from, and whatever `--root <dir>` points at. `--prefix` takes the same form to scope a listing to a subtree.

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
- **Minimal on purpose:** zero external dependencies, a single static binary, native on macOS, Linux, and Windows. Git is recommended but entirely optional.
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
