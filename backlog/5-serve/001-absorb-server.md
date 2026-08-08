---
type: task
title: "wiki serve: absorb the bundle server into the CLI"
status: cancelled
priority: high
tags: [feature, architecture, ui]
---

**Cancelled (2026-08-08): the UI lives in its own repo, `wikiview`.** The design work moved with it; only the [retro](./005-lessons.md) stays here, because the reverted attempt happened in this repo.

The proposal was to bring the server in as `wiki serve`: one binary, one name, using `internal/{bundle,index,parse}` directly so the shell-out and its two stand-ins disappeared.

**The argument against it, which decided the call:** absorbing the UI changes what `wiki` is. It is a zero-dependency static binary and a neutral engine, and serving means `net/http` plus a file watcher, roughly tripling the binary and making "zero external dependencies" false. Keeping the engine lean and the UI separate preserves the layering the spec states outright: the format stands alone, the tool is a neutral engine over it, and presentation is somebody else's job.

**What this repo owes the decision instead**, and it is the harder half: a separate module cannot reach `internal/`, so the engine needs a real public API ([005](../4-release-and-docs/005-public-packages.md)). That is now a prerequisite rather than a third-party nicety.

The trap to keep in view is the one the retro names. A module boundary is exactly what excused two frontmatter writers and three link resolvers last time: each was written where it was needed because the right home was unreachable. Re-establishing that boundary means the public API has to be **good enough that no consumer needs to reimplement a rule** — that is the whole test of 005, and the reason it must be designed for consumers rather than extracted to unblock a port.
