# Agentic Wiki CLI

A standalone CLI for [agentic-wiki](https://github.com/agentic-wiki/spec) bundles: plain-markdown knowledge bases on the Open Knowledge Format. It indexes a bundle and answers structured queries (by type, tag, path), reports the link graph (broken links, orphans), lists tasks, and checks conformance. No Obsidian, no daemon, no runtime dependencies, a single static Go binary.

Three layers come together: the **format** (markdown bundles, defined in the [spec repo](https://github.com/agentic-wiki/spec)); the **tool** (this repo, `wiki`, built for agents to call, though a human can run every command); and the **skill** (the manual an agent follows to drive the tool). This repo is the tool.

## Install

```sh
# macOS
curl -L https://github.com/agentic-wiki/wiki/releases/latest/download/wiki_darwin_arm64.tar.gz | tar xz
sudo mv wiki /usr/local/bin/

# Linux (amd64)
curl -L https://github.com/agentic-wiki/wiki/releases/latest/download/wiki_linux_amd64.tar.gz | tar xz
sudo mv wiki /usr/local/bin/
```

Other platforms (darwin/amd64, linux/arm64) are on the [releases page](https://github.com/agentic-wiki/wiki/releases). With a Go toolchain: `go install github.com/agentic-wiki/wiki/cmd/wiki@latest`.

## Use

Run any command from inside a wiki bundle:

```sh
wiki status                              # bundle + index summary
wiki list --type dataset --tag accounts  # filter entries (--type, --tag, --path)
wiki read /finance/expenses.md           # print an entry's body (frontmatter stripped)
wiki outline /finance/expenses.md        # print its heading hierarchy
wiki search "language model" --lines     # full-text search (--type, --tag, --path, --lines)
wiki tasks                               # open checkbox tasks (--all, --done, --path)
wiki unresolved                          # broken internal links
wiki orphans                             # entries with no incoming links
wiki check                               # report conformance + health issues
wiki version
```

Every command takes `--format text|json`. Exit codes are `0` results, `1` none, `2` error, so commands compose in scripts:

```sh
wiki unresolved >/dev/null && echo "broken links found" || echo "clean"
```

Creating a bundle with `wiki init` is planned; for now a bundle is simply a `wiki.toml` file with your markdown content beside it (see the spec).

## Design

- **Standalone-first:** agents only call `wiki` directly. Git is recommended but optional.
- **Minimal by design:** zero external dependencies.
- **Files are truth:** the index is derived from disk and fully disposable.

## Development

Requires Go 1.24+ and [just](https://github.com/casey/just).

```sh
just            # list all recipes
just check      # vet + lint + test
just test-all   # unit + smoke
just smoke      # end-to-end smoke test against a temp bundle
just build      # build the binary → ./bin/wiki
```

## The backlog is a wiki

No issue tracker, no `TASKS.md`: this repo's own backlog lives in [`tasks/`](tasks/index.md), and it is itself an agentic-wiki bundle, the exact format `wiki` operates. Each task is a markdown entry with frontmatter; the board links them together. So the tool runs on its own to-do list, with the same commands as above:

```sh
cd tasks
wiki tasks                        # what's left to build
wiki list --type task --tag debt  # known debt
wiki check                        # the backlog stays conformant
```
