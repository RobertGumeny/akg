package akg

// Cross-SDK behavioral parity tests.
// These tests load testdata/behavior/parity-graph.akg and assert against
// testdata/behavior/parity-spec.json — the same spec the TypeScript SDK uses.
// If both SDKs pass, they agree on the full read-side behavioral contract.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBehaviorParity(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "behavior")
	specBytes, err := os.ReadFile(filepath.Join(root, "parity-spec.json"))
	if err != nil {
		t.Fatalf("read parity-spec.json: %v", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		t.Fatalf("parse parity-spec.json: %v", err)
	}

	s, err := Open(filepath.Join(root, "parity-graph.akg"))
	if err != nil {
		t.Fatalf("Open parity-graph.akg: %v", err)
	}
	defer s.Close()

	a := spec["assertions"].(map[string]any)

	// Node and edge counts.
	nodes, _ := s.ListNodes("")
	assertInt(t, "node_count", len(nodes), int(spec["node_count"].(float64)))

	edges, _ := s.ListEdges(EdgeFilter{})
	assertInt(t, "edge_count", len(edges), int(spec["edge_count"].(float64)))

	// ListEdges: empty filter.
	assertInt(t, "list_edges_all_count", len(edges), int(a["list_edges_all_count"].(float64)))

	// ListEdges: relation filters.
	for _, rel := range []string{"knows", "assigned", "manages"} {
		got, _ := s.ListEdges(EdgeFilter{Relation: rel})
		key := "list_edges_" + rel + "_count"
		assertInt(t, key, len(got), int(a[key].(float64)))
	}

	// ListEdges: metadata filters.
	for _, src := range []string{"inferred", "manual"} {
		got, _ := s.ListEdges(EdgeFilter{Meta: map[string]any{"source": src}})
		key := "list_edges_meta_source_" + src + "_count"
		assertInt(t, key, len(got), int(a[key].(float64)))
	}

	// Edge strength semantics.
	checkEdgeStrength(t, s, a, "bob", "knows", "alice", "bob_knows_alice_strength")
	checkEdgeStrength(t, s, a, "alice", "assigned", "t1", "alice_assigned_t1_strength")
	checkEdgeStrength(t, s, a, "alice", "knows", "bob", "alice_knows_bob_strength")

	// ListNodesFiltered.
	checkFilteredIDs(t, s, NodeFilter{Type: "person", Tag: "active"}, a["list_nodes_type_person_tag_active_ids"])
	checkFilteredIDs(t, s, NodeFilter{Type: "person", Tag: "researcher"}, a["list_nodes_type_person_tag_researcher_ids"])
	checkFilteredIDs(t, s, NodeFilter{Meta: map[string]any{"role": "engineer"}}, a["list_nodes_meta_role_engineer_ids"])
	checkFilteredIDs(t, s, NodeFilter{Meta: map[string]any{"tags": []any{"urgent", "p1"}}}, a["list_nodes_meta_tags_array_ids"])

	// Snapshot.
	snap, _ := s.Snapshot()
	assertInt(t, "snapshot_node_count", len(snap.Nodes), int(a["snapshot_node_count"].(float64)))
	assertInt(t, "snapshot_edge_count", len(snap.Edges), int(a["snapshot_edge_count"].(float64)))

	// GetNodes: positional, preserves order, nil for missing.
	inputRaw := a["get_nodes_input"].([]any)
	refs := make([]NodeRef, len(inputRaw))
	for i, r := range inputRaw {
		m := r.(map[string]any)
		refs[i] = NodeRef{Type: m["type"].(string), ID: m["id"].(string)}
	}
	result, err := s.GetNodes(refs)
	if err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	titlesRaw := a["get_nodes_titles"].([]any)
	for i, expected := range titlesRaw {
		if expected == nil {
			if result[i] != nil {
				t.Errorf("GetNodes[%d]: expected nil, got %v", i, result[i])
			}
		} else {
			if result[i] == nil {
				t.Errorf("GetNodes[%d]: expected %v, got nil", i, expected)
			} else if result[i].Title != expected.(string) {
				t.Errorf("GetNodes[%d]: expected title %q, got %q", i, expected, result[i].Title)
			}
		}
	}
}

func assertInt(t *testing.T, name string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %d, want %d", name, got, want)
	}
}

func checkEdgeStrength(t *testing.T, s *Store, a map[string]any, fromID, rel, toID, key string) {
	t.Helper()
	edges, err := s.OutboundEdges(NodeRef{Type: "person", ID: fromID}, rel)
	if err != nil {
		t.Fatalf("OutboundEdges(%s, %s): %v", fromID, rel, err)
	}
	// find the specific edge
	var strength float64
	found := false
	for _, e := range edges {
		if e.To.ID == toID {
			strength = e.Strength
			found = true
			break
		}
	}
	// also check task edges
	if !found {
		edges2, _ := s.OutboundEdges(NodeRef{Type: "person", ID: fromID}, rel)
		for _, e := range edges2 {
			if e.To.ID == toID {
				strength = e.Strength
				found = true
				break
			}
		}
	}
	if !found {
		t.Errorf("%s: edge %s-[%s]->%s not found", key, fromID, rel, toID)
		return
	}
	want := a[key].(float64)
	if strength != want {
		t.Errorf("%s: got strength %v, want %v", key, strength, want)
	}
}

func checkFilteredIDs(t *testing.T, s *Store, filter NodeFilter, expectedRaw any) {
	t.Helper()
	nodes, err := s.ListNodesFiltered(filter)
	if err != nil {
		t.Fatalf("ListNodesFiltered: %v", err)
	}
	expectedSlice := expectedRaw.([]any)
	if len(nodes) != len(expectedSlice) {
		ids := make([]string, len(nodes))
		for i, n := range nodes {
			ids[i] = n.ID
		}
		t.Errorf("filter %+v: got ids %v, want %v", filter, ids, expectedSlice)
		return
	}
	for i, n := range nodes {
		if n.ID != expectedSlice[i].(string) {
			t.Errorf("filter %+v pos %d: got id %q, want %q", filter, i, n.ID, expectedSlice[i])
		}
	}
}
