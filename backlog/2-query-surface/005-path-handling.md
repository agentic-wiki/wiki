---
type: task
title: path handling — --root location, --prefix filter, positional subject
status: done
priority: medium
tags: [feature, dx]
---

Two path namespaces, each role with exactly one mechanism (no duplicate ways to say the same thing):

- **OS filesystem paths** — where bundles live on disk: global `--root <dir>` (which existing bundle to operate on; redirects discovery, **no chdir**) and `init [dir]` (where to create one).
- **Bundle paths** (`/…` root-absolute, or a unique basename — the same form as links in content):
  - **Subject** → positional `<file>` on commands acting on one entry (`read`, `outline`, `links`, `backlinks`, `move`).
  - **Filter** → `--prefix <p>` on commands that narrow a listing (`list`, `tasks`, `search`), alongside `--type`/`--tag`.

Rule of thumb: on disk → `--root` / `init [dir]`; within a bundle, a path that *is the subject* is positional, a path that *narrows* is `--prefix`. Because `--root` only redirects discovery (no chdir), `init` is unaffected — so there is no `--root` vs `init [dir]` overlap.

Done: `--root` sets the discovery start dir in `loadIndex` (no chdir); `--path` renamed to `--prefix`; positionals are bundle-path subjects. The path-vs-basename resolution rules are documented in the README.

Rejected: a git-style `-C` that `chdir`s — it entangles `init`'s relative `[path]` and models "cwd" rather than "the bundle," which was convoluted.
