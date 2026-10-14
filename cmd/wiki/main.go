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
  init          scaffold a new bundle (--workflow <name>, --force; 'init -h' lists workflows)
  status        bundle counts: entries, links, tags, checkboxes, broken, orphans
  list, ls      list entries (--where key=value --prefix --sort=path|timestamp --reverse)
  read          print an entry's body (frontmatter stripped)
  dump          print every entry as one markdown stream, e.g. llms-full.txt (--frontmatter --prefix --where)
  outline       print an entry's heading hierarchy
  table         extract a dataset's markdown table as csv/json (--n)
  search        full-text search over entries (every word; --any --exact --where --prefix --lines)
  checkboxes    list open checklist items; optional [file] scopes to one entry (--all --done --prefix --where)
  tags          list tags in use (--counts --sort=name|count --prefix --where)
  properties    list frontmatter keys in use (--counts --sort --prefix --where)
  property      list values of a frontmatter key (--counts --sort --prefix --where)
  unresolved    broken internal links
  orphans       entries with no incoming links
  links         an entry's outgoing links
  backlinks     every link that points to an entry
  move, mv      relocate or rename an entry, rewriting links to it (--dry-run, --include-frontmatter)
  tidy          canonicalize the bundle: --links, --slug, --wikilinks, --all (bare = preview)
  check         report conformance + health issues (--fix repairs safe ones)
  version       print the version

Run 'wiki <command> -h' to see a command's flags
  --root <dir>      operate on the bundle at <dir>, before or after the command (default: discover from cwd)

Every command that reports results accepts --format text|json|csv|tsv (default text; csv/tsv suit
  list-shaped results); version prints a bare string, dump raw markdown
Two filters, available wherever a set of entries is narrowed (list, search, dump, checkboxes,
  tags, properties, property): --prefix <path> for where, --where key=value or key!=value for what
  (repeatable = AND). type and tags are ordinary fields, e.g. --where type=note, --where status!=done
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
	case "dump":
		return cmdDump(args[1:])
	case "outline":
		return cmdOutline(args[1:])
	case "search":
		return cmdSearch(args[1:])
	case "checkboxes":
		return cmdCheckboxes(args[1:])
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

// applyRoot consumes a --root <dir> option wherever it appears, before or after
// the command, recording the bundle to operate on (no chdir), and returns the
// remaining args. It is global, so no command declares it: it is taken out here,
// before a command's own flags are parsed. Arguments after a "--" are left
// alone. Unlike git's -C it does not change the working directory: it only
// redirects bundle discovery, so commands that don't open a bundle (notably
// init) are unaffected.
func applyRoot(args []string) ([]string, int) {
	rootDir = ""
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--":
			return append(rest, args[i:]...), 0
		case a == "--root" || a == "-root":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "wiki: --root needs a directory")
				return nil, 2
			}
			i++
			rootDir = args[i]
		case strings.HasPrefix(a, "--root=") || strings.HasPrefix(a, "-root="):
			_, rootDir, _ = strings.Cut(a, "=")
		default:
			rest = append(rest, a)
		}
	}
	return rest, 0
}
