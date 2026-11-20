
The server with no UI: the third step of the sequence, and the first code in this repo.

Scope is deliberately narrow. No watcher, no SSE, no frontend, no writes. Just enough to prove the engine serves a bundle over HTTP and to give the reader something to fetch.

## What it is

- **`internal/store`** — one bundle, one index, rebuilt on demand. `Rebuild` builds fully before swapping, so a failed rebuild leaves the previous index serving: an entry saved mid-edit with broken frontmatter must not take the server down. `View` returns the current index and the version describing it together, immutable until the next rebuild, so a caller reads it lock-free and a request spanning a rebuild sees consistent data.
- **`internal/server`** — `GET /api/bundle` and `GET /api/entry/{path...}`. Entries carry frontmatter verbatim, the unrendered body, and the graph around them with **every link already resolved to a bundle path**, because turning a written link into a path is the engine's rule and a browser doing it would be a second implementation.
- **`cmd/wikishell`** — a positional bundle path defaulting to the working directory, plus `--host` and `--port`.

## The path guard

**A request path is only ever a map key, never a file operation.** `/api/entry/{path...}` is rooted and handed to `index.Resolve`, which for a path containing `/` is a plain lookup in `byPath`. So `..`, absolute paths, and encoded traversal are not dangerous here, they are merely *absent from the map* — the guard is structural rather than a filter that has to be kept correct.

Rooting also matters for a second reason: `Resolve` falls back to a basename scan for an argument with no `/`, which would make `notes` mean "the entry named notes, wherever it lives". A server must not guess.

## Deliberately not yet

- **Rendering.** The body is served as markdown. Where it becomes HTML is the reader's decision and should be made when there is a reader to make it for.
- **Writes.** Checkboxes and frontmatter are next, and they go through the engine's write API, which is atomic and surgical. Nothing here writes.
- **The watcher, digest, and SSE.** Picked from the previous attempt nearly as-is, but a rebuild trigger with nothing subscribed is plumbing before it is required.

**Acceptance:** the module builds against the engine with no reimplemented rules; `wikishell <bundle>` serves both endpoints; the store is safe under concurrent snapshot and rebuild; a failed rebuild keeps the previous index serving; traversal attempts resolve to nothing.

## Done

All of the above, verified against this repo's own backlog: 8 entries, `tools: ["wikiview"]` reported from its `wiki.toml`, links resolved, backlinks correct.

Two things learned building it:

- **`ServeMux` cleans a path before routing, but not a percent-encoded one.** A literal `..` is answered with a redirect and never reaches the handler; `%2e%2e%2f` arrives intact. So the handler *does* see traversal, and it is harmless for the only reason that matters — the path is a map key and there is no such key. That is the guard working structurally rather than by filtering, and it now has a test that proves it by content (a real file outside the bundle that must never appear in a response) rather than by status code.
- **`Resolve` guesses at a bare name**, scanning basenames when the argument has no `/`. Fine for a CLI, wrong for a server, so the path is always rooted before lookup.

**Not verified here: `-race`.** It needs cgo and this environment has no C compiler. The concurrency test exists and passes without the detector; `just test-race` runs it properly where a compiler is available.
