
The Boards list in the rail, and how a folder gets onto it.

**A board is declared, and declaring it is what gives it an address.** `[[tool.wikiview.board]]` decides which boards exist beyond the built-in `root`, because an address needs an id and an id is not something a folder has. So this task owns the moment a folder becomes a board — which is now a config write rather than a navigation.

## `id`, and what it unlocks

Today `path` is the board's identity, which means one board per folder. That forbids the thing people will reach for first: **two views of the same backlog**, one showing everything and one filtered to bugs, or a board grouped by `priority` beside the same folder grouped by `area`. The config can express the settings; it cannot express two of them over one path.

So a board gets an `id`:

```toml
[[tool.wikiview.board]]
id    = "bugs"
path  = "/backlog"
where = ["type=task", "kind=bug"]
```

**Required, and never derived.** A board without an id is reported and unreachable. Deriving one from the path is the tempting shortcut and the thing that breaks the whole scheme: the first segment of a board address would then be an id sometimes and a folder name other times, which is the ambiguity the id exists to remove.

**An id is a word, not a path.** No slashes, so it is exactly one segment — which is what lets everything after it be an entry path, slashes and all:

```
/kanban/bugs                          the board
/kanban/bugs/backlog/x.md             a card on it
```

No query, no separator to invent, nothing to guess: the address splits at the first slash and both halves are unambiguous.

**One board needs no declaring.** `root` covers the whole bundle and always exists, so an unconfigured bundle still has a kanban to open. A declared board may claim that id, and then it is simply that board.

**Everything else is declared.** This replaces "any folder boards by URL whether listed or not", which is what made the address ambiguous: an undeclared folder has no id, so `/kanban/a/b` would have meant either the board `a` showing `/b` or the folder `/a/b`. Boarding a folder now means giving it an id, which is what the creation flow below is for.

**A name suggests an id.** "Team bugs" proposes `team-bugs`, checked for collisions against the ids already declared, and editable before it is written. A suggestion at the moment of creation, where a person can see it — never a rule that runs later against a path.

**Nothing else changes.** `path` still says which folder; `id` only says which of the boards over it. A bundle that never declares two boards over one folder never sees the difference.

## Adding one

Two ways in, because they suit different moments:

- **From the folder you are looking at** — a "make this a board" action, which proposes an id from the folder's name and writes it.
- **From a picker** over the same tree the panel already shows, for choosing a folder you are not currently in.

Both write config, because both create something with an address. That is a heavier action than the earlier design intended — "just trying it" was a navigation — and the cost of unambiguous addresses. Trying a folder out is still possible without committing: `root` boards everything, and `where` narrows it.

**With no boards declared, the panel is where this starts.** Rather than a note explaining that folders board by URL, it offers to make the first one: pick a folder, and that is the whole interaction. The empty state of a feature is the one moment somebody is definitely willing to be told how it works.

Adding a board to the *list* is a config write: appending a `[[tool.wikiview.board]]` table to the bundle's `wiki.toml`. That is a real edit to a file the user owns and `wiki` also reads, so it is explicit, never implicit in having visited a folder.

**The engine will not help write it.** `bundle.DecodeTool` reads the namespace; nothing writes it, and `wiki`'s frontmatter write API is for entries, not for `wiki.toml`. So this needs a TOML writer here — the one place a second config implementation is unavoidable. Worth checking whether it can be narrow enough to be obviously correct: appending a table, not rewriting the file.

**Candidates can be suggested.** A folder whose entries are mostly `type: task` carrying a status key is probably a backlog, and offering it is cheaper than making the user hunt. A suggestion, never an automatic entry in the config.

## The board you were last on

Remembered locally, under this bundle's own key ([UI state scoped per bundle](../3-reader/002-per-bundle-ui-state.md)), because it is one person's view preference rather than something everyone opening the folder shares. The list of boards is never remembered: it comes from `wiki.toml` on every load, where it is versioned with the files.

A stored board that no longer resolves is a small problem. The reader is the default view — `/` opens `index.md`, never a board — so the stored value is never a startup route, only what the rail preselects. When it does not resolve:

- **Drop it and preselect nothing.** Silently: the folder was renamed or deleted by whoever did it, and a dialog about a view preference is noise.
- **Do not fall through to another board.** Picking "the next one" lands someone on a board they did not ask for, which is worse than landing nowhere.
- **Do not follow a rename.** `wiki move` could have relocated the folder, and matching it up would be a heuristic that is wrong exactly when it matters.

Nothing is lost by forgetting: the board is still in the rail, and `root` still boards everything.

## Validation belongs here

`wiki` never parses inside `[tool.*]` — that is the point of reserving it — so it cannot warn about a board path that does not exist or a `where` expression that does not parse. wikiview validates its own section and reports on startup.

**Acceptance:** declared boards appear in the rail under their names; making a folder a board proposes an id, checks it against the ones already taken, and appends a table to `wiki.toml` without disturbing the rest of the file; a bad path, id or filter is reported rather than silently ignored.

## Done

`POST /api/board` appends a `[[tool.wikiview.board]]` table, and `config.Declare` is the writer. Appended rather than rewritten: the file is the user's, `wiki` reads it too, and it holds comments, formatting and keys this package has never heard of, none of which survives a parse-and-reserialize. Written through a temp file and a rename, keeping the permissions, so nothing ever reads a half-written config.

Only `id`, `path` and `name`. Everything else has a default, and writing the defaults out is a config file full of settings nobody chose.

A name is a TOML basic string with two escapes, and anything needing more is refused rather than escaped: a control character in a board name is a paste accident, and carrying an escape table for it would be carrying a second TOML implementation.

**The empty state is the form.** The Boards panel with nothing declared shows the folder picker rather than a paragraph about what to hand-write, which is the step it can take for you. The same form appears on a board with no cards, under the reason it has none — `root` exists without any config, and in a bundle of notes it matches nothing, which is where a first-time reader lands.

~~**A board that would be empty is refused**~~ (reversed 2026-10-01, see below), asked with the board's own defaults so the answer is the one the board would give. An empty page is the hardest thing for whoever declared their first board to debug, and `no cards under /notes` says the thing the page could not.

**A config change moves the version.** It did not before: the digest covered entries only, so declaring a board — or hand-editing `wiki.toml` — rebuilt the index and told nobody, and the change reached whoever reloaded next. wiki.toml is hashed into the digest now, so the stream carries it like any other change and nothing here needs its own refresh path.

**Not here:** a "make this a board" action in the reader, and suggesting candidate folders. Both are ways to reach this form rather than things it cannot do, and the panel already reaches it from the state where it is needed.

## Revised after use (2026-10-01)

- **A board over a folder with no tasks yet is allowed.** The user hit the refusal setting up `/backlog/tasks` before writing any tasks, which is the ordinary way to start a board. The filter editor already says as much: a filter is often written before the entries catch up. What is refused now is a path that is not a folder in the bundle (missing, or a file), which is a typo far more often than a plan. The empty-page worry this rule answered is handled by the page itself. A declared board with no cards says "No cards yet" and what would make one (an entry under its folder with `type: task` and its status field). The undeclared `root` board a fresh bundle lands on still offers the form to point a board elsewhere. Graphs follow the same rule ([7-graphs/002](../7-graphs/002-declaring-graphs.md)).
