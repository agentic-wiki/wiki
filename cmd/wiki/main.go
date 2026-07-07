// Command wiki is a standalone CLI for agentic-wiki bundles (OKF markdown).
package main

import (
	"fmt"
	"os"
	"strings"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

// rootDir is the bundle to operate on, set by a leading --root flag. Empty
// means discover from the current directory.
var rootDir string

const usage = `Wiki: query an agentic-wiki bundle

Usage: wiki [--root <dir>] <command> [flags]

Commands:
  init          scaffold a new bundle (--workflow <name>, --force)
  status        bundle + index summary
  list, ls      list entries (--where key=value --prefix --sort=path|timestamp --reverse)
  read          print an entry's body (frontmatter stripped)
  outline       print an entry's heading hierarchy
  table         extract a dataset's markdown table as csv/json (--n)
  search        full-text search over entries (--where key=value --prefix --lines)
  tasks         list open checklist items; optional [file] scopes to one entry (--all --done --prefix)
  tags          list tags in use (--counts --sort=name|count --prefix)
  properties    list frontmatter keys in use (--counts --sort --prefix)
  property      list values of a frontmatter key (--counts --sort --prefix)
  unresolved    broken internal links
  orphans       entries with no incoming links
  links         an entry's outgoing links
  backlinks     every link that points to an entry
  move, mv      relocate or rename an entry, rewriting links to it (--dry-run)
  tidy          canonicalize the bundle: --links, --slug, --all (bare = preview)
  check         report conformance + health issues (--fix repairs safe ones)
  version       print the version

Run 'wiki <command> -h' to see a command's flags
  --root <dir>      operate on the bundle at <dir> (default: discover from cwd)

Every command accepts --format text|json|csv|tsv (default text; csv/tsv suit list-shaped results)
Filter frontmatter with --where key=value (repeatable = AND) on list/search; type and tags are
  ordinary fields, e.g. --where type=note, --where tags=bug
list --format json carries each entry's full frontmatter; csv/tsv carry the canonical columns
Exit codes: 0 ok, 1 no match or check errors, 2 error
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	args, code := applyRoot(args)
	if code != 0 {
		return code
	}
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	switch cmd := args[0]; cmd {
	case "help", "-h", "--help":
		fmt.Print(usage)
		return 0
	case "init":
		return cmdInit(args[1:])
	case "version", "--version", "-v":
		fmt.Println("wiki", Version)
		return 0
	case "status":
		return cmdStatus(args[1:])
	case "list", "ls":
		return cmdList(args[1:])
	case "read":
		return cmdRead(args[1:])
	case "outline":
		return cmdOutline(args[1:])
	case "search":
		return cmdSearch(args[1:])
	case "tasks":
		return cmdTasks(args[1:])
	case "table":
		return cmdTable(args[1:])
	case "tags":
		return cmdTags(args[1:])
	case "properties":
		return cmdProperties(args[1:])
	case "property":
		return cmdProperty(args[1:])
	case "unresolved":
		return cmdUnresolved(args[1:])
	case "orphans":
		return cmdOrphans(args[1:])
	case "links":
		return cmdLinks(args[1:])
	case "backlinks":
		return cmdBacklinks(args[1:])
	case "move", "mv":
		return cmdMove(args[1:])
	case "tidy":
		return cmdTidy(args[1:])
	case "check":
		return cmdCheck(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", cmd, usage)
		return 2
	}
}

// applyRoot consumes a leading --root <dir> option, recording the bundle to
// operate on (no chdir), and returns the remaining args. Unlike git's -C it
// does not change the working directory: it only redirects bundle discovery,
// so commands that don't open a bundle (notably init) are unaffected.
func applyRoot(args []string) ([]string, int) {
	rootDir = ""
	for len(args) > 0 {
		switch {
		case args[0] == "--root":
			if len(args) < 2 {
				fmt.Fprintln(os.Stderr, "wiki: --root needs a directory")
				return nil, 2
			}
			rootDir, args = args[1], args[2:]
		case strings.HasPrefix(args[0], "--root="):
			rootDir, args = args[0][len("--root="):], args[1:]
		default:
			return args, 0
		}
	}
	return args, 0
}
