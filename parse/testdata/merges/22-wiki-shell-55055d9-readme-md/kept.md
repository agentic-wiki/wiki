# WikiShell

[![Release](https://img.shields.io/github/v/release/agentic-wiki/wiki-shell?sort=semver)](https://github.com/agentic-wiki/wiki-shell/releases)
[![Homebrew](https://img.shields.io/badge/brew-agentic--wiki%2Ftap%2Fwikishell-FBB040?logo=homebrew&logoColor=white)](https://github.com/agentic-wiki/homebrew-tap)
[![CI](https://github.com/agentic-wiki/wiki-shell/actions/workflows/ci.yml/badge.svg)](https://github.com/agentic-wiki/wiki-shell/actions/workflows/ci.yml)

> A web UI for [agentic-wiki](https://github.com/agentic-wiki/wiki) bundles.

A wiki bundle is a folder of Markdown that an agent maintains and the `wiki` CLI queries like a database. WikiShell is the screen for it: point it at the folder, read it in a browser, follow the links, tick the checkboxes, and see the same entries as boards and graphs.

![A board of tasks, one of them open in the side panel](img/2-kanban.png)

One binary with the frontend built into it. Your files stay ordinary Markdown.

```sh
brew install agentic-wiki/tap/wikishell
cd my-kb && wikishell      # Open http://localhost:8080
```

## Install

Homebrew, on macOS or Linux:

```sh
brew install agentic-wiki/tap/wikishell
```

Or grab a binary directly:

```sh
# macOS
curl -L https://github.com/agentic-wiki/wiki-shell/releases/latest/download/wikishell_darwin_arm64.tar.gz | tar xz
sudo mv wikishell /usr/local/bin/

# Linux or WSL (amd64)
curl -L https://github.com/agentic-wiki/wiki-shell/releases/latest/download/wikishell_linux_amd64.tar.gz | tar xz
sudo mv wikishell /usr/local/bin/
```

Other platforms (linux/arm64 for a Raspberry Pi, darwin/amd64, windows/amd64, windows/arm64) are on the [releases page](https://github.com/agentic-wiki/wiki-shell/releases).

> [!WARNING]
> **Do not use `go install`.** It produces a binary without the UI in it. The
> frontend is built rather than committed. Use Homebrew or a release binary above, or build from
> source below.

From source, with [bun](https://bun.sh) and a Go toolchain:

```sh
git clone https://github.com/agentic-wiki/wiki-shell && cd wiki-shell
just build          # builds the frontend, then embeds it in the binary
./bin/wikishell ../backlog
```

## Running it

```sh
wikishell [path] [flags]
```

The path defaults to the working directory and walks up looking for `wiki.toml`, so running it inside a bundle needs no arguments. Flags may come before or after the path.

| Flag | Default | |
| --- | --- | --- |
| `--host` | `localhost` | interface to listen on; any non-loopback address (`0.0.0.0` for all) turns the token on |
| `--port` | `8080` | port to listen on |
| `--version` | | print the version and exit |

```sh
wikishell my-kb --port 3000
```

**On localhost, open the address it prints.** Nothing else is needed, and a bookmark keeps working. Behind the scenes, every `/api` call carries an `X-Wikishell-Token` header with any value. A web page on another site cannot add that header, so it cannot write to your wiki through localhost. On a shared machine, every user of it can open your wiki.

**On a network address, open the link it prints.** With `--host 0.0.0.0` or another non-loopback address, WikiShell creates a token at every start and prints it in the link: `http://<this machine>:8080/#t=…`. With `0.0.0.0`, the link names the machine by its hostname. If another machine cannot resolve that name, use this machine's IP in its place. Open the link once: the app keeps the token for that tab and takes it out of the address bar. A tab without it says what to do and shows no entry. The token lasts until the process exits, and a restart prints a new one.

There is no TLS, so the token and every page cross the network in clear text. Use a network address only on a network you trust.

## A quick tour

<table>
  <tr>
    <td width="50%"><img src="img/1-main.png" alt="The reader: the folder tree, an entry, and its heading map"></td>
    <td width="50%"><img src="img/2-kanban.png" alt="A board of tasks, one of them open in the side panel"></td>
  </tr>
  <tr>
    <td><b>Read.</b> The folder as a tree and each entry as a page, with a heading map in the margin and a dot on what changed since you last looked.</td>
    <td><b>Board.</b> Any folder's entries as columns and lanes, from their frontmatter. Drag a card and its file changes; open one and it reads beside the board.</td>
  </tr>
  <tr>
    <td><img src="img/3-multiple-kanbans.png" alt="A board with lanes, and the form that adds another"></td>
    <td><img src="img/4-graph.png" alt="A graph of tasks, coloured by folder, with the groups legend open"></td>
  </tr>
  <tr>
    <td><b>Many views, one folder.</b> Boards and graphs are declared in <code>wiki.toml</code>, and a form writes that for you.</td>
    <td><b>Graph.</b> Entries as nodes and their links as edges, coloured by folder. Hide a group, or show only one.</td>
  </tr>
  <tr>
    <td><img src="img/5-subgraph.png" alt="Pointing at a node lights its links and shows which way they go"></td>
    <td><img src="img/7-read-later-and-git.png" alt="The read later list, with the source control popover open over it"></td>
  </tr>
  <tr>
    <td><b>Follow a thread.</b> Point at a node and its links light, with arrows for which way they go.</td>
    <td><b>Read later.</b> A queue of entries to come back to, in your own order.<br><b>Git.</b> The branch pill says what changed; commit, pull and push from its popover without leaving the page.</td>
  </tr>
</table>

## What you get

Browse the folder tree, follow links between entries, jump to headings. A folder opens its `index.md`, or gets a listing if it has none.

Tick a checkbox and it edits the file: one character, written atomically through the engine's write API. Moving a card writes its status (and its lane, when that changes) the same way, and the board and graph settings write their table in `wiki.toml`. Nothing else is written: prose is never edited.

The screen keeps up with the files. An agent or an editor working on the same folder shows up within a second. Entries that changed since you last opened them get a dot in the tree, so you notice the ones you were not watching.

Images display inline. A link to a contract or a spreadsheet sitting beside the notes about it opens in a new tab.

Callouts are set apart rather than shown as syntax: `> [!warning]`, `> [!success] Shipped`, and any other marker word, in GitHub's spelling or Obsidian's. A word with no colour of its own is still labelled with itself.

Light, dark and system themes, and a reading column you can widen for a big screen. Both are yours rather than the bundle's, and both are applied before the first paint.

Boards are built: columns, lanes, drag by both at once, and a card sheet. So are graphs: the entries of a folder you declare as nodes, the links between them as edges, the way Obsidian draws them. On either, ⌘F (Ctrl+F) goes to the view's own search box, filtering cards or highlighting nodes; press it again there for the browser's find. Escape there clears it. Git is there too: refresh, pull and sync, each showing what it will do before it does it, and a failed pull undoing itself and offering your work as a named branch. Dataset tables are not built yet, and nothing edits prose. Your editor is already open on these files and an agent is writing them at the same time, so a browser textarea would come third. The plan lives in [`backlog/`](backlog/index.md), which is itself a bundle you can serve.

## Configuration

Optional, and it lives in the bundle's own `wiki.toml` under `[tool.wikiview]`, alongside whatever else that file already holds. There is no second config file and nothing to create: a bundle with none of this serves every view.

What it configures is boards and [graphs](#graphs). Every bundle already has one board — `/kanban/root`, the whole bundle — so this is for the others:

```toml
[[tool.wikiview.board]]
id   = "backlog"
path = "/backlog"
```

That is a whole board, reachable at `/kanban/backlog`. You do not have to type it: **New board** in the Boards panel opens a dialog where you pick the folder, the name and which entries become cards, and it writes this table for you.

The id is what the URL carries and what tells two boards apart; every other key has a default, and writing them out is only worth it when one is wrong:

```toml
[[tool.wikiview.board]]
id       = "backlog"
path     = "/backlog"
name     = "Backlog"       # default: the folder, made readable
where    = ["type=task"]   # default; [] is every entry under path
status   = "status"        # default: the frontmatter field the columns come from
columns  = []              # default: column keys inferred from the entries
lane     = ""              # default: no lanes, which is to say one
lanes    = []              # default: the order described below
blockers = "blockers"      # default: the field naming what an entry waits on
```

An array of tables rather than a list of paths, because every board has its own settings — and because **two boards can be over one folder**, which is the reason ids exist. Everything and just bugs, or the same tasks grouped by `priority` beside the same tasks grouped by `area`:

```toml
[[tool.wikiview.board]]
id    = "bugs"
path  = "/backlog"
where = ["type=task", "kind=bug"]
```

`where` follows the same spelling as `wiki list --where status!=done`. A board holds tasks by default, but any filter works: `where = ["type=idea"]` is a board of ideas, and `where = []` is every entry under `path`. That last one is not the same as leaving `where` out, which brings back the `type=task` default.

An id is a word, never a path: it is the first segment of a board's address, and everything after it is an entry, so `/kanban/backlog/3-reader/006-x.md` opens that card on that board. Ids are declared rather than derived, because a derived one would come from the path and then that first segment would sometimes be an id and sometimes a folder name.

**Order comes from the vocabulary first, and from you when you say so.** `status` and `priority` read the way they mean without being configured: `todo, in-progress, done` rather than alphabetical, and `high, medium, low` rather than `high, low, medium`. `severity` and `size` too. A field WikiShell does not recognise falls back to alphabetical, and `columns` or `lanes` replaces the order outright for any field at all. A value nothing in the table mentions still appears, after the ones that do.

**`columns` orders and adds. It never hides.** A status present in your entries but missing from the list still gets a column, appended after the ones you named. Declaring `["todo", "in-progress", "done"]` pins that order and shows `in-progress` while it is still empty, which is the thing inference cannot do for you — but a card whose status you forgot to list appears anyway rather than vanishing off a board while sitting in the folder. Hiding cards is `where`'s job, where it is explicit.

### Graphs

A graph draws a folder's entries and the links between them, reachable at `/graph/<id>`. Unlike boards, none is built in: every graph is declared. **New graph** in the Graphs panel declares one for you, through the same dialog boards use, and a graph's Settings change its name, filter and neighbours without opening this file.

```toml
[[tool.wikiview.graph]]
id         = "people"
path       = "/people"
where      = ["type=person"]  # default: none, every entry under path
neighbours = false            # default
```

`id` and `path` work as they do for a board, but graph ids are their own namespace, so a board and a graph can both be called `people`. `where` has no default: without it, the graph is every entry under `path`.

An edge is any link from one entry on the graph to another: a link in the body, or a frontmatter value naming one (`manager: ./ana.md`, `blockers: [...]`). Several links between the same two entries are one edge, and two entries linking each other are one line. Self-links, links to entries nobody has written, and links to images and other files are not edges. An entry with no edges is still drawn, as a dot on its own.

`neighbours = true` also draws the entries one link away from those, in either direction, even though they fail the filter. They are hollow, since they are context rather than what the graph is about, and no edge is drawn between two of them. A folder's `index.md` is drawn as a solid ring in its group's colour and named after its folder, and its name shows whenever labels are on, not only when it is a hub. A neighbour that is an index stays hollow. A folder's `log.md` is named the same way, as "Folder (log)", and shown under the same rule, but drawn as an ordinary dot.

Drag a node and its neighbours follow; point at one to light up what it touches, with arrowheads saying which way each of its links points; click it to open the entry over the graph. Pan by dragging the background. The wheel zooms by spreading nodes apart: dots and labels stay the same size, so zooming in makes room to read. How big they are is a separate Small, Medium or Large control in the header. Long titles are shortened, and shown whole on the node you point at. The text size is remembered in your browser. So is **Highlight follows preview**, in a graph's Settings and off by default: when it is on, the node open beside the graph keeps the highlight hovering gives it until you close it. It is the one setting there that is not written to `wiki.toml`. Past 500 nodes the graph suggests narrowing `where`, and still draws every node.

## HTTP API

Useful for scripting against a running server. One rule: every `/api` request carries `X-Wikishell-Token`. If the server has a token (network bind), the value must equal it. So on localhost any value works, and on a network address you copy the token from the printed link (the fragment is never sent to the server, so the link itself cannot be used here). On a network address, `/raw` needs the `wikishell_raw_<port>` cookie that every accepted `/api` response sets; the browser does this for you.

```sh
curl -H "X-Wikishell-Token: local" http://localhost:8080/api/bundle
```

```
GET  /api/bundle              the bundle itself: dir, spec, entry count, [tool.*] tables, version, declared boards and graphs, and every tag in first-appearance order
GET  /api/tree                the folder tree, each folder's entries and its index.md if it has one,
                              with when each last changed and how many entries it is
                              linked with, either way (its degree on a graph)
GET  /api/entry/{path...}     one entry: body, frontmatter, checkboxes, and resolved-link
                              and heading-id tables, and when it last changed (its
                              `timestamp`, else the file's mtime, as `wiki` sorts)
GET  /api/board/{id}          one board as columns of cards, in the config's order,
                              with the frontmatter keys its folder uses and the
                              entries under it whose frontmatter couldn't be read
GET  /api/graph/{id}          one graph as nodes and edges, each edge saying which
                              way it points and whether it came from the body or a field
GET  /api/git                 the bundle's repository: branch, upstream, ahead/behind,
                              the commits a pull would take (up to 50, as of the
                              last fetch), and everything a commit would carry
GET  /api/events              server-sent events carrying the current version
GET  /raw/{path...}           a file as it is on disk, frontmatter and all
PUT  /api/checkbox/{path...}  toggle a checkbox, guarded by the version you read
PUT  /api/card/{id}/{path...} move a card to another column and lane, same guard
POST /api/board               declare a board, appending it to the bundle's wiki.toml
PUT  /api/board/{id}          change a board's settings in place
POST /api/graph               declare a graph, the same way
PUT  /api/graph/{id}          change a graph's settings in place
POST /api/refresh             re-read the files
POST /api/git/fetch           ask the remote what it has
POST /api/git/pull            rebase onto the upstream, undoing the attempt if it fails
POST /api/git/sync            commit what is in the bundle, and push
POST /api/git/branch          push the current work to a new branch
```

A write carries the `version` it was read at, and one that has moved is refused with `409` and the current version. `/api/card` takes the values a drop landed on, `{"value": "done", "lane": "high", "version": 7}`, and the board decides which frontmatter keys those stand for. Both are written in one pass, and an empty `lane` leaves that field alone rather than clearing it.

`POST /api/board` takes `{"id": "bugs", "path": "/backlog", "name": "Bugs", "where": ["type=task"]}` and appends a `[[tool.wikiview.board]]` table, leaving the rest of the file alone. Leave `where` out to get the default; send `[]` for every entry under the path. The dialog gets its starting filter and the folder's keys from `GET /api/draft/{board|graph}/{folder}`, so the default is defined once, on the server. It refuses an id that is not a word, one already declared, a filter that does not parse, and a path that is not a folder in the bundle. A folder with no tasks in it yet is fine: the board starts empty and says what will fill it.

`PUT /api/board/{id}` takes `name`, `where`, `status`, `columns`, `lane`, `lanes` and `blockers` together and rewrites those lines in that board's table. A setting sent empty is a key removed, except `where`: an empty one is written as `where = []`, because leaving the key out would bring the default back. `id` and `path` are not settings: they are what the board is, and changing an id breaks every link to it.

`POST /api/graph` and `PUT /api/graph/{id}` do the same for graphs. A graph's settings are `name`, `where` and `neighbours`, and a graph's id is checked only against other graphs. A graph is refused by the same rule, a path that is not a folder; an empty folder is fine. All of these writes edit `wiki.toml` line by line and never reserialize it, so comments, other tools' tables and your formatting survive; a value written across several lines is reported rather than edited around.

`/raw` serves what the index refers to, not what the directory contains: every entry, plus every non-entry an entry links to. A `.env` sitting beside your notes has no key there, so it cannot be requested.

It is also the only way to get an entry's exact bytes. `/api/entry` returns the body with frontmatter stripped and the frontmatter parsed, and you cannot reassemble the original file from those.

Frontmatter is read by the `wiki` library, so each value is what `wiki` shows: the text as YAML reads it (`'company''s'` is `company's`), and a list or map inside a value as one line of YAML (`{path: a.pdf, pages: 40}`). A page whose frontmatter isn't valid YAML, or is over one of `wiki`'s limits, has no fields. Its page says why (`frontmatterError` on `/api/entry` and on its tree entry), and a board lists it under `unread` rather than dropping it, since no column can hold it. A card move onto a header `wiki` can't edit safely is refused with `422` and the reason, and the file is left as it was.

## Relationship to `wiki`

`wiki` is the engine and stays one: a static binary with a single dependency, a neutral index over a folder. WikiShell imports its packages rather than shelling out to the CLI, so a rule like link resolution has exactly one implementation and it lives in the engine.

Install `wiki` too for the terminal side of the same folder: querying, refactoring, `check`, and the skills an agent drives it with.

## Development

```sh
just serve      # build the frontend and serve this repo's own backlog
just check      # vet, lint and tests, Go and UI
just test-all   # unit, UI, and end-to-end
just ui-dev     # the Vite dev server, proxying /api to a running WikiShell
```
