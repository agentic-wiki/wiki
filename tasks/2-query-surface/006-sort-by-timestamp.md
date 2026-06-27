---
type: task
title: sort entries by timestamp
status: todo
priority: low
tags: [feature, query]
---

`wiki list --sort=path|timestamp` (default `path`). `timestamp` sorts by the frontmatter `timestamp` field, falling back to file mtime when it is absent — the field is the curated "last meaningful change", mtime is the noisy fallback. Newest-first by default; `--reverse` flips it (oldest/stalest first, for the grooming pass).

Scope: `list` only for now (the other commands have their own natural orders). Capture mtime on the entry at index time; parse/compare timestamps reusing the RFC3339-or-date logic already in `check` (`validTimestamp`). Keep `--sort`'s value vocabulary distinct from tags/properties' `--sort=name|count` — same flag, per-command values.
