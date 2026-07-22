---
type: task
title: "list --limit and --skip"
status: todo
tags: [feature, query]
---

`wiki list` (and potentially `wiki search`) should support pagination via `--limit N` and `--skip N`, so a consumer can page through results without pulling the full set into memory or piping to `head`/`tail`.

Target: two new flags on `wiki list`:
- `--limit N`: cap output to the first N entries (after sorting and filtering).
- `--skip N`: skip the first N entries (after sorting and filtering, before limit applies).

Order of operations: filter (`--where`, `--prefix`) -> sort -> skip -> limit.

Implementation notes:
- Apply both flags **after** sorting (the sorted slice is already materialized in `cmdList`; just index into it).
- `--skip` without `--limit` is valid (tail of the list).
- `--limit 0` is a no-op (show all); negative values are an error.
- `--skip` past the end of the list returns empty output (exit 0, consistent with other enumeration commands).
- Text output is trivially truncated; json/csv/tsv output respects the slice naturally.
- Consider backporting the same flags to `wiki search` for consistency (same pattern, hits slice already available).
