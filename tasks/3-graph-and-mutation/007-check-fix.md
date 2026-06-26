---
type: task
title: wiki check --fix
status: done
priority: medium
tags: [feature, graph]
---

`wiki check --fix`: auto-repair the conformance issues that are safe to write, starting with the bundle-root `index.md` `okf_version` (sync it to the value `wiki.toml`'s `spec` embeds; `check` only flags the drift today). Scope is "make this bundle conform to its declared spec," not version migration (see [spec upgrade](/3-graph-and-mutation/008-spec-upgrade.md)). Other safe fixes fold in here too, e.g. normalizing non-root-absolute links ([relative-link lint](/3-graph-and-mutation/006-relative-link-lint.md)). The first writing command, so validate before writing and report exactly what changed.

Done: `--fix` on `check`. `Index.Fix(apply)` computes the okf_version sync (insert when missing, rewrite when drifted), building and validating the frontmatter edit before any write; `check --fix` applies it, re-reads the bundle, and reports each `fixed …` line plus the remaining issues. Other safe repairs (relative-link normalization, 006) will plug into `Fix`.
