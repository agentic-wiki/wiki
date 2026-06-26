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

const usage = `wiki — query an agentic-wiki bundle

usage: wiki [--root <dir>] <command> [flags]

commands:
  init          scaffold a new bundle (--force into a non-empty dir)
  status        bundle + index summary
  list          list entries (--type --tag --prefix)
  read          print an entry's body (frontmatter stripped)
  outline       print an entry's heading hierarchy
  search        full-text search over entries (--type --tag --prefix --lines)
  tasks         list checkbox tasks (--all --done --prefix)
  unresolved    broken internal links
  orphans       entries with no incoming links
  links         an entry's outgoing links
  backlinks     entries that link to an entry
  move          relocate or rename an entry, rewriting links to it (--dry-run)
  check         report conformance + health issues (--fix repairs safe ones)
  version       print the version

run 'wiki <command> -h' to see a command's flags
--root <dir>      operate on the bundle at <dir> (default: discover from cwd)
every command accepts --format text|json (default text)
exit codes: 0 results, 1 none, 2 error
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
	case "list":
		return cmdList(args[1:])
	case "read":
		return cmdRead(args[1:])
	case "outline":
		return cmdOutline(args[1:])
	case "search":
		return cmdSearch(args[1:])
	case "tasks":
		return cmdTasks(args[1:])
	case "unresolved":
		return cmdUnresolved(args[1:])
	case "orphans":
		return cmdOrphans(args[1:])
	case "links":
		return cmdLinks(args[1:])
	case "backlinks":
		return cmdBacklinks(args[1:])
	case "move":
		return cmdMove(args[1:])
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
