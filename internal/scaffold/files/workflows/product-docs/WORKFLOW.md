# How these docs work

This is the **workflow** layer for a `product-docs` bundle: the conventions on top of what [AGENTS.md](/AGENTS.md) and the `wiki` tool define. It is a starting point: **edit it to fit your product(s).**

This is **wiki-first product documentation**, in two layers:

- **Concepts, the graph.** Atomic `concept`, `reference`, and `example` entries, one idea per page, richly linked to each other. This is the substance, consumed *non-linearly*: you land on a page and follow links (`wiki links` / `backlinks`) to related ones.
- **Guides, the linear layer.** `guide` entries that read top-to-bottom (a tutorial, a how-to) and **link into** the concepts for depth, never restating them. A thin, optional layer over the graph.

The concepts are the source of truth; guides are paths through them.

**This bundle is git-managed.** Commits are its history and its undo; pull before editing, commit in batches once `wiki check` passes.

## First run: pin your conventions

Ships as a **template with options**. Before writing docs, commit the primitives with the user and **edit this file to lock them**:

- **Product boundaries** (the key decision): start with **one product, laid out flat**, concept/reference/example entries at the root and a `guides/` folder. Add a **folder per product** (`checkout/`, `billing/`) only when a second product will coexist; each product folder then repeats the same shape. (If products will share most concepts, an alternative is one graph tagged by a `product:` field, decide up front, reshaping later is costly.)
- **The types**: `concept` (an idea/term), `reference` (precise lookup: API, config, **data model / schema**, CLI), `guide` (a linear tutorial or how-to), `example` (a worked, copy-pasteable sample). This is a starting set, **extend it to fit the product**: a `model` type if you document many data entities, split `tutorial` from `howto` if you follow Diátaxis, and so on. Fold `reference` into `concept` if the split won't earn its keep.
- **Entry naming and the map**: file slugs (`rate-limiting.md`, lowercase-hyphenated), and confirming each product's `index.md` is a hand-curated entry point / concept map (recommended, see *The entry point* below).

Then scaffold a **skeleton** the user validates: one product folder, three or four linked concepts, one guide that links into them, and the product `index.md`. Confirm it reads right, then fill in. **Delete this section when done.**

## Structure

Start with **one product, flat**: concept/reference/example entries at the root, guides in a `guides/` folder, and `index.md` as the entry point.

```
my-docs/
├── index.md              # entry point: intro + links to guides and the key concepts to start from
├── guides/               # optional linear layer (type: guide)
│   ├── index.md          # guides in reading order
│   └── getting-started.md
├── idempotency.md        (type: concept)
├── webhooks.md           (type: concept)
├── api-tokens.md         (type: reference)
├── first-payment.md      (type: example)
├── inbox/                # type: draft (rough captures; ignore_orphans'd)
└── log.md                # dated record of enrichment / sync passes (see Ingesting)
```

**Multiple products?** Add one folder level, a folder per product (`checkout/`, `billing/`), each repeating the shape above (its own concepts + `guides/` + `index.md`); the root `index.md` then links to each product. Keep it shallow either way: the graph (links), not the folder tree, is how readers navigate.

## The entry point (`index.md`)

A reader (or an agent) lands on `index.md`. Make it the front door, hand-curated, not a dump of everything:

- An introduction to the product.
- Links to the **guides** (if any): the guided, linear way in.
- Links to the **key concepts** to start from, for a reader not following a guide (the handful of pages the rest of the graph hangs off).

Guides are the **parallel entry layer**: when they exist they are the primary way in; when they don't, `index.md`'s links to the key concepts are. Keep `index.md` to the important starting points, not every page, that is what queries are for.

## Concepts: the graph

Each concept is **one atomic idea**, defined **once**, and linked to what it relates to:

- **Define once, link everywhere.** A term is explained on its own page; every other page that mentions it **links** to that page instead of re-explaining it (a link is a reference, not a copy). This one rule is what keeps docs consistent: the definition changes in a single place.
- **Link generously.** The value is the graph. `wiki backlinks /checkout/idempotency.md` shows everything that depends on the concept; `wiki links` shows what it builds on.
- **concept vs reference**: a `concept` explains an idea ("what idempotency is, and why"); a `reference` is precise lookup material ("the `/charges` endpoint and its fields"). Same graph, different texture. Split them when readers want "understand" apart from "look up"; merge them if that is overkill.

`wiki unresolved` is your **to-write list**: a link to a concept you haven't written yet is not an error, it is a promise. Writing docs is largely turning `unresolved` into pages.

## Guides: the linear layer

A **guide** (`type: guide`) walks a reader through a task start to finish, **linking into concepts** for depth rather than restating them. It comes in two sizes:

- **A single page** for something short.
- **A sequence** for anything longer: one `type: guide` entry per chapter/step, each with a **next** and **previous** link so the reader clicks through in order (like a book's chapters). Keep the chapters in a folder (`guides/getting-started/01-install.md`, `02-configure.md`, …) with a short landing page listing them in order.

Rules either way:

- A guide **references** concepts, it never duplicates them: "set up a webhook (see [Webhooks](/webhooks.md))", not a re-explanation. If you are tempted to explain a concept inside a guide, that concept wants its own page.
- **Order is explicit**, since a guide is read start to finish (unlike a concept): the next/prev links within a sequence, and `guides/index.md` listing the guides (and a sequence's chapters) in reading order.
- Guides are entry points, so little links *to* the first page, that is fine; link it from `guides/index.md` (and the product `index.md`) so it is reachable and not an orphan. Within a sequence the prev/next links keep the middle chapters linked, so none show up as orphans.

## Multiple products

When products coexist (a folder each, see *Structure*), a concept **shared** by two products still has a single home, the product that owns it, or a top-level `shared/`, and the other product **links** to it, never copies it (define-once-link-everywhere applies across products too). If most concepts turn out shared, that is the signal to switch to one graph with a `product:` field instead of folders (a first-run choice).

## Ingesting from source

Docs are usually distilled from **source material**: source code, specs, tickets, existing docs, a subject-matter expert. That material is the *input*, not the wiki; your job is to turn it into atomic, linked concepts. Do it in two phases, and gradually, not one giant edit:

1. **Extract, into an intermediate place.** Pull the relevant facts out of the source into rough `type: draft` notes in `inbox/` (one draft per source, area, or question), *before* shaping them. This is a good task to delegate to a separate extraction pass or agent: point it at the source, have it dump facts as drafts, no wiki conventions required yet. `inbox/` is `ignore_orphans`'d, so these unshaped drafts don't pollute `wiki orphans`.
2. **Build incrementally, from the drafts.** Promote drafts into concept/reference entries a few at a time: write one atomic page, link it to what exists, `wiki check`, repeat. Small batches keep the graph consistent and reviewable, and let `wiki unresolved` guide you, each concept you write names others, and those names become your next to-write items. Prefer this over a single massive dump: a hundred pages landed at once are unlinked, unreviewed, and inconsistent.

Keep **provenance**: link a concept back to where it came from (a `type: source` entry, or a `source:` / `resource:` field), so a fact can be re-checked against the source later.

**Record each pass in `log.md`, dated and high-level.** One dated line per enrichment or sync pass (what source you covered, roughly what you added), not one per edit, git already holds the fine detail. On the next pass, read the last date and re-sync only what changed in the source since then, rather than re-deriving everything. For docs kept in step with a moving product, that dated, high-level trail is what makes incremental sync cheap.

Scale the ceremony to the job: a small, well-understood product can skip the inbox and be written directly; a large or unfamiliar one benefits from the extract-then-iterate split. Either way, the unit is the **atomic concept**, never one monster page.

## Grooming

- `wiki check`, then `wiki unresolved` (the to-write list) and `wiki orphans` (a concept nothing links to is **undiscoverable**, link it in from a related concept or the product index).
- Reread the stalest pages (`wiki list --sort=timestamp --reverse`) as the product changes.
- Watch for a concept explained in two places: merge to one and link.

## Answering everyday questions

Readers and agents hit **search** constantly, and the graph is only as good as its linking, so **index well**: every concept linked from where it is relevant, `wiki orphans` empty, `wiki unresolved` worked down. Bad indexing shows up as unfindable pages. The palette:

**Find**

```sh
wiki search "idempotency key"          # free-text over every page (frontmatter + body)
wiki search "webhook" --lines          # matching lines as file:line
wiki list --where type=concept         # all concepts
wiki list --where type=guide           # the guides
wiki list --where type=reference       # reference pages only
wiki list --where type=example         # worked examples
wiki list --prefix checkout/           # everything for one product (multi-product layout)
```

**Navigate the graph**

```sh
wiki backlinks /idempotency.md         # what depends on this concept
wiki links /guides/first-payment.md    # what a page references (its prerequisites)
wiki read /index.md                    # the hand-curated entry point (intro + starting links)
```

**Keep quality high** (docs live or die by this)

```sh
wiki unresolved                        # promised-but-unwritten pages: the to-write queue
wiki orphans                           # concepts nothing links to: undiscoverable, link them in
wiki list --sort=timestamp --reverse   # stalest pages, re-check against the product
```

- **"What is X?"**: `wiki search "X"`, then `wiki read` the concept.
- **"What must I understand first?"**: `wiki links <page>` (its prerequisites) and `wiki backlinks <page>` (what builds on it).
- **"What's missing or unreachable?"**: `wiki unresolved` (unwritten) plus `wiki orphans` (unlinked).

Anything beyond a filter (a coverage report, a link-density audit) is a small skill over `wiki list --format json`.

## Make it yours

Nothing here is fixed beyond "every entry has a `type`." One product or many, `reference` split out or folded in, guides or none: reshape this file and the folders to fit.
