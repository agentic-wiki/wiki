---
type: task
title: "--where key!=value (inequality filter)"
status: done
priority: medium
tags: [feature, query]
---

`--where` matches only equality, so "not X" is inexpressible, most importantly `status!=done` (active work), and any other negation. Today you work around it with per-value queries (`--where status=todo`, `--where status=in-progress`, …) or `--format json | jq`. Surfaced repeatedly by the stress tests (an assignee's *active* workload, "what's not done").

Target: `wiki list --where type=task --where status!=done`.

- Parse `!=` before `=` in `whereFilters.Set` (split on the first `!=`, else the first `=`); carry a `Negate bool` on `index.PropFilter`, and have `matchesAll` invert that clause.
- **Missing-key semantics (decide + document):** `status!=done` on an entry with no `status` should **match** (it is not done), i.e. negation means "does not hold this value," which is true when the key is absent. Document it so it's predictable.
- **Stay eq/neq only.** No `<` / `>` / `OR` / precedence, that would be the query language the tool deliberately avoids (richer reporting stays a skill over `--format json`).
- Tests: match / no-match, missing key, combined with an equality `--where` (AND), list-valued field (`tags!=bug` = "no bug tag"), and a value containing `=`.
