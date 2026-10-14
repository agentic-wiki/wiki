---
type: task
title: "a UTF-8 BOM hides an entry's frontmatter"
status: todo
priority: medium
tags: [debt, bug]
---

A file starting with a UTF-8 BOM (`EF BB BF`, which some Windows editors write) is read as having **no frontmatter**: `parse.Frontmatter` requires the content to begin with `---`, and the BOM comes first. So the entry has no `type`, `check` errors with "missing required `type`", `--where` never matches it, and the frontmatter shows up as body text in `read`.

Repro: `printf '\xef\xbb\xbf---\ntype: note\n---\nhello\n' > a.md && wiki check`.

The write API has the same blind spot (`frontmatterFence` checks `lines[0] == "---"`), so fixing only the parser would be worse than today: the parser would see a frontmatter block that `SetField` cannot find, and `SetField` would prepend a second block. **Fix both together**: strip a leading BOM in `parse.Frontmatter`, and have `frontmatterFence` accept a BOM before the opening `---` (keeping the BOM on write, so the file's encoding marker is left alone).

`wiki dump` already drops a BOM before splitting frontmatter, so its output is right either way; only `--where` filtering on such an entry is affected there. Found while building [dump](../2-query-surface/018-dump-single-file.md).
