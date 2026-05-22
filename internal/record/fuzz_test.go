package record

import "testing"

func FuzzDecodeNodePayload(f *testing.F) {
	valid, _ := EncodeNodePayload(Node{Type: "note", Title: "hello", CreatedAt: 1, UpdatedAt: 1})
	f.Add([]byte{})
	f.Add([]byte{0x81, 0xa4, 't', 'y', 'p', 'e', 0xa4, 'n', 'o', 't', 'e'})
	f.Add(valid)

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeNodePayload(data)
	})
}

func FuzzDecodeEdgePayload(f *testing.F) {
	valid, _ := EncodeEdgePayload(Edge{FromType: "note", FromNode: "a", Relation: "links", ToType: "note", ToNode: "b", CreatedAt: 1, UpdatedAt: 1})
	f.Add([]byte{})
	f.Add([]byte{0x81, 0xa9, 'f', 'r', 'o', 'm', '_', 'n', 'o', 'd', 'e', 0xa1, 'a'})
	f.Add(valid)

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeEdgePayload(data)
	})
}

func FuzzDecodeNodeDeletePayload(f *testing.F) {
	valid, _ := EncodeNodeDeletePayload(NodeDelete{Type: "note", ID: "1"})
	f.Add([]byte{})
	f.Add([]byte{0x82, 0xa4, 't', 'y', 'p', 'e', 0xa4, 'n', 'o', 't', 'e', 0xa2, 'i', 'd', 0xa1, '1'})
	f.Add(valid)

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeNodeDeletePayload(data)
	})
}

func FuzzDecodeEdgeDeletePayload(f *testing.F) {
	valid, _ := EncodeEdgeDeletePayload(EdgeDelete{FromType: "note", FromNode: "a", Relation: "links", ToType: "note", ToNode: "b"})
	f.Add([]byte{})
	f.Add([]byte{0x83, 0xa9, 'f', 'r', 'o', 'm', '_', 'n', 'o', 'd', 'e', 0xa1, 'a', 0xa8, 'r', 'e', 'l', 'a', 't', 'i', 'o', 'n', 0xa5, 'l', 'i', 'n', 'k', 's'})
	f.Add(valid)

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeEdgeDeletePayload(data)
	})
}
