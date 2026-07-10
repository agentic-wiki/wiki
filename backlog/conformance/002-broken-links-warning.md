---
type: task
title: broken links as warning in check, not error
status: done
priority: medium
tags: [dx, conformance]
---

`wiki check` reports broken links as an `error` (exit `1`). But a broken link is not malformed: per the OKF profile it may be not-yet-written knowledge, which is why consumers tolerate it and `wiki unresolved` already lists them as a to-write backlog.

**Change:** demote broken links from `error` to `warning` in `check`. Warnings keep exit `0`, so a bundle with dangling links still passes `check`. Genuine malformations (missing or invalid `type`) stay `error` (exit `1`). `wiki unresolved` is unaffected: it remains the dedicated surface for the to-write list.

This makes `check` a clean linter: `0` clean or warnings only, `1` real conformance errors, `2` couldn't run. See the [exit-codes task](../2-query-surface/008-exit-codes.md).

**Also update:** spec `README.md` and the skills note "errors like broken links exit 1"; adjust to reflect the demotion.

**Done:** `check` now records broken links at `warning` level (was `error`) in `index.Check()`, so a bundle whose only issues are broken links exits `0`. Errors are now just a missing or invalid `type`. Updated the `TestCheckSeverity` unit test, `smoke.sh`, and the agentic-wiki skill's `check` line. The spec and CLI READMEs already framed broken links as tolerated.
