---
type: task
title: normalize exit codes to ls/grep/lint conventions
status: done
priority: medium
tags: [dx, query]
---

`wiki` should return exit codes that match the Unix tools an agent already knows, so "no results" is never mistaken for failure.

The rule: enumeration and diagnostic commands answer "what's here" (like `ls`/`find`), so an empty result is success. Only `search` (a `grep`) and `check` (a linter) treat an empty/clean result as a branchable negative.

| Command | Analog | Exit on empty result |
|---|---|---|
| `list`, `tasks`, `tags`, `properties`, `property`, `links`, `backlinks`, `orphans`, `unresolved` | `ls` / `find` | `0` |
| `search` | `grep` | `1` (no match) |
| `check` | linter | `0` clean/warnings, `1` errors (see [broken-links task](/conformance/002-broken-links-warning.md)) |

`2` stays the universal "couldn't run" code everywhere (missing bundle, bad args, unreadable file).

`unresolved` and `orphans` are `0` even when non-empty: broken links and orphans are conformant per OKF, not errors, so they are listings to read (returning non-zero only on a real failure, `2`), not pass/fail gates. `check` is the conformance gate.

**Change:** the nine enumeration/diagnostic commands currently exit `1` on an empty result; make them exit `0`. Leave `search` and `check` unchanged.

**Also update:** spec `README.md` exit-code section (currently says "treat exit 1 as normal none-found") and both skills' Exit-codes sections.

Supersedes the original spec-repo task, which would have zeroed `search` too; keeping `search` grep-like is the better fit.

**Done:** dropped the `return 1`-on-empty branch from `list`, `tasks`, `tags`, `properties`, `property`, `links`, `backlinks`, `orphans`, `unresolved`; `search` and `check` unchanged. Updated unit tests, `smoke.sh`, the CLI README, spec `README.md`, and both skills' Exit-codes sections. The README's `wiki unresolved`-as-gate example was rewritten (it had relied on the old exit `1`).
