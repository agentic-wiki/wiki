---
type: task
title: wiki move on directories (move a whole subtree, rewrite all links)
status: todo
priority: medium
tags: [feature, mutation]
---

`wiki move <src> <dest>` operates on a single entry (`idx.Move` resolves `src` as one entry). Relocating or renaming a whole folder (`projects/acme/` → `archive/acme/`, or renaming `people/` → `staff/`) means N moves by hand, and each missed one strands links. This is the common shape of a real reorganization, so `move` should take a directory and move everything under it in one pass, rewriting every inbound link and each moved entry's own outbound relative links (exactly what the single-file move already does, batched over the subtree).

Builds on [move-rename](./002-move-rename.md) and [relative links are canonical](./012-switch-canonical-links.md).

**To design before building:**

- **`dest` semantics.** `move projects/acme archive/acme` renames the subtree. Require `dest` to not already exist (a clean relocate), or allow merging into an existing folder? Start with "dest must not exist" (simplest, no collision rules); revisit merge only if needed.
- **Non-entry files in the subtree.** A folder can hold non-`.md` files (a gitignored `inbox/resources/*.pdf`, an image). `move` only knows about indexed entries. Decide: move the whole directory on disk (entries + everything else) and only rewrite links for the entries, or move entries only and leave the rest? Moving the whole dir on disk is what a user expects from "move the folder"; the link graph only covers the `.md`, which is fine.
- **Rollback.** A subtree move is many file writes; a mid-way failure leaves a half-moved base. This sharpens [move: no rollback on a partial write](../debt/004-move-no-rollback.md), which should be resolved first or together (a dir move is the case that makes it hurt).
- **`--dry-run`** must preview the full set of file relocations and link rewrites, not just the top-level dir.
- **Ambiguity guard.** If `src` matches both a file and a directory (e.g. `projects/acme.md` and `projects/acme/`), define the rule (both move together? the file is the entry, the dir its parts, per the `thing.md` + `thing/` convention, so moving one should probably move both, keeping the pair intact).

**Acceptance:** `wiki move projects/acme projects/acme-old` relocates every entry under it, rewrites every inbound link and every moved entry's own outbound links, and `--dry-run` previews all of it. Tests: nested dirs, inbound + outbound rewrite, dry-run, the `thing.md` + `thing/` pair moved as a unit, dest-exists rejected.
