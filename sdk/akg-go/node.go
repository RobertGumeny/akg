package akg

// NodeFields contains the content fields for a node write.
type NodeFields struct {
	Title string
	Body  string
	Meta  map[string]any
}

// Node is the public current-state view of an AKG node.
type Node struct {
	Type      string
	ID        string
	Title     string
	Body      string
	Meta      map[string]any
	Tags      []string
	CreatedAt uint64
	UpdatedAt uint64
	Version   uint32
}

func nodeFromRecord(rec nodeRecord) *Node {
	return &Node{
		Type:      rec.Node.Type,
		ID:        string(rec.ID),
		Title:     rec.Node.Title,
		Body:      rec.Node.Body,
		Meta:      cloneMap(rec.Node.Meta),
		Tags:      cloneStrings(rec.Node.Tags),
		CreatedAt: uint64(rec.Node.CreatedAt),
		UpdatedAt: uint64(rec.Node.UpdatedAt),
		Version:   uint32(rec.Node.Version),
	}
}

func coreNodeFromFields(typeName string, fields NodeFields, tags []string) (coreNode, error) {
	n := coreNode{
		Type:  typeName,
		Title: fields.Title,
		Body:  fields.Body,
		Meta:  cloneMap(fields.Meta),
		Tags:  cloneStrings(tags),
	}
	return n, n.validateForWrite()
}
