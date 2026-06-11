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
		Strength:   StrengthOf(0.75),
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
	if err := st.PutEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}, EdgeFields{Strength: StrengthOf(0.4)}); err != nil {
		t.Fatalf("PutEdge initial: %v", err)
	}
	st.state.now = func() timestampMicros { return 25 }
	if err := st.PutEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}, EdgeFields{Strength: StrengthOf(0.8)}); err != nil {
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

	// invalid typeName (colon is the key delimiter — always rejected) returns error
	_, err = st.ListNodes("bad:type")
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

	// Invalid type name (over the 64-byte cap) → ErrInvalidInput
	if _, err := st.PutNode(strings.Repeat("a", 65), "id", NodeFields{Title: "t"}, nil); !errors.Is(err, ErrInvalidInput) {
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
	// Invalid relation name (contains colon) → ErrInvalidInput
	if err := st.PutEdge(n1, "bad:relation", n2, EdgeFields{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid relation name: expected ErrInvalidInput, got %v", err)
	}

	// Invalid tag (contains colon) → ErrInvalidInput
	if _, err := st.PutNode("note", "n3", NodeFields{Title: "three"}, []string{"bad:tag"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid tag: expected ErrInvalidInput, got %v", err)
	}

	// CONF-1: uppercase / non-ASCII type, relation, and tag are now accepted.
	un, err := st.PutNode("Person", "u1", NodeFields{Title: "upper"}, []string{"Active", "café"})
	if err != nil {
		t.Fatalf("uppercase/non-ascii type+tags should be accepted, got %v", err)
	}
	un2, err := st.PutNode("Person", "u2", NodeFields{Title: "upper2"}, nil)
	if err != nil {
		t.Fatalf("PutNode u2: %v", err)
	}
	if err := st.PutEdge(un, "KNOWS", un2, EdgeFields{}); err != nil {
		t.Fatalf("uppercase relation should be accepted, got %v", err)
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

// --- SDK-PARITY-001: edge strength default ---

func TestEdgeStrengthDefaultIs05(t *testing.T) {
	path := filepath.Join(t.TempDir(), "strength-default.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	alice, _ := st.PutNode("person", "alice", NodeFields{Title: "Alice"}, nil)
	bob, _ := st.PutNode("person", "bob", NodeFields{Title: "Bob"}, nil)
	if err := st.PutEdge(alice, "knows", bob, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	edges, err := st.OutboundEdges(alice, "knows")
	if err != nil {
		t.Fatalf("OutboundEdges: %v", err)
	}
	if len(edges) != 1 || edges[0].Strength != 0.5 {
		t.Fatalf("expected strength 0.5, got %v", edges[0].Strength)
	}
}

func TestEdgeStrengthExplicitZeroRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "strength-zero.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	alice, _ := st.PutNode("person", "alice", NodeFields{Title: "Alice"}, nil)
	bob, _ := st.PutNode("person", "bob", NodeFields{Title: "Bob"}, nil)
	if err := st.PutEdge(alice, "knows", bob, EdgeFields{Strength: StrengthOf(0.0)}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	edges, err := st.OutboundEdges(alice, "knows")
	if err != nil {
		t.Fatalf("OutboundEdges: %v", err)
	}
	if len(edges) != 1 || edges[0].Strength != 0.0 {
		t.Fatalf("expected strength 0.0, got %v", edges[0].Strength)
	}
}

func TestEdgeStrengthExplicitRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "strength-explicit.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	alice, _ := st.PutNode("person", "alice", NodeFields{Title: "Alice"}, nil)
	bob, _ := st.PutNode("person", "bob", NodeFields{Title: "Bob"}, nil)
	if err := st.PutEdge(alice, "knows", bob, EdgeFields{Strength: StrengthOf(0.75)}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	edges, err := st.OutboundEdges(alice, "knows")
	if err != nil {
		t.Fatalf("OutboundEdges: %v", err)
	}
	if len(edges) != 1 || edges[0].Strength != 0.75 {
		t.Fatalf("expected strength 0.75, got %v", edges[0].Strength)
	}
	// round-trip through commit and reopen
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	st2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	edges2, err := st2.OutboundEdges(alice, "knows")
	if err != nil {
		t.Fatalf("OutboundEdges after reopen: %v", err)
	}
	if len(edges2) != 1 || edges2[0].Strength != 0.75 {
		t.Fatalf("expected strength 0.75 after reopen, got %v", edges2[0].Strength)
	}
}

// --- SDK-PARITY-002: compaction ---

func TestCompactPendingMutationsCommittedBeforeCompaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compact-pending.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "hello"}, nil); err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	// Do not commit — pending mutation should be committed by Compact.
	if err := st.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	node, err := st.GetNode("note", "n1")
	if err != nil || node == nil {
		t.Fatalf("node should be present after Compact: err=%v node=%v", err, node)
	}
}

func TestCompactNoWALAfterCompaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compact-wal.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	alice, _ := st.PutNode("person", "alice", NodeFields{Title: "Alice"}, nil)
	bob, _ := st.PutNode("person", "bob", NodeFields{Title: "Bob"}, nil)
	if err := st.PutEdge(alice, "knows", bob, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	if err := st.DeleteEdge(alice, "knows", bob); err != nil {
		t.Fatalf("DeleteEdge: %v", err)
	}
	if err := st.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	// The file should have no WAL section.
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	c, err := decodeContainer(fileBytes)
	if err != nil {
		t.Fatalf("decodeContainer: %v", err)
	}
	if c.WAL != nil {
		t.Fatalf("expected no WAL after compaction, got WAL len=%d", len(c.WAL))
	}
}

func TestCompactReopenPreservesGraph(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compact-reopen.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	alice, _ := st.PutNode("person", "alice", NodeFields{Title: "Alice"}, nil)
	bob, _ := st.PutNode("person", "bob", NodeFields{Title: "Bob"}, nil)
	if err := st.PutEdge(alice, "knows", bob, EdgeFields{Strength: StrengthOf(0.7)}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	if err := st.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	// Store still usable after compaction.
	nodes, err := st.ListNodes("")
	if err != nil || len(nodes) != 2 {
		t.Fatalf("expected 2 nodes after Compact, got %v err=%v", len(nodes), err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Reopen should have the same logical content.
	st2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	nodes2, err := st2.ListNodes("")
	if err != nil || len(nodes2) != 2 {
		t.Fatalf("expected 2 nodes after reopen, got %v err=%v", len(nodes2), err)
	}
	edges2, err := st2.ListEdges(EdgeFilter{})
	if err != nil || len(edges2) != 1 || edges2[0].Strength != 0.7 {
		t.Fatalf("expected 1 edge with strength 0.7 after reopen, got %v err=%v", edges2, err)
	}
}

// --- SDK-PARITY-003: global edge listing and snapshots ---

func TestListEdgesEmptyFilterReturnsAll(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list-edges.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	n1, _ := st.PutNode("note", "n1", NodeFields{Title: "one"}, nil)
	n2, _ := st.PutNode("note", "n2", NodeFields{Title: "two"}, nil)
	n3, _ := st.PutNode("note", "n3", NodeFields{Title: "three"}, nil)
	if err := st.PutEdge(n1, "links_to", n2, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	if err := st.PutEdge(n2, "mentions", n3, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	if err := st.PutEdge(n1, "mentions", n3, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	edges, err := st.ListEdges(EdgeFilter{})
	if err != nil {
		t.Fatalf("ListEdges: %v", err)
	}
	if len(edges) != 3 {
		t.Fatalf("expected 3 edges, got %d", len(edges))
	}
}

func TestListEdgesRelationFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list-edges-rel.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	n1, _ := st.PutNode("note", "n1", NodeFields{Title: "one"}, nil)
	n2, _ := st.PutNode("note", "n2", NodeFields{Title: "two"}, nil)
	n3, _ := st.PutNode("note", "n3", NodeFields{Title: "three"}, nil)
	if err := st.PutEdge(n1, "links_to", n2, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	if err := st.PutEdge(n1, "mentions", n3, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	edges, err := st.ListEdges(EdgeFilter{Relation: "links_to"})
	if err != nil {
		t.Fatalf("ListEdges: %v", err)
	}
	if len(edges) != 1 || edges[0].Relation != "links_to" {
		t.Fatalf("expected 1 links_to edge, got %v", edges)
	}
}

func TestListEdgesMetaFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list-edges-meta.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	n1, _ := st.PutNode("note", "n1", NodeFields{Title: "one"}, nil)
	n2, _ := st.PutNode("note", "n2", NodeFields{Title: "two"}, nil)
	n3, _ := st.PutNode("note", "n3", NodeFields{Title: "three"}, nil)
	if err := st.PutEdge(n1, "links_to", n2, EdgeFields{Meta: map[string]any{"source": "inferred"}}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	if err := st.PutEdge(n1, "links_to", n3, EdgeFields{Meta: map[string]any{"source": "manual"}}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	edges, err := st.ListEdges(EdgeFilter{Meta: map[string]any{"source": "inferred"}})
	if err != nil {
		t.Fatalf("ListEdges: %v", err)
	}
	if len(edges) != 1 || edges[0].To.ID != "n2" {
		t.Fatalf("expected 1 inferred edge to n2, got %v", edges)
	}
}

func TestListEdgesRelationAndMetaAND(t *testing.T) {
	path := filepath.Join(t.TempDir(), "list-edges-and.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	n1, _ := st.PutNode("note", "n1", NodeFields{Title: "one"}, nil)
	n2, _ := st.PutNode("note", "n2", NodeFields{Title: "two"}, nil)
	n3, _ := st.PutNode("note", "n3", NodeFields{Title: "three"}, nil)
	if err := st.PutEdge(n1, "links_to", n2, EdgeFields{Meta: map[string]any{"source": "inferred"}}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	if err := st.PutEdge(n1, "mentions", n3, EdgeFields{Meta: map[string]any{"source": "inferred"}}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	edges, err := st.ListEdges(EdgeFilter{Relation: "links_to", Meta: map[string]any{"source": "inferred"}})
	if err != nil {
		t.Fatalf("ListEdges: %v", err)
	}
	if len(edges) != 1 || edges[0].Relation != "links_to" {
		t.Fatalf("expected 1 edge, got %v", edges)
	}
}

func TestSnapshotAllLiveNodesAndEdges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snapshot.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	n1, _ := st.PutNode("note", "n1", NodeFields{Title: "one"}, nil)
	n2, _ := st.PutNode("note", "n2", NodeFields{Title: "two"}, nil)
	if err := st.PutEdge(n1, "links_to", n2, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	snap, err := st.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap.Nodes) != 2 || len(snap.Edges) != 1 {
		t.Fatalf("expected 2 nodes and 1 edge, got %d/%d", len(snap.Nodes), len(snap.Edges))
	}
}

// --- SDK-PARITY-004: node filtering and batch inspection ---

func TestListNodesFilteredTypeAndTag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filter-nodes.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	st.PutNode("person", "alice", NodeFields{Title: "Alice"}, []string{"active"})
	st.PutNode("person", "bob", NodeFields{Title: "Bob"}, []string{"inactive"})
	st.PutNode("task", "t1", NodeFields{Title: "Task"}, []string{"active"})

	nodes, err := st.ListNodesFiltered(NodeFilter{Type: "person", Tag: "active"})
	if err != nil {
		t.Fatalf("ListNodesFiltered: %v", err)
	}
	if len(nodes) != 1 || nodes[0].ID != "alice" {
		t.Fatalf("expected [alice], got %v", nodes)
	}
}

func TestListNodesFilteredMetaDeepEquality(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filter-meta.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	st.PutNode("note", "n1", NodeFields{Title: "one", Meta: map[string]any{"status": "accepted"}}, nil)
	st.PutNode("note", "n2", NodeFields{Title: "two", Meta: map[string]any{"status": "rejected"}}, nil)
	st.PutNode("note", "n3", NodeFields{Title: "three", Meta: map[string]any{"tags": []any{"a", "b"}}}, nil)
	st.PutNode("note", "n4", NodeFields{Title: "four", Meta: map[string]any{"obj": map[string]any{"x": 1.0, "y": 2.0}}}, nil)

	// scalar equality
	nodes, err := st.ListNodesFiltered(NodeFilter{Meta: map[string]any{"status": "accepted"}})
	if err != nil || len(nodes) != 1 || nodes[0].ID != "n1" {
		t.Fatalf("scalar filter: expected [n1], got %v (err=%v)", nodes, err)
	}

	// array equality
	nodes, err = st.ListNodesFiltered(NodeFilter{Meta: map[string]any{"tags": []any{"a", "b"}}})
	if err != nil || len(nodes) != 1 || nodes[0].ID != "n3" {
		t.Fatalf("array filter: expected [n3], got %v (err=%v)", nodes, err)
	}

	// object equality (key order ignored)
	nodes, err = st.ListNodesFiltered(NodeFilter{Meta: map[string]any{"obj": map[string]any{"y": 2.0, "x": 1.0}}})
	if err != nil || len(nodes) != 1 || nodes[0].ID != "n4" {
		t.Fatalf("object filter: expected [n4], got %v (err=%v)", nodes, err)
	}

	// missing key excludes node
	nodes, err = st.ListNodesFiltered(NodeFilter{Meta: map[string]any{"nonexistent": "x"}})
	if err != nil || len(nodes) != 0 {
		t.Fatalf("missing key: expected empty, got %v (err=%v)", nodes, err)
	}
}

func TestGetNodesPreservesOrderAndDuplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "get-nodes.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	st.PutNode("note", "n1", NodeFields{Title: "one"}, nil)
	st.PutNode("note", "n2", NodeFields{Title: "two"}, nil)

	refs := []NodeRef{
		{Type: "note", ID: "n2"},
		{Type: "note", ID: "n1"},
		{Type: "note", ID: "missing"},
		{Type: "note", ID: "n2"},
	}
	nodes, err := st.GetNodes(refs)
	if err != nil {
		t.Fatalf("GetNodes: %v", err)
	}
	if len(nodes) != 4 {
		t.Fatalf("expected 4 positions, got %d", len(nodes))
	}
	if nodes[0] == nil || nodes[0].ID != "n2" {
		t.Fatalf("position 0: expected n2, got %v", nodes[0])
	}
	if nodes[1] == nil || nodes[1].ID != "n1" {
		t.Fatalf("position 1: expected n1, got %v", nodes[1])
	}
	if nodes[2] != nil {
		t.Fatalf("position 2: expected nil for missing, got %v", nodes[2])
	}
	if nodes[3] == nil || nodes[3].ID != "n2" {
		t.Fatalf("position 3: expected n2 duplicate, got %v", nodes[3])
	}
}

func TestListNodesFilteredUnknownTypeReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filter-unknown.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	st.PutNode("note", "n1", NodeFields{Title: "one"}, nil)
	nodes, err := st.ListNodesFiltered(NodeFilter{Type: "nonexistent"})
	if err != nil || len(nodes) != 0 {
		t.Fatalf("unknown type: expected empty, got %v (err=%v)", nodes, err)
	}
}

// --- SDK-PARITY-005: recency helpers ---

func TestRecentNodesNewestFirst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recency-nodes.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	st.state.now = func() timestampMicros { return 100 }
	st.PutNode("task", "t1", NodeFields{Title: "Task1"}, nil)
	st.state.now = func() timestampMicros { return 200 }
	st.PutNode("task", "t2", NodeFields{Title: "Task2"}, nil)
	st.state.now = func() timestampMicros { return 300 }
	st.PutNode("task", "t3", NodeFields{Title: "Task3"}, nil)

	nodes, err := st.RecentNodes(RecencyFilter{})
	if err != nil {
		t.Fatalf("RecentNodes: %v", err)
	}
	if len(nodes) != 3 || nodes[0].ID != "t3" || nodes[1].ID != "t2" || nodes[2].ID != "t1" {
		t.Fatalf("expected [t3,t2,t1], got %v", nodeIDs(nodes))
	}
}

func TestRecentNodesSinceUntilBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recency-bounds.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	st.state.now = func() timestampMicros { return 100 }
	st.PutNode("task", "t1", NodeFields{Title: "Task1"}, nil)
	st.state.now = func() timestampMicros { return 200 }
	st.PutNode("task", "t2", NodeFields{Title: "Task2"}, nil)
	st.state.now = func() timestampMicros { return 300 }
	st.PutNode("task", "t3", NodeFields{Title: "Task3"}, nil)

	// inclusive since
	nodes, err := st.RecentNodes(RecencyFilter{SinceUpdatedAt: 200})
	if err != nil || len(nodes) != 2 {
		t.Fatalf("since 200: expected 2, got %v (err=%v)", nodeIDs(nodes), err)
	}
	if nodes[0].ID != "t3" || nodes[1].ID != "t2" {
		t.Fatalf("since 200: expected [t3,t2], got %v", nodeIDs(nodes))
	}

	// inclusive until
	nodes, err = st.RecentNodes(RecencyFilter{UntilUpdatedAt: 200})
	if err != nil || len(nodes) != 2 {
		t.Fatalf("until 200: expected 2, got %v (err=%v)", nodeIDs(nodes), err)
	}

	// both bounds
	nodes, err = st.RecentNodes(RecencyFilter{SinceUpdatedAt: 150, UntilUpdatedAt: 250})
	if err != nil || len(nodes) != 1 || nodes[0].ID != "t2" {
		t.Fatalf("range 150-250: expected [t2], got %v (err=%v)", nodeIDs(nodes), err)
	}
}

func TestRecentNodesLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recency-limit.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for i := 0; i < 5; i++ {
		st.state.now = func() timestampMicros { return timestampMicros(i * 100) }
		st.PutNode("task", "t"+string(rune('a'+i)), NodeFields{Title: "Task"}, nil)
	}

	// limit 0 = unlimited
	nodes, err := st.RecentNodes(RecencyFilter{Limit: 0})
	if err != nil || len(nodes) != 5 {
		t.Fatalf("limit 0: expected 5, got %v (err=%v)", len(nodes), err)
	}

	// positive limit
	nodes, err = st.RecentNodes(RecencyFilter{Limit: 2})
	if err != nil || len(nodes) != 2 {
		t.Fatalf("limit 2: expected 2, got %v (err=%v)", len(nodes), err)
	}

	// negative limit is invalid
	if _, err := st.RecentNodes(RecencyFilter{Limit: -1}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("negative limit: expected ErrInvalidInput, got %v", err)
	}
}

func TestRecentEdgesEndpointAndRelationFilter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recency-edges.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	alice, _ := st.PutNode("person", "alice", NodeFields{Title: "Alice"}, nil)
	bob, _ := st.PutNode("person", "bob", NodeFields{Title: "Bob"}, nil)
	carol, _ := st.PutNode("person", "carol", NodeFields{Title: "Carol"}, nil)
	st.state.now = func() timestampMicros { return 100 }
	st.PutEdge(alice, "knows", bob, EdgeFields{})
	st.state.now = func() timestampMicros { return 200 }
	st.PutEdge(alice, "knows", carol, EdgeFields{})
	st.state.now = func() timestampMicros { return 300 }
	st.PutEdge(bob, "knows", carol, EdgeFields{})

	// from filter
	edges, err := st.RecentEdges(EdgeRecencyFilter{From: &alice})
	if err != nil || len(edges) != 2 {
		t.Fatalf("from alice: expected 2, got %v (err=%v)", len(edges), err)
	}
	// newest first
	if edges[0].To.ID != "carol" || edges[1].To.ID != "bob" {
		t.Fatalf("from alice order: expected [carol,bob], got [%v,%v]", edges[0].To.ID, edges[1].To.ID)
	}

	// to filter
	edges, err = st.RecentEdges(EdgeRecencyFilter{To: &carol})
	if err != nil || len(edges) != 2 {
		t.Fatalf("to carol: expected 2, got %v (err=%v)", len(edges), err)
	}

	// relation filter
	edges, err = st.RecentEdges(EdgeRecencyFilter{Relation: "knows", From: &alice})
	if err != nil || len(edges) != 2 {
		t.Fatalf("relation+from: expected 2, got %v (err=%v)", len(edges), err)
	}

	// negative limit is invalid
	if _, err := st.RecentEdges(EdgeRecencyFilter{Limit: -1}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("negative limit: expected ErrInvalidInput, got %v", err)
	}
}

// --- SDK-PARITY-006: edge reconciliation ---

func TestReconcileOutboundEdgesAddMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reconcile-add.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	alice, _ := st.PutNode("person", "alice", NodeFields{Title: "Alice"}, nil)
	bob, _ := st.PutNode("person", "bob", NodeFields{Title: "Bob"}, nil)
	carol, _ := st.PutNode("person", "carol", NodeFields{Title: "Carol"}, nil)

	result, err := st.ReconcileOutboundEdges(alice, "knows", []NodeRef{bob, carol}, EdgeFields{Strength: StrengthOf(0.8)})
	if err != nil {
		t.Fatalf("ReconcileOutboundEdges: %v", err)
	}
	if result.Added != 2 || result.Removed != 0 || result.Unchanged != 0 {
		t.Fatalf("expected Added=2 Removed=0 Unchanged=0, got %+v", result)
	}
	edges, _ := st.OutboundEdges(alice, "knows")
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}
}

func TestReconcileOutboundEdgesRemoveStale(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reconcile-remove.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	alice, _ := st.PutNode("person", "alice", NodeFields{Title: "Alice"}, nil)
	bob, _ := st.PutNode("person", "bob", NodeFields{Title: "Bob"}, nil)
	carol, _ := st.PutNode("person", "carol", NodeFields{Title: "Carol"}, nil)
	st.PutEdge(alice, "knows", bob, EdgeFields{})
	st.PutEdge(alice, "knows", carol, EdgeFields{})

	// Reconcile to only bob
	result, err := st.ReconcileOutboundEdges(alice, "knows", []NodeRef{bob}, EdgeFields{})
	if err != nil {
		t.Fatalf("ReconcileOutboundEdges: %v", err)
	}
	if result.Added != 0 || result.Removed != 1 || result.Unchanged != 1 {
		t.Fatalf("expected Added=0 Removed=1 Unchanged=1, got %+v", result)
	}
	edges, _ := st.OutboundEdges(alice, "knows")
	if len(edges) != 1 || edges[0].To.ID != "bob" {
		t.Fatalf("expected only bob edge, got %v", edges)
	}
}

func TestReconcileOutboundEdgesUnrelatedUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reconcile-unrelated.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	alice, _ := st.PutNode("person", "alice", NodeFields{Title: "Alice"}, nil)
	bob, _ := st.PutNode("person", "bob", NodeFields{Title: "Bob"}, nil)
	carol, _ := st.PutNode("person", "carol", NodeFields{Title: "Carol"}, nil)
	// Different relation: "likes"
	st.PutEdge(alice, "likes", carol, EdgeFields{})

	_, err = st.ReconcileOutboundEdges(alice, "knows", []NodeRef{bob}, EdgeFields{})
	if err != nil {
		t.Fatalf("ReconcileOutboundEdges: %v", err)
	}
	// "likes" edge should still exist
	likes, _ := st.OutboundEdges(alice, "likes")
	if len(likes) != 1 {
		t.Fatalf("unrelated 'likes' edge removed, should be unchanged")
	}
}

// --- SDK-PARITY-007: cascade delete ---

func TestDeleteNodeCascadeRemovesEdgesAndNode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cascade.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	alice, _ := st.PutNode("person", "alice", NodeFields{Title: "Alice"}, nil)
	bob, _ := st.PutNode("person", "bob", NodeFields{Title: "Bob"}, nil)
	carol, _ := st.PutNode("person", "carol", NodeFields{Title: "Carol"}, nil)
	st.PutEdge(alice, "knows", bob, EdgeFields{})   // outbound from alice
	st.PutEdge(carol, "knows", alice, EdgeFields{}) // inbound to alice

	result, err := st.DeleteNodeCascade("person", "alice")
	if err != nil {
		t.Fatalf("DeleteNodeCascade: %v", err)
	}
	if result.DeletedInboundEdges != 1 || result.DeletedOutboundEdges != 1 || !result.DeletedNode {
		t.Fatalf("unexpected result: %+v", result)
	}
	node, _ := st.GetNode("person", "alice")
	if node != nil {
		t.Fatal("alice should be absent after cascade delete")
	}
	edges, _ := st.OutboundEdges(bob, "knows")
	_ = edges
	inbound, _ := st.InboundEdges(NodeRef{Type: "person", ID: "bob"}, "")
	if len(inbound) != 0 {
		t.Fatalf("expected no inbound edges to bob, got %d", len(inbound))
	}
}

func TestDeleteNodeNormalStillBlockedByEdges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-cascade.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	alice, _ := st.PutNode("person", "alice", NodeFields{Title: "Alice"}, nil)
	bob, _ := st.PutNode("person", "bob", NodeFields{Title: "Bob"}, nil)
	st.PutEdge(alice, "knows", bob, EdgeFields{})

	if err := st.DeleteNode("person", "alice"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput when node has edges, got %v", err)
	}
}

func TestDeleteNodeCascadeNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cascade-notfound.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := st.DeleteNodeCascade("person", "nonexistent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestDecodeEdgePayloadIntegerEncodedFloats verifies that decodeEdgePayload
// accepts uint64-encoded values for strength and confidence — the encoding
// a JS runtime emits for whole-number floats (e.g. confidence: 1).
func TestDecodeEdgePayloadIntegerEncodedFloats(t *testing.T) {
	conf := float64(1)
	cases := []struct {
		name       string
		strength   any
		confidence any
		wantStr    float64
		wantConf   *float64
	}{
		{"float64 encoding", float64(0.8), float64(1), 0.8, &conf},
		{"uint64 encoding", uint64(1), uint64(1), 1.0, &conf},
		{"mixed encoding", float64(0.5), uint64(1), 0.5, &conf},
		{"nil confidence", float64(0.5), nil, 0.5, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := map[string]any{
				"from_node_type": "note",
				"from_node":      "n1",
				"to_node_type":   "note",
				"to_node":        "n2",
				"relation":       "links_to",
				"strength":       tc.strength,
				"created_at":     uint64(1000),
				"updated_at":     uint64(2000),
			}
			if tc.confidence != nil {
				m["confidence"] = tc.confidence
			} else {
				m["confidence"] = nil
			}
			b, err := encodeMsgpack(m)
			if err != nil {
				t.Fatalf("encodeMsgpack: %v", err)
			}
			edge, err := decodeEdgePayload(b)
			if err != nil {
				t.Fatalf("decodeEdgePayload: %v", err)
			}
			if edge.Strength != tc.wantStr {
				t.Errorf("strength: got %v, want %v", edge.Strength, tc.wantStr)
			}
			if tc.wantConf == nil {
				if edge.Confidence != nil {
					t.Errorf("confidence: got %v, want nil", *edge.Confidence)
				}
			} else {
				if edge.Confidence == nil {
					t.Errorf("confidence: got nil, want %v", *tc.wantConf)
				} else if *edge.Confidence != *tc.wantConf {
					t.Errorf("confidence: got %v, want %v", *edge.Confidence, *tc.wantConf)
				}
			}
		})
	}
}

// --- helpers ---

func nodeIDs(nodes []Node) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}
