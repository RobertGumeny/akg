// Command akg-go is the akg-go SDK's command-line entry point. It is a single
// multiplexer binary in the conventional `akg-go <command> [args]` shape:
//
//	akg-go show <PATH>     render a .akg file as readable text
//	akg-go docs ...        query the embedded akg-go API documentation graph
//	akg-go gen-docs        regenerate the embedded docs graph from docs/manifest.json
//
// Each subcommand is implemented in its own file (show.go, docs.go, gendocs.go)
// as a runX(args, stdout, stderr) int, so they share one binary and one test
// harness instead of shipping as separate per-command executables.
package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `usage: akg-go <command> [args]

Commands:
  show <PATH> [--json] [--all]        render a .akg file as readable text
  docs <overview|explain|search|dump> query the embedded akg-go API docs graph
  gen-docs                            regenerate the embedded docs graph from docs/manifest.json
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "show":
		return runShow(rest, stdout, stderr)
	case "docs":
		return runDocs(rest, stdout, stderr)
	case "gen-docs":
		return runGenDocs(rest, stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", cmd, usage)
		return 2
	}
}
