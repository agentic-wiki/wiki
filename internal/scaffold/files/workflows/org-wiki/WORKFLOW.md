# How this knowledge base works

This is the **workflow** layer for an `org-wiki` bundle: the conventions on top of what [AGENTS.md](/AGENTS.md) and the `wiki` tool define. It is a starting point: **edit it to fit how your organization works.**

This bundle is one organization's internal knowledge base: its **projects, clients, products, people, teams**, and the decisions, meetings, and processes that connect them. The value is not the pages in isolation, it is the **graph between them**: a project links to its client, its product, and the people on it, so `wiki backlinks /clients/acme.md` tells you everything Acme touches.

**This bundle is git-managed.** Commits are its history and its undo, and the workflow assumes it: an agent can capture, file, move, and groom freely, and you can always revert. Pull before editing, and commit in batches once `wiki check` passes.

## First run: pin your conventions

This file ships as a **template with options**. Before filling the base, sit with the user and commit the primitives, then **edit this file to lock the choices** (delete what you won't use), so it becomes the org's actual playbook rather than a menu. Decide together:

- **Which entities are first-class** (each a `type` and a folder). The shipped set is `project, client, product, person, team, decision, meeting, process`, plus `note`/`concept` for everything else and `source` for external references. Keep the ones you will actually file; drop the rest. `entity` is the generic fallback for a named thing with no dedicated type (a vendor, a competitor, a place).
- **How far the people model goes**: `person` alone, or `person` + `team`? Do you track external people (client contacts) as `person` too, or only staff?
- **Whether you track work here at all** (see *Projects and initiatives*): light inline checklists, or a separate `project-backlog` bundle.
- **The tag vocabulary** for cross-cutting themes (`security`, `2026`, `gdpr`, ...).

Then scaffold a **skeleton** the user validates: the chosen folders (each with an `index.md`), the root `index.md` linking to them, and one real entry of each main type in the right shape. Confirm it reads right, then fill in the rest. **When this is done, delete this "First run" section: it has served its purpose.**

## Structure

Group by **what a thing is**, one folder per entity kind, each with an `index.md` that links its members:

```
my-org/
├── index.md          # home: links to every domain
├── projects/         # type: project (active and past efforts)
├── clients/          # type: client
├── products/         # type: product
├── people/           # type: person (contributors; client contacts if you track them)
├── teams/            # type: team
├── decisions/        # type: decision (decision records)
├── meetings/         # type: meeting (notes)
├── processes/        # type: process (how we do X)
├── inbox/            # type: draft (rough captures, not yet filed; ignore_orphans'd)
└── log.md            # optional: a dated org-wide changelog
```

Not every org needs every folder, prune in first run. Keep it shallow (2-3 levels); a big domain can nest (`projects/acme-migration/` with its own sub-entries).

## The graph is the point

Each entity is one entry that **owns its own facts** (a client's contract terms live on the client, a person's role on the person). Everything else **links** to it, and a link is a reference, never a copy: don't restate Acme's details on every project for them, link to `/clients/acme.md`.

Wire the entities together so the graph answers questions:

- A **project** points to its **client**, its **product**, and the **people** on it.
- A **person** points to their **team**; a **team** points to the products or projects it owns.
- A **decision** points to what it affects (a project, a product); a **meeting** points to its attendees and the decisions it produced.

Then the tool walks it:

```sh
wiki backlinks /clients/acme.md          # every project, meeting, decision touching Acme
wiki backlinks /people/dana.md           # what Dana is on
wiki links /projects/acme-migration.md   # what that project points to
```

**How you record a relationship is a recipe, pick one per relationship and keep it** (the same trade-off as epic links elsewhere):

- **Body link** (`[Acme](/clients/acme.md)` in the prose): a real graph edge that `wiki backlinks` follows, and it keeps the target off `wiki orphans`. Best for the org graph, which is the whole point here, so this is the default.
- **Frontmatter field** (`client: /clients/acme.md`): filterable with `wiki list --where client=/clients/acme.md`, but *not* an edge (`backlinks` won't see it, and the target still needs a link from somewhere to stay off `orphans`). Reach for it when you want to filter by the relationship.

## Capture then promote

Rough notes land in `inbox/` as `type: draft`, so `wiki list --where type=draft` is the to-file queue (and `inbox/` is `ignore_orphans`'d, so unfiled drafts don't show as orphans):

1. **Capture** a new `inbox/<slug>.md` (`type: draft`) with whatever you know.
2. **Refine**: read it back, sharpen, fill it in.
3. **Promote**: set the real `type`, `wiki move` it into its domain, and **link it in** from that domain's `index.md` and any related entities. An unlinked entry is lost knowledge (`wiki orphans` will flag it).

Fully-formed knowledge can be created in place; the inbox is only for unrefined captures.

## Projects and initiatives

A **project** (or a broader **initiative**, add the type in first run if you want it distinct) is a knowledge entry: what it is, who is on it, its client/product, its current `status`, and its **mid-term milestones as an inline `- [ ]` checklist** on the entry. Those checkboxes are the project's *own* subtasks (a checklist belongs to the entry it lives in), a quick way to track achievements without a separate tracker:

```markdown
---
type: project
status: active
---

Migrate Acme off the legacy stack. Client: [Acme](/clients/acme.md). Product: [Vault](/products/vault.md).

## Milestones
- [x] Discovery and plan signed off
- [ ] Data model migrated
- [ ] Cutover
```

`wiki tasks /projects/acme-migration.md` then shows that project's checklist. Keep this **light**: when a project needs real task-tracking (many tasks, assignees, sprints, a board), give it its own **`project-backlog` bundle** (a sub-folder or a sibling repo) and link to it from the project entry, that is the tool for the job. org-wiki stays the knowledge layer; don't grow a full backlog in here.

## Decisions and meetings (optional)

- A **decision** (`type: decision`) is a short record: the choice, the context, the alternatives, dated. Link it to what it affects, so `wiki backlinks /decisions/adopt-sqlite.md` shows what rests on it.
- A **meeting** (`type: meeting`) is dated notes linking to its attendees (`/people/...`) and any decisions it produced. Keep the durable outcomes; let the play-by-play fade.

## Grooming

Little and often, aim for a base that gets **more discoverable** over time.

- Run `wiki check` and fix what it flags.
- `wiki orphans`: `inbox/` is `ignore_orphans`'d, so a hit is a *filed* entry nothing links to, wire it into its domain (a client with no project? a person on no team?). This is how an org KB rots: entries that never got linked.
- `wiki unresolved`: links to entities not yet written, a to-write list (the client you referenced but never gave a page).
- Skim the stalest entries (`wiki list --sort=timestamp --reverse`) and add the links between related ones that were never made.
- Consolidate duplicates (two pages for one client) with `wiki move`.

## Answering everyday questions

Most questions are a query plus judgment: `wiki` narrows the set, you read and decide.

- **"What do we have on Acme?"**: `wiki backlinks /clients/acme.md` (everything pointing at them), plus `wiki read /clients/acme.md`.
- **"What is Dana working on?"**: `wiki backlinks /people/dana.md`.
- **"Which projects are active?"**: `wiki list --where type=project --where status=active`.
- **"What decisions affect Vault?"**: `wiki backlinks /products/vault.md`, and read the decisions among them.
- **"What's unwritten?"**: `wiki unresolved`.

Anything beyond a filter (a roll-up, a report) is a small skill over `wiki list --format json`, not a `wiki` feature.

## Make it yours

Nothing here is fixed beyond "every entry has a `type`." Which entities are first-class, how people and teams are modeled, whether you track work inline or in a `project-backlog`: reshape this file and the folders to match how your org actually runs.
