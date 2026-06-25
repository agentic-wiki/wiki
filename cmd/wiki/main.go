// Command wiki is a standalone CLI for agentic-wiki bundles (OKF markdown).
package main

import (
	"fmt"
	"os"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

const usage = `wiki — query an agentic-wiki bundle

usage: wiki <command> [flags]

commands:
  status        bundle + index summary
  list          list entries (--type --tag --path)
  tasks         list checkbox tasks (--all --done --path)
  unresolved    broken internal links
  orphans       entries with no incoming links
  check         report conformance + health issues
  version       print the version

every command accepts --format text|json (default text)
exit codes: 0 results, 1 none, 2 error
`

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
	switch cmd := args[0]; cmd {
	case "help", "-h", "--help":
		fmt.Print(usage)
		return 0
	case "version", "--version", "-v":
		fmt.Println("wiki", Version)
		return 0
	case "status":
		return cmdStatus(args[1:])
	case "list":
		return cmdList(args[1:])
	case "tasks":
		return cmdTasks(args[1:])
	case "unresolved":
		return cmdUnresolved(args[1:])
	case "orphans":
		return cmdOrphans(args[1:])
	case "check":
		return cmdCheck(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", cmd, usage)
		return 2
	}
}
