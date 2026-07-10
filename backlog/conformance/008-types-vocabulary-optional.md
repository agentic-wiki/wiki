---
type: task
title: "wiki.toml `types`: opt-in error on undeclared type; free-form when absent"
status: done
priority: medium
tags: [feature, conformance]
---

**Decision made.** Today `check` treats the two type problems with inconsistent severity: a *missing* `type` is an **error**, but an *undeclared* `type` (not in `wiki.toml` `types = [...]`) is only a **warning** (`index.go:857`, via `Bundle.KnownType` = `slices.Contains`). And because `KnownType` is a plain membership test, an **empty or absent `types` list makes every entry warn**, so the list is effectively mandatory just to keep `check` quiet. The current `types` list buys almost nothing: the set is derivable (`wiki property type --counts`), adding a new kind is a legitimate choice (not a defect), and tags are already free-form, so half-governing `type` is an unprincipled middle.

Resolve it by making the vocabulary a real, consistent, opt-in gate:

- **Existence, always validated**: a missing or empty `type` on an entry stays an **error** (unchanged). "Validated for existence" is the floor.
- **No `types` declared (absent or empty list)**: any non-empty `type` is valid, **no vocabulary check** at all. This fixes the footgun and removes the maintenance burden, you only declare a vocabulary if you want one.
- **`types = [...]` declared**: `wiki check` **errors** (not warns) on any entry whose `type` is not in the list. Same severity as the missing-type error, so the two are consistent, and the gate is genuinely opt-in (declaring the list is the opt-in).

The value proposition this preserves: **typo / drift protection** for a base that wants a controlled vocabulary (`type: conept`, or `task`/`todo`/`tasks` accreting), now enforced (error) rather than a nag, and off entirely when you don't opt in.

Implementation:
- `Bundle.KnownType`: return `true` when `Types` is empty (empty/absent vocabulary ⇒ everything is known). Treat an explicit `types = []` the same as absent (no constraint), an empty declared vocabulary erroring on everything would be nonsensical.
- Check branch (`index.go:855-859`): change the undeclared-type issue from `warning` to `error`; reword to name the fix ("type `X` not declared in wiki.toml `types`; add it there or fix the entry"). Only emit when a vocabulary is declared.
- Docs: AGENTS/workflows wording shifts from "extend `wiki.toml` as you introduce new kinds" to "types are free-form unless you declare `types` in `wiki.toml`, which then gates them (error on undeclared)". `wiki property type --counts` remains the way to see what's in use.
- Tests: absent/empty list ⇒ any type passes (no warning, no error); declared list ⇒ undeclared type is an **error** (exit 1); missing type still an error.

Migration note: for a bundle that declares `types`, an undeclared type flips from warning (exit 0) to error (exit 1). Intended.

Resolved: the **scaffold starters ship the suggested vocabulary commented out** (free-form by default; uncomment one line to opt into enforcement), matching the existing commented-`ignore_orphans` precedent. Honors "opt-in" while keeping the starter list visible and one edit away.

Shipped in v0.7.0: `Bundle.KnownType` returns true when no vocabulary is declared; the check branch errors (was warning) on an undeclared type only when `types` is declared, naming the fix; all four scaffold `wiki.toml` files comment out `types`; AGENTS + `default`/`product-docs` workflow wording reframed to "free-form unless you declare `types`"; unit tests (`KnownType` empty-list, check declared-vs-free-form), plus a smoke assertion. See [unknown config keys warn](/conformance/007-unknown-config-keys.md) (`types` stays a recognized key regardless).
