---
type: task
title: sort entries by timestamp
status: done
priority: low
tags: [feature, query]
---

`wiki list --sort=path|timestamp` (default `path`). `timestamp` sorts by the frontmatter `timestamp` field, falling back to file mtime when it is absent — the field is the curated "last meaningful change", mtime is the noisy fallback. Newest-first by default; `--reverse` flips it (oldest first, for the grooming pass).

Scope: `list` only for now (the other commands have their own natural orders). Capture mtime on the entry at index time; parse/compare timestamps reusing the RFC3339-or-date logic already in `check` (`validTimestamp`). Keep `--sort`'s value vocabulary distinct from tags/properties' `--sort=name|count` — same flag, per-command values.

**Done:** `wiki list --sort=path|timestamp` with `--reverse`; `timestamp` is newest-first, ties broken by path. Diverged from the plan above: rather than capturing mtime at index time for every file, `SortTime` reads mtime on demand and only for entries without a frontmatter `timestamp` (the fallback), and `cmdList` computes each entry's key once (not in the comparator). `parseTimestamp` was factored out of `validTimestamp`. Tests, smoke, PRD, README, and the agentic-wiki skill updated.
