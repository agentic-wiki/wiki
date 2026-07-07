---
type: task
title: init — scaffold from embedded starter
status: done
priority: medium
tags: [feature, scaffold]
---

`wiki init [path]` writes a fresh conformant bundle into `path` (default: current directory): `wiki.toml`, a root `index.md` (carrying `okf_version`, so the new bundle is `check`-clean), `.gitignore`, and a minimal linked example, from a starter embedded via `go:embed`. Refuses a non-empty target unless `--force`. The ignore file ships as `gitignore` and is written out as `.gitignore`.

v1 embeds the starter in this repo (`internal/scaffold/files/`). Growing it into a self-contained operating manual (`AGENTS.md`) plus a selectable `--workflow`, and trimming the example, is [workflow scaffold](/3-graph-and-mutation/005-workflow-scaffold.md).
