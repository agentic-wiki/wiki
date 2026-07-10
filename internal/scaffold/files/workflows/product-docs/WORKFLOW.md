# How these docs work

This is the **workflow** layer for a `product-docs` bundle: the conventions on top of what [AGENTS.md](/AGENTS.md) and the `wiki` tool define. It is a starting point: **edit it to fit your product(s).**

This is **wiki-first product documentation**, in two layers:

- **Concepts, the graph.** Atomic `concept`, `reference`, and `example` entries, one idea per page, richly linked to each other. This is the substance, consumed *non-linearly*: you land on a page and follow links (`wiki links` / `backlinks`) to related ones.
- **Guides, the linear layer.** `guide` entries that read top-to-bottom (a tutorial, a how-to) and **link into** the concepts for depth, never restating them. A thin, optional layer over the graph.

The concepts are the source of truth; guides are paths through them.

**This bundle is git-managed.** Commits are its history and its undo; pull before editing, commit in batches once `wiki check` passes.

## First run: shape the base

Ships as a **template with options**. Before writing docs, settle the base's shape with the user and **edit this file to lock it in**. Two decisions:

- **Product boundaries** (the first decision): one product, or several? For one, the areas below are its top level; for several, it is a folder per product, each repeating that shape (see *Structure*). If products will share most concepts, consider instead a single graph tagged by a `product:` field: decide up front, reshaping later is costly.
- **The initial area map** (the decision that shapes everything else): name the product's major **areas**, the subsystems or domains a reader would look under (`payments/`, `webhooks/`, `auth/`). Start with **two or three real ones per product**, not a speculative taxonomy; add more as the product reveals them. These become the top-level folders, and filing into them from the first page is what stops the base silting into a flat pile (see *Structure*).

**Types are not a first-run question.** The suggested set (`concept`, `reference`, `guide`, `example`, plus `note`/`decision`/`source`, commented in `wiki.toml`) covers the start; use a new kind whenever a real need appears (a `model` type once you document many data entities, `tutorial` vs `howto` if you follow Diátaxis), and fold `reference` into `concept` if that split never earns its keep. Types are free-form by default; if you want them enforced, uncomment `types` in `wiki.toml` and keep it current (`wiki check` then errors on any undeclared type). Grow the vocabulary on demand rather than interrogating the user up front.

Then scaffold a **skeleton** the user validates: the agreed area folders (each with its `index.md`), three or four linked concepts filed into them, one guide that links into them, and the product `index.md`. Confirm it reads right, then fill in. **Delete this section when done.**

## Structure

Give the base its shape on day one. A product's pages live in two physical layers: **graph pages (concept/reference/example) filed by area**, and the **linear `guides/` layer** beside them. Start from the **two or three areas** agreed in *First run*, each a folder with its own `index.md`, and file every new page into the area it belongs to. The root is not a dumping ground: it carries the front-door `index.md`, `raw/`, `log.md`, and only the odd **general-purpose page** that genuinely belongs to no single area (a glossary, say).

```
my-docs/
├── index.md                    # front door: intro + links to guides and each area's key pages
├── glossary.md                 (type: concept)   general-purpose, belongs to no single area
│
├── guides/                     # the linear layer (type: guide): the only layer not area-filed
│   ├── index.md                # guides in reading order
│   ├── getting-started.md      # a single-page guide
│   ├── accepting-payments.md   # a sequence's landing (type: guide): intro + chapter order
│   └── accepting-payments/     # its chapters
│       ├── 01-setup.md         # each chapter links next / prev
│       └── 02-first-charge.md
│
├── payments/                   # an AREA: a subsystem a reader looks under (never a type)
│   ├── index.md                # area map: what's here + where to start
│   ├── idempotency.md          (type: concept)     related but DISTINCT pages,
│   ├── idempotency-keys.md     (type: concept)     co-located and linked, never merged
│   ├── charges.md              (type: reference)
│   └── first-payment.md        (type: example)     one area mixes concept + reference + example
│
├── webhooks/                   # an AREA
│   ├── index.md
│   ├── webhooks.md             (type: concept)
│   ├── signing.md              (type: concept)
│   └── events.md               (type: reference)
│
├── auth/                       # an AREA
│   ├── index.md
│   ├── api-tokens.md           (type: reference)
│   └── oauth.md                (type: concept)
│
├── raw/                        # raw source captures (git-ignored, not wiki entries; see Ingesting)
└── log.md                      # dated record of enrichment / sync passes (see Ingesting)
```

**File by area, from the first page.** Group by **area, not by `type`** (an area folder mixes `concept`, `reference`, and `example`; there is no `concepts/` folder), keep it **one level deep**, and give each area its own `index.md`. Add a new area when a real cluster appears, not before, and don't pre-build a taxonomy of empty folders past the two or three you started with. This is a browsing aid, not a boundary: pages link freely across areas (define-once-link-everywhere), and the graph, not the tree, carries the relationships. Only guides are exempt from area-filing; they live in the `guides/` layer. A genuinely tiny product (a handful of pages that won't grow) can stay flat, but the moment a second related page appears, that is its area asking to exist.

**Multiple products?** Add one folder level, a folder per product (`checkout/`, `billing/`), each repeating the shape above (its own areas + `guides/` + `index.md`); the root `index.md` then links to each product. Keep it shallow either way.

## The entry point (`index.md`)

A reader (or an agent) lands on `index.md`. Make it the front door, hand-curated, not a dump of everything:

- An introduction to the product.
- Links to the **guides** (if any): the guided, linear way in.
- Links to the **key concepts** to start from, for a reader not following a guide (the handful of pages the rest of the graph hangs off).

Guides are the **parallel entry layer**: when they exist they are the primary way in; when they don't, `index.md`'s links to the key concepts are. Keep `index.md` to the important starting points, not every page, that is what queries are for.

## Concepts: the graph

Each concept is **one atomic idea**, defined **once**, and linked to what it relates to:

- **Define once, link everywhere.** A term is explained on its own page; every other page that mentions it **links** to that page instead of re-explaining it (a link is a reference, not a copy). This one rule is what keeps docs consistent: the definition changes in a single place.
- **Distinct ideas stay distinct.** Closely related or similarly named things (a type and its factory, a resource and its setup/lifecycle) are still separate concepts: give each its own page and link them. Being adjacent is a reason to link, not to merge. Atomic cuts both ways, don't split one idea across pages, don't fold two into one. And grouping is not merging: two closely related concepts can share an area folder (see *Structure*) and stay two distinct pages, the folder is where a reader looks, the link is how they relate.
- **Link generously.** The value is the graph. `wiki backlinks /payments/idempotency.md` shows everything that depends on the concept; `wiki links` shows what it builds on.
- **concept vs reference**: a `concept` explains an idea ("what idempotency is, and why"); a `reference` is precise lookup material ("the `/charges` endpoint and its fields"). Same graph, different texture. Split them when readers want "understand" apart from "look up"; merge them if that is overkill.

`wiki unresolved` is your **to-write list**: a link to a concept you haven't written yet is not an error, it is a promise. Writing docs is largely turning `unresolved` into pages.

## Guides: the linear layer

A **guide** (`type: guide`) walks a reader through a task start to finish, **linking into concepts** for depth rather than restating them. It comes in two sizes:

- **A single page** for something short.
- **A sequence** for anything longer: one `type: guide` entry per chapter/step, each with a **next** and **previous** link so the reader clicks through in order (like a book's chapters). Give the sequence a **landing entry** (`guides/accepting-payments.md`, `type: guide`: the intro, listing the chapters in order) and keep the chapters beside it under `guides/accepting-payments/` (`01-setup.md`, `02-first-charge.md`, …). The landing is a real typed guide, not a typeless `.../index.md`.

Rules either way:

- A guide **references** concepts, it never duplicates them: "set up a webhook (see [Webhooks](/webhooks/webhooks.md))", not a re-explanation. If you are tempted to explain a concept inside a guide, that concept wants its own page.
- **Order is explicit**, since a guide is read start to finish (unlike a concept): the next/prev links within a sequence, and `guides/index.md` listing the guides (and a sequence's chapters) in reading order.
- Guides are entry points, so little links *to* the first page, that is fine; link it from `guides/index.md` (and the product `index.md`) so it is reachable and not an orphan. Within a sequence the prev/next links keep the middle chapters linked, so none show up as orphans.

## Multiple products

When products coexist (a folder each, see *Structure*), a concept **shared** by two products still has a single home, the product that owns it, or a top-level `shared/`, and the other product **links** to it, never copies it (define-once-link-everywhere applies across products too). If most concepts turn out shared, that is the signal to switch to the single tagged graph instead (see *First run*).

## Ingesting from source

Docs are usually distilled from **source material**: source code, specs, tickets, existing docs, a subject-matter expert. That material is the *input*, not the wiki; your job is to turn it into atomic, linked concepts. Do it in two phases, and gradually, not one giant edit:

1. **Extract into `raw/`.** Pull the relevant facts out of the source into rough notes in `raw/`, one per source or question, *before* shaping them. Delegate this to a separate extraction pass or agent if it helps: it can dump plain notes with no wiki conventions at all, since `raw/` is **git-ignored and excluded from the index** (nothing there is listed, searched, or checked). It is interim, regenerable scratch you mine into real entries, not the committed base, so losing it just means re-reading the source. The exception is a source you can't re-read (a subject-matter expert's answers): promote those promptly, since nothing in `raw/` survives a fresh checkout.
2. **Build incrementally, from `raw/`.** Promote the raw notes into concept/reference entries a few at a time: write one atomic page, **file it into its area folder** (see *Structure*) rather than leaving everything at the root, link it to what exists, `wiki check`, then **delete the raw note you just mined** (or the lines of it you used) and repeat. `raw/` is thus a shrinking worklist: an empty `raw/` means the batch is done. (A `- [ ]` checkbox inside a raw note would track this too, but `wiki checkboxes` can't see `raw/` since it is unindexed, so deleting is the clearer signal.) Small batches keep the graph consistent and reviewable, and let `wiki unresolved` guide you, each concept you write names others, and those names become your next to-write items. Prefer this over a single massive dump: a hundred pages landed at once are unlinked, unreviewed, and dumped flat at the root.

Keep **provenance**: link a concept back to where it came from (a `type: source` entry, or a `source:` / `resource:` field), so a fact can be re-checked against the source later.

**Record each pass in `log.md`, dated and high-level.** One dated line per enrichment or sync pass (what source you covered, roughly what you added), not one per edit, git already holds the fine detail. On the next pass, read the last date and re-sync only what changed in the source since then, rather than re-deriving everything. For docs kept in step with a moving product, that dated, high-level trail is what makes incremental sync cheap.

Scale the ceremony to the job: a small, well-understood product can skip `raw/` and be written directly; a large or unfamiliar one benefits from the extract-then-iterate split. Either way, the unit is the **atomic concept**, never one monster page.

## Grooming

Groom to make the base more findable. Don't restructure, rename, or re-file just because you can; only do it when it's needed.

- `wiki check`, then `wiki unresolved` (the to-write list) and `wiki orphans` (a concept nothing links to is **undiscoverable**, link it in from a related concept or the product index).
- Reread the stalest pages (`wiki list --sort=timestamp --reverse`) as the product changes.
- Watch for a concept explained in two places: merge to one and link.
- The area map is not frozen: when a cluster outgrows its home add an area (or split one that grew too big), and `wiki move` the pages into it (see *Structure*), which rewrites the links as it goes. If general-purpose pages have collected at the root and a real area has emerged among them, that is the signal to file them. Keep it reasonably shallow, and let the graph keep crossing folders.

## Answering everyday questions

The graph is only as good as its linking, so **index well**: every concept linked from where it is relevant, `wiki orphans` empty, `wiki unresolved` worked down. Bad indexing shows up as unfindable pages.

Beyond AGENTS.md's general palette (`search`, `backlinks`/`links`, `unresolved`/`orphans`), the filters you reach for most here are by type and by cross-cutting tag:

```sh
wiki list --where type=concept                             # concepts (swap for guide / reference / example)
wiki list --where tags=deprecated                          # by cross-cutting tag (status/version/audience), any area
wiki list --where type=reference --where tags=deprecated   # combine fields (repeat --where = AND)
wiki list --prefix checkout/                               # everything for one product (multi-product layout)
```

- **"What is an idempotency key?"**: `wiki search "idempotency key"`, then `wiki read` the concept (every word by default; `--any` broadens, `--exact` matches the phrase).
- **"What must I understand first?"**: `wiki links <page>` (its prerequisites) and `wiki backlinks <page>` (what builds on it).
- **"What's missing or unreachable?"**: `wiki unresolved` (unwritten) plus `wiki orphans` (unlinked).

## Make it yours

Nothing here is fixed beyond "every entry has a `type`." One product or many, `reference` split out or folded in, guides or none: reshape this file and the folders to fit.
