# Changelog

All notable changes to `wiki` are documented here. This project follows [semantic versioning](https://semver.org); while pre-1.0, breaking changes bump the minor version.

## v0.6.0

### New

- **`wiki search` matches every word by default (AND).** A multi-word query now matches a line containing *all* of its words (any order, per line), instead of only a contiguous run of the whole query. `--any` broadens to a line with *any* one word (OR); `--exact` matches the verbatim phrase. A single-word query is unchanged. `--any` and `--exact` are mutually exclusive.

### Improved

- **`product-docs` scaffold files by area from day one.** The workflow now pins an initial **area map** (two or three subsystem folders per product) at first run and files pages into it from the first page, instead of starting flat and sorting once the root got crowded (which tended to silt into an unstructured pile). Types are no longer a first-run question: the shipped vocabulary covers the start, extend `wiki.toml` on demand.
- **`product-docs` ingest stages to a git-ignored `raw/`.** Source extraction now lands in `raw/` (git-ignored, regenerable scratch; still indexed as `type: draft` via `ignore_orphans`, so the to-file queue keeps working) instead of a committed `inbox/`, keeping rough drafts out of git history.
- **A folder that is one thing gets a typed hub, not `index.md`.** AGENTS.md and all four starter workflows (`default`, `product-docs`, `org-wiki`, `project-backlog`) front a folder that represents a single thing (a project, product, plugin, or a multi-page guide) with a typed `thing.md` and its parts under `thing/`, instead of a typeless `thing/index.md`, which holds no frontmatter. The distinction is sharpened: a **`thing.md` holds a concept and is load-bearing** (it needs a `type` and a link target), whereas a folder **`index.md` is only the entry point into a collection** of otherwise-independent entries (the bundle root, a grouping folder, a board). The deciding question is which the folder is: one concept with parts, or a bag of separate entries.
- **Similar-but-distinct things stay separate entries.** AGENTS.md now states a "one thing per entry" principle (a `type` and its factory, a `Plugin` and its `PluginSetup` each get their own linked entry), and every "merge duplicates" grooming step (`default`, `org-wiki`, AGENTS.md) is qualified to merge only *true* duplicates, so an over-eager groom no longer folds two adjacent concepts into one. `product-docs` already carried this.
- **`org-wiki` gains a basic per-project board.** Work-tracking now scales in three steps: inline milestone checklists, a bare kanban (a `type: task` entry per item, the project entry as the board with `## Now` / `## Next` links), or a full `project-backlog` bundle. Adds the `task` type. The "Answering everyday questions" section was also trimmed to org-specific queries plus a pointer to AGENTS.md, matching the `product-docs` change.

## v0.5.0

### New

- **Wikilink compatibility (handled, not officially supported).** `wiki` now resolves Obsidian-style `[[wikilinks]]` (with `#anchors`, `|display`, `![[embeds]]`, and `aliases:` frontmatter) into the graph the way Obsidian would, so `backlinks`/`orphans`/`links` see them and a relocation still resolves them by basename. Markdown links remain the format: `wiki check` flags any wikilinks (once per file), and `wiki tidy --wikilinks` converts them to standard root-absolute links (`[[t]]`/`[[t|d]]`/`[[t#h]]`/`![[e]]`), leaving and reporting any that don't resolve.

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
