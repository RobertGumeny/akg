package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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
	SourcePath  string `json:"source_path"`
	GeneratedAt string `json:"generated_at"`
}

// changelogVersionRE matches a released CHANGELOG heading like `## v0.1.4`,
// capturing the bare version (`0.1.4`). `## Unreleased` does not match, so the
// most recent *released* version is the first match scanning top-down.
var changelogVersionRE = regexp.MustCompile(`^##\s+v(\d+\.\d+\.\d+)\s*$`)

// latestChangelogVersion returns the most recent released version recorded in a
// CHANGELOG.md (the first `## vX.Y.Z` heading from the top, skipping
// `## Unreleased`). It is the single source of truth for the git-tag-versioned
// Go SDK's doc-graph version stamp.
func latestChangelogVersion(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if m := changelogVersionRE.FindStringSubmatch(scanner.Text()); m != nil {
			return m[1], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("no released version heading (## vX.Y.Z) found in %s", path)
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

	// Version is sourced from the CHANGELOG's latest released heading — the Go SDK
	// has no version field (it is git-tag versioned), and the CHANGELOG is the
	// in-repo artifact prep-for-release stamps on release. This avoids a
	// hand-edited manifest constant that drifts from the shipped version.
	version, err := latestChangelogVersion(filepath.Join(cwd, "CHANGELOG.md"))
	if err != nil {
		fmt.Fprintf(stderr, "resolving version: %v\n", err)
		return 1
	}

	// generated_at is build-time-stampable (a release can inject a real timestamp
	// via AKG_DOCS_GENERATED_AT) but defaults to the committed manifest value so
	// CI's freshness `git diff` stays deterministic and never churns on a clock.
	generatedAtStr := m.Meta.GeneratedAt
	if env := os.Getenv("AKG_DOCS_GENERATED_AT"); env != "" {
		generatedAtStr = env
	}
	generatedAt, err := time.Parse(time.RFC3339, generatedAtStr)
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
				"version":      version,
				"generated_at": generatedAtStr,
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
