---
type: task
title: "reserve a [tool.*] namespace in wiki.toml for other tools"
status: done
priority: medium
tags: [conformance, config, ecosystem]
---

`wiki check` warns on any `wiki.toml` key it does not recognize, which was the right call: a typo or a renamed field used to fail silently ([conformance/007](./007-unknown-config-keys.md)). But it also means **no other tool can put its configuration where a bundle's configuration lives.**

So `wikanban` carries a separate `wikanban.toml` beside the bundle, purely to avoid making every user's `wiki check` noisy. A second tool would need a third file. The bundle ends up surrounded by satellite configs describing the same directory.

**Proposal:** reserve `[tool.<name>]` tables. `wiki` ignores them entirely: never parsed, never validated, never warned about. Everything outside that namespace keeps warning exactly as it does now.

```toml
spec = "0.1"
types = ["task", "note"]

[tool.wikanban]
group_by = "status"
lane_by = "assignee"
```

This is the `pyproject.toml` `[tool.*]` convention, and it exists for the same reason: one file describes the project, and tools namespace their own opinions inside it rather than each adding a dotfile.

**Why it fits the separation principle rather than violating it.** The concern would be format-layer creep, the spec's own warning that "the format stands alone, the tool is a neutral engine over it, and the skill is where opinion lives". But a reserved, ignored namespace adds no opinion to the format: `wiki` gains no field it interprets, no behaviour, and no validation. It grants *space*, which is the opposite of taking a position on what belongs there.

**To decide:**

- **`[tool.x]` or a flat `tool.x.y` prefix?** The table form reads better and matches the precedent.
- **Does `check` verify anything at all inside it?** Recommendation: no. The moment it validates one tool's keys it has an opinion about that tool.
- **Does `bundle.Bundle` expose the raw tables** to a Go consumer (see [public packages](../4-release-and-docs/005-public-packages.md)), or does each tool re-read the file? Exposing them is the point; otherwise every tool grows a second `wiki.toml` parser, which is the drift this repo keeps refusing.
- **Migration:** none needed. Nothing uses the namespace yet, and separate files keep working for anyone who prefers them.

**Acceptance:** `[tool.anything]` in `wiki.toml` produces no `check` warning; keys outside it still do; the tables are readable by a Go consumer without parsing the file again.

## Done

All three decisions above went the recommended way: `[tool.x]` tables, no validation inside them, and `bundle.Bundle.Tool` exposing them (as `toml.Primitive`, with `DecodeTool` unmarshalling one into a caller's own struct — the deferred decode is what lets `wiki` carry the tables without ever interpreting them).

**It required parsing `wiki.toml` as TOML.** The config was read by a hand-rolled line scanner: split on newlines, cut on `=`, switch on the key name. That cannot implement this task at all, because it has no notion of a table — a header line has no `=` and was skipped, so keys inside one were indistinguishable from top-level keys. Reaching for the namespace surfaced two bugs that were already live:

- **A key in any table silently overrode bundle config.** `[tool.x] types = [...]` replaced the bundle's vocabulary, and entries with undeclared types then passed `check` clean. The namespace this task proposes would have made that reachable by design rather than by accident.
- **A multi-line array silently disabled its setting.** `types = [` parsed to an empty list, read as "no vocabulary declared", allowing every type. Valid TOML that any other tool reads correctly.

So `bundle` now depends on `BurntSushi/toml` (one dependency, no transitive ones). **`wiki` is no longer zero-dependency**, and the README, the retired half of [the hand-rolled parser debt](../debt/001-yaml-frontmatter-subset.md), and wikiview's design notes were corrected rather than left claiming otherwise. The frontmatter parser stays hand-rolled: it is deliberately a subset, and the surgical write API depends on that, where `wiki.toml` is a config file whose users reasonably expect all of TOML.

A malformed config is now an error rather than a shrug, which is a behaviour change worth knowing about: input that was never valid TOML but happened to scan (unquoted array items, most likely) now fails with a line number.
