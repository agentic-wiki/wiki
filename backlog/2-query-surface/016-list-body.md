---
type: task
title: "list --body: bodies in the same json call"
status: todo
priority: low
tags: [feature, query]
---

`wiki list --format json` carries every entry's full frontmatter plus `_path`, which makes it a one-call snapshot of a whole bundle's metadata. It carries no **body**, so a consumer that wants to show content alongside metadata has to follow up with one `wiki read` per entry: N process spawns, each rebuilding the whole index, to fetch text the `list` pass already had open.

Surfaced by `wikanban` (card excerpts on a board), but it applies to any renderer, exporter, or static-site generator over a bundle.

**Proposal:** `wiki list --body`, json only, adding a `body` key per entry (frontmatter stripped, exactly what `read` returns).

- **json only.** Bodies are multi-line; they do not belong in text/csv/tsv output. Reject the flag on other formats rather than emitting something unusable.
- **Opt-in**, because it changes the cost profile: `list` reads bodies on demand today (only `SortTime` stats timestamp-less entries), so the default stays cheap.
- **Reuses `Entry.Body()`**, so there is one definition of "the body" across `read` and `list`.
- The key is `body`, matching `read --format json`'s existing `{_path, type, body}` shape. Note the collision risk with a user's own `body:` frontmatter field, which `MarshalJSON` emits verbatim; either accept that it wins (consistent with frontmatter being the user's namespace) or reserve `_body` like `_path`. **Leaning `_body`**, for the same reason `_path` earned its underscore.

**Acceptance:** `list --body --format json` includes each entry's body; other formats error; the flag composes with `--where`/`--prefix`/`--sort`; one bundle read, no extra spawns. Consider the same flag on `search`.
