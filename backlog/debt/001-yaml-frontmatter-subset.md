---
type: task
title: hand-rolled YAML frontmatter subset
status: done
priority: low
tags: [debt, parser]
---

`parse.Frontmatter` parses only our scalar + string-list subset. Chosen for zero dependencies and a network-free, single-static-binary build. Swap to `gopkg.in/yaml.v3` only if frontmatter grows nested structures — the parser is isolated behind `parse.Frontmatter`.

**The `wiki.toml` half of this is gone.** The config was read by the same hand-rolled approach and it went wrong in two ways that were invisible: a multi-line array parsed to an empty list (so a declared `types` vocabulary silently allowed everything), and a key inside any table was treated as top-level (so `[tool.x] types = [...]` silently replaced the bundle's vocabulary). `bundle` now uses `BurntSushi/toml`.

That weakens the zero-dependency argument for keeping the frontmatter parser hand-rolled, but does not remove it. The two cases are not alike: `wiki.toml` is a config file where users legitimately expect the whole of TOML, while frontmatter is deliberately a **subset**, and the surgical write API depends on that — it edits the lines belonging to a key and leaves the rest byte-for-byte, which a parse-and-reserialize round trip through a full YAML library would destroy. See [the write API](../4-release-and-docs/005-public-packages.md).
