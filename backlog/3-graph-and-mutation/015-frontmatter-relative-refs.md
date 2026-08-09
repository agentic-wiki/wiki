---
type: task
title: "move --include-frontmatter: find relative frontmatter refs, normalize to root-absolute"
status: done
priority: high
tags: [bug, graph, links]
---

**Done (2026-08-07):** frontmatter refs are now matched by *resolved target*, so a relative ref is found as readily as a root-absolute one, and normalized to the canonical **root-absolute** form on write.

[move --include-frontmatter](./011-move-frontmatter-refs.md) shipped when root-absolute was still the canonical on-disk link form, so it matched frontmatter values by **exact string equality against the moved entry's root-absolute path**. Then [relative links became canonical](./012-switch-canonical-links.md) for bodies, and frontmatter was left behind. Two bugs fell out:

- **A relative frontmatter ref was invisible to `move`.** `blockers: [./task-1.md]` never equalled `/active/task-1.md`, so the flag silently did nothing and the ref dangled. The natural spelling, the one that matches every body link in the same bundle, was the one that broke.
- **The moved file's own relative frontmatter refs dangled on a cross-folder move.** `move` had already gained this responsibility for body links; frontmatter never got the same treatment.

The fix mirrors the body-link pass in `Move`, in a new `frontmatterRewrites`:

- **Match by resolved target, not spelling.** Each value goes through `normalizeLink` (from the entry's *current* path), so relative and root-absolute refs are both found.
- **Write root-absolute.** See below: this is where frontmatter deliberately parts company with body links.
- **Normalize the moved file's own relative refs**, which would otherwise dangle from its new directory.
- **Anchors survive** (`spec: /lib/c.md#usage`), and **out-of-bundle values are left exactly as authored**, as out-of-bundle body links are.

## Why frontmatter does not follow bodies to relative

Writing relative was implemented first and **reverted**, because it broke the one thing the frontmatter-field recipe exists for.

A root-absolute value is a **stable key**: every entry referencing `/epics/onboarding.md` spells it identically, so `wiki list --where epic=/epics/onboarding.md` finds them all. A relative value is a *per-file* spelling, so the same target reads `./x.md` from the root and `../epics/x.md` from `active/`, and **no single `--where` query can match every referrer**. Since `--where` is exact string equality (deliberately, so the tool never has to guess that a value is a path), relative storage silently returns a subset. It would also convert existing bundles on their first move, breaking queries that used to work.

Meanwhile the argument that motivated relative bodies does not apply at all: 012 flipped body links because root-absolute breaks navigation on GitHub, in editors, and in VS Code preview. **Frontmatter is never rendered as a link anywhere** (GitHub shows a table, Obsidian shows properties), so there is no navigation to preserve. 012 said as much and was right.

The resulting split is a principle worth stating, not an inconsistency: **a body link is relative because it must navigate; a frontmatter ref is root-absolute because it must be a stable key.**

Rejected along the way: having `--where` resolve path-shaped values before comparing, so relative storage stays queryable. It would force the tool to decide which values are paths (the opacity 011 protected) on the hottest query path, break the property that `--where` matches exactly what `list --format json` prints, and still leave `property --counts`, `jq`, and every other consumer seeing fragmented spellings.

**The `.md` suffix is the whole heuristic**, and it is what makes this safe. Resolving *every* frontmatter value would rewrite `title: Some Note` into a path, since an arbitrary string resolves to a perfectly valid in-bundle one. Requiring the value to end in `.md` (after any `#anchor`) means only unmistakable path references are touched. The opt-in caveat from 011 is unchanged and still applies: the flag rewrites *every* matching value, including a snapshot field that happens to name the moved path.

Unchanged and still deliberately out of scope (both rejected in 011 as opinionated): `check` guessing at dangling frontmatter refs, and markdown links inside frontmatter. A plain path stays `--where`-filterable and does not collide with YAML flow-sequence syntax; wrapping it in `[text](path)` would sacrifice both for no gain, since frontmatter is not rendered.

Tests: `TestMoveIncludeFrontmatterRelative` (relative inbound refs, the moved file's own refs across folders, anchors, non-path values, out-of-bundle values), plus updated expectations in `TestMoveIncludeFrontmatter` and `TestCmdMoveIncludeFrontmatter`.
