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

	// Collect incident edges from the secondary indexes — O(degree), not a full
	// edge scan — copying identities first so deleteEdge can mutate the indexes.
	// A self-loop appears in both index sets; dedup so it is deleted once.
	node := nodeIdentity{typeName: typeName, id: nodeID(id)}
	seen := make(map[edgeIdentity]struct{})
	var toDelete []edgeIdentity
	for eid := range s.state.outIndex[node] {
		if _, dup := seen[eid]; dup {
			continue
		}
		seen[eid] = struct{}{}
		toDelete = append(toDelete, eid)
	}
	for eid := range s.state.inIndex[node] {
		if _, dup := seen[eid]; dup {
			continue
		}
		seen[eid] = struct{}{}
		toDelete = append(toDelete, eid)
	}

	for _, eid := range toDelete {
		if err := s.deleteEdge(eid.fromType, eid.from, eid.relation, eid.toType, eid.to); err != nil {
			return CascadeDeleteResult{}, err
		}
		if eid.fromType == typeName && eid.from == nodeID(id) {
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
