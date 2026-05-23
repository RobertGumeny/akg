package store

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/RobertGumeny/akg/internal/format"
	"github.com/RobertGumeny/akg/internal/record"
	"github.com/RobertGumeny/akg/internal/state"
)

func fixedClock(values ...record.TimestampMicros) func() record.TimestampMicros {
	i := 0
	return func() record.TimestampMicros {
		if i >= len(values) {
			return values[len(values)-1]
		}
		v := values[i]
		i++
		return v
	}
}

func TestMaterializeDataEntriesGeneratesPrimaryAndDerivedKeys(t *testing.T) {
	s := state.New(state.WithNow(fixedClock(1000, 2000, 3000)))
	if _, err := s.PutNode("n2", record.Node{Type: "person", Title: "Bea", Tags: []string{"beta", "graph"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutNode("n1", record.Node{Type: "person", Title: "Ada", Tags: []string{"alpha"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutEdge(record.Edge{FromType: "person", FromNode: "n1", Relation: "knows", ToType: "person", ToNode: "n2", Strength: 0.9}); err != nil {
		t.Fatal(err)
	}

	entries, err := MaterializeDataEntries(s)
	if err != nil {
		t.Fatalf("MaterializeDataEntries: %v", err)
	}
	got := entryMap(entries)
	wantKeys := []string{
		"e:person:n1:knows:person:n2",
		"ei:person:n2:knows:person:n1",
		"n:person:n1",
		"n:person:n2",
		"t:alpha:n1",
		"t:beta:n2",
		"t:graph:n2",
		"ts:1000:n:person:n2",
		"ts:2000:n:person:n1",
		"ts:3000:e:person:n1:knows:person:n2",
	}
	if !reflect.DeepEqual(keysOf(entries), wantKeys) {
		t.Fatalf("keys mismatch\ngot  %q\nwant %q", keysOf(entries), wantKeys)
	}
	for _, key := range []string{"ei:person:n2:knows:person:n1", "t:alpha:n1", "t:beta:n2", "t:graph:n2", "ts:1000:n:person:n2", "ts:2000:n:person:n1", "ts:3000:e:person:n1:knows:person:n2"} {
		if value := got[key]; len(value) != 0 {
			t.Fatalf("derived key %q value len = %d, want 0", key, len(value))
		}
	}
	if _, err := record.DecodeNodePayload(got["n:person:n1"]); err != nil {
		t.Fatalf("node payload not canonical-decodable: %v", err)
	}
	if _, err := record.DecodeEdgePayload(got["e:person:n1:knows:person:n2"]); err != nil {
		t.Fatalf("edge payload not canonical-decodable: %v", err)
	}
}

func TestMaterializeDataEntriesIsDeterministicAndSorted(t *testing.T) {
	s := state.New(state.WithNow(fixedClock(10, 20, 30)))
	if _, err := s.PutNode("b", record.Node{Type: "note", Title: "B", Tags: []string{"tag_b"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutNode("a", record.Node{Type: "note", Title: "A", Tags: []string{"tag_a"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutEdge(record.Edge{FromType: "note", FromNode: "b", Relation: "rel", ToType: "note", ToNode: "a"}); err != nil {
		t.Fatal(err)
	}

	first, err := MaterializeDataEntries(s)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MaterializeDataEntries(s)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("materialization is not deterministic")
	}
	for i := 1; i < len(first); i++ {
		if bytes.Compare(first[i-1].Key, first[i].Key) >= 0 {
			t.Fatalf("entries are not strictly sorted at %d: %q >= %q", i, first[i-1].Key, first[i].Key)
		}
	}
}

func TestMaterializedEntriesAreAcceptedByDataSectionDecoderWithEmptyIndexValues(t *testing.T) {
	s := state.New(state.WithNow(fixedClock(1, 2)))
	if _, err := s.PutNode("n1", record.Node{Type: "note", Title: "A", Tags: []string{"topic"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutEdge(record.Edge{FromType: "note", FromNode: "n1", Relation: "links", ToType: "note", ToNode: "n2"}); err != nil {
		t.Fatal(err)
	}
	entries, err := MaterializeDataEntries(s)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := format.EncodeDataEntries(entries)
	if err != nil {
		t.Fatalf("EncodeDataEntries: %v", err)
	}
	decoded, err := format.DecodeDataEntries(payload)
	if err != nil {
		t.Fatalf("DecodeDataEntries: %v", err)
	}
	for _, entry := range decoded {
		key := string(entry.Key)
		if key == "ei:note:n2:links:note:n1" || key == "t:topic:n1" || key == "ts:1:n:note:n1" || key == "ts:2:e:note:n1:links:note:n2" {
			if len(entry.Value) != 0 {
				t.Fatalf("%q decoded value len = %d, want 0", key, len(entry.Value))
			}
		}
	}
}

func TestMaterializeDataEntriesRejectsDuplicateDerivedKeys(t *testing.T) {
	s := state.New(state.WithNow(fixedClock(1, 2)))
	if _, err := s.PutNode("same", record.Node{Type: "person", Title: "A", Tags: []string{"shared"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutNode("same", record.Node{Type: "project", Title: "B", Tags: []string{"shared"}}); err != nil {
		t.Fatal(err)
	}
	_, err := MaterializeDataEntries(s)
	if !errors.Is(err, ErrDuplicateMaterializedKey) {
		t.Fatalf("MaterializeDataEntries error = %v, want ErrDuplicateMaterializedKey", err)
	}
}

func TestMaterializeDataEntriesOmitsDeletedAndSupersededRecords(t *testing.T) {
	s := state.New(state.WithNow(fixedClock(1, 2, 3, 4)))
	if _, err := s.PutNode("n1", record.Node{Type: "note", Title: "old", Tags: []string{"old_tag"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutNode("n1", record.Node{Type: "note", Title: "new", Tags: []string{"new_tag"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutNode("gone", record.Node{Type: "note", Title: "gone", Tags: []string{"gone_tag"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteNode("note", "gone"); err != nil {
		t.Fatal(err)
	}
	entries, err := MaterializeDataEntries(s)
	if err != nil {
		t.Fatal(err)
	}
	keys := keysOf(entries)
	for _, absent := range []string{"t:old_tag:n1", "ts:1:n:note:n1", "n:note:gone", "t:gone_tag:gone"} {
		if contains(keys, absent) {
			t.Fatalf("deleted or superseded key %q appeared in %q", absent, keys)
		}
	}
	for _, present := range []string{"n:note:n1", "t:new_tag:n1", "ts:2:n:note:n1"} {
		if !contains(keys, present) {
			t.Fatalf("current key %q missing from %q", present, keys)
		}
	}
}

func TestHydrateDataEntriesRoundTripsMaterializedState(t *testing.T) {
	s := state.New(state.WithNow(fixedClock(11, 22, 33)))
	if _, err := s.PutNode("a", record.Node{Type: "note", Title: "A", Tags: []string{"alpha"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutNode("b", record.Node{Type: "note", Title: "B"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutEdge(record.Edge{FromType: "note", FromNode: "a", Relation: "links", ToType: "note", ToNode: "b"}); err != nil {
		t.Fatal(err)
	}
	entries, err := MaterializeDataEntries(s)
	if err != nil {
		t.Fatal(err)
	}
	hydrated, err := HydrateDataEntries(entries)
	if err != nil {
		t.Fatalf("HydrateDataEntries: %v", err)
	}
	again, err := MaterializeDataEntries(hydrated)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(entries, again) {
		t.Fatalf("materialize/hydrate/materialize changed entries")
	}
}

func TestHydrateDataEntriesRejectsMalformedPrimaryAndIdentityMismatch(t *testing.T) {
	s := state.New(state.WithNow(fixedClock(1)))
	if _, err := s.PutNode("n1", record.Node{Type: "note", Title: "A"}); err != nil {
		t.Fatal(err)
	}
	entries, err := MaterializeDataEntries(s)
	if err != nil {
		t.Fatal(err)
	}
	badPayload := cloneEntries(entries)
	badPayload[indexOfKey(t, badPayload, "n:note:n1")].Value = []byte{0x80}
	if _, err := HydrateDataEntries(badPayload); err == nil {
		t.Fatalf("expected malformed primary payload rejection")
	}

	mismatch := cloneEntries(entries)
	mismatch[indexOfKey(t, mismatch, "n:note:n1")].Key = []byte("n:other:n1")
	if _, err := HydrateDataEntries(mismatch); !errors.Is(err, ErrIdentityMismatch) {
		t.Fatalf("HydrateDataEntries mismatch error = %v, want ErrIdentityMismatch", err)
	}
}

func TestHydrateDataEntriesValidatesDerivedIndexes(t *testing.T) {
	s := state.New(state.WithNow(fixedClock(1, 2)))
	if _, err := s.PutNode("n1", record.Node{Type: "note", Title: "A", Tags: []string{"topic"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutEdge(record.Edge{FromType: "note", FromNode: "n1", Relation: "links", ToType: "note", ToNode: "n2"}); err != nil {
		t.Fatal(err)
	}
	entries, err := MaterializeDataEntries(s)
	if err != nil {
		t.Fatal(err)
	}
	missing := withoutKey(entries, "t:topic:n1")
	if _, err := HydrateDataEntries(missing); !errors.Is(err, ErrDerivedIndexMismatch) {
		t.Fatalf("missing derived error = %v, want ErrDerivedIndexMismatch", err)
	}
	wrong := cloneEntries(entries)
	wrong[indexOfKey(t, wrong, "ei:note:n2:links:note:n1")].Key = []byte("ei:note:n2:wrong:note:n1")
	if _, err := HydrateDataEntries(wrong); !errors.Is(err, ErrDerivedIndexMismatch) {
		t.Fatalf("wrong derived error = %v, want ErrDerivedIndexMismatch", err)
	}
	nonEmpty := cloneEntries(entries)
	nonEmpty[indexOfKey(t, nonEmpty, "t:topic:n1")].Value = []byte("x")
	if _, err := HydrateDataEntries(nonEmpty); !errors.Is(err, ErrNonEmptyDerivedValue) {
		t.Fatalf("non-empty derived error = %v, want ErrNonEmptyDerivedValue", err)
	}
}

func TestHydrateDataEntriesDropsUnknownMessagePackFieldsAfterRewrite(t *testing.T) {
	entries := []format.DataEntry{
		{Key: []byte("n:note:n1"), Value: nodePayloadWithUnknownField()},
		{Key: []byte("ts:7:n:note:n1")},
	}
	hydrated, err := HydrateDataEntries(entries)
	if err != nil {
		t.Fatalf("HydrateDataEntries: %v", err)
	}
	rewritten, err := MaterializeDataEntries(hydrated)
	if err != nil {
		t.Fatal(err)
	}
	value := entryMap(rewritten)["n:note:n1"]
	if bytes.Equal(value, entries[0].Value) {
		t.Fatalf("node payload was not canonically rewritten")
	}
	node, err := record.DecodeNodePayload(value)
	if err != nil {
		t.Fatal(err)
	}
	if node.Type != "note" || node.Title != "A" || node.UpdatedAt != 7 {
		t.Fatalf("unexpected rewritten node: %+v", node)
	}
}

func keysOf(entries []format.DataEntry) []string {
	out := make([]string, len(entries))
	for i, entry := range entries {
		out[i] = string(entry.Key)
	}
	return out
}

func entryMap(entries []format.DataEntry) map[string][]byte {
	out := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		out[string(entry.Key)] = entry.Value
	}
	return out
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func cloneEntries(entries []format.DataEntry) []format.DataEntry {
	out := make([]format.DataEntry, len(entries))
	for i, entry := range entries {
		out[i] = format.DataEntry{
			Key:   append([]byte(nil), entry.Key...),
			Value: append([]byte(nil), entry.Value...),
		}
	}
	return out
}

func indexOfKey(t *testing.T, entries []format.DataEntry, key string) int {
	t.Helper()
	for i, entry := range entries {
		if string(entry.Key) == key {
			return i
		}
	}
	t.Fatalf("key %q not found in %q", key, keysOf(entries))
	return -1
}

func withoutKey(entries []format.DataEntry, key string) []format.DataEntry {
	out := make([]format.DataEntry, 0, len(entries))
	for _, entry := range entries {
		if string(entry.Key) != key {
			out = append(out, entry)
		}
	}
	return out
}

func nodePayloadWithUnknownField() []byte {
	var out []byte
	out = append(out, 0x85)
	appendMsgpackString(&out, "created_at")
	out = append(out, 0x07)
	appendMsgpackString(&out, "extra")
	appendMsgpackString(&out, "ignored")
	appendMsgpackString(&out, "title")
	appendMsgpackString(&out, "A")
	appendMsgpackString(&out, "type")
	appendMsgpackString(&out, "note")
	appendMsgpackString(&out, "updated_at")
	out = append(out, 0x07)
	return out
}

func appendMsgpackString(out *[]byte, s string) {
	*out = append(*out, 0xa0|byte(len(s)))
	*out = append(*out, s...)
}
