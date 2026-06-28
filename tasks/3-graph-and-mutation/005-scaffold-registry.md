---
type: task
title: scaffold registry — --template / --from
status: todo
priority: low
tags: [feature, scaffold]
---

Generalise `init` (003) from a single embedded template into a small registry. Mid-term, not v1.

- `wiki init --template <name> [path]` — choose among built-in templates (e.g. `personal`, `corporate`); default stays a personal KB.
- `wiki init --from <path|git-url>` — scaffold from any local dir or git repo. The only path that touches the network; built-ins stay embedded and offline.

Built-ins are embedded via `go:embed`, sourced from the template and skills repos as **go-module dependencies (no submodule)**. Depends on the template repo being registry-shaped (global roadmap `003`). Builds on [init scaffold](/3-graph-and-mutation/003-init-scaffold.md).
