---
type: task
title: unify entry filtering under one --where flag
status: done
priority: medium
tags: [feature, query]
---

Filtering was three special cases, `--type` (scalar), `--tag` (array), plus json output limited to path/name/type/title/tags, so a base that records `status`, `assignee`, `epic`, `priority` as frontmatter could only *count* those values (`property <key> --counts`), never list entries by value or export them. Surfaced by stress-test feedback; the recommended `wiki search "assignee: john"` fallback is a substring scan that also matches body text.

Done: one generic frontmatter filter, no per-field flags.

- **`--where key=value`** on `list` and `search`: matches an exact frontmatter value; repeatable = AND; a list-valued field (e.g. `tags`) matches on membership (includes), a scalar on equality; values may be quoted/composite and may contain `=` (split on the first). `index.PropFilter` + `Entry.MatchProperty` (via `parse.Strings`), applied inside `Filter`/`Search`.
- **Dropped `--type` and `--tag`**: they are exactly `--where type=X` and `--where tags=Y`, so the dedicated flags were redundant. `--prefix` (path) stays as the one non-frontmatter filter. Pre-release, so no compatibility shim.
- **`--format json` carries full frontmatter**: `Entry.MarshalJSON` emits every field flat, so `wiki list --format json | jq` is the reporting surface for rollups the CLI won't compute (velocity, burndown, cycle time). CSV/TSV keep the canonical columns.

Named `--where` (not `--property`) after weighing the trade-off: more ergonomic, and with `--type`/`--tag` gone there was no flag family left to stay consistent with. Docs (README, PRD, AGENTS, both WORKFLOWs) moved to `--where`; project-backlog recipes moved off `search "assignee:"`. Tests: `TestCmdListWhere`, `TestCmdListJSONFrontmatter`, `TestFilter`, `TestSearch`.
