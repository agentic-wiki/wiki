---
type: task
title: extract a dataset's markdown table (wiki table)
status: todo
priority: low
tags: [feature, query]
---

`wiki table <file> [--format csv|json]` extracts a `type: dataset` entry's single markdown table as structured rows, so querying composes with `duckdb`/`jq`/an LLM without a query DSL of our own. One dataset per file equals one table (the format rule), so the common case is unambiguous; on a multi-table file, list the tables and take a `--n`/`--section` selector rather than silently taking the first. Reuses the csv/json writer in `output`.

(Recaptured after the spec merge: this lived in the old GLOBAL-SPEC §7 "Future" and would have been lost when that file folded into the spec README.)
