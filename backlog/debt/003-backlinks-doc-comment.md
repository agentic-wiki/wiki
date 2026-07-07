---
type: task
title: "stale duplicated doc comment on index.Backlinks"
status: done
priority: low
tags: [debt, docs]
---

`internal/index/index.go` — `Backlinks` carries two stacked doc comments. The leftover first sentence ("returns the unique entries that link to the given root-absolute path (the first link from each source)…") contradicts the accurate one right below it ("every internal link that points to target, one LinkRef per occurrence…"). The code matches the second (one `LinkRef` per occurrence). Delete the stale first sentence. Trivial; flagged during the 2026-07-06 code read.

**Done (2026-07-07):** removed the leftover leading sentence; the accurate one-`LinkRef`-per-occurrence description remains.
