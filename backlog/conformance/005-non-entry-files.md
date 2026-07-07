---
type: task
title: "wiki.toml ignore list: exempt meta files and acknowledged out-of-bundle links"
status: done
priority: medium
tags: [conformance, config, dx]
---

`init` will drop `AGENTS.md`, `CLAUDE.md`, `WORKFLOW.md` at the bundle root (= the content root). These are operating docs, not knowledge entries: the tool should treat them as non-content and leave them out of the index entirely (a link *to* one still resolves on disk, so nothing breaks). Today `check` would flag each "missing required `type`" and `orphans` would list them (and a `CLAUDE.md` → `AGENTS.md` symlink would index as a second entry of the same content).

Reserved files (`index.md`/`log.md`) are indexed but exempt; `ignore` goes one step further — the listed paths are not indexed at all. Add it as a configurable list (`ignore` matches exact root-relative paths; the OKF `index.md`/`log.md` basename handling stays).

**Decided (design session 2026-07-06):** add an **`ignore`** field to `wiki.toml` naming the files exempt from conformance reporting — indexed like any entry, but skipped by the "missing `type`" error, the orphan report, etc. (`ignore` reads as skip-from-checks; the files are **not** skipped from the walker, so they stay searchable and linkable.) A warning you already know about and won't change is noise; this removes it and doubles as an escape hatch for any meta file (`README.md`, `LICENSE`, ...).

**Decided (2026-07-07):**

- **Exact paths, no globs.** `ignore = ["AGENTS.md", "CLAUDE.md", "WORKFLOW.md"]`. Glob support only if a real need appears.
- **Entries are relative to `wiki.toml` (the bundle root), and the list is unified** — one `ignore`, two effects:
  - a path resolving **inside** the bundle → that file is excluded from the content index (not an entry: absent from `list`/`search`/the graph, no `check` issue; a link to it still resolves on disk, so it is not broken);
  - a path resolving **outside** the bundle (e.g. `../PRD.md`) → any out-of-bundle link whose target resolves to it has its `out-of-bundle link -> …` advisory suppressed (see [out-of-bundle links](/conformance/004-out-of-bundle-links.md)).
- **Root-relative and resolved, not raw-string match:** the same external file is spelled differently from different entries (`../PRD.md` from a root entry vs `../../PRD.md` from a subfolder resolve to the same file). Resolving each link target and each `ignore` entry to a canonical absolute path and comparing collapses the spellings; matching the as-written string would not. Matching stays pure lexical path arithmetic — `wiki` never stats or reads anything outside the bundle (the containment guard holds). In-bundle matches are exact root-relative paths (`/AGENTS.md`), not basename-anywhere.

Touches: `bundle.parseConfig` (read `ignore`); `index.Check`/`index.Orphans`/`index.Broken` (fully exempt listed in-bundle entries, alongside the OKF reserved `index.md`/`log.md`); the out-of-bundle advisory in `Check` (suppress listed targets); and `resolveLinks`/`Entry.Outside` (retain each out-of-bundle link's lexical resolved path to compare — `normalizeLink` already computes it before discarding). The walker (`index.Build`) is unchanged — files stay indexed (searchable/listable). **Needs a spec note** (`wiki.toml` gains an `ignore` field). Unblocks [workflow scaffold](/3-graph-and-mutation/005-workflow-scaffold.md).

**Done (2026-07-07):** `wiki.toml` gains `ignore = [...]` (`bundle.parseConfig` + `Bundle.Ignore`). `index.resolveIgnore` resolves each entry relative to the bundle root into `ignoreIn` (bundle paths) / `ignoreOut` (absolute fs paths). In-bundle `ignore` paths are **excluded from the walk** — not indexed, so absent from `list`/`search`/the graph and raising no `check` issue (a link *to* one still resolves via `FileExists`' disk stat, so it is not broken). Out-of-bundle `ignore` paths suppress the `out-of-bundle link` advisory (`normalizeLink` returns the resolved outside path so `Entry.Outside` carries it, matching every spelling of the same file). Kept as **one** list (intent: "not part of the content graph"; the tool routes by where the path lands). Tests: `bundle` (`TestParseConfigIgnore`), `index` (`TestIgnoreExcludesFromIndex`/`TestIgnoreAbsentStillFlags`/`TestIgnoreOutOfBundleAdvisory`), plus a smoke section; vet + smoke green. Spec updated (`../spec` README). **Follow-up:** wiring `ignore` into `init` scaffolding is [workflow scaffold](/3-graph-and-mutation/005-workflow-scaffold.md).
