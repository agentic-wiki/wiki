# Agentic Wiki CLI

A standalone CLI for [agentic-wiki](https://github.com/agentic-wiki/spec) bundles: plain-markdown knowledge bases on the Open Knowledge Format. It indexes a bundle and answers structured queries (by type, tag, path prefix), reports the link graph (broken links, orphans), lists tasks, and checks conformance. No Obsidian, no daemon, no runtime dependencies, a single static Go binary.

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
wiki status                                # bundle + index summary
wiki list --type dataset --tag accounts    # filter entries (--type, --tag, --prefix)
wiki read /finance/expenses.md             # print an entry's body (frontmatter stripped)
wiki outline /finance/expenses.md          # print its heading hierarchy
wiki search "language model" --lines       # full-text search (--type, --tag, --prefix, --lines)
wiki tasks                                 # open checkbox tasks (--all, --done, --prefix)
wiki tags --counts --sort=count            # tags in use, by frequency
wiki properties                            # frontmatter keys in use (--counts)
wiki property status --counts              # values of a key, e.g. open/done (--prefix)
wiki unresolved                            # broken internal links
wiki orphans                               # entries with no incoming links
wiki links /index.md                       # outgoing links (unique targets)
wiki backlinks /finance/income.md          # every incoming link (one per occurrence)
wiki move /a.md /archive/a.md --dry-run    # relocate/rename + rewrite links to it
wiki tidy                                  # preview tidy-ups (--links, --slug, --all to apply)
wiki check                                 # report conformance + health issues
wiki check --fix                           # repair safe issues (e.g. sync okf_version)
wiki version
```

Every command takes `--format text|json|csv|tsv` (csv/tsv suit the list-shaped commands; other commands fall back to text). Exit codes are `0` results, `1` none, `2` error, so commands compose in scripts:

```sh
wiki unresolved >/dev/null && echo "broken links found" || echo "clean"
```

Create a new bundle with `wiki init [dir]` (default: the current directory): it writes a small example, ready to use with `wiki`. Pass `--force` to write into a non-empty directory.

File arguments are **bundle paths**, not filesystem paths: they name an entry *inside* the wiki bundle, the same way links do. Two forms:

- a root-absolute path from the bundle root: `/finance/income.md`
- a bare basename: `income.md`, resolved when it's unambiguous (a name shared by two entries errors)

Because they resolve against the bundle's index rather than the working directory, they mean the same thing wherever you run from and whatever `--root` points at. `--prefix` takes the same root-absolute form to scope a listing to a subtree.

## Design

- **Standalone-first:** agents only call `wiki` directly.
- **Minimal by design:** zero external dependencies. Git is recommended but optional.
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
