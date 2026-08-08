---
type: task
title: "promote the core packages to an importable API"
status: done
priority: high
tags: [feature, api, architecture]
---

Everything that makes `wiki` useful lives under `internal/`, so Go forbids anyone else importing it. Any other program over a bundle must therefore spawn the CLI and parse its output.

That is a reasonable contract for **occasional, whole-bundle questions**, and it is what `wikanban` was built on. It stops being reasonable the moment a consumer asks *many small* questions: a bundle browser wants an entry, its backlinks, a search, a tag list, a folder tree, and each of those is currently a process spawn that re-reads and re-parses the entire content tree. The cost scales with interaction rather than with change, which is exactly backwards.

The tool is described as "a neutral, deterministic engine that indexes the folder and answers structured queries, **built for agents to call, runnable by anyone**". A Go program is the one caller that currently cannot.

## What to promote

- **`bundle`** — locate a bundle, read `wiki.toml`. Small, stable, already a clean surface.
- **`index`** — build, query (`Filter`, `Search`), the graph (`Links`, `Backlinks`, `Orphans`, `Broken`), and mutation (`Move`, `Check`, `Fix`, `Tidy`).
- **`parse`** — required because `index`'s API exposes its types (`Checkbox`, `Heading`, `Table`).

**Stays internal:** `output` (CLI presentation, not a library concern) and `wikilink` (a compat shim `index` uses without exposing).

Top-level `bundle/`, `index/`, `parse/` rather than a `pkg/` directory, which modern Go does not favour.

## What it buys

- **No second implementations.** `wikanban` currently carries its own surgical frontmatter writer purely because `setFrontmatterValue` is unreachable, and its own link resolver because `normalizeLink` is. Both are deliberate stand-ins, both are drift risks, and both evaporate here. (`wiki set` is still worth having for CLI users, but this removes the pressure behind it.)
- **One index, held in memory.** A consumer builds it once and answers queries from it, instead of paying O(bundle) per question. The `.wiki` incremental cache becomes an optimisation rather than a prerequisite.
- **No runtime dependency.** A Go consumer stops needing the `wiki` binary on `PATH` at all, which removes a whole class of "works on my machine".
- **Stronger correctness than shelling out.** "Cannot disagree with `wiki list`" becomes "is the same code as `wiki list`".

## To decide before building

- **API stability.** Promoting is a commitment. Pre-1.0 this is acceptable with a stated policy, but say it out loud rather than discovering it through a breaking change.
- **What `Index` exposes.** Today `Entry` keeps `fm` and `abs` unexported with accessors; that is a good instinct to preserve. Decide deliberately what a consumer may reach, rather than exporting fields because the move made it easy.
- **`Build` cost and reuse.** A long-lived consumer wants to rebuild incrementally, or at least cheaply. Worth pairing with [.wiki cache](../3-graph-and-mutation/004-incremental-cache.md).
- **The CLI must keep using the same packages**, or the library becomes a second implementation of the thing it was extracted from.

## Done

**The move.** `bundle`, `index`, and `parse` are at the module root; `output` and `wikilink` stayed internal. An external module has been verified building an index, filtering, and walking the graph against a real bundle. `--where` parsing moved from `cmd/wiki` into `index.ParseFilter`, which was a defect independent of consumers: the query syntax is part of the query contract, not of the CLI's flag handling. The CLI keeps its own error wording, since the library cannot know it was reached from a flag.

**The four rules a consumer would otherwise reimplement**, which is the exact failure the retro records:

- **Generic frontmatter access** — `Entry.Field` / `Entry.FieldList` mirroring `parse.String`/`parse.Strings`, plus `Frontmatter()` for the whole map. Reading a field no longer means round-tripping through JSON.
- **Link resolution** — `Index.ResolveLink` (target → bundle path, reporting out-of-bundle) and `RelativeLink` (the inverse). This is the rule that had three homes.
- **Frontmatter writes** — `SetField`, `SetFields`, `UnsetField`. The edit is surgical: it replaces exactly the lines belonging to a key and leaves every other byte alone, never parsing to a struct and re-serializing, which would silently drop what the YAML subset does not model. This is the rule that had two.
- **Checkbox toggling** — `SetCheckbox`, keyed by line because a checkbox's text may repeat within an entry. The format's inline task mechanism had no write primitive at all.

Writes refresh the in-memory entry, so a caller holding an index never reads back what it just overwrote. `BacklinkMap` was added alongside: one pass for the whole graph, where a consumer rendering every entry would otherwise call `Backlinks` per entry.

**Writes became atomic** as part of this, since a library consumer holds the index while other processes read the same files. See the CHANGELOG for the behaviour that changed with it (symlinks, hardlinks, read-only directories).

**Deliberately deferred:** incremental rebuild, which a long-lived consumer will want and which belongs with [.wiki cache](../3-graph-and-mutation/004-incremental-cache.md). `Build` is still whole-bundle.

**Stability:** pre-1.0, so the surface may break; the module version is the only promise. Stated here rather than discovered through a breaking change.

**Acceptance, met:** an external Go module imports `github.com/agentic-wiki/wiki/index`, builds a bundle index, and answers the same questions the CLI does; the CLI runs on the same packages; the test suite passes. Verified as no-behaviour-change by diffing the pre-move binary against the current one — 759 read captures (7 bundles × 23 commands × 4 formats, plus error paths and every `-h`) and 59 mutating cases compared by entire resulting file tree, on both clean and deliberately dirty fixtures.
