package state

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/RobertGumeny/akg/internal/keys"
	"github.com/RobertGumeny/akg/internal/record"
)

const maxTags = 32

var (
	ErrNotFound      = errors.New("not found")
	ErrInvalidInput  = errors.New("invalid input")
	ErrDuplicateTags = errors.New("duplicate tags")
	ErrTooManyTags   = errors.New("too many tags")
)

type nowFunc func() record.TimestampMicros

type Option func(*State)

// WithNow overrides the timestamp source. It is primarily useful for focused
// state tests; production callers should use the default wall-clock source.
func WithNow(now func() record.TimestampMicros) Option {
	return func(s *State) {
		if now != nil {
			s.now = now
		}
	}
}

type nodeIdentity struct {
	typeName string
	id       record.NodeID
}

type edgeIdentity struct {
	fromType string
	from     record.NodeID
	relation record.Relation
	toType   string
	to       record.NodeID
}

// NodeRecord is an authoritative in-memory node plus the node ID carried by the
// key space. The ID is intentionally separate from the node payload.
type NodeRecord struct {
	ID   record.NodeID
	Node record.Node
}

// State contains only authoritative live logical nodes and edges. Derived keys
// such as ei:, t:, ts:, and Bloom data are intentionally not stored here.
type State struct {
	nodes map[nodeIdentity]NodeRecord
	edges map[edgeIdentity]record.Edge
	now   nowFunc
}

func New(opts ...Option) *State {
	s := &State{
		nodes: make(map[nodeIdentity]NodeRecord),
		edges: make(map[edgeIdentity]record.Edge),
		now: func() record.TimestampMicros {
			return record.TimestampMicros(time.Now().UnixMicro())
		},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// PutNode upserts a node by (type, id). If id is empty, a 16-character random
// lowercase hexadecimal ID is generated.
func (s *State) PutNode(id record.NodeID, n record.Node) (NodeRecord, error) {
	if s == nil {
		return NodeRecord{}, ErrInvalidInput
	}
	if err := n.ValidateForWrite(); err != nil {
		return NodeRecord{}, err
	}
	if err := validateTags(n.Tags); err != nil {
		return NodeRecord{}, err
	}
	if id == "" {
		generated, err := s.generateNodeID(n.Type)
		if err != nil {
			return NodeRecord{}, err
		}
		id = generated
	}
	if _, err := keys.BuildNodeKey(n.Type, id); err != nil {
		return NodeRecord{}, err
	}
	for _, tag := range n.Tags {
		if _, err := keys.BuildTagKey(tag, n.Type, id); err != nil {
			return NodeRecord{}, err
		}
	}

	ident := nodeIdentity{typeName: n.Type, id: id}
	n.Meta = cloneMap(n.Meta)
	n.Tags = cloneTags(n.Tags)
	n.ApplyReadDefaults()
	now := s.now()
	if existing, ok := s.nodes[ident]; ok {
		n.CreatedAt = existing.Node.CreatedAt
		n.UpdatedAt = now
		n.Version = existing.Node.Version + 1
	} else {
		n.CreatedAt = now
		n.UpdatedAt = now
		n.Version = 1
	}
	rec := NodeRecord{ID: id, Node: n}
	s.nodes[ident] = rec
	return cloneNodeRecord(rec), nil
}

// PutEdge upserts an edge by (from_node_type, from_node, relation, to_node_type, to_node). Dangling edges are
// accepted; no referential-integrity or cascade policy is enforced here.
func (s *State) PutEdge(e record.Edge) (record.Edge, error) {
	if s == nil {
		return record.Edge{}, ErrInvalidInput
	}
	if err := e.ValidateForWrite(); err != nil {
		return record.Edge{}, err
	}
	if _, err := keys.BuildEdgeKey(e.FromType, e.FromNode, e.Relation, e.ToType, e.ToNode); err != nil {
		return record.Edge{}, err
	}
	ident := edgeIdentity{fromType: e.FromType, from: e.FromNode, relation: e.Relation, toType: e.ToType, to: e.ToNode}
	e.Meta = cloneMap(e.Meta)
	e.ApplyReadDefaults()
	now := s.now()
	if existing, ok := s.edges[ident]; ok {
		e.CreatedAt = existing.CreatedAt
		e.UpdatedAt = now
		e.Version = existing.Version + 1
	} else {
		e.CreatedAt = now
		e.UpdatedAt = now
		e.Version = 1
	}
	s.edges[ident] = e
	return cloneEdge(e), nil
}

func (s *State) DeleteNode(typeName string, id record.NodeID) error {
	if s == nil {
		return ErrInvalidInput
	}
	if _, err := keys.BuildNodeKey(typeName, id); err != nil {
		return err
	}
	ident := nodeIdentity{typeName: typeName, id: id}
	if _, ok := s.nodes[ident]; !ok {
		return ErrNotFound
	}
	delete(s.nodes, ident)
	return nil
}

func (s *State) DeleteEdge(fromType string, from record.NodeID, relation record.Relation, toType string, to record.NodeID) error {
	if s == nil {
		return ErrInvalidInput
	}
	if _, err := keys.BuildEdgeKey(fromType, from, relation, toType, to); err != nil {
		return err
	}
	ident := edgeIdentity{fromType: fromType, from: from, relation: relation, toType: toType, to: to}
	if _, ok := s.edges[ident]; !ok {
		return ErrNotFound
	}
	delete(s.edges, ident)
	return nil
}

// LoadNodeRecord installs a node decoded from durable storage without applying
// writer-owned timestamp/version mutation semantics.
func (s *State) LoadNodeRecord(rec NodeRecord) error {
	if s == nil {
		return ErrInvalidInput
	}
	if err := rec.Node.ValidateForWrite(); err != nil {
		return err
	}
	if rec.Node.Type == "" || rec.ID == "" {
		return ErrInvalidInput
	}
	if err := validateTags(rec.Node.Tags); err != nil {
		return err
	}
	if _, err := keys.BuildNodeKey(rec.Node.Type, rec.ID); err != nil {
		return err
	}
	for _, tag := range rec.Node.Tags {
		if _, err := keys.BuildTagKey(tag, rec.Node.Type, rec.ID); err != nil {
			return err
		}
	}
	rec.Node.ApplyReadDefaults()
	rec.Node.Meta = cloneMap(rec.Node.Meta)
	rec.Node.Tags = cloneTags(rec.Node.Tags)
	s.nodes[nodeIdentity{typeName: rec.Node.Type, id: rec.ID}] = rec
	return nil
}

// LoadEdge installs an edge decoded from durable storage without applying
// writer-owned timestamp/version mutation semantics.
func (s *State) LoadEdge(e record.Edge) error {
	if s == nil {
		return ErrInvalidInput
	}
	if err := e.ValidateForWrite(); err != nil {
		return err
	}
	if _, err := keys.BuildEdgeKey(e.FromType, e.FromNode, e.Relation, e.ToType, e.ToNode); err != nil {
		return err
	}
	e.ApplyReadDefaults()
	e.Meta = cloneMap(e.Meta)
	if e.Confidence != nil {
		v := *e.Confidence
		e.Confidence = &v
	}
	s.edges[edgeIdentity{fromType: e.FromType, from: e.FromNode, relation: e.Relation, toType: e.ToType, to: e.ToNode}] = e
	return nil
}

func (s *State) GetNode(typeName string, id record.NodeID) (NodeRecord, bool) {
	if s == nil {
		return NodeRecord{}, false
	}
	rec, ok := s.nodes[nodeIdentity{typeName: typeName, id: id}]
	return cloneNodeRecord(rec), ok
}

func (s *State) GetEdge(fromType string, from record.NodeID, relation record.Relation, toType string, to record.NodeID) (record.Edge, bool) {
	if s == nil {
		return record.Edge{}, false
	}
	e, ok := s.edges[edgeIdentity{fromType: fromType, from: from, relation: relation, toType: toType, to: to}]
	return cloneEdge(e), ok
}

func (s *State) Nodes() []NodeRecord {
	out := make([]NodeRecord, 0, len(s.nodes))
	for _, rec := range s.nodes {
		out = append(out, cloneNodeRecord(rec))
	}
	return out
}

func (s *State) Edges() []record.Edge {
	out := make([]record.Edge, 0, len(s.edges))
	for _, e := range s.edges {
		out = append(out, cloneEdge(e))
	}
	return out
}

func validateTags(tags []string) error {
	if len(tags) > maxTags {
		return ErrTooManyTags
	}
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		if _, ok := seen[tag]; ok {
			return ErrDuplicateTags
		}
		seen[tag] = struct{}{}
		if _, err := keys.BuildTagKey(tag, "tag-validation-placeholder", "tag-validation-placeholder"); err != nil {
			return err
		}
	}
	return nil
}

func (s *State) generateNodeID(typeName string) (record.NodeID, error) {
	var b [8]byte
	for attempts := 0; attempts < 256; attempts++ {
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		id := record.NodeID(hex.EncodeToString(b[:]))
		if _, exists := s.nodes[nodeIdentity{typeName: typeName, id: id}]; !exists {
			return id, nil
		}
	}
	return "", ErrInvalidInput
}

func cloneNodeRecord(rec NodeRecord) NodeRecord {
	rec.Node.Meta = cloneMap(rec.Node.Meta)
	rec.Node.Tags = cloneTags(rec.Node.Tags)
	return rec
}

func cloneEdge(e record.Edge) record.Edge {
	e.Meta = cloneMap(e.Meta)
	if e.Confidence != nil {
		v := *e.Confidence
		e.Confidence = &v
	}
	return e
}

func cloneTags(in []string) []string {
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
