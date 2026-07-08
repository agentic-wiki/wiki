---
type: task
title: detect & convert wikilinks
status: done
priority: medium
tags: [feature, conformance]
---

The format does not support `[[wikilinks]]`, but the tool does not enforce it: `wiki` never parses `[[...]]` as a link, so an Obsidian-authored wikilink is invisible (no graph edge, missed by `backlinks`/`orphans`/`move`, and `wiki check` stays silent). Silent graph drift, the exact failure the deterministic engine is meant to prevent, and the README actively courts Obsidian users, so a forgotten "turn off wikilinks" setting loses links with no warning.

**Stance: recognized, not officially supported.** Meaning "not what we want you to do, but we still make it work." So resolution matches Obsidian (zero promises), and the tool nudges you back toward standard links.

## Decided design

**Isolation.** All of this lives in a dedicated `internal/wikilink` compatibility package (ported from the `obsy` predecessor: `parser/links.go` + `vault/resolve.go`, both already zero-dep, tested, and fuzzed). It holds the parser and the resolver as pure functions (input: content bytes / the entry list; output: links / resolved paths). `internal/parse` (standard markdown) and `internal/index` (the graph) stay clean and only *call into* it; the dependency flows one way (index -> wikilink). The compat layer is a ramp *off* wikilinks, deliberately not a second first-class system.

**Two resolvers, on purpose.** wiki's own `Resolve` (root-absolute markdown) is untouched. The wikilink resolver uses Obsidian semantics instead: strip the `#anchor` and `|display` (and Obsidian's escaped `\|`), add `.md`, then match. `[[folder/note]]` is an exact vault-relative path; a bare `[[note]]` matches by **basename anywhere in the bundle**, so `[[Something]]` resolves even when it lives in a sibling or parent folder, tiebroken by fewest path segments -> same folder as the source -> alphabetical. This dual-resolution is a deliberate, quarantined inconsistency: it is the honest encoding of "resolve like Obsidian for this syntax, zero promises." An unresolvable or genuinely ambiguous target is treated as unresolved, surfaced by `unresolved`/`check` exactly like a broken markdown link.

**Aliases.** Obsidian lets a note declare alternate names in its frontmatter (`aliases: [Foo, Bar]`), and `[[Foo]]` then resolves to that note even though no file is named `foo.md`. To resolve like Obsidian, the compat resolver builds an alias map (alias -> entry path) from every entry's `aliases:` frontmatter and checks it before the basename search. This is the *one* place the compat layer reads a frontmatter field; it is confined to the compat package, so wiki's opaque-frontmatter stance elsewhere is unaffected. (Aliases are optional to ship: the common case works without them, but they are already written and tested in `obsy`, so we include them.)

**Embeds.** `![[x]]` has no transclusion in wiki, so it is just a reference: detected, resolved, and `tidy` rewrites it to a plain link `[x](/x.md)` (dropping an embed intent wiki never honored).

## Behavior

- **Graph.** Resolved wikilinks become normal edges (marked as wiki-origin so `check`/`tidy`/the nudge can see them), so `backlinks`/`orphans`/`move` just work.
- **`check`.** Warns per wikilink (non-conformant; unlike relative links, which are valid OKF and silent).
- **`tidy`.** Converts `[[t]]` / `[[t|d]]` / `[[t#h]]` / `![[e]]` to canonical root-absolute markdown, resolving via the compat resolver, reporting unresolved ones (left as-is) the way `tidy --slug` reports collisions.
- **stderr nudge.** Graph commands (`links`/`backlinks`/`orphans`/`move`) print one summarized line to **stderr** (never stdout) when a wikilink was resolved: e.g. `note: 2 wikilink(s) resolved; not officially supported, run 'wiki tidy' to convert`. Keeps stdout clean and pipeable while steering toward standard links.

## Do NOT port

obsy's `cobra`/`yaml.v3` deps, and its wikilink-*first* framing. wiki stays markdown-links-first; wikilinks are the compat exception.

## Phasing

1. **[done]** `internal/wikilink` package (parser + resolver + aliases), ported and adapted to wiki's `/`-paths, zero-dep, fully tested (`Parse`/`Split`/`Full`/`Resolve`/`AliasMap`).
2. **[done]** `index` build resolves wikilinks into marked graph edges (`Link.Wiki`), so `backlinks`/`orphans`/`links` see them; `Move` skips them (they re-resolve by basename); `check` warns per wikilink. Covered by `TestWikilinkGraph`.
3. **[done]** `wiki tidy --wikilinks` converts `[[t]]` / `[[t|d]]` / `[[t#h]]` / `![[e]]` (embed -> plain reference) to canonical root-absolute markdown, resolving via the compat resolver, leaving + reporting the unresolvable. Covered by `TestConvertWikilinks` + smoke.

**Stderr nudge: dropped.** It would have required threading wikilink-counting into the graph commands (a spread of the compat concept), and `check` (flags which files) plus `tidy` (lists + converts) already cover "find" and "fix". So the compat footprint stays minimal: `internal/wikilink` (algorithm), one `Link.Wikilink` bool (for `Move` to skip), `Entry.wikilinks` (for `check`/`tidy`), and the `ConvertWikilinks` + `check` wiring. No changes to `LinkRef` or any query output contract.
