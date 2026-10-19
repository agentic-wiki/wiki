---
type: task
title: "dump: leave log.md out by default (--logs brings it back)"
status: done
priority: medium
tags: [feature, query]
---

`log.md` is the side narrative of a folder (OKF §7), not its content: an agent reading `llms-full.txt` wants what the base says, not the history of how it got there. So `wiki dump` leaves every `log.md` out, and `--logs` includes them, placed by the usual reading order.

## Decided

- **One rule, no exceptions:** without `--logs` no `log.md` is dumped, even one `--prefix` names directly. `wiki dump --prefix /proj/log.md` prints nothing and exits 0, but warns on stderr when the filters matched only logs, so the empty result is not a mystery: `wiki: warning: only log.md entries matched (1), left out without --logs`.
- **Flag `--logs`**, a bare boolean like `--frontmatter`.
- **Inclusion only.** Logs never ordered anything (an index's links do), so leaving them out moves nothing else.
- **A log left out is not published, so it is not linted for publishing:** its out-of-bundle links are not warned about on stderr.
- **Links *to* a left-out log** are still made root-absolute, so they name an entry the stream does not contain. Not rewritten, but each occurrence (markdown or `[[wikilink]]`, anchor or not) is warned on stderr as `wiki: warning: /z.md:4 link to a left-out log -> /proj/log.md (--logs includes it)`. Only logs left out by the missing `--logs` count: a log `--prefix`/`--where` already excluded is like any filtered-out entry, unwarned, since `--logs` would not bring it back.
- `log.md` is matched by exact base name, case-sensitive, the same as `check`, `orphans` and the reserved-file rule, now through one helper, `Entry.IsLog`.
- Recorded in the CHANGELOG as v0.11.0, a behaviour change of `dump`; the stderr warnings (links to a left-out log, an only-logs empty dump) followed as v0.12.0.

## Tests

- Default dump: logs at the root and in subfolders are absent, everything else unchanged and in the same order.
- `--logs`: they appear exactly where reading order puts them (linked by an index, or by path when not).
- `--prefix` naming a log: empty with the only-logs warning without `--logs`, the log with it. A genuinely empty match stays silent.
- Links to a left-out log: one warning per occurrence, in dump order, for markdown links (anchored, repeated on a line) and wikilinks; none for a log the filters already excluded; none with `--logs`.
- A log's out-of-bundle link warns only with `--logs`.
- `IsLog`: `/log.md` and `/a/log.md` yes; `/changelog.md`, `/log.md/x.md`, `/LOG.md`, `/a/log.markdown` no.

**Done (2026-10-19):** `wiki dump --logs`, filtered in `cmdDump` next to `--prefix`/`--where` (the library's `Dump` already takes the entries to write, so it needed nothing). The logs it drops are kept as a set, which drives both warnings, so they only ever name what `--logs` would bring back. `Entry.IsLog` replaced the three `"log.md"` literals in `check`, `orphans` and the reserved-file rule. Tests: `TestCmdDumpLogs`, `TestIsLog`, and a `finance/log.md` in the smoke fixture (so `status` there counts 7 entries). On the way: `TestCmdDump` leaked the global `rootDir` from its `run --root` call into later tests; it now resets it like `TestGlobalRoot` does.
