package akg

import (
	"os"
	"path/filepath"
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
