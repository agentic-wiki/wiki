# Changelog

All notable changes to `wiki` are documented here. This project follows [semantic versioning](https://semver.org); while pre-1.0, breaking changes bump the minor version.

## Unreleased

### Changed

- **The core packages are importable.** `bundle`, `index`, and `parse` moved out of `internal/` to the module root, so a Go program can build a bundle index and query it directly instead of spawning the CLI and parsing its output. `output` (CLI presentation) and `wikilink` (a compat shim `index` uses without exposing) stay internal. The CLI is unchanged and still runs on the same packages, so there is no second implementation of anything.

  Shelling out is a fine contract for occasional whole-bundle questions and a poor one for a consumer asking many small ones, since every invocation re-reads and re-parses the whole tree: cost scales with interaction rather than with change. It also forced consumers to reimplement rules that have one correct home here, which is how a separate UI ended up carrying its own frontmatter writer and its own link resolver.

- **`index.ParseFilter` reads a `key=value` / `key!=value` expression.** The spelling is part of the query contract rather than of the CLI's argument handling, so it moved out of `cmd/wiki`; the flag now calls it. Any consumer accepting the same syntax gets the same parse, including the details that are easy to miss (`!=` matched before `=`, so a value may contain `=`; the value unquoted the way frontmatter is).

### Fixed

- **`move --include-frontmatter` finds relative frontmatter refs.** The flag shipped when root-absolute was still the canonical link form, so it matched frontmatter values by exact string equality against the moved entry's root-absolute path. Once relative links became canonical for bodies (v0.7.0), the natural spelling was the broken one: `blockers: [./task-1.md]` never equalled `/active/task-1.md`, so the flag silently did nothing and the ref dangled. Frontmatter refs are now **resolved** and matched the way body links are, so relative and root-absolute are both found, and they are **normalized to root-absolute** on write. The moved file's own relative refs are normalized too, closing the cross-folder dangle that body links already handled. Anchors are preserved and out-of-bundle values are left as authored. Only values ending in `.md` are treated as references, which is what keeps the pass from rewriting ordinary metadata like `title: Some Note` (an arbitrary string resolves to a valid in-bundle path). The opt-in caveat is unchanged: the flag rewrites every matching value, snapshot fields included.

  **Frontmatter stays root-absolute on purpose**, and does not follow bodies to relative. A root-absolute value is a *stable key*, so every entry referencing `/epics/x.md` spells it identically and `wiki list --where epic=/epics/x.md` finds them all; a relative value spells the same target differently from each directory, so no single `--where` query can match every referrer (matching is exact string equality, by design, so the tool never has to guess that a value is a path). The rendering argument that motivated relative bodies does not apply either, since frontmatter is never rendered as a link. The rule: **a body link is relative because it must navigate; a frontmatter ref is root-absolute because it must be a stable key.**

## v0.8.0

### Changed

- **`org-wiki` starter renamed to `org-base`.** The base holds not just linked knowledge (projects, clients, people) but tabular datasets (invoices, expenses), and "base" (the term the docs already use for a bundle) conveys both, where "wiki" leaned knowledge-only. Scaffold with `wiki init --workflow org-base`.

### Improved

- **`wiki check` flags dead anchors.** A link whose target file exists but whose `#anchor` names no heading there is now a warning, covering markdown links, `[[wikilink#…]]`, and same-page `#anchor` self-links. Fragment and headings are compared by GitHub-style slug, and *both sides* are slugged, so an Obsidian-style `#Real Heading` and a markdown `#real-heading` resolve to the same heading (duplicate headings disambiguated `-1`/`-2` as GitHub does). Warning severity, so exit stays `0`; broken targets aren't double-reported. Obsidian block refs (`#^id`) are left alone. Internally a link now carries a parsed `Anchor` field (retiring ad-hoc raw-string parsing) and pure `#anchor` links are captured off-graph. AGENTS + spec docs updated.
- **Uniform records go in one `dataset`, not one entry each.** AGENTS.md now teaches the counterpart to "one thing per entry": a pile of homogeneous records you total and filter in bulk (invoices, expenses, transactions) belongs in a single `type: dataset` entry as one Markdown table, queried with `wiki table … | duckdb`/`jq`, not as hundreds of graph nodes. The deciding question is whether you want `backlinks` to each one or `SUM`/`GROUP BY` over all of them. The `org-base` workflow gains a **Records and datasets** section with the concrete invoices example and partitioning (`invoices/2026.md` fronted by a typeless `invoices/index.md`), clarifying that a dataset must be a typed file and can never be an `index.md` (which carries no frontmatter); its "rollups are a skill over `list --format json`" note now distinguishes entry-frontmatter rollups from tabular-record rollups. Both also require dataset cells to hold **raw, machine-readable values** from the first row (unformatted numbers `1234.55`, ISO dates, a unit such as currency in its own column), never display-formatted (`1,234.55 USD`), so `wiki table` aggregates in `duckdb`/`jq` with no `gsub`/`tonumber` cleanup; the `org-base` example shows the raw table. `org-base`'s suggested `types` vocabulary adds `dataset`.

## v0.7.0

### Changed

- **Relative links are the canonical on-disk form.** Internal links are now stored relative to the linking file (`../ref/api.md`), so they navigate in any renderer, GitHub/GitLab, a plain editor, Obsidian, not just tools that resolve a bundle-root `/…` (which breaks on GitHub). The internal graph is unchanged (every link still resolves to a root-absolute key), so `backlinks`/`orphans`/`search`/`links` are unaffected. `wiki tidy --links` now normalizes root-absolute links **to** relative (the reverse of before); `wiki move` writes relative links, respelling both the links *to* a moved file (relative from each linking file) and the moved file's **own** outgoing links (relative from its new directory, which previously would have dangled on a cross-folder move); `wiki tidy --wikilinks` emits relative links too. A hand-written root-absolute link still resolves and is never "broken", `tidy` just respells it. Frontmatter path values (opt-in `move --include-frontmatter`) stay root-absolute, as metadata rather than rendered links. AGENTS/spec/scaffold docs updated.

### Improved

- **`wiki.toml` `types` is now an opt-in vocabulary.** Declaring `types = [...]` is optional: with no list (or an empty one) any non-empty `type` is valid, types are free-form, like tags. Declaring a list turns it into an enforced gate, `wiki check` now **errors** (was: only warned) on any entry whose `type` is not in the list, and the message names the fix. This removes the old footgun where an *absent* list warned on every entry, and makes the two type problems consistent (a missing type and an undeclared type are both errors). The scaffold starters ship the suggested vocabulary **commented out**, so a fresh base is free-form until you opt in. `wiki property type --counts` remains the way to see which types are in use.

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
