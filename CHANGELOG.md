# Changelog

All notable changes to `wiki` are documented here. This project follows [semantic versioning](https://semver.org); while pre-1.0, breaking changes bump the minor version.

## Unreleased

### Fixed

- **A `move` whose rename fails changes nothing.** It used to rewrite the links first, so a refused rename (a name the system won't take, a locked file, no permission) left them pointing at a file that never moved. The rename now comes first, and a folder made for it is removed again if it fails. If a rewrite fails after the rename, the other files are still rewritten, and the error names each file left pointing at the old path.

## v0.15.0

### Breaking

- **Frontmatter is read by yaml.v3, the same parser `check` uses.** Values read as YAML reads them:
  - escapes (`''`, `\"`) are decoded, and spaces inside quotes are kept;
  - `>` text is folded, and `|` keeps its final newline;
  - a value continued on the next line is joined with a space;
  - empty list items (`""`, `''`, a bare `-`) are kept;
  - anchors, aliases and `!!str` are applied;
  - a quoted key loses its quotes, and keys are matched by their text, whatever their tag.

  Rule T is unchanged, and dates stay as written.
- **Nested values are read.** `--format json` and `jsonl` give maps as JSON objects and lists as arrays, to any depth. Wherever a value must be text (csv/tsv cells, `property`, `--where`, and the library's `Field` and `Frontmatter`), a list or map is one line of YAML in flow style: `{income: 2025-12-31, balance: 2025-09-30}`. `FieldList` still gives a list's items. `--where` matches a list by any item, and a map by its whole flow text.
- **A list in a csv/tsv cell is one line of YAML** (`[finance, 10-K]`), not JSON. JSON appears only in `--format json` and `jsonl`. Strings that another YAML reader would type (`yes`, `0x1F`, `"10"`) are quoted.
- **`--format json` lists keys in file order, with `_path` first. Nested maps are in file order in every format. jsonl, csv and tsv keep one column order across entries.**
- **A header that isn't valid YAML reads with no fields.** Every command except `check` warns, naming the file (`wiki: warning: /x.md: frontmatter not read (line 3: …); run wiki check`). Writes refuse it.
- **Writes refuse a header wiki can't edit safely** (the library's `Entry.SetField`, `SetFields`, `SetFieldList` and `UnsetField`, and `check --fix`), and `check` reports it. That is a header whose keys aren't each a scalar key at column 0 (a flow-map header, a `?` key). A refusal names the file: `frontmatter of /x.md can't be edited safely: line 3 …`.
- **A write is refused when its result wouldn't read back the same**: when the header would no longer read (an anchor removed while an alias still names it, a limit passed), or when another key would change value through an anchor. Nothing is written.
- **A key written twice refuses a write** with `frontmatter has key "k" twice (lines 2 and 4); …`.
- **`move --include-frontmatter` follows the same rules, file by file.** A file whose rewrite would be refused (a header wiki can't edit safely, a key written twice with a reference in any of its occurrences, a result that wouldn't read back as planned) keeps its frontmatter as it is, with a warning naming the file and the reason, and the move goes on, body links included. A rewritten key written twice used to stop the whole move. A new value is quoted when it needs it (`related: "/x: y.md"`, `spec: "/lib/c.md#usage"`).
- **Library:**
  - `parse.ShapeIssues` is replaced by `parse.CheckFrontmatter`;
  - `Entry.TypedFrontmatter` returns `parse.Map`, with keys in file order, as a deep copy;
  - `parse.Scan`, `Header`, `Field` and `TypedFrontmatter` are removed;
  - a write's refusal is a `*parse.EditError`.
- **`wiki init --shape` replaces `--workflow`.** A wiki is composed from parts:
  - `notes`: what we know, filed by topic;
  - `kanban`: tasks with a status;
  - `records`: one entry per record;
  - `specs`: a product map.

  `--shape specs,kanban` gives both. One part fills the wiki; two or more each get a folder (`notes/`, `backlog/`, `records/`, `product/`), linked from the root `index.md`. `wiki.toml` gets the parts' `types` and `ignore_orphans`. The `default`, `org-base`, `product-docs` and `project-backlog` starters are removed, and without `--shape`, `init` writes the minimal starter: `index.md`, `wiki.toml` and `AGENTS.md`, with `CLAUDE.md` and `.gitignore`. `WORKFLOW.md` is no longer written: each part's section is in `AGENTS.md`, and `CLAUDE.md` is one line, `@AGENTS.md`. A `WORKFLOW.md` from an earlier `init` is left in place: fold what you still need into `AGENTS.md`, then delete it.

### Added

- `parse.ReadFrontmatter` (both views from one decode).
- `parse.Body` (the body, without a decode).
- `parse.Flow` (one line of YAML for a value).
- `parse.Map`, with `Get`, `Clone` and `MarshalJSON`.
- `parse.Empty`, the one "no value" test.
- `parse.CheckFrontmatter` (the syntax error, then the shape issues).
- `parse.EditError` (why a write refused, with the file's `Path`).
- `parse.RewriteValues` (respell frontmatter values item by item, asking a callback about every value of every occurrence, under the same checks as `SetField`).
- `index.Entry.FrontmatterErr` (why a header wasn't read).
- `index.MoveResult.Warnings` (the files whose frontmatter a move left as it is, and why; not in the json result).

### Changed

- **`check` accepts nested maps and lists of maps.** It still reports a key that is a list or an object. It now also reports merge keys (`<<`), keys written with escapes, and headers wiki can't edit safely. It no longer reports a value continued on the next line.
- **`--where k=` matches `k: ~` and `k: null`.** One test of "no value" now serves `--where k=`, empty csv cells and `property`: absent, null, `""`, `[]`, `{}`, or a list of empty items. `property` skips empty items.
- A duplicate key is now found by its text alone, at any depth: `status` and `!x status` are the same key.
- A header over 1 MiB, or one whose read expands to more than 10,000 values or 1 MiB of text (through aliases or flow text), is refused by both `check` and the read.
- A value holding `?` is written quoted, by `SetField`/`SetFieldList` and in flow text: yaml.v3 can't read `[Why?, b]`.
- Writes (`SetField`, `UnsetField`) and `move` edit a header whose value continues on an unindented line, as YAML reads it, instead of refusing; `check` no longer reports it.
- A key such as `a:b` can be written.
- A key that YAML wouldn't read back as itself can't be written: one starting with a YAML indicator (`[`, `{`, `*`, `'`, `%`, `@`, `|`), and `~`, `null` or `<<`.
- `check --fix` names the file when it can't edit the root `index.md`.
- **The scaffolded `AGENTS.md` teaches the practice in every wiki**: a setting-up block to work through with the user and then delete, the model, structure (`index.md`, `thing.md` beside `thing/`), links, work, data (records and datasets), capture, health and the everyday commands, then one short section per part. The kanban part's wikiview board view is in `wiki.toml`, commented out. A test runs every command the scaffolded docs show, so they can't drift from the CLI.

### Fixed

- `'company''s'` reads `company's`, and `'text '` keeps its space (71 pages in the reporting team's wiki).
- `Entry.SetField` on a quoted key (`"my key": v`) replaces it instead of adding a second key.
- `move --include-frontmatter` no longer rewrites a path in a trailing comment.

## v0.14.0

### Breaking

- **`list --format json` and `orphans --format json` give every unquoted number, boolean and null as a JSON value** (`"1"` → `1`, `"true"` → `true`, `"null"`/`"~"`/`""` → `null`). Numbers are byte for byte as written (`1.10` stays `1.10`). Quoted values, dates and YAML spellings that aren't JSON numbers (`007`, `+1`, `.5`) stay strings. `--where` still compares text: `--where n=10` matches `n: 10` and `n: '10'`, and `--where key=` doesn't match `key: null` (filter nulls in your query instead).
- **`wiki check` checks the shape of frontmatter values.** A nested object, a list of objects, a list inside a list, a value or a list item that continues on the next line, and a value with no key, or a key that is a list or an object, are each reported as an error, next to the entry's other issues (`line 4: "m" holds a nested object; …`). Frontmatter values are scalars or lists of scalars. `check --fix` still repairs what it can.
- **`csv` and `tsv` give each command's rows, and so does the new `jsonl`.**
  - A list result is one row per item: `outline` its headings, `move` its rewritten files, `check --fix` its fixes and then its issues, `search --lines` its matching lines.
  - `status`, `init` and `read` give a header and one row (`read`'s body is one multi-line cell).
  - csv and tsv no longer fall back to the text output. `json` and `text` are unchanged.
  - The README lists each command's columns.
- **`table --format json` (and `jsonl`) types its cells.** `12.5` is a number, `true`/`false` are booleans, and an empty cell, `~` or `null` is `null` (an empty cell used to be `""`). A cell in quotes is a string without them, unless the same quote also appears inside it. text, csv and tsv print cells exactly as written, as before.

### Added

- **`--format jsonl` on every command that has `--format`.** One compact JSON object per row (the same rows as csv, not the json elements), with the row's own keys and its values in their JSON types; nothing at all for an empty result.
- **Library: typed frontmatter.** `parse.TypedFrontmatter` returns the frontmatter with YAML types: an unquoted number is a `json.Number` as written, `true`/`false` a bool, `~`/`null`/an empty value nil, and a quoted value a string. `parse.Frontmatter` reads the same pass and gives the same strings as before.
- **Library: `Entry.TypedFrontmatter`** returns a copy of an entry's typed frontmatter, like `Entry.Frontmatter`.
- **Library: `parse.Scan`** reads a frontmatter header into its keys and the lines each key owns (`parse.Header`, `parse.Field`; `Field.Values` returns one occurrence's values, as `parse.Strings` does for the whole header). `Frontmatter`, `TypedFrontmatter` and `ValidateFrontmatter` read through it.
- **Library: `parse.SetField`, `parse.SetFieldList` and `parse.UnsetField`** edit frontmatter text the way `Entry.SetField` does: they replace or remove the lines a key owns (see `parse.Scan`), and refuse an invalid key, an unterminated header and a key that appears twice.
- **Library: `parse.ShapeIssues`** reports frontmatter values that are not scalars or lists of scalars, one error per key, as `wiki check` does.
- **Library: `parse.Table.TypedRows`** types a table's cells by the same rule as `parse.TypedFrontmatter`.

### Changed

- **`list --format csv|tsv` and `orphans --format csv|tsv` add one column per frontmatter key after `_path,type`; `--fields` chooses and orders them.** Without `--fields`, the columns are `_path`, `type`, then every other key any listed entry uses, in byte order. With `--fields a,b,c`, they are `_path,a,b,c`. A list is a cell of JSON text (`[1,"a"]`), and null, an empty list or a missing key is an empty cell. A `--fields` key no listed entry has is still a column, with a warning on stderr.
- **`search --lines --format csv` gives one row per matching line** (`_path,type,line,text`). It used to print Go syntax in a cell.
- **`move --format json` always gives `frontmatter_refs`, including 0.** It used to leave the key out when no frontmatter value was rewritten.
- **A block-list item that is `- ""` or only a comment (`- # c`) is dropped**, as an empty flow-list item (`[a, , b]`) already was. It used to give `""`.
- **A block list whose items are all empty gives an empty list**, as `key: [""]` does. It used to give `""` (bare `-` items) or `[""]` (`- ""` items).
- **An empty list is `[]` in JSON, not `null`** (`key: []`, `key: [""]`, in `list --format json` and wikiview's entry frontmatter). In Go it is an empty `[]string` instead of a nil one.
- **A block list continues past a blank line or a comment at column 0, as in YAML.** `k:` / `  - a` / (blank) / `  - b` reads `["a","b"]`. It used to read `["a"]`.
- **In a block list, a line that isn't an item continues the item before it**, joined with a newline: `k:` / `  - a long` / `    value` reads `["a long\nvalue"]`. It used to end the list. `wiki check` reports it.
- **A nested map, and a value that continues on the next line, read as a multi-line string**: the lines the key owns, de-indented and joined with newlines.
  - `m:` / `  a: 1` / `  b: x` gives `"a: 1\nb: x"`; it used to give `""`.
  - `k: a long` / `  value` gives `"a long\nvalue"`; it used to give `"a long"`.
  - `k: [a,` / `  b]` gives `"[a,\nb]"`; it used to give `["a"]`.

  `wiki check` reports each of them. A flow map (`{a: 1}`) still reads as its text.
- **A `|` or `>` text keeps its lines' indentation relative to the least-indented line.** It used to trim every line.
- **A line with nothing before its colon (`: x`) is no longer read under the key `""`.** `wiki check` reports it.
- **A line whose key contains ` #` (`a #b: x`) is a comment, as in YAML.** It used to be read under the key `a #b`.
- **Writing a key that wouldn't read back as the same key is an error**: `key " k" can't be written: it would not read back as the same key (a key can't start with a space, "-" or "#", or contain ":" or " #")`. A key starting with `_` is still refused as reserved.
- **Writing to a key that appears twice in the frontmatter is an error**, and so is `move --include-frontmatter` when a key it would rewrite appears twice: `key "k" appears twice in frontmatter (lines 3 and 7); fix the file first (wiki check reports it)`. Nothing is written, dry run included.
- **Removing a field from a file whose header has no closing `---` is an error**; it used to do nothing.
- **Writing a field keeps its type and its comment.**
  - A number or boolean replacing a value of the same type, or a null placeholder (`key:`, `~`, `null`, `[]`), is written bare, for scalars and for each list item: `n: 10` set to `11` gives `n: 11`, and `r: [0.35, 0.36]` set to `0.35`, `0.37` gives `r: [0.35, 0.37]`. They used to be quoted (`n: "11"`), which turned them into strings in `list --format json`. A list's type is the type its items share.
  - The inline comment on the key's line is kept: `n: 10 # ten` set to `11` gives `n: 11 # ten`. It used to be dropped.
  - A value replacing a quoted one keeps its quote character whenever the value fits in it, even when the value would be quoted anyway (`1.1`), and when a comment follows: `v: '1.0'` set to `1.1` gives `v: '1.1'`, and `v: 'abc' # note` set to `xyz` gives `v: 'xyz' # note`. A value that doesn't fit (`it's` in single quotes) is written in double quotes.

### Fixed

- **An empty `-` item in a block list no longer ends the list and drops the items after it.** `l:` / `  - a` / `  -` / `  - b` gives `["a","b"]`; it used to give `["a"]`.
- **An unknown `--format` is an error on every command.** `--format xml` used to print text and exit 0; it now exits 2 with `invalid value "xml" for flag -format: unknown --format "xml" (use text|json|jsonl|csv|tsv)`.
- **Setting or removing a field replaces or removes every line the field owns**: block-list items (including a bare `-`, items under a commented key, and items after a blank line or a comment line), `|`/`>` text lines, nested-map lines, and continuation lines. Each of these used to be left in the file, usually as invalid YAML.
- **A quote opens only at the start of a value or of a list item, as in YAML.** `title: Don't panic # draft` reads `"Don't panic"`, and `[it's, b]` reads `["it's","b"]`; the `'` used to open a quote, so the comment, or the rest of the list, was read as part of the value.

## v0.13.0

### Fixed

- **`wiki check` rejects malformed YAML frontmatter.** Unclosed quotes, unfinished lists, repeated scalar mapping keys, non-mapping headers, and missing closing fences now produce per-file errors and exit 1. Validation preserves the existing metadata interpretation and file contents; `check --fix` leaves malformed headers for the author to repair.

## v0.12.0
### Added

- **`wiki dump` explains what leaving logs out did.** On stderr, after the dump, exit still 0:
  - every link to a log left out for want of `--logs` (markdown or `[[wikilink]]`, one line per occurrence), since its root-absolute target names an entry the dump does not hold: `wiki: warning: /z.md:4 link to a left-out log -> /proj/log.md (--logs includes it)`. A log the filters exclude anyway is not warned about, since `--logs` would not bring it back;
  - a dump left empty only because every match was a log (`wiki dump --prefix /proj/log.md`), which used to print nothing at all: `wiki: warning: only log.md entries matched (1), left out without --logs`.

## v0.11.0 — 2026-10-19

### Changed

- **`wiki dump` leaves `log.md` out; `--logs` brings it back.** A folder's log is the history of how it changed, not what it says, and an agent reading `llms-full.txt` wants the latter. Every `log.md` is now left out by default, even one `--prefix` names directly (`wiki dump --prefix /proj/log.md` prints nothing); `--logs` includes them where reading order puts them, composing with `--prefix`, `--where` and `--frontmatter`. A log left out is not published, so its out-of-bundle links are no longer warned about. A link *to* a log is still made root-absolute, as for any entry a filter leaves out, so it names an entry the dump does not hold.

  Library: `Entry.IsLog` (exact base name `log.md`, case-sensitive), now the one test behind `dump`, `check` and `orphans`.

## v0.10.0 — 2026-10-14

The release that makes a base publishable in one file. `wiki dump` prints every entry as a single markdown stream in the order its indexes give, with links made root-absolute so they still resolve once the files are concatenated: the whole base at one URL for an agent to read (`llms-full.txt`). Nothing on disk changes, and the same source always dumps the same bytes.

No API breaks: the library only gains functions. Two behaviour changes worth knowing: `wiki --root <dir> init` now fails instead of ignoring `--root` and scaffolding into the current directory, and a link's `text` in `links` / `backlinks` json now keeps the inline code written in it. Both are argued below.

### Added

- **`wiki dump`: the whole base as one markdown stream**, for publishing at a single URL for agents (`wiki dump > llms-full.txt`; on llmstxt.org `llms.txt` is the index and `llms-full.txt` the full text, and this produces the latter). Each entry is headed by `<!-- /its/path.md -->` and they come in **reading order**: each folder's `index.md`, then the children it links in the order it links them, then the rest by path, a subfolder emitted whole where it lands. Links decide order, never inclusion, so an entry nothing links to is still there, and a bundle without indexes dumps in path order. `--prefix` / `--where` choose what is included, while the order is always that of the whole bundle; `--frontmatter` keeps frontmatter, which is stripped by default.

  It is read-only and transforms only what it prints. Relative links, the on-disk canon, mean nothing once files are concatenated, so internal links become root-absolute (anchors kept) and resolved `[[wikilinks]]` become markdown links to the same path. URLs, out-of-bundle links and unresolvable wikilinks are left as written; out-of-bundle links are also listed on stderr, minus those acknowledged in `ignore`. The bytes depend only on the source: `\r\n` becomes `\n`, a UTF-8 BOM is dropped, and blank lines between entries are exact.

  Library: `Index.ReadingOrder`, `Index.Dump`, `Index.OutOfBundle` (also what `check` now reports out-of-bundle links from), and `parse.MapInternalLinks`, which rewrites link targets in place through the same scanner `parse.Links` uses.

### Changed

- **`--root` works anywhere on the command line.** It was accepted only before the command, so `wiki dump --root kb` failed with "flag provided but not defined: -root", the natural spelling for anyone used to per-command flags. It is now taken out wherever it appears (up to a `--`), in every spelling the flag package accepts (`--root x`, `--root=x`, `-root x`). `init` refuses it in either position: it makes a bundle rather than finding one, and ignoring the flag, as the leading form used to, scaffolded into the current directory. Outside a bundle, the error now also says how to point at one.
- **A link's text keeps the inline code in it.** ``[`wiki` docs](…)`` reported its text as `" docs"`, since inline code was deleted from the line before matching; it is now blanked in place only to find links, and the text is read as written. Visible as the `text` field of `links` / `backlinks` json.

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
