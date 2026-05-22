package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	akg "github.com/RobertGumeny/akg-go"
)

func main() {
	path := filepath.Join(os.TempDir(), "akg-basic-example.akg")
	os.Remove(path)

	// Open (or create) a store at the given path.
	store, err := akg.Open(path)
	must(err, "open store")

	// Write nodes. Tags let you group nodes by any label you choose.
	alice, err := store.PutNode("person", "alice", akg.NodeFields{
		Title: "Alice",
		Body:  "A researcher in knowledge graphs.",
		Meta:  map[string]any{"role": "lead"},
	}, []string{"active", "researcher"})
	must(err, "put alice")

	bob, err := store.PutNode("person", "bob", akg.NodeFields{
		Title: "Bob",
		Body:  "A software engineer.",
		Meta:  map[string]any{"role": "engineer"},
	}, []string{"active"})
	must(err, "put bob")

	paper, err := store.PutNode("paper", "paper-001", akg.NodeFields{
		Title: "Graph-Based Context Compression",
		Body:  "Explores AKG as an agent memory substrate.",
	}, []string{"published"})
	must(err, "put paper")

	// Write edges connecting the nodes.
	err = store.PutEdge(alice, "authored", paper, akg.EdgeFields{Strength: 1.0})
	must(err, "put authored edge")

	err = store.PutEdge(bob, "reviewed", paper, akg.EdgeFields{Strength: 0.8})
	must(err, "put reviewed edge")

	err = store.PutEdge(alice, "collaborates_with", bob, akg.EdgeFields{})
	must(err, "put collaborates-with edge")

	// Commit and close. Reopen to show durability.
	must(store.Close(), "close store")

	store, err = akg.Open(path)
	must(err, "reopen store")
	defer store.Close()

	// Read a single node by type and ID.
	node, err := store.GetNode("person", "alice")
	must(err, "get alice")
	fmt.Printf("Node: %s/%s — %q\n", node.Type, node.ID, node.Title)
	fmt.Printf("  body: %s\n", node.Body)
	fmt.Printf("  tags: [%s]\n", strings.Join(node.Tags, ", "))
	fmt.Printf("  meta: %v\n", node.Meta)

	// List all nodes carrying a tag.
	fmt.Println("\nActive people:")
	actives, err := store.ListNodesByTag("active")
	must(err, "list by tag")
	for _, n := range actives {
		fmt.Printf("  %s/%s — %q\n", n.Type, n.ID, n.Title)
	}

	// Walk outbound edges from Alice.
	fmt.Printf("\nOutbound edges from %s/%s:\n", alice.Type, alice.ID)
	edges, err := store.OutboundEdges(alice, "")
	must(err, "outbound edges")
	for _, e := range edges {
		fmt.Printf("  -[%s]-> %s/%s (strength %.1f)\n", e.Relation, e.To.Type, e.To.ID, e.Strength)
	}
}

func must(err error, label string) {
	if err != nil {
		log.Fatalf("%s: %v", label, err)
	}
}
