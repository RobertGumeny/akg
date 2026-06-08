package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	akg "github.com/RobertGumeny/akg/sdk/akg-go"
)

// Node types above this count are collapsed to a summary so a long session
// (hundreds of per-hand nodes) still prints as one clean screen. --all overrides.
const collapseThreshold = 12

// runShow renders a .akg file as readable text: the knowledge an agent wrote,
// grouped by the node types it invented, pulled straight out of the binary store
// through the SDK's Open + Snapshot. It is the human-facing companion to the
// reference CLI's JSON `inspect`, and a worked example of reading a store.
func runShow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("akg-go show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit the full snapshot as JSON instead of readable text")
	all := fs.Bool("all", false, "print every node body, including large/per-hand types")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: akg-go show [--json] [--all] PATH")
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}
	path := fs.Arg(0)

	// Open creates an empty store when the file is absent; for a read-only viewer a
	// missing path is a typo, so fail loudly instead of printing an empty graph.
	if _, err := os.Stat(path); err != nil {
		fmt.Fprintf(stderr, "cannot read %s: %v\n", path, err)
		return 1
	}

	store, err := akg.Open(path)
	if err != nil {
		fmt.Fprintf(stderr, "open failed: %v\n", err)
		return 1
	}
	defer store.Close() //nolint:errcheck

	snap, err := store.Snapshot()
	if err != nil {
		fmt.Fprintf(stderr, "snapshot failed: %v\n", err)
		return 1
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(snap); err != nil {
			fmt.Fprintf(stderr, "encode failed: %v\n", err)
			return 1
		}
		return 0
	}

	renderText(stdout, path, snap, *all)
	return 0
}

func renderText(w io.Writer, path string, snap akg.Snapshot, all bool) {
	fmt.Fprintf(w, "%s\n%d nodes / %d edges\n", path, len(snap.Nodes), len(snap.Edges))

	byType := map[string][]akg.Node{}
	for _, n := range snap.Nodes {
		byType[n.Type] = append(byType[n.Type], n)
	}

	// Smallest groups first: the curated, high-signal types an agent distills
	// (opponent, pattern) lead; bulky per-hand logs sink to the bottom.
	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool {
		if len(byType[types[i]]) != len(byType[types[j]]) {
			return len(byType[types[i]]) < len(byType[types[j]])
		}
		return types[i] < types[j]
	})

	for _, t := range types {
		nodes := byType[t]
		sortNodes(nodes)
		fmt.Fprintf(w, "\n%s (%d)\n", strings.ToUpper(t), len(nodes))

		if !all && len(nodes) > collapseThreshold {
			for _, n := range nodes[:3] {
				fmt.Fprintf(w, "  - %s\n", titleOf(n))
			}
			fmt.Fprintf(w, "  ... %d more (pass --all to show every node)\n", len(nodes)-3)
			continue
		}
		for _, n := range nodes {
			fmt.Fprintf(w, "  %s\n", titleOf(n))
			if body := strings.TrimSpace(n.Body); body != "" {
				fmt.Fprintf(w, "    %s\n", indent(body))
			}
		}
	}

	if len(snap.Edges) > 0 {
		fmt.Fprintf(w, "\nEDGES (%d)\n", len(snap.Edges))
		edges := append([]akg.Edge(nil), snap.Edges...)
		sort.Slice(edges, func(i, j int) bool { return edgeKey(edges[i]) < edgeKey(edges[j]) })
		shown := edges
		if !all && len(edges) > collapseThreshold {
			shown = edges[:collapseThreshold]
		}
		for _, e := range shown {
			fmt.Fprintf(w, "  %s/%s -%s-> %s/%s\n", e.From.Type, e.From.ID, e.Relation, e.To.Type, e.To.ID)
		}
		if len(shown) < len(edges) {
			fmt.Fprintf(w, "  ... %d more (pass --all)\n", len(edges)-len(shown))
		}
	}
}

func titleOf(n akg.Node) string {
	if strings.TrimSpace(n.Title) != "" {
		return n.Title
	}
	return n.ID
}

func sortNodes(nodes []akg.Node) {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
}

func edgeKey(e akg.Edge) string {
	return e.From.Type + "/" + e.From.ID + "|" + e.Relation + "|" + e.To.Type + "/" + e.To.ID
}

func indent(body string) string {
	return strings.ReplaceAll(body, "\n", "\n    ")
}
