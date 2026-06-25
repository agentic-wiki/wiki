---
type: index
title: Wiki CLI — Tasks
okf_version: "0.1"
---

# Wiki CLI — Tasks

Backlog for the `wiki` CLI itself, kept in the format `wiki` implements (dogfood). Open items: `wiki tasks`. Every entry: `wiki list --type task`. Debt only: `wiki list --type task --tag debt`.

## 2 — Query surface
- [ ] [search](/2-query-surface/002-search.md)
- [ ] [tags & properties](/2-query-surface/003-tags-and-properties.md)
- [ ] [csv/tsv output](/2-query-surface/004-output-formats.md)
- [ ] [path handling (-C / --path / positional)](/2-query-surface/005-path-handling.md)

## 3 — Graph & mutation
- [ ] [link graph](/3-graph-and-mutation/001-link-graph.md)
- [ ] [move & rename](/3-graph-and-mutation/002-move-rename.md)
- [ ] [init scaffold](/3-graph-and-mutation/003-init-scaffold.md)
- [ ] [.wiki cache](/3-graph-and-mutation/004-incremental-cache.md)
- [ ] [scaffold registry (--template / --from / skill install)](/3-graph-and-mutation/005-scaffold-registry.md)
- [ ] [lint + normalize non-root-absolute links](/3-graph-and-mutation/006-relative-link-lint.md)
- [ ] [check --fix (repair drift)](/3-graph-and-mutation/007-check-fix.md)
- [ ] [spec upgrade / cross-version migration](/3-graph-and-mutation/008-spec-upgrade.md)

## Debt
- [ ] [yaml frontmatter subset](/debt/001-yaml-frontmatter-subset.md)

## Done
- [x] 1 — Foundation: discovery, parsers, index + graph, commands (status/list/tasks/unresolved/orphans/check), text+json, tests, full justfile + CI + goreleaser + smoke.
- [x] Bundle model finalized: root = content root (git-style), `bundle` package/type, `okf_version` badge synced by `check`; spec + READMEs modernized.
- [x] [README — install + modernization](/4-release-and-docs/001-readme-and-install.md): curl one-liners, three-layer framing, dogfood roadmap (URL liveness verified at publish).
- [x] [read & outline](/2-query-surface/001-read-and-outline.md): body (frontmatter stripped) + heading hierarchy, with a shared `Resolve` (path or basename).
