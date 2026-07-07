# Operating this knowledge base

This folder is an **agentic wiki bundle**: plain Markdown (someone's notes, documents, datasets, and tasks) that you operate with the `wiki` CLI. The goal is knowledge that stays plain and human-readable (portable, greppable, diffable, open in any editor or by any LLM, the user's forever) yet is **structured well enough to answer questions like a database**. Three things make that work, and upholding them is your job:

- **Every entry has one home** (a folder), **one kind** (`type`), and **tags** for what cuts across. So the base is never a loose pile: you can ask for every dataset, everything tagged `2026`, everything under `finance/`.
- **Entries link to what they relate to**, root-absolute (`/finance/income.md`). Those links form a **graph** (you can see what points at a page), what has no home (orphans), and what's been promised but not yet written (unresolved). A well-kept base can be *navigated* from `index.md` top-down, not grepped blindly.
- **The structure is the value.** The better everything is filed and linked, the more findable it all becomes. Filing precisely, linking generously, and never leaving a dangling reference is the point, not overhead.

`grep` and `ls` can't compute that graph or answer "which notes have no home." `wiki` indexes the folder and does, deterministically. The files are the source (the index `wiki` builds is derived and disposable): **you are the librarian, `wiki` is your database engine.** So for anything structural (finding, moving, checking) call `wiki` rather than guessing or hand-editing paths.

`wiki` runs within the bundle (it walks up to find `wiki.toml`) or from anywhere with `wiki --root <dir>`. This file teaches *how to operate it*; run `wiki <command> -h` for a command's exact flags, and `wiki help` for the full list.

**How *this* base is organized (its types, folders, and the loop you follow) lives in [WORKFLOW.md](/WORKFLOW.md); read it next.**

On a brand-new base, treat `WORKFLOW.md` as a starting template, not a finished spec: before populating the base, help the user commit its conventions (prune it to what they will actually use), scaffold a small skeleton, and have them validate it. Consolidate first, populate second.

## The model

Three orthogonal axes classify every entry (keep them separate and the base stays friction-free):

- **Folder** = one stable home, by domain (`finance/`, `tech/infra/`). Moving is `wiki move` (it rewrites the links for you); never hand-move a file.
- **`type`** = what an entry *is* (`note`, `concept`, `dataset`, `task`, …), declared in `wiki.toml`, required on every entry.
- **Tags** = everything cross-cutting (`2026`, `needs-review`, a task's `feature`/`bug`). If a thing would ever live in two folders, it's a tag, not a folder.

Entries link with standard Markdown, root-absolute from the bundle root: `[Income](/finance/income.md)`. No `[[wikilinks]]`. Two reserved filenames carry no `type`: `index.md` (a folder's navigation surface) and the optional `log.md` (a dated chronicle). Files matched by `wiki.toml`'s `ignore` list (this manual, `WORKFLOW.md`) are operating docs, not wiki entries; paths matched by `ignore_orphans` stay entries but are kept out of the `orphans` report (a parked backlog, say). Both accept an exact path or a glob (`*`, `?`, `**`), so a single file (`AGENTS.md`) and a subtree (`archive/**`) both work.

## Get oriented

On a base you don't already know, look around first, and the fastest way to learn its shape, vocabulary, and health:

```sh
wiki status                                # entries, links, tags, tasks, broken, orphans
wiki list --sort=timestamp | head -n 40    # what changed recently (--reverse for the stalest)
wiki property type --counts                # what kinds of entries exist
wiki tags --counts --sort=count            # dominant topics
find -maxdepth 1 -type d
```

Then follow `/index.md` and its links down, rather than grepping in the dark.

## Division of labor

- **You** read, write, and edit Markdown directly: create entries, compose frontmatter, draft content, add links. This is the judgment work `wiki` can't do.
- **`wiki`** is the deterministic engine: queries, the link graph, moves, tidying, health. It never creates content.

A query is only as good as the indexing preceding it: an entry nothing links to is invisible to `backlinks`; a task never added to a board won't show in `tasks`. The tool reflects what is *indexed*, not what exists. Using `ls`, `find` or `grep` is the last resort.

## Recipes: a need, and the command that meets it

### Find and recall

```sh
wiki search "docker networking"                # literal, case-insensitive, over frontmatter + body
wiki search "docker" --lines                   # matching lines as file:line
wiki list --where type=concept --where tags=docker   # filter by kind and topic (repeatable = AND)
wiki list --where type=task --where status=done       # any frontmatter field, one flag
wiki list --where type=note --prefix personal/       # scope to a subtree
wiki read /tech/infra/docker.md                # an entry's body, frontmatter stripped
wiki outline /tech/infra/docker.md             # its heading map
wiki table /finance/expenses.md --format csv   # extract a dataset's table, for jq/duckdb
```

`--prefix <path>` scopes to a subtree and works on `list`, `search`, `tasks`, `tags`, `properties`, and `property`: reach for it to narrow any query in a large base. When the user asks something, check the base first: it may hold the answer, or reveal a gap worth a new entry. For structured questions ("all blocked tasks"), prefer `list --where status=blocked` (exact frontmatter-value match, repeatable for AND) over `search`, which is a substring scan of the whole file and will also match body text.

### Follow the graph (what grep can't do)

```sh
wiki links /index.md                  # what it points to
wiki backlinks /tech/infra/docker.md  # what points here
wiki unresolved                       # promised but not yet written: a to-write list
wiki orphans                          # nothing links in: knowledge to index
```

### Capture → refine → promote

Knowledge often arrives rough and matures in place. You write the file; `wiki` only queries it.

1. **Capture** the rough thought as an entry now (don't lose it to a scratchpad).
2. **Refine**: read it back (`wiki read`), ask the user a sharpening question or two, fill it in.
3. **Promote**: give it a real `type`, `wiki move` it into its domain, and **link it in** from the domain's `index.md` and related entries. An unlinked entry is lost knowledge.

*(How this base holds unclassified thoughts, such as an inbox or a `draft` type, is a [WORKFLOW.md](/WORKFLOW.md) choice.)*

### Reshape safely

```sh
wiki move --dry-run /a.md /archive/a.md   # preview the link rewrites
wiki move /a.md /archive/a.md             # relocate + rewrite every inbound link in one pass
wiki tidy                                 # preview canonicalization; `tidy --all` applies (links→absolute, names→slugs)
```

Prefer `wiki move` over read-delete-rewrite by hand: hand-moving strands every backlink.

### Track work

A task is an entry of `type: task`; a board is an `index.md` of `- [ ]` checkboxes linking to them. These are two distinct surfaces, and it matters which you query:

```sh
wiki tasks                # scans boards for `- [ ]` checkboxes (not type:task entries)
wiki list --where type=task   # every type:task entry, whether or not a board links it
```

The board is **authored, not generated**: `wiki` never adds a task to it, ticks a box, or prunes a done one. You keep it current, and you keep each checkbox in agreement with its entry's `status` (checked exactly when `status` is `done`), both changed in one edit. A task with no board checkbox is invisible to `wiki tasks` (find it with `wiki list --where type=task`); reconcile that during grooming.

*(How this base runs a board (columns, priorities, pruning) is in [WORKFLOW.md](/WORKFLOW.md).)*

## Keep it healthy

**After any batch of edits, run `wiki check` before you're done.** It's the gate:

```sh
wiki check        # warnings (e.g. a broken link) exit 0; errors (missing/unknown type) exit 1
wiki check --fix  # apply the safe auto-repairs (e.g. okf_version sync)
```

Groom in small steps that compound: turn `unresolved` links into entries when it helps, re-home `orphans`, surface and merge duplicates, consolidate redundant tags, link related entries, and keep each folder's `index.md` current. **Change something only when it makes the base more findable, never restructure for its own sake.** A broken link is tolerated (it's future knowledge, and `unresolved` lists it).

## Git and safety

Git is optional but highly recommended: it is the undo for a base an agent edits. When the base is at the root of a repo: pull before editing (otherwise ask the user), and after `wiki check` passes, commit the batch with a clear message; resolve conflicts by preserving both sides' intent and merging frontmatter sensibly. **Without git there is no undo, so don't groom or restructure unattended; make the change you were asked for and leave sweeping grooming for when the user is present.**

## Conventions

- Every entry has a `type` (reserved `index.md`/`log.md`); slug filenames (lowercase, hyphenated, no spaces); shallow folders (2–3 levels).
- Root-absolute links, no wikilinks. If you meet a `[[wikilink]]`, rewrite it as a standard Markdown link.
- Every command takes `--format text|json|csv|tsv` (`json`/`csv`/`tsv` for structured output you can pipe). `list --format json` carries each entry's full frontmatter (every field, not just the shown columns), so `wiki list --where type=task --format json | jq …` is the reporting surface for rollups the CLI does not compute itself; csv/tsv carry the canonical columns.
- Exit codes: `0` ok (enumerations return `0` even when empty), `1` no match (`search`/`table`) or `check` errors, `2` a real error.
- `wiki` not installed? `brew install agentic-wiki/tap/wiki`, or see the [wiki repo](https://github.com/agentic-wiki/wiki).
