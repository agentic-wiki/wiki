# Changelog

All notable changes to `wiki` are documented here. This project follows [semantic versioning](https://semver.org); while pre-1.0, breaking changes bump the minor version.

## v0.9.0 — 2026-08-09

The release that makes `wiki` importable. Everything that makes the tool useful now lives in public packages, so a Go program can build a bundle index and query it directly instead of spawning the CLI and parsing its output. The CLI runs on those same packages, so there is no second implementation of anything.

### Breaking

1. **Package paths.** `bundle`, `index`, and `parse` moved out of `internal/` to the module root. Nothing could import them before, so this breaks no existing code — it is listed because the paths are now a commitment.
2. **A malformed `wiki.toml` is an error** rather than being silently half-read. Input that was never valid TOML but happened to scan — most likely unquoted array items, `types = [note, concept]` — now fails with a line number.
3. **Two json keys renamed**, for consistency across commands: `checkboxes` reports the entry as `entry` (was `file`), and `unresolved` reports a link's far end as `to` (was `target`). csv/tsv headers follow; text output is unchanged.
4. **`Index.OutLinks(*Entry)` is now `Index.Links(path string)`**, and returns one ref per occurrence rather than unique targets.
5. **`Entry.SetFields` takes `map[string]any`** (was `map[string]string`), accepting a `string` or a `[]string` per key.

Two behaviour changes worth knowing that are not API breaks: writing into a read-only directory now fails where a plain write succeeded, and a hardlinked entry now diverges instead of sharing an inode. Both are consequences of atomic writes and both are argued below.

### Changed

- **The core packages are importable.** `bundle`, `index`, and `parse` moved out of `internal/` to the module root, so a Go program can build a bundle index and query it directly instead of spawning the CLI and parsing its output. `output` (CLI presentation) and `wikilink` (a compat shim `index` uses without exposing) stay internal. The CLI is unchanged and still runs on the same packages, so there is no second implementation of anything.

  Shelling out is a fine contract for occasional whole-bundle questions and a poor one for a consumer asking many small ones, since every invocation re-reads and re-parses the whole tree: cost scales with interaction rather than with change. It also forced consumers to reimplement rules that have one correct home here, which is how a separate UI ended up carrying its own frontmatter writer and its own link resolver.

- **`index.ParseFilter` reads a `key=value` / `key!=value` expression.** The spelling is part of the query contract rather than of the CLI's argument handling, so it moved out of `cmd/wiki`; the flag now calls it. Any consumer accepting the same syntax gets the same parse, including the details that are easy to miss (`!=` matched before `=`, so a value may contain `=`; the value unquoted the way frontmatter is).

- **A write API on `Entry`.** `SetField`, `SetFields`, `UnsetField`, and `SetCheckbox` change frontmatter and checkboxes surgically: the matching lines are replaced and every other byte is left alone, never parsing the frontmatter into a map and re-serializing it, which would silently drop nested maps, anchors, comments, and quoting style. `SetFields` applies several keys in one pass, since two writes can leave an entry half-updated. Each refreshes the entry in place, because inserting or removing a line shifts the line numbers that links, checkboxes, and headings all carry. `SetCheckbox` is keyed by line, the only stable identity a checkbox has.

  `SetFieldList` writes list-valued fields (`tags`, `blockers`, and anything else the bundle spells as a list). Separate from `SetField` because a list is not a string that happens to contain brackets: passing `"[a, b]"` to `SetField` writes `key: "[a, b]"`, correctly quoted for a scalar and a one-element list when read back. A key already written as a block list stays one, so the API does not reformat frontmatter it was only asked to change.

  `SetFields` takes `map[string]any`, accepting a `string` or a `[]string` per key and rejecting anything else by name and type. That mirrors `Frontmatter`, which returns the same shape, so a consumer can read the frontmatter, edit it, and write it back. With scalars and lists in separate calls, setting a status and a tag list together took two writes — precisely the half-updated entry one pass exists to prevent. Frontmatter is genuinely heterogeneous, so `map[string]string` was never the honest type for it.

- **json keys are consistent across commands.** There were five names for two concepts: `checkboxes` called the entry `file` while `check` called it `entry`, and `unresolved` called a link's far end `target` while `links` and `backlinks` called it `to`. Now three categories with one name each — `_path` for rows that merge your frontmatter and so need a reserved key (`list`, `orphans`), `entry` for rows *about* an entry (`check`, `checkboxes`), and `from`/`to` for link rows (`links`, `backlinks`, `unresolved`). csv/tsv headers follow, since they derive from the json fields; text output is unchanged. `wiki version` also no longer claims to accept `--format`.

- **`Index.OutLinks(*Entry)` is now `Index.Links(path string)`**, the mirror of `Backlinks(path string)`: same shape, same return type, both yielding `nil` for an unknown path. It also stops de-duplicating by target and returns one `LinkRef` per occurrence, as `Backlinks` always has. De-duplicating while still reporting a `Line` made that line the first of several, silently. Which behaviour is right depends on how the result is shown — `wiki links` prints a bare target so it collapses repeats, `wiki backlinks` prints `file:line` so it shows each one — and that is a presentation choice, so it moved to `cmd/wiki`. CLI output is unchanged.

- **Reading and resolving, for consumers.** `Entry.Field`, `Entry.FieldList`, and `Entry.Frontmatter` reach arbitrary frontmatter; `FieldList` applies the same scalar-as-one-element-list rule matching uses, so filtering by hand agrees with `--where` rather than being subtly different. `Index.ResolveLink` and `RelativeLink` expose both directions of link spelling.

- **`--where` works on the vocabulary commands.** `tags`, `properties`, `property`, and `checkboxes` now take `--where key=value` / `key!=value` with the same semantics `list` has (repeatable, ANDed, list-match-any), composing with `--prefix`.

  `--prefix` scoped every one of these and `--where` scoped none, so a subtree could be narrowed anywhere and a field could not. The gap showed the moment a folder held more than one kind of entry: a backlog that also holds notes reported *their* statuses (`published`, `retired`) beside the tasks', with no way to ask the narrower question short of post-processing `list --format json` through `jq`. Now there are two filters, available wherever a set of entries is narrowed: **`--prefix` for where, `--where` for what.** `checkboxes` is included because its unit is a `- [ ]` line but its *scope* is still a set of entries; a named `[file]` stays explicit and ignores both filters.

  Library signatures gained the parameter: `TagCounts`, `PropertyKeyCounts`, and `PropertyValueCounts` each take `props []PropFilter` alongside the path prefix. `checkboxes` also dropped a second copy of prefix matching and now routes through `Index.Filter` like everything else.

- **`wiki.toml` is parsed as TOML, and `[tool.*]` is reserved.** `bundle` now uses `BurntSushi/toml` (one dependency, no transitive ones) instead of a hand-rolled line scanner, and `[tool.<name>]` tables are space granted to other tools over the same bundle: never parsed by `wiki`, never validated, never warned about. `bundle.Bundle.Tool` carries them and `DecodeTool` unmarshals one into a caller's own struct, so no tool writes a second `wiki.toml` parser.

  Without the namespace, a tool with an opinion about a bundle had to put it in a satellite config beside `wiki.toml`, and a second tool meant a third file. `pyproject.toml` is the precedent: one file describes the directory, tools namespace their own settings inside it. Reserving space adds no opinion to the format — `wiki` gains no field it interprets and no behaviour.

  **Behaviour change:** a malformed `wiki.toml` is now an error instead of being silently half-read. The config decides what counts as an entry and which types are valid, so carrying on with a partial parse produced confidently wrong answers. This also means input that was never valid TOML but happened to work — most likely unquoted array items, `types = [note, concept]` — now fails with a line number instead of being accepted.

### Fixed

- **Docs described the old canonical link form.** The format spec still called root-absolute links canonical and said `tidy --links` rewrote *to* root-absolute; since relative became the canonical on-disk form (v0.7.0) both are backwards. The scaffolded `AGENTS.md` contradicted itself, describing relative links in one section and `tidy --all` producing absolute ones in another, and both READMEs told Obsidian users to write *Absolute path in vault*. The spec also still documented `skip`, a field renamed to `ignore` long ago — so a config copied from the spec silently did nothing and warned as an unknown key — and never documented `ignore_orphans` at all.

- **A quoted list item containing a comma was read as two broken items.** `parse` split a frontmatter flow list on every comma without honouring quotes, so `tags: ["a,b", "c"]` came back as `["\"a", "b\"", "c"]` — valid YAML, silently mis-parsed, in any bundle that spelled a list that way. The split now tracks the open quote.

- **A key inside any `wiki.toml` table silently overrode bundle config.** The line-based reader ignored table headers, so every key was treated as top-level: `[tool.wikiview] types = [...]` replaced the bundle's `types` vocabulary, last-one-wins, and entries with undeclared types then passed `check` clean. Any table containing a key named `spec`, `types`, `ignore`, or `ignore_orphans` reconfigured the bundle from inside a namespace that was supposed to be inert.

- **A multi-line array in `wiki.toml` silently disabled the setting.** `types = [` parsed to an empty list, which means "no vocabulary declared", which allows every type — the exact opposite of what the author wrote, with nothing reported. The one-line spelling of the same vocabulary errored on an undeclared type as intended. Valid TOML that any other tool would read correctly, so nothing suggested it was being misread.

- **Unknown-key warnings name the full path.** They reported the leaf key with its table stripped, so a nested `path` or `columns` was unfindable in a file with several tables. Now `nested.key`, and only the shallowest unrecognized key is reported, since flagging every key inside an unknown table is noise rather than information.

- **Writes are atomic.** Every file the engine rewrites now goes through a temp file and a rename instead of `os.WriteFile`, which opens with `O_TRUNC` and so empties the file before the new content lands. Anything reading in that window — an editor, an agent, a watcher, another `wiki` run — could see an empty or partial entry, and a process killed mid-write left it truncated on disk. Measured against a concurrent reader, roughly 9% of reads saw a torn file before; none do now. Permissions are preserved across the rename, and a symlinked entry is written through rather than replaced. Atomicity is per file: a command rewriting several can still be interrupted between them, which is a separate concern. A hardlinked entry now diverges instead of sharing an inode, which is the point: two names are two entries at two paths, so each needs its own relative links, and the shared inode meant one of them ended up pointing nowhere. Writing into a read-only directory now fails where a plain write succeeded.

  **Not every system allows a replace while a file is open**, and on those the write now fails where the plain write it replaced succeeded. The replace retries briefly, but only while the error is contention, so a genuine permission error still fails at once; that covers the common cause, which is transient and not the user's doing (a scanner or indexer opening a file it just saw change). Contention held longer still fails, and the durable fix is filed as debt rather than rushed. Where a replace does not care who is reading, there is no retry and no cost.

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
