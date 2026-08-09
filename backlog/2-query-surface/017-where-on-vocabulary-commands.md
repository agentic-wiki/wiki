---
type: task
title: "--where on the vocabulary commands (tags, properties, property)"
status: done
priority: medium
tags: [feature, query, consistency]
---

`--prefix` and `--where` are both filters, but only one of them works everywhere. `--prefix` scopes `list`, `search`, `checkboxes`, `tags`, `properties`, and `property`; `--where` works on `list` and `search` alone. So a subtree can be narrowed anywhere, and a field cannot.

The gap shows up the moment a folder holds more than one kind of entry:

```sh
wiki property status --prefix /backlog                 # every status under /backlog…
wiki property status --prefix /backlog --where type=task   # …no such flag
```

A backlog folder that also holds notes, articles, or places reports *their* statuses (`published`, `retired`, `visit-only`) alongside the tasks', with no way to ask the narrower question. Today the workaround is to leave the vocabulary commands behind entirely and post-process `list --format json` through `jq`, which is a lot of ceremony for "how many tasks are in each status".

Surfaced by `wikanban`, which used `property status --prefix` to discover a board's columns and got a column per foreign status. It no longer needs this (columns now come from the board's own filtered entries, which is both more accurate and one fewer spawn), so **nothing is blocked on it**. Filed because the asymmetry is real on its own terms.

**Proposal:** accept `--where key=value` / `key!=value` on `tags`, `properties`, and `property`, with exactly the semantics `list` has (repeatable, ANDed, list-match-any, empty value tests emptiness).

- The filter already exists as `index.PropFilter` and `Filter(prefix, props)`; these commands currently call the prefix-only path, so this is mostly threading the same flag through, not new matching logic.
- `checkboxes` is the interesting one: its unit is a `- [ ]` line, not an entry, but `--where` would still be a meaningful *entry* filter ("open checkboxes on blocked tasks"). Decide whether to include it or keep it prefix-only, and say which in the help.
- Keeps the story simple to explain: **two filters, `--prefix` for where and `--where` for what, both available wherever a set of entries is being narrowed.**

**Acceptance:** `wiki property status --where type=task` reports only tasks' statuses; the same for `tags` and `properties`; `--where` composes with `--prefix`; docs and `AGENTS.md` updated so the two filters are described as the pair they are.

**Done.** `--where` accepted on `tags`, `properties`, `property`, and `checkboxes`, with `list`'s exact semantics and composing with `--prefix`.

`checkboxes` was included: its unit is a `- [ ]` line, but its *scope* is a set of entries, so the filter is meaningful there and excluding it would have made the story "two filters everywhere, except one place" — which is the asymmetry this task existed to remove. A named `[file]` stays explicit and ignores both filters.

Mostly threading, as expected: the counters already routed through `Index.Filter(prefix, nil)`, so they took a `props []PropFilter` parameter and passed it on. `checkboxes` was the exception — it carried its own inline copy of prefix matching rather than calling `Filter`, behaviourally identical but a second implementation of the same rule; it now routes through `Filter` like everything else.

Help, README, and CHANGELOG describe the pair rather than two separate flags: **`--prefix` for where, `--where` for what.**
