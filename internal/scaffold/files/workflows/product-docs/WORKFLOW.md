# How these docs work

This is the **workflow** layer for a `product-docs` bundle: the conventions on top of what [AGENTS.md](/AGENTS.md) and the `wiki` tool define. It is a starting point: **edit it to fit your product(s).**

This is **wiki-first product documentation**, in two layers:

- **Concepts, the graph.** Atomic `concept` and `reference` entries, one idea per page, richly linked to each other. This is the substance, consumed *non-linearly*: you land on a page and follow links (`wiki links` / `backlinks`) to related ones.
- **Guides, the linear layer.** `guide` entries that read top-to-bottom (a tutorial, a how-to) and **link into** the concepts for depth, never restating them. A thin, optional layer over the graph.

The concepts are the source of truth; guides are paths through them.

**This bundle is git-managed.** Commits are its history and its undo; pull before editing, commit in batches once `wiki check` passes.

## First run: pin your conventions

Ships as a **template with options**. Before writing docs, commit the primitives with the user and **edit this file to lock them**:

- **Product boundaries** (the key decision): one product or several, and how they are separated. Default: **one folder per product** (`checkout/`, `billing/`), each holding that product's concepts/reference plus its own `guides/`. A single product can start flat (concepts at the root plus a `guides/`) and grow folders when a second product appears. If products share most concepts, the alternative is one shared graph tagged by a `product:` field, decide up front, because reshaping later is costly.
- **The type split**: `concept` (an idea/term), `reference` (precise lookup: API, config, schema, CLI), `guide` (linear how-to). Keep all three, or fold `reference` into `concept` if the distinction won't earn its keep.
- **Taxonomy**: slugs for concept files (`rate-limiting.md`), and whether each product keeps an `index.md` as its concept map.

Then scaffold a **skeleton** the user validates: one product folder, three or four linked concepts, one guide that links into them, and the product `index.md`. Confirm it reads right, then fill in. **Delete this section when done.**

## Structure

One folder per product; concepts and reference at its root, guides in a `guides/` subfolder:

```
my-docs/
├── index.md              # links to each product
├── checkout/
│   ├── index.md          # the concept map for Checkout
│   ├── idempotency.md    (type: concept)
│   ├── webhooks.md       (type: concept)
│   ├── api-tokens.md     (type: reference)
│   └── guides/
│       ├── index.md      # ordered list of guides
│       └── first-payment.md   (type: guide)
├── billing/
│   └── ...
└── inbox/                # type: draft (rough captures; ignore_orphans'd)
```

Keep it shallow. A large product can group concepts into sub-areas, but resist deep nesting: the graph (links), not the folder tree, is how readers navigate.

## Concepts: the graph

Each concept is **one atomic idea**, defined **once**, and linked to what it relates to:

- **Define once, link everywhere.** A term is explained on its own page; every other page that mentions it **links** to that page instead of re-explaining it (a link is a reference, not a copy). This one rule is what keeps docs consistent: the definition changes in a single place.
- **Link generously.** The value is the graph. `wiki backlinks /checkout/idempotency.md` shows everything that depends on the concept; `wiki links` shows what it builds on.
- **concept vs reference**: a `concept` explains an idea ("what idempotency is, and why"); a `reference` is precise lookup material ("the `/charges` endpoint and its fields"). Same graph, different texture. Split them when readers want "understand" apart from "look up"; merge them if that is overkill.

`wiki unresolved` is your **to-write list**: a link to a concept you haven't written yet is not an error, it is a promise. Writing docs is largely turning `unresolved` into pages.

## Guides: the linear layer

A **guide** (`type: guide`) is one linear document, a tutorial or how-to, that walks a reader through a task and **links into concepts** for the details:

- A guide **references** concepts, it does not duplicate them: "set up a webhook (see [Webhooks](/checkout/webhooks.md))", not a re-explanation of webhooks. If you are tempted to explain a concept inside a guide, that concept wants its own page.
- **Order** guides in `guides/index.md` (getting-started to advanced). A guide is read start to finish, so sequence matters here in a way it never does for concepts.
- Guides are entry points, so nothing may link *to* them, and that is fine. Link them *from* `guides/index.md` (and the product `index.md`) so they are reachable and not flagged as orphans.

## Multiple products

Each product is a folder with its own concepts plus `guides/`, and its own `index.md` as the concept map. A concept **shared** by two products has a single home (the product that owns it, or a top-level `shared/`), and the other product **links** to it, never copies it. If you find most concepts are shared, that is the signal to switch to one graph with a `product:` field instead of folders (a first-run choice).

## Grooming

- `wiki check`, then `wiki unresolved` (the to-write list) and `wiki orphans` (a concept nothing links to is **undiscoverable**, link it in from a related concept or the product index).
- Reread the stalest pages (`wiki list --sort=timestamp --reverse`) as the product changes.
- Watch for a concept explained in two places: merge to one and link.

## Answering everyday questions

- **"What depends on this concept?"**: `wiki backlinks /checkout/idempotency.md`.
- **"All guides for Checkout"**: `wiki list --where type=guide --prefix checkout/`.
- **"Everything about Checkout"**: `wiki list --prefix checkout/`, or `wiki read /checkout/index.md` for the curated map.
- **"What's not written yet?"**: `wiki unresolved`.
- **"Reference pages only"**: `wiki list --where type=reference`.

## Make it yours

Nothing here is fixed beyond "every entry has a `type`." One product or many, `reference` split out or folded in, guides or none: reshape this file and the folders to fit.
