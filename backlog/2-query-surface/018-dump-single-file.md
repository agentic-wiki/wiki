---
type: task
title: "dump: every entry as one markdown stream (llms-full.txt)"
status: done
priority: medium
tags: [feature, query]
---

Concatenate a bundle's entries into one markdown document on stdout, so a whole base can be published at a single URL for agents to consume (`wiki dump > llms-full.txt`).

Naming note: on [llmstxt.org](https://llmstxt.org) `llms.txt` is an *index* (title, summary, link lists, much like our `index.md`), and the full concatenation is `llms-full.txt`. This task is the latter only.

## Decided

**Read-only; transforms only what goes to stdout.** Nothing is written to disk. Files keep their canonical relative links ([012](../3-graph-and-mutation/012-switch-canonical-links.md)); every transformation below happens in memory, on the dumped stream. The stream is a pure function of the source: same source in, same bytes out.

**Scope: indexed entries, not git-tracked files.** Git is optional; the index (entries minus `ignore`) is the set, minus every `log.md` unless `--logs` ([019](./019-dump-skips-logs.md)). `--prefix` / `--where` narrow it with their usual semantics. Filters decide **inclusion only**: ordering is always computed over the full tree, so a filtered-out `index.md` still orders its folder.

**Every entry in scope appears exactly once, linked or not.** Links only decide *order*, never inclusion: an entry nothing links to is still dumped.

**Ordering: index-guided tree walk** (one strategy, no `--order` flag). The walk visits every folder, starting at the root. In each folder:

1. the folder's `index.md`, if present;
2. its direct children (files, and subfolders via their `index.md`) in the order `index.md` first links them;
3. the remaining children, files and subfolders, by path.

A subfolder, wherever it lands, is emitted whole and recursively (its `index.md`, then its children by the same rule) before the walk moves on.

A folder without `index.md` therefore falls back to plain path order, so a bundle with no indexes dumps in path order. Only links to *direct children* count for ordering: a link to a grandchild orders it inside its own folder, and a link elsewhere orders nothing. First occurrence wins on repeats. Links come from the index already in document order, with code spans and fences excluded.

**Delimiter: an HTML comment carrying the root-absolute path**, on its own line before each entry:

```
<!-- /finance/income.md -->
```

**Links rewritten to root-absolute in the output.** A relative link means nothing once files are concatenated, so every in-bundle link target is emitted as its root-absolute path, keeping its `#anchor`, so it matches the delimiter of the entry it names.
- **Unresolved** in-bundle links: still normalized to root-absolute (the path they *would* have).
- **`[[wikilinks]]`**: resolved ones become `[text](/path.md#anchor)`, so the dump has one link form. An unresolvable wikilink has no path, so it stays verbatim.
- **URLs** (any scheme): left verbatim.
- **Out-of-bundle links** (`../PRD.md`): left verbatim. After the dump, stderr gets one warning listing each one as `file:line target`, since such a target cannot exist where the dump is published. The list comes from the existing `Entry.Outside`, and paths acknowledged via `wiki.toml` `ignore` are silenced, the same as `check` does. Stdout stays pure content; the exit code stays 0.
- Only the link target changes; every other byte of the body is left alone. Reuse the in-memory side of `normalizeEntryLinks` / `normalizeLink` / `ConvertWikilinks` rather than a second rewriter.

**Frontmatter stripped by default** (the body as `read` returns it, via `Entry.Body()`); `--frontmatter` includes it, verbatim between `---` fences right after the delimiter.

**Byte-stable output.** The same source gives identical bytes on every OS and run, independent of filesystem iteration order, timestamps or environment: `\r\n` normalized to `\n` (a Windows checkout or `core.autocrlf` would otherwise change the bytes), a leading UTF-8 BOM stripped, entries separated by exactly one blank line, each entry ending with exactly one newline (empty-body entries included), and a single trailing newline at the end.

**Privacy: no guard.** Publishing the dump is the same act as publishing the bundle. Scope it with `--prefix`/`--where` if needed.

## Edge cases to cover in tests

- **Spoofed boundary:** a body line identical to a delimiter (`<!-- /x.md -->`) is indistinguishable from a real one. Bodies are author-controlled, so accept and document it rather than escape.
- **Unterminated `<!--` in a body:** when the dump is *rendered*, it hides every following entry up to the next `-->`. Raw text for agents is unaffected. Document it; `check` could flag it later.
- Links inside code fences/spans are not rewritten. Image targets (`![](./img.png)`) are normalized like any in-bundle path.
- Ordering: `index.md` linking a child twice; linking a grandchild; linking a sibling folder; a subfolder with no `index.md`; a filtered-out `index.md`; a root with no `index.md`; cycles between indexes (none matter, since ordering only looks at direct children).
- Mixed CRLF/LF entries; a BOM; empty bodies; a frontmatter-only file; a file with no trailing newline.
- Files on disk are byte-identical before and after a dump.
- An entry nothing links to (not even its folder's `index.md`) is still dumped, exactly once.
- Out-of-bundle links: verbatim in stdout, listed on stderr, silenced by `ignore`; nothing on stderr when there are none.
- Determinism: dump twice and compare bytes; a golden-file test over a fixture bundle. "By path" means a plain byte-wise sort of the root-absolute path, never locale- or case-aware, so macOS, Windows and Linux agree.

**Acceptance:** one bundle read; no writes; output as specified above; composes with `--prefix`/`--where`; README command table and examples updated; smoke test covers it.

**Done (2026-10-14):** shipped as `wiki dump [--prefix] [--where] [--frontmatter]` (the name was chosen over widening `read`).

- `index/dump.go`: `Index.ReadingOrder` (the walk), `Index.Dump` (the transform and layout), and `Index.OutOfBundle`, which `check` now reports from as well, so the two cannot disagree about which links get reported.
- `parse.MapInternalLinks` rewrites link targets in place through `matchLinks`, now the only link scanner (`Links` reads it too). Inline code is blanked in place rather than deleted, so offsets hold; a side effect is that a link's text now keeps the inline code written inside it, where that code used to be deleted from the text.
- Wikilink conversion is shared with `tidy --wikilinks` (`Entry.wikilinkRewrites`, spelled relative there and root-absolute here), and `wikilinkResolver` replaces three copies of the resolver setup.
- Out-of-bundle warnings go to stderr as `wiki: warning: /a.md:5 out-of-bundle link -> ../PRD.md`, after the dump; exit stays 0.
- Tests: `index/dump_test.go` (ordering, transforms, layout, CRLF/BOM, read-only, determinism, out-of-bundle), `parse` (`MapInternalLinks` incl. byte-exact identity), `cmd/wiki` (`TestCmdDump`, incl. filtered-out index still ordering), and a smoke section.

Left as documented limitations: two links of different forms on the same line of an index keep the index's form order ([debt 009](../debt/009-links-document-order.md)); a path containing `-->` would close its own delimiter comment early (no such filename has been seen; `check` could flag it if one appears). Found and filed on the way: [BOM hides frontmatter](../debt/007-bom-hides-frontmatter.md), [links inside two kinds of code](../debt/008-code-detection-gaps.md).
