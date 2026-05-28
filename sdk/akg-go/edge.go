package akg

// EdgeFields contains the content fields for an edge write.
//
// Strength is a pointer so that nil ("omitted") is distinguishable from an
// explicit 0.0. When nil, the AKG v1 spec default of 0.5 is applied.
// Confidence uses the same convention: nil means "no confidence recorded".
type EdgeFields struct {
	Strength   *float64
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

// StrengthOf returns a pointer to v, for use in EdgeFields.Strength.
// It is the idiomatic way to supply an explicit strength value:
//
//	store.PutEdge(a, "knows", b, akg.EdgeFields{Strength: akg.StrengthOf(0.75)})
func StrengthOf(v float64) *float64 { return &v }

func cloneConfidence(in *float64) *float64 {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

func coreEdgeFromFields(fromRef NodeRef, relationValue string, toRef NodeRef, fields EdgeFields) (coreEdge, error) {
	strength := 0.5
	if fields.Strength != nil {
		strength = *fields.Strength
	}
	e := coreEdge{
		FromType:   fromRef.Type,
		FromNode:   nodeID(fromRef.ID),
		ToType:     toRef.Type,
		ToNode:     nodeID(toRef.ID),
		Relation:   relation(relationValue),
		Strength:   strength,
		Confidence: cloneConfidence(fields.Confidence),
		Meta:       cloneMap(fields.Meta),
	}
	e.applyReadDefaults()
	return e, e.validateForWrite()
}
