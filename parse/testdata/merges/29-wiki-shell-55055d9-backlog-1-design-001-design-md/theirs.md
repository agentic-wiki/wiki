
WikiShell is a web UI over a bundle: a markdown **reader** first, with kanban boards and dataset tables as views onto the same index. It is its own module, importing the engine rather than shelling out to it, so `wiki` stays a zero-dependency neutral engine.

**Nothing gets written until this task is settled.** A UI shaped around *a board over one folder* cannot be retrofitted into a *reader over a whole bundle*: the difference is structural, not cosmetic. Retrofitting it produces a second copy of every rule the two shapes disagree about, and the copies drift.

## What has to be decided here, not discovered later

**The public surface of `index`.** This module needs to read arbitrary frontmatter, filter, walk the graph, and search. Today `Entry` keeps `fm` unexported with no generic accessor, so none of it is reachable. That surface is a hard prerequisite rather than a nicety, and it is designed upstream (wiki backlog, *promote the core packages*). **The bar is that nothing here should ever need to reimplement a rule**; every accessor missing from the engine becomes a second implementation over here.

**Where the config lives.** Two files describing one directory is the wrong answer, and so is a second parser for either. See [where the config lives](./002-backlogs-config.md).

**What owns "the bundle in memory".** One index, rebuilt on change, with every view derived from it. No view may be privileged by living in the store; a bundle has one index and many presentations.

**Where a rule lives when both the engine and the UI need it.** Link resolution, frontmatter writing, and `--where` parsing are each needed on both sides. Each has exactly one correct home, and it is the engine. The browser needs some of them too, which is a third copy unless the server answers the question instead of the client asking it.

**The cost, stated up front.** This module takes `net/http`, a file watcher, and a frontend toolchain. That is the price of keeping the engine clean, and it is paid here rather than there: `wiki` keeps its zero-dependency claim precisely because this repo exists.

## Sequence

Design, then the engine's public surface, then the server with no UI, then the reader, then boards. Each step whole before the next begins.

## Settled

Every question above is answered, so writing can start.

- **The public surface of `index`** shipped upstream: frontmatter reads, link resolution both ways, surgical frontmatter writes, and checkbox toggling. The bar held — none of the four rules this repo would have reimplemented needs a second home.
- **Where the config lives:** `[tool.wikiview]` inside `wiki.toml`, read through `bundle.DecodeTool` so there is no second parser. See [where the config lives](./002-backlogs-config.md).
- **What owns the bundle in memory:** one index, rebuilt on change, swapped in whole. A reader takes a snapshot and reads it lock-free, so a request that started before a rebuild finishes against consistent data. No view lives in the store.
- **Where a rule lives when both sides need it:** the engine, and the server answers rather than the browser asking. Link resolution happens server-side and entries are served with targets already resolved, so the browser never learns how a link becomes a path.

**The cost is now concrete rather than predicted:** this module takes `net/http` and (soon) a file watcher and a frontend toolchain. The engine took exactly one dependency of its own, a TOML parser, and that was to *fix* silent config bugs rather than to serve this repo.
