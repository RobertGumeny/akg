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
	store, err := akg.Create(path)
	if err != nil {
		return fmt.Errorf("create AKG file: %w", err)
	}

	now := uint64(1_700_000_000_000_000)
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

	if _, err := store.PutEdge(akg.Edge{
		FromNode:  "paper-akg-v1",
		Relation:  "supports",
		ToNode:    "decision-minimal-api",
		Strength:  0.9,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}); err != nil {
		return fmt.Errorf("put supports edge: %w", err)
	}
	if _, err := store.PutEdge(akg.Edge{
		FromNode:  "conformance-corpus",
		Relation:  "checks",
		ToNode:    "paper-akg-v1",
		Strength:  0.8,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}); err != nil {
		return fmt.Errorf("put checks edge: %w", err)
	}

	if err := store.Commit(); err != nil {
		return fmt.Errorf("commit mutations: %w", err)
	}
	if err := store.Close(); err != nil {
		return fmt.Errorf("close store: %w", err)
	}

	reopened, err := akg.Open(path)
	if err != nil {
		return fmt.Errorf("reopen AKG file: %w", err)
	}

	note, ok := reopened.GetNode("note", "paper-akg-v1")
	if !ok {
		return errors.New("expected note paper-akg-v1 after reopen")
	}
	decision, ok := reopened.GetNode("decision", "decision-minimal-api")
	if !ok {
		return errors.New("expected decision decision-minimal-api after reopen")
	}
	edge, ok := reopened.GetEdge("paper-akg-v1", "supports", "decision-minimal-api")
	if !ok {
		return errors.New("expected supports edge after reopen")
	}

	fmt.Printf("AKG lifecycle example wrote %s\n\n", path)
	printNode(note)
	printNode(decision)
	fmt.Printf("Read edge: %s --%s--> %s (strength %.1f)\n\n", edge.FromNode, edge.Relation, edge.ToNode, edge.Strength)
	fmt.Printf("Current state contains %d nodes and %d edges.\n", len(reopened.ListNodes()), len(reopened.ListEdges()))

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
		if _, err := os.Stat(path); err == nil {
			return "", func() {}, fmt.Errorf("refusing to overwrite existing file: %s", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", func() {}, err
		}
		return path, func() {}, nil
	}

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
