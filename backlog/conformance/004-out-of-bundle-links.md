---
type: task
title: out-of-bundle links warn as their own category, not "broken"
status: done
priority: medium
tags: [conformance, dx]
---

A link that resolves *outside* the bundle (a relative `../PRD.md` climbing above the bundle root, common when a bundle is nested in a repo) was silently resolved to a fabricated in-bundle path — `path.Join` clamps `..` at the root, so `../PRD.md` became `/PRD.md` — and then reported as a **broken link**. Wrong twice: the file is not missing (it lives outside the bundle), and the reported target (`/PRD.md`) was never what the author wrote. The same clamping fed `tidy --links`, which would silently rewrite `../PRD.md` to `/PRD.md` on disk, turning a valid reference into a broken one.

The escape guard itself is deliberate and stays: the tool must never stat or read a path outside the bundle, or `check` becomes a host-filesystem oracle. That is the `FileExists` `withinDir` guard.

**Change:**
- `normalizeLink` resolves a target to its final on-disk path and uses `withinDir` (the same containment guard `FileExists` uses) to decide in/out — one containment check for the whole codebase, correct for relative *and* absolute `..`.
- A link resolving outside the bundle is no longer an edge and no longer "broken". It is collected per entry (`Entry.Outside`) and surfaced by `check` as its own advisory, `out-of-bundle link -> ../PRD.md`, at warning level so `check` still exits `0`. See the sibling [broken-links demotion](/conformance/002-broken-links-warning.md).
- `tidy --links` leaves out-of-bundle links exactly as authored.
- To silence the advisory, reference the outside file as a code span or a full URL, not a markdown link.

Note: an absolute `/README.md` still resolves in the bundle namespace (`<bundle>/README.md`) and is an ordinary broken link if absent — only a relative climb above the root is out-of-bundle.

**Done:** `normalizeLink` rebuilt on `withinDir`; `resolveLinks` returns in-bundle edges plus out-of-bundle links; new `Entry.Outside`; `check` emits the advisory. Tests: `TestNormalizeLink` escape cases, `TestEscapingLinkIsOutOfBundleNotBroken`, `TestOutOfBundleReferenceWarns`, and `TestAngleBracketLinkResolvesButNotNormalized` moved its relative source into a subdir (a `../` from the root is now correctly out-of-bundle).
