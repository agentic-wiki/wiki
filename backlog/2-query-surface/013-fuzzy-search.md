---
type: task
title: "search --fuzzy: opt-in typo-tolerant matching"
status: todo
priority: low
tags: [feature, query]
---

An opt-in `wiki search --fuzzy` that tolerates typos (Levenshtein distance), for when you misremember a spelling. Deliberately **not** the default: plain `search` stays an exact, explainable substring match (all-words AND default / `--any` / `--exact`, task 012). Fuzzy is the escape hatch when that finds nothing. See [search: match by word](/2-query-surface/012-search-any-word.md).

Efficiency is not the constraint (full-text scan is already grep-scale, and bounded Levenshtein is ~15 zero-dep lines with early-exit once a row exceeds the max distance). The cost is that fuzzy forces a **ranking** and a **threshold**, which is why it is a separate mode, not a default.

Target behaviour:
- `--fuzzy` matches **per word**: a query token matches a line if some word in that line is within the edit distance. This is a shift from the current substring model (fuzzy is word-oriented), so the text must be tokenized into words (split on whitespace/punctuation, lowercased both sides).
- **Ranking**: results sorted by Levenshtein distance, **lowest first** (exact/near-exact on top); ties broken by path (the current stable order). An entry's rank is its **best (minimum)** matching-word distance.
- **`--distance N`**: the max edit distance, default **length-scaled: 1 for short query tokens, 2 for longer** (matches the "1–2" intuition and avoids short words matching everything). A fixed `--distance N` overrides. Guard: never allow distance ≥ token length (else a 2-char token matches any 2-char word); short tokens effectively require near-exact.
- **`--limit N`**: cap the number of hits after ranking, **default `0` (no cap), opt-in only**. One consistent rule across every search mode: no fuzzy-special default and no "did the user set it?" flag detection. Because fuzzy can match a lot, the docs (below) tell users to pair `--fuzzy` with `--limit`; the tool does not cap for them.

Decisions to pin before building:
- **Show the score?** Ranking by an invisible number is opaque. Recommend surfacing the distance: add a `distance` field to `SearchHit` json (`omitempty`, so non-fuzzy output is byte-identical) and show it in text only in fuzzy mode (e.g. a leading `d=1`). Confirm whether csv/tsv should carry it too.
- **When a set `--limit` truncates, say so** (a `… (N more, raise --limit)` stderr note), so an opt-in cap never reads as "that's everything." No silent truncation. (With the default `0` there is nothing to truncate.)

Mode interaction:
- `--fuzzy` composes with the word modes: default (AND) = every query token must fuzzy-match some word on the line; `--any` = any one query token fuzzy-matches.
- `--fuzzy` + `--exact` is contradictory (verbatim phrase vs approximate) → usage error (exit 2), like the existing `--any`/`--exact` conflict. `--fuzzy` is orthogonal to the AND/`--any` choice.

Docs to sweep when shipped (not before, so a fresh scaffold never documents a phantom flag): the `search` usage string, README, and AGENTS.md search wording. **Every `WORKFLOW.md` (all four starters) gets a short warning** that `--fuzzy` is unranked-flood-prone and should be paired with `--limit`, since the default is no cap. (AGENTS.md is the shared home for the full description; the per-workflow line is just the pairing reminder.)

Tests: single-token typo within/over the distance boundary; length-scaled default (short token needs distance 1, long token allows 2); ranking order (distance ascending, path tiebreak); default AND requiring every token to fuzzy-match (and `--any` relaxing to one); `--limit` truncation + the note (only when set); `--fuzzy --exact` rejected.
