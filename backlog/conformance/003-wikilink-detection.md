---
type: task
title: detect & convert wikilinks
status: todo
priority: medium
tags: [feature, conformance]
---

The format does not support `[[wikilinks]]`, but the tool does not enforce it: `wiki` never
parses `[[...]]` as a link, so an Obsidian-authored wikilink is invisible (no graph
edge, missed by `backlinks`/`orphans`/`move`, and `wiki check` stays silent). Silent
graph drift, the exact failure the deterministic engine is meant to prevent.

Make the rule enforceable:

- `wiki check` warns on `[[...]]` outside code spans (reuse the code-span masking the
  [table-pipe debt](/debt/002-table-pipe-edge-cases.md) needs, so both share one masker).
- `wiki tidy` converts `[[Target]]` / `[[Target|alias]]` to `[alias](/canonical/target.md)`,
  resolving the target via the existing basename `Resolve` and reporting collisions the
  way `tidy --slug` does.

Pairs with the stopgap already shipped: the skills tell the agent to rewrite-and-advise,
and the spec + wiki READMEs document the Obsidian settings that stop wikilinks appearing
in the first place. This task is the deterministic backstop.
