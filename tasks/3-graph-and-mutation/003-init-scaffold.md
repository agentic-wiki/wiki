---
type: task
title: init — scaffold from embedded template
status: todo
priority: medium
tags: [feature, scaffold]
---

`wiki init [path]` writes a fresh conformant bundle into `path` (default: current directory): `wiki.toml`, a root `index.md` (carrying `okf_version`, so the new bundle is `check`-clean), `.gitignore`, and a minimal example, from a template embedded via `go:embed`. Refuse to overwrite a non-empty target unless `--force`. Template source is the template repo (tracked in the global roadmap), consumed at build time as a Go-module dependency; embed `bundle/` only so the template's `.git` is never bundled, and ship its ignore file as `gitignore` (renamed on write). No network at runtime. When it ships, document `wiki init` usage in the README (currently noted as planned).
