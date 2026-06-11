package akg

import (
	"fmt"
	"path/filepath"
	"testing"
)

// TestSecondaryIndexesAreMatchSized proves PERF-1: ListNodesByTag, OutboundEdges,
// and InboundEdges draw from secondary indexes sized to the number of matches, not
// the total store size. A full O(total) scan would have to examine every node/edge;
// here the backing index sets hold exactly the matching entries regardless of how
// large the rest of the store is.
func TestSecondaryIndexesAreMatchSized(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "idx.akg"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	// A mass of untagged "bulk" nodes the indexes must NOT have to scan. Kept
	// under the 1000-entry WAL auto-flush threshold so the test exercises the
	// indexes, not repeated whole-file rewrites (that cost is IO-1's concern).
	const bulk = 500
	for i := 0; i < bulk; i++ {
		if _, err := st.PutNode("bulk", fmt.Sprintf("b%d", i), NodeFields{Title: "b"}, nil); err != nil {
			t.Fatalf("PutNode bulk: %v", err)
		}
	}
	// Five tagged nodes.
	for i := 0; i < 5; i++ {
		if _, err := st.PutNode("doc", fmt.Sprintf("d%d", i), NodeFields{Title: "d"}, []string{"special"}); err != nil {
			t.Fatalf("PutNode tagged: %v", err)
		}
	}
	// A hub with outbound edges to 10 of the bulk nodes, and one leaf with a single
	// inbound edge.
	if _, err := st.PutNode("hub", "h", NodeFields{Title: "hub"}, nil); err != nil {
		t.Fatalf("PutNode hub: %v", err)
	}
	for i := 0; i < 10; i++ {
		if err := st.PutEdge(NodeRef{Type: "hub", ID: "h"}, "links", NodeRef{Type: "bulk", ID: fmt.Sprintf("b%d", i)}, EdgeFields{}); err != nil {
			t.Fatalf("PutEdge: %v", err)
		}
	}

	// Index sets are sized to matches, independent of the 2000-node bulk.
	if got := len(st.state.tagIndex["special"]); got != 5 {
		t.Fatalf("tagIndex[special] size = %d, want 5 (must not include %d bulk nodes)", got, bulk)
	}
	hub := nodeIdentity{typeName: "hub", id: "h"}
	if got := len(st.state.outIndex[hub]); got != 10 {
		t.Fatalf("outIndex[hub] size = %d, want 10", got)
	}
	leaf := nodeIdentity{typeName: "bulk", id: "b0"}
	if got := len(st.state.inIndex[leaf]); got != 1 {
		t.Fatalf("inIndex[b0] size = %d, want 1", got)
	}

	// And the queries return exactly those matches.
	tagged, err := st.ListNodesByTag("special")
	if err != nil || len(tagged) != 5 {
		t.Fatalf("ListNodesByTag = %d nodes (err=%v), want 5", len(tagged), err)
	}
	out, err := st.OutboundEdges(NodeRef{Type: "hub", ID: "h"}, "")
	if err != nil || len(out) != 10 {
		t.Fatalf("OutboundEdges = %d (err=%v), want 10", len(out), err)
	}
	in, err := st.InboundEdges(NodeRef{Type: "bulk", ID: "b0"}, "")
	if err != nil || len(in) != 1 {
		t.Fatalf("InboundEdges = %d (err=%v), want 1", len(in), err)
	}
}

// TestSecondaryIndexConsistency exercises every mutation path that must keep the
// indexes in sync: tag replace, node delete, edge delete, cascade delete, and a
// reopen that rebuilds the indexes from persisted records.
func TestSecondaryIndexConsistency(t *testing.T) {
	path := filepath.Join(t.TempDir(), "consistency.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Replace tags: "old" must leave the index, "new" must enter it.
	if _, err := st.PutNode("doc", "n1", NodeFields{Title: "t"}, []string{"old"}); err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	if _, err := st.PutNode("doc", "n1", NodeFields{Title: "t"}, []string{"new"}); err != nil {
		t.Fatalf("PutNode replace: %v", err)
	}
	if _, ok := st.state.tagIndex["old"]; ok {
		t.Fatal("tagIndex still has 'old' after tag replace")
	}
	if got := len(st.state.tagIndex["new"]); got != 1 {
		t.Fatalf("tagIndex[new] = %d, want 1", got)
	}

	// Edge add/delete keeps both directions consistent.
	if _, err := st.PutNode("doc", "n2", NodeFields{Title: "t"}, nil); err != nil {
		t.Fatalf("PutNode n2: %v", err)
	}
	if err := st.PutEdge(NodeRef{Type: "doc", ID: "n1"}, "links", NodeRef{Type: "doc", ID: "n2"}, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	n1 := nodeIdentity{typeName: "doc", id: "n1"}
	n2 := nodeIdentity{typeName: "doc", id: "n2"}
	if len(st.state.outIndex[n1]) != 1 || len(st.state.inIndex[n2]) != 1 {
		t.Fatal("edge indexes not populated after PutEdge")
	}
	if err := st.DeleteEdge(NodeRef{Type: "doc", ID: "n1"}, "links", NodeRef{Type: "doc", ID: "n2"}); err != nil {
		t.Fatalf("DeleteEdge: %v", err)
	}
	if len(st.state.outIndex[n1]) != 0 || len(st.state.inIndex[n2]) != 0 {
		t.Fatal("edge indexes not cleared after DeleteEdge")
	}

	// Delete node clears its tag index entry.
	if err := st.DeleteNode("doc", "n1"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	if len(st.state.tagIndex["new"]) != 0 {
		t.Fatal("tagIndex[new] not cleared after DeleteNode")
	}

	// Cascade delete removes edges and node, leaving indexes empty.
	if _, err := st.PutNode("doc", "a", NodeFields{Title: "t"}, []string{"k"}); err != nil {
		t.Fatalf("PutNode a: %v", err)
	}
	if err := st.PutEdge(NodeRef{Type: "doc", ID: "a"}, "links", NodeRef{Type: "doc", ID: "n2"}, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge a->n2: %v", err)
	}
	if err := st.PutEdge(NodeRef{Type: "doc", ID: "n2"}, "links", NodeRef{Type: "doc", ID: "a"}, EdgeFields{}); err != nil {
		t.Fatalf("PutEdge n2->a: %v", err)
	}
	res, err := st.DeleteNodeCascade("doc", "a")
	if err != nil {
		t.Fatalf("DeleteNodeCascade: %v", err)
	}
	if res.DeletedOutboundEdges != 1 || res.DeletedInboundEdges != 1 || !res.DeletedNode {
		t.Fatalf("cascade result = %+v, want 1 out / 1 in / deleted", res)
	}
	a := nodeIdentity{typeName: "doc", id: "a"}
	if len(st.state.outIndex[a]) != 0 || len(st.state.inIndex[a]) != 0 || len(st.state.tagIndex["k"]) != 0 {
		t.Fatal("indexes not cleared after cascade delete")
	}

	// Reopen: indexes must be rebuilt from persisted records so queries still work.
	if _, err := st.PutNode("doc", "survivor", NodeFields{Title: "t"}, []string{"keep"}); err != nil {
		t.Fatalf("PutNode survivor: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	re, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer re.Close()
	if got := len(re.state.tagIndex["keep"]); got != 1 {
		t.Fatalf("after reopen, tagIndex[keep] = %d, want 1 (index not rebuilt at load)", got)
	}
	survivors, err := re.ListNodesByTag("keep")
	if err != nil || len(survivors) != 1 {
		t.Fatalf("after reopen, ListNodesByTag(keep) = %d (err=%v), want 1", len(survivors), err)
	}
}

// TestSecondaryIndexReplayHonorsTagReplace covers the subtle case where committed
// WAL replay sees two PUT_NODE records for the same node with different tags: the
// rebuilt index must reflect only the latest tags, with no stale entry.
func TestSecondaryIndexReplayHonorsTagReplace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay-replace.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	// Two PUT_NODE records for the same node, both committed to the WAL (no
	// compaction) so reopen exercises replay, not hydrate.
	if _, err := st.PutNode("doc", "r", NodeFields{Title: "t"}, []string{"first"}); err != nil {
		t.Fatalf("PutNode first: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if _, err := st.PutNode("doc", "r", NodeFields{Title: "t"}, []string{"second"}); err != nil {
		t.Fatalf("PutNode second: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	re, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer re.Close()
	if _, ok := re.state.tagIndex["first"]; ok {
		t.Fatal("stale 'first' tag survived WAL replay")
	}
	if got := len(re.state.tagIndex["second"]); got != 1 {
		t.Fatalf("tagIndex[second] = %d, want 1", got)
	}
	if got, _ := re.ListNodesByTag("first"); len(got) != 0 {
		t.Fatalf("ListNodesByTag(first) = %d, want 0", len(got))
	}
	if got, _ := re.ListNodesByTag("second"); len(got) != 1 {
		t.Fatalf("ListNodesByTag(second) = %d, want 1", len(got))
	}
}
