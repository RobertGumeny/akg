package akg

import (
	"os"
	"path/filepath"
	"testing"
)

// TestGenBehaviorParityGraph regenerates the shared cross-SDK behavior fixture
// testdata/behavior/parity-graph.akg. It is the reproducible replacement for the
// original throwaway script (the binary was hand-built once and committed without
// a generator). Run with:
//
//	GEN_BEHAVIOR_GRAPH=1 go test -run TestGenBehaviorParityGraph .
//
// The graph reproduces the four person/task nodes and four edges described by
// parity-spec.json verbatim, then adds the tag-index key-collision regression
// pair: two nodes sharing the id "preflop__vpip" across the types "counter" and
// "tendency", both tagged "preflop" (the EPIC-18-003 repro). Under the major-1
// tag key (t:{tag}:{id}) the pair collapsed to one key and broke compaction;
// under the major-2 type-qualified key (t:{tag}:{type}:{id}) they stay distinct,
// so this file materializes and reads clean. Because all first-party SDKs write
// byte-identically (the uniform-write-path rule), an akg-go-generated file is
// exactly what akg-ts would produce, so it is a legitimate shared fixture.
//
// After regenerating, run the behavior_parity tests in BOTH SDKs to confirm they
// still agree, and keep parity-spec.json in sync with any assertion changes.
func TestGenBehaviorParityGraph(t *testing.T) {
	if os.Getenv("GEN_BEHAVIOR_GRAPH") == "" {
		t.Skip("GEN_BEHAVIOR_GRAPH not set; skipping behavior fixture generation")
	}
	path := filepath.Join("..", "..", "testdata", "behavior", "parity-graph.akg")
	if err := genBehaviorParityGraph(path); err != nil {
		t.Fatalf("generate parity-graph.akg: %v", err)
	}
}

func genBehaviorParityGraph(path string) error {
	const ts = timestampMicros(1_000_000)
	s := newStoreState()
	s.now = func() timestampMicros { return ts }

	// ---- Core graph (must match parity-spec.json verbatim) ----
	nodes := []coreNode{
		{Type: "person", Title: "Alice", Tags: []string{"active", "researcher"}, Meta: map[string]any{"role": "lead"}},
		{Type: "person", Title: "Bob", Tags: []string{"active"}, Meta: map[string]any{"role": "engineer"}},
		{Type: "person", Title: "Carol", Tags: []string{"inactive"}, Meta: map[string]any{"role": "engineer"}},
		{Type: "task", Title: "First Task", Tags: []string{"open"}, Meta: map[string]any{"priority": "high", "tags": []any{"urgent", "p1"}}},
	}
	ids := []nodeID{"alice", "bob", "carol", "t1"}
	for i, n := range nodes {
		if _, err := s.putNode(ids[i], n); err != nil {
			return err
		}
	}

	edges := []coreEdge{
		{FromType: "person", FromNode: "alice", Relation: "assigned", ToType: "task", ToNode: "t1", Strength: 0.5, Meta: map[string]any{"source": "manual"}},
		{FromType: "person", FromNode: "alice", Relation: "knows", ToType: "person", ToNode: "bob", Strength: 0.7, Meta: map[string]any{"source": "inferred"}},
		{FromType: "person", FromNode: "alice", Relation: "manages", ToType: "person", ToNode: "bob", Strength: 0.9},
		{FromType: "person", FromNode: "bob", Relation: "knows", ToType: "person", ToNode: "alice", Strength: 0},
	}
	for _, e := range edges {
		if _, err := s.putEdge(e); err != nil {
			return err
		}
	}

	// ---- Tag-index key-collision regression pair (EPIC-18-003) ----
	// Same id "preflop__vpip" across two types, both tagged "preflop".
	if _, err := s.putNode("preflop__vpip", coreNode{Type: "counter", Title: "VPIP counter", Tags: []string{"preflop"}}); err != nil {
		return err
	}
	if _, err := s.putNode("preflop__vpip", coreNode{Type: "tendency", Title: "VPIP tendency", Tags: []string{"preflop"}}); err != nil {
		return err
	}

	return writeCompactedStore(path, s)
}
