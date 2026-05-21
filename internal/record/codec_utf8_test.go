package record

import (
	"errors"
	"testing"
)

func TestDecodeRejectsInvalidUTF8PayloadStrings(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		fn   func([]byte) error
	}{
		{
			name: "node title",
			data: []byte{0x82, 0xa4, 't', 'y', 'p', 'e', 0xa4, 'n', 'o', 't', 'e', 0xa5, 't', 'i', 't', 'l', 'e', 0xa1, 0xff},
			fn:   func(b []byte) error { _, err := DecodeNodePayload(b); return err },
		},
		{
			name: "node nested meta string value",
			data: []byte{0x83, 0xa4, 't', 'y', 'p', 'e', 0xa4, 'n', 'o', 't', 'e', 0xa5, 't', 'i', 't', 'l', 'e', 0xa2, 'o', 'k', 0xa4, 'm', 'e', 't', 'a', 0x81, 0xa1, 'k', 0x91, 0xa1, 0xff},
			fn:   func(b []byte) error { _, err := DecodeNodePayload(b); return err },
		},
		{
			name: "node nested meta key",
			data: []byte{0x83, 0xa4, 't', 'y', 'p', 'e', 0xa4, 'n', 'o', 't', 'e', 0xa5, 't', 'i', 't', 'l', 'e', 0xa2, 'o', 'k', 0xa4, 'm', 'e', 't', 'a', 0x81, 0xa1, 0xff, 0xa1, 'v'},
			fn:   func(b []byte) error { _, err := DecodeNodePayload(b); return err },
		},
		{
			name: "edge relation",
			data: []byte{0x83, 0xa9, 'f', 'r', 'o', 'm', '_', 'n', 'o', 'd', 'e', 0xa1, 'a', 0xa7, 't', 'o', '_', 'n', 'o', 'd', 'e', 0xa1, 'b', 0xa8, 'r', 'e', 'l', 'a', 't', 'i', 'o', 'n', 0xa1, 0xff},
			fn:   func(b []byte) error { _, err := DecodeEdgePayload(b); return err },
		},
		{
			name: "delete node id",
			data: []byte{0x82, 0xa4, 't', 'y', 'p', 'e', 0xa4, 'n', 'o', 't', 'e', 0xa2, 'i', 'd', 0xa1, 0xff},
			fn:   func(b []byte) error { _, err := DecodeNodeDeletePayload(b); return err },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(tt.data); !errors.Is(err, ErrInvalidPayload) {
				t.Fatalf("decode error = %v, want %v", err, ErrInvalidPayload)
			}
		})
	}
}

func TestEncodeRejectsInvalidUTF8PayloadStrings(t *testing.T) {
	bad := string([]byte{0xff})
	tests := []struct {
		name string
		fn   func() error
	}{
		{name: "node title", fn: func() error {
			_, err := EncodeNodePayload(Node{Type: "note", Title: bad, CreatedAt: 1, UpdatedAt: 1})
			return err
		}},
		{name: "node meta key", fn: func() error {
			_, err := EncodeNodePayload(Node{Type: "note", Title: "ok", Meta: map[string]any{bad: "value"}, CreatedAt: 1, UpdatedAt: 1})
			return err
		}},
		{name: "node nested meta value", fn: func() error {
			_, err := EncodeNodePayload(Node{Type: "note", Title: "ok", Meta: map[string]any{"k": []any{bad}}, CreatedAt: 1, UpdatedAt: 1})
			return err
		}},
		{name: "edge relation", fn: func() error {
			_, err := EncodeEdgePayload(Edge{FromNode: "a", Relation: Relation(bad), ToNode: "b", CreatedAt: 1, UpdatedAt: 1})
			return err
		}},
		{name: "delete node id", fn: func() error { _, err := EncodeNodeDeletePayload(NodeDelete{Type: "note", ID: NodeID(bad)}); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(); !errors.Is(err, ErrInvalidPayload) {
				t.Fatalf("encode error = %v, want %v", err, ErrInvalidPayload)
			}
		})
	}
}
