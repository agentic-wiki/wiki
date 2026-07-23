---
type: task
title: check warns on malformed wikilink/markdown hybrid links
status: done
priority: medium
tags: [dx, conformance]
---

**Done:** `wiki check` now warns on a `[[text](/x.md)]` hybrid (Obsidian-style outer brackets wrapped around a standard markdown link). Warning severity, so a bundle with one still exits `0`, like a broken link.

**Why it was invisible.** The string falls into a blind spot between both link parsers:

- `internal/wikilink` looks for `[[ … ]]`; there is no closing `]]` (the brackets are `[`, `[`, `]`, `)`, `]`), so it sees nothing (no wikilink warning, `tidy --wikilinks` no-op).
- `internal/parse`'s `linkRe` matches the *inner* `[…](…)`, greedily pulling the leading `[` into the link **text**. The result is a valid, resolvable `Absolute` link, so `Broken()` is empty and `check` reported "ok". The trailing literal `]` after the `)` isn't captured at all. In a renderer this shows as `[` + working link + `]`: stray brackets around a link the author never meant.

**Detection.** The reliable tell is a `[` captured into the link text: `linkRe`'s text group (`[^\]]*`) can't hold a `]`, but it *can* swallow the outer `[`. `Check` scans every internal link an entry carries (new `Entry.allLinks()` over `Links` + `SelfAnchors` + `Outside`) and warns `malformed link syntax (stray '[' in link text) -> [text](raw)` on a hit. Narrow by design (the confirmed hybrid signature), so a well-formed `[text](/x.md)` stays silent.

**Out of scope (by design):**
- A hybrid whose inner target is an external URL (`[[text](https://…)]`): the external bucket isn't stored on the entry, so it isn't scanned. Internal links were the reported case.
- The trailing-only typo `[text](/x.md)]`: no `[` reaches the text, so it isn't flagged. The `[[…]]`-hybrid always carries the leading `[` and is caught.
- No autofix. Rewriting malformed input is riskier than flagging it; `check` surfaces, the author fixes.
- A valid CommonMark nested-bracket text like `[a [b] c](/x.md)` would also trip the `[`-in-text signal (our regex mishandles it anyway). Rare; accepted as a false positive rather than adding a bracket-balancer.

**Acceptance:** a `[[text](/x.md)]` hybrid warns as malformed (naming the reconstructed link); its inner target still resolves and is never reported broken; a well-formed markdown link does not warn; severity is warning (bundle exits `0`). Unit test in `index_test.go` (hybrid warns + not broken + not an error; clean link silent) + CLI smoke.
