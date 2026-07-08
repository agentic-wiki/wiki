# Changelog

All notable changes to `wiki` are documented here. This project follows [semantic versioning](https://semver.org); while pre-1.0, breaking changes bump the minor version.

## v0.5.0

### Improved

- `check`'s "missing required `type`" error now points at the fix (add a type, or list the file in `wiki.toml` `ignore` if it is not an entry), so a stray `PROMPT.md`/`README` leads somewhere instead of a dead end.

## v0.4.0

### ⚠️ Breaking changes

- **Filtering unified under `--where`; `--type` and `--tag` are removed.** Use `--where type=note` / `--where tags=bug` (repeatable = AND; `type`/`tags` are ordinary frontmatter fields). Applies to `list` and `search`.
- **`wiki tasks` → `wiki checkboxes`.** It scans `- [ ]` checklist items (not `type: task` entries), so the old name misled. `wiki status` likewise renames its count `Tasks:` → `Checkboxes:`.
- **Structured output shape changed** (`--format json` / `csv`):
  - The entry's file path moved from `path` to the reserved key `_path`, and the redundant `name` field was dropped (it is `basename(_path)`). This lets a frontmatter `name:` / `path:` field round-trip untouched.
  - `--format json` now emits frontmatter verbatim (plus `_path`): no field coercion, so `tags` reflects how you wrote it (a scalar stays a scalar) instead of always being a list.
  - CSV/TSV canonical columns are now just `_path,type`; title, tags, and any other fields are JSON-only.

Migration: `--type X` → `--where type=X`, `--tag Y` → `--where tags=Y`, `wiki tasks` → `wiki checkboxes`; JSON/CSV consumers switch `path` → `_path` and stop expecting `name`/`title`/`tags` as CSV columns.

### New

- **`--where key=value` filtering** on `list` / `search`: any frontmatter field, repeatable (AND), arrays match on membership, composite/spaced values work. Also `key!=value` (negation) and `key=` / `key!=` to test emptiness (e.g. unassigned vs. assigned). `--format json` carries each entry's full frontmatter as the reporting surface for rollups.
- **`wiki.toml` `ignore` and `ignore_orphans`** with globs (`*`, `?`, `**`): exclude meta files from the index, and keep chosen subtrees out of `wiki orphans`.
- **`wiki move --include-frontmatter`**: opt-in, also rewrites frontmatter values equal to the moved path.
- **Selectable starter workflows**: `wiki init --workflow <name>` with `default`, `project-backlog`, `org-wiki`, and `product-docs`, each scaffolding an `AGENTS.md` operating manual + an editable `WORKFLOW.md`.
- **`wiki check` warns on unknown `wiki.toml` keys** (catches typos and stale fields).

### Improved

- Leaner `Entry` model (single source of truth: path + type, everything else read from frontmatter on demand), wider test coverage, clearer help text.
