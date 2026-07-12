---
type: task
title: backlinks/links list each linking page once (per-page granularity)
status: todo
priority: low
tags: [feature, graph]
---

`wiki backlinks` lists one row per link *occurrence* (`file:line  text`), so a page that links a target many times shows many rows. Usually harmless (a client is linked once per project), but a dataset whose N rows each link the same entity floods that entity's backlinks with N lines from one file. The fix is to make **one row per linking page** the natural unit: `backlinks` conceptually answers "what links here?", and the honest answer is a *set of pages*, the graph edge is page→page, so a page mentioning Acme three times still has one relationship to it. The current per-occurrence output leaks raw link hits rather than the graph.

Born from the dataset-links discussion alongside [wiki table --links](../2-query-surface/014-table-link-transform.md): that one makes a linked cell *extract* cleanly, this one keeps such a cell from *flooding the graph output*. Together they make per-row cell links viable for small datasets.

**Recommendation: per-page is the default; `--lines` expands.**

Default to one row per linking page. The two output layers have different jobs, so hold them to different bars (the standing rule: text is for humans, `--format json`/`csv`/`tsv` is the machine contract):

- **text (human): a readable, adaptive locator is fine.** Show the line(s) when a page links a few times, collapse to a count when it links many:

  ```
  /projects/acme-migration.md:7
  /projects/website.md:12,40
  /invoices.md ×500
  ```

  This is deliberately irregular (`:7` / `:12,40` / `×500`), and that is OK: nobody should parse text output.

- **json/csv/tsv (machine): regular and predictable.** One object/row per source, fixed fields, no adaptive shape:
  - default: `{from, to, count}` (csv/tsv header `from,to,count`).
  - `--lines`: today's per-occurrence `{from, to, text, line}`.

  Every row in a given mode has the same fields, so a consumer never branches on shape. The `×500` vs `:12,40` adaptivity lives only in text.

`--lines` opts back into per-occurrence rows in every format for the "show me every reference" case.

**Trade-off:** breaking change to `backlinks`/`links` default output (a script parsing text `file:line` must switch to `--lines`, or better to `--format csv`/`json`). Judged worth it pre-1.0: it's the correct graph model, matches "one intuitive default", and makes text consistent with `orphans`/`unresolved`. Confirm the flip before building.

**Naming note (why not `--unique`/`--group`):** with collapse as the default, no collapse flag is needed. `--lines` names the expansion, the clearer, smaller surface.

**To settle:**

- **Text locator threshold.** Two candidate shapes for a page linked many times (text-only; the structured formats always carry the exact `count`):
  - *Recommended:* list all lines up to a small cap (≈10), else collapse to `×N`. Below the cap the human wants *where* (line numbers); above it they want *how many* (the count is the signal, individual lines are noise), so lead with the number. Cases stay short.
  - *Variant (list-then-more):* always comma-separate, truncating past ~20 with `:4,8,15,… +N more`. Keeps a sample and navigability, but buries the count and makes long rows; "+N more" also forces the reader to sum. If kept, lead with the total (`×500 (4,8,15,…)`) rather than a bare truncated list.
  - Skip a per-row "see `--format json`" hint (repeated clutter); the truncation/`×N` already implies more, and json/`--lines` is the documented full view.
- **Scope.** Apply consistently to `backlinks` and `links`; the structured default is one object per source file.
- **`orphans`/`unresolved`** are unaffected (already per file / per target).

**Acceptance:** `backlinks`/`links` emit one row per linking page by default. Text shows an adaptive locator (`:line`, `:l1,l2`, or `×N`); json/csv are regular (`from,to,count`, same fields every row). `--lines` restores per-occurrence rows in every format (`file:line  text`; json/csv `+text,line`). Tests: a page linking a target N times is one row by default (correct `count`) and N rows under `--lines`; json fields are identical across rows within a mode.
