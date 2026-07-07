---
type: task
title: OKF alignment — reserved files, timestamp, real YAML, check as a lint
status: done
priority: high
tags: [feature, conformance]
---

Made agentic-wiki OKF-friendly: our bundles are valid OKF, and `wiki` accepts any OKF bundle. `wiki.toml` and `check`'s opinions stay as our opt-in layer.

Done:
- **Reserved files** (the real incompatibility): `index.md`/`log.md` are reserved *filenames*, not types — no frontmatter (the root `index.md` may carry `okf_version`), exempt from the `type` rule. `check` softly warns on stray frontmatter; scaffold/dogfood/smoke and spec §2.2/2.4/2.5 + README updated.
- **`timestamp`**: adopted OKF's field (retired `created`/`updated`); `check` errors on a present-but-malformed value (RFC 3339 or `YYYY-MM-DD`), silent when absent.
- **YAML**: zero-dep parser extended — inline `#` comments, block scalars (`|`/`>`), graceful nested-map skip; supported subset documented (superseded `debt/001`).
- **`log.md`**: `check` warns on non-ISO date headings (§7).
- **`check` reframed** as an opt-in lint, not an "OKF conformance" gate.
