package record

import (
	"errors"
	"reflect"
	"testing"
)

func TestNodeDefaultsAndWriteValidation(t *testing.T) {
	n := Node{Type: "note", Title: "Hello"}
	n.ApplyReadDefaults()

	if n.Body != "" || n.Meta == nil || len(n.Meta) != 0 || n.Tags == nil || len(n.Tags) != 0 || n.Version != 1 {
		t.Fatalf("unexpected defaults: %#v", n)
	}
	if err := n.ValidateForWrite(); err != nil {
		t.Fatalf("valid node rejected: %v", err)
	}
	if err := (Node{Title: "missing type"}).ValidateForWrite(); !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("missing node type error = %v", err)
	}
	if err := (Node{Type: "note"}).ValidateForWrite(); !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("missing node title error = %v", err)
	}
}

func TestEdgeDefaultsAndWriteValidation(t *testing.T) {
	e := Edge{FromType: "note", FromNode: "a", Relation: "links", ToType: "note", ToNode: "b"}
	e.ApplyReadDefaults()

	if e.Strength != 0.0 || e.Confidence != nil || e.Meta == nil || len(e.Meta) != 0 || e.Version != 1 {
		t.Fatalf("unexpected defaults: %#v", e)
	}
	if err := e.ValidateForWrite(); err != nil {
		t.Fatalf("valid edge rejected: %v", err)
	}
	if err := (Edge{Relation: "links", ToNode: "b"}).ValidateForWrite(); !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("missing edge from_node error = %v", err)
	}
	if err := (Edge{FromNode: "a", ToNode: "b"}).ValidateForWrite(); !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("missing edge relation error = %v", err)
	}
	if err := (Edge{FromNode: "a", Relation: "links"}).ValidateForWrite(); !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("missing edge to_node error = %v", err)
	}
}

func TestNodeDeletePayloadShapeAndReadTolerance(t *testing.T) {
	d := NodeDelete{Type: "note", ID: "n1"}
	wantMap := map[string]any{"type": "note", "id": "n1"}
	if got := d.Map(); !reflect.DeepEqual(got, wantMap) {
		t.Fatalf("DELETE_NODE map = %#v, want %#v", got, wantMap)
	}
	if err := d.ValidateForWrite(); err != nil {
		t.Fatalf("valid delete rejected: %v", err)
	}

	got, err := NodeDeleteFromMap(map[string]any{"type": "note", "id": "n1", "ignored": true})
	if err != nil {
		t.Fatalf("read with unknown field rejected: %v", err)
	}
	if got != d {
		t.Fatalf("decoded delete = %#v, want %#v", got, d)
	}
	if _, err := NodeDeleteFromMap(map[string]any{"type": "note"}); !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("missing id error = %v", err)
	}
	if err := (NodeDelete{Type: "note"}).ValidateForWrite(); !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("write validation error = %v", err)
	}
}

func TestEdgeDeletePayloadShapeAndReadTolerance(t *testing.T) {
	d := EdgeDelete{FromType: "note", FromNode: "a", Relation: "links", ToType: "note", ToNode: "b"}
	wantMap := map[string]any{"from_node_type": "note", "from_node": "a", "relation": "links", "to_node_type": "note", "to_node": "b"}
	if got := d.Map(); !reflect.DeepEqual(got, wantMap) {
		t.Fatalf("DELETE_EDGE map = %#v, want %#v", got, wantMap)
	}
	if err := d.ValidateForWrite(); err != nil {
		t.Fatalf("valid delete rejected: %v", err)
	}

	got, err := EdgeDeleteFromMap(map[string]any{"from_node_type": "note", "from_node": "a", "relation": "links", "to_node_type": "note", "to_node": "b", "ignored": 123})
	if err != nil {
		t.Fatalf("read with unknown field rejected: %v", err)
	}
	if got != d {
		t.Fatalf("decoded delete = %#v, want %#v", got, d)
	}
	if _, err := EdgeDeleteFromMap(map[string]any{"from_node_type": "note", "from_node": "a", "relation": "links", "to_node_type": "note"}); !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("missing to_node error = %v", err)
	}
	if err := (EdgeDelete{FromType: "note", FromNode: "a", Relation: "links"}).ValidateForWrite(); !errors.Is(err, ErrMissingRequiredField) {
		t.Fatalf("write validation error = %v", err)
	}
}
