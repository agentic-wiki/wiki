---
type: task
title: "promote the core packages to an importable API"
status: in-progress
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

## Progress

**Done: the move and the first deduplication.** `bundle`, `index`, and `parse` are at the module root and an external module has been verified building an index, filtering, and walking the graph against a real bundle. `output` and `wikilink` stayed internal. `--where` parsing moved from `cmd/wiki` into `index.ParseFilter`, which was a defect independent of consumers: the query syntax is part of the query contract, not of the CLI's flag handling.

**Still to decide, deliberately, before any consumer needs it.** The surface below is what a consumer must otherwise reimplement, which is the exact failure the retro records:

- **Generic frontmatter access.** `Entry.fm` is unexported and only `MarshalJSON` reveals it, so reading an arbitrary field means round-tripping through JSON. Needs an accessor pair mirroring `parse.String`/`parse.Strings`.
- **Link resolution.** `normalizeLink` is unexported, so anything rendering a body reimplements how a target becomes a bundle path. This is the rule that had three homes.
- **Frontmatter writes.** `setFrontmatterValue` is unexported, so a consumer writes its own. This is the rule that had two.
- **Checkbox toggling.** Does not exist at all; the format's inline task mechanism has no write primitive.

Each is a *mutation of the engine's public contract*, so each should be designed for consumers in general rather than for the first one that asks.

**Acceptance:** an external Go module can `import "github.com/agentic-wiki/wiki/index"`, build a bundle index, and answer the same questions the CLI does; the CLI is refactored onto the public packages with no behaviour change; the existing test suite passes unchanged.
