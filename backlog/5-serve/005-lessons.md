---
type: task
title: "what the first attempt got wrong"
status: done
priority: medium
tags: [design, retro]
---

**Done (reverted, nothing shipped).** A working board UI was ported into this repo over an afternoon: ~3,800 lines of Go and ~3,400 of TypeScript. It ran, and every test passed. It was reverted anyway, and this records why so the second attempt does not repeat it.

**Three implementations of one rule.** Resolving a link target to a bundle path existed in `index.normalizeLink`, again in the server's board model, and a third time in the browser. Each was written where it was needed rather than where it belonged.

**Two frontmatter writers inside one binary.** The server carried its own surgical writer, built when it was a separate program and `setFrontmatterValue` was unreachable. Absorbing it removed the module boundary that had excused the duplication, and nothing replaced it, so the same rule sat twice in one module.

**Two config files with two parsers.** `wiki-serve.toml` beside `wiki.toml`, each with its own reader, both describing the same directory.

**Public API added to unblock a port.** `Entry.Field`, `FieldString`, `Frontmatter`, and `ParsePropFilter` were exported because the migration needed them that afternoon. [005](../4-release-and-docs/005-public-packages.md) had already written down "decide deliberately what a consumer may reach, rather than exporting fields because the move made it easy", and that is precisely what happened anyway.

**Documentation left false rather than merely stale.** The README still said "zero external dependencies" and "no server in the way" while both had stopped being true.

**Tests deleted rather than migrated.** Two README drift guards were dropped because their target moved, trading a real check for a green suite.

The common thread: every one of these is a *migration* artifact. None would have been written by someone building the thing from its intended shape. Porting preserved decisions that were correct for a board over one folder and wrong for a reader over a bundle, and the compounding cost of those decisions was invisible while the tests were green.
