---
type: task
title: lint + normalize non-root-absolute links
status: todo
priority: low
tags: [feature, check]
---

The format mandates root-absolute links (`/path.md`); relative/bare ones (`../x.md`, `sibling.md`) are currently parsed out by `InternalLinks`, so they pass `check` silently (neither a graph edge nor a warning).

- **Detect**: surface body links whose target has no URL scheme and does not start with `/`; `check` warns ("link not root-absolute").
- **Normalize** (suggest / `--fix`): resolve the relative target against the file's directory to its root-absolute form. If the resolved file exists, rewrite the link (dry-run suggests, `--fix` applies). If it does not resolve, leave it and report a broken link, never guess a target. External URLs are left untouched.

Reuses the link-rewrite engine from move (002); the `--fix` lives under [`check --fix`](/3-graph-and-mutation/007-check-fix.md). Keeps the graph root-absolute-only.
