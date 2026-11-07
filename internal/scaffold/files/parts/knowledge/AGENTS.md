What we know, filed by topic.

- **Topic folders** start as `{dir}people/`, `{dir}processes/` and `{dir}decisions/`, each linked from `{dir}index.md`. Rename or delete them to fit. Start a new one when a topic has three entries, and `wiki move` them into it. Until then, file next to `{dir}index.md` and link from it. To rename one, `wiki move` each file in it (`wiki move {dir}people/index.md {dir}team/index.md`), then change its heading.
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
