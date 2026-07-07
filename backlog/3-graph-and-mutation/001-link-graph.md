---
type: task
title: link graph — links / backlinks
status: done
priority: medium
tags: [feature, graph]
---

`wiki links <file>` (outgoing, unique targets) and `wiki backlinks <file>` (incoming, unique sources). The index already holds the edge set used by `unresolved`/`orphans`; expose the navigational views. (A no-outgoing-links view was skipped)

Related decision (2026-07-06): dependencies between entries are expressed as ordinary **body links** and walked with `links`/`backlinks` in both directions — enough to navigate. Structured *frontmatter* dependency tracking (a `depends` field resolved into the graph) was considered and **deferred**; revisit only if structured `ready`/`blocked` queries are needed.
