---
type: task
title: "more workflows beyond default + interactive init selector"
status: todo
priority: low
tags: [feature, scaffold]
---

`wiki init --workflow` ships with only `default` ([workflow scaffold](/3-graph-and-mutation/005-workflow-scaffold.md)). Two related pieces:

**More curated workflows.** Add starters as embedded `internal/scaffold/files/workflows/<name>/` dirs — each a `wiki.toml` (types + `skip`), an `index.md`, and a `WORKFLOW.md`. `AGENTS.md` stays shared and workflow-independent; only `WORKFLOW.md`, the type vocabulary, and any seed structure vary. Candidates from the design session: `org-wiki`, `project-backlog` (a kanban board), `product-docs`, `personal`. Add on real need, not speculatively — one good second flavor (likely `project-backlog`, mirroring this repo's own `backlog/`) validates the shape.

**Interactive selector on `init`.** Once more than one workflow exists, `wiki init` with no `--workflow` should, on an interactive terminal, show a numbered selector; a single workflow (today's case) is used straight away, and a non-interactive run (piped / CI) falls back to `default` without prompting. Detect the TTY zero-dep via `os.Stdin.Stat()` + `os.ModeCharDevice` (no `x/term` dependency); a plain numbered prompt fits the tool's minimalism (no TUI library). Deferred here because with one workflow it can't trigger or be exercised.
