---
type: note
title: Welcome
tags: [getting-started]
---

# Welcome to Wiki CLI

This is your knowledge base: a folder of markdown files that you, and an AI agent, can read, search, and keep tidy.

A few light conventions make it work:

- **Every file says what it is** via a `type` in the block at the top (`note`, `concept`, `dataset`, `task`, ...). The set you use is listed in `wiki.toml`.
- **Folders group by topic**, one home per thing. Link between files with paths from the root, like [Home](/index.md).
- **Tags** (also in the top block) cut across folders for anything that recurs.

The `wiki` tool operates it:

- `wiki list --type dataset` — find files by type, tag, or folder
- `wiki tasks` — list checkbox tasks
- `wiki search budget` — search the text
- `wiki check` — make sure that everything links up

Checkboxes are quick to-dos, and `wiki tasks` gathers the open ones:

- [x] run `wiki status`
- [ ] remove this page when ready
