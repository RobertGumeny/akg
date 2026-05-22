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

func edgeFromRecord(rec coreEdge) *Edge {
	return &Edge{
		From:       NodeRef{Type: rec.FromType, ID: string(rec.FromNode)},
		Relation:   string(rec.Relation),
		To:         NodeRef{Type: rec.ToType, ID: string(rec.ToNode)},
		Strength:   rec.Strength,
		Confidence: cloneConfidence(rec.Confidence),
		Meta:       cloneMap(rec.Meta),
		CreatedAt:  uint64(rec.CreatedAt),
		UpdatedAt:  uint64(rec.UpdatedAt),
		Version:    uint32(rec.Version),
	}
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
		FromType:   fromRef.Type,
		FromNode:   nodeID(fromRef.ID),
		ToType:     toRef.Type,
		ToNode:     nodeID(toRef.ID),
		Relation:   relation(relationValue),
		Strength:   fields.Strength,
		Confidence: cloneConfidence(fields.Confidence),
		Meta:       cloneMap(fields.Meta),
	}
	e.applyReadDefaults()
	return e, e.validateForWrite()
}
