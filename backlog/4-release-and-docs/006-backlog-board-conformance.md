---
type: task
title: "this backlog's own board does not follow the workflow it ships"
status: todo
priority: medium
tags: [docs, dogfood, workflow]
---

`wiki` ships the `project-backlog` workflow, and its own backlog breaks the workflow's central rule.

> Each `index.md` is a board: overarching **goal(s)** at the top, then sections of **plain links** to the issue entries. A link is a reference, not a copy of state: the linked entry owns its `status`, the single source of truth for progress, so the board is a curated *view* with nothing to keep in sync and nothing to drift (**a board checkbox would be a second copy of the entry's done-ness**).

This board is 98 lines listing 78 entries, 63 of them as `- [x]` checkboxes mirroring each entry's `status`. That is the variant the workflow warns about, and it has already drifted: task `conformance/010` sat at `status: done` with an unchecked box until an audit caught it. Nothing catches that automatically, because a checkbox and a `status` are two facts and only one of them is queried.

`wikiview`'s backlog was converted to the intended shape and is the working reference: goal and epics on the board, tasks found by query, `ignore_orphans` so unlisted entries do not become orphans.

## Why this is not a five-minute edit

Deleting the checkboxes would destroy history. Two separate problems, both measured:

- **22 done entries carry no `Done` section of their own**, so the board annotation is the only record of what was decided (`/2-query-surface/001-read-and-outline.md`, `002-search.md`, `003-tags-and-properties.md`, `004-output-formats.md`, and 18 more). That text has to move *into* each entry first.
- **22 board lines are bare checklist items, not links to entries at all** — real work with no entry anywhere ("datasets guidance + `org-wiki`→`org-base` rename", "stress-test 6/7 polish", …). There is nothing to move them into.

The second is the interesting one, and the format already answers it: `log.md` is defined as an append-only chronological narrative of what happened in an area over time, exempt from `orphans` and the `type` requirement. That is exactly what those lines are. They are not tasks and never should have been checkboxes.

## Shape

1. Move each done entry's board annotation into that entry, under `## Done`, so the entry is the single record of its own outcome.
2. Move the bare checklist items into `log.md`, dated, as the chronicle they already are.
3. Rewrite `index.md` as goal + epics + queries, matching `wikiview`'s.
4. Add `ignore_orphans` for the section folders, or every entry becomes an orphan the moment it stops being linked. A per-subfolder glob (`*/**`) needs no edit when a section is added, and leaves the bundle root covered so `index.md` itself stays honest.

Steps 1 and 2 are the work; 3 and 4 are minutes. Do them in that order and nothing is lost at any point.

## Worth deciding while in here

The board's sections are numbered phases (`2-query-surface`, `3-graph-and-mutation`) that are really **epics**, and they are folder names as well as headings. `wikiview` treats them as prose on the board. Whether they should instead be `type: epic` entries that tasks reference — making "what is in this epic" a query rather than a folder convention — is a real question, and `wiki.toml` here declares `types = ["task"]`, so it is currently unaskable.

**Acceptance:** the board carries goals and epics, not task state; `wiki checkboxes` over this bundle returns nothing; every done entry records its own outcome; the bare items live in `log.md`; `orphans` is clean; nothing that was on the board is lost.
