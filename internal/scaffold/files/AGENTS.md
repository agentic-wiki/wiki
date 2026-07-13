# Operating this knowledge base

This folder is an **agentic wiki bundle**: plain Markdown (someone's notes, documents, datasets, and tasks) that you operate with the `wiki` CLI. The goal is knowledge that stays plain and human-readable (portable, greppable, diffable, open in any editor or by any LLM, the user's forever) yet is **structured well enough to answer questions like a database**. Three things make that work, and upholding them is your job:

- **Every entry has one home** (a folder), **one kind** (`type`), and **tags** for what cuts across. So the base is never a loose pile: you can ask for every dataset, everything tagged `2026`, everything under `finance/`.
- **Entries link to what they relate to**, with relative Markdown links (`../finance/income.md`), so they navigate in any tool (GitHub, an editor, Obsidian). Those links form a **graph** (you can see what points at a page), what has no home (orphans), and what's been promised but not yet written (unresolved). A well-kept base can be *navigated* from `index.md` top-down, not grepped blindly.
- **The structure is the value.** The better everything is filed and linked, the more findable it all becomes. Filing precisely, linking generously, and never leaving a dangling reference is the point, not overhead.

`grep` and `ls` can't compute that graph or answer "which notes have no home." `wiki` indexes the folder and does, deterministically. The files are the source (the index `wiki` builds is derived and disposable): **you are the librarian, `wiki` is your database engine.** So for anything structural (finding, moving, checking) call `wiki` rather than guessing or hand-editing paths.

`wiki` runs within the bundle (it walks up to find `wiki.toml`) or from anywhere with `wiki --root <dir>`. This file teaches *how to operate it*; run `wiki <command> -h` for a command's exact flags, and `wiki help` for the full list.

**How *this* base is organized (its types, folders, and the loop you follow) lives in [WORKFLOW.md](./WORKFLOW.md); read it next.**

On a brand-new base, treat `WORKFLOW.md` as the starting point, not the final spec: before populating the base, help the user commit its conventions (prune it to what they will actually use), scaffold a small skeleton, and have them validate it. Consolidate first, populate second. (This base came from a `--workflow` starter; `wiki init -h` lists the others, and both this file and `WORKFLOW.md` are yours to reshape.)

## The model

Three orthogonal axes classify every entry (keep them separate and the base stays friction-free):

- **Folder** = one stable home, by domain (`finance/`, `tech/infra/`). Moving is `wiki move` (it rewrites the links for you); never hand-move a file.
- **`type`** = what an entry *is* (`note`, `concept`, `dataset`, `task`, …), required on every entry. Free-form by default; declare a `types` vocabulary in `wiki.toml` to enforce a fixed set (`wiki check` then errors on any undeclared type). `wiki property type --counts` shows what's in use.
- **Tags** = everything cross-cutting (`2026`, `needs-review`, a task's `feature`/`bug`). If a thing would ever live in two folders, it's a tag, not a folder.

**One thing per entry.** Similar-but-distinct things (a `type` and its factory) each get their own entry, linked to each other, never folded into one. Being adjacent or similarly named is a reason to link, not to merge. Merge only true duplicates: two entries for the *same* thing.

**A pile of records is one entry, not many.** "One thing per entry" governs *knowledge* you link to and reason about. Its opposite is *uniform records* you mostly count, sum, or filter in bulk (invoices, expenses, transactions, sensor readings): those go in a single `type: dataset` entry as one Markdown table, one row per record, queried with `wiki table <file> --format csv | duckdb` (or `jq` over `--format json`). The test is whether you want `backlinks` to each one or `SUM`/`GROUP BY` over all of them: individual identity wants entries, aggregation wants a dataset. **Store each cell raw and machine-readable** from the first row (an unformatted number `1234.55`, an ISO date `2026-02-05`, any unit such as a currency in its own column), never display-formatted (`1,234.55 USD`): `wiki table` then feeds `duckdb`/`jq` with no cleanup, and formatting stays a presentation concern rather than something baked into the stored data (a formatted `"1,234.55"` is a string that won't sum, and forces a fragile `gsub` at query time). A table row is data, not a graph node (a cell may name `/clients/acme.md`, but that is text, not an edge `backlinks` follows); if one table grows unwieldy, partition it by year or owner (`finance/invoices/2026.md`). And since a dataset needs a `type`, it is always a typed file (`invoices.md`), never an `index.md` (which carries no frontmatter): a folder of yearly tables is a collection fronted by a typeless `invoices/index.md` that *links* the dataset files, it cannot *be* one.

Entries link with standard Markdown, relative to the linking file: `[Income](../finance/income.md)`, so they navigate in any renderer. Two conveniences, both normalized by `wiki tidy`: a hand-written root-absolute link (`/finance/income.md`) still resolves and `tidy --links` respells it relative; an Obsidian `[[wikilink]]` resolves too and `tidy --wikilinks` converts it to a standard Markdown link. Two reserved filenames carry no frontmatter at all (so no `type`): `index.md` (a folder's navigation surface) and the optional `log.md` (a dated chronicle); the one exception is the bundle-root `index.md`, which carries `okf_version`.

Since an `index.md` holds no frontmatter (no `type`, so no concept of its own), the two serve different jobs, and the deciding question is what the folder *is*.

- A **`thing.md` is load-bearing**: when a folder's contents are the parts of one thing (a project, a product, a plugin), that thing needs a home that can be typed, linked to, and found, so it gets a `thing.md` beside its `thing/` folder, never a typeless `thing/index.md`. 
- A **folder `index.md` is only the entry point into a collection**: it fronts a folder of otherwise-independent entries (the bundle root, `projects/`, `people/`) to navigate what's inside, and carries no concept of its own.

Ask which the folder is: one concept with parts (`thing.md`), or a bag of separate entries (`index.md`).

`wiki` only indexes `.md` files. Files matched by `wiki.toml`'s `ignore` list (this manual, `WORKFLOW.md`) are operating docs, not wiki entries: absent from the index and from `check`. Paths matched by `ignore_orphans` stay full entries (indexed and `check`ed), only kept out of the `orphans` report (a parked backlog, say). So a file you want left out entirely (a `LEARNINGS.md`, a `README.md`) goes in `ignore`, not `ignore_orphans`. Both accept an exact path or a glob (`*`, `?`, `**`), so a single file (`AGENTS.md`) and a subtree (`archive/**`) both work.

## Get oriented

On a base you don't already know, look around first, and the fastest way to learn its shape, vocabulary, and health:

```sh
wiki status                                # entries, links, tags, tasks, broken, orphans
wiki list --sort=timestamp | head -n 40    # what changed recently (--reverse for the stalest)
wiki property type --counts                # what kinds of entries exist
wiki tags --counts --sort=count            # dominant topics
find -maxdepth 2 -type d | grep -v .git/
```

Then follow `/index.md` and its links down, rather than grepping in the dark.

## Division of labor

- **You** read, write, and edit Markdown directly: create entries, compose frontmatter, draft content, add links. This is the judgment work `wiki` can't do.
- **`wiki`** is the deterministic engine: queries, the link graph, moves, tidying, health. It never creates content.

A query is only as good as the indexing preceding it: an entry nothing links to is invisible to `backlinks`; a task never added to a board won't show in `tasks`. The tool reflects what is *indexed*, not what exists. Using `ls`, `find` or `grep` is the last resort.

## Recipes: a need, and the command that meets it

### Find and recall

```sh
wiki search "docker networking"                # every word by default (AND), case-insensitive, over frontmatter + body
wiki search "docker networking" --any          # any word (OR): broadens the net
wiki search "docker networking" --exact        # the verbatim phrase (contiguous)
wiki search "docker" --lines                   # matching lines as file:line
wiki list --where type=concept --where tags=docker   # filter by kind and topic (repeatable = AND)
wiki list --where type=task --where status=done       # any frontmatter field, one flag
wiki list --where type=task --where status!=done      # key!=value negates (here: still-open work)
wiki list --where type=note --prefix personal/       # scope to a subtree
wiki read /tech/infra/docker.md                # an entry's body, frontmatter stripped
wiki outline /tech/infra/docker.md             # its heading map
wiki table /finance/expenses.md --format csv   # extract a dataset's table, for jq/duckdb
```

`--prefix <path>` scopes to a subtree and works on `list`, `search`, `checkboxes`, `tags`, `properties`, and `property`: reach for it to narrow any query in a large base. When the user asks something, check the base first: it may hold the answer, or reveal a gap worth a new entry. For structured questions ("all blocked tasks"), prefer `list --where status=blocked` (exact frontmatter-value match, repeatable for AND) over `search`, which matches query words as substrings anywhere in the file (frontmatter + body): by default every word must appear, in any order (broaden with `--any`, or narrow to a verbatim phrase with `--exact`). Use `key!=value` for negation (`status!=done` is everything still open, including entries with no `status` yet). Compare against the empty string to test emptiness: `--where assignee=` is entries with no assignee (unset or blank), and `--where assignee!=` is those that have one. It stays equality/inequality only; richer reporting is a skill over `--format json`.

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

*(How this base holds unclassified thoughts, such as an inbox or a `draft` type, is a [WORKFLOW.md](./WORKFLOW.md) choice.)*

### Reshape safely

```sh
wiki move --dry-run /a.md /archive/a.md   # preview the link rewrites
wiki move /a.md /archive/a.md             # relocate + rewrite every inbound link in one pass
wiki tidy                                 # preview canonicalization; `tidy --all` applies (links→absolute, names→slugs)
```

Prefer `wiki move` over read-delete-rewrite by hand: hand-moving strands every backlink.

### Track work

Two things wear a checkbox-ish shape; the model separates them by **who owns the state**:

- A **`type: task` entry** is a first-class thing that owns its own state (`status` frontmatter is the source of truth). List them with `wiki list --where type=task`.
- A **`- [ ]` checklist item** is a *subtask or step of the entry it lives in* (a task's steps, a note's to-dos), owned by that entry and nothing else. `wiki checkboxes` gathers open checklist items across the base, a `--prefix` subtree, or one `[file]`. (The command is `checkboxes`, not `tasks`: it scans the `- [ ]` construct, it does not list `type: task` entries.)

```sh
wiki checkboxes                 # open '- [ ]' items across the base
wiki checkboxes /active/login.md  # just that entry's own subtasks
wiki list --where type=task     # the task entries themselves
```

A **board** (`index.md`) references task entries with **plain links**, not by checkboxing them: a link is an external reference and never carries the target's state (the entry owns that, so a board checkbox would just be a second copy that drifts). A genuinely trivial to-do not worth its own entry can sit as a bare `- [ ]` on the board, but then it is a checklist item, not a queryable entry, that is the trade-off. The board is **authored, not generated**: `wiki` never edits it, you keep it current; a task the board omits shows up under `wiki orphans` if nothing else links it.

*(How this base runs a board (columns, priorities, pruning) is in [WORKFLOW.md](./WORKFLOW.md).)*

## Keep it healthy

**After any batch of edits, run `wiki check` before you're done.** It's the gate:

```sh
wiki check        # warnings (e.g. a broken link) exit 0; errors (missing/unknown type) exit 1
wiki check --fix  # apply the safe auto-repairs (e.g. okf_version sync)
```

Groom in small steps that compound: turn `unresolved` links into entries when it helps, re-home `orphans`, surface and merge true duplicates (two entries for one thing; similar-but-distinct entries stay separate and linked), consolidate redundant tags, link related entries, and keep each folder's `index.md` current. **Change something only when it makes the base more findable, never restructure for its own sake.** A broken link is tolerated (it's future knowledge, and `unresolved` lists it); `check` likewise warns on a `#anchor` link whose target has no such heading.

## Git and safety

Git is optional but highly recommended: it is the undo for a base an agent edits. When the base is at the root of a repo: pull before editing (otherwise ask the user), and after `wiki check` passes, commit the batch with a clear message; resolve conflicts by preserving both sides' intent and merging frontmatter sensibly. **Without git there is no undo, so don't groom or restructure unattended; make the change you were asked for and leave sweeping grooming for when the user is present.**

## Conventions

- Every entry has a `type` (reserved `index.md`/`log.md`); slug filenames (lowercase, hyphenated, no spaces); shallow folders (2–3 levels).
- Root-absolute links, no wikilinks: `wiki check` flags any `[[wikilink]]` (they're resolved into the graph for compatibility, but aren't part of the format), so rewrite them as standard Markdown links (or run `wiki tidy --wikilinks` to convert them).
- Every command takes `--format text|json|csv|tsv` (`json`/`csv`/`tsv` for structured output you can pipe). `list --format json` carries each entry's full frontmatter (every field, not just the shown columns), so `wiki list --where type=task --format json | jq …` is the reporting surface for rollups the CLI does not compute itself. text and csv/tsv show only the canonical columns, the entry's path and `type` (the two fields every entry is guaranteed to have); everything else (title, tags, status, …) is json-only. In json the entry's own file path is under the reserved key `_path` (leading underscore; the basename is just `basename(_path)`), so a frontmatter field named `name:` or `path:` round-trips untouched; every other key is your frontmatter verbatim.
- Exit codes: `0` ok (enumerations return `0` even when empty), `1` no match (`search`/`table`) or `check` errors, `2` a real error.
- `wiki` not installed? `brew install agentic-wiki/tap/wiki`, or see the [wiki repo](https://github.com/agentic-wiki/wiki).
