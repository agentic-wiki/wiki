---
type: task
title: wiki tidy — opt-in bundle canonicalization (links + slugs)
status: done
priority: medium
tags: [feature, mutation]
---

Replace the standalone `consolidate` command with `wiki tidy`, an umbrella for opt-in canonicalization of an already-valid bundle:

- bare `wiki tidy` → **preview only**: report what each category would change, write nothing (the agent-friendly default; `--format json` for machines). `-h` lists the flags.
- `--links` → normalize relative links to root-absolute (today's `consolidate`, reuses `Index.Consolidate`).
- `--slug` → rename spaced filenames to hyphenated slugs, rewriting inbound links (via `move`, which now handles relative links).
- `--all` → every category.

Non-interactive (no prompts): validate before writing, report exactly what changed. No `--dry-run`: the bare command already previews every category, so a per-scope dry-run would be redundant. To preview, run bare `wiki tidy`; then apply the categories you want. v1 `--slug` despaces filenames (`a b.md` → `a-b.md`, case preserved); skips name collisions; spaced *directories* are out of scope (still flagged by `check`); cross-linked spaced files may need a second run.

Replaces the standalone `consolidate` command, and folds in what was a separate slugify task (its open questions are resolved above: slug = spaces→hyphens with case preserved, collisions skipped & reported, reusing `move`'s rename + link-rewrite).
