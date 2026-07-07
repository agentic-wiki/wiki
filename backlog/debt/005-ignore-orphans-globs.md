---
type: task
title: "ignore_orphans matches only subtrees/exact paths, not full globs"
status: todo
priority: low
tags: [debt, config]
---

`ignore_orphans` ([conformance/006](/conformance/006-orphan-exempt-globs.md)) ships with minimal matching: `index.orphanExempt` treats each pattern as a **directory subtree** (`backlog/**`, `backlog/`, and bare `backlog` all mean "under `/backlog`") or an **exact path**. It does **not** support finer globs:

- a segment wildcard, e.g. `drafts/*.md` or `something-*.md`;
- a mid-path `**`, e.g. `teams/**/scratch.md`.

Those patterns are taken literally today, so they silently match nothing (an entry that should be exempt still shows as an orphan). Fix: a small zero-dep matcher supporting `*` (within a segment) and `**` (across segments), applied to each entry's bundle path. `path.Match` covers `*` per segment but not `**`, so `**` needs a tiny hand-rolled pass. Keep it zero-dep (no `doublestar`). Flagged when `ignore_orphans` shipped with the subtree-only matcher.
