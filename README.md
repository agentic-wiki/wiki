# Agentic Wiki CLI

> A knowledge base that is just a folder of Markdown, and answers like a database.

Your notes, documents, datasets, and to-dos live as plain text files in one folder. `wiki` reads that folder as a structured, linked graph: it answers questions by type, tag, and topic, follows links in both directions, surfaces what is broken or orphaned, gathers every task, and refactors the whole thing without breaking a single link.

Built for your AI agent to operate from end to end, while every command is just as runnable by you.

No app. No database. No daemon. No sync service. No lock-in. One static binary, reading the files you will own and be able to open for the rest of your life.

```sh
wiki list --type dataset --tag finance      # query by kind and topic
wiki search "quarterly revenue"             # full-text search
wiki backlinks /finance/income.md           # everything that points here
wiki tasks                                  # every open checkbox, gathered
```

## Why this exists

Markdown in a folder is the only way to keep knowledge that is, at once, readable by a human, native to a language model, diffable, versioned by git, portable, self-hostable, and standard. Obsidian, your editor, this CLI, and any LLM all open the very same files. Nothing is trapped inside someone's product.

The catch has always been that a folder of files is dumb. `grep` cannot tell you what links to a page, which notes have no home, or every entry of a given kind carrying a given tag. It cannot move a file and fix the links behind it. That missing structure is exactly what a knowledge base needs, and exactly what `wiki` computes, while leaving the files as ordinary Markdown.

So you get both at the same time: the freedom of plain text, and the structure of a database. The files are the single source of truth; the index `wiki` builds from them is derived and disposable. Uninstall the tool tomorrow and you still hold a clean, navigable, complete knowledge base.

## Install

```sh
# macOS
curl -L https://github.com/agentic-wiki/wiki/releases/latest/download/wiki_darwin_arm64.tar.gz | tar xz
sudo mv wiki /usr/local/bin/

# Linux or WSL (amd64)
curl -L https://github.com/agentic-wiki/wiki/releases/latest/download/wiki_linux_amd64.tar.gz | tar xz
sudo mv wiki /usr/local/bin/
```

```powershell
# Windows (amd64), PowerShell
irm https://github.com/agentic-wiki/wiki/releases/latest/download/wiki_windows_amd64.zip -OutFile wiki.zip
Expand-Archive wiki.zip .; # then put wiki.exe on your PATH
```

Other platforms (darwin/amd64, linux/arm64, windows/arm64) are on the [releases page](https://github.com/agentic-wiki/wiki/releases). With a Go toolchain on any OS: `go install github.com/agentic-wiki/wiki/cmd/wiki@latest`.

## Sixty seconds

```sh
wiki init my-wiki && cd my-wiki   # a bare, ready-to-use knowledge base
wiki status                       # what is here
wiki check                        # is it healthy? (links resolve, entries typed)
wiki list                         # every entry
```

That `my-wiki` folder is the whole thing. Open it in any editor, commit it to git, point Obsidian at it. `wiki` simply makes it queryable and keeps it honest.

## What you can ask it

Everything is a fast, scriptable query over the folder.

**Find things**

```sh
wiki list --type concept --tag crypto --prefix tech/    # by kind, topic, and subtree
wiki search "language model" --lines                    # full-text over frontmatter + body
wiki read /tech/infra/hetzner.md                        # an entry's body, frontmatter stripped
wiki outline /tech/infra/hetzner.md                     # its heading map
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
wiki tasks                         # every open - [ ] checkbox, across the whole base
wiki list --type task              # list task entries (detailed entries)
wiki tags --counts --sort=count    # what you write about most
wiki property status --counts      # how many open vs done, draft vs final
```

**Reshape it safely**

```sh
wiki move /a.md /archive/a.md   # relocate or rename, rewriting every link to it
wiki tidy                       # canonicalize links and filenames (run it bare to preview)
wiki check --fix                # health report, and repair the safe issues
```

Every command speaks `--format text|json|csv|tsv` and returns clean exit codes (`0` found, `1` none, `2` error), so it drops straight into scripts and pipelines:

```sh
wiki unresolved >/dev/null && echo "broken links found" || echo "all links resolve"
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

It is designed to work with any Wiki-LLM deployment, as well as any [Open Knowledge Format](https://cloud.google.com/blog/products/data-analytics/how-the-open-knowledge-format-can-improve-data-sharing) bundle.

## Run by an agent

The `wiki` CLI is built so an AI agent can operate the whole base, and it ships with the skills that teach one how.

You capture a half-formed thought on the go, and the agent files it as a `draft`. Later it reads your drafts back to you, asks the one or two questions that sharpen each, then promotes it into a real entry in the right place, linked to what it relates to. It keeps the indexes current, finds the orphans, normalizes the links, and tells you what is still unwritten. You think and decide; it does the filing.

The loop, in the tool's own verbs: capture, refine, promote, index, retrieve, maintain. Two skills live in [`.claude/skills/`](.claude/skills):

- **agentic-wiki**, the operating manual for a knowledge base.
- **wiki-tasks**, for a task backlog that is itself a wiki bundle.

Because the agent drives the same commands you do, you are never locked out of your own knowledge, and it is never locked out of helping.

## Everything it does

| Command | What it does |
|---|---|
| `init [dir]` | Scaffold a new bundle (`--force` to write into a non-empty dir) |
| `status` | Bundle and index summary |
| `list` | Entries, filtered by `--type` / `--tag` / `--prefix` |
| `search <q>` | Full-text over frontmatter and body (`--lines` for file:line) |
| `read <path>` | An entry's body, frontmatter stripped |
| `outline <path>` | An entry's heading hierarchy |
| `tasks` | Open `- [ ]` checkboxes (`--all`, `--done`) |
| `tags` / `properties` / `property <key>` | The base's vocabulary (`--counts`, `--sort=name\|count`) |
| `links <path>` / `backlinks <path>` | Outgoing / incoming links |
| `unresolved` / `orphans` | Broken links / entries with nothing linking in |
| `move <src> <dst>` | Relocate or rename, rewriting every link to it |
| `tidy` | Canonicalize the base: `--links`, `--slug`, `--all` (bare previews) |
| `check` | Health lint (`--fix` repairs the safe issues, e.g. version drift) |
| `version` | Print the version |

File arguments are **bundle paths**, not filesystem paths: a root-absolute `/finance/income.md`, or a bare `income.md` when it is unambiguous. They mean the same thing wherever you run from, and whatever `--root <dir>` points at. `--prefix` takes the same form to scope a listing to a subtree.

## Three layers, and what you actually own

The format is one of three layers, and only the first is required:

1. **Format.** The bundle itself: Markdown, frontmatter, links. Complete and navigable on its own, by a human, an LLM, or any OKF reader. Defined in the [spec repo](https://github.com/agentic-wiki/spec).
2. **Tool.** This repo, `wiki`: a neutral engine that indexes the bundle and answers structured queries. Built for agents to call, runnable by anyone.
3. **Skill.** The manual your agent follows to drive the tool. This is where workflow opinion lives, and you customize it.

The format is data at rest and stands on its own; the tool is a swappable engine over it; the skill is where the opinions live. You own all three, and you can keep just the first.

## The backlog itself is a wiki

There is no issue tracker and no "TASKS.md" here: this repo's own backlog lives in [`tasks/`](tasks/index.md), and it is itself an agentic-wiki bundle, managed by `wiki` itself. The tool runs on its own to-do list, with the same commands as above:

```sh
cd tasks
wiki tasks                           # what is left to build
wiki list --type task --tag feature  # new features
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
