// Package akg exposes the intentionally minimal v1 reference API for AKG files.
//
// The public surface is limited to ordinary create/open/validate, current
// logical node/edge mutation, exact lookup, whole-state listing, commit, close,
// and explicit compaction. Tag lookup, inbound/outbound edge scans, WAL
// internals, derived index mutation, recovery/salvage, query planning,
// traversal, merge, and flush controls remain internal for v1.
package akg

import (
	"github.com/RobertGumeny/akg/internal/record"
	"github.com/RobertGumeny/akg/internal/state"
	"github.com/RobertGumeny/akg/internal/store"
)

// Node is the public current-state node payload. The node ID is carried by
// NodeRecord, not by the payload.
type Node struct {
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Body      string         `json:"body,omitempty"`
	Meta      map[string]any `json:"meta,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	CreatedAt uint64         `json:"created_at"`
	UpdatedAt uint64         `json:"updated_at"`
	Version   uint32         `json:"version"`
}

// NodeRecord pairs a node payload with its key-space identity.
type NodeRecord struct {
	ID   string `json:"id"`
	Node Node   `json:"node"`
}

// Edge is the public current-state edge payload. Its identity is
// (from_node_type, from_node, relation, to_node_type, to_node).
type Edge struct {
	FromType   string         `json:"from_node_type"`
	FromNode   string         `json:"from_node"`
	ToType     string         `json:"to_node_type"`
	ToNode     string         `json:"to_node"`
	Relation   string         `json:"relation"`
	Strength   float64        `json:"strength"`
	Confidence *float64       `json:"confidence,omitempty"`
	Meta       map[string]any `json:"meta,omitempty"`
	CreatedAt  uint64         `json:"created_at"`
	UpdatedAt  uint64         `json:"updated_at"`
	Version    uint32         `json:"version"`
}

// Store is an opened AKG file exposing only current logical state operations.
type Store struct {
	inner *store.Store
}

// Create writes a new empty AKG file and opens it.
func Create(path string) (*Store, error) {
	st, err := store.Create(path)
	if err != nil {
		return nil, err
	}
	return &Store{inner: st}, nil
}

// Open opens an AKG file with ordinary strict validation and committed-WAL
// replay. Trailing uncommitted WAL is ignored by the internal store rules.
func Open(path string) (*Store, error) {
	st, err := store.Open(path)
	if err != nil {
		return nil, err
	}
	return &Store{inner: st}, nil
}

// Validate verifies that path opens under ordinary strict validation semantics.
func Validate(path string) error { return store.Validate(path) }

// Compact explicitly rewrites path to contain only current live state and an
// empty WAL.
func Compact(path string) error { return store.Compact(path) }

// PutNode upserts a node by (type, id). If id is empty, an ID is generated.
func (s *Store) PutNode(id string, n Node) (NodeRecord, error) {
	rec, err := s.inner.PutNode(record.NodeID(id), toRecordNode(n))
	if err != nil {
		return NodeRecord{}, err
	}
	return fromStateNode(rec), nil
}

// PutEdge upserts an edge by (from_node, relation, to_node).
func (s *Store) PutEdge(e Edge) (Edge, error) {
	edge, err := s.inner.PutEdge(toRecordEdge(e))
	if err != nil {
		return Edge{}, err
	}
	return fromRecordEdge(edge), nil
}

// DeleteNode deletes an existing node by (type, id).
func (s *Store) DeleteNode(typeName, id string) error {
	return s.inner.DeleteNode(typeName, record.NodeID(id))
}

// DeleteEdge deletes an existing edge by (from_node_type, from_node, relation, to_node_type, to_node).
func (s *Store) DeleteEdge(fromType, fromNode, relation, toType, toNode string) error {
	return s.inner.DeleteEdge(fromType, record.NodeID(fromNode), record.Relation(relation), toType, record.NodeID(toNode))
}

// GetNode returns a current live node by (type, id).
func (s *Store) GetNode(typeName, id string) (NodeRecord, bool) {
	rec, ok := s.inner.State().GetNode(typeName, record.NodeID(id))
	if !ok {
		return NodeRecord{}, false
	}
	return fromStateNode(rec), true
}

// GetEdge returns a current live edge by (from_node_type, from_node, relation, to_node_type, to_node).
func (s *Store) GetEdge(fromType, fromNode, relation, toType, toNode string) (Edge, bool) {
	edge, ok := s.inner.State().GetEdge(fromType, record.NodeID(fromNode), record.Relation(relation), toType, record.NodeID(toNode))
	if !ok {
		return Edge{}, false
	}
	return fromRecordEdge(edge), true
}

// ListNodes returns all current live nodes. It does not expose tombstones,
// superseded records, or derived index entries.
func (s *Store) ListNodes() []NodeRecord {
	in := s.inner.State().Nodes()
	out := make([]NodeRecord, len(in))
	for i, rec := range in {
		out[i] = fromStateNode(rec)
	}
	return out
}

// ListEdges returns all current live edges. It does not expose tombstones,
// superseded records, or derived index entries.
func (s *Store) ListEdges() []Edge {
	in := s.inner.State().Edges()
	out := make([]Edge, len(in))
	for i, edge := range in {
		out[i] = fromRecordEdge(edge)
	}
	return out
}

// Commit durably appends pending mutations to the file WAL.
func (s *Store) Commit() error { return s.inner.Commit() }

// Close commits outstanding mutations.
func (s *Store) Close() error { return s.inner.Close() }

// Compact commits outstanding mutations, compacts the file, and refreshes the
// opened store's state.
func (s *Store) Compact() error { return s.inner.Compact() }

func toRecordNode(n Node) record.Node {
	return record.Node{Type: n.Type, Title: n.Title, Body: n.Body, Meta: cloneMap(n.Meta), Tags: cloneStrings(n.Tags), CreatedAt: record.TimestampMicros(n.CreatedAt), UpdatedAt: record.TimestampMicros(n.UpdatedAt), Version: record.Version(n.Version)}
}

func fromRecordNode(n record.Node) Node {
	return Node{Type: n.Type, Title: n.Title, Body: n.Body, Meta: cloneMap(n.Meta), Tags: cloneStrings(n.Tags), CreatedAt: uint64(n.CreatedAt), UpdatedAt: uint64(n.UpdatedAt), Version: uint32(n.Version)}
}

func fromStateNode(rec state.NodeRecord) NodeRecord {
	return NodeRecord{ID: string(rec.ID), Node: fromRecordNode(rec.Node)}
}

func toRecordEdge(e Edge) record.Edge {
	return record.Edge{FromType: e.FromType, FromNode: record.NodeID(e.FromNode), ToType: e.ToType, ToNode: record.NodeID(e.ToNode), Relation: record.Relation(e.Relation), Strength: e.Strength, Confidence: cloneFloat(e.Confidence), Meta: cloneMap(e.Meta), CreatedAt: record.TimestampMicros(e.CreatedAt), UpdatedAt: record.TimestampMicros(e.UpdatedAt), Version: record.Version(e.Version)}
}

func fromRecordEdge(e record.Edge) Edge {
	return Edge{FromType: e.FromType, FromNode: string(e.FromNode), ToType: e.ToType, ToNode: string(e.ToNode), Relation: string(e.Relation), Strength: e.Strength, Confidence: cloneFloat(e.Confidence), Meta: cloneMap(e.Meta), CreatedAt: uint64(e.CreatedAt), UpdatedAt: uint64(e.UpdatedAt), Version: uint32(e.Version)}
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneFloat(in *float64) *float64 {
	if in == nil {
		return nil
	}
	v := *in
	return &v
}
