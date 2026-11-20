
A callout is a blockquote whose first line is a type marker:

```md
> [!success] Shipped
> The write went through and the version moved.
```

The reader renders that as an ordinary blockquote with `[!success]` sitting at the top as literal text. Nothing is lost, but the one thing the author was reaching for — *this passage is a warning, that one is a result* — is the part that does not survive. Callouts are how agents and people mark the asides in these bundles, and WikiShell's own README already writes one.

Nothing about this involves the engine. Links, headings and checkboxes come from the server because a client that re-derived them would disagree with what gets written; a callout is presentation over text the parser already hands us, so it is a client change and only a client change: no API field, no index, no server.

## Which markers count

Two dialects are in circulation and both will appear in bundles:

- **GitHub**: `NOTE`, `TIP`, `IMPORTANT`, `WARNING`, `CAUTION`, uppercase, no title allowed.
- **Obsidian**: a much longer list — `info`, `success`, `question`, `failure`, `danger`, `bug`, `example`, `quote`, `abstract`, `todo` and more, case-insensitive, with an optional title on the marker line and `+`/`-` suffixes for foldable blocks.

`success` is in the second list only, which is exactly why the set should not be a list. Match **any** word case-insensitively, style the handful that earn a distinct colour, and give everything else one neutral callout treatment titled with its own word. An unknown marker then reads as an aside with a label, which is what the author meant, instead of as a quote with syntax leaking out of it. Enumerating a set means someone's `> [!tldr]` renders as garbage until this file is edited, and that is the chronic work worth not creating.

The optional title replaces the type word as the heading; the `+`/`-` fold suffix is recognised so it does not end up printed inside the title, but folding itself is out of scope — a collapsed block hides content from ⌘F and from print, and neither is worth trading for an animation.

## Shape

A ~25-line remark plugin, no dependency. On a blockquote whose first paragraph starts with the marker, it strips the marker text and records the type and title on the node; the `blockquote` component in `Markdown.tsx` reads them and renders the icon, the label and the body. There are published plugins for this, but they cover one dialect each and this is smaller than the argument about which one.

Doing it in the component alone is the other option and is worse: the marker would have to be dug out of already-rendered React children and put back without it, which is the fragile half of `stripRenderedCheckbox` applied to a whole paragraph.

## Watch for

- **Positions must survive.** `li` and the headings match server data by `node.position.start.line`. Removing a text node does not move anything else, but a plugin that rebuilds nodes instead of mutating them would drop positions and silently unhook every checkbox and anchor inside a callout. A checkbox inside a callout is the test that catches it.
- **An ordinary blockquote must stay an ordinary blockquote.** A quote whose first line merely begins with a bracket is not a callout.
- **Print.** Callouts need `break-inside: avoid` and colours that survive ink-on-white; a tinted panel that prints as a grey slab is worse than no panel. Per [010](./010-print-view.md), the print rules must not restyle anything, and the test that reads the stylesheet will say so.
- **Theme tokens.** Distinct colours per severity is the one place this could sprawl. A handful of tokens derived alongside the existing `--color-accent`/`--color-muted`, working in both themes, not a palette per type.
- **One renderer, three surfaces.** `Markdown.tsx` is also what the board's card sheet and the print page use, so this lands everywhere at once — and has to look right in a card's narrower column, not just in the reader.
- **Nesting.** A callout inside a callout is legal in both dialects. It only needs to not break; it does not need to look designed.

**Acceptance:** `> [!success] Shipped` renders as a labelled callout with its title, `> [!WARNING]` renders as one too, `> [!tldr]` renders as a neutral callout labelled *tldr* rather than as leaked syntax, a plain blockquote is untouched, a checkbox inside a callout still toggles the right line, and the whole thing prints legibly from a dark-theme browser. Covered by a `Markdown.test.tsx` that pins the marker matching, the untouched-blockquote case and the position survival.

## What building it settled

`remarkCallout` in `ui/src/markdown/callout.ts`, ~30 lines and no dependency: it mutates the first text node of a quote that opens with a marker and leaves `data-callout` and `data-callout-label` behind. `Markdown.tsx` renders the label; `index.css` colours by type.

> [!success] Which is this block
> The three lines at the top of this entry are fenced, so they stay source. These are not: read in WikiShell, this is the feature looking at itself, and read anywhere else it is an ordinary quote that says `[!success] Which is this block` first. Both are fine, which is the point of building it on a blockquote.

**No icons.** The task said icon, label and body, and the icon is the part that does not survive the open set: an icon per type is a table that has to grow every time somebody invents a marker, which is the thing this deliberately does not have. Colour and a label carry it, and an unlisted word gets the same treatment as a listed one.

**The label is an element, not a `::before`.** `content: attr(data-callout-label)` would have been the shorter version and would make "Warning" a thing a screen reader never hears, which is the half of a callout carrying the meaning.

**A third state colour.** The palette had amber and red and no green, so `--color-ok` was added beside them, at the hues a string already uses in a code block.

**Print drops the tint.** A background prints as a grey slab where background graphics are on and as nothing where they are off, so neither is worth having: on paper the left rule and the label do it. Ink-black like everything else, `--color-ok` included.

**A bug found on the way, older than this.** `.markdown > * + *` is direct-child only, so *nothing* nested inside a blockquote had any separation: a two-paragraph quote rendered as a wall. One rule for all quotes, which the callout's label needed anyway.

Positions survived because nodes are mutated and never rebuilt, and that is the part worth a test: a checkbox inside a callout still toggles the line the server gave, and a heading after one still takes its server id. Both under StrictMode, since a double render is what exposed the last position bug.

An inline-formatted title — `> [!note] **Shipped**` — stays in the body and the type labels the callout. Reading it would mean rendering inline nodes into an attribute.

> [!warning] What no test here holds
> How it *looks*. There is no renderer in these tests, so the DOM and the stylesheet's text are what is checkable — the same honest limit the print rules already live with.
