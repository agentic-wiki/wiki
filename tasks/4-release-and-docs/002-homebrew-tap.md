---
type: task
title: Homebrew tap (cross-platform formula)
status: done
priority: medium
tags: [release, docs]
---

`brew install agentic-wiki/tap/wiki` on macOS **and** Linux from a single formula.
Shipped in v0.3.0.

A Homebrew *formula* (not a cask) is the only artifact that serves both OSes, so
`scripts/update-formula.sh` renders one and pushes it to the `agentic-wiki/homebrew-tap`
tap after goreleaser in CI (a step in `.github/workflows/release.yml`, authed by the
`HOMEBREW_TAP_TOKEN` secret). We render it ourselves rather than via GoReleaser's
`brews:`, which is deprecated and removed in GoReleaser v2.16; the formula depends only
on the stable Homebrew DSL (`on_macos`/`on_linux` + `on_arm`/`on_intel`). Preview the
rendered formula locally with `just formula-preview`.

The tap repo holds only the generated formula, no LICENSE: it is near-uncopyrightable,
auto-regenerated boilerplate, and the generator lives here under GPL-3.0.

Follow-up, not started: homebrew-core for the bare `brew install wiki`, gated on
notability and an open-source license.
