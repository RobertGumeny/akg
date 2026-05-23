package akg

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCreatesEmptyStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat: %v", err)
	}
}

func TestCommitCloseReopenPreservesCommittedNodeAndWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roundtrip.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err = st.PutNode("note", "n1", NodeFields{Title: "hello"}, []string{"topic"})
	if err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen Open: %v", err)
	}
	node, err := reopened.GetNode("note", "n1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node == nil {
		t.Fatal("committed node missing after reopen")
	}
	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	c, err := decodeContainer(file)
	if err != nil {
		t.Fatalf("decodeContainer: %v", err)
	}
	if len(c.WAL) == 0 {
		t.Fatal("expected committed WAL to remain in file after close")
	}
}

func TestPutNodeGetNodeAndUpdateCurrentState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node-current.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	st.state.now = func() timestampMicros { return 100 }

	ref, err := st.PutNode("note", "n1", NodeFields{
		Title: "hello",
		Body:  "draft",
		Meta:  map[string]any{"color": "blue"},
	}, []string{"topic", "draft"})
	if err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	if ref != (NodeRef{Type: "note", ID: "n1"}) {
		t.Fatalf("unexpected NodeRef: %#v", ref)
	}

	node, err := st.GetNode("note", "n1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node == nil {
		t.Fatal("GetNode returned nil")
	}
	if node.Title != "hello" || node.Body != "draft" {
		t.Fatalf("unexpected node contents: %#v", node)
	}
	if node.CreatedAt != 100 || node.UpdatedAt != 100 || node.Version != 1 {
		t.Fatalf("unexpected node lifecycle fields: %#v", node)
	}

	st.state.now = func() timestampMicros { return 250 }
	_, err = st.PutNode("note", "n1", NodeFields{Title: "hello v2"}, []string{"topic"})
	if err != nil {
		t.Fatalf("second PutNode: %v", err)
	}
	updated, err := st.GetNode("note", "n1")
	if err != nil {
		t.Fatalf("GetNode after update: %v", err)
	}
	if updated == nil {
		t.Fatal("updated node missing")
	}
	if updated.Title != "hello v2" || updated.Body != "" {
		t.Fatalf("unexpected updated node contents: %#v", updated)
	}
	if updated.CreatedAt != 100 || updated.UpdatedAt != 250 || updated.Version != 2 {
		t.Fatalf("unexpected updated lifecycle fields: %#v", updated)
	}
}

func TestListNodesByTagCurrentStateAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tag-list.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	st.state.now = func() timestampMicros { return 10 }
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "one"}, []string{"topic"}); err != nil {
		t.Fatalf("PutNode n1: %v", err)
	}
	st.state.now = func() timestampMicros { return 11 }
	if _, err := st.PutNode("note", "n2", NodeFields{Title: "two"}, []string{"topic", "other"}); err != nil {
		t.Fatalf("PutNode n2: %v", err)
	}
	if _, err := st.PutNode("note", "n3", NodeFields{Title: "three"}, []string{"other"}); err != nil {
		t.Fatalf("PutNode n3: %v", err)
	}

	nodes, err := st.ListNodesByTag("topic")
	if err != nil {
		t.Fatalf("ListNodesByTag current: %v", err)
	}
	if len(nodes) != 2 || nodes[0].ID != "n1" || nodes[1].ID != "n2" {
		t.Fatalf("unexpected current tag listing: %#v", nodes)
	}

	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen Open: %v", err)
	}
	nodes, err = reopened.ListNodesByTag("topic")
	if err != nil {
		t.Fatalf("ListNodesByTag reopened: %v", err)
	}
	if len(nodes) != 2 || nodes[0].ID != "n1" || nodes[1].ID != "n2" {
		t.Fatalf("unexpected reopened tag listing: %#v", nodes)
	}
}

func TestPutEdgeOutboundInboundAndRelationFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "edge-current.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, tc := range []struct {
		typeName string
		id       string
		title    string
	}{
		{typeName: "note", id: "n1", title: "one"},
		{typeName: "note", id: "n2", title: "two"},
		{typeName: "note", id: "n3", title: "three"},
	} {
		if _, err := st.PutNode(tc.typeName, tc.id, NodeFields{Title: tc.title}, nil); err != nil {
			t.Fatalf("PutNode %s: %v", tc.id, err)
		}
	}

	st.state.now = func() timestampMicros { return 100 }
	confidence := 0.9
	if err := st.PutEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}, EdgeFields{
		Strength:   0.75,
		Confidence: &confidence,
		Meta:       map[string]any{"source": "test"},
	}); err != nil {
		t.Fatalf("PutEdge links_to n1->n2: %v", err)
	}
	st.state.now = func() timestampMicros { return 200 }
	if err := st.PutEdge(NodeRef{Type: "note", ID: "n1"}, "mentions", NodeRef{Type: "note", ID: "n3"}, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge mentions n1->n3: %v", err)
	}
	st.state.now = func() timestampMicros { return 300 }
	if err := st.PutEdge(NodeRef{Type: "note", ID: "n3"}, "links_to", NodeRef{Type: "note", ID: "n2"}, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge links_to n3->n2: %v", err)
	}

	outbound, err := st.OutboundEdges(NodeRef{Type: "note", ID: "n1"}, "")
	if err != nil {
		t.Fatalf("OutboundEdges all: %v", err)
	}
	if len(outbound) != 2 || outbound[0].Relation != "links_to" || outbound[0].To.ID != "n2" || outbound[1].Relation != "mentions" || outbound[1].To.ID != "n3" {
		t.Fatalf("unexpected outbound edges: %#v", outbound)
	}
	if outbound[0].Strength != 0.75 || outbound[0].Confidence == nil || *outbound[0].Confidence != 0.9 || outbound[0].CreatedAt != 100 || outbound[0].UpdatedAt != 100 || outbound[0].Version != 1 {
		t.Fatalf("unexpected outbound edge fields: %#v", outbound[0])
	}

	filtered, err := st.OutboundEdges(NodeRef{Type: "note", ID: "n1"}, "mentions")
	if err != nil {
		t.Fatalf("OutboundEdges filtered: %v", err)
	}
	if len(filtered) != 1 || filtered[0].To.ID != "n3" {
		t.Fatalf("unexpected filtered outbound edges: %#v", filtered)
	}

	inbound, err := st.InboundEdges(NodeRef{Type: "note", ID: "n2"}, "links_to")
	if err != nil {
		t.Fatalf("InboundEdges filtered: %v", err)
	}
	if len(inbound) != 2 || inbound[0].From.ID != "n1" || inbound[1].From.ID != "n3" {
		t.Fatalf("unexpected inbound edges: %#v", inbound)
	}
}

func TestDeleteNodeBasicSemantics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "delete-node.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "hello"}, nil); err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	if err := st.DeleteNode("note", "n1"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	node, err := st.GetNode("note", "n1")
	if err != nil {
		t.Fatalf("GetNode after delete: %v", err)
	}
	if node != nil {
		t.Fatal("node should be absent after DeleteNode")
	}

	// ErrNotFound when node does not exist
	if err := st.DeleteNode("note", "n1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second DeleteNode, got %v", err)
	}
	if err := st.DeleteNode("note", "nonexistent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for nonexistent node, got %v", err)
	}
}

func TestDeleteNodeBlockedByLiveEdges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "delete-node-edges.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "one"}, nil); err != nil {
		t.Fatalf("PutNode n1: %v", err)
	}
	if _, err := st.PutNode("note", "n2", NodeFields{Title: "two"}, nil); err != nil {
		t.Fatalf("PutNode n2: %v", err)
	}
	if err := st.PutEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}

	if err := st.DeleteNode("note", "n1"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for node with outbound edge, got %v", err)
	}
	if err := st.DeleteNode("note", "n2"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for node with inbound edge, got %v", err)
	}

	// After deleting the edge, node deletion should succeed.
	if err := st.DeleteEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}); err != nil {
		t.Fatalf("DeleteEdge: %v", err)
	}
	if err := st.DeleteNode("note", "n1"); err != nil {
		t.Fatalf("DeleteNode after edge removal: %v", err)
	}
}

func TestDeleteNodeRoundTripReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "delete-node-reopen.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "hello"}, nil); err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	st2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	if err := st2.DeleteNode("note", "n1"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	if err := st2.Commit(); err != nil {
		t.Fatalf("second Commit: %v", err)
	}
	if err := st2.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	st3, err := Open(path)
	if err != nil {
		t.Fatalf("third Open: %v", err)
	}
	node, err := st3.GetNode("note", "n1")
	if err != nil {
		t.Fatalf("GetNode after reopen: %v", err)
	}
	if node != nil {
		t.Fatal("deleted node should be absent after reopen")
	}
}

func TestDeleteEdgeBasicSemantics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "delete-edge.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "one"}, nil); err != nil {
		t.Fatalf("PutNode n1: %v", err)
	}
	if _, err := st.PutNode("note", "n2", NodeFields{Title: "two"}, nil); err != nil {
		t.Fatalf("PutNode n2: %v", err)
	}
	if err := st.PutEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	if err := st.DeleteEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}); err != nil {
		t.Fatalf("DeleteEdge: %v", err)
	}
	out, err := st.OutboundEdges(NodeRef{Type: "note", ID: "n1"}, "")
	if err != nil {
		t.Fatalf("OutboundEdges: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 outbound edges after delete, got %d", len(out))
	}

	// ErrNotFound when edge does not exist
	if err := st.DeleteEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on second DeleteEdge, got %v", err)
	}
}

func TestDeleteEdgeRoundTripReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "delete-edge-reopen.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "one"}, nil); err != nil {
		t.Fatalf("PutNode n1: %v", err)
	}
	if _, err := st.PutNode("note", "n2", NodeFields{Title: "two"}, nil); err != nil {
		t.Fatalf("PutNode n2: %v", err)
	}
	if err := st.PutEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	st2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	if err := st2.DeleteEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}); err != nil {
		t.Fatalf("DeleteEdge: %v", err)
	}
	if err := st2.Commit(); err != nil {
		t.Fatalf("second Commit: %v", err)
	}
	if err := st2.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	st3, err := Open(path)
	if err != nil {
		t.Fatalf("third Open: %v", err)
	}
	out, err := st3.OutboundEdges(NodeRef{Type: "note", ID: "n1"}, "")
	if err != nil {
		t.Fatalf("OutboundEdges after reopen: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("deleted edge should be absent after reopen, got %d edges", len(out))
	}
	// Both nodes should still exist
	for _, id := range []string{"n1", "n2"} {
		node, err := st3.GetNode("note", id)
		if err != nil {
			t.Fatalf("GetNode %s: %v", id, err)
		}
		if node == nil {
			t.Fatalf("node %s should still exist after edge deletion", id)
		}
	}
}

func TestOutboundEdgesDoesNotReturnEdgesFromDifferentNodeType(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cross-type.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Two nodes sharing the same ID string but different types.
	if _, err := st.PutNode("note", "shared", NodeFields{Title: "note node"}, nil); err != nil {
		t.Fatalf("PutNode note/shared: %v", err)
	}
	if _, err := st.PutNode("concept", "shared", NodeFields{Title: "concept node"}, nil); err != nil {
		t.Fatalf("PutNode concept/shared: %v", err)
	}
	if _, err := st.PutNode("note", "target", NodeFields{Title: "target"}, nil); err != nil {
		t.Fatalf("PutNode note/target: %v", err)
	}

	// Connect note/shared -> links_to -> note/target.
	if err := st.PutEdge(NodeRef{Type: "note", ID: "shared"}, "links_to", NodeRef{Type: "note", ID: "target"}, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}

	// OutboundEdges on concept/shared must return empty — different type, same ID.
	out, err := st.OutboundEdges(NodeRef{Type: "concept", ID: "shared"}, "")
	if err != nil {
		t.Fatalf("OutboundEdges concept/shared: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected 0 outbound edges for concept/shared, got %d (cross-type contamination bug)", len(out))
	}

	// OutboundEdges on note/shared must return the edge.
	out, err = st.OutboundEdges(NodeRef{Type: "note", ID: "shared"}, "")
	if err != nil {
		t.Fatalf("OutboundEdges note/shared: %v", err)
	}
	if len(out) != 1 || out[0].To.ID != "target" {
		t.Fatalf("expected 1 outbound edge from note/shared, got %#v", out)
	}

	// InboundEdges on note/target must return the edge from note/shared (not concept/shared).
	in, err := st.InboundEdges(NodeRef{Type: "note", ID: "target"}, "")
	if err != nil {
		t.Fatalf("InboundEdges note/target: %v", err)
	}
	if len(in) != 1 || in[0].From != (NodeRef{Type: "note", ID: "shared"}) {
		t.Fatalf("expected 1 inbound edge from note/shared, got %#v", in)
	}
}

func TestEdgeRoundTripAcrossCloseReopenAndUpdate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "edge-reopen.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, tc := range []struct {
		typeName string
		id       string
		title    string
	}{
		{typeName: "note", id: "n1", title: "one"},
		{typeName: "note", id: "n2", title: "two"},
	} {
		if _, err := st.PutNode(tc.typeName, tc.id, NodeFields{Title: tc.title}, nil); err != nil {
			t.Fatalf("PutNode %s: %v", tc.id, err)
		}
	}

	st.state.now = func() timestampMicros { return 10 }
	if err := st.PutEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}, EdgeFields{Strength: 0.4}); err != nil {
		t.Fatalf("PutEdge initial: %v", err)
	}
	st.state.now = func() timestampMicros { return 25 }
	if err := st.PutEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}, EdgeFields{Strength: 0.8}); err != nil {
		t.Fatalf("PutEdge update: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen Open: %v", err)
	}
	outbound, err := reopened.OutboundEdges(NodeRef{Type: "note", ID: "n1"}, "links_to")
	if err != nil {
		t.Fatalf("OutboundEdges reopened: %v", err)
	}
	if len(outbound) != 1 {
		t.Fatalf("unexpected reopened outbound count: %#v", outbound)
	}
	if outbound[0].Strength != 0.8 || outbound[0].CreatedAt != 10 || outbound[0].UpdatedAt != 25 || outbound[0].Version != 2 {
		t.Fatalf("unexpected reopened edge fields: %#v", outbound[0])
	}

	inbound, err := reopened.InboundEdges(NodeRef{Type: "note", ID: "n2"}, "")
	if err != nil {
		t.Fatalf("InboundEdges reopened: %v", err)
	}
	if len(inbound) != 1 || inbound[0].From != (NodeRef{Type: "note", ID: "n1"}) {
		t.Fatalf("unexpected reopened inbound edges: %#v", inbound)
	}
}

func TestListNodes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "listnodes.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err = st.PutNode("note", "n1", NodeFields{Title: "Note 1"}, nil)
	if err != nil {
		t.Fatalf("PutNode note n1: %v", err)
	}
	_, err = st.PutNode("note", "n2", NodeFields{Title: "Note 2"}, nil)
	if err != nil {
		t.Fatalf("PutNode note n2: %v", err)
	}
	_, err = st.PutNode("card", "c1", NodeFields{Title: "Card 1"}, nil)
	if err != nil {
		t.Fatalf("PutNode card c1: %v", err)
	}

	// all nodes (empty typeName)
	all, err := st.ListNodes("")
	if err != nil {
		t.Fatalf("ListNodes empty: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 nodes, got %d: %#v", len(all), all)
	}

	// type-filtered
	notes, err := st.ListNodes("note")
	if err != nil {
		t.Fatalf("ListNodes note: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("expected 2 note nodes, got %d", len(notes))
	}
	for _, n := range notes {
		if n.Type != "note" {
			t.Fatalf("unexpected type in filtered result: %s", n.Type)
		}
	}

	// unknown type returns empty, not error
	unknown, err := st.ListNodes("ghost")
	if err != nil {
		t.Fatalf("ListNodes unknown: %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("expected empty for unknown type, got %d nodes", len(unknown))
	}

	// invalid typeName (uppercase rejected by tightened validateComponent) returns error
	_, err = st.ListNodes("BadType")
	if err == nil {
		t.Fatal("expected error for invalid typeName")
	}

	// results are sorted consistently (same order as a second call)
	all2, err := st.ListNodes("")
	if err != nil {
		t.Fatalf("ListNodes second call: %v", err)
	}
	for i := range all {
		if all[i].ID != all2[i].ID || all[i].Type != all2[i].Type {
			t.Fatalf("sort not stable between calls: %v vs %v", all[i], all2[i])
		}
	}
}

func TestCloseCommitsPendingMutations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "close-commits.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err = st.PutNode("note", "n1", NodeFields{Title: "hello"}, nil)
	if err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	// Close without an explicit Commit — Close must commit the pending mutation.
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	node, err := reopened.GetNode("note", "n1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node == nil {
		t.Fatal("mutation written after last Commit not durable after Close + Open")
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("Close reopened: %v", err)
	}
}

func TestCommitIsNoOpWhenNothingPending(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commit-noop.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	_, err = st.PutNode("note", "n1", NodeFields{Title: "hello"}, nil)
	if err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("first Commit: %v", err)
	}
	// Second Commit with nothing pending — must return nil without corrupting state.
	if err := st.Commit(); err != nil {
		t.Fatalf("second Commit (no-op): %v", err)
	}
	node, err := st.GetNode("note", "n1")
	if err != nil {
		t.Fatalf("GetNode after double Commit: %v", err)
	}
	if node == nil {
		t.Fatal("node missing after double Commit — state corrupted")
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestCloseOnAlreadyClosedStoreIsNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "close-twice.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("second Close on already-closed store: %v", err)
	}
}

func TestEmptyStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen Open: %v", err)
	}
	defer reopened.Close()

	nodes, err := reopened.ListNodes("")
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected 0 nodes after reopen of empty store, got %d", len(nodes))
	}
}

func TestValidationErrorsErrInvalidInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "validation.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// Invalid type name (uppercase) → ErrInvalidInput
	if _, err := st.PutNode("BadType", "id", NodeFields{Title: "t"}, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid type name: expected ErrInvalidInput, got %v", err)
	}
	// Invalid type name (contains colon) → ErrInvalidInput
	if _, err := st.PutNode("bad:type", "id", NodeFields{Title: "t"}, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("colon in type name: expected ErrInvalidInput, got %v", err)
	}

	// Set up two valid nodes to test invalid relation name in PutEdge.
	n1, err := st.PutNode("note", "n1", NodeFields{Title: "one"}, nil)
	if err != nil {
		t.Fatalf("PutNode n1: %v", err)
	}
	n2, err := st.PutNode("note", "n2", NodeFields{Title: "two"}, nil)
	if err != nil {
		t.Fatalf("PutNode n2: %v", err)
	}
	// Invalid relation name (uppercase) → ErrInvalidInput
	if err := st.PutEdge(n1, "BadRelation", n2, EdgeFields{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid relation name: expected ErrInvalidInput, got %v", err)
	}

	// Invalid tag (uppercase) → ErrInvalidInput
	if _, err := st.PutNode("note", "n3", NodeFields{Title: "three"}, []string{"BadTag"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid tag: expected ErrInvalidInput, got %v", err)
	}
}

func TestMissingRequiredFieldTitle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-title.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	if _, err := st.PutNode("note", "n1", NodeFields{Title: ""}, nil); !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("empty title: expected ErrMissingRequiredField, got %v", err)
	}
}

func TestPutEdgeNonexistentNodes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "edge-nonexistent.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	existing, err := st.PutNode("note", "n1", NodeFields{Title: "one"}, nil)
	if err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	ghost := NodeRef{Type: "note", ID: "ghost"}

	// Source node does not exist
	if err := st.PutEdge(ghost, "links_to", existing, EdgeFields{}); err == nil {
		t.Fatal("expected error for nonexistent source node, got nil")
	}
	// Target node does not exist
	if err := st.PutEdge(existing, "links_to", ghost, EdgeFields{}); err == nil {
		t.Fatal("expected error for nonexistent target node, got nil")
	}
}

func TestPutNodeAutoID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auto-id.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	ref, err := st.PutNode("note", "", NodeFields{Title: "auto"}, nil)
	if err != nil {
		t.Fatalf("PutNode with empty id: %v", err)
	}
	if ref.ID == "" {
		t.Fatal("expected non-empty generated ID, got empty string")
	}
}

func TestNodeIDConstraints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "id-constraints.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// Node ID containing a colon
	if _, err := st.PutNode("note", "bad:id", NodeFields{Title: "t"}, nil); err == nil {
		t.Fatal("expected error for node ID with colon, got nil")
	}

	// Node ID longer than 64 characters
	longID := strings.Repeat("a", 65)
	if _, err := st.PutNode("note", longID, NodeFields{Title: "t"}, nil); err == nil {
		t.Fatal("expected error for node ID longer than 64 chars, got nil")
	}
}

func TestOpenMalformedNodePayloadReturnsErrMissingRequiredField(t *testing.T) {
	// Build a node data payload that is structurally valid msgpack but missing "title".
	// msgpack fixmap with 1 key: {type: "note"}
	badNodePayload := []byte{
		0x81,                         // fixmap, 1 pair
		0xa4, 't', 'y', 'p', 'e',    // key "type"
		0xa4, 'n', 'o', 't', 'e',    // value "note"
	}

	nodeKey, err := buildNodeKey("note", "n1")
	if err != nil {
		t.Fatalf("buildNodeKey: %v", err)
	}
	entries, err := encodeDataEntries([]dataEntry{{Key: nodeKey, Value: badNodePayload}})
	if err != nil {
		t.Fatalf("encodeDataEntries: %v", err)
	}
	file, err := encodeContainer(container{
		Data:  entries,
		Bloom: encodeBloom([][]byte{nodeKey}),
	})
	if err != nil {
		t.Fatalf("encodeContainer: %v", err)
	}

	path := filepath.Join(t.TempDir(), "malformed.akg")
	if err := os.WriteFile(path, file, 0o666); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err = Open(path)
	if err == nil {
		t.Fatal("expected error opening malformed file, got nil")
	}
	if !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("expected errors.Is(err, ErrMissingRequiredField) = true, got error: %v", err)
	}
}

func TestTagArrayConstraints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tag-constraints.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// More than 32 tags
	tooMany := make([]string, 33)
	for i := range tooMany {
		tooMany[i] = "tag" + string(rune('a'+i%26))
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "t"}, tooMany); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("too many tags: expected ErrInvalidInput, got %v", err)
	}

	// Duplicate tags
	dups := []string{"alpha", "beta", "alpha"}
	if _, err := st.PutNode("note", "n2", NodeFields{Title: "t"}, dups); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("duplicate tags: expected ErrInvalidInput, got %v", err)
	}
}
