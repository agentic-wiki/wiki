---
type: task
title: check anchor links point at a real heading
status: todo
priority: medium
tags: [dx, conformance]
---

`wiki check` verifies a link's target *file* resolves, but ignores the `#anchor`. So `[Something](./thing.md#title-that-does-not-exist)` passes even when `thing.md` has no such heading: a dangling reference the link graph currently can't see. Detect it.

**Feasible with what's already parsed:** `parse.Headings` extracts every ATX heading (kept on `Entry.Headings`), and link targets already carry their `#anchor` suffix (`anchorOf` in `internal/index`). The missing piece is a slugifier to turn a heading into its anchor form and match it against the link's anchor.

**Proposal:** in `check`, for each internal link that has an `#anchor` and whose target file resolves, verify the anchor equals the slug of some heading in the target. A self-link (`#anchor`, no path) validates against the entry's *own* headings. A miss is a **warning**, not an error, since it is a dangling reference like a broken link, not a malformation (keeps exit `0`; see [broken links as warning](./002-broken-links-warning.md)).

**To design before building:**

- **Slug algorithm.** Must match the renderer users target. GitHub-flavored is the sensible baseline (lowercase, spaces → `-`, strip punctuation except `-`); document the rule so the check is predictable. Obsidian differs (keeps case, spaces → `%20`-ish): pick one, state it, don't try to satisfy every renderer.
- **Duplicate headings.** GitHub disambiguates a repeated heading with `-1`, `-2`. Support the suffix, or (simpler v1) accept the base slug and note the limitation.
- **Out of scope initially:** Obsidian block references (`#^blockid`) and HTML anchors (`<a name>`); don't warn on `#^…` so block refs aren't false-positived.
- **Interaction with `move`/rename:** renaming a heading silently breaks inbound anchors. Out of scope here, but this check is what would *surface* it; note it as a follow-up rather than solving anchor rewriting now.

**Acceptance:** a link to a nonexistent heading warns (naming the file and the bad anchor); a valid anchor passes; a self-anchor resolves against the entry's own headings; `#^blockid` is not flagged. Tests cover slug matching (case, spaces, punctuation) and the warning severity (bundle still exits `0`).
