---
type: task
title: README — install + modernization
status: done
priority: medium
tags: [docs, release]
---

The README carries:

- **Install**: curl one-liners per platform (`wiki_<os>_<arch>.tar.gz`) + `go install`, modeled on the `vars` README.
- **Modernized content**: `validate` → `check`, three-layer framing (format / tool / skill), a Roadmap section that dogfoods `wiki` on its own backlog, examples refreshed (no tax), build-from-source under Development.

Install-URL liveness is verified when the repo is published and tagged (global roadmap, publish). `wiki init` usage docs travel with the [init command](../3-graph-and-mutation/003-init-scaffold.md).
