# How this base is organized

This is the **workflow** layer: the conventions for *this* base, on top of what [AGENTS.md](AGENTS.md) and the `wiki` tool define. It is a starting point: **edit it to fit how you work.**

This is the `default` workflow: a general-purpose knowledge base (notes, concepts, datasets) with a task backlog. Other starter workflows will exist (`wiki init --workflow <name>`); or just reshape this file.

## Structure

- **Group by domain, not by type.** A travel plan (`concept`), its flights (`dataset`), and the trip (`event`) sit together in `personal/travel/`, sliced by `type` on demand, not scattered across `concepts/ datasets/ events/`.
- **Keep the top levels stable and relatively shallow** (2–3 folders deep). Each folder gets an `index.md` linking to what is inside, so you and an agent read top-down.
- The starting `type` vocabulary is in `wiki.toml`; extend it as you introduce new kinds.

A shape to steal (pick buckets that match how *you* think, not these):

```
my-base/
├── index.md
├── inbox/             # rough drafts (type: draft) until promoted
├── projects/          # one folder per initiative
│   ├── index.md
│   ├── acme-migration/
│   └── website-redo/
├── products/
│   ├── index.md
│   ├── checkout-v2/
│   └── billing/
├── research/          # topics you're digging into
│   ├── index.md
│   ├── llm-agents/
│   └── vector-search/
├── content/           # ideas → drafts → published
│   ├── index.md
│   ├── ideas/
│   ├── articles/
│   └── watch-later/
└── tasks/             # the backlog (see below)
    ├── index.md       # the board: goals + grouped checkboxes
    ├── now/
    ├── next/
    ├── sometime/
    └── archive/
```

Each `index.md` is the map for its level; the leaves are entries.

## Capture → promote (the inbox)

Unclassified thoughts land in `inbox/` as `type: draft`, so `wiki list --type draft` is the to-refine queue:

1. **Capture** a new `inbox/<slug>.md` with `type: draft` and whatever you know. A binary (a PDF) goes in a gitignored `inbox/resources/`, pointed at by the draft's `resource:`.
2. **Refine** it: read it back, sharpen, fill it in.
3. **Promote** it: set the real `type`, `wiki move` it into its domain, link it from the domain `index.md`.

Fully-formed knowledge can be created in place; the inbox is only for unrefined drafts.

## Backlog

Treat tasks as a **backlog**, not a flat checklist. A `tasks/index.md` (or the root `index.md`) is the board:

- **Goals first.** Open the board with the one or two overarching objectives the current work serves, so "what's next" always has a *why*.
- **Grouped, linked checkboxes.** Under the goals, group tasks (by timeframe (`now` / `next` / `sometime`) or by `status`) as `- [ ]` checkboxes that link to the real entries.
- **Entries hold the detail.** Each non-trivial task is a `type: task` file in a subfolder (`tasks/now/`, `tasks/next/`, …) carrying frontmatter like `status`, `priority`, and `tags` (`feature`/`bug`/`debt`/`chore`), plus links to related entries. Trivial tasks can stay inline `- [ ]` without a full entry.
- **Archive the past.** Move shipped or dead tasks to `tasks/archive/` (or delete them once their value is captured elsewhere), so the board stays a lean snapshot of what's next.

```markdown
# Backlog

**Goal:** ship v1 of the importer this quarter.

## Now
- [ ] 🔵 [CSV header parser](/tasks/now/csv-parser.md)
- [ ] 🔴 [Timezone bug in ingest](/tasks/now/tz-bug.md)

## Next
- [ ] [Dedup incoming rows](/tasks/next/dedup.md)
```

A checkbox is checked exactly when its entry's `status` is `done`: update both in one change. If you *don't* group by status, a subtle leading emoji can carry it at a glance (🔵 in progress, 🔴 blocked, ✅ done); skip the emoji when the grouping already says it.

## Grooming

Little and often. Aim for a base that gets **more discoverable** over time, so **change something only when it adds that value (never restructure for its own sake):**

- Run `wiki check` and fix what it flags. Turn `wiki unresolved` links into entries when it helps. Re-home `wiki orphans`.
- **Surface redundancy:** duplicate or near-duplicate entries to merge, `k8s`/`kubernetes`-style tag variants to consolidate, a cluster of entries circling a missing hub that wants its own `index.md`.
- Skim the stalest entries (`wiki list --sort=timestamp --reverse`) and add the links between related ones that were never made.
- Optionally keep a root `LEARNINGS.md` (give it a `type`): a running list of gaps and inconsistencies you notice while working, to batch-fix later.
- When grooming reclassifies or merges entries, an optional dated note in the folder's `log.md` records *why*.

## Make it yours

Nothing here is fixed beyond "every entry has a `type`." Have one inbox or none, a backlog or just inline checkboxes, these folders or your own. Rewrite this file to match your base; [AGENTS.md](AGENTS.md) points here for the specifics.
