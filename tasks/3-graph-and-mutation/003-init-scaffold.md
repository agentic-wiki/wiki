---
type: task
title: init — scaffold from embedded starter
status: done
priority: medium
tags: [feature, scaffold]
---

`wiki init [path]` writes a fresh conformant bundle into `path` (default: current directory): `wiki.toml`, a root `index.md` (carrying `okf_version`, so the new bundle is `check`-clean), `.gitignore`, and a minimal linked example, from a starter embedded via `go:embed`. Refuses a non-empty target unless `--force`. The ignore file ships as `gitignore` and is written out as `.gitignore`.

v1 embeds the starter in this repo (`internal/scaffold/files/`). Sourcing it from the separate `agentic-wiki/template` repo as a go-module dependency, plus `--template`/`--from`, is the [scaffold registry](/3-graph-and-mutation/005-scaffold-registry.md).
