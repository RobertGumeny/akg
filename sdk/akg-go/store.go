package akg

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	errDerivedIndexMismatch = errors.New("derived index mismatch")
	errInvalidWALPayload    = errors.New("invalid wal payload")
	errInvalidDataPayload   = errors.New("invalid data payload")
)

// Writer-side flush thresholds (spec docs/spec/05-wal.md:116-121, matching the
// Go reference internal/store/file.go:16-17 and the TypeScript SDK). The first
// threshold reached wins. This is a durability safety valve, not a compaction
// trigger: an auto-flush appends a COMMIT exactly as a manual Commit would and
// never rewrites Data/Bloom or discards WAL history.
const (
	walEntryFlushThreshold = 1000
	walByteFlushThreshold  = 10 * 1024 * 1024
	// walRecordOverhead is the per-record WAL framing (13-byte header + 4-byte
	// trailing CRC) used to estimate pending byte growth for the flush policy.
	walRecordOverhead = 13 + 4
)

type pendingWALRecord struct {
	op      walOperation
	payload []byte
}

// Store is an opened AKG store bound to one filesystem path.
type Store struct {
	path         string
	state        *storeState
	pending      []pendingWALRecord
	committedWAL []walRecord
	nextWALSeq   walSequenceNumber
	closed       bool
	// baseData and baseBloom are the compaction-baseline Data and Bloom sections
	// exactly as they sit on disk. A commit leaves them untouched and only grows
	// the WAL (logical append — the reference SDK and akg-ts do the same); only
	// Compact re-materializes them from live state. Re-materializing Data on every
	// commit (the prior behavior) wrote the whole Data section each time and
	// recorded mutations twice (Data + WAL); appending writes only the new WAL
	// records.
	baseData  []byte
	baseBloom []byte
	// pendingBytes estimates the persisted size of buffered (uncommitted)
	// mutations; uncompactedWALBytes is the persisted size of the committed WAL
	// after the most recent write. Together they drive the auto-flush policy.
	pendingBytes        int
	uncompactedWALBytes int
}

// Open opens an existing store or creates a new empty store if path does not exist.
func Open(path string) (*Store, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		emptyData, err := encodeDataEntries(nil)
		if err != nil {
			return nil, err
		}
		st := &Store{path: path, state: newStoreState(), nextWALSeq: 1, baseData: emptyData, baseBloom: encodeBloom(nil)}
		if err := st.writeFile(); err != nil {
			return nil, err
		}
		return st, nil
	} else if err != nil {
		return nil, err
	}
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	st, err := openBytes(file)
	if err != nil {
		return nil, err
	}
	st.path = path
	return st, nil
}

func openBytes(file []byte) (*Store, error) {
	c, err := decodeContainer(file)
	if err != nil {
		return nil, err
	}
	entries, err := decodeDataEntries(c.Data)
	if err != nil {
		return nil, err
	}
	if c.Bloom != nil {
		if err := decodeBloom(c.Bloom); err != nil {
			return nil, err
		}
		keys := make([][]byte, len(entries))
		for i, entry := range entries {
			keys[i] = entry.Key
		}
		if want := encodeBloom(keys); !bytes.Equal(c.Bloom, want) {
			return nil, errInvalidBloomSection
		}
	}
	state, err := hydrateDataEntries(entries, c.Major)
	if err != nil {
		return nil, err
	}
	committedWAL, nextSeq, err := inspectAndReplayWAL(state, c.WAL)
	if err != nil {
		return nil, err
	}
	return &Store{
		state:               state,
		committedWAL:        committedWAL,
		nextWALSeq:          nextSeq,
		uncompactedWALBytes: len(c.WAL),
		baseData:            c.Data,
		baseBloom:           c.Bloom,
	}, nil
}

// OpenBytes opens an AKG store from in-memory bytes. The store is fully
// readable but has no backing file — calling Commit or Close with pending
// mutations will fail. Use it for read-only access to embedded or in-memory
// .akg data. Close is safe to call when no mutations are pending.
func OpenBytes(data []byte) (*Store, error) {
	return openBytes(data)
}

// PutNode writes or replaces the current live node for (typeName, id).
// Title is required. Tags are validated against the AKG tag rules and become
// the node's current tag set. If id is empty, a new node ID is generated.
func (s *Store) PutNode(typeName, id string, fields NodeFields, tags []string) (NodeRef, error) {
	if s == nil || s.closed {
		return NodeRef{}, ErrInvalidInput
	}
	n, err := coreNodeFromFields(typeName, fields, tags)
	if err != nil {
		return NodeRef{}, err
	}
	rec, err := s.putNode(nodeID(id), n)
	if err != nil {
		return NodeRef{}, err
	}
	return NodeRef{Type: rec.Node.Type, ID: string(rec.ID)}, nil
}

// GetNode returns the current live node for (typeName, id), or nil if missing.
func (s *Store) GetNode(typeName, id string) (*Node, error) {
	if s == nil || s.closed {
		return nil, ErrInvalidInput
	}
	if _, err := buildNodeKey(typeName, nodeID(id)); err != nil {
		return nil, err
	}
	rec, ok := s.state.nodes[nodeIdentity{typeName: typeName, id: nodeID(id)}]
	if !ok {
		return nil, nil
	}
	return nodeFromRecord(rec), nil
}

// ListNodesByTag returns all current live nodes carrying tag.
func (s *Store) ListNodesByTag(tag string) ([]Node, error) {
	if s == nil || s.closed {
		return nil, ErrInvalidInput
	}
	if err := validateTag(tag); err != nil {
		return nil, err
	}
	// Index lookup: O(nodes carrying tag), not a full O(total nodes) scan.
	matches := make([]nodeRecord, 0, len(s.state.tagIndex[tag]))
	for ident := range s.state.tagIndex[tag] {
		if rec, ok := s.state.nodes[ident]; ok {
			matches = append(matches, cloneNodeRecord(rec))
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		ik, _ := buildNodeKey(matches[i].Node.Type, matches[i].ID)
		jk, _ := buildNodeKey(matches[j].Node.Type, matches[j].ID)
		return bytes.Compare(ik, jk) < 0
	})
	nodes := make([]Node, len(matches))
	for i, rec := range matches {
		nodes[i] = *nodeFromRecord(rec)
	}
	return nodes, nil
}

// ListNodes returns all current live nodes. If typeName is non-empty, only nodes of
// that type are returned. Results are sorted by node key. An unknown type returns an
// empty slice and nil error.
func (s *Store) ListNodes(typeName string) ([]Node, error) {
	if s == nil || s.closed {
		return nil, ErrInvalidInput
	}
	if typeName != "" {
		if err := validateComponent(typeName); err != nil {
			return nil, err
		}
	}
	matches := make([]nodeRecord, 0)
	for _, rec := range s.state.nodes {
		if typeName != "" && rec.Node.Type != typeName {
			continue
		}
		matches = append(matches, cloneNodeRecord(rec))
	}
	sort.Slice(matches, func(i, j int) bool {
		ik, _ := buildNodeKey(matches[i].Node.Type, matches[i].ID)
		jk, _ := buildNodeKey(matches[j].Node.Type, matches[j].ID)
		return bytes.Compare(ik, jk) < 0
	})
	nodes := make([]Node, len(matches))
	for i, rec := range matches {
		nodes[i] = *nodeFromRecord(rec)
	}
	return nodes, nil
}

// PutEdge writes or replaces the current live edge for (fromRef, relation, toRef).
// Both referenced nodes must already exist.
func (s *Store) PutEdge(fromRef NodeRef, relationValue string, toRef NodeRef, fields EdgeFields) error {
	if s == nil || s.closed {
		return ErrInvalidInput
	}
	if _, ok := s.state.nodes[nodeIdentity{typeName: fromRef.Type, id: nodeID(fromRef.ID)}]; !ok {
		return ErrNotFound
	}
	if _, ok := s.state.nodes[nodeIdentity{typeName: toRef.Type, id: nodeID(toRef.ID)}]; !ok {
		return ErrNotFound
	}
	e, err := coreEdgeFromFields(fromRef, relationValue, toRef, fields)
	if err != nil {
		return err
	}
	_, err = s.putEdge(e)
	return err
}

// OutboundEdges returns all current live outbound edges for nodeRef.
// If relationValue is non-empty, results are filtered to that relation.
func (s *Store) OutboundEdges(nodeRef NodeRef, relationValue string) ([]Edge, error) {
	if s == nil || s.closed {
		return nil, ErrInvalidInput
	}
	if _, err := buildNodeKey(nodeRef.Type, nodeID(nodeRef.ID)); err != nil {
		return nil, err
	}
	if relationValue != "" {
		if err := validateComponent(relationValue); err != nil {
			return nil, err
		}
	}
	// Index lookup: O(out-degree of nodeRef), not a full O(total edges) scan.
	from := nodeIdentity{typeName: nodeRef.Type, id: nodeID(nodeRef.ID)}
	matches := make([]coreEdge, 0, len(s.state.outIndex[from]))
	for eid := range s.state.outIndex[from] {
		rec, ok := s.state.edges[eid]
		if !ok {
			continue
		}
		if relationValue != "" && rec.Relation != relation(relationValue) {
			continue
		}
		matches = append(matches, cloneEdge(rec))
	}
	sort.Slice(matches, func(i, j int) bool {
		ik, _ := buildEdgeKey(matches[i].FromType, matches[i].FromNode, matches[i].Relation, matches[i].ToType, matches[i].ToNode)
		jk, _ := buildEdgeKey(matches[j].FromType, matches[j].FromNode, matches[j].Relation, matches[j].ToType, matches[j].ToNode)
		return bytes.Compare(ik, jk) < 0
	})
	edges := make([]Edge, len(matches))
	for i, rec := range matches {
		edges[i] = *edgeFromRecord(rec)
	}
	return edges, nil
}

// InboundEdges returns all current live inbound edges for nodeRef.
// If relationValue is non-empty, results are filtered to that relation.
func (s *Store) InboundEdges(nodeRef NodeRef, relationValue string) ([]Edge, error) {
	if s == nil || s.closed {
		return nil, ErrInvalidInput
	}
	if _, err := buildNodeKey(nodeRef.Type, nodeID(nodeRef.ID)); err != nil {
		return nil, err
	}
	if relationValue != "" {
		if err := validateComponent(relationValue); err != nil {
			return nil, err
		}
	}
	// Index lookup: O(in-degree of nodeRef), not a full O(total edges) scan.
	to := nodeIdentity{typeName: nodeRef.Type, id: nodeID(nodeRef.ID)}
	matches := make([]coreEdge, 0, len(s.state.inIndex[to]))
	for eid := range s.state.inIndex[to] {
		rec, ok := s.state.edges[eid]
		if !ok {
			continue
		}
		if relationValue != "" && rec.Relation != relation(relationValue) {
			continue
		}
		matches = append(matches, cloneEdge(rec))
	}
	sort.Slice(matches, func(i, j int) bool {
		ik, _ := buildEdgeIndexKey(matches[i].ToType, matches[i].ToNode, matches[i].Relation, matches[i].FromType, matches[i].FromNode)
		jk, _ := buildEdgeIndexKey(matches[j].ToType, matches[j].ToNode, matches[j].Relation, matches[j].FromType, matches[j].FromNode)
		return bytes.Compare(ik, jk) < 0
	})
	edges := make([]Edge, len(matches))
	for i, rec := range matches {
		edges[i] = *edgeFromRecord(rec)
	}
	return edges, nil
}

// DeleteNode removes the node identified by (typeName, id).
// Returns ErrNotFound if the node does not exist.
// Returns ErrInvalidInput if the node has any live inbound or outbound edges.
func (s *Store) DeleteNode(typeName, id string) error {
	if s == nil || s.closed {
		return ErrInvalidInput
	}
	if _, err := buildNodeKey(typeName, nodeID(id)); err != nil {
		return err
	}
	return s.deleteNode(typeName, nodeID(id))
}

// DeleteEdge removes the edge identified by (fromRef, relation, toRef).
// Returns ErrNotFound if the edge does not exist.
func (s *Store) DeleteEdge(fromRef NodeRef, relationValue string, toRef NodeRef) error {
	if s == nil || s.closed {
		return ErrInvalidInput
	}
	if _, err := buildNodeKey(fromRef.Type, nodeID(fromRef.ID)); err != nil {
		return err
	}
	if _, err := buildNodeKey(toRef.Type, nodeID(toRef.ID)); err != nil {
		return err
	}
	if err := validateComponent(relationValue); err != nil {
		return err
	}
	return s.deleteEdge(fromRef.Type, nodeID(fromRef.ID), relation(relationValue), toRef.Type, nodeID(toRef.ID))
}

// Commit durably writes all pending mutations to disk, followed by a COMMIT
// record. It is a no-op (returns nil) when there are no pending mutations —
// calling it on a store with nothing pending is safe and does not alter state.
func (s *Store) Commit() error {
	if s == nil {
		return ErrInvalidInput
	}
	if s.closed {
		return ErrInvalidInput
	}
	if len(s.pending) == 0 {
		return nil
	}
	records := make([]walRecord, 0, len(s.pending)+1)
	next := s.nextWALSeq
	for _, p := range s.pending {
		records = append(records, walRecord{Sequence: next, Operation: p.op, Payload: append([]byte(nil), p.payload...)})
		next++
	}
	records = append(records, walRecord{Sequence: next, Operation: walOpCommit})
	s.committedWAL = append(s.committedWAL, records...)
	s.pending = nil
	s.pendingBytes = 0
	s.nextWALSeq = next + 1
	return s.writeFile()
}

// maybeAutoFlush commits buffered mutations when the buffered-plus-uncompacted
// WAL crosses a flush threshold. It is the writer-side safety valve described in
// docs/spec/05-wal.md: it appends a COMMIT exactly as Commit would and never
// compacts. Stores opened from bytes (no backing path) and closed stores never
// auto-flush.
func (s *Store) maybeAutoFlush() error {
	if s.path == "" || s.closed {
		return nil
	}
	if !s.shouldAutoFlush() {
		return nil
	}
	return s.Commit()
}

func (s *Store) shouldAutoFlush() bool {
	entries := len(s.committedWAL) + len(s.pending)
	byteCount := s.uncompactedWALBytes + s.pendingBytes
	return entries >= walEntryFlushThreshold || byteCount >= walByteFlushThreshold
}

// UncompactedWALEntries reports the number of committed WAL records accumulated
// since the last compaction. It mirrors the input to the auto-flush policy and
// resets to zero on Compact.
func (s *Store) UncompactedWALEntries() int {
	if s == nil {
		return 0
	}
	return len(s.committedWAL)
}

// UncompactedWALBytes reports the persisted size in bytes of the committed WAL
// accumulated since the last compaction. It resets to zero on Compact.
func (s *Store) UncompactedWALBytes() int {
	if s == nil {
		return 0
	}
	return s.uncompactedWALBytes
}

// Close commits any pending mutations and then marks the store as closed.
// Pending mutations are committed exactly as if Commit had been called — they
// are durable after Close returns. Calling Close on an already-closed store is
// a no-op: it returns nil and leaves the store unchanged.
func (s *Store) Close() error {
	if s == nil {
		return ErrInvalidInput
	}
	if s.closed {
		return nil
	}
	if err := s.Commit(); err != nil {
		return err
	}
	s.closed = true
	return nil
}

// writeFile persists the store by logical append: the compaction-baseline Data
// and Bloom sections are written back unchanged and only the committed WAL grows.
// It does NOT re-materialize Data from live state — that is Compact's job. The
// write itself is crash-atomic (temp + fsync + rename). This matches the
// reference SDK (internal/store) and akg-ts byte-for-byte.
func (s *Store) writeFile() error {
	walPayload, err := encodeWALRecords(s.committedWAL)
	if err != nil {
		return err
	}
	file, err := encodeContainer(container{Data: s.baseData, Bloom: s.baseBloom, WAL: walPayload})
	if err != nil {
		return err
	}
	if err := writeFileAtomicRename(s.path, file); err != nil {
		return err
	}
	s.uncompactedWALBytes = len(walPayload)
	return nil
}

func (s *Store) putNode(id nodeID, n coreNode) (nodeRecord, error) {
	rec, err := s.state.putNode(id, n)
	if err != nil {
		return nodeRecord{}, err
	}
	payload, err := encodeNodePutPayload(nodePut(rec))
	if err != nil {
		return nodeRecord{}, err
	}
	s.pending = append(s.pending, pendingWALRecord{op: walOpPutNode, payload: payload})
	s.pendingBytes += len(payload) + walRecordOverhead
	if err := s.maybeAutoFlush(); err != nil {
		return nodeRecord{}, err
	}
	return rec, nil
}

func (s *Store) putEdge(e coreEdge) (coreEdge, error) {
	rec, err := s.state.putEdge(e)
	if err != nil {
		return coreEdge{}, err
	}
	payload, err := encodeEdgePutPayload(edgePut{Edge: rec})
	if err != nil {
		return coreEdge{}, err
	}
	s.pending = append(s.pending, pendingWALRecord{op: walOpPutEdge, payload: payload})
	s.pendingBytes += len(payload) + walRecordOverhead
	if err := s.maybeAutoFlush(); err != nil {
		return coreEdge{}, err
	}
	return rec, nil
}

func (s *Store) deleteNode(typeName string, id nodeID) error {
	ident := nodeIdentity{typeName: typeName, id: id}
	rec, ok := s.state.nodes[ident]
	if !ok {
		return ErrNotFound
	}
	if s.state.hasIncidentEdges(ident) {
		return ErrInvalidInput
	}
	delete(s.state.nodes, ident)
	s.state.indexRemoveTags(ident, rec.Node.Tags)
	payload, err := encodeNodeDeletePayload(nodeDelete{Type: typeName, ID: id})
	if err != nil {
		return err
	}
	s.pending = append(s.pending, pendingWALRecord{op: walOpDeleteNode, payload: payload})
	s.pendingBytes += len(payload) + walRecordOverhead
	return s.maybeAutoFlush()
}

func (s *Store) deleteEdge(fromType string, fromNode nodeID, rel relation, toType string, toNode nodeID) error {
	ident := edgeIdentity{fromType: fromType, from: fromNode, relation: rel, toType: toType, to: toNode}
	if _, ok := s.state.edges[ident]; !ok {
		return ErrNotFound
	}
	delete(s.state.edges, ident)
	s.state.indexRemoveEdge(ident)
	payload, err := encodeEdgeDeletePayload(edgeDelete{FromType: fromType, FromNode: fromNode, Relation: rel, ToType: toType, ToNode: toNode})
	if err != nil {
		return err
	}
	s.pending = append(s.pending, pendingWALRecord{op: walOpDeleteEdge, payload: payload})
	s.pendingBytes += len(payload) + walRecordOverhead
	return s.maybeAutoFlush()
}

// hydrateDataEntries reconstructs live state from decoded Data entries. major is
// the file's binary major; it selects the tag-key shape the derived index is
// validated against, so a major-1 file (3-part tag keys) is checked against a
// major-1 materialization while major-2 files are checked strictly.
func hydrateDataEntries(entries []dataEntry, major uint8) (*storeState, error) {
	for i := 1; i < len(entries); i++ {
		cmp := bytes.Compare(entries[i-1].Key, entries[i].Key)
		if cmp == 0 {
			return nil, errDuplicateDataKey
		}
		if cmp > 0 {
			return nil, errInvalidDataSection
		}
	}
	s := newStoreState()
	for _, entry := range entries {
		key := entry.Key
		switch {
		case strings.HasPrefix(string(key), "n:"):
			parsed, err := parseNodeKey(key)
			if err != nil {
				return nil, err
			}
			node, err := decodeNodePayload(entry.Value)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", errInvalidDataPayload, err)
			}
			if node.Type != parsed.Type {
				return nil, ErrInvalidInput
			}
			if err := s.loadNodeRecord(nodeRecord{ID: parsed.ID, Node: node}); err != nil {
				return nil, err
			}
		case strings.HasPrefix(string(key), "t:"):
			if len(entry.Value) != 0 {
				return nil, ErrInvalidInput
			}
			if _, _, _, err := parseTagKey(key); err != nil {
				return nil, err
			}
		case strings.HasPrefix(string(key), "ts:"):
			if len(entry.Value) != 0 {
				return nil, ErrInvalidInput
			}
			if err := parseTemporalKey(key); err != nil {
				return nil, err
			}
		case strings.HasPrefix(string(key), "e:"):
			parsed, err := parseEdgeKey(key)
			if err != nil {
				return nil, err
			}
			edge, err := decodeEdgePayload(entry.Value)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", errInvalidDataPayload, err)
			}
			if edge.FromType != parsed.FromType || edge.FromNode != parsed.FromNode || edge.Relation != parsed.Relation || edge.ToType != parsed.ToType || edge.ToNode != parsed.ToNode {
				return nil, ErrInvalidInput
			}
			if err := s.loadEdgeRecord(edge); err != nil {
				return nil, err
			}
		case strings.HasPrefix(string(key), "ei:"):
			if len(entry.Value) != 0 {
				return nil, ErrInvalidInput
			}
			if _, err := parseEdgeIndexKey(key); err != nil {
				return nil, err
			}
		default:
			return nil, ErrInvalidInput
		}
	}
	if err := validateDerivedKeys(s, entries, major); err != nil {
		return nil, err
	}
	return s, nil
}

func validateDerivedKeys(s *storeState, entries []dataEntry, major uint8) error {
	expected, err := materializeDataEntriesForMajor(s, major)
	if err != nil {
		return err
	}
	if len(expected) != len(entries) {
		return errDerivedIndexMismatch
	}
	for i := range expected {
		if !bytes.Equal(expected[i].Key, entries[i].Key) {
			return errDerivedIndexMismatch
		}
	}
	return nil
}

// materializeDataEntries derives the live Data key set at the current binary
// major (always major 2 — type-qualified tag keys). Writers use this, so a
// compacted file's tag index is always type-qualified.
func materializeDataEntries(s *storeState) ([]dataEntry, error) {
	return materializeDataEntriesForMajor(s, currentMajor)
}

// materializeDataEntriesForMajor derives the live Data key set as it must appear
// in a file of the given binary major. major selects the tag-key shape: major 1
// emits legacy t:{tag}:{id} keys, major 2 the type-qualified t:{tag}:{type}:{id}.
// Read-side validation passes the file's own major so a major-1 file's 3-part
// tag keys validate against a major-1 materialization.
func materializeDataEntriesForMajor(s *storeState, major uint8) ([]dataEntry, error) {
	if s == nil {
		return nil, ErrInvalidInput
	}
	var entries []dataEntry
	seen := map[string]struct{}{}
	add := func(key, value []byte) error {
		k := string(key)
		if _, ok := seen[k]; ok {
			return errDuplicateDataKey
		}
		seen[k] = struct{}{}
		entries = append(entries, dataEntry{Key: append([]byte(nil), key...), Value: append([]byte(nil), value...)})
		return nil
	}
	for _, node := range s.nodes {
		key, err := buildNodeKey(node.Node.Type, node.ID)
		if err != nil {
			return nil, err
		}
		value, err := encodeNodePayload(node.Node)
		if err != nil {
			return nil, err
		}
		if err := add(key, value); err != nil {
			return nil, err
		}
		for _, tag := range node.Node.Tags {
			tagKey, err := buildTagKeyForMajor(major, tag, node.Node.Type, node.ID)
			if err != nil {
				return nil, err
			}
			if err := add(tagKey, nil); err != nil {
				return nil, err
			}
		}
		temporalKey, err := buildTemporalNodeKey(node.Node.UpdatedAt, node.Node.Type, node.ID)
		if err != nil {
			return nil, err
		}
		if err := add(temporalKey, nil); err != nil {
			return nil, err
		}
	}
	for _, edge := range s.edges {
		key, err := buildEdgeKey(edge.FromType, edge.FromNode, edge.Relation, edge.ToType, edge.ToNode)
		if err != nil {
			return nil, err
		}
		value, err := encodeEdgePayload(edge)
		if err != nil {
			return nil, err
		}
		if err := add(key, value); err != nil {
			return nil, err
		}
		inboundKey, err := buildEdgeIndexKey(edge.ToType, edge.ToNode, edge.Relation, edge.FromType, edge.FromNode)
		if err != nil {
			return nil, err
		}
		if err := add(inboundKey, nil); err != nil {
			return nil, err
		}
		temporalKey, err := buildTemporalEdgeKey(edge.UpdatedAt, edge.FromType, edge.FromNode, edge.Relation, edge.ToType, edge.ToNode)
		if err != nil {
			return nil, err
		}
		if err := add(temporalKey, nil); err != nil {
			return nil, err
		}
	}
	sort.Slice(entries, func(i, j int) bool { return bytes.Compare(entries[i].Key, entries[j].Key) < 0 })
	return entries, nil
}

func inspectAndReplayWAL(state *storeState, payload []byte) ([]walRecord, walSequenceNumber, error) {
	next := walSequenceNumber(1)
	var all []walRecord
	lastCommit := -1
	for len(payload) > 0 {
		r, n, err := decodeWALRecord(payload)
		if err != nil {
			if lastCommit >= 0 {
				break
			}
			return nil, 0, err
		}
		all = append(all, r)
		if r.Sequence >= next {
			next = r.Sequence + 1
		}
		if r.Operation == walOpCommit {
			lastCommit = len(all) - 1
		}
		payload = payload[n:]
	}
	if lastCommit < 0 {
		return nil, next, nil
	}
	var prev walSequenceNumber
	committed := append([]walRecord(nil), all[:lastCommit+1]...)
	for i, r := range committed {
		if i > 0 && r.Sequence <= prev {
			return nil, 0, errInvalidWALRecord
		}
		prev = r.Sequence
		if err := validateWALPayload(r); err != nil {
			return nil, 0, fmt.Errorf("%w: %w", errInvalidWALPayload, err)
		}
		switch r.Operation {
		case walOpPutNode:
			put, err := decodeNodePutPayload(r.Payload)
			if err != nil {
				return nil, 0, err
			}
			if err := state.loadNodeRecord(nodeRecord(put)); err != nil {
				return nil, 0, err
			}
		case walOpDeleteNode:
			d, err := decodeNodeDeletePayload(r.Payload)
			if err != nil {
				return nil, 0, err
			}
			ident := nodeIdentity{typeName: d.Type, id: d.ID}
			if existing, ok := state.nodes[ident]; ok {
				state.indexRemoveTags(ident, existing.Node.Tags)
			}
			delete(state.nodes, ident)
		case walOpPutEdge:
			put, err := decodeEdgePutPayload(r.Payload)
			if err != nil {
				return nil, 0, err
			}
			if err := state.loadEdgeRecord(put.Edge); err != nil {
				return nil, 0, err
			}
		case walOpDeleteEdge:
			d, err := decodeEdgeDeletePayload(r.Payload)
			if err != nil {
				return nil, 0, err
			}
			eident := edgeIdentity{fromType: d.FromType, from: d.FromNode, relation: d.Relation, toType: d.ToType, to: d.ToNode}
			delete(state.edges, eident)
			state.indexRemoveEdge(eident)
		}
	}
	return committed, next, nil
}
