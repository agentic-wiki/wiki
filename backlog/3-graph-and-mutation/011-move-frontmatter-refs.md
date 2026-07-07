---
type: task
title: "move --include-frontmatter (opt-in: rewrite frontmatter refs on move)"
status: todo
priority: medium
tags: [feature, graph]
---

By default, frontmatter is **opaque** to the tool: `move` rewrites body markdown links but leaves frontmatter values untouched, and `check` never inspects them. That default is correct and non-opinionated, the tool can't know a path-shaped value is a reference versus a snapshot (`origin: /backlog/old.md`), an example, or a placeholder.

Add an **opt-in** `wiki move <src> <dst> --include-frontmatter`: when set, also rewrite any frontmatter scalar or list element whose value equals `src`'s root-absolute path, to `dst`. This is not the tool guessing, it's the user asserting "these fields are references, keep them valid on this move", so it stays consistent with the opaque-by-default stance (no flag = frontmatter untouched, same as `check`). A convenience for the field-based relationship recipe (`epic: /epics/x.md`, `client: /clients/y.md`), whose refs otherwise silently dangle after a move.

- **Exact path equality** (value == src's path), scalar or list element. No "looks like a path" heuristic, no partial matches.
- **Documented caveat on the flag:** it rewrites *every* matching value, including a snapshot field that happens to equal the moved path. That is the opt-in tradeoff, don't pass the flag if you keep literal-path snapshot fields.
- Requires parsing + rewriting the frontmatter block (frontmatter-relative line numbers), not just the body.

**Kept out of scope (these *would* be opinionated):**

- `check` flagging "dangling" frontmatter path-values, the tool would have to assume a `/….md` value is a ref; wrong for snapshot / example / placeholder fields.
- Interpreting a markdown link *inside* a frontmatter value as an edge, sacrifices the field's `--where` filtering (the value becomes a link, not a plain path) and collides with YAML flow-sequence syntax (`[...]`); a plain body link is simpler for the same result.

Body links stay the zero-config maintained-edge recipe; `--include-frontmatter` serves teams who prefer filterable fields and accept the opt-in.
