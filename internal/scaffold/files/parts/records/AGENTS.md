Records, one entry each, in a collection folder per kind.

- **`{dir}items/`** is the first collection. To rename it (`{dir}companies/`), move each file in it with `wiki move`, its `index.md` included, then change that index's heading. Nothing has to link to a record.
- **A new kind** gets its own folder, linked from `{dir}index.md`, and its own `type` in place of `record`.

```markdown
---
type: record
title: Acme Corp
country: DE
founded: 1998
sources:
  - url: https://example.com/acme-2025.pdf
    as_of: 2025-12-31
---

What the fields don't say: context, judgement, links to related records.
```

```sh
wiki list --where type=record --where country=DE
wiki list --where type=record --format csv --fields title,country,founded
wiki list --where type=record --format json | jq '.[] | select(.founded != null and .founded < 2000) | .title'
```
