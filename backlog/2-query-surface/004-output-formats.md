---
type: task
title: csv & tsv output
status: done
priority: low
tags: [feature, output]
---

Add `csv` and `tsv` to `output.Emit` alongside text/json: a header row with column names matching the json fields. Makes the tabular commands (`list`, `tasks`, `property`) awk/cut/spreadsheet-friendly.

Done: `output.Emit` gained `csv`/`tsv`. It reflects over the result slice, deriving the header and columns from each element's json field tags (`json:"-"` and unexported fields skipped; a list field such as `tags` joins with `; `; `encoding/csv` quotes embedded commas/tabs/newlines). Empty typed slices still print the header; a slice of scalars becomes a single `value` column; non-slice (non-tabular) results fall back to the text lines, so `--format csv` never emits nothing. Zero new dependencies (stdlib `encoding/csv`). Applies to every command uniformly, not just the obviously-tabular ones.
