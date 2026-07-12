---
type: task
title: wiki table --links raw|text|path (transform link cells on extraction)
status: todo
priority: medium
tags: [feature, query]
---

A dataset cell that is a real markdown link (`[Acme](../clients/acme.md)`) is a graph edge `wiki backlinks` follows, but `wiki table` exports the *literal* cell, so the column value is `[Acme](../clients/acme.md)`: useless as a `duckdb` grouping key. Today a cell must therefore be a clean value **or** an edge, not both (see the org-base *Records and datasets* note). A transform on extraction dissolves that trade-off: the cell stays a real link on disk, and `wiki table` yields a clean value.

**Proposal:** `wiki table <file> --links raw|text|path`, default `raw`.

- **`raw`** (default): the literal cell text. Today's behavior; lossless, backward-compatible.
- **`text`**: render the cell's markdown to plain text, so `[Acme](x)` becomes `Acme`. Well-defined for any cell (multiple links, mixed prose): strip all link markup, keep display text and surrounding text. Use: a human-readable label/grouping column.
- **`path`**: replace a link with its target resolved to the **root-absolute** key, so `[Acme](../clients/acme.md)` becomes `/clients/acme.md`. Use: a stable foreign key to join on (against another table's path column, or `wiki list --format json`'s `_path`). Because links are stored relative (see [canonical relative links](../3-graph-and-mutation/012-switch-canonical-links.md)), resolving to the canonical `/…` key is what makes the join reliable wherever the dataset sits.

**To design before building:**

- **Non-single-link cells under `path`.** `path` only has a clean meaning when the cell is exactly one link. Rule: transform a cell that is a lone link; leave anything else (multi-link, link-plus-prose, plain text) as raw. Avoids inventing semantics for "two paths in one cell." `text` needs no such carve-out (rendering to plain text is always defined).
- **Interaction with per-row graph noise.** This makes *extraction* clean but does not fix that `backlinks` counts a linked cell once per row; see [backlinks/links per-page granularity](../3-graph-and-mutation/014-backlinks-granularity.md). The two together make per-row cell links viable for small datasets (a dozen contract line-items each pointing at a clause), not thousand-row ledgers.
- **Docs:** if this ships, soften the org-base *Records and datasets* trade-off note to mention `--links` as the escape hatch (don't touch it until built).

**Acceptance:** `--links text` and `--links path` transform lone-link cells (and `text` handles mixed cells), `raw` stays the default and unchanged, `path` leaves non-single-link cells raw and resolves relative links to the root-absolute key. Tests: each mode, mixed/multi-link cells, plain-text cells, all output formats.
