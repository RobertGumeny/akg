package akg

// NodeRef is a stable, compact, JSON-serializable identifier for a node.
//
// Its JSON shape is part of the public SDK API and must remain:
//
//	{"type":"Hand","id":"h_47"}
//
// The TypeScript SDK must match this shape exactly, including field names and
// JSON keys.
type NodeRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}
