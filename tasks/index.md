---
okf_version: "0.1"
---

# Wiki CLI — Tasks

Backlog for the `wiki` CLI itself, kept in the format `wiki` implements (dogfood). Open items: `wiki tasks`. Every entry: `wiki list --type task`. Debt only: `wiki list --type task --tag debt`.

## 2 — Query surface
- [ ] [sort entries by timestamp](/2-query-surface/006-sort-by-timestamp.md)
- [ ] [extract a dataset's table (wiki table)](/2-query-surface/007-table-extract.md)

## 3 — Graph & mutation
- [ ] [.wiki cache](/3-graph-and-mutation/004-incremental-cache.md)
- [ ] [scaffold registry (--template / --from)](/3-graph-and-mutation/005-scaffold-registry.md)
- [ ] [spec upgrade / cross-version migration](/3-graph-and-mutation/008-spec-upgrade.md)

## Done
- [x] Windows support: `wiki.exe` is now a target
- [x] [csv/tsv output](/2-query-surface/004-output-formats.md): `--format csv|tsv` in `output.Emit`, reflecting a header + columns from each result's json field tags (list fields like tags join with `; `; commas/tabs quoted via `encoding/csv`). Non-tabular results fall back to text. Now uniform across every command.
- [x] [tags & properties](/2-query-surface/003-tags-and-properties.md): introspection — `wiki tags` (tags in use), `wiki properties` (frontmatter keys), `wiki property <key>` (its values), each with `--counts` / `--sort=name|count` / `--prefix`. Dropped the proposed `tag <name>` (pure duplicate of `list --tag`). Reuses `Filter`; index methods `TagCounts`/`PropertyKeyCounts`/`PropertyValueCounts`.
- [x] 1 — Foundation: discovery, parsers, index + graph, commands (status/list/tasks/unresolved/orphans/check), text+json, tests, full justfile + CI + goreleaser + smoke.
- [x] Bundle model finalized: root = content root (git-style), `bundle` package/type, `okf_version` badge synced by `check`; spec + READMEs modernized.
- [x] [README — install + modernization](/4-release-and-docs/001-readme-and-install.md): curl one-liners, three-layer framing, dogfood roadmap (URL liveness verified at publish).
- [x] [read & outline](/2-query-surface/001-read-and-outline.md): body (frontmatter stripped) + heading hierarchy, with a shared `Resolve` (path or basename).
- [x] [search](/2-query-surface/002-search.md): case-insensitive full-text (frontmatter + body), `--type/--tag/--prefix` + grep-style `--lines`; also fixed flags-after-positional for read/outline.
- [x] [link graph](/3-graph-and-mutation/001-link-graph.md): `links` (outgoing) + `backlinks` (incoming).
- [x] [move](/3-graph-and-mutation/002-move-rename.md): relocate/rename an entry + rewrite every link to it (precise, anchor-preserving, `--dry-run`).
- [x] [init](/3-graph-and-mutation/003-init-scaffold.md): scaffold a fresh check-clean bundle from an embedded starter (`go:embed`); `--force` for non-empty dirs.
- [x] Hardening + coverage pass: fixed a `move` path-traversal (plus escaping link-targets, CRLF frontmatter, link titles, JSON `null`→`[]`).
- [x] [path handling](/2-query-surface/005-path-handling.md): `--root` to operate on a bundle elsewhere (redirects discovery, no chdir) + `--path`→`--prefix`, settling the model: `--root` (which bundle, OS path) / positional subject + `--prefix` filter (bundle paths).
- [x] [check --fix](/3-graph-and-mutation/007-check-fix.md): first writing command, repairs safe conformance drift (today, the root `okf_version` badge), validates before writing, reports each change.
- [x] [consolidate relative links](/3-graph-and-mutation/006-relative-link-lint.md): relative links are valid (OKF) and resolved into the graph at build, so `backlinks`/`orphans` see them; `wiki tidy --links` normalizes them to canonical root-absolute. (`check` no longer flags relative links; broken ones stay errors.)
- [x] slug filenames: `check` warns on a space in a path; spec + skill recommend lowercase-hyphenated names. `wiki` reads `<…>`-wrapped links (so they resolve cleanly, not garbage) but still flags the spaced filename and skips it in `tidy --links`; bare-space and `%20` forms aren't special-cased. `normalizeLink` is the single canonical-link helper.
- [x] bug fix: `move` now rewrites **relative** links (and `<…>`/anchored ones) to a moved file, not just root-absolute. Each indexed link carries its on-disk `Raw` form; move matches by resolved target and rewrites by `Raw`. Was a silent dangling-link bug exposed by relative-links-as-edges.
- [x] [OKF alignment](/conformance/001-okf-alignment.md): removed the reserved-file frontmatter incompatibility (`index.md`/`log.md` are reserved filenames, no `type`, no frontmatter); adopted OKF `timestamp` (validated when present, never required); zero-dep YAML now handles inline comments + block scalars + graceful nesting; `log.md` ISO date-heading lint; reframed `check` as an opt-in lint, not an OKF gate. Net: our bundles are valid OKF and `wiki` accepts any OKF bundle.
- [x] [yaml frontmatter](/debt/001-yaml-frontmatter-subset.md): the subset is expanded + documented and the hand-rolled, zero-dep approach is intentional (superseded by the OKF-alignment YAML work).
- [x] [wiki tidy](/3-graph-and-mutation/009-tidy-command.md): replaced `consolidate` with `tidy` — bare previews every category and writes nothing; `--links` (relative→absolute), `--slug` (rename spaced files + rewrite inbound `<…>` links via `move`), `--all` apply. Non-interactive; no `--dry-run` (the bare command is the preview). Folds in the earlier slugify task: slug = spaces→hyphens (case preserved), collisions skipped & reported.
