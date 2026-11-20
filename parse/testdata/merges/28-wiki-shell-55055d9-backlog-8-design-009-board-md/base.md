
Mock lines 339–398. Every behaviour of `BoardView` stays: pointer-event drags with the ghost, lane bands as drop targets, column-header drag to pin order, condensed empty lanes until a drag starts, optimistic moves, the empty-board form, print as stacked sections. The mock's HTML5 drag-and-drop is not adopted.

## Header

`14px 20px`, `line` hairline: the name (18px 600 `-0.02em`), `path · N cards` in mono; spacer; a **Filter cards or tags** input (200px, `panel-2`, magnifier); a **Lanes / Flat** segmented control (only on a board with a lane field); a **Settings** button with the sliders glyph.

- **Filter** is client-side, case-insensitive, over the card's label, title and tags; kept in the URL query (`?q=`) so it survives the sheet opening and a reload. Counts and progress bars follow the filter.
- **Flat** hides the lane bands without touching config, per board, in `useBundleState`.

## Columns

316px, `panel-2`, `line` border, 14px radius, max-height the board, 14px gap, padding `18px 20px 24px`. Header: a 9px dot in the column's colour ([003](./003-colour.md)) with a 3px halo, the label (13.5px 600, separators as spaces), the count in mono, and a 44×4px progress bar — the column's share of all cards on the board. The pinned mark stays, as a faint tooltip'd dot after the label. Drag-over: `accent` border.

## Lanes

A 10px-radius band, 4px padding, 6px gap. Header: the three-bar glyph lit by position, the lane name in 10.5px 600 `.09em` caps `faint`, the count in mono. Drop-over: `accent-bg` fill and a 1.5px dashed `accent` outline. An empty band while dragging: a 38px dashed "Drop here".

## Cards

`elev`, `line` border (`line-2` on hover; the reference's −1px lift was dropped on 2026-10-02, as it read as the card jumping), 10px radius, `11px 12px 10px`:

- the title, 500 13.5px, line-height 1.35 — the entry's title when it has one, else the label;
- the label under it in `muted` 12.5px, clamped to two lines, when it differs (the mock's description slot);
- **Waiting on**: `danger` 11.5px, the barred-circle glyph, "Waiting on <first blocker's title>", ellipsized, and "+N" when there are more — only while a blocker is not done;
- a footer: tag chips (20px, `panel-2`, a 6px dot in the tag's colour), capped at 3 with "+N"; spacer; the link glyph and count in mono. The "holding up N" badge stays in the footer.

The dragged card stays in place at 35% opacity; the ghost is the card in `elev` with the shadow and `accent` border. The card open in the peek has a 1.5px `accent` ring.

## Server

`Card` gains `blocker` (the first unfinished blocker's title) and `links` (as on a stub).

**Tests:** the filter over label, title and tags, in the URL, counts following it; Flat persisted and hiding bands; column colours from the gradient with `archived` gray; the waiting-on line only for unfinished blockers, with "+N"; progress bar share. Every existing drag test still passes.

## What building it settled

- **Columns snap to the nearest stop, never a blend**, decided with the user after seeing it. The oklch midpoint of blue and amber came out **pink** (the short way round the hue wheel), and a straight oklab blend is beige. Neither is in the gradient. Three live columns are now gray, amber, green; five are gray, blue, amber, amber, green. Pinned in `colour.test.ts` for 1–12 columns, including "only the four stops ever appear". The first test only pinned the CSS string, which is why the pink got past it; the screenshot caught it.
- **The first blocker is the first listed, not the first unfinished.** The server reports blocker edges without verdicts because nothing in wikiview knows which status means done. "Waiting on …" appears exactly when the old count badge did, names `Card.blocker`, and adds "+N". An unwritten blocker is named from its filename.
- **Title first on the face** (`title`, else the filename label), the filename beneath when it differs. That settles most of [4-boards/008](../4-boards/008-card-titles.md); what it still asks for is a server-side fallback to the body's first heading.
- **The filter lives in `?q=`** and rides into card addresses and back out on close, so opening a card keeps the narrowing. The header says "N of M cards" while filtering, and a column the filter empties says "No cards match".
- **Flat** is per board, per bundle (`board:<id>:flat`), and never touches config. Dropping onto a flat column names no lane, so a card keeps its own, by the existing `moved()` rule.
- **The column colour is a CSS custom property** (`--c`), read by the dot, its halo and the share bar. That is one value, and it is also what happy-dom can parse (it drops `color-mix()` from an inline `background`).
- **Column names are sentence-cased by CSS** (`first-letter:uppercase`), so the text is still the value written back. Lane names stay in capitals. The lane count moved beside its `h3`, so a heading's text is only its name.
- **`Card.links`** is the same degree as on a tree stub, from `connections()`.

## Revised after use (2026-09-30)

- **"Waiting on …" is amber** (`warn`), not red: waiting is a state to notice, not a failure.
- **Not on a shelf.** A card in an `archived` or `parked` column (the words `SHELVED` names) does not say what it waits on. There, it no longer matters.
- **With lanes on, a dragged card lights only the band it would land in**, never the column. Over a column but no band (its header, say), the drop keeps the card's own lane, so that band is the one lit. With lanes off, the column lights, as before.
- **A dragged column has weight like a card.** A `ColumnGhost` (its header and first cards) follows the pointer, the original stays in place at 40%, and the column it would land before carries an accent line on its left edge.
- Checked mid-drag in a real browser, each drag returned to where it began before release, so nothing was written.
