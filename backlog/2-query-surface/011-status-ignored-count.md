---
type: task
title: "status: report the count of ignored files"
status: todo
priority: low
tags: [feature, query]
---

`wiki status` reports entries, links, tags, checkboxes, broken, and orphans, but not how many files the bundle holds that it is *not* indexing (matched by `wiki.toml`'s `ignore` list). That count is a cheap, useful signal: it makes the excluded set visible (`AGENTS.md`, `WORKFLOW.md`, a git-ignored `raw/**`), and it catches an over-broad `ignore` glob silently swallowing files you meant to index.

Target: an `Ignored: N` line in `wiki status` (text + structured output).

- Count the `ignore` set only (files excluded from the index/check entirely), **not** `ignore_orphans` (those stay indexed, so they already show up in `Entries`). Keep the two distinct in the wording.
- Decide the denominator: count **markdown files skipped by `ignore`** (candidate entries that were excluded), not every ignored path (an `ignore` glob can match binaries or whole subtrees). The useful signal is "`.md` files that would be entries but are ignored."
- Cheap to compute: the index walk already tests each file against the ignore matcher (`internal/index`); increment a counter on the skip branch and carry it on the status result.
- Surface in every format (a text line + a field in `--format json`); extend the status test.
- Optional follow-up, not this task: a listing of *which* files are ignored (a `--verbose` or a `wiki ignored`). Keep this task to the count.
