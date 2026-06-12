package akg

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

const maxTags = 32

type nodeID string
type relation string
type timestampMicros uint64
type version uint32

type coreNode struct {
	Type      string
	Title     string
	Body      string
	Meta      map[string]any
	Tags      []string
	CreatedAt timestampMicros
	UpdatedAt timestampMicros
	Version   version
}

type coreEdge struct {
	FromType   string
	FromNode   nodeID
	ToType     string
	ToNode     nodeID
	Relation   relation
	Strength   float64
	Confidence *float64
	Meta       map[string]any
	CreatedAt  timestampMicros
	UpdatedAt  timestampMicros
	Version    version
}

type nodePut struct {
	ID   nodeID
	Node coreNode
}

type nodeDelete struct {
	Type string
	ID   nodeID
}

type edgeDelete struct {
	FromType string
	FromNode nodeID
	Relation relation
	ToType   string
	ToNode   nodeID
}

type edgePut struct {
	Edge coreEdge
}

type nodeRecord struct {
	ID   nodeID
	Node coreNode
}

type nodeIdentity struct {
	typeName string
	id       nodeID
}

type edgeIdentity struct {
	fromType string
	from     nodeID
	relation relation
	toType   string
	to       nodeID
}

// ErrNotFound is returned when a requested node or edge does not exist.
var ErrNotFound = errors.New("not found")

// ErrInvalidInput is returned when a caller passes an argument that violates a
// format or semantic constraint (e.g. an invalid type name, a missing required
// field, or an operation that would leave the graph inconsistent).
var ErrInvalidInput = errors.New("invalid input")

// ErrMissingRequiredField is returned when a decoded record is structurally
// valid but is missing a field that the format requires (e.g. a node with no
// title, or an edge with no relation). Callers see this when opening a
// malformed file or when a bug in a writer omitted a required field.
var ErrMissingRequiredField = errors.New("missing required field")

var (
	errInvalidPayload = errors.New("invalid payload")
	errDuplicateTags  = fmt.Errorf("duplicate tags: %w", ErrInvalidInput)
	errTooManyTags    = fmt.Errorf("too many tags: %w", ErrInvalidInput)
)

// testNow, when non-nil, overrides the clock for all Store writes.
// Used only by deterministic generator tooling.
var testNow *timestampMicros

// SetTestNow pins the clock used by all Store writes to a fixed Unix-microsecond
// timestamp. Call SetTestNow(0) to restore wall-clock behavior.
// Intended only for deterministic artifact generation.
func SetTestNow(micros uint64) {
	if micros == 0 {
		testNow = nil
	} else {
		v := timestampMicros(micros)
		testNow = &v
	}
}

func (n *coreNode) applyReadDefaults() {
	if n.Meta == nil {
		n.Meta = map[string]any{}
	}
	if n.Tags == nil {
		n.Tags = []string{}
	}
	if n.Version == 0 {
		n.Version = 1
	}
}

func (n coreNode) validateForWrite() error {
	if n.Type == "" || n.Title == "" {
		return ErrMissingRequiredField
	}
	return nil
}

func (e *coreEdge) applyReadDefaults() {
	if e.Meta == nil {
		e.Meta = map[string]any{}
	}
	if e.Version == 0 {
		e.Version = 1
	}
}

func (e coreEdge) validateForWrite() error {
	if e.FromType == "" || e.FromNode == "" || e.ToType == "" || e.ToNode == "" || e.Relation == "" {
		return ErrMissingRequiredField
	}
	return nil
}

type storeState struct {
	nodes map[nodeIdentity]nodeRecord
	edges map[edgeIdentity]coreEdge
	// Secondary in-memory indexes derived from nodes/edges (PERF-1). They turn
	// ListNodesByTag / OutboundEdges / InboundEdges from O(total) full scans into
	// O(matches) lookups. They are pure derived state — rebuilt at load from the
	// same primary records the persisted derived keys validate — so there is no
	// format change. Every mutation path keeps them consistent.
	tagIndex map[string]map[nodeIdentity]struct{}       // tag -> node identities
	outIndex map[nodeIdentity]map[edgeIdentity]struct{} // from-node -> outbound edges
	inIndex  map[nodeIdentity]map[edgeIdentity]struct{} // to-node -> inbound edges
	now      func() timestampMicros
}

func newStoreState() *storeState {
	return &storeState{
		nodes:    make(map[nodeIdentity]nodeRecord),
		edges:    make(map[edgeIdentity]coreEdge),
		tagIndex: make(map[string]map[nodeIdentity]struct{}),
		outIndex: make(map[nodeIdentity]map[edgeIdentity]struct{}),
		inIndex:  make(map[nodeIdentity]map[edgeIdentity]struct{}),
		now: func() timestampMicros {
			if testNow != nil {
				return *testNow
			}
			return timestampMicros(time.Now().UnixMicro())
		},
	}
}

func (s *storeState) putNode(id nodeID, n coreNode) (nodeRecord, error) {
	if s == nil {
		return nodeRecord{}, ErrInvalidInput
	}
	if err := n.validateForWrite(); err != nil {
		return nodeRecord{}, err
	}
	if err := validateTags(n.Tags); err != nil {
		return nodeRecord{}, err
	}
	if id == "" {
		generated, err := s.generateNodeID(n.Type)
		if err != nil {
			return nodeRecord{}, err
		}
		id = generated
	}
	if _, err := buildNodeKey(n.Type, id); err != nil {
		return nodeRecord{}, err
	}
	for _, tag := range n.Tags {
		if _, err := buildTagKey(tag, id); err != nil {
			return nodeRecord{}, err
		}
	}
	ident := nodeIdentity{typeName: n.Type, id: id}
	n.Meta = cloneMap(n.Meta)
	n.Tags = cloneStrings(n.Tags)
	n.applyReadDefaults()
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
	if existing, ok := s.nodes[ident]; ok {
		s.indexRemoveTags(ident, existing.Node.Tags)
	}
	rec := nodeRecord{ID: id, Node: n}
	s.nodes[ident] = cloneNodeRecord(rec)
	s.indexAddTags(ident, n.Tags)
	return cloneNodeRecord(rec), nil
}

func (s *storeState) loadNodeRecord(rec nodeRecord) error {
	if s == nil {
		return ErrInvalidInput
	}
	if err := rec.Node.validateForWrite(); err != nil {
		return err
	}
	if rec.Node.Type == "" || rec.ID == "" {
		return ErrInvalidInput
	}
	if err := validateTags(rec.Node.Tags); err != nil {
		return err
	}
	if _, err := buildNodeKey(rec.Node.Type, rec.ID); err != nil {
		return err
	}
	rec.Node.applyReadDefaults()
	rec.Node.Meta = cloneMap(rec.Node.Meta)
	rec.Node.Tags = cloneStrings(rec.Node.Tags)
	ident := nodeIdentity{typeName: rec.Node.Type, id: rec.ID}
	// loadNodeRecord is reused for committed-WAL replay, where a later PUT_NODE can
	// replace an earlier one with different tags — drop the old tags first.
	if existing, ok := s.nodes[ident]; ok {
		s.indexRemoveTags(ident, existing.Node.Tags)
	}
	s.nodes[ident] = rec
	s.indexAddTags(ident, rec.Node.Tags)
	return nil
}

func (s *storeState) putEdge(e coreEdge) (coreEdge, error) {
	if s == nil {
		return coreEdge{}, ErrInvalidInput
	}
	if err := e.validateForWrite(); err != nil {
		return coreEdge{}, err
	}
	if _, err := buildEdgeKey(e.FromType, e.FromNode, e.Relation, e.ToType, e.ToNode); err != nil {
		return coreEdge{}, err
	}
	if _, err := buildEdgeIndexKey(e.ToType, e.ToNode, e.Relation, e.FromType, e.FromNode); err != nil {
		return coreEdge{}, err
	}
	e.Meta = cloneMap(e.Meta)
	e.applyReadDefaults()
	now := s.now()
	ident := edgeIdentity{fromType: e.FromType, from: e.FromNode, relation: e.Relation, toType: e.ToType, to: e.ToNode}
	if existing, ok := s.edges[ident]; ok {
		e.CreatedAt = existing.CreatedAt
		e.UpdatedAt = now
		e.Version = existing.Version + 1
	} else {
		e.CreatedAt = now
		e.UpdatedAt = now
		e.Version = 1
	}
	s.edges[ident] = cloneEdge(e)
	s.indexAddEdge(ident) // idempotent; identity is stable across replace
	return cloneEdge(e), nil
}

func (s *storeState) loadEdgeRecord(e coreEdge) error {
	if s == nil {
		return ErrInvalidInput
	}
	if err := e.validateForWrite(); err != nil {
		return err
	}
	if _, err := buildEdgeKey(e.FromType, e.FromNode, e.Relation, e.ToType, e.ToNode); err != nil {
		return err
	}
	if _, err := buildEdgeIndexKey(e.ToType, e.ToNode, e.Relation, e.FromType, e.FromNode); err != nil {
		return err
	}
	e.applyReadDefaults()
	e.Meta = cloneMap(e.Meta)
	ident := edgeIdentity{fromType: e.FromType, from: e.FromNode, relation: e.Relation, toType: e.ToType, to: e.ToNode}
	s.edges[ident] = e
	s.indexAddEdge(ident)
	return nil
}

func (s *storeState) generateNodeID(typeName string) (nodeID, error) {
	var b [8]byte
	for attempts := 0; attempts < 256; attempts++ {
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		id := nodeID(hex.EncodeToString(b[:]))
		if _, exists := s.nodes[nodeIdentity{typeName: typeName, id: id}]; !exists {
			return id, nil
		}
	}
	return "", ErrInvalidInput
}

func validateTags(tags []string) error {
	if len(tags) > maxTags {
		return errTooManyTags
	}
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		if _, ok := seen[tag]; ok {
			return errDuplicateTags
		}
		seen[tag] = struct{}{}
		if _, err := buildTagKey(tag, "tag-validation-placeholder"); err != nil {
			return err
		}
	}
	return nil
}

func cloneNodeRecord(rec nodeRecord) nodeRecord {
	rec.Node.Meta = cloneMap(rec.Node.Meta)
	rec.Node.Tags = cloneStrings(rec.Node.Tags)
	return rec
}

func cloneEdge(e coreEdge) coreEdge {
	e.Meta = cloneMap(e.Meta)
	if e.Confidence != nil {
		value := *e.Confidence
		e.Confidence = &value
	}
	return e
}
