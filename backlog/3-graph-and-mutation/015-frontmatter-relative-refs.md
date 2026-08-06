---
type: task
title: "move --include-frontmatter: resolve and write relative frontmatter refs"
status: done
priority: high
tags: [bug, graph, links]
---

**Done (2026-08-06):** frontmatter refs are now matched by *resolved target* and written in the canonical **relative** form, exactly like body links.

[move --include-frontmatter](./011-move-frontmatter-refs.md) shipped when root-absolute was still the canonical on-disk link form, so it matched frontmatter values by **exact string equality against the moved entry's root-absolute path** and wrote the new value the same way. Then [relative links became canonical](./012-switch-canonical-links.md) for bodies, and frontmatter was left behind. Two bugs fell out:

- **A relative frontmatter ref was invisible to `move`.** `blockers: [./task-1.md]` never equalled `/active/task-1.md`, so the flag silently did nothing and the ref dangled. The natural spelling, the one that matches every body link in the same bundle, was the one that broke.
- **The moved file's own frontmatter refs dangled on a cross-folder move.** `move` had already gained this responsibility for body links (a file's relative links must be respelled from its new directory); frontmatter never got the same treatment.

The fix mirrors the body-link pass in `Move`, in a new `frontmatterRewrites`:

- **Match by resolved target, not spelling.** Each value goes through `normalizeLink` (from the entry's *current* path), so relative and root-absolute refs are both found. Root-absolute stays accepted on the way in and is never "wrong", it is just respelled, the same courtesy body links get.
- **Write relative.** The new value is `relativeLink` from the entry's *post-move* path, so a frontmatter ref navigates in a plain renderer just like a body link, and one canonical on-disk form covers both.
- **Respell the moved file's own refs** from its new directory, the frontmatter twin of the existing body-link rule.
- **Anchors survive** (`spec: ../lib/c.md#usage`), and **out-of-bundle values are left exactly as authored**, as out-of-bundle body links are.

**The `.md` suffix is the whole heuristic**, and it is what makes this safe. Resolving *every* frontmatter value would rewrite `title: Some Note` into `./Some Note`, since an arbitrary string resolves to a perfectly valid in-bundle path. Requiring the value to end in `.md` (after any `#anchor`) means only things that are unmistakably path references are touched. The opt-in caveat from 011 is unchanged and still applies: the flag rewrites *every* matching value, including a snapshot field that happens to name the moved path.

Unchanged and still deliberately out of scope (both rejected in 011 as opinionated): `check` guessing at dangling frontmatter refs, and markdown links inside frontmatter. A plain path stays `--where`-filterable and does not collide with YAML flow-sequence syntax; wrapping it in `[text](path)` would sacrifice both for no gain, since frontmatter is not rendered.

Tests: `TestMoveIncludeFrontmatterRelative` (relative inbound refs, the moved file's own refs across folders, anchors, non-path values, out-of-bundle values), plus updated expectations in `TestMoveIncludeFrontmatter` and `TestCmdMoveIncludeFrontmatter`.
