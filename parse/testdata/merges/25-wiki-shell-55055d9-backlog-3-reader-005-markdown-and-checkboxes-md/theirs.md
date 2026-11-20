
The reader itself, and the first thing in WikiShell that writes.

## Rendering

A React markdown pipeline with plugins — GFM, task lists, syntax highlighting, mermaid — rendering into real components rather than injected HTML. That is the whole reason the API serves the body as source: a plugin pipeline needs markdown as input, and an injected HTML blob would put the body outside the component tree, making every interaction manual delegation over a foreign DOM.

**What the API identifies, and why that set.** It ships positions and identities for anything the client would otherwise re-derive with a rule that has to agree with the engine — and especially anything that will be written to. That is links, heading ids, and checkboxes; not tables, yet, because nothing writes to a cell and sorting over what was displayed is self-consistent with whatever parsed it. The moment a cell becomes editable, table boundaries have to come from `parse.Tables`: the engine has known `|` edge-case debt, and a disagreement there would corrupt a row rather than merely look wrong.

Two things the client must **not** work out for itself, both already served as lookup tables:

- **Link hrefs** resolve by looking the raw form up in `links[]`. No path arithmetic, no bundle-root knowledge. An href absent from the table — an external URL, a link out of the bundle — is left exactly as authored.
- **Heading ids** come from `headings[]`. Every markdown library ships its own slugger and they disagree in small ways; generating them here would silently break `#anchor` links `wiki check` considers valid.
- **Checkbox lines** come from `checkboxes[]`. The engine decides what a checkbox *is* — which bullet markers count, what indentation, and that fenced code blocks are skipped — so a client parser that disagrees would write to the wrong line.

A link whose target does not exist still renders as a link, marked. Per the format that is not an error: it may be knowledge not yet written.

Frontmatter renders as a compact strip (type, status, tags, title), not a raw YAML block. Raw YAML is only interesting when editing, which is not this phase.

Below the body: **linked mentions**, from `backlinks[]`, with the source path and line.

## Checkboxes

Clicking `- [ ]` toggles it and writes the entry. This needs the first write endpoint:

```
PUT /api/entry/{path...}/checkbox   { "line": 12, "done": true }
```

Keyed by **line**, because that is the only stable identity a checkbox has — its text may repeat within an entry, and `parse.Checkbox` already carries the line. The engine's `SetCheckbox` does the write: exactly one character changes, atomically, and the entry refreshes in the index.

The interesting part is not the write, it is what happens around it:

- **The line may be stale.** The client holds a version; the file may have changed since. Send the version with the request and reject the write if it moved, rather than toggling whatever now sits on line 12. A rejected write tells the client to refetch.
- **The write moves the version**, so the client's own SSE event will arrive announcing a change it caused. It should reconcile rather than flicker: apply optimistically, and let the refetch confirm.

## Deliberately not

Editing prose. The layout leaves room for it and the API already serves what an editor would need, but nothing edits markdown in this phase.

**Acceptance:** an entry renders with working internal links, anchors that scroll, and highlighted code; a link to an unwritten entry is visibly different; toggling a checkbox writes the file and survives a reload; a toggle against a stale version is refused rather than applied to the wrong line.

## Done

react-markdown with GFM and highlighting, links resolved by lookup, ids from the server, and checkboxes that write through `SetCheckbox` behind a version guard.

Three bugs found in the building, each invisible to a screenshot:

- **Two coordinate systems.** The engine's line numbers count from the top of the *file*, frontmatter included; the body is served stripped. Every heading and checkbox was off by the frontmatter length, so nothing matched. Both coordinates are now sent, because having one and computing the other is the arithmetic that goes wrong silently.
- **Every write would have been refused.** The client sent `version: 0` — the version state was only ever set by SSE and never seeded from the bundle. Writes now carry `bundle.version`, the version the data on screen was actually read at.
- **Matching by walking the document was wrong.** A cursor assumes the renderer visits nodes in source order exactly once; StrictMode's double render exhausted it and silently dropped every id. Matching is by source position, which react-markdown supplies for every node.

The staleness rule works as designed: a replayed version is refused with `409` and the current version, and the file is untouched.
