---
type: task
title: "wiki.toml ref_fields: declare frontmatter fields that hold entry paths"
status: todo
priority: medium
tags: [feature]
---

Found while building wikanban on the `project-backlog` workflow, which uses a `blockers: []` frontmatter field for task dependencies. The field recipes (`epic:`, `client:`, `blockers:`) are recommended in the shipped workflows, yet a frontmatter path ref has **no integrity support at all**, and `move --include-frontmatter` fails silently on the spellings the format otherwise prefers.

## Two concrete gaps (verified against v0.8.0)

1. **`move --include-frontmatter` matches by exact string equality**, so with the flag set (the user having asserted the fields are refs) only a **root-absolute** value equal to the moved path is rewritten. A `./dep.md` or bare `dep.md` ref to the moved file is left dangling, with no warning. Reproduced: three entries with `[/dep.md]`, `[./dep.md]`, `[dep.md]`; a move rewrote one and silently broke two (output: `1 frontmatter ref(s)`).
2. **Nothing validates a frontmatter path ref.** `check` does not, `unresolved` does not, `backlinks` does not follow one. A documented pattern has zero safety net, so a typo or a hand-move rots invisibly.

## Proposal

Optional `ref_fields = ["epic", "blockers"]` in `wiki.toml`, declaring which frontmatter fields hold entry paths. Then:

- **`check`** validates each listed field's value(s) resolve to a real entry, warning (like a broken body link) on a miss. Closes gap 2.
- **`move`** rewrites those fields without the blanket `--include-frontmatter` opt-in, because the config already asserts they are refs. Closes gap 1's "did the user mean it" ambiguity.
- **Relative frontmatter refs become viable**: because `move` now knows the field is a ref, it can respell the value relative to the linking file's location the way it already does for a moved file's own body links, retiring the root-absolute special case entirely. This is the real payoff, it lets the format be relative *everywhere* instead of "relative except frontmatter". See [the docs task](../conformance/011-frontmatter-ref-docs.md).
- **`backlinks`** *optionally* follows ref-fields (behind the same declaration), so a dependency graph built on fields is traversable. Consider carefully: it blurs the "a field is not an edge" model, so it may stay out of scope.

## Acceptance criteria

- `ref_fields` parsed; absent = today's behaviour exactly (no validation, `move` needs the opt-in flag).
- `check` validates listed fields (scalar and list values), warning severity, exit 0.
- `move` rewrites listed fields automatically; decide and document whether it writes relative (preferred) or keeps absolute.
- Unknown field names in `ref_fields` are a config warning, not a silent ignore.
- Tests: list-valued field, missing target, both path spellings on read, a move that respells refs, a `check` over a dangling ref.
- Sweep AGENTS.md + both field-recipe WORKFLOWs + spec once behaviour lands.
