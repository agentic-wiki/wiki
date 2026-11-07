What we know, filed by topic.

- **Topic folders** (`{dir}billing/`, `{dir}people/`) each have an `index.md`, linked from `{dir}index.md`. Start one when a topic has three entries, and `wiki move` them into it. Until then, file next to `{dir}index.md` and link from it.
- **`{dir}inbox/`** is where unsorted and unfinished entries wait, until they're promoted. Nothing has to link to them there.
- **`type: note`** is what we know. **`type: source`** summarizes one outside document, with its `url:`. A note links the sources it rests on.

```markdown
---
type: note
tags: [billing]
---

# Late fees

What it is, what we know about it, and why. Links to what it relates to.
```

```sh
# waiting to be promoted
wiki list --prefix {dir}inbox
wiki list --where type=source --where tags=billing  # the sources on a topic
```
