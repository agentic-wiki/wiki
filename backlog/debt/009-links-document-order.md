---
type: task
title: "Entry.Links is not in document order when link forms mix"
status: todo
priority: low
tags: [debt, links]
---

`Index.Links` documents "one LinkRef per occurrence, in document order", but `resolveLinks` appends every root-absolute link before every relative one, and `resolveWikilinks` appends wikilinks after both. An entry with `[rel](./c.md)` on line 7 and `[abs](/b.md)` on line 8 lists `/b.md` first in `wiki links`.

Now that relative is the on-disk canon and root-absolute links are still accepted, mixed entries are common. `ReadingOrder` (dump) works around it by sorting an index's links by line, which is exact except for two links of different forms on the **same** line, since links carry no column.

Fix: record the column alongside the line (`parse.matchLinks` already knows it; `wikilink.parseLine` does too) and keep `Entry.Links` sorted by (line, column) after both passes in `Build`. Then `Links` matches its doc and `ReadingOrder` can drop its sort. Found while building [dump](../2-query-surface/018-dump-single-file.md).
