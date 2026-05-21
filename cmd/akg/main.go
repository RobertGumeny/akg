package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/RobertGumeny/akg-format"
)

type inspectOutput struct {
	NodeCount int              `json:"node_count"`
	EdgeCount int              `json:"edge_count"`
	Nodes     []akg.NodeRecord `json:"nodes"`
	Edges     []akg.Edge       `json:"edges"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 {
		fmt.Fprintln(stderr, "usage: akg validate|inspect|compact PATH")
		return 2
	}
	cmd, path := args[0], args[1]
	switch cmd {
	case "validate":
		if err := akg.Validate(path); err != nil {
			fmt.Fprintf(stderr, "validation failed: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "valid")
		return 0
	case "inspect":
		st, err := akg.Open(path)
		if err != nil {
			fmt.Fprintf(stderr, "inspect failed: %v\n", err)
			return 1
		}
		nodes := st.ListNodes()
		edges := st.ListEdges()
		out := inspectOutput{NodeCount: len(nodes), EdgeCount: len(edges), Nodes: nodes, Edges: edges}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintf(stderr, "inspect failed: %v\n", err)
			return 1
		}
		return 0
	case "compact":
		if err := akg.Compact(path); err != nil {
			fmt.Fprintf(stderr, "compaction failed: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "compacted")
		return 0
	default:
		fmt.Fprintln(stderr, "usage: akg validate|inspect|compact PATH")
		return 2
	}
}
