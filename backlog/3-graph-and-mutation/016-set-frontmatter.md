---
type: task
title: "wiki set: write a frontmatter field from the CLI"
status: todo
priority: low
tags: [feature, mutation]
---

A `wiki set <file> <key>=<value>` command, writing through `index.SetFields`.

## Why it survived the library work

The public API made this look redundant, and for Go consumers it is: `SetField` and `SetFields` are the surface, and `wikiview` calls them directly. It stays open because **the CLI's constituency is agents**, and an agent is not a Go program.

`wiki` is described as "built for agents to call". An agent moving a task from `todo` to `in-progress` today opens the file and edits the frontmatter with its own text tools — hand-rolling the exact surgical edit `SetFields` exists to prevent, with none of its guarantees: no quoting rule, no block-list handling, no atomic write, no "change this key's lines and nothing else". That is the second-implementation failure the retro names, relocated outside the module where the guard test cannot see it.

So the case is not "a consumer needs it" but "the largest class of caller cannot reach it".

## Shape

- `wiki set <file> <key>=<value>` — repeatable, so several fields land in one pass (`SetFields` already exists for exactly this reason: two writes can leave an entry half-updated).
- `--unset <key>` for removal, mapping to `UnsetField`.
- Reserved keys (`_`-prefixed) are rejected by the library; the CLI surfaces that error rather than reimplementing the check.
- No `--force`, no type coercion: the value is written as the string given, quoted only when YAML requires it. That is the library's rule and the CLI must not add a second one.

**Not in scope:** checkbox toggling from the CLI. `SetCheckbox` is keyed by line number, which is a fine API and a poor command-line ergonomic; it wants its own design if anyone asks.

**Acceptance:** an agent can set and unset frontmatter fields without opening the file; the command is a thin wrapper with no editing logic of its own; existing conformance and tidy behaviour is unchanged.
