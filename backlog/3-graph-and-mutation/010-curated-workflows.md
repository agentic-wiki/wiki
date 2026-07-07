---
type: task
title: "org-wiki + product-wiki starter workflows"
status: todo
priority: low
tags: [feature, scaffold]
---

Two more curated starters beyond `default` and `project-backlog` ([workflow scaffold](/3-graph-and-mutation/005-workflow-scaffold.md)). Each is an embedded `internal/scaffold/files/workflows/<name>/` dir (a `wiki.toml` with types + `skip`, a seed `index.md`, and a `WORKFLOW.md`); `AGENTS.md` stays shared. Keep each seed minimal (index.md + meta only) so scaffolds stay `check`-clean.

- **`org-wiki`**: a team/company knowledge base. Types `note, concept, entity, process, decision, meeting, project, source, draft`; folders `teams/ people/ processes/ decisions/ meetings/ projects/`.
- **`product-wiki`**: **wiki-first** product documentation. Atomic, linked `concept`/`reference` entries are the substance, with an optional parallel `guides/` layer that links *into* them (never duplicating). Named `product-wiki` (not `product-docs`) to signal wiki-first and to parallel `org-wiki`. Types `concept, reference, guide, decision, note, source, draft`.

Each new workflow ships a **First run: pin your conventions** section (like `project-backlog`), so an agent consolidates the template with the user (prune options, lock choices, scaffold a validated skeleton) before populating the base.

`personal` was dropped as too close to `default`.

**Done (2026-07-07):** `project-backlog` (kanban-style, multi-team; debt; everyday-question recipes; explicit lifecycle; a first-run consolidation step) shipped. An interactive `init` picker was prototyped and **removed**: zero-dep TTY detection (`os.ModeCharDevice`) can't tell `/dev/null` from a real terminal, so `wiki init >/dev/null` spuriously prompted (and could hang). Instead `--workflow` is optional and defaults to `default` with a `Using 'default' workflow` notice; a picker could return only with a proper zero-dep `isatty`.
