---
type: task
title: "links inside two kinds of code still count as links"
status: todo
priority: low
tags: [debt, bug, links]
---

Text inside code is documentation, not a link, and the scanners mostly honor that. Two forms slip through:

1. **`[[wikilinks]]` in a `~~~` fence.** `parse.maskedLines` treats both ```` ``` ```` and `~~~` as fences, but `internal/wikilink.Parse` only knows ```` ``` ````. So a `[[b]]` inside a `~~~` block is a graph edge (it shows in `backlinks`, rescues an orphan), and `tidy --wikilinks` and `dump` rewrite it, while a markdown link on the same line is correctly ignored.
   Repro: an entry with `~~~` / `[[b]] [md](./b.md)` / `~~~`; `wiki backlinks /b.md` lists the `[[b]]` line only.
2. **Multi-backtick inline code.** `inlineCodeRe` matches single-backtick spans only. CommonMark's double-backtick span, the way to show a backtick inside code, is read as an empty span followed by text, so a markdown link written inside one (an example of link syntax, say) becomes a real link, usually a broken one in `check`/`unresolved`.

Fix: one rule for "is this code", owned by `parse`. Have `wikilink.Parse` take fence detection from it rather than keep its own, and match inline code spans by backtick-run length as CommonMark does. Found while building [dump](../2-query-surface/018-dump-single-file.md).
