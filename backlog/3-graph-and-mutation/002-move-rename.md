---
type: task
title: move with link rewrite
status: done
priority: high
tags: [feature, graph]
---

`wiki move <src> <dest>`: relocate a file and rewrite every root-absolute link to it across the bundle. `--dry-run` previews. Validate all writes first; on partial failure, report exactly what changed (no rollback) and let `unresolved` surface leftovers. The keystone safe-refactor that grep/`mv` cannot do.

Builds on the [link graph](/3-graph-and-mutation/001-link-graph.md).
