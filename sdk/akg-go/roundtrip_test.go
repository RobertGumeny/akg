package akg

// Cross-SDK write-path round-trip: open testdata/roundtrip/ts-written.akg — a
// file written by the akg-ts SDK — and assert the Go SDK sees identical graph
// state. This proves a Go process can read a TS-written file. Equality is
// asserted semantically (counts, titles, relations, strengths), not by byte
// identity, because TS and Go stamp timestamps independently (PRD Open Q1).
//
// Regenerate the fixture with: cd sdk/akg-ts && npm run generate:roundtrip

import (
	"path/filepath"
	"testing"
)

func tsWrittenFixturePath() string {
	return filepath.Join("..", "..", "testdata", "roundtrip", "ts-written.akg")
}

func TestOpenTSWrittenFile(t *testing.T) {
	store, err := Open(tsWrittenFixturePath())
	if err != nil {
		t.Fatalf("Open(ts-written.akg): %v", err)
	}
	defer store.Close()

	nodes, err := store.ListNodes("")
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("node count = %d, want 3", len(nodes))
	}

	edges, err := store.ListEdges(EdgeFilter{})
	if err != nil {
		t.Fatalf("ListEdges: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("edge count = %d, want 2", len(edges))
	}

	// Known node: person/alice with its TS-written title and tags.
	alice, err := store.GetNode("person", "alice")
	if err != nil {
		t.Fatalf("GetNode(person/alice): %v", err)
	}
	if alice == nil {
		t.Fatal("person/alice not found")
	}
	if alice.Title != "Alice Researcher" {
		t.Fatalf("alice.Title = %q, want %q", alice.Title, "Alice Researcher")
	}

	// Known edge: alice -authored-> topic/akg with strength 0.9 and confidence 0.95.
	authored, err := store.OutboundEdges(NodeRef{Type: "person", ID: "alice"}, "authored")
	if err != nil {
		t.Fatalf("OutboundEdges(alice, authored): %v", err)
	}
	if len(authored) != 1 {
		t.Fatalf("alice authored edge count = %d, want 1", len(authored))
	}
	e := authored[0]
	if e.Relation != "authored" {
		t.Fatalf("edge.Relation = %q, want %q", e.Relation, "authored")
	}
	if e.To.Type != "topic" || e.To.ID != "akg" {
		t.Fatalf("edge.To = %+v, want topic/akg", e.To)
	}
	if e.Strength != 0.9 {
		t.Fatalf("edge.Strength = %v, want 0.9", e.Strength)
	}
	if e.Confidence == nil || *e.Confidence != 0.95 {
		t.Fatalf("edge.Confidence = %v, want 0.95", e.Confidence)
	}

	// The edge written with confidence: null must survive as a nil confidence,
	// not the 0.5 default.
	reviews, err := store.OutboundEdges(NodeRef{Type: "person", ID: "bob"}, "reviews")
	if err != nil {
		t.Fatalf("OutboundEdges(bob, reviews): %v", err)
	}
	if len(reviews) != 1 {
		t.Fatalf("bob reviews edge count = %d, want 1", len(reviews))
	}
	if reviews[0].Confidence != nil {
		t.Fatalf("bob reviews confidence = %v, want nil", *reviews[0].Confidence)
	}
	if reviews[0].Strength != 0.42 {
		t.Fatalf("bob reviews strength = %v, want 0.42", reviews[0].Strength)
	}
}
