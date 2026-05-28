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
	akgdocs "github.com/RobertGumeny/akg/sdk/akg-go/docs"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: akg-go-docs <overview|explain <Name>|search <query>|dump [--format markdown|json]>")
		return 1
	}

	store, err := akg.OpenBytes(akgdocs.Graph)
	if err != nil {
		fmt.Fprintf(stderr, "Error opening docs graph: %v\n", err)
		return 1
	}
	defer store.Close() //nolint:errcheck

	switch args[0] {
	case "overview":
		return cmdOverview(store, stdout, stderr)
	case "explain":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "Usage: akg-go-docs explain <Name>")
			return 1
		}
		return cmdExplain(store, args[1], stdout, stderr)
	case "search":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "Usage: akg-go-docs search <query>")
			return 1
		}
		return cmdSearch(store, args[1], stdout, stderr)
	case "dump":
		fs := flag.NewFlagSet("dump", flag.ContinueOnError)
		fs.SetOutput(stderr)
		format := fs.String("format", "markdown", "output format: markdown or json")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		if *format != "markdown" && *format != "json" {
			fmt.Fprintln(stderr, "--format must be markdown or json")
			return 1
		}
		return cmdDump(store, *format, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n", args[0])
		return 1
	}
}

func cmdOverview(store *akg.Store, stdout, _ io.Writer) int {
	nodes, _ := store.ListNodes("")

	byType := make(map[string][]akg.Node)
	for _, n := range nodes {
		byType[n.Type] = append(byType[n.Type], n)
	}

	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Strings(types)

	for _, t := range types {
		fmt.Fprintf(stdout, "\n## %s\n\n", t)
		for _, n := range byType[t] {
			fmt.Fprintf(stdout, "- **%s** — %s\n", n.Title, n.Body)
		}
	}
	return 0
}

func cmdExplain(store *akg.Store, name string, stdout, stderr io.Writer) int {
	nodes, _ := store.ListNodes("")

	var match *akg.Node
	for i := range nodes {
		if strings.EqualFold(nodes[i].Title, name) {
			match = &nodes[i]
			break
		}
	}

	if match == nil {
		fmt.Fprintf(stderr, "Not found: no node with title %q\n", name)
		return 1
	}

	nodeRef := akg.NodeRef{Type: match.Type, ID: match.ID}
	outEdges, _ := store.OutboundEdges(nodeRef, "")
	inEdges, _ := store.InboundEdges(nodeRef, "")

	fmt.Fprintf(stdout, "# %s\n", match.Title)
	fmt.Fprintf(stdout, "\n**Type:** `%s`\n", match.Type)
	if match.Body != "" {
		fmt.Fprintf(stdout, "\n%s\n", match.Body)
	}
	if len(match.Tags) > 0 {
		tagStrs := make([]string, len(match.Tags))
		for i, t := range match.Tags {
			tagStrs[i] = "`" + t + "`"
		}
		fmt.Fprintf(stdout, "\n**Tags:** %s\n", strings.Join(tagStrs, ", "))
	}

	type edgeEntry struct {
		dir  string
		edge akg.Edge
	}
	byRelation := make(map[string][]edgeEntry)
	for _, e := range outEdges {
		byRelation[e.Relation] = append(byRelation[e.Relation], edgeEntry{"out", e})
	}
	for _, e := range inEdges {
		byRelation[e.Relation] = append(byRelation[e.Relation], edgeEntry{"in", e})
	}

	if len(byRelation) > 0 {
		relations := make([]string, 0, len(byRelation))
		for r := range byRelation {
			relations = append(relations, r)
		}
		sort.Strings(relations)

		fmt.Fprintf(stdout, "\n## Relations\n")
		for _, rel := range relations {
			fmt.Fprintf(stdout, "\n### %s\n\n", rel)
			for _, entry := range byRelation[rel] {
				if entry.dir == "out" {
					fmt.Fprintf(stdout, "- → **%s/%s**\n", entry.edge.To.Type, entry.edge.To.ID)
				} else {
					fmt.Fprintf(stdout, "- ← **%s/%s**\n", entry.edge.From.Type, entry.edge.From.ID)
				}
			}
		}
	}

	meta := match.Meta
	srcPath, _ := meta["source_path"].(string)
	anchor, _ := meta["anchor"].(string)
	if srcPath != "" && anchor != "" {
		fmt.Fprintf(stdout, "\n**Source:** `%s#%s`\n", srcPath, anchor)
	}
	return 0
}

func cmdSearch(store *akg.Store, query string, stdout, _ io.Writer) int {
	nodes, _ := store.ListNodes("")
	q := strings.ToLower(query)

	var matches []akg.Node
	for _, n := range nodes {
		if strings.Contains(strings.ToLower(n.Title), q) ||
			strings.Contains(strings.ToLower(n.Body), q) ||
			tagsContain(n.Tags, q) {
			matches = append(matches, n)
		}
	}

	if len(matches) == 0 {
		fmt.Fprintf(stdout, "No results for %q\n", query)
		return 0
	}

	fmt.Fprintf(stdout, "## Search results for %q\n\n", query)
	for _, n := range matches {
		fmt.Fprintf(stdout, "- **%s** (`%s`) — %s\n", n.Title, n.Type, n.Body)
	}
	return 0
}

func tagsContain(tags []string, q string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

func nodeToMarkdown(n akg.Node) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "**%s** (`%s/%s`)\n", n.Title, n.Type, n.ID)
	if n.Body != "" {
		fmt.Fprintf(&sb, "> %s\n", n.Body)
	}
	if len(n.Tags) > 0 {
		tagStrs := make([]string, len(n.Tags))
		for i, t := range n.Tags {
			tagStrs[i] = "`" + t + "`"
		}
		fmt.Fprintf(&sb, "Tags: %s\n", strings.Join(tagStrs, ", "))
	}
	return sb.String()
}

func cmdDump(store *akg.Store, format string, stdout, stderr io.Writer) int {
	if format == "json" {
		snap, err := store.Snapshot()
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		data, err := json.MarshalIndent(snap, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "%s\n", data)
		return 0
	}

	nodes, _ := store.ListNodes("")
	byType := make(map[string][]akg.Node)
	for _, n := range nodes {
		byType[n.Type] = append(byType[n.Type], n)
	}

	types := make([]string, 0, len(byType))
	for t := range byType {
		types = append(types, t)
	}
	sort.Strings(types)

	fmt.Fprintln(stdout, "# akg-go Documentation")
	for _, t := range types {
		fmt.Fprintf(stdout, "\n## %s\n\n", t)
		for _, n := range byType[t] {
			fmt.Fprintln(stdout, nodeToMarkdown(n))
		}
	}
	return 0
}
