---
type: task
title: consolidate relative links to root-absolute
status: done
priority: low
tags: [feature, check]
---

OKF allows two internal link forms: root-absolute (`/x.md`) and relative (`./x.md`, `../x.md`, `sibling.md`). Both are valid; root-absolute is the canonical form we prefer for stability. Relative links must still resolve into the graph (so `backlinks`/`orphans` see them), and there should be an opt-in way to normalize them.

- **Resolve**: `parse.Links` classifies every link into `{Absolute, Relative, External}`; `index.Build` resolves relatives against the entry's directory (`path.Join`) so the whole graph is absolute in memory. A relative link that resolves nowhere is a normal broken link (reported by `check`/`unresolved`), not a special case.
- **Consolidate**: `wiki consolidate [--dry-run]` rewrites relative links to their canonical root-absolute form (anchor + title preserved). It is an opt-in normalization, not a `fix`, relative links aren't wrong. The absolute form is deterministic, so it applies whether or not the target exists.

Done: `parse.Links` 3-bucket primitive; `index.Build` resolves relatives via `canonicalTarget`; `Consolidate`/`consolidateEntry` rewrite them (reusing move's regex engine on file-relative lines); `wiki consolidate` command (`--dry-run`). `check` no longer flags relative links.

Note: an earlier cut treated relative links as violations (`check` warned, `check --fix` normalized). Corrected after re-reading the OKF spec, which explicitly permits relative links and mandates that consumers tolerate broken ones.
