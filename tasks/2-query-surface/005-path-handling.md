---
type: task
title: path handling — -C location, --path filter, positional targets
status: todo
priority: medium
tags: [feature, dx]
---

Three distinct "path" roles, each with exactly one mechanism (no duplicate ways to say the same thing):

- **Bundle location** → global `-C <dir>` (git-style; `--root <dir>` alias), parsed before the subcommand, so any command runs against the bundle at or above `<dir>` without a `cd`. Default: cwd.
- **Within-bundle filter** → `--path <prefix>` flag on commands that narrow results (`list`, `tasks`, future `search`/`property`), consistent with `--type`/`--tag`.
- **Single-file target** → positional `<file>` on commands that act on one entry (`read`, `outline`, `links`, `backlinks`, `move`, `rename`).

Rule of thumb: a path that *narrows* is a `--path` flag; a path that *is the subject* is positional; the bundle root is `-C`. (`wiki init [path]` is a fourth, distinct role: the creation target.)

Implement `-C`/`--root` (in `cmd/wiki/helpers.go`, threaded into `loadIndex`); keep `--path` a flag (do not also make it positional).
