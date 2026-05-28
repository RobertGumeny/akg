package main

import (
	"encoding/json"
	"fmt"
	"log"
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

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	docsDir := filepath.Join(cwd, "docs")
	manifestPath := filepath.Join(docsDir, "manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		log.Fatalf("reading manifest: %v", err)
	}

	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		log.Fatalf("parsing manifest: %v", err)
	}

	generatedAt, err := time.Parse(time.RFC3339, m.Meta.GeneratedAt)
	if err != nil {
		log.Fatalf("parsing generated_at: %v", err)
	}
	fixedMicros := uint64(generatedAt.UnixMicro())
	akg.SetTestNow(fixedMicros)
	defer akg.SetTestNow(0)

	outAkg := filepath.Join(docsDir, "akg-go-docs.akg")
	outJSON := filepath.Join(docsDir, "akg-go-docs.json")

	if err := os.Remove(outAkg); err != nil && !os.IsNotExist(err) {
		log.Fatalf("removing old akg file: %v", err)
	}

	store, err := akg.Open(outAkg)
	if err != nil {
		log.Fatalf("opening store: %v", err)
	}

	refs := make(map[string]akg.NodeRef)

	for _, node := range m.Nodes {
		colonIdx := strings.Index(node.ID, ":")
		if colonIdx < 0 {
			log.Fatalf("invalid node id (no colon): %s", node.ID)
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
			log.Fatalf("PutNode %s: %v", node.ID, err)
		}
		refs[node.ID] = ref
	}

	for _, edge := range m.Edges {
		fromRef, ok := refs[edge.From]
		if !ok {
			log.Fatalf("edge references unknown node: %s", edge.From)
		}
		toRef, ok := refs[edge.To]
		if !ok {
			log.Fatalf("edge references unknown node: %s", edge.To)
		}
		if err := store.PutEdge(fromRef, edge.Relation, toRef, akg.EdgeFields{}); err != nil {
			log.Fatalf("PutEdge %s -> %s: %v", edge.From, edge.To, err)
		}
	}

	snap, err := store.Snapshot()
	if err != nil {
		log.Fatalf("snapshot: %v", err)
	}

	if err := store.Compact(); err != nil {
		log.Fatalf("compact: %v", err)
	}
	if err := store.Close(); err != nil {
		log.Fatalf("close: %v", err)
	}

	jsonData, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		log.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(outJSON, append(jsonData, '\n'), 0o644); err != nil {
		log.Fatalf("writing json: %v", err)
	}

	fmt.Printf("Generated %d nodes, %d edges\n", len(m.Nodes), len(m.Edges))
	fmt.Printf("  %s\n", outAkg)
	fmt.Printf("  %s\n", outJSON)
}
