---
type: task
title: "table parser: pipes inside inline code, trailing escaped pipe"
status: todo
priority: low
tags: [debt, parser]
---

`parse.Tables` (behind [`wiki table`](../2-query-surface/007-table-extract.md)) splits a row on `|` and unescapes `\|`, but two rare cases are still there:

- A `|` inside an inline-code span in a cell (`` | `a|b` | c | ``) splits as a column separator; GFM treats it as literal. Fix: skip pipes inside backtick spans when splitting a row (without stripping the code text, which is why `maskedLines` is not reused here).
- A cell ending in `\|` with no closing outer pipe (`| a | b\|`) can be nipped by the outer-pipe `TrimSuffix`.

Low importance: both are rare in real dataset tables (one clean, fully-delimited table per file is the norm). Tracked so it is discoverable rather than silently wrong; revisit only if a real bundle hits it.
