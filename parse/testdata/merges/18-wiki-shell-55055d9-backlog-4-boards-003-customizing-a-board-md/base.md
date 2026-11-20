
Editing a board's settings from the board itself, rather than by hand-editing `wiki.toml`.

Everything a board reads is already configurable — `where`, `status`, `columns`, `lane` — and all of it is optional. What is missing is a way to change it without leaving the UI, and a way to see what is currently inferred versus what is pinned.

## What is editable

- **Column order**, by dragging column headers on the board. Inference gives the columns that exist; only config gives them an order, so dragging one pins them all.
- **Empty columns**, added explicitly. A board with nothing `in-progress` still wants that column, and no amount of inference can produce it.
- **The status field**, when a bundle spells it something other than `status`.
- **The lane field**, or none.
- **The filter**, in `--where` syntax — the same expression the CLI takes, parsed by `index.ParseFilter` rather than by anything written here.

## The distinction the UI has to show

A board is inference plus config, and which is which matters. A column that appeared because an entry has that status behaves differently from one someone pinned: renaming a status in the entries makes the first vanish and leaves the second empty. Showing them identically makes the config feel haunted.

So: inferred columns render as present-but-unpinned, with pinning as an explicit act.

**Config orders and adds; it never filters.** A status present in the entries but absent from `columns` still gets a column. Hiding cards through the column list would make config a filter by accident, and filtering is `where`'s job where it is visible.

## Writing it

Shares the narrow TOML appender from [choosing boards](./002-choosing-boards.md) — the one place a second config implementation is unavoidable, since the engine reads `[tool.*]` and deliberately never writes it. Updating an existing `[[tool.wikiview.board]]` table is harder than appending one, and the same rule applies: change the keys in question and leave every other byte of `wiki.toml` alone, the way the engine's own frontmatter writer does.

That constraint is worth taking seriously here. `wiki.toml` may hold comments, other tools' tables, and formatting the user chose. A parse-and-reserialize would quietly discard all of it.

## Where a setting lives

Settled, since the first one is now written: **`wiki.toml` holds what a board is, and [per-bundle UI state](../3-reader/002-per-bundle-ui-state.md) holds what one person did to it.** The test is whether the setting would make sense to somebody else opening the same folder. A lane field would; a collapsed column would not. Nothing here writes the second kind, so nothing has to be migrated later.

**Acceptance:** a board's columns can be reordered and pinned, its lane and filter set, from the board; changes land in `wiki.toml` without disturbing comments, other tables, or formatting; inferred and pinned columns are visibly different; an invalid filter is reported rather than silently dropped.

## Done

`PUT /api/board/{id}` takes the whole settings object — name, `where`, `status`, `columns`, `lane` — rather than one key at a time. Clearing a setting is how you say a board has no lanes, so a partial update would have to decide whether an absent key means "unchanged" or "cleared", and getting that wrong either way loses something. Not `id` or `path`: those are what the board *is*, and changing an id breaks every link to it.

`config.Update` edits the board's own table in place, line by line. Parsing is the thing that loses the file, so it never parses: it finds the `[[tool.wikiview.board]]` whose `id` matches, replaces the lines for the keys it owns, appends the ones that were missing, and leaves every other byte alone — comments, other tools' tables, and the alignment the table already had, which appended keys are padded to match. A key cleared is a key removed, because `lane = ""` is a lane called "" and `columns = []` says what saying nothing says.

**It refuses rather than guessing.** A value that does not finish on its line — a `columns` array written across four of them — cannot be edited by a line-based writer without moving a bracket into the wrong table, so it says so and names the file to edit by hand. A filter is checked with `index.ParseFilter` before anything is written, so a typo is reported once here rather than on every startup afterwards.

**`root` is configurable, which means declaring it.** It exists without any config, and refusing to configure the board somebody is looking at would be a dead end, so settings for `root` write the table it never had.

**Pinned is on the wire.** `Column.pinned` says which columns the config declares, and the board marks them. Renaming a status in the entries makes an inferred column vanish and leaves a pinned one empty, and showing them identically is what makes config feel haunted.

**Dragging a column header pins every column**, because order is a thing only config has — inference gives you the columns that exist and nothing more. So a reorder writes the whole list, and the header says as much on hover. The unnamed column is not draggable: it is not a status anybody declared, so there is no place for it in a list of declared ones. It reuses the same `useDrag` the cards do, resolving to the same `data-drop` targets.

## Choosing from what is there

`GET /api/board/{id}` reports `fields`: the frontmatter keys the board's folder uses, each with its values when there are few enough to be a choice. A key with more than a couple of dozen is free text — a title has as many values as there are entries, and a list of them is not something to pick from.

**Taken before the board's own filter**, which is the filter you would be replacing: a list narrowed by it can only ever offer what you already have, so `type=note` has to still be offerable on a board of tasks.

**A list-valued key contributes its items**, because that is what `tags=bug` matches. An inventory that said otherwise would offer values nothing can be filtered by.

**A list filters and does not group.** A column or a lane is one value; a list has many, and the engine's scalar read of one returns nothing — so `lane = "tags"` renders as every card in a single nameless lane, which reads as a broken board and says nothing about why. So `Field.list` is on the wire, the two field pickers leave those keys out, `PUT` refuses to write one, and a hand-written config that names one is reported at startup like any other config mistake. It stays in the filter, where membership is exactly what it means.

So the status and lane fields are lists of what the folder has rather than boxes to type a key into: the mistake worth designing out is spelling it `state` when this bundle says `status`, which is a board with one column and nothing saying why. The current value stays in the list even when nothing has it, since a field can be configured before the entries catch up.

**The filter is rows, not syntax.** `key=value` is a small language but not one anybody should have to be told, and a mistyped condition empties the board rather than complaining. A row is a key, `is`/`is not`, and a value, with a button to remove it.

The key is a list to pick from and **the value is typed**, with what the key already holds as suggestions. They are not symmetrical: a key that nothing has is a typo, and a *value* that nothing has yet is ordinary — `status=in-review` on the day you invent that status. A board you cannot describe until something already matches it is a board you cannot set up. Empty is a real value there, since `status=` matches an entry that has no status, which is the one thing an empty box does not say for itself and so is the placeholder.

A `where` a hand-written `wiki.toml` holds that is *not* a filter is shown as it was written and sent back as it was written, so the server names it. Reshaping it into something that parses would change what the board means without saying so.

## The order values read in

Added after lanes started showing as `mid, low, high` — first-seen order, which is no order at all.

**One rule for both axes**, because it is one question asked twice: what the config declares, then what the field is known to read like, then whatever is left, alphabetically. `columns` had the first and third parts already; `lanes` is its counterpart, and the middle part is new to both.

**A table of vocabularies wikiview recognises**, keyed by field name: `status`, `priority`, `severity`, `size`. Alphabetical is the obvious fallback and the wrong one for exactly these — `high, low, medium` is not a priority ordering — and nobody should have to configure their way out of the default. Keyed by *name* because the opinion only holds where the name does: a board grouped by `area` has no natural order and gets none invented for it.

It is only a default. A value the table has never heard of still appears rather than being dropped, and declaring `columns` or `lanes` replaces it outright.

**On the server, though the complaint was visual.** The order has two sources the browser does not have: `wiki.toml`, which only the server reads, and the vocabulary table. Ordering lanes in the client would mean shipping the declared list over anyway and then writing the merge a second time in TypeScript, with the columns' copy already in Go. What is rendering stays rendering: the client still groups the cards and still decides which bands to show.

## Ordering an axis from the UI

`lanes` existed as a config key before anything could set it, and column order was only reachable by dragging headers on the board. Both axes are edited in the settings dialog now, in a tabbed section.

**Tabs rather than two stacked sections.** They are the same shape of thing — an ordered list of values for a field — so one component serves both, and they are alternatives to look at rather than two things to fill in. Side by side would halve the width each has for a value like `in-progress`.

**The list *is* the config value.** Checkboxes said which values were pinned and could not say in what order; a list says both, because being in it is what pinning means and where in it is the order. Values the entries have but nothing pinned sit below it as one click each, and a value nothing has yet can be typed — which is the whole reason to pin one.

**Buttons, not dragging.** The board already drags and this could too, but this is where a board gets configured, and configuration reachable only with a pointer is configuration some people cannot do. A five-item list is not the place to spend that.

**The lanes tab explains itself when there is no lane field** rather than disappearing. A tab that is not there reads as a feature that does not exist, and the fix is one control up in the same dialog.
