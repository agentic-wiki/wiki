
# WikiShell

Backlog for [WikiShell](../README.md), kept as an agentic-wiki bundle and operated with `wiki`.

**Goal:** a web UI over a bundle that reads like a wiki, boards like a kanban, and never becomes a markdown editor.

This board carries the things that outlast a task: the goal, the epics, and where we are in them. Individual tasks are entries, and entries are queried:

```sh
wiki list --where type=task --where status!=done --where status!=archived  # queued
wiki list --where type=task --where status=in-progress                     # being worked
wiki list --where type=task --where status=archived                        # thought through, not queued
wiki list --where type=task --prefix /2-server                             # one epic's tasks
wiki property status --counts                                              # the shape of the work
```

`archived` is a fourth status alongside `todo`, `in-progress` and `done`: a task that has been thought through and deliberately not queued, kept because the thinking is worth more than the ticket. The reasoning lives in the task, including what would change the answer.

Nothing here mirrors a task's state. A task owns its `status`, so the board cannot drift out of sync with one, and a task that is never listed here is still found by the queries above.

## Epics

Each step is finished before the next begins. The order matters, it is not just tidiness: a reader built on top of a board inherits the board's shape, and that is not something you undo later.

**1 — Design** *(settled)*
One index rebuilt on change, no view privileged in the store. Rules stay in the engine and the server answers rather than the browser asking. Config is `[tool.wikiview]` in the bundle's own `wiki.toml`, and all of it is optional.

**2 — Server** *(done)*
A bundle served over HTTP with no UI: the index in memory, a read-only API, and a watcher that follows the files so the index never needs asking. Clients hear a version over SSE and refetch; the version moves only when content actually did.

**3 — Reader** *(current)*
Real routes, the front door at `index.md`, navigation by links, datasets rendered as tables. The reader is the product; the board is a view inside it. The engine's rules travel as data, so the client resolves links by lookup and never learns what a bundle root is.

Tables render; sorting and totalling them is parked. The datasets a bundle holds are small enough to read, so a grid buys little, and the version worth building is a sort handler on the table already on screen rather than a view of its own.

The shell is settled: an icon rail, a collapsible panel, ellipsizing breadcrumbs, and a ⌘K overlay. One layout rather than a preference — the three are affordances that compose, and two selectable layouts would mean every future view built twice.

**4 — Boards** *(done)*
Columns, lanes, drag on pointer events, the card sheet. A view over one folder of the same index, reached from the reader and returning to it. A board is addressed by an id — `/kanban/<id>/<entry path>` — so the whole address is unambiguous and a card can live in it. `root` is the one board every bundle has; the rest are declared in `[[tool.wikiview.board]]`, from the UI or by hand.

Boards and graphs are the only things that write `wiki.toml`, which they do line by line and never by reserializing: the file is the user's, `wiki` reads it too, and a parse-and-write loses the comments and formatting that make it theirs.

**5 — Actions**
Refresh, pull, sync. The first things that reach outside the machine, so each previews before acting, and a failed pull restores the previous state and offers the work as a named branch rather than stranding anyone in a conflicted tree.

**6 — Export**
A folder and everything under it as one document, in reading order, printed by the browser. Rendering stays in the client so the export cannot disagree with the screen about what an entry looks like.

**7 — Graphs** *(done)*
A folder's entries as nodes and the links between them as edges, at `/graph/<id>`. Declared like a board and never built in, filtered by `path` and an optional `where`, with body and frontmatter links both counting as edges. Declared and edited from the UI through the writer boards use, which now takes the kind of table it is editing.

**8 — Design** *(done)*
The reference design (`wikiview-ui-ref/`) applied to every surface: its tokens and type, positional colour, a redrawn shell, reader, board, graph and lists, a side peek, a palette with commands, toasts, and narrow screens last. The look changes and the behaviour does not. The plan and the decisions behind it are in [the reference design, applied](./8-design/001-the-reference-design.md).

## Upstream

Built on [`wiki` v0.9.0+](https://github.com/agentic-wiki/wiki), imported directly. Every rule lives there and none is reimplemented here: frontmatter reads and writes, link resolution both ways, checkbox toggling, `--where` parsing. Config is `[tool.wikiview]` inside the bundle's own `wiki.toml`, read through `bundle.DecodeTool`.

Wanted upstream, not blocking: an incremental rebuild (`.wiki` cache), since a long-lived server rebuilds the whole bundle on every change; and a git-aware `SortTime`, since a clone or pull rewrites mtimes and every entry it touched then reads as changed "just now" ([8-design/007](./8-design/007-entry.md)).

## Releases

Tagged releases build the frontend into the binary, publish archives for macOS, Linux and Windows, and push a Homebrew formula to `agentic-wiki/homebrew-tap`. See [CHANGELOG.md](../CHANGELOG.md).

