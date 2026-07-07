---
type: task
title: hand-rolled YAML frontmatter subset
status: done
priority: low
tags: [debt, parser]
---

`internal/parse.Frontmatter` parses only our scalar + string-list subset; `wiki.toml`'s `types` array must be one line. Chosen for zero dependencies and a network-free, single-static-binary build. Swap to `gopkg.in/yaml.v3` only if frontmatter grows nested structures — the parsers are isolated behind `parse.Frontmatter` and `bundle.parseConfig`.
