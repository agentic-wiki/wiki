---
type: task
title: extract a dataset's markdown table (wiki table)
status: done
priority: low
tags: [feature, query]
---

`wiki table <file> [--format csv|json]` extracts a `type: dataset` entry's single markdown table as structured rows, so querying composes with `duckdb`/`jq`/an LLM without a query DSL of our own. One dataset per file equals one table (the format rule), so the common case is unambiguous; on a multi-table file, list the tables and take a `--n`/`--section` selector rather than silently taking the first. Reuses the csv/json writer in `output`.

(Recaptured after the spec merge: this lived in the old GLOBAL-SPEC §7 "Future" and would have been lost when that file folded into the spec README.)

**Done:** `wiki table <file> [--n N] [--format text|csv|json]`. `parse.Tables` parses GFM tables (fence-aware); `output.Table` renders the dynamic columns (text aligned; csv/tsv via `encoding/csv`; json as an array of header-keyed objects, column order preserved). A lone table needs no flag; several require `--n` and the command lists them rather than guessing. Exit codes: `0` a table, `1` no match (no table at all, or `--n` past the end), `2` ambiguous (several tables, no `--n`) or a real error. No `type` gate, the tool extracts a table from any entry (the dataset = one-table rule is the skill's to enforce, per the separation principle). Tests across parse/output/commands + smoke; PRD, both READMEs, and the agentic-wiki skill updated.

**Known limitations** (acceptable for v1, rare in real dataset tables): a `|` inside an inline-code span in a cell (`` `a|b` ``) is not treated as literal and will mis-split the row; a cell ending in `\|` with no closing outer pipe can be nipped by the outer-pipe trim. An escaped `\|` elsewhere is handled. Tracked as [debt/002](../debt/002-table-pipe-edge-cases.md).
