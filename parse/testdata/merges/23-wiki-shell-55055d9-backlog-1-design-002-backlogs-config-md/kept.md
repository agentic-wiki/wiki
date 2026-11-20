
Config is **entirely optional**. A stock bundle serves with no configuration at all: the reader needs none, table and grid views need none (the structure is the markdown table), and a board infers its columns from the entries in the folder. Config exists for what inference gets wrong, not for what it gets right.

## Where it lives

`[tool.wikiview]` inside `wiki.toml`, depending on the upstream `[tool.*]` namespace (`wiki` backlog, *reserve a per-tool config namespace in wiki.toml*). Named rather than linked: a relative link out of this bundle would assume a sibling checkout, and `check` is right to flag it.

The alternative is a `wikishell.toml` beside the bundle: two files describing one directory, read by two parsers. What settles it is that the alternative is not *one* extra file, it is one per tool, forever. `pyproject.toml` is the precedent and it works.

**Never a second parser.** `bundle.Bundle` hands the tables over; a TOML reader here would be another rule with two homes.

## Shape

```toml
[[tool.wikiview.board]]
id      = "backlog"
path    = "/backlog"
where   = ["type=task"]                              # default
status  = "status"                                   # default
columns = ["backlog", "todo", "in-progress", "done"] # default: inferred
lane    = "priority"                                 # default: no lanes
```

An **array of tables**, because every setting here is per board: two backlogs in one bundle can legitimately use different status vocabularies, and a flat `backlogs = [...]` cannot carry that.

`id` and `path` are the required keys. `path` says which folder; `id` is the board's identity and its address, so two boards can sit over one folder and be told apart. It is written, never derived from the path, which is what keeps [the first segment of `/kanban/<id>/<entry path>`](../4-boards/002-choosing-boards.md) an id and only an id.

**`where` reuses the `--where` spelling**, parsed by `index.ParseFilter`. This is the reason that parser was extracted from the CLI: the query syntax belongs to the query contract, so a board filter, a CLI flag, and a URL query are one language with one implementation. A bare `type` key would have been a second, weaker filter syntax that grows into the first one anyway.

## The rule that governs the defaults

**No entry in scope may be invisible.** Config pins order and adds empty columns; it never hides cards. Two consequences, and they are not negotiable:

- A status present in the entries but absent from `columns` gets a column appended after the declared ones. Silently dropping it would make a card vanish from the board while still sitting in the folder, which reads as data loss.
- Entries with no status key at all get a column, shown only when non-empty.

This is what keeps `columns` an ordering aid rather than a filter. Filtering is `where`'s job, where it is explicit.

## One board needs no declaring

`root` covers the whole bundle with every default, so a bundle with no config still has a kanban to open. `board` entries add the rest, and adding one is what gives it an address. The UI can still point at candidate folders (a folder whose entries are mostly `type: task` carrying a status key), but as a suggestion to declare, not as a board that already exists.

## Validation belongs here, not upstream

Resolved from the earlier open question, which asked whether `wiki check` should warn when a declared board path does not exist. It cannot, and should not: the whole point of `[tool.*]` is that `wiki` never parses or validates what is inside it. The moment it validated one tool's keys it would hold an opinion about that tool.

So **WikiShell validates its own section** and reports on startup: an unknown key under `[tool.wikiview]`, a `board.path` that does not exist, a `where` expression that does not parse. Same footgun class as an unknown `wiki.toml` key, handled by the tool that understands the keys.

**Unblocked.** The upstream `[tool.*]` namespace shipped in `wiki` v0.9.0, read through `bundle.DecodeTool`. This bundle's own `wiki.toml` already declares a board with it and `wiki check` stays silent, so the shape above is exercised rather than only proposed. What remains here is code that reads it: the API reports which tool tables exist, but nothing decodes the board yet.

**Acceptance:** a bundle with no `[tool.wikiview]` serves every view correctly; a declared board pins its column order and surfaces in navigation; an entry whose status is undeclared still appears; WikiShell reports its own config errors and `wiki check` stays silent about the namespace.

## Done

`internal/config` decodes `[tool.wikiview]` through `bundle.DecodeTool`, fills every default but `id` and `path`, and parses `where` with `index.ParseFilter` so the query spelling keeps one implementation. `/api/bundle` reports the boards that have an address. Startup reports the problems:

```
wiki.toml: unknown key [tool.wikiview] backlogs
wiki.toml: unknown key in board 1: collumns
wiki.toml: board 2 (/notes): id is required
wiki.toml: board /gone: no entries there
wiki.toml: board /: "nonsense" is not a filter: expected key=value or key!=value
```

None of them stops the server, and every board survives its own mistakes: a bad filter on one does not cost the next its defaults.

Catching a misspelled key needs decoding twice, once into `map[string]any` to see what is written and once into the real shape. A typo otherwise decodes into nothing and is ignored in silence, which is the same footgun the engine surfaces for its own keys.

The config is decoded per request rather than held. It follows the same file the index does, so a board declared in `wiki.toml` is there on the next fetch, and the store keeps one concern.

**Decoding is serialised, and has to be.** `bundle.DecodeTool` decodes through a `toml.MetaData` that records what it has read as it goes, so it writes to the very thing it reads from. Two requests decoding one bundle at the same moment is a concurrent map write, which is not a race the Go runtime tolerates — it takes the process down. That is not theoretical: it killed a server with a board open while the tree refetched beside it, on a bundle an agent was editing.

A lock around the two `DecodeTool` calls, and nothing wider: the checks after them walk the index, which is read-only and wants no lock. Worth revisiting if decoding ever shows up as contention, where holding the decoded config per rebuild is the obvious answer — but that trades a decode for cache invalidation, and the crash is fixed either way.

**Left to the board tasks**, since they are about rendering rather than reading: pinning column order on screen, surfacing boards in navigation, and the rule that an entry whose status is undeclared still gets a column.
