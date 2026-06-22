package akg

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		{"m2-full-node.akg", genFixtureFullNode},
		{"m2-collision-type-qualified.akg", genFixtureCollisionTypeQualified},
		{"m2-committed-wal-replay.akg", genFixtureCommittedWALReplay},
		{"m2-uncommitted-wal-tail.akg", genFixtureUncommittedWALTail},
		{"m2-deletes-before-compaction.akg", genFixtureDeletesBeforeCompaction},
		{"m2-node-deleted-before-commit.akg", genFixtureNodeDeletedBeforeCommit},
		{"m2-node-deletion-survives-reopen.akg", genFixtureNodeDeletionSurvivesReopen},
		{"m2-edge-deleted-before-commit.akg", genFixtureEdgeDeletedBeforeCommit},
		{"m2-edge-deletion-survives-reopen.akg", genFixtureEdgeDeletionSurvivesReopen},
		{"m2-reject-derived-index-mismatch.akg", genFixtureRejectDerivedIndexMismatch},
		{"m2-reject-malformed-committed-wal.akg", genFixtureRejectMalformedCommittedWAL},
		{"m2-utf8-keys.akg", genFixtureUTF8Keys},
		{"m3-reject-oversize-type-key.akg", genFixtureRejectOversizeTypeKey},
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

// genFixtureFullNode: one compacted node populated with a body, tags, and
// metadata. Exercises the full node payload plus the type-qualified tag index
// (major 2: t:{tag}:{type}:{id}).
func genFixtureFullNode(path string) error {
	const ts = timestampMicros(6_000_000)
	s := newStoreState()
	s.now = func() timestampMicros { return ts }
	if _, err := s.putNode("full1", coreNode{
		Type:  "note",
		Title: "Full Node",
		Body:  "A populated node with a body, tags, and metadata.",
		Tags:  []string{"alpha", "beta"},
		Meta:  map[string]any{"author": "ada", "priority": 1},
	}); err != nil {
		return err
	}
	return writeCompactedStore(path, s)
}

// genFixtureCollisionTypeQualified: the major-2 regression lock for the tag-index
// key collision. Two nodes share the id "shared" across types (person, project)
// and both carry the tag "topic". Under the major-1 key shape (t:{tag}:{id}) both
// collapsed to the identical tag key and broke compaction; the major-2
// type-qualified key (t:{tag}:{type}:{id}) keeps them distinct, so this file
// materializes and reads clean. A conformant reader MUST accept it.
func genFixtureCollisionTypeQualified(path string) error {
	const ts = timestampMicros(7_000_000)
	s := newStoreState()
	s.now = func() timestampMicros { return ts }
	if _, err := s.putNode("shared", coreNode{Type: "person", Title: "Ada", Tags: []string{"topic"}}); err != nil {
		return err
	}
	if _, err := s.putNode("shared", coreNode{Type: "project", Title: "Atlas", Tags: []string{"topic"}}); err != nil {
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
	if err := st.PutEdge(NodeRef{Type: "note", ID: "a1"}, "links_to", NodeRef{Type: "note", ID: "a2"}, EdgeFields{Strength: StrengthOf(0.8)}); err != nil {
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

// genFixtureNodeDeletedBeforeCommit: PutNode + DeleteNode in a single commit.
// WAL: PUT_NODE@1, DELETE_NODE@2, COMMIT@3 → nextWALSeq=4
func genFixtureNodeDeletedBeforeCommit(path string) error {
	_ = os.Remove(path)
	st, err := Open(path)
	if err != nil {
		return err
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "hello"}, nil); err != nil {
		return err
	}
	if err := st.DeleteNode("note", "n1"); err != nil {
		return err
	}
	return st.Close()
}

// genFixtureNodeDeletionSurvivesReopen: PutNode committed, then DeleteNode in a second commit.
// WAL: PUT_NODE@1, COMMIT@2, DELETE_NODE@3, COMMIT@4 → nextWALSeq=5
func genFixtureNodeDeletionSurvivesReopen(path string) error {
	_ = os.Remove(path)
	st, err := Open(path)
	if err != nil {
		return err
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "hello"}, nil); err != nil {
		return err
	}
	if err := st.Close(); err != nil {
		return err
	}

	st2, err := Open(path)
	if err != nil {
		return err
	}
	if err := st2.DeleteNode("note", "n1"); err != nil {
		return err
	}
	return st2.Close()
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

// genFixtureUTF8Keys: compacted graph proving CONF-1/CONF-2 — type, relation, and
// tag are any key-safe UTF-8 string (no snake_case rule), each capped at 64 bytes.
// Uses an uppercase type (Person), an uppercase relation (KNOWS), and non-ASCII
// tags (café), plus a node whose type sits exactly at the 64-byte cap. A
// conformant reader MUST accept all of these. (Fails against the pre-CONF-1
// snake_case-only validators, which rejected uppercase/non-ASCII keys on load.)
func genFixtureUTF8Keys(path string) error {
	const ts = timestampMicros(4_000_000)
	s := newStoreState()
	s.now = func() timestampMicros { return ts }
	if _, err := s.putNode("ada", coreNode{Type: "Person", Title: "Ada", Tags: []string{"café", "Active"}}); err != nil {
		return err
	}
	if _, err := s.putNode("bob", coreNode{Type: "Person", Title: "Bob"}); err != nil {
		return err
	}
	// A node whose type is exactly 64 bytes — at the cap, must be accepted.
	if _, err := s.putNode("edge-case", coreNode{Type: strings.Repeat("a", 64), Title: "AtCap"}); err != nil {
		return err
	}
	if _, err := s.putEdge(coreEdge{FromType: "Person", FromNode: "ada", Relation: "KNOWS", ToType: "Person", ToNode: "bob"}); err != nil {
		return err
	}
	return writeCompactedStore(path, s)
}

// genFixtureRejectOversizeTypeKey: a Data section whose primary node key carries a
// 65-byte type — one over the 64-byte cap (CONF-2). A conformant reader MUST reject
// it on load. Built by materializing a valid at-cap (64-byte type) store, then
// extending the type by one byte in every key that embeds it, so only the byte cap
// (not an identity/derived-index mismatch) is what fails. Fails against the
// pre-CONF-2 behavior, which had no length cap on type and would accept this file.
func genFixtureRejectOversizeTypeKey(path string) error {
	const ts = timestampMicros(5_000_000)
	atCap := strings.Repeat("a", 64) // valid: exactly at the cap
	overCap := strings.Repeat("a", 65)
	s := newStoreState()
	s.now = func() timestampMicros { return ts }
	if _, err := s.putNode("x", coreNode{Type: atCap, Title: "over"}); err != nil {
		return err
	}
	entries, err := materializeDataEntries(s)
	if err != nil {
		return err
	}
	// Push the type one byte over the cap in every key that embeds it. The id ("x")
	// and the numeric timestamp contain no 'a' run, so this replacement is
	// unambiguous and preserves byte-ordering of the entry set.
	for i := range entries {
		entries[i].Key = bytes.Replace(entries[i].Key, []byte(atCap), []byte(overCap), 1)
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
