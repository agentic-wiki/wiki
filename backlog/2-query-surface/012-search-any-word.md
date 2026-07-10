---
type: task
title: "search: match by word, not the whole query as a substring"
status: done
priority: high
tags: [feature, query]
---

`wiki search` lowercased the entire query and did a single `strings.Contains` per line (`index.go` `Search`), so a multi-word query only matched a **contiguous** run: `wiki search "link graph"` missed a line saying "the graph of links". A multi-word query should match by its words, not the literal phrase.

Shipped: tokenize the query on whitespace and match a line by one of three modes, **all-words (AND) the default**. A single-word query behaves identically in all three modes.

Three modes, selected by flag (no flag = all-words):
- **AND (default)**: a line is a hit only if it contains **every** word, in any order.
- **`--any`**: a line is a hit if it contains **any** one word (OR): broadens the net.
- **`--exact`**: match the query **verbatim as one substring** (no tokenizing). The literal-phrase escape hatch: the only way to require a contiguous multi-word run.

Notes:
- Split on whitespace, lowercase each token, drop empties. Each token match is a plain case-insensitive `Contains` (no regex, no word-boundary logic). `--exact` skips the split and tests the whole lowercased query.
- `--any` and `--exact` are **mutually exclusive** (broaden vs narrow are contradictory intents): passing both is a usage error (exit 2). The default AND needs no flag, so there is no `--all`.
- The mode is a query-layer param into `index.Search` (`SearchMode`, zero value = the default `SearchAll`), so the `strings.Contains`-per-line loop stays the single matching site.
- No ranking/scoring change: hits stay sorted by path; a line counts once per hit regardless of how many tokens it holds. `--lines` output shape is unchanged (`path:line: text`); the mode only changes which lines match.
- Swept the usage string, AGENTS, README, main help, and product-docs search wording; `index`/command tests + smoke cover all three modes and the `--any`/`--exact` conflict.

Follow-up: opt-in typo tolerance is [013](/2-query-surface/013-fuzzy-search.md).
