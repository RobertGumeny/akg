package akg

import (
	"bytes"
	"encoding/json"
	"sort"
)

// EdgeFilter selects live edges by relation and/or metadata.
// Empty/zero values match all edges in that dimension.
// Multiple non-zero fields combine with AND semantics.
type EdgeFilter struct {
	Relation string
	Meta     map[string]any
}

// NodeFilter selects live nodes by type, tag, and/or metadata.
// Empty/zero values match all nodes in that dimension.
// Multiple non-zero fields combine with AND semantics.
type NodeFilter struct {
	Type string
	Tag  string
	Meta map[string]any
}

// Snapshot is a point-in-time view of all live nodes and edges.
// It is JSON-serializable with standard library tooling.
type Snapshot struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// ListEdges returns all live edges that match filter. An empty filter returns
// all edges. Results are sorted by edge key.
func (s *Store) ListEdges(filter EdgeFilter) ([]Edge, error) {
	if s == nil || s.closed {
		return nil, ErrInvalidInput
	}
	if filter.Relation != "" {
		if err := validateComponent(filter.Relation); err != nil {
			return nil, err
		}
	}
	matches := make([]coreEdge, 0)
	for _, e := range s.state.edges {
		if filter.Relation != "" && string(e.Relation) != filter.Relation {
			continue
		}
		if filter.Meta != nil && !metaMatches(e.Meta, filter.Meta) {
			continue
		}
		matches = append(matches, cloneEdge(e))
	}
	sort.Slice(matches, func(i, j int) bool {
		ik, _ := buildEdgeKey(matches[i].FromType, matches[i].FromNode, matches[i].Relation, matches[i].ToType, matches[i].ToNode)
		jk, _ := buildEdgeKey(matches[j].FromType, matches[j].FromNode, matches[j].Relation, matches[j].ToType, matches[j].ToNode)
		return bytes.Compare(ik, jk) < 0
	})
	edges := make([]Edge, len(matches))
	for i, e := range matches {
		edges[i] = *edgeFromRecord(e)
	}
	return edges, nil
}

// Snapshot returns all live nodes and all live edges in deterministic order.
func (s *Store) Snapshot() (Snapshot, error) {
	if s == nil || s.closed {
		return Snapshot{}, ErrInvalidInput
	}
	nodes, err := s.ListNodes("")
	if err != nil {
		return Snapshot{}, err
	}
	edges, err := s.ListEdges(EdgeFilter{})
	if err != nil {
		return Snapshot{}, err
	}
	if nodes == nil {
		nodes = []Node{}
	}
	if edges == nil {
		edges = []Edge{}
	}
	return Snapshot{Nodes: nodes, Edges: edges}, nil
}

// ListNodesFiltered returns all live nodes that match filter. An empty filter
// returns all nodes. Results are sorted by node key.
func (s *Store) ListNodesFiltered(filter NodeFilter) ([]Node, error) {
	if s == nil || s.closed {
		return nil, ErrInvalidInput
	}
	if filter.Type != "" {
		if err := validateComponent(filter.Type); err != nil {
			return nil, err
		}
	}
	if filter.Tag != "" {
		if err := validateTag(filter.Tag); err != nil {
			return nil, err
		}
	}
	matches := make([]nodeRecord, 0)
	for _, rec := range s.state.nodes {
		if filter.Type != "" && rec.Node.Type != filter.Type {
			continue
		}
		if filter.Tag != "" && !hasTag(rec.Node.Tags, filter.Tag) {
			continue
		}
		if filter.Meta != nil && !metaMatches(rec.Node.Meta, filter.Meta) {
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

// GetNodes returns nodes for the given refs in input order. Missing refs return
// nil at the corresponding position. Duplicate refs produce duplicate positions.
func (s *Store) GetNodes(refs []NodeRef) ([]*Node, error) {
	if s == nil || s.closed {
		return nil, ErrInvalidInput
	}
	result := make([]*Node, len(refs))
	for i, ref := range refs {
		if _, err := buildNodeKey(ref.Type, nodeID(ref.ID)); err != nil {
			return nil, err
		}
		rec, ok := s.state.nodes[nodeIdentity{typeName: ref.Type, id: nodeID(ref.ID)}]
		if !ok {
			result[i] = nil
			continue
		}
		result[i] = nodeFromRecord(cloneNodeRecord(rec))
	}
	return result, nil
}

// metaMatches reports whether all key/value pairs in filter exist in meta with
// deep JSON equality semantics.
func metaMatches(meta map[string]any, filter map[string]any) bool {
	for k, fv := range filter {
		mv, ok := meta[k]
		if !ok {
			return false
		}
		if !deepEqual(mv, fv) {
			return false
		}
	}
	return true
}

// deepEqual compares two JSON-like values with recursive equality.
// Objects are compared key-by-key ignoring key order.
func deepEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	switch av := a.(type) {
	case map[string]any:
		bm, ok := b.(map[string]any)
		if !ok {
			return false
		}
		if len(av) != len(bm) {
			return false
		}
		for k, av2 := range av {
			bv2, ok := bm[k]
			if !ok {
				return false
			}
			if !deepEqual(av2, bv2) {
				return false
			}
		}
		return true
	case []any:
		bv, ok := b.([]any)
		if !ok {
			return false
		}
		if len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !deepEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	default:
		// Normalize numeric types via JSON round-trip for cross-type comparison.
		aj, err1 := json.Marshal(a)
		bj, err2 := json.Marshal(b)
		if err1 != nil || err2 != nil {
			return false
		}
		return bytes.Equal(aj, bj)
	}
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
