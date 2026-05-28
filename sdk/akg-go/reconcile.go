package akg

// ReconcileResult reports the outcome of a ReconcileOutboundEdges call.
type ReconcileResult struct {
	Added     int
	Removed   int
	Unchanged int
}

// ReconcileOutboundEdges synchronizes the outbound edges for source+relation to
// exactly the desired target set. Missing desired edges are added, stale edges
// (same source+relation but not in desired) are removed, and matching edges are
// left unchanged. Relations and source nodes not referenced are untouched.
func (s *Store) ReconcileOutboundEdges(source NodeRef, relationValue string, desired []NodeRef, fields EdgeFields) (ReconcileResult, error) {
	if s == nil || s.closed {
		return ReconcileResult{}, ErrInvalidInput
	}
	if _, err := buildNodeKey(source.Type, nodeID(source.ID)); err != nil {
		return ReconcileResult{}, err
	}
	if _, ok := s.state.nodes[nodeIdentity{typeName: source.Type, id: nodeID(source.ID)}]; !ok {
		return ReconcileResult{}, ErrNotFound
	}
	if err := validateComponent(relationValue); err != nil {
		return ReconcileResult{}, err
	}
	for _, d := range desired {
		if _, err := buildNodeKey(d.Type, nodeID(d.ID)); err != nil {
			return ReconcileResult{}, err
		}
	}

	desiredSet := make(map[nodeIdentity]struct{}, len(desired))
	for _, d := range desired {
		desiredSet[nodeIdentity{typeName: d.Type, id: nodeID(d.ID)}] = struct{}{}
	}

	// Find existing edges for this source+relation.
	existing := make(map[nodeIdentity]struct{})
	for ident := range s.state.edges {
		if ident.fromType == source.Type && ident.from == nodeID(source.ID) && ident.relation == relation(relationValue) {
			existing[nodeIdentity{typeName: ident.toType, id: ident.to}] = struct{}{}
		}
	}

	var result ReconcileResult

	// Remove stale edges.
	for toIdent := range existing {
		if _, ok := desiredSet[toIdent]; !ok {
			if err := s.deleteEdge(source.Type, nodeID(source.ID), relation(relationValue), toIdent.typeName, toIdent.id); err != nil {
				return ReconcileResult{}, err
			}
			result.Removed++
		}
	}

	// Add missing edges.
	for _, d := range desired {
		toIdent := nodeIdentity{typeName: d.Type, id: nodeID(d.ID)}
		if _, ok := existing[toIdent]; ok {
			result.Unchanged++
		} else {
			if _, ok := s.state.nodes[toIdent]; !ok {
				return ReconcileResult{}, ErrNotFound
			}
			e, err := coreEdgeFromFields(source, relationValue, d, fields)
			if err != nil {
				return ReconcileResult{}, err
			}
			if _, err := s.putEdge(e); err != nil {
				return ReconcileResult{}, err
			}
			result.Added++
		}
	}

	return result, nil
}
