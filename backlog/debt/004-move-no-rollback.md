---
type: task
title: "move has no rollback on a mid-way write error"
status: todo
priority: low
tags: [debt, correctness]
---

`internal/index/index.go` `Move` rewrites inbound links file-by-file, then renames the source. A write error partway through leaves some files rewritten and others not (and, if it fails before the rename, the source still in place) — a partially-applied move. It returns what it did so `unresolved` can surface leftovers, but there is no transaction.

`move --dry-run` already simulates the *plan* (the tool's simulate-first pattern), but that only proves the plan is computable: dry-run and apply are separate runs, and the apply writes sequentially without staging. So the gap is atomic *commit* of the writes, not plan validation. Fix: extend simulate-first to the write layer — compute every rewritten file's bytes up front, then commit them together (or write-temp-then-rename), all-or-nothing.

Low priority: acceptable at the tool's scale, no known real-world failure; tracked so it is a decision. Flagged during the 2026-07-06 code read.
