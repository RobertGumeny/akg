package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	akg "github.com/RobertGumeny/akg/sdk/akg-go"
)

type manifestNode struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Title   string   `json:"title"`
	Body    string   `json:"body"`
	Tags    []string `json:"tags"`
	Anchor  string   `json:"anchor"`
	Heading string   `json:"heading"`
}

type manifestEdge struct {
	From     string `json:"from"`
	Relation string `json:"relation"`
	To       string `json:"to"`
}

type manifestMeta struct {
	Language    string `json:"language"`
	Package     string `json:"package"`
	Version     string `json:"version"`
	SourcePath  string `json:"source_path"`
	GeneratedAt string `json:"generated_at"`
}

type manifest struct {
	Meta  manifestMeta   `json:"meta"`
	Nodes []manifestNode `json:"nodes"`
	Edges []manifestEdge `json:"edges"`
}

// runGenDocs regenerates the embedded docs graph (docs/akg-go-docs.akg and the
// companion JSON) from docs/manifest.json, relative to the current directory.
// This is a maintainer/build step, run by CI as `go run ./cmd/akg-go gen-docs`.
func runGenDocs(_ []string, stdout, stderr io.Writer) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	docsDir := filepath.Join(cwd, "docs")
	manifestPath := filepath.Join(docsDir, "manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "reading manifest: %v\n", err)
		return 1
	}

	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		fmt.Fprintf(stderr, "parsing manifest: %v\n", err)
		return 1
	}

	generatedAt, err := time.Parse(time.RFC3339, m.Meta.GeneratedAt)
	if err != nil {
		fmt.Fprintf(stderr, "parsing generated_at: %v\n", err)
		return 1
	}
	fixedMicros := uint64(generatedAt.UnixMicro())
	akg.SetTestNow(fixedMicros)
	defer akg.SetTestNow(0)

	outAkg := filepath.Join(docsDir, "akg-go-docs.akg")
	outJSON := filepath.Join(docsDir, "akg-go-docs.json")

	if err := os.Remove(outAkg); err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "removing old akg file: %v\n", err)
		return 1
	}

	store, err := akg.Open(outAkg)
	if err != nil {
		fmt.Fprintf(stderr, "opening store: %v\n", err)
		return 1
	}

	refs := make(map[string]akg.NodeRef)

	for _, node := range m.Nodes {
		colonIdx := strings.Index(node.ID, ":")
		if colonIdx < 0 {
			fmt.Fprintf(stderr, "invalid node id (no colon): %s\n", node.ID)
			return 1
		}
		akgType := node.ID[:colonIdx]
		akgID := node.ID[colonIdx+1:]

		ref, err := store.PutNode(akgType, akgID, akg.NodeFields{
			Title: node.Title,
			Body:  node.Body,
			Meta: map[string]any{
				"source_path":  m.Meta.SourcePath,
				"heading":      node.Heading,
				"anchor":       node.Anchor,
				"language":     m.Meta.Language,
				"package":      m.Meta.Package,
				"version":      m.Meta.Version,
				"generated_at": m.Meta.GeneratedAt,
			},
		}, node.Tags)
		if err != nil {
			fmt.Fprintf(stderr, "PutNode %s: %v\n", node.ID, err)
			return 1
		}
		refs[node.ID] = ref
	}

	for _, edge := range m.Edges {
		fromRef, ok := refs[edge.From]
		if !ok {
			fmt.Fprintf(stderr, "edge references unknown node: %s\n", edge.From)
			return 1
		}
		toRef, ok := refs[edge.To]
		if !ok {
			fmt.Fprintf(stderr, "edge references unknown node: %s\n", edge.To)
			return 1
		}
		if err := store.PutEdge(fromRef, edge.Relation, toRef, akg.EdgeFields{}); err != nil {
			fmt.Fprintf(stderr, "PutEdge %s -> %s: %v\n", edge.From, edge.To, err)
			return 1
		}
	}

	snap, err := store.Snapshot()
	if err != nil {
		fmt.Fprintf(stderr, "snapshot: %v\n", err)
		return 1
	}

	if err := store.Compact(); err != nil {
		fmt.Fprintf(stderr, "compact: %v\n", err)
		return 1
	}
	if err := store.Close(); err != nil {
		fmt.Fprintf(stderr, "close: %v\n", err)
		return 1
	}

	jsonData, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "marshal json: %v\n", err)
		return 1
	}
	if err := os.WriteFile(outJSON, append(jsonData, '\n'), 0o644); err != nil {
		fmt.Fprintf(stderr, "writing json: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Generated %d nodes, %d edges\n", len(m.Nodes), len(m.Edges))
	fmt.Fprintf(stdout, "  %s\n", outAkg)
	fmt.Fprintf(stdout, "  %s\n", outJSON)
	return 0
}
