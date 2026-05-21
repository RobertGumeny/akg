package store

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/earendil-works/akg/internal/format"
	"github.com/earendil-works/akg/internal/record"
	"github.com/earendil-works/akg/internal/state"
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
	if _, err := s.PutEdge(record.Edge{FromNode: "n1", Relation: "knows", ToNode: "n2", Strength: 0.9}); err != nil {
		t.Fatal(err)
	}

	entries, err := MaterializeDataEntries(s)
	if err != nil {
		t.Fatalf("MaterializeDataEntries: %v", err)
	}
	got := entryMap(entries)
	wantKeys := []string{
		"e:n1:knows:n2",
		"ei:n2:knows:n1",
		"n:person:n1",
		"n:person:n2",
		"t:alpha:n1",
		"t:beta:n2",
		"t:graph:n2",
		"ts:1000:n:person:n2",
		"ts:2000:n:person:n1",
		"ts:3000:e:n1:knows:n2",
	}
	if !reflect.DeepEqual(keysOf(entries), wantKeys) {
		t.Fatalf("keys mismatch\ngot  %q\nwant %q", keysOf(entries), wantKeys)
	}
	for _, key := range []string{"ei:n2:knows:n1", "t:alpha:n1", "t:beta:n2", "t:graph:n2", "ts:1000:n:person:n2", "ts:2000:n:person:n1", "ts:3000:e:n1:knows:n2"} {
		if value := got[key]; len(value) != 0 {
			t.Fatalf("derived key %q value len = %d, want 0", key, len(value))
		}
	}
	if _, err := record.DecodeNodePayload(got["n:person:n1"]); err != nil {
		t.Fatalf("node payload not canonical-decodable: %v", err)
	}
	if _, err := record.DecodeEdgePayload(got["e:n1:knows:n2"]); err != nil {
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
	if _, err := s.PutEdge(record.Edge{FromNode: "b", Relation: "rel", ToNode: "a"}); err != nil {
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
	if _, err := s.PutEdge(record.Edge{FromNode: "n1", Relation: "links", ToNode: "n2"}); err != nil {
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
		if key == "ei:n2:links:n1" || key == "t:topic:n1" || key == "ts:1:n:note:n1" || key == "ts:2:e:n1:links:n2" {
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
