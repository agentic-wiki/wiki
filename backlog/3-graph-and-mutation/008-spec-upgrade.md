---
type: task
title: wiki upgrade (cross-spec migration)
status: todo
priority: low
tags: [feature, graph]
---

`wiki upgrade`: migrate a bundle from one agentic-wiki spec version to a newer one, applying whatever the bump requires (frontmatter/content transforms, link-style changes, and the new embedded `okf_version`). Distinct from [`check --fix`](./007-check-fix.md), which only repairs drift against the *current* spec. A v2 concern: in v1 the tool branches on no version, so this stays parked until a second spec version exists.
