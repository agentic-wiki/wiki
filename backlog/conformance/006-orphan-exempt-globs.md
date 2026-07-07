---
type: task
title: "wiki.toml: globs exempt from orphan reporting"
status: todo
priority: medium
tags: [conformance, config, dx]
---

Parked or retired entries that nothing links to (a `project-backlog`'s `backlog/` and `archive/`, say) show up under `wiki orphans` as noise. The current workaround is to give those folders an `index.md` that links their entries purely to silence the report. That is a non-feature: bookkeeping to appease the tool, not real navigation.

Instead, let `wiki.toml` list globs whose entries are **not reported as orphans**:

```toml
orphan_skip = ["backlog/**", "archive/**"]
```

Those files stay fully indexed (searchable, listable, linkable, still checked for a valid `type`); only `wiki orphans` (and the `orphans` count in `status`) skips them.

**Distinct from `skip`** ([non-entry files](/conformance/005-non-entry-files.md)): `skip` drops a path from the index entirely, whereas `orphan_skip` keeps it a normal entry (searchable, linkable, `type`-checked) and only leaves it out of the orphan report. Two skip-lists, two scopes; do not merge them.

**Open questions:**

- **Name.** `orphan_skip` (decided): parallels the existing `skip` key, a list of paths skipped, here by the orphan report rather than by indexing. Alternatives considered: `allow_orphans`, `skip_orphans`.
- **Glob syntax, zero-dep.** `filepath.Match` matches a single path segment only (no `**`); "everything under a folder" can be a simple path-prefix rule, or a tiny hand-rolled `**` matcher. Pick the minimal form that covers the folder case without a dependency.
- **Scope.** Orphans only; leave `broken` / `unresolved` / the `type` check untouched.

Touches `bundle.parseConfig` (read the field) and `index.Orphans` (filter matched paths). **Payoff:** removes the folder-index workaround, so the `project-backlog` WORKFLOW can drop its "give `backlog/` and `archive/` an index just to avoid orphan noise" guidance and keep folder indexes for real navigation only.
