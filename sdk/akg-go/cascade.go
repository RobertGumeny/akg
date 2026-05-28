package akg

// CascadeDeleteResult reports the outcome of a DeleteNodeCascade call.
type CascadeDeleteResult struct {
	DeletedInboundEdges  int
	DeletedOutboundEdges int
	DeletedNode          bool
}

// DeleteNodeCascade deletes all inbound and outbound edges of the node and then
// deletes the node itself. It is an explicit opt-in helper; normal DeleteNode
// behavior is unchanged.
//
// Returns ErrNotFound if the node does not exist.
func (s *Store) DeleteNodeCascade(typeName, id string) (CascadeDeleteResult, error) {
	if s == nil || s.closed {
		return CascadeDeleteResult{}, ErrInvalidInput
	}
	if _, err := buildNodeKey(typeName, nodeID(id)); err != nil {
		return CascadeDeleteResult{}, err
	}
	if _, ok := s.state.nodes[nodeIdentity{typeName: typeName, id: nodeID(id)}]; !ok {
		return CascadeDeleteResult{}, ErrNotFound
	}

	var result CascadeDeleteResult

	// Collect all edges to delete to avoid mutating the map while iterating.
	type edgeKey struct {
		fromType string
		from     nodeID
		rel      relation
		toType   string
		to       nodeID
	}
	var toDelete []edgeKey
	for ident := range s.state.edges {
		if (ident.fromType == typeName && ident.from == nodeID(id)) ||
			(ident.toType == typeName && ident.to == nodeID(id)) {
			toDelete = append(toDelete, edgeKey{
				fromType: ident.fromType,
				from:     ident.from,
				rel:      ident.relation,
				toType:   ident.toType,
				to:       ident.to,
			})
		}
	}

	for _, ek := range toDelete {
		if err := s.deleteEdge(ek.fromType, ek.from, ek.rel, ek.toType, ek.to); err != nil {
			return CascadeDeleteResult{}, err
		}
		if ek.fromType == typeName && ek.from == nodeID(id) {
			result.DeletedOutboundEdges++
		} else {
			result.DeletedInboundEdges++
		}
	}

	if err := s.deleteNode(typeName, nodeID(id)); err != nil {
		return CascadeDeleteResult{}, err
	}
	result.DeletedNode = true
	return result, nil
}
