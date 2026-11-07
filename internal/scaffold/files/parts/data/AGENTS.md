One example of each form of data: records in `{dir}companies/`, a dataset in `{dir}expenses.md`.

- **`{dir}companies/`** holds one entry per company, `type: company`, with the fields its `index.md` lists. Nothing has to link to a record, but `wiki orphans` doesn't check this folder, so link every new collection from `{dir}index.md`.
- **`{dir}expenses.md`** is a dataset, `type: dataset`: one table, one row per expense.

```markdown
---
type: company
title: Acme Corp
url: https://acme.example
country: DE
founded: 1998
sources:
  - url: https://example.com/acme-2025.pdf
    as_of: 2025-12-31
---

What the fields don't say: context, judgement, links to related records.
```

```sh
wiki list --where type=company --where country=DE
wiki list --where type=company --format csv --fields title,country,founded
wiki list --where type=company --format json | jq '.[] | select(.founded != null and .founded < 2000) | .title'
wiki table {dir}expenses.md --format csv
```
