package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	akg "github.com/RobertGumeny/akg-format"
)

func main() {
	// The example writes to a temporary file by default so it is safe to run from
	// a clean checkout. If the user passes a path, we keep that file for them to
	// inspect afterward.
	path, cleanup, err := examplePath(os.Args[1:])
	if err != nil {
		fatal(err)
	}
	defer cleanup()

	if err := run(path); err != nil {
		fatal(err)
	}
}

func run(path string) error {
	// Create starts a brand-new AKG file and opens it for mutation. This is the
	// normal starting point when an application or SDK wants to initialize a graph.
	store, err := akg.Create(path)
	if err != nil {
		return fmt.Errorf("create AKG file: %w", err)
	}

	// Real applications should use their own timestamp policy. This example uses
	// one fixed value so the data is easy to read and repeatable.
	now := uint64(1_700_000_000_000_000)

	// Nodes are the durable pieces of knowledge in the graph. The string passed to
	// PutNode is the node ID. The node Type is stored in the payload and is used
	// again later for exact lookup.
	if _, err := store.PutNode("paper-akg-v1", akg.Node{
		Type:      "note",
		Title:     "AKG v1 format note",
		Body:      "AKG stores current graph state in a compact binary file with an append-only WAL for committed mutations.",
		Tags:      []string{"akg", "format"},
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}); err != nil {
		return fmt.Errorf("put format note: %w", err)
	}
	if _, err := store.PutNode("decision-minimal-api", akg.Node{
		Type:      "decision",
		Title:     "Keep the v1 API minimal",
		Body:      "The core package exposes lifecycle operations, exact reads, and whole-state lists instead of SDK-style query helpers.",
		Tags:      []string{"api", "v1"},
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}); err != nil {
		return fmt.Errorf("put API decision: %w", err)
	}
	if _, err := store.PutNode("conformance-corpus", akg.Node{
		Type:      "artifact",
		Title:     "Conformance corpus",
		Body:      "Fixtures let other implementations check whether they accept and reject the same AKG files as the reference implementation.",
		Tags:      []string{"conformance", "fixtures"},
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}); err != nil {
		return fmt.Errorf("put conformance artifact: %w", err)
	}

	// Edges make relationships explicit. An edge identity is the triple
	// (from node, relation, to node), so later we can read this exact edge back.
	if _, err := store.PutEdge(akg.Edge{
		FromType:  "note",
		FromNode:  "paper-akg-v1",
		Relation:  "supports",
		ToType:    "decision",
		ToNode:    "decision-minimal-api",
		Strength:  0.9,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}); err != nil {
		return fmt.Errorf("put supports edge: %w", err)
	}
	if _, err := store.PutEdge(akg.Edge{
		FromType:  "note",
		FromNode:  "conformance-corpus",
		Relation:  "checks",
		ToType:    "note",
		ToNode:    "paper-akg-v1",
		Strength:  0.8,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}); err != nil {
		return fmt.Errorf("put checks edge: %w", err)
	}

	// Commit makes the pending node and edge mutations durable in the AKG file.
	// Close commits any remaining work and releases the file handle.
	if err := store.Commit(); err != nil {
		return fmt.Errorf("commit mutations: %w", err)
	}
	if err := store.Close(); err != nil {
		return fmt.Errorf("close store: %w", err)
	}

	// Reopen through the ordinary public path. This proves the file on disk can be
	// read back by a normal AKG reader, not just by the in-memory store we wrote to.
	reopened, err := akg.Open(path)
	if err != nil {
		return fmt.Errorf("reopen AKG file: %w", err)
	}

	// The v1 core API intentionally keeps reads simple: exact node lookup, exact
	// edge lookup, and whole-state lists. Richer query helpers belong in SDKs.
	note, ok := reopened.GetNode("note", "paper-akg-v1")
	if !ok {
		return errors.New("expected note paper-akg-v1 after reopen")
	}
	decision, ok := reopened.GetNode("decision", "decision-minimal-api")
	if !ok {
		return errors.New("expected decision decision-minimal-api after reopen")
	}
	edge, ok := reopened.GetEdge("note", "paper-akg-v1", "supports", "decision", "decision-minimal-api")
	if !ok {
		return errors.New("expected supports edge after reopen")
	}

	// Print a small human-readable summary so someone running the example can see
	// what was written and read without inspecting the binary file directly.
	fmt.Printf("AKG lifecycle example wrote %s\n\n", path)
	printNode(note)
	printNode(decision)
	fmt.Printf("Read edge: %s --%s--> %s (strength %.1f)\n\n", edge.FromNode, edge.Relation, edge.ToNode, edge.Strength)
	fmt.Printf("Current state contains %d nodes and %d edges.\n", len(reopened.ListNodes()), len(reopened.ListEdges()))

	// Compaction rewrites the file to current live state only. Validation then
	// checks that the compacted file still opens under normal strict rules.
	if err := reopened.Compact(); err != nil {
		return fmt.Errorf("compact AKG file: %w", err)
	}
	if err := akg.Validate(path); err != nil {
		return fmt.Errorf("validate compacted AKG file: %w", err)
	}
	fmt.Println("Compacted and validated successfully.")
	return nil
}

func printNode(rec akg.NodeRecord) {
	// Tags remain []string in the AKG API. Joining them here is only for readable
	// terminal output.
	fmt.Printf("Read node %q (%s)\n", rec.ID, rec.Node.Type)
	fmt.Printf("Title: %s\n", rec.Node.Title)
	fmt.Printf("Tags:  %s\n", strings.Join(rec.Node.Tags, ", "))
	fmt.Printf("Body:  %s\n\n", rec.Node.Body)
}

func examplePath(args []string) (string, func(), error) {
	if len(args) > 1 {
		return "", func() {}, fmt.Errorf("usage: go run ./examples/lifecycle [output.akg]")
	}
	if len(args) == 1 {
		path := args[0]
		// Refuse to overwrite an existing file; examples should be safe by default.
		if _, err := os.Stat(path); err == nil {
			return "", func() {}, fmt.Errorf("refusing to overwrite existing file: %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", func() {}, err
		}
		return path, func() {}, nil
	}

	// No path was provided, so create a throwaway directory and clean it up when
	// the program exits.
	dir, err := os.MkdirTemp("", "akg-lifecycle-*")
	if err != nil {
		return "", func() {}, err
	}
	return filepath.Join(dir, "example.akg"), func() { _ = os.RemoveAll(dir) }, nil
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}
