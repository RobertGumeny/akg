package akg

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Run with: GEN_FIXTURE_DIR=../../testdata/conformance go test -run TestGenEdgeConformanceFixtures .
func TestGenEdgeConformanceFixtures(t *testing.T) {
	dir := os.Getenv("GEN_FIXTURE_DIR")
	if dir == "" {
		t.Skip("GEN_FIXTURE_DIR not set; skipping fixture generation")
	}

	fixtures := []struct {
		name string
		fn   func(path string) error
	}{
		{"m2-single-edge.akg", genFixtureSingleEdge},
		{"m2-small-graph.akg", genFixtureSmallGraph},
		{"m1-data-bloom-wal.akg", genFixtureDataBloomWAL},
		{"m2-committed-wal-replay.akg", genFixtureCommittedWALReplay},
		{"m2-uncommitted-wal-tail.akg", genFixtureUncommittedWALTail},
		{"m2-deletes-before-compaction.akg", genFixtureDeletesBeforeCompaction},
		{"m2-edge-deleted-before-commit.akg", genFixtureEdgeDeletedBeforeCommit},
		{"m2-edge-deletion-survives-reopen.akg", genFixtureEdgeDeletionSurvivesReopen},
		{"m2-reject-derived-index-mismatch.akg", genFixtureRejectDerivedIndexMismatch},
		{"m2-reject-malformed-committed-wal.akg", genFixtureRejectMalformedCommittedWAL},
	}

	for _, fx := range fixtures {
		path := filepath.Join(dir, fx.name)
		if err := fx.fn(path); err != nil {
			t.Fatalf("generate %s: %v", fx.name, err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", fx.name, err)
		}
		sum := sha256.Sum256(data)
		fmt.Printf("%-50s sha256:%s\n", fx.name, hex.EncodeToString(sum[:]))
	}
}

// genFixtureSingleEdge: compacted graph with 2 nodes and 1 edge, no WAL.
func genFixtureSingleEdge(path string) error {
	const ts = timestampMicros(1_000_000)
	s := newStoreState()
	s.now = func() timestampMicros { return ts }
	if _, err := s.putNode("n1", coreNode{Type: "note", Title: "one"}); err != nil {
		return err
	}
	if _, err := s.putNode("n2", coreNode{Type: "note", Title: "two"}); err != nil {
		return err
	}
	if _, err := s.putEdge(coreEdge{FromType: "note", FromNode: "n1", Relation: "links_to", ToType: "note", ToNode: "n2"}); err != nil {
		return err
	}
	return writeCompactedStore(path, s)
}

// genFixtureSmallGraph: compacted small graph with mixed node types, tags, and edges.
func genFixtureSmallGraph(path string) error {
	const ts = timestampMicros(2_000_000)
	s := newStoreState()
	s.now = func() timestampMicros { return ts }
	if _, err := s.putNode("c1", coreNode{Type: "concept", Title: "Alpha", Tags: []string{"published"}}); err != nil {
		return err
	}
	if _, err := s.putNode("c2", coreNode{Type: "concept", Title: "Beta"}); err != nil {
		return err
	}
	if _, err := s.putNode("n1", coreNode{Type: "note", Title: "First"}); err != nil {
		return err
	}
	if _, err := s.putEdge(coreEdge{FromType: "concept", FromNode: "c1", Relation: "links_to", ToType: "note", ToNode: "n1"}); err != nil {
		return err
	}
	if _, err := s.putEdge(coreEdge{FromType: "concept", FromNode: "c2", Relation: "links_to", ToType: "note", ToNode: "n1"}); err != nil {
		return err
	}
	return writeCompactedStore(path, s)
}

// genFixtureDataBloomWAL: format-scope fixture with Data, Bloom, and WAL sections containing edges.
func genFixtureDataBloomWAL(path string) error {
	// Remove any existing file so Open creates fresh (old-format files would fail to open).
	_ = os.Remove(path)
	// Build the store via the public API so Data+Bloom+WAL are all present.
	st, err := Open(path)
	if err != nil {
		return err
	}
	st.state.now = func() timestampMicros { return 3_000_000 }
	if _, err := st.PutNode("note", "x1", NodeFields{Title: "X One"}, nil); err != nil {
		return err
	}
	if _, err := st.PutNode("note", "x2", NodeFields{Title: "X Two"}, nil); err != nil {
		return err
	}
	if err := st.PutEdge(NodeRef{Type: "note", ID: "x1"}, "links_to", NodeRef{Type: "note", ID: "x2"}, EdgeFields{}); err != nil {
		return err
	}
	return st.Close()
}

// genFixtureCommittedWALReplay: base Data plus committed WAL. 2 nodes, 1 edge. nextWALSeq=10.
// WAL: PUT_NODE@1, PUT_NODE@2, COMMIT@3 | PUT_EDGE@4, PUT_NODE@5, COMMIT@6 | PUT_NODE@7, PUT_EDGE@8, COMMIT@9
func genFixtureCommittedWALReplay(path string) error {
	_ = os.Remove(path)
	st, err := Open(path)
	if err != nil {
		return err
	}
	st.state.now = func() timestampMicros { return 10 }
	if _, err := st.PutNode("note", "a1", NodeFields{Title: "Node A"}, nil); err != nil {
		return err
	}
	if _, err := st.PutNode("note", "a2", NodeFields{Title: "Node B"}, nil); err != nil {
		return err
	}
	if err := st.Commit(); err != nil { // seqs 1,2,3
		return err
	}
	st.state.now = func() timestampMicros { return 20 }
	if err := st.PutEdge(NodeRef{Type: "note", ID: "a1"}, "links_to", NodeRef{Type: "note", ID: "a2"}, EdgeFields{}); err != nil {
		return err
	}
	if _, err := st.PutNode("note", "a1", NodeFields{Title: "Node A v2"}, nil); err != nil {
		return err
	}
	if err := st.Commit(); err != nil { // seqs 4,5,6
		return err
	}
	st.state.now = func() timestampMicros { return 30 }
	if _, err := st.PutNode("note", "a2", NodeFields{Title: "Node B v2"}, nil); err != nil {
		return err
	}
	if err := st.PutEdge(NodeRef{Type: "note", ID: "a1"}, "links_to", NodeRef{Type: "note", ID: "a2"}, EdgeFields{Strength: 0.8}); err != nil {
		return err
	}
	return st.Close() // seqs 7,8,9 — nextWALSeq=10
}

// genFixtureUncommittedWALTail: committed WAL (nextWALSeq would be 10) plus one uncommitted record at seq 10.
// Final nextWALSeq=11.
func genFixtureUncommittedWALTail(path string) error {
	// First generate the base committed-wal-replay content into a temp file.
	tmp := path + ".tmp"
	defer os.Remove(tmp)
	if err := genFixtureCommittedWALReplay(tmp); err != nil {
		return err
	}
	data, err := os.ReadFile(tmp)
	if err != nil {
		return err
	}
	c, err := decodeContainer(data)
	if err != nil {
		return err
	}

	// Append an uncommitted PUT_NODE record at seq 10 to the WAL.
	extraPayload, err := encodeNodePutPayload(nodePut{
		ID:   "extra",
		Node: coreNode{Type: "note", Title: "uncommitted", CreatedAt: 99, UpdatedAt: 99, Version: 1},
	})
	if err != nil {
		return err
	}
	extraRecord, err := encodeWALRecord(walRecord{Sequence: 10, Operation: walOpPutNode, Payload: extraPayload})
	if err != nil {
		return err
	}
	c.WAL = append(c.WAL, extraRecord...)

	out, err := encodeContainer(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// genFixtureDeletesBeforeCompaction: WAL with logical deletes. 1 node, 0 edges. nextWALSeq=8.
// WAL: PUT_NODE@1, PUT_NODE@2, PUT_EDGE@3, COMMIT@4 | DELETE_EDGE@5, DELETE_NODE@6, COMMIT@7
func genFixtureDeletesBeforeCompaction(path string) error {
	_ = os.Remove(path)
	st, err := Open(path)
	if err != nil {
		return err
	}
	st.state.now = func() timestampMicros { return 100 }
	if _, err := st.PutNode("note", "keep", NodeFields{Title: "Keeper"}, nil); err != nil {
		return err
	}
	if _, err := st.PutNode("note", "drop", NodeFields{Title: "Dropped"}, nil); err != nil {
		return err
	}
	if err := st.PutEdge(NodeRef{Type: "note", ID: "keep"}, "links_to", NodeRef{Type: "note", ID: "drop"}, EdgeFields{}); err != nil {
		return err
	}
	if err := st.Commit(); err != nil { // seqs 1,2,3,4
		return err
	}
	if err := st.DeleteEdge(NodeRef{Type: "note", ID: "keep"}, "links_to", NodeRef{Type: "note", ID: "drop"}); err != nil {
		return err
	}
	if err := st.DeleteNode("note", "drop"); err != nil {
		return err
	}
	return st.Close() // seqs 5,6,7 — nextWALSeq=8
}

// genFixtureEdgeDeletedBeforeCommit: PutNode×2, PutEdge, DeleteEdge in one commit.
// WAL: PUT_NODE@1, PUT_NODE@2, PUT_EDGE@3, DELETE_EDGE@4, COMMIT@5 — nextWALSeq=6
func genFixtureEdgeDeletedBeforeCommit(path string) error {
	_ = os.Remove(path)
	st, err := Open(path)
	if err != nil {
		return err
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "one"}, nil); err != nil {
		return err
	}
	if _, err := st.PutNode("note", "n2", NodeFields{Title: "two"}, nil); err != nil {
		return err
	}
	if err := st.PutEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}, EdgeFields{}); err != nil {
		return err
	}
	if err := st.DeleteEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}); err != nil {
		return err
	}
	return st.Close()
}

// genFixtureEdgeDeletionSurvivesReopen: PutNode×2+PutEdge committed, then DeleteEdge committed.
// WAL: PUT_NODE@1, PUT_NODE@2, PUT_EDGE@3, COMMIT@4 | DELETE_EDGE@5, COMMIT@6 — nextWALSeq=7
func genFixtureEdgeDeletionSurvivesReopen(path string) error {
	_ = os.Remove(path)
	st, err := Open(path)
	if err != nil {
		return err
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "one"}, nil); err != nil {
		return err
	}
	if _, err := st.PutNode("note", "n2", NodeFields{Title: "two"}, nil); err != nil {
		return err
	}
	if err := st.PutEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}, EdgeFields{}); err != nil {
		return err
	}
	if err := st.Close(); err != nil {
		return err
	}

	st2, err := Open(path)
	if err != nil {
		return err
	}
	if err := st2.DeleteEdge(NodeRef{Type: "note", ID: "n1"}, "links_to", NodeRef{Type: "note", ID: "n2"}); err != nil {
		return err
	}
	return st2.Close()
}

// genFixtureRejectDerivedIndexMismatch: valid graph with edge but missing the edge index key.
func genFixtureRejectDerivedIndexMismatch(path string) error {
	const ts = timestampMicros(1_000_000)
	s := newStoreState()
	s.now = func() timestampMicros { return ts }
	if _, err := s.putNode("p1", coreNode{Type: "concept", Title: "Pivot"}); err != nil {
		return err
	}
	if _, err := s.putNode("p2", coreNode{Type: "concept", Title: "Target"}); err != nil {
		return err
	}
	if _, err := s.putEdge(coreEdge{FromType: "concept", FromNode: "p1", Relation: "links_to", ToType: "concept", ToNode: "p2"}); err != nil {
		return err
	}

	// Materialize all entries, then remove the edge index key to create a mismatch.
	entries, err := materializeDataEntries(s)
	if err != nil {
		return err
	}
	filtered := entries[:0]
	for _, e := range entries {
		key := string(e.Key)
		if len(key) > 3 && key[:3] == "ei:" {
			continue // drop all edge index entries
		}
		filtered = append(filtered, e)
	}

	data, err := encodeDataEntries(filtered)
	if err != nil {
		return err
	}
	keys := make([][]byte, len(filtered))
	for i, e := range filtered {
		keys[i] = e.Key
	}
	file, err := encodeContainer(container{Data: data, Bloom: encodeBloom(keys), WAL: nil})
	if err != nil {
		return err
	}
	return os.WriteFile(path, file, 0o644)
}

// genFixtureRejectMalformedCommittedWAL: empty store with a committed PUT_EDGE WAL record
// whose length field is set too large, so decodeWALRecord returns errInvalidWALRecord before CRC check.
func genFixtureRejectMalformedCommittedWAL(path string) error {
	// Build a valid PUT_EDGE payload.
	edgePayload, err := encodeEdgePutPayload(edgePut{Edge: coreEdge{
		FromType: "note", FromNode: "n1", Relation: "links_to", ToType: "note", ToNode: "n2",
		Strength: 0.5, CreatedAt: 1_000_000, UpdatedAt: 1_000_000, Version: 1,
	}})
	if err != nil {
		return err
	}

	// Encode a valid PUT_EDGE@1 record, then corrupt its length field (bytes 9-12, LE uint32)
	// to a very large value so the size check fires before the CRC check.
	validRecord, err := encodeWALRecord(walRecord{Sequence: 1, Operation: walOpPutEdge, Payload: edgePayload})
	if err != nil {
		return err
	}
	// Overwrite length bytes with 0x7FFFFFFF (max positive int32, way larger than any real payload).
	binary.LittleEndian.PutUint32(validRecord[9:13], 0x7FFFFFFF)

	// Encode the COMMIT record.
	commitRecord, err := encodeWALRecord(walRecord{Sequence: 2, Operation: walOpCommit})
	if err != nil {
		return err
	}

	walBytes := append(validRecord, commitRecord...)

	// Wrap in an empty store container with this WAL.
	emptyData, err := encodeDataEntries(nil)
	if err != nil {
		return err
	}
	file, err := encodeContainer(container{Data: emptyData, WAL: walBytes})
	if err != nil {
		return err
	}
	return os.WriteFile(path, file, 0o644)
}

// writeCompactedStore writes a store state as a compacted file (Data+Bloom, no WAL).
func writeCompactedStore(path string, s *storeState) error {
	entries, err := materializeDataEntries(s)
	if err != nil {
		return err
	}
	data, err := encodeDataEntries(entries)
	if err != nil {
		return err
	}
	keys := make([][]byte, len(entries))
	for i, e := range entries {
		keys[i] = e.Key
	}
	file, err := encodeContainer(container{Data: data, Bloom: encodeBloom(keys), WAL: nil})
	if err != nil {
		return err
	}
	return os.WriteFile(path, file, 0o644)
}
