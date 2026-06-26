---
type: task
title: slugify filenames (spaces -> hyphens)
status: todo
priority: low
tags: [feature, mutation]
---

`check` warns on a space in a filename (the slug convention). A command could auto-apply the fix: rename each spaced file to a hyphenated slug and rewrite every link to it, reusing `move`'s rename + link-rewrite engine, with `--dry-run`.

Open questions:
- **Home.** Not `check --fix` (that repairs conformance, e.g. okf_version; a bulk rename is canonicalization, and riskier). Either a `--slug` flag on `consolidate` ("canonicalize the bundle": link form *and* filenames) or a dedicated `wiki slug`. Lean dedicated, given the bulk-rename risk profile differs from consolidate's in-file rewrites.
- **Collisions.** `a b.md` and `a-b.md` both present: never clobber, report and skip.
- **Scope of "slug".** Start minimal: spaces -> `-` (and maybe uppercase -> lowercase). Don't over-normalize punctuation.
- **Reuses `move`** (rename + link rewrite), which now rewrites relative *and* root-absolute links to a moved file, so a renamed entry's inbound links are all fixed.
