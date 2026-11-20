
Two questions the reader cannot answer today:

- **What changed under me?** An agent edits this bundle while it is open. The tree marks what moved, but a mark is only an answer where you are already looking: after a `tidy --all` across thirty files, a folder dot says "something in here moved" and you open folders until you find them. The tree turns a digest into a hunt.
- **What did I mean to come back to?** An entry you find while looking for something else, that plainly matters and that you cannot read now. There is nowhere to put it, so you either stop what you are doing or you lose it.

One task, because deciding where each of them lives is one decision. They do not get the same answer.

## The rule that decides where a list lives

**A list whose rows survive being clicked belongs beside your work. A list whose rows are consumed by being clicked belongs in the view area.**

Opening a changed entry marks it seen, so its row leaves the list. In a left panel that is a broken handle: the thing you clicked disappears from under the cursor, the rows below jump up, and the next click lands on something you did not aim at. In the view area nobody sees it happen — you left the page. Coming back renders the list as it now stands, minus what you read, which is the honest answer and the one you expect.

A queued row is not consumed. It stays until you remove it, so the panel is stable, and the panel is also where it wants to be: you are reading A *while knowing* B and C are waiting. Triage is a moment you go to; a queue is a companion you keep.

Same rule, applied elsewhere as a check: [search](./016-search.md) results survive being clicked, so the panel stays the right home for them.

## What changed: a page, shaped like a folder listing

A route, so it has an address, back and forward work, and ⌘K can reach it. Rendered with the same row model as a folder with no `index.md` — icon, the entry's name, its own title underneath when it says something the filename does not, and something on the right where a folder listing puts the type. Ordering is newest first.

`FolderView`'s `Row` is already the right component and takes `to`/`icon`/`title`/`subtitle`/`meta`. Extract it and share it; do **not** synthesize a `TreeNode` for a folder that does not exist, which would put a fabricated path into links and breadcrumbs.

It stores nothing new. `useSeen` already returns `unseen` and `changedAt`, so this is a rendering of state the app holds, and it empties itself as you read. Its empty state — nothing has changed since you were last here — is a real answer rather than an apology.

Ordering is free; wall-clock is not. `changedAt` is a bundle version, not a time, so "newest first" is exact and "four minutes ago" is unavailable without the API carrying something it does not carry today. Say the order, not the time.

**The alternative considered and rejected:** keep it in the panel and stop consuming rows — the mail-client model, where a read row stays and only loses its emphasis. That needs a second notion of seen, session-scoped and unversioned, sitting beside the versioned one the bundle actually has. Two definitions of "you have looked at this" is a bug waiting for a reason to happen. The page needs no new state at all.

## What you queued

*(Planned as a panel. It shipped as a page: see [what building it settled](#what-building-it-settled).)*

A queue, not favourites. A queue is consumed and emptied, so what is in it is what you still owe yourself; favourites only grow, and a shelf that only grows stops being read. Only one of the two is missing here: a wiki already answers "somewhere I go back to often" with the tree, links, breadcrumbs and ⌘K, while "this matters, but not now" is answered by nothing.

Add, open, remove. Opening does not remove — that is the whole difference from the other list — so removal is a control on the row. Ordered by when it went in, the only ordering the queue itself knows.

Personal, so `localStorage`, per bundle, through `useBundleState`, alongside `seen`, which settled this question already: what one person owes themselves is not a property of the bundle, and writing it into the files would put one reader's attention into everybody's repository and into git. The cost, stated plainly: an agent cannot see this list, `wiki list --where` cannot query it, and clearing site data loses it. Acceptable for a personal queue, and none of it would be acceptable for a shared one — a shared read-later is a different feature with a different store, a frontmatter field, queryable from the CLI, and somebody else's queue in your diff.

Stored as JSON under one key, which is what `state.ts` prescribes for a value that is genuinely a list: these are paths, and paths hold whatever filenames hold.

## The rail

Four icons: Entries, Boards, Changed, Queued. Two questions, two icons, because one icon holding both lists would need a switch inside the panel and the switch would be the thing you clicked twice on every visit.

The Search icon comes out at the same time, ahead of the feature. It has been permanent chrome answering a click with an apology, and a gap is better than an apology beside three icons that work. It returns when search does — see [016](./016-search.md).

That leaves the rail with one section of a kind it has not had: a route with no panel behind it. `Shell.pick` currently does two things — navigate when there is somewhere to go, toggle the panel when there is not — and a repeat click on Changed has no panel to toggle. That function has been wrong twice already, both times by tangling navigation with panel state, so the third case gets written deliberately rather than discovered.

## Decisions left

- **One mark vocabulary.** The tree already carries the unseen dot. A queued mark next to it must be a different shape, not a different colour, or the two compete and neither reads at a glance. An entry that is both is the case to design for.
- **Where the queue toggle is.** The entry header is obvious. The card sheet needs the same control or a board is a place where nothing can be queued. The palette too, if it can be done without a modifier nobody discovers.
- **Coming back to a shorter list.** Scroll restoration puts you where you were, and the page you return to is one row shorter than the one you left. Probably fine, possibly a jump worth pinning to the top instead.
- **An entry that stops existing.** Paths go stale, the same problem search has. A queued item silently dropped looks like the queue lost something; shown as missing, unopenable and removable is honest and costs one branch. The changed list does not have this problem, since it only ever lists what the tree currently holds.

## The blind spot both lists inherit

A deleted entry is invisible. `seen` collects paths from the tree, so an entry that is gone is absent rather than reported, and "deleted while you were not looking" is the one change neither list can show. Already true of the marks shipped today. Worth knowing before either list is described to anybody as "what changed", and worth its own task if it matters.

## The argument against the queue

The browser already has it. A bookmark is read-later, it survives a cache clear, it works across every tool, and now that the document title names the entry and its bundle, a bookmark of one finally has a usable name.

What a bookmark cannot do is sit next to the entry in the tree, follow a bundle that moves, or know that what you queued has changed since you queued it — and that last is free here, since `changedAt` is on every entry. If the built queue does not do those three things, the bookmark is better and it should not exist. None of this applies to the changed list, which has no equivalent anywhere.

**Acceptance:** two routes, each listing entries in the row model a folder listing uses, each with a heading and a line saying what the list is and an empty state that is an answer. Recently changed is newest first and loses a row when you open it, without reflowing a list under the cursor. Read later keeps its rows until you take them off, from the row; it survives a reload, does not leak into another bundle, can be added to from the reader and from a card, and says so rather than dropping an entry that has gone. Rows name what the entry calls itself and the folder it lives in, readably. A saved entry and a changed one are distinguishable in the tree at a glance, including on the same row. The rail carries four working sections and no dead one.

## What building it settled

**Both lists are pages, and the rule above is half wrong.** Rows that survive being clicked make a panel *possible*; they do not make one necessary. The read-later list shipped as a panel first and read as thin: 256 pixels truncates a title at about four words, and the rail icon behaved unlike every other one because clicking it changed nothing in the view area. The deciding question turned out to be how often you consult a list, not what happens to its rows. The tree is a panel because steering a hierarchy is constant while reading; both lists are consulted when you finish something and pick the next thing, which is a place you go.

Two homes were tried and dropped. With a page *and* a panel, the rail icon can only do one of them, and the second needs its own affordance — so removal ends up in one home and not the other, and the same list has two shapes to keep in step. One home, one row shape, one place that can take something off the list.

**Names say what a section is for.** "Recently changed" and "Read later" rather than "Changed" and "Queued": the first pair says why you would click, the second describes the condition of a list and leaves you to infer the rest. The store is still a queue in the code, because that is what the structure is.

**A list of things to read names them the way a page does.** The tree, the breadcrumb and a folder listing name *files* — you are moving around a folder and a row that renames itself is a row you cannot navigate back through. Both lists name what the entry calls itself, with the folder underneath, readable, as the tree spells it. That second rule already existed for the browser tab; it now lives in `tree.ts` as `nameOf` and is used by all three.

**And an `index.md` is named by its folder.** "Index" names every folder's front door, so in a list of rows it names none of them — which is what shipped first and was wrong. The tree can call it Index because the folder is drawn around it on screen; a list has no folder around it, so the folder becomes the name: `Notes (index)`, and `Backlog (index)` for the bundle's own front door. The folder is then not repeated underneath, since the name already says it.

Not a new rule. The server names a backlink exactly this way, in these words, and its comment gives this reason (`backlinkName`, `internal/server/entry.go`) — a backlinks footer has no folder around it either. What is new is only that the client now needs the same rule, and has everything to compute it: the tree carries folder labels.

What is *not* done, and is a live option rather than a decision: an entry with no frontmatter title at all still falls back to its filename made readable. A first heading or a truncated opening line would be better for entries whose filenames are dates or ids. The cheap way is the server adding the first heading to the tree's entry stubs, since the index has already parsed every heading — that is wikiview's own server, not the engine, and it is a few lines. The expensive way is the client fetching bodies for rows it is listing. Nobody should be reverse-engineering titles from `body` in the browser.

**The panel a rail icon opens was scoped to one URL, which is a bug the panel version found and the page version does not depend on.** `Shell` kept the picked section against `location.pathname`, so the first entry opened from a panel list took that list away and left the tree in its place — the same disappearing-handle problem as the changed list, one level up. It is fixed rather than sidestepped, because the case survives the redesign: Boards in a bundle that declares none is still picked with nowhere to navigate, and search will be too. The section is kept per *family* of routes, and a pick that navigates hands it back to the route, so clicking Entries means the tree rather than whatever was last beside the reader.

The route prefixes each section owns are one table now, read by both `sectionFor` and the rail's click handler. They were two chains of `startsWith` in two functions, which is how a section ends up navigable from the rail and unrecognised by the route.

**Nothing was stored for "changed since you queued it".** Queueing happens on the entry or on a card, which are the two places you can only be while looking at the thing — so the moment you queue is the moment you last saw it, and `unseen` already answers the question exactly. One definition of "you have looked at this", not two. What would break that is queueing from somewhere you cannot see the entry: the palette, or a row on the changed page. Either of those arrives with a stamp on each queued path, or it silently starts meaning something else.

**The card sheet needed no work.** A card is `EntryView` inside a dialog, so the bookmark button, its state and its tooltip arrived there for free. That is the second time that shape has paid off, after the checkbox.

**The changed page shares `Row` with folder listings**, extracted to `views/listing.tsx` with the two icons and the counter. No synthesized `TreeNode`: a folder that does not exist would have put a fabricated path into every link on the page.

**Both marks live in one group on a tree row**, bookmark then dot, so an entry that is queued *and* changed reads as two facts rather than a crowded row. The bookmark is muted on purpose: the accent belongs to the dot, which is the mark that is news.

Left undone, deliberately: queueing from the palette, ordering the queue by anything other than when things went in, and any answer to a deleted entry beyond the queue's own "no longer in this bundle" row. The [blind spot](#the-blind-spot-both-lists-inherit) is unchanged — a deleted entry is still invisible to the changed list, because `seen` collects paths from the tree and a path that is gone is absent rather than reported.
