package keys

import (
	"bytes"
	"errors"
	"testing"

	"github.com/RobertGumeny/akg-format/internal/record"
)

func TestNodeKeyBuildParse(t *testing.T) {
	key, err := BuildNodeKey("note", "abc123")
	if err != nil {
		t.Fatalf("BuildNodeKey: %v", err)
	}
	if got, want := string(key), "n:note:abc123"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	parsed, err := ParseNodeKey(key)
	if err != nil {
		t.Fatalf("ParseNodeKey: %v", err)
	}
	if parsed != (NodeKey{Type: "note", ID: "abc123"}) {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestEdgeKeyBuildParse(t *testing.T) {
	key, err := BuildEdgeKey("a", "links_to", "b")
	if err != nil {
		t.Fatalf("BuildEdgeKey: %v", err)
	}
	if got, want := string(key), "e:a:links_to:b"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	parsed, err := ParseEdgeKey(key)
	if err != nil {
		t.Fatalf("ParseEdgeKey: %v", err)
	}
	if parsed != (EdgeKey{FromNode: "a", Relation: "links_to", ToNode: "b"}) {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestEdgeIndexKeyBuildParse(t *testing.T) {
	key, err := BuildEdgeIndexKey("b", "links_to", "a")
	if err != nil {
		t.Fatalf("BuildEdgeIndexKey: %v", err)
	}
	if got, want := string(key), "ei:b:links_to:a"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	parsed, err := ParseEdgeIndexKey(key)
	if err != nil {
		t.Fatalf("ParseEdgeIndexKey: %v", err)
	}
	if parsed != (EdgeIndexKey{ToNode: "b", Relation: "links_to", FromNode: "a"}) {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestTagKeyBuildParse(t *testing.T) {
	key, err := BuildTagKey("multi_word2", "node1")
	if err != nil {
		t.Fatalf("BuildTagKey: %v", err)
	}
	if got, want := string(key), "t:multi_word2:node1"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	parsed, err := ParseTagKey(key)
	if err != nil {
		t.Fatalf("ParseTagKey: %v", err)
	}
	if parsed != (TagKey{Tag: "multi_word2", NodeID: "node1"}) {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestTemporalNodeKeyBuildParse(t *testing.T) {
	key, err := BuildTemporalNodeKey(1700000000000000, "note", "abc123")
	if err != nil {
		t.Fatalf("BuildTemporalNodeKey: %v", err)
	}
	if got, want := string(key), "ts:1700000000000000:n:note:abc123"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	parsed, err := ParseTemporalKey(key)
	if err != nil {
		t.Fatalf("ParseTemporalKey: %v", err)
	}
	if parsed.Timestamp != 1700000000000000 || parsed.Kind != "n" || parsed.Node != (NodeKey{Type: "note", ID: "abc123"}) {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestTemporalEdgeKeyBuildParse(t *testing.T) {
	key, err := BuildTemporalEdgeKey(1700000000000001, "a", "links_to", "b")
	if err != nil {
		t.Fatalf("BuildTemporalEdgeKey: %v", err)
	}
	if got, want := string(key), "ts:1700000000000001:e:a:links_to:b"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	parsed, err := ParseTemporalKey(key)
	if err != nil {
		t.Fatalf("ParseTemporalKey: %v", err)
	}
	if parsed.Timestamp != 1700000000000001 || parsed.Kind != "e" || parsed.Edge != (EdgeKey{FromNode: "a", Relation: "links_to", ToNode: "b"}) {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestBuildersRejectInvalidInputs(t *testing.T) {
	longID := record.NodeID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	tests := []struct {
		name string
		fn   func() ([]byte, error)
	}{
		{"node empty type", func() ([]byte, error) { return BuildNodeKey("", "id") }},
		{"node id colon", func() ([]byte, error) { return BuildNodeKey("note", "bad:id") }},
		{"node id too long", func() ([]byte, error) { return BuildNodeKey("note", longID) }},
		{"edge empty from", func() ([]byte, error) { return BuildEdgeKey("", "rel", "b") }},
		{"edge relation colon", func() ([]byte, error) { return BuildEdgeKey("a", "bad:rel", "b") }},
		{"edge index empty to", func() ([]byte, error) { return BuildEdgeIndexKey("", "rel", "a") }},
		{"tag uppercase", func() ([]byte, error) { return BuildTagKey("Bad", "id") }},
		{"tag spaces", func() ([]byte, error) { return BuildTagKey("bad tag", "id") }},
		{"tag double underscore", func() ([]byte, error) { return BuildTagKey("bad__tag", "id") }},
		{"temporal node bad id", func() ([]byte, error) { return BuildTemporalNodeKey(1, "note", "bad:id") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.fn()
			if !errors.Is(err, ErrInvalidComponent) {
				t.Fatalf("err = %v, want ErrInvalidComponent", err)
			}
		})
	}
}

func TestParsersRejectMalformedKeys(t *testing.T) {
	tests := []struct {
		name string
		key  []byte
		fn   func([]byte) error
	}{
		{"node incomplete", []byte("n:note"), func(k []byte) error { _, err := ParseNodeKey(k); return err }},
		{"node ambiguous extra delimiter", []byte("n:note:id:extra"), func(k []byte) error { _, err := ParseNodeKey(k); return err }},
		{"node wrong prefix", []byte("x:note:id"), func(k []byte) error { _, err := ParseNodeKey(k); return err }},
		{"edge incomplete", []byte("e:a:rel"), func(k []byte) error { _, err := ParseEdgeKey(k); return err }},
		{"edge empty component", []byte("e:a::b"), func(k []byte) error { _, err := ParseEdgeKey(k); return err }},
		{"edge index wrong prefix", []byte("e:b:rel:a"), func(k []byte) error { _, err := ParseEdgeIndexKey(k); return err }},
		{"tag uppercase", []byte("t:Bad:id"), func(k []byte) error { _, err := ParseTagKey(k); return err }},
		{"temporal missing suffix", []byte("ts:123"), func(k []byte) error { _, err := ParseTemporalKey(k); return err }},
		{"temporal unknown kind", []byte("ts:123:x:a:b"), func(k []byte) error { _, err := ParseTemporalKey(k); return err }},
		{"temporal non numeric", []byte("ts:abc:n:note:id"), func(k []byte) error { _, err := ParseTemporalKey(k); return err }},
		{"temporal noncanonical leading zero", []byte("ts:001:n:note:id"), func(k []byte) error { _, err := ParseTemporalKey(k); return err }},
		{"temporal incomplete node", []byte("ts:1:n:note"), func(k []byte) error { _, err := ParseTemporalKey(k); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.fn(tt.key); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestRawBytewiseLexicographicOrdering(t *testing.T) {
	noteA, _ := BuildNodeKey("note", "a")
	noteB, _ := BuildNodeKey("note", "b")
	taskA, _ := BuildNodeKey("task", "a")
	edge, _ := BuildEdgeKey("a", "rel", "b")
	inbound, _ := BuildEdgeIndexKey("b", "rel", "a")
	ts1, _ := BuildTemporalNodeKey(1700000000000000, "note", "a")
	ts2, _ := BuildTemporalNodeKey(1700000000000001, "note", "a")

	ordered := [][]byte{edge, inbound, noteA, noteB, taskA, ts1, ts2}
	for i := 1; i < len(ordered); i++ {
		if !BytewiseLess(ordered[i-1], ordered[i]) {
			t.Fatalf("%q should sort before %q", ordered[i-1], ordered[i])
		}
	}
	if bytes.Compare(noteA, []byte("n:note:")) <= 0 {
		t.Fatalf("node key %q should sort after its scan prefix", noteA)
	}
}
