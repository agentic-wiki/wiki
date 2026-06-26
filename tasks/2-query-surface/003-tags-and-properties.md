---
type: task
title: tags & properties commands
status: done
priority: low
tags: [feature, query]
---

`wiki tags` (list; `--counts`, `--sort=name|count`) and `wiki tag <name>` (entries with a tag). `wiki properties` (frontmatter keys; `--counts`) and `wiki property <name>` (values across entries; `--prefix`). Generalises the `--type/--tag` filters into introspection.

Done: `wiki tags`, `wiki properties`, and `wiki property <name>`, each with `--counts` / `--sort=name|count` / `--prefix` (uniform flags via `countFlags`, shared rendering via `sortedCounts`/`emitCounts`). Aggregation lives in the index (`TagCounts`, `PropertyKeyCounts`, `PropertyValueCounts`, reusing `Filter` for `--prefix`; a value counts an entry once even if repeated in a list). Reserved files contribute only what they carry (the root `index.md`'s `okf_version`), so `property type` ignores them. **Dropped `wiki tag <name>`:** it is exactly `wiki list --tag <name>`, and a second spelling violates one-correct-way; the introspection commands enumerate, `list --tag` filters.
