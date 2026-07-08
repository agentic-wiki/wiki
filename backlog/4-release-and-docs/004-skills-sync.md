---
type: task
title: "docs: align the skills repo with the current AGENTS.md + workflow model"
status: todo
priority: medium
tags: [docs]
---

The `skills/` repo (the agent-facing manual: `agentic-wiki/SKILL.md`, `cron/*.md`) still teaches a superseded task model. The mechanical staleness is already fixed (the removed `--type`/`--tag` flags are now `--where`, and `wiki tasks` is now `wiki checkboxes`), but the *model* diverges from what the scaffolded `AGENTS.md` and the `project-backlog` `WORKFLOW.md` now encode.

What to reconcile (skill currently teaches the left; the bundle now teaches the right):

- **Board shape.** SKILL says the board is `index.md` with `- [ ]` checkboxes linking to task entries, and to keep each board checkbox in sync with the entry's `status`. Current model: the board references task entries with **plain links**; a `type: task` entry **owns its `status`** (single source of truth); a `- [ ]` is an *entry's own subtask*, not a board mirror. A checkbox-per-task board is an allowed variant but you own the reconciliation (spec README "task source of truth"), so it is not the default to teach.
- **`wiki checkboxes` framing.** Present it as "an entry's own checklist items," not "the board's next-up list."
- **Prune / status sections.** The "flip the board checkbox to `- [x]`" and the prune table's board-checkbox column should become the plain-link retire flow (remove the board line, set `status`, move to `archive/` if kept), matching the workflow.
- **Cross-check** the whole SKILL against `internal/scaffold/files/AGENTS.md` and the four `WORKFLOW.md` starters for any other drift (e.g. `_path` in json, canonical columns are `_path,type`, `--where` negation/emptiness), and against `cron/*.md`.

Open question for the pass: how much should the general SKILL prescribe vs. defer to per-bundle `WORKFLOW.md`? The principle is "workflows offer recipes, not prescriptions," so the SKILL should teach the entry-owns-state defaults and point at the bundle's own `AGENTS.md`/`WORKFLOW.md` for specifics, rather than hard-coding one board recipe.

Note: `skills/` is a separate repo. Confirm scope before editing (it ships independently of `wiki`).
