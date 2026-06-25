# Agentic Wiki CLI

A standalone CLI for [agentic-wiki](https://github.com/agentic-wiki/spec) bundles: plain-markdown knowledge bases on the Open Knowledge Format. It indexes a bundle and answers structured queries (by type, tag, path), reports the link graph (broken links, orphans), lists tasks, and checks conformance. No Obsidian, no daemon, no runtime dependencies, a single static Go binary.

The **format** is defined in the spec repo ([agentic-wiki/spec](https://github.com/agentic-wiki/spec)); this repo is the **tool**.

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

Creating a bundle with `wiki init` is planned; for now a bundle is just a `wiki.toml` beside a `wiki/` content folder (see the spec).

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
