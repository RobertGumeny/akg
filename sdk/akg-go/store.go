package akg

import (
	"bytes"
	"errors"
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
}

// Open opens an existing store or creates a new empty store if path does not exist.
func Open(path string) (*Store, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		st := &Store{path: path, state: newStoreState(), nextWALSeq: 1}
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
	state, err := hydrateDataEntries(entries)
	if err != nil {
		return nil, err
	}
	committedWAL, nextSeq, err := inspectAndReplayWAL(state, c.WAL)
	if err != nil {
		return nil, err
	}
	return &Store{state: state, committedWAL: committedWAL, nextWALSeq: nextSeq}, nil
}

// PutNode writes or replaces the current live node for (typeName, id).
// Title is required. Tags are validated against the AKG tag rules and become
// the node's current tag set. If id is empty, a new node ID is generated.
func (s *Store) PutNode(typeName, id string, fields NodeFields, tags []string) (NodeRef, error) {
	if s == nil || s.closed {
		return NodeRef{}, errInvalidInput
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
		return nil, errInvalidInput
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
		return nil, errInvalidInput
	}
	if err := validateTag(tag); err != nil {
		return nil, err
	}
	matches := make([]nodeRecord, 0)
	for _, rec := range s.state.nodes {
		for _, nodeTag := range rec.Node.Tags {
			if nodeTag == tag {
				matches = append(matches, cloneNodeRecord(rec))
				break
			}
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
		return nil, errInvalidInput
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
// Both referenced nodes must already exist. Strength defaults to 0.5 if zero.
func (s *Store) PutEdge(fromRef NodeRef, relationValue string, toRef NodeRef, fields EdgeFields) error {
	if s == nil || s.closed {
		return errInvalidInput
	}
	if _, ok := s.state.nodes[nodeIdentity{typeName: fromRef.Type, id: nodeID(fromRef.ID)}]; !ok {
		return errNotFound
	}
	if _, ok := s.state.nodes[nodeIdentity{typeName: toRef.Type, id: nodeID(toRef.ID)}]; !ok {
		return errNotFound
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
		return nil, errInvalidInput
	}
	if _, err := buildNodeKey(nodeRef.Type, nodeID(nodeRef.ID)); err != nil {
		return nil, err
	}
	if relationValue != "" {
		if err := validateComponent(relationValue); err != nil {
			return nil, err
		}
	}
	matches := make([]coreEdge, 0)
	for _, rec := range s.state.edges {
		if rec.FromType != nodeRef.Type || rec.FromNode != nodeID(nodeRef.ID) {
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
		return nil, errInvalidInput
	}
	if _, err := buildNodeKey(nodeRef.Type, nodeID(nodeRef.ID)); err != nil {
		return nil, err
	}
	if relationValue != "" {
		if err := validateComponent(relationValue); err != nil {
			return nil, err
		}
	}
	matches := make([]coreEdge, 0)
	for _, rec := range s.state.edges {
		if rec.ToType != nodeRef.Type || rec.ToNode != nodeID(nodeRef.ID) {
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
// Returns errNotFound if the node does not exist.
// Returns errInvalidInput if the node has any live inbound or outbound edges.
func (s *Store) DeleteNode(typeName, id string) error {
	if s == nil || s.closed {
		return errInvalidInput
	}
	if _, err := buildNodeKey(typeName, nodeID(id)); err != nil {
		return err
	}
	return s.deleteNode(typeName, nodeID(id))
}

// DeleteEdge removes the edge identified by (fromRef, relation, toRef).
// Returns errNotFound if the edge does not exist.
func (s *Store) DeleteEdge(fromRef NodeRef, relationValue string, toRef NodeRef) error {
	if s == nil || s.closed {
		return errInvalidInput
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

// Commit durably writes pending mutations and a COMMIT record.
func (s *Store) Commit() error {
	if s == nil {
		return errInvalidInput
	}
	if s.closed {
		return errInvalidInput
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
	s.nextWALSeq = next + 1
	return s.writeFile()
}

// Close commits outstanding mutations.
func (s *Store) Close() error {
	if s == nil {
		return errInvalidInput
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

func (s *Store) writeFile() error {
	entries, err := materializeDataEntries(s.state)
	if err != nil {
		return err
	}
	data, err := encodeDataEntries(entries)
	if err != nil {
		return err
	}
	keys := make([][]byte, len(entries))
	for i, entry := range entries {
		keys[i] = entry.Key
	}
	walPayload, err := encodeWALRecords(s.committedWAL)
	if err != nil {
		return err
	}
	file, err := encodeContainer(container{Data: data, Bloom: encodeBloom(keys), WAL: walPayload})
	if err != nil {
		return err
	}
	return writeFileSync(s.path, file)
}

func (s *Store) putNode(id nodeID, n coreNode) (nodeRecord, error) {
	rec, err := s.state.putNode(id, n)
	if err != nil {
		return nodeRecord{}, err
	}
	payload, err := encodeNodePutPayload(nodePut{ID: rec.ID, Node: rec.Node})
	if err != nil {
		return nodeRecord{}, err
	}
	s.pending = append(s.pending, pendingWALRecord{op: walOpPutNode, payload: payload})
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
	return rec, nil
}

func (s *Store) deleteNode(typeName string, id nodeID) error {
	ident := nodeIdentity{typeName: typeName, id: id}
	if _, ok := s.state.nodes[ident]; !ok {
		return errNotFound
	}
	for _, edge := range s.state.edges {
		if (edge.FromType == typeName && edge.FromNode == id) || (edge.ToType == typeName && edge.ToNode == id) {
			return errInvalidInput
		}
	}
	delete(s.state.nodes, ident)
	payload, err := encodeNodeDeletePayload(nodeDelete{Type: typeName, ID: id})
	if err != nil {
		return err
	}
	s.pending = append(s.pending, pendingWALRecord{op: walOpDeleteNode, payload: payload})
	return nil
}

func (s *Store) deleteEdge(fromType string, fromNode nodeID, rel relation, toType string, toNode nodeID) error {
	ident := edgeIdentity{fromType: fromType, from: fromNode, relation: rel, toType: toType, to: toNode}
	if _, ok := s.state.edges[ident]; !ok {
		return errNotFound
	}
	delete(s.state.edges, ident)
	payload, err := encodeEdgeDeletePayload(edgeDelete{FromType: fromType, FromNode: fromNode, Relation: rel, ToType: toType, ToNode: toNode})
	if err != nil {
		return err
	}
	s.pending = append(s.pending, pendingWALRecord{op: walOpDeleteEdge, payload: payload})
	return nil
}

func hydrateDataEntries(entries []dataEntry) (*storeState, error) {
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
				return nil, errInvalidDataPayload
			}
			if node.Type != parsed.Type {
				return nil, errInvalidInput
			}
			if err := s.loadNodeRecord(nodeRecord{ID: parsed.ID, Node: node}); err != nil {
				return nil, err
			}
		case strings.HasPrefix(string(key), "t:"):
			if len(entry.Value) != 0 {
				return nil, errInvalidInput
			}
			if _, _, err := parseTagKey(key); err != nil {
				return nil, err
			}
		case strings.HasPrefix(string(key), "ts:"):
			if len(entry.Value) != 0 {
				return nil, errInvalidInput
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
				return nil, errInvalidDataPayload
			}
			if edge.FromType != parsed.FromType || edge.FromNode != parsed.FromNode || edge.Relation != parsed.Relation || edge.ToType != parsed.ToType || edge.ToNode != parsed.ToNode {
				return nil, errInvalidInput
			}
			if err := s.loadEdgeRecord(edge); err != nil {
				return nil, err
			}
		case strings.HasPrefix(string(key), "ei:"):
			if len(entry.Value) != 0 {
				return nil, errInvalidInput
			}
			if _, err := parseEdgeIndexKey(key); err != nil {
				return nil, err
			}
		default:
			return nil, errInvalidInput
		}
	}
	if err := validateDerivedKeys(s, entries); err != nil {
		return nil, err
	}
	return s, nil
}

func validateDerivedKeys(s *storeState, entries []dataEntry) error {
	expected, err := materializeDataEntries(s)
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

func materializeDataEntries(s *storeState) ([]dataEntry, error) {
	if s == nil {
		return nil, errInvalidInput
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
			tagKey, err := buildTagKey(tag, node.ID)
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
			return nil, 0, errInvalidWALPayload
		}
		switch r.Operation {
		case walOpPutNode:
			put, err := decodeNodePutPayload(r.Payload)
			if err != nil {
				return nil, 0, err
			}
			if err := state.loadNodeRecord(nodeRecord{ID: put.ID, Node: put.Node}); err != nil {
				return nil, 0, err
			}
		case walOpDeleteNode:
			d, err := decodeNodeDeletePayload(r.Payload)
			if err != nil {
				return nil, 0, err
			}
			delete(state.nodes, nodeIdentity{typeName: d.Type, id: d.ID})
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
			delete(state.edges, edgeIdentity{fromType: d.FromType, from: d.FromNode, relation: d.Relation, toType: d.ToType, to: d.ToNode})
		}
	}
	return committed, next, nil
}

func writeFileSync(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
	if err != nil {
		return err
	}
	n, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if n != len(data) {
		return errors.New("short file write")
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}
