# How this base is organized

This is the **workflow** layer: the conventions for *this* base, on top of what [AGENTS.md](/AGENTS.md) and the `wiki` tool define. It is a starting point: **edit it to fit how you work.**

This is the `default` workflow: a general-purpose knowledge base (notes, concepts, datasets) with a task backlog. Other starter workflows will exist (`wiki init --workflow <name>`); or just reshape this file.

**This base is git-managed.** Commits are its history and its undo, and the workflow assumes it: an agent can capture, move, and groom freely, and you can always revert. Pull before editing, and commit in batches once `wiki check` passes.

## Structure

- **Group by domain, not by type.** A travel plan (`concept`), its flights (`dataset`), and the trip (`event`) sit together in `personal/travel/`, sliced by `type` on demand, not scattered across `concepts/ datasets/ events/`.
- **Keep the top levels stable and relatively shallow** (2–3 folders deep). The deciding question for a folder is *what it is*. A **collection** folder (`projects/`, `research/`) holds otherwise-independent entries; its `index.md` is just the entry point that links what's inside, so you and an agent read top-down. A folder that is a single **concept** with parts (one initiative, one product) is load-bearing: front it with a typed `thing.md` beside its `thing/` folder, since a concept needs a `type` and a link target, and a typeless `thing/index.md` can be neither.
- Types are free-form: use whatever kinds fit, and `wiki property type --counts` shows what's in use. To enforce a fixed set, uncomment (and edit) `types` in `wiki.toml`; `wiki check` then errors on any undeclared type. The commented list there is a suggested starting vocabulary.

A shape to steal (pick buckets that match how *you* think, not these):

```
my-base/
├── index.md
├── inbox/             # rough drafts (type: draft) until promoted
├── projects/          # grouping: one entry per initiative
│   ├── index.md       # the map: links each initiative
│   ├── acme-migration.md      # the initiative (a typed hub) ...
│   ├── acme-migration/        # ... its notes, specs, sub-pages
│   └── website-redo.md
├── products/
│   ├── index.md
│   ├── checkout-v2.md
│   └── billing.md
├── research/          # topics you're digging into
│   ├── index.md
│   ├── llm-agents.md          # a topic; grows an llm-agents/ folder if it needs depth
│   └── vector-search.md
├── content/           # ideas → drafts → published
│   ├── index.md
│   ├── ideas/
│   ├── articles/
│   └── watch-later/
└── tasks/             # the backlog (see below)
    ├── index.md       # the board: goals + ## Now / ## Next
    ├── active/        # committed work, linked from the board
    ├── backlog/       # unscheduled / someday
    └── archive/       # shipped or dropped
```

A grouping folder's `index.md` is the map for its level; a folder that is one thing is fronted by its typed `thing.md`, not an `index.md`. The leaves are entries.

## Capture → promote (the inbox)

Unclassified thoughts land in `inbox/` as `type: draft`, so `wiki list --where type=draft` is the to-refine queue:

1. **Capture** a new `inbox/<slug>.md` with `type: draft` and whatever you know. A binary (a PDF) goes in a gitignored `inbox/resources/`, pointed at by the draft's `resource:`.
2. **Refine** it: read it back, sharpen, fill it in.
3. **Promote** it: set the real `type`, `wiki move` it into its domain, link it from the domain `index.md`.

Fully-formed knowledge can be created in place; the inbox is only for unrefined drafts.

## Backlog

Treat tasks as a **backlog**, not a flat checklist. A `tasks/index.md` (or the root `index.md`) is the board:

- **Goals first.** Open the board with the one or two overarching objectives the current work serves, so "what's next" always has a *why*.
- **Group by schedule, link the entries.** Under the goals, group work into `## Now` / `## Next` sections (scheduling lives here, not in folders). A non-trivial task is a `type: task` entry referenced by a **plain link** (the entry's `status` is the source of truth, so there's no checkbox to drift); a genuinely trivial to-do can be an inline `- [ ]` right on the board.
- **Entries hold the detail.** Each `type: task` file carries frontmatter like `status`, `priority`, and `tags` (`feature`/`bug`/`debt`/`chore`), plus links to related entries. Keep committed tasks in `tasks/active/` and parked ones in `tasks/backlog/`; a task's progress is its `status`, not a folder to shuffle it between.
- **Archive the past.** Move shipped or dead tasks to `tasks/archive/` (or delete them once their value is captured elsewhere), so the board stays a lean snapshot of what's next. If you keep `tasks/backlog/` or `tasks/archive/`, add `ignore_orphans = ["tasks/backlog/**", "tasks/archive/**"]` to `wiki.toml` so parked work is not flagged as orphans.

```markdown
# Backlog

**Goal:** ship v1 of the importer this quarter.

## Now
- 🔵 [CSV header parser](/tasks/active/csv-parser.md)
- 🔴 [Timezone bug in ingest](/tasks/active/tz-bug.md)
- [ ] bump the changelog       # a trivial inline to-do, no entry

## Next
- [Dedup incoming rows](/tasks/active/dedup.md)
```

A plain-linked task's truth is its entry's `status` (done work leaves the board); an inline `- [ ]` to-do is just checked off in place. A subtle leading emoji (🔵 in progress, 🔴 blocked, ✅ done) shows progress at a glance; skip it when the grouping already says it.

## Grooming

Little and often. Aim for a base that gets **more discoverable** over time, so **change something only when it adds that value (never restructure for its own sake):**

- Run `wiki check` and fix what it flags. Turn `wiki unresolved` links into entries when it helps. Re-home `wiki orphans`.
- **Surface redundancy:** true duplicates to merge (two entries for one thing; keep similar-but-distinct ones separate and linked), `k8s`/`kubernetes`-style tag variants to consolidate, a cluster of entries circling a missing hub that wants its own page (a grouping's `index.md`, or a `thing.md` if they orbit one thing).
- Skim the stalest entries (`wiki list --sort=timestamp --reverse`) and add the links between related ones that were never made.
- Optionally keep a root `LEARNINGS.md` (give it a `type`): a running list of gaps and inconsistencies you notice while working, to batch-fix later.
- When grooming reclassifies or merges entries, an optional dated note in the folder's `log.md` records *why*.

## Make it yours

Nothing here is fixed beyond "every entry has a `type`." Have one inbox or none, a backlog or just inline checkboxes, these folders or your own. Rewrite this file to match your base; [AGENTS.md](/AGENTS.md) points here for the specifics.
