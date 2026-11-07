# Operating this wiki

This folder is a wiki: Markdown files with YAML frontmatter, written by people and agents, and queried with the `wiki` command. The files are the source: edit them, and use `wiki` to find, follow and check them. This file is how to work in it. Change it when the way you work changes.

## Setting up

This wiki is new. Do these steps with the user, not alone. If they asked you for something else, do that first, then offer this setup once.

1. Agree what the wiki is for and who reads it, and write that in a sentence or two at the top of `index.md` (in its **Goal:** line, when it has one).
2. Read the rest of this file together, and delete what they won't use: a rule, a section, or a part's section with its folder and its types.
3. Set `types` in `wiki.toml` to the kinds of entry they need, and no others (uncomment the line if it is commented out).
4. Make the first folders, each with an `index.md`, and link each one from the root `index.md`.
5. Run `wiki check`, delete this section, and commit.

## The model

- **Folder, `type`, tags.** The folder is where an entry lives, by topic (`billing/`). The `type` is what it is (`note`, `task`), and every entry has one. Tags, when you need them, are what cuts across folders (`2026`, `needs-review`). A thing that would belong in two folders gets a tag instead.
- **When `wiki.toml` lists `types`, use only those.** Add a type there before you use it.
- **One thing per entry.** Similar but distinct things (a product and its pricing, a person and their team) get their own entries, linked to each other: being related is a reason to link, not to merge. Merge only true duplicates, two entries for the same thing.
- **Search before you write**: update the entry that exists rather than write a second one.

## Frontmatter

Frontmatter is YAML. Values can be text, numbers, `true`/`false`, lists and nested maps:

```yaml
---
type: note
title: "Late fees: how they're charged"
tags: [billing, 2026]
---
```

- Quote a text value that holds `: ` or ` #`, or starts with anything but a letter or digit, as `title` does above.
- An entry's title is its `title:` when this file's template for its type has one, and otherwise its first `# heading`. Use one, not both.
- An entry whose frontmatter isn't valid YAML has no fields until it's fixed, and `wiki check` names it.

## Structure

- **Each name is a file or a folder, never both.** A file (`late-fees.md`) is one thing. A folder with an `index.md` (`people/`) is a bag of separate entries: its `index.md` links them, unless this file says nothing has to link to them. An `index.md` is navigation, so it carries no frontmatter and no type (a type is what queries find); the root one carries only `okf_version`. Never write `expenses.md` and `expenses/index.md` together.
- **One thing with parts is the only exception: `thing.md` beside `thing/`.** `thing.md` is the typed entry that others link to, and `thing/` holds its parts, with no `index.md`. Ask: is the folder one thing with parts (`thing.md`), or a bag of separate entries (`index.md`)?
- **Keep folders shallow**, two or three levels (`wiki check` warns past three). File names are lowercase hyphenated slugs (`late-fees.md`), with no spaces.
- **`log.md`** is an optional dated chronicle, in any folder: no frontmatter, one `## 2026-11-07` heading per day. It records what happened; entries say what is.

## Links

- **Link with relative Markdown links**: `[Late fees](../billing/late-fees.md)`, with a `#heading` anchor when you need one. `wiki tidy` turns root-absolute links and `[[wikilinks]]` into this form.
- **The links are a graph.** `wiki backlinks` shows what points at an entry, `wiki orphans` what nothing points at (link it, or it's lost), and `wiki unresolved` the links to entries not written yet: a list of what to write. A link to a missing entry is allowed, as a promise: `check` warns about it until the entry is written.
- **Never move or rename a file by hand**: `wiki move` rewrites every link to it (add `--dry-run` to preview, and `--include-frontmatter` for paths in frontmatter). Before deleting an entry, run `wiki backlinks` on it and fix what links there.

## Work

- **A task is an entry that owns its state**: its `status` field is the truth, wherever the task is filed.
- **A `- [ ]` checkbox is a step of the entry it lives in**, and only that entry owns it. Never use one to track another entry's state: that copy drifts. `wiki checkboxes` gathers the open ones.
- **Boards are queries, not lists.** Find work by query (`wiki list --where type=task …`). Never list tasks in an entry or an `index.md`: queries find them, so nothing has to link to them.

## Data

Data takes one of two shapes, by what you do with it:

- **Records**, things you look up, link to and reason about one by one (companies, people, products): one entry per record, in a folder per kind (`companies/`), with its frontmatter as the columns. Every record of a kind carries the same keys, with values of the same shape. Write an unknown value as `key:` (null): don't leave the key out, and don't write `n/a`. Query them with `wiki list --where … --format csv` or `--format json`.
- **A dataset**, uniform rows you only count, sum or filter (transactions, readings): one entry (`type: dataset`, added to `types`) holding one Markdown table, one row per record, read with `wiki table <file> --format csv`. A cell that names an entry is text, not a link: when you need backlinks to each row, use records.

For both:

- **Store values raw and machine-readable**: `1234.55`, not `1,234.55 USD` (the unit gets its own column or key); dates as `2026-02-05`; numbers unquoted in frontmatter.
- **A dataset that grows becomes a bag**: `wiki move expenses.md expenses/2025.md`, add `expenses/2026.md`, and write an `expenses/index.md` that links them.

## Capture, refine, promote

Knowledge often arrives rough. Don't lose it, and don't leave it rough:

1. **Capture**: write the thought down now, as an entry with the closest type you have, where this wiki keeps unsorted entries (a section below names the place; with none, next to the root `index.md`, linked from it).
2. **Refine**: read it back, ask the user a question or two, and fill it in.
3. **Promote**: give it its real `type`, `wiki move` it into its folder, and link it from related entries and, as Structure says, from its folder's `index.md`.

## Health

- **Before you finish, run `wiki check`** and fix every error and warning. The one exception is a link to an entry not written yet that you mean to keep as a promise: `wiki unresolved` lists those.
- **Groom in small steps**: write what `unresolved` promises, link `orphans`, merge true duplicates, fold redundant tags, and keep each `index.md` current. Change something only when it makes things easier to find. Never restructure for its own sake.
- **Commit the wiki with the work that changed it**, after `wiki check` passes. Pull before you edit, when there is a remote. Without git there is no undo: make only the change you were asked for, and leave grooming for when the user is there.

## Commands

Run them inside the wiki, or from anywhere with `wiki --root <dir>`. `wiki <command> -h` gives a command's flags; `wiki help` lists them all.

```sh
wiki status                                        # what's here, and what's broken
wiki search "late fee"                             # every word must match; --exact for a phrase
wiki read billing/late-fees.md                     # an entry, without its frontmatter
wiki outline billing/late-fees.md                  # its headings
wiki list --where type=task --where status!=done   # entries, by their frontmatter
wiki property status --counts                      # the values a key takes, and how often
wiki backlinks billing/late-fees.md                # what links here
wiki unresolved                                    # links to entries not written yet
wiki orphans                                       # entries nothing links to
wiki move old.md billing/new.md                    # move or rename, rewriting every link
wiki tidy                                          # links and file names to fix; --all fixes them
wiki check                                         # errors and warnings; --fix repairs the safe ones
```

Every query command takes `--format json` (or `jsonl`, `csv`, `tsv`) for scripts: `wiki list --format json | jq`.
