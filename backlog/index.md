---
okf_version: "0.1"
---

# Wiki CLI — Tasks

Backlog for the `wiki` CLI itself, kept in the format `wiki` implements (dogfood). Open items: `wiki tasks`. Every entry: `wiki list --where type=task`. Debt only: `wiki list --where type=task --where tags=debt`.

## 3 — Graph & mutation
- [ ] [.wiki cache](/3-graph-and-mutation/004-incremental-cache.md)
- [ ] [spec upgrade / cross-version migration](/3-graph-and-mutation/008-spec-upgrade.md)
- [ ] [org-wiki + product-docs workflows](/3-graph-and-mutation/010-curated-workflows.md)

## 4 — Release & docs
- [ ] [reframe stack: format + tool + workflow](/4-release-and-docs/003-stack-framing.md)

## Conformance
- [ ] [detect & convert wikilinks](/conformance/003-wikilink-detection.md)

## Debt
- [ ] [table parser: rare `|` edge cases](/debt/002-table-pipe-edge-cases.md)
- [ ] [move: no rollback on a partial write](/debt/004-move-no-rollback.md)

## Done
- [x] [unify filtering under --where](/2-query-surface/009-property-filter.md): one generic `--where key=value` filter on list/search (exact, repeatable=AND, arrays=includes, composite values), **replacing `--type`/`--tag`**; `--prefix` stays for paths; `--format json` now carries every frontmatter field (`Entry.MarshalJSON`; csv/tsv keep canonical columns). Docs + tests swept.
- [x] [ignore/ignore_orphans full globs](/debt/005-ignore-orphans-globs.md): a small zero-dep matcher (`*`, `?`, `**` across segments, `internal/index/glob.go`) now backs **both** `ignore` and `ignore_orphans`; exact single-file patterns still work. Closes the subtree-only limitation from [conformance/006](/conformance/006-orphan-exempt-globs.md).
- [x] [check warns on unknown wiki.toml keys](/conformance/007-unknown-config-keys.md): a typo or a renamed field (the old `skip`) is surfaced as a warning instead of silently ignored.
- [x] workflow-feedback docs pass (from the stress-test LEARNINGS): scaffold `AGENTS.md`/`WORKFLOW.md` use root-absolute links (a fresh init is `tidy`-clean); AGENTS clarifies board-is-authored + `wiki tasks` (checkboxes) vs `list --type task` (entries) + json-as-reporting-surface; project-backlog WORKFLOW warns against folder-per-status (folders now active/backlog/archive; scheduling in board sections), moves queries to `--where`, points deps at real links, bounds reporting to the skill layer; fixed a stale `scaffold.go` doc comment (`generic`→`default`).
- [x] [wiki.toml `ignore_orphans`](/conformance/006-orphan-exempt-globs.md): globs (subtree/exact) whose entries stay indexed but drop out of `wiki orphans`; wired into `project-backlog` (`backlog/**`, `archive/**`), replacing the folder-index workaround. Full-glob support is debt/005.
- [x] [init: operating manual + selectable workflow](/3-graph-and-mutation/005-workflow-scaffold.md): `wiki init --workflow` scaffolds an `AGENTS.md` operating manual + editable `WORKFLOW.md` + a `CLAUDE.md` symlink + minimal `index.md`, `ignore` wired in. `default` shipped; more flavors + the framing rewrite are follow-ups.
- [x] [stale Backlinks doc comment](/debt/003-backlinks-doc-comment.md): removed the leftover leading sentence; the accurate one-`LinkRef`-per-occurrence description stays.
- [x] [wiki.toml `ignore` list](/conformance/005-non-entry-files.md): meta files (e.g. `AGENTS.md`) are excluded from the index entirely (not entries; a link to one still resolves), and out-of-bundle refs (`../PRD.md`) have their advisory silenced. Root-relative, resolved; one list, `bundle` parse + `index` ignoreIn/ignoreOut. Unblocks the scaffolding epic.
- [x] [out-of-bundle links warn, not "broken"](/conformance/004-out-of-bundle-links.md): a link resolving above the bundle root (e.g. `../PRD.md` from a nested bundle) is no longer clamped to a fake in-bundle target and mislabeled broken. `normalizeLink` now decides in/out via `withinDir` (one containment check, shared with `FileExists`); out-of-bundle links get their own `check` advisory (`out-of-bundle link -> …`, warning, exit `0`) and `tidy --links` leaves them untouched. The path-traversal guard is preserved.
- [x] bug fix: `wiki tasks` / `wiki outline` no longer strip inline code spans from checkbox and heading text (they reused the link scanner's masking, which blanks inline code). Task/heading text now keeps `` `code` `` verbatim; only fenced blocks are still skipped. (`parse.maskedLines` + tests)
- [x] bug fix: `wiki init .` tolerates a lone `.git` directory (a fresh repo is a normal init target); only real content still needs `--force`. (`scaffold.Write` + test)
- [x] [Homebrew tap (cross-platform formula)](/4-release-and-docs/002-homebrew-tap.md): `brew install agentic-wiki/tap/wiki` on macOS + Linux from one cross-platform formula, rendered by `scripts/update-formula.sh` and pushed to the tap by CI on each tag. Shipped in v0.3.0.
- [x] [extract a dataset's table (wiki table)](/2-query-surface/007-table-extract.md): `wiki table <file>` renders a dataset's markdown table as text/csv/json (`--n` to pick among several). New `parse.Tables` + `output.Table`.
- [x] [sort entries by timestamp](/2-query-surface/006-sort-by-timestamp.md): `list --sort=path|timestamp` (`--reverse`); newest-first, with the mtime fallback read on demand only for timestamp-less entries.
- [x] [broken links as warning in check](/conformance/002-broken-links-warning.md): `check` demotes broken links from error to warning, so a bundle with not-yet-written links passes (exit 0); only a missing or invalid `type` is an error. `unresolved` stays the to-write surface.
- [x] [normalize exit codes](/2-query-surface/008-exit-codes.md): enumeration/diagnostic commands now exit `0` when empty (like `ls`); only `search` (no match) and `check` (errors) use `1`. Updated CLI + tests + smoke, spec README, and both skills.
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
