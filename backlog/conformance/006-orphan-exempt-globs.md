---
type: task
title: "wiki.toml: globs exempt from orphan reporting"
status: done
priority: medium
tags: [conformance, config, dx]
---

Parked or retired entries that nothing links to (a `project-backlog`'s `backlog/` and `archive/`, say) show up under `wiki orphans` as noise. The current workaround is to give those folders an `index.md` that links their entries purely to silence the report. That is a non-feature: bookkeeping to appease the tool, not real navigation.

Instead, let `wiki.toml` list globs whose entries are **not reported as orphans**:

```toml
ignore_orphans = ["backlog/**", "archive/**"]
```

Those files stay fully indexed (searchable, listable, linkable, still checked for a valid `type`); only `wiki orphans` (and the `orphans` count in `status`) skips them.

**Distinct from `ignore`** ([non-entry files](./005-non-entry-files.md)): `ignore` drops a path from the index entirely, whereas `ignore_orphans` keeps it a normal entry (searchable, linkable, `type`-checked) and only leaves it out of the orphan report. Different keys, different scopes; do not merge them.

**Open questions:**

- **Name.** `ignore_orphans` (decided): extends the `ignore` family with a scope suffix, `ignore` drops files from the index, `ignore_orphans` keeps them but drops them from the orphan report. Alternatives considered: `allow_orphans` (reads like a bool), `orphan_skip`.
- **Glob syntax, zero-dep.** `filepath.Match` matches a single path segment only (no `**`); "everything under a folder" can be a simple path-prefix rule, or a tiny hand-rolled `**` matcher. Pick the minimal form that covers the folder case without a dependency.
- **Scope.** Orphans only; leave `broken` / `unresolved` / the `type` check untouched.

Touches `bundle.parseConfig` (read the field) and `index.Orphans` (filter matched paths). **Payoff:** removes the folder-index workaround, so the `project-backlog` WORKFLOW can drop its "give `backlog/` and `archive/` an index just to avoid orphan noise" guidance and keep folder indexes for real navigation only.

**Done (2026-07-07):** `ignore_orphans` shipped. `bundle.parseConfig` reads it (`Bundle.IgnoreOrphans`); `index.Orphans` skips any entry matched by `orphanExempt`. Glob support is minimal by design: a pattern is a **directory subtree** (`backlog/**`, `backlog/`, or bare `backlog`, all normalized to the `/backlog` prefix) or an **exact path**; finer globs (`*.md`, `a/**/b.md`) are deferred to [debt/005](../debt/005-ignore-orphans-globs.md). Wired into the `project-backlog` scaffold (`ignore_orphans = ["backlog/**", "archive/**"]`) and its WORKFLOW simplified (dropped the folder-index-to-silence-orphans workaround). Tests: `bundle` (`TestParseConfigIgnore`), `index` (`TestIgnoreOrphans`), plus an e2e check. Spec + AGENTS updated.
