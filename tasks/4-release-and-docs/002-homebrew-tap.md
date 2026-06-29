---
type: task
title: Homebrew tap (cross-platform formula)
status: in-progress
priority: medium
tags: [release, docs]
---

`brew install agentic-wiki/tap/wiki` on macOS **and** Linux from a single formula.

A Homebrew *formula* (not a cask) is the only artifact that serves both OSes, so
`scripts/update-formula.sh` renders one and pushes it to the tap after goreleaser
in CI. We render it ourselves rather than via GoReleaser's `brews:`, which is
deprecated and removed in GoReleaser v2.16; the formula depends only on the
stable Homebrew DSL (`on_macos`/`on_linux` + `on_arm`/`on_intel`).

One-time manual setup (cannot be scripted from this repo):

- Create `agentic-wiki/homebrew-tap` (public, empty). The `homebrew-` prefix is
  required; it is dropped in the install command.
- Add a `HOMEBREW_TAP_TOKEN` secret to this repo: a PAT with `contents:write` on
  the tap repo. The default `GITHUB_TOKEN` cannot push cross-repo.
- Pick a LICENSE (none in the repo today) and add `license "<spdx-id>"` to the
  formula in `scripts/update-formula.sh`.

Verify after the next `vX.Y.Z` tag: `brew install agentic-wiki/tap/wiki` on both
macOS and Linux. Preview the rendered formula locally with `just formula-preview`.
