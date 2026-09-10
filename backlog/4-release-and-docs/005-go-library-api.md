---
type: task
title: "public Go library: import the bundle engine, not just the binary"
status: todo
priority: medium
tags: [feature]
---

Today the whole engine lives under `internal/` (`bundle`, `index`, `parse`, `wikilink`), so the only integration path for another Go program is shelling out to the `wiki` binary and parsing `--format json`. That works and is the right contract for agents, but a Go consumer (e.g. a wikanban server) then pays a process spawn per query, depends on `wiki` being on `PATH`, and is exposed to output-format skew.

Raised by wikanban, which would rather import the reader/graph in-process than exec the binary.

## The trap to avoid

Do **not** promote `internal/` wholesale. Those packages are shaped for the CLI's convenience, and a public API is a permanent compatibility contract: the wrong surface frozen early is worse than no surface. `Check`/`Tidy`/`Move`'s current signatures, the `Entry` internals, and the index construction are not the API we want to guarantee.

## Proposal: a curated facade

A single top-level package (e.g. `wiki` or `bundle`) exposing the read + graph surface a consumer actually needs, with the internals staying internal behind it:

```go
b, err := wiki.Open(dir)           // discover + index once
entries := b.List(wiki.Where{...}) // the list query
e := b.Entry(path)                 // frontmatter (verbatim) + body + subtasks
boxes := b.Checkboxes(prefix)      // open items
refs := b.Backlinks(path)          // the graph
```

Design notes:

- **Read + graph first.** Mutations (`Move`, `Tidy`, `check --fix`) are higher-risk to freeze; expose them only once the read surface has proven stable, or keep them CLI-only.
- **A write surface is the highest-value follow-on**, because a downstream editor otherwise reimplements the frontmatter serializer and drifts from `parse`'s reader (the exact risk wikanban hits). A `b.SetFrontmatter(path, key, value)` that reuses the reader's quoting rules would make the round-trip safe. Coordinate with any `wiki set` CLI verb so both share one implementation.
- Keep zero external dependencies and the single-static-binary build intact; this is a re-export boundary, not a rewrite.
- The CLI (`cmd/wiki`) becomes a thin consumer of the same public package, which is a good forcing function for the API's shape.

## Acceptance criteria

- A documented public package with the read + graph surface above, `internal/` kept internal behind it.
- The CLI reimplemented on top of it (no behaviour change; the existing tests and smoke stay green).
- `go doc` clean; a short "use wiki as a library" section in the README.
- A stability note: what is covered by semver, what is still experimental.
