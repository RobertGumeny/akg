package akg

// Secondary-index maintenance for storeState (PERF-1). These helpers keep the
// tag→node and node→edge indexes consistent with the primary nodes/edges maps.
// They are all idempotent at the set level, so callers may add an already-present
// entry without harm.

func (s *storeState) indexAddTags(ident nodeIdentity, tags []string) {
	for _, tag := range tags {
		set := s.tagIndex[tag]
		if set == nil {
			set = make(map[nodeIdentity]struct{})
			s.tagIndex[tag] = set
		}
		set[ident] = struct{}{}
	}
}

func (s *storeState) indexRemoveTags(ident nodeIdentity, tags []string) {
	for _, tag := range tags {
		set := s.tagIndex[tag]
		if set == nil {
			continue
		}
		delete(set, ident)
		if len(set) == 0 {
			delete(s.tagIndex, tag)
		}
	}
}

func (s *storeState) indexAddEdge(eid edgeIdentity) {
	from := nodeIdentity{typeName: eid.fromType, id: eid.from}
	to := nodeIdentity{typeName: eid.toType, id: eid.to}
	addEdgeToSet(s.outIndex, from, eid)
	addEdgeToSet(s.inIndex, to, eid)
}

func (s *storeState) indexRemoveEdge(eid edgeIdentity) {
	from := nodeIdentity{typeName: eid.fromType, id: eid.from}
	to := nodeIdentity{typeName: eid.toType, id: eid.to}
	removeEdgeFromSet(s.outIndex, from, eid)
	removeEdgeFromSet(s.inIndex, to, eid)
}

func addEdgeToSet(index map[nodeIdentity]map[edgeIdentity]struct{}, node nodeIdentity, eid edgeIdentity) {
	set := index[node]
	if set == nil {
		set = make(map[edgeIdentity]struct{})
		index[node] = set
	}
	set[eid] = struct{}{}
}

func removeEdgeFromSet(index map[nodeIdentity]map[edgeIdentity]struct{}, node nodeIdentity, eid edgeIdentity) {
	set := index[node]
	if set == nil {
		return
	}
	delete(set, eid)
	if len(set) == 0 {
		delete(index, node)
	}
}

// hasIncidentEdges reports whether the node has any live inbound or outbound edge,
// in O(1) via the edge indexes (replaces a full edge scan).
func (s *storeState) hasIncidentEdges(ident nodeIdentity) bool {
	return len(s.outIndex[ident]) > 0 || len(s.inIndex[ident]) > 0
}
