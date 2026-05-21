package record

import "errors"

// Canonical scalar types shared by Milestone 1 internal packages.
type (
	NodeID          string
	Relation        string
	TimestampMicros uint64
	Version         uint32
)

// Node is the canonical AKG node payload shape. Node identity is carried by the
// key layer, not in the payload.
type Node struct {
	Type      string
	Title     string
	Body      string
	Meta      map[string]any
	Tags      []string
	CreatedAt TimestampMicros
	UpdatedAt TimestampMicros
	Version   Version
}

// Edge is the canonical AKG edge payload shape. Its identity is the tuple
// (FromNode, Relation, ToNode).
type Edge struct {
	FromNode   NodeID
	ToNode     NodeID
	Relation   Relation
	Strength   float64
	Confidence *float64
	Meta       map[string]any
	CreatedAt  TimestampMicros
	UpdatedAt  TimestampMicros
	Version    Version
}

// NodeDelete identifies a node delete WAL payload.
type NodeDelete struct {
	Type string
	ID   NodeID
}

// EdgeDelete identifies an edge delete WAL payload.
type EdgeDelete struct {
	FromNode NodeID
	Relation Relation
	ToNode   NodeID
}

var (
	ErrMissingRequiredField = errors.New("missing required field")
	ErrInvalidPayload       = errors.New("invalid payload")
)

// ApplyReadDefaults fills defaults that AKG readers apply for omitted optional
// node fields.
func (n *Node) ApplyReadDefaults() {
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

// ValidateForWrite enforces node fields required of conformant writers.
func (n Node) ValidateForWrite() error {
	if n.Type == "" || n.Title == "" {
		return ErrMissingRequiredField
	}
	return nil
}

// ApplyReadDefaults fills defaults that AKG readers apply for omitted optional
// edge fields.
func (e *Edge) ApplyReadDefaults() {
	if e.Strength == 0 {
		e.Strength = 0.5
	}
	if e.Meta == nil {
		e.Meta = map[string]any{}
	}
	if e.Version == 0 {
		e.Version = 1
	}
}

// ValidateForWrite enforces edge fields required of conformant writers.
func (e Edge) ValidateForWrite() error {
	if e.FromNode == "" || e.ToNode == "" || e.Relation == "" {
		return ErrMissingRequiredField
	}
	return nil
}

// Map returns the MessagePack map identity shape for a DELETE_NODE payload.
func (d NodeDelete) Map() map[string]any {
	return map[string]any{"type": d.Type, "id": string(d.ID)}
}

// ValidateForWrite enforces DELETE_NODE required identity fields.
func (d NodeDelete) ValidateForWrite() error {
	if d.Type == "" || d.ID == "" {
		return ErrMissingRequiredField
	}
	return nil
}

// Map returns the MessagePack map identity shape for a DELETE_EDGE payload.
func (d EdgeDelete) Map() map[string]any {
	return map[string]any{
		"from_node": string(d.FromNode),
		"relation":  string(d.Relation),
		"to_node":   string(d.ToNode),
	}
}

// ValidateForWrite enforces DELETE_EDGE required identity fields.
func (d EdgeDelete) ValidateForWrite() error {
	if d.FromNode == "" || d.Relation == "" || d.ToNode == "" {
		return ErrMissingRequiredField
	}
	return nil
}

// NodeDeleteFromMap decodes a DELETE_NODE identity map and ignores unknown extra
// fields, matching read-time WAL payload rules.
func NodeDeleteFromMap(m map[string]any) (NodeDelete, error) {
	typeValue, ok := m["type"].(string)
	if !ok || typeValue == "" {
		return NodeDelete{}, ErrMissingRequiredField
	}
	idValue, ok := m["id"].(string)
	if !ok || idValue == "" {
		return NodeDelete{}, ErrMissingRequiredField
	}
	return NodeDelete{Type: typeValue, ID: NodeID(idValue)}, nil
}

// EdgeDeleteFromMap decodes a DELETE_EDGE identity map and ignores unknown extra
// fields, matching read-time WAL payload rules.
func EdgeDeleteFromMap(m map[string]any) (EdgeDelete, error) {
	from, ok := m["from_node"].(string)
	if !ok || from == "" {
		return EdgeDelete{}, ErrMissingRequiredField
	}
	relation, ok := m["relation"].(string)
	if !ok || relation == "" {
		return EdgeDelete{}, ErrMissingRequiredField
	}
	to, ok := m["to_node"].(string)
	if !ok || to == "" {
		return EdgeDelete{}, ErrMissingRequiredField
	}
	return EdgeDelete{FromNode: NodeID(from), Relation: Relation(relation), ToNode: NodeID(to)}, nil
}
