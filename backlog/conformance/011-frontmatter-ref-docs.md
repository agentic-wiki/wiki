---
type: task
title: "docs: frontmatter path refs stay root-absolute (fix contradiction + document the exception)"
status: todo
priority: medium
tags: [bug, docs]
---

Found while building a downstream tool (wikanban) on top of the `project-backlog` workflow. Two documentation problems around the relative-links change (v0.8.0), both actively misleading.

## The problem

Since v0.8.0 body links are stored **relative** to the linking file, but frontmatter path values (the `epic:` / `client:` field recipes, opt-in `move --include-frontmatter`) deliberately stay **root-absolute**. That split is correct, but it is documented in exactly one place, a single CHANGELOG line, and one shipped doc directly contradicts it.

1. **`internal/scaffold/files/AGENTS.md` contradicts itself.** Line ~29 says links are stored *relative to the linking file* (`../finance/income.md`), while the Conventions summary near line ~145 still reads **"Root-absolute links, no wikilinks"** (pre-v0.8.0 wording). The Conventions list is the part an agent skims, so every scaffolded bundle ships a rule that disagrees with the tool's actual behaviour.
2. **The frontmatter exception is effectively undocumented.** That frontmatter path values stay root-absolute (while body links are relative) appears only in the v0.8.0 CHANGELOG. It is absent from AGENTS.md, the spec's Links section, and `wiki move -h`. A reader who learned "links are relative" will reasonably write `blockers: [./x.md]` and it will silently not round-trip through `move` (see [ref-fields](../3-graph-and-mutation/015-ref-fields.md)).

## Acceptance criteria

- AGENTS.md Conventions bullet corrected to relative body links, consistent with line ~29 (no remaining "root-absolute links" instruction for body links).
- The frontmatter-refs-are-root-absolute exception stated where a reader meets refs: AGENTS.md (the field-recipe note), the spec Links section, and ideally a line in `move -h` for `--include-frontmatter`.
- One sentence on *why* (frontmatter values are metadata, not rendered links, so relative-to-file spelling would be ambiguous without a base) so it reads as a decision, not a quirk.
- Grep the repo for other stale "root-absolute link" phrasings and sweep them.
