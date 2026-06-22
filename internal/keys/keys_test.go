package keys

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/RobertGumeny/akg/internal/record"
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
	key, err := BuildEdgeKey("note", "a", "links_to", "task", "b")
	if err != nil {
		t.Fatalf("BuildEdgeKey: %v", err)
	}
	if got, want := string(key), "e:note:a:links_to:task:b"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	parsed, err := ParseEdgeKey(key)
	if err != nil {
		t.Fatalf("ParseEdgeKey: %v", err)
	}
	if parsed != (EdgeKey{FromType: "note", FromNode: "a", Relation: "links_to", ToType: "task", ToNode: "b"}) {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestEdgeIndexKeyBuildParse(t *testing.T) {
	key, err := BuildEdgeIndexKey("task", "b", "links_to", "note", "a")
	if err != nil {
		t.Fatalf("BuildEdgeIndexKey: %v", err)
	}
	if got, want := string(key), "ei:task:b:links_to:note:a"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	parsed, err := ParseEdgeIndexKey(key)
	if err != nil {
		t.Fatalf("ParseEdgeIndexKey: %v", err)
	}
	if parsed != (EdgeIndexKey{ToType: "task", ToNode: "b", Relation: "links_to", FromType: "note", FromNode: "a"}) {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestTagKeyBuildParse(t *testing.T) {
	key, err := BuildTagKey("multi_word2", "note", "node1")
	if err != nil {
		t.Fatalf("BuildTagKey: %v", err)
	}
	if got, want := string(key), "t:multi_word2:note:node1"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	parsed, err := ParseTagKey(key)
	if err != nil {
		t.Fatalf("ParseTagKey: %v", err)
	}
	if parsed != (TagKey{Tag: "multi_word2", Type: "note", NodeID: "node1"}) {
		t.Fatalf("parsed = %#v", parsed)
	}
}

// TestTagKeyV1BuildParse pins the read-compat path: the legacy major-1 builder
// emits the 3-part t:{tag}:{id} key, and ParseTagKey round-trips it with an
// empty Type, disambiguated from the 4-part major-2 key by component count.
func TestTagKeyV1BuildParse(t *testing.T) {
	key, err := BuildTagKeyV1("multi_word2", "node1")
	if err != nil {
		t.Fatalf("BuildTagKeyV1: %v", err)
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
	key, err := BuildTemporalEdgeKey(1700000000000001, "note", "a", "links_to", "task", "b")
	if err != nil {
		t.Fatalf("BuildTemporalEdgeKey: %v", err)
	}
	if got, want := string(key), "ts:1700000000000001:e:note:a:links_to:task:b"; got != want {
		t.Fatalf("key = %q, want %q", got, want)
	}
	parsed, err := ParseTemporalKey(key)
	if err != nil {
		t.Fatalf("ParseTemporalKey: %v", err)
	}
	if parsed.Timestamp != 1700000000000001 || parsed.Kind != "e" || parsed.Edge != (EdgeKey{FromType: "note", FromNode: "a", Relation: "links_to", ToType: "task", ToNode: "b"}) {
		t.Fatalf("parsed = %#v", parsed)
	}
}

func TestBuildersRejectInvalidInputs(t *testing.T) {
	over := record.NodeID(strings.Repeat("a", 65)) // 65 bytes, one over the cap
	overType := strings.Repeat("a", 65)            // 65 bytes
	multibyteOver := strings.Repeat("é", 33)       // 33 codepoints, 66 bytes
	tests := []struct {
		name string
		fn   func() ([]byte, error)
	}{
		{"node empty type", func() ([]byte, error) { return BuildNodeKey("", "id") }},
		{"node id colon", func() ([]byte, error) { return BuildNodeKey("note", "bad:id") }},
		{"node id too long", func() ([]byte, error) { return BuildNodeKey("note", over) }},
		{"node id multibyte over cap", func() ([]byte, error) { return BuildNodeKey("note", record.NodeID(multibyteOver)) }},
		{"node type too long", func() ([]byte, error) { return BuildNodeKey(overType, "id") }},
		{"edge empty from type", func() ([]byte, error) { return BuildEdgeKey("", "a", "rel", "note", "b") }},
		{"edge empty from", func() ([]byte, error) { return BuildEdgeKey("note", "", "rel", "note", "b") }},
		{"edge relation colon", func() ([]byte, error) { return BuildEdgeKey("note", "a", "bad:rel", "note", "b") }},
		{"edge relation too long", func() ([]byte, error) { return BuildEdgeKey("note", "a", record.Relation(overType), "note", "b") }},
		{"edge index empty to type", func() ([]byte, error) { return BuildEdgeIndexKey("", "b", "rel", "note", "a") }},
		{"edge index empty to", func() ([]byte, error) { return BuildEdgeIndexKey("note", "", "rel", "note", "a") }},
		{"tag colon", func() ([]byte, error) { return BuildTagKey("bad:tag", "note", "id") }},
		{"tag empty", func() ([]byte, error) { return BuildTagKey("", "note", "id") }},
		{"tag too long", func() ([]byte, error) { return BuildTagKey(overType, "note", "id") }},
		{"tag multibyte over cap", func() ([]byte, error) { return BuildTagKey(multibyteOver, "note", "id") }},
		{"tag type empty", func() ([]byte, error) { return BuildTagKey("active", "", "id") }},
		{"tag type colon", func() ([]byte, error) { return BuildTagKey("active", "bad:type", "id") }},
		{"tag id colon", func() ([]byte, error) { return BuildTagKey("active", "note", "bad:id") }},
		{"tag v1 colon", func() ([]byte, error) { return BuildTagKeyV1("bad:tag", "id") }},
		{"tag v1 id colon", func() ([]byte, error) { return BuildTagKeyV1("active", "bad:id") }},
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

// TestBuildersAcceptUTF8AndByteCap pins the CONF-1/CONF-2 contract: type,
// relation, and tag are any key-safe UTF-8 string (no snake_case rule), capped
// at 64 bytes. Casing and non-ASCII values are accepted; the cap is on bytes.
func TestBuildersAcceptUTF8AndByteCap(t *testing.T) {
	atCap := strings.Repeat("a", 64)          // exactly 64 bytes
	multibyteAtCap := strings.Repeat("é", 32) // 32 codepoints, 64 bytes
	tests := []struct {
		name string
		fn   func() ([]byte, error)
	}{
		{"type uppercase", func() ([]byte, error) { return BuildNodeKey("Person", "id") }},
		{"type non-ascii", func() ([]byte, error) { return BuildNodeKey("café", "id") }},
		{"relation uppercase", func() ([]byte, error) { return BuildEdgeKey("Person", "a", "KNOWS", "Person", "b") }},
		{"relation non-ascii", func() ([]byte, error) { return BuildEdgeKey("note", "a", "café", "note", "b") }},
		{"tag uppercase", func() ([]byte, error) { return BuildTagKey("Active", "note", "id") }},
		{"tag non-ascii", func() ([]byte, error) { return BuildTagKey("café", "note", "id") }},
		{"tag with space", func() ([]byte, error) { return BuildTagKey("in progress", "note", "id") }},
		{"type at byte cap", func() ([]byte, error) { return BuildNodeKey(atCap, "id") }},
		{"type multibyte at byte cap", func() ([]byte, error) { return BuildNodeKey(multibyteAtCap, "id") }},
		{"id at byte cap", func() ([]byte, error) { return BuildNodeKey("note", record.NodeID(atCap)) }},
		{"tag at byte cap", func() ([]byte, error) { return BuildTagKey(atCap, "note", "id") }},
		{"tag type at byte cap", func() ([]byte, error) { return BuildTagKey("active", atCap, "id") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.fn(); err != nil {
				t.Fatalf("err = %v, want nil", err)
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
		{"edge incomplete", []byte("e:note:a:rel:note"), func(k []byte) error { _, err := ParseEdgeKey(k); return err }},
		{"edge empty component", []byte("e:note:a::note:b"), func(k []byte) error { _, err := ParseEdgeKey(k); return err }},
		{"edge index wrong prefix", []byte("e:note:b:rel:note:a"), func(k []byte) error { _, err := ParseEdgeIndexKey(k); return err }},
		{"tag over byte cap", []byte("t:" + strings.Repeat("a", 65) + ":id"), func(k []byte) error { _, err := ParseTagKey(k); return err }},
		{"tag too few components", []byte("t:active"), func(k []byte) error { _, err := ParseTagKey(k); return err }},
		{"tag too many components", []byte("t:active:note:id:extra"), func(k []byte) error { _, err := ParseTagKey(k); return err }},
		{"tag v2 empty type", []byte("t:active::id"), func(k []byte) error { _, err := ParseTagKey(k); return err }},
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
	edge, _ := BuildEdgeKey("note", "a", "rel", "note", "b")
	inbound, _ := BuildEdgeIndexKey("note", "b", "rel", "note", "a")
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
