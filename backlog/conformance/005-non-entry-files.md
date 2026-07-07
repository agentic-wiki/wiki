---
type: task
title: "wiki.toml field to exempt meta files from conformance reports"
status: todo
priority: medium
tags: [conformance, config, dx]
---

`init` will drop `AGENTS.md`, `CLAUDE.md`, `WORKFLOW.md` at the bundle root (= the content root). These are operating docs, not knowledge entries: they still get **indexed** (searchable, listable, linkable) — they just should not be *reported* against conformance. Today `check` would flag each "missing required `type`" and `orphans` would list them (and a `CLAUDE.md` → `AGENTS.md` symlink shows up as a second entry of the same content, a minor listing dupe).

This is exactly the treatment reserved files already get: `Check` and `Orphans` hardcode-exempt `index.md`/`log.md`. Generalize that into a configurable list.

**Decided (design session 2026-07-06):** add a **`skip`** field to `wiki.toml` naming the files exempt from conformance reporting — indexed like any entry, but skipped by the "missing `type`" error, the orphan report, etc. (`skip` reads as skip-from-checks; the files are **not** skipped from the walker, so they stay searchable and linkable.) A warning you already know about and won't change is noise; this removes it and doubles as an escape hatch for any meta file (`README.md`, `LICENSE`, ...).

**Open decisions:**

- **Match form.** Exact paths vs globs (`*.md`, `docs/*`). Keep minimal if exact paths cover the scaffolded set.
- **Unify with out-of-bundle links?** (open question from the design session.) The same list could also acknowledge out-of-bundle link targets like `../PRD.md` and silence the `out-of-bundle link -> …` advisory (see [out-of-bundle links](/conformance/004-out-of-bundle-links.md)). Two effects from one list: a match on an entry → exempt it from conformance reports; a match on a link target resolving outside the bundle → suppress its advisory. Same "not wiki's concern" intent. Decide: one unified field, or two (e.g. `meta` for files + `external`/`allow` for links).

Touches `index.Check` and `index.Orphans` (consult the list instead of the hardcoded `index.md`/`log.md` names) and `bundle.parseConfig` (read the field). The walker (`index.Build`) is unchanged — files stay indexed. **Needs a spec note** (`wiki.toml` gains a field). Unblocks [workflow scaffold](/3-graph-and-mutation/005-workflow-scaffold.md).
