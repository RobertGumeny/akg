package akg

// EdgeFields contains the content fields for an edge write.
type EdgeFields struct {
	Strength   float64
	Confidence *float64
	Meta       map[string]any
}

// Edge is the public current-state view of an AKG edge.
type Edge struct {
	From       NodeRef
	Relation   string
	To         NodeRef
	Strength   float64
	Confidence *float64
	Meta       map[string]any
	CreatedAt  uint64
	UpdatedAt  uint64
	Version    uint32
}

func edgeFromRecord(s *storeState, rec coreEdge) (*Edge, error) {
	fromType, err := s.resolveNodeType(rec.FromNode)
	if err != nil {
		return nil, err
	}
	toType, err := s.resolveNodeType(rec.ToNode)
	if err != nil {
		return nil, err
	}
	return &Edge{
		From:       NodeRef{Type: fromType, ID: string(rec.FromNode)},
		Relation:   string(rec.Relation),
		To:         NodeRef{Type: toType, ID: string(rec.ToNode)},
		Strength:   rec.Strength,
		Confidence: cloneConfidence(rec.Confidence),
		Meta:       cloneMap(rec.Meta),
		CreatedAt:  uint64(rec.CreatedAt),
		UpdatedAt:  uint64(rec.UpdatedAt),
		Version:    uint32(rec.Version),
	}, nil
}

func cloneConfidence(in *float64) *float64 {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

func coreEdgeFromFields(fromRef NodeRef, relationValue string, toRef NodeRef, fields EdgeFields) (coreEdge, error) {
	e := coreEdge{
		FromNode:   nodeID(fromRef.ID),
		ToNode:     nodeID(toRef.ID),
		Relation:   relation(relationValue),
		Strength:   fields.Strength,
		Confidence: cloneConfidence(fields.Confidence),
		Meta:       cloneMap(fields.Meta),
	}
	e.applyReadDefaults()
	return e, e.validateForWrite()
}

func (s *storeState) resolveNodeType(id nodeID) (string, error) {
	var typeName string
	for ident := range s.nodes {
		if ident.id != id {
			continue
		}
		if typeName != "" && typeName != ident.typeName {
			return "", errInvalidInput
		}
		typeName = ident.typeName
	}
	if typeName == "" {
		return "", errNotFound
	}
	return typeName, nil
}
