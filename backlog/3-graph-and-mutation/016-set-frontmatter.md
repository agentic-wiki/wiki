---
type: task
title: "wiki set <path> <key> <value>: the missing write primitive"
status: todo
priority: medium
tags: [feature, mutation]
---

`wiki` has no way to change a frontmatter field. Every mutation it offers moves or canonicalizes files (`move`, `tidy`, `check --fix`); advancing `status: todo` to `status: in-progress`, the single most common edit in a backlog bundle, has to be done by hand-editing the file.

That is defensible for an agent (which reads and writes Markdown anyway, and where composing frontmatter is the judgment work `wiki` deliberately stays out of), but it breaks down for **any other consumer**. Surfaced by `wikanban`, a kanban UI over a bundle: dragging a card between columns is exactly this edit, and with no primitive the UI has to grow its own frontmatter writer, a second implementation of the format that can drift from `parse`.

**Proposal:** `wiki set <path> <key> <value>`, plus `--unset <key>`.

The implementation already exists, unexported: `setFrontmatterValue` (`internal/index/index.go`) backs `check --fix`'s `okf_version` repair. It is deliberately **surgical**, and that is the property to preserve and document: it finds the `---` fence, replaces exactly the one matching line, and leaves every other byte alone. Never parse-to-struct and re-serialize, `parse.parseYAMLSubset` is lossy by design (it skips nested maps and anchors, drops comments, normalizes quoting), so a round-trip would silently delete anything the subset does not model.

**To design before building:**

- **Quoting.** `setFrontmatterValue` always emits `key: "value"`, so setting `status` to `todo` rewrites `status: todo` as `status: "todo"`. Harmless for `check --fix` (which touches one machine-managed field) but churny as a general command. Emit bare when the value needs no quoting, and match the existing line's style where there is one.
- **List values.** `--add` / `--remove` for a list-valued key (`tags`), or scalars only in v1? Preserving flow (`[a, b]`) versus block (`- a`) style is the wrinkle.
- **Creating the key** when absent (today: inserted before the closing fence) versus erroring. Probably insert, but say so.
- **Validation.** Should `set type=…` refuse a value outside a declared `types` vocabulary, or stay dumb and let `check` catch it? Leaning dumb + `check`, consistent with the tool's opacity about field meaning.
- **Concurrency.** Atomic write (temp file + rename), so a reader never sees a half-written entry.

**Acceptance:** `wiki set /active/x.md status in-progress` changes exactly that line and nothing else; `--unset` removes the key; an absent key is inserted; every other byte of the file (comments, key order, nested blocks, CRLF) round-trips identically. Tests over each frontmatter shape the parser supports.
