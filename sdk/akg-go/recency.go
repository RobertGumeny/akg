package akg

import (
	"bytes"
	"sort"
)

// RecencyFilter selects nodes by type, tag, and updatedAt window.
// Omitted/zero fields match all records in that dimension.
// Limit 0 means unlimited; negative Limit is invalid.
type RecencyFilter struct {
	Type           string
	Tag            string
	SinceUpdatedAt uint64
	UntilUpdatedAt uint64
	Limit          int
}

// EdgeRecencyFilter selects edges by relation, endpoints, and updatedAt window.
// Omitted/zero fields match all records in that dimension.
// Limit 0 means unlimited; negative Limit is invalid.
type EdgeRecencyFilter struct {
	Relation       string
	From           *NodeRef
	To             *NodeRef
	SinceUpdatedAt uint64
	UntilUpdatedAt uint64
	Limit          int
}

// RecentNodes returns live nodes matching filter, sorted newest-first by
// updatedAt. Tie-breaker order: createdAt desc, type asc, id asc.
func (s *Store) RecentNodes(filter RecencyFilter) ([]Node, error) {
	if s == nil || s.closed {
		return nil, ErrInvalidInput
	}
	if filter.Limit < 0 {
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
		ua := uint64(rec.Node.UpdatedAt)
		if filter.SinceUpdatedAt != 0 && ua < filter.SinceUpdatedAt {
			continue
		}
		if filter.UntilUpdatedAt != 0 && ua > filter.UntilUpdatedAt {
			continue
		}
		matches = append(matches, cloneNodeRecord(rec))
	}

	sort.Slice(matches, func(i, j int) bool {
		ni, nj := matches[i].Node, matches[j].Node
		if ni.UpdatedAt != nj.UpdatedAt {
			return ni.UpdatedAt > nj.UpdatedAt
		}
		if ni.CreatedAt != nj.CreatedAt {
			return ni.CreatedAt > nj.CreatedAt
		}
		if ni.Type != nj.Type {
			return ni.Type < nj.Type
		}
		ik, _ := buildNodeKey(matches[i].Node.Type, matches[i].ID)
		jk, _ := buildNodeKey(matches[j].Node.Type, matches[j].ID)
		return bytes.Compare(ik, jk) < 0
	})

	if filter.Limit > 0 && len(matches) > filter.Limit {
		matches = matches[:filter.Limit]
	}

	nodes := make([]Node, len(matches))
	for i, rec := range matches {
		nodes[i] = *nodeFromRecord(rec)
	}
	return nodes, nil
}

// RecentEdges returns live edges matching filter, sorted newest-first by
// updatedAt. Tie-breaker order: createdAt desc, from.type asc, from.id asc,
// relation asc, to.type asc, to.id asc.
func (s *Store) RecentEdges(filter EdgeRecencyFilter) ([]Edge, error) {
	if s == nil || s.closed {
		return nil, ErrInvalidInput
	}
	if filter.Limit < 0 {
		return nil, ErrInvalidInput
	}
	if filter.Relation != "" {
		if err := validateComponent(filter.Relation); err != nil {
			return nil, err
		}
	}
	if filter.From != nil {
		if _, err := buildNodeKey(filter.From.Type, nodeID(filter.From.ID)); err != nil {
			return nil, err
		}
	}
	if filter.To != nil {
		if _, err := buildNodeKey(filter.To.Type, nodeID(filter.To.ID)); err != nil {
			return nil, err
		}
	}

	matches := make([]coreEdge, 0)
	for _, e := range s.state.edges {
		if filter.Relation != "" && string(e.Relation) != filter.Relation {
			continue
		}
		if filter.From != nil {
			if e.FromType != filter.From.Type || string(e.FromNode) != filter.From.ID {
				continue
			}
		}
		if filter.To != nil {
			if e.ToType != filter.To.Type || string(e.ToNode) != filter.To.ID {
				continue
			}
		}
		ua := uint64(e.UpdatedAt)
		if filter.SinceUpdatedAt != 0 && ua < filter.SinceUpdatedAt {
			continue
		}
		if filter.UntilUpdatedAt != 0 && ua > filter.UntilUpdatedAt {
			continue
		}
		matches = append(matches, cloneEdge(e))
	}

	sort.Slice(matches, func(i, j int) bool {
		ei, ej := matches[i], matches[j]
		if ei.UpdatedAt != ej.UpdatedAt {
			return ei.UpdatedAt > ej.UpdatedAt
		}
		if ei.CreatedAt != ej.CreatedAt {
			return ei.CreatedAt > ej.CreatedAt
		}
		if ei.FromType != ej.FromType {
			return ei.FromType < ej.FromType
		}
		if ei.FromNode != ej.FromNode {
			return string(ei.FromNode) < string(ej.FromNode)
		}
		if ei.Relation != ej.Relation {
			return string(ei.Relation) < string(ej.Relation)
		}
		if ei.ToType != ej.ToType {
			return ei.ToType < ej.ToType
		}
		return string(ei.ToNode) < string(ej.ToNode)
	})

	if filter.Limit > 0 && len(matches) > filter.Limit {
		matches = matches[:filter.Limit]
	}

	edges := make([]Edge, len(matches))
	for i, e := range matches {
		edges[i] = *edgeFromRecord(e)
	}
	return edges, nil
}
