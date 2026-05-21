package state

import (
	"errors"
	"regexp"
	"testing"

	"github.com/RobertGumeny/akg-format/internal/keys"
	"github.com/RobertGumeny/akg-format/internal/record"
)

func tickingClock(start record.TimestampMicros) func() record.TimestampMicros {
	now := start
	return func() record.TimestampMicros {
		v := now
		now++
		return v
	}
}

func TestPutNodeUpsertIncrementsVersionAndOwnsTimestamps(t *testing.T) {
	s := New(WithNow(tickingClock(100)))

	created, err := s.PutNode("n1", record.Node{
		Type: "person", Title: "Sam", CreatedAt: 1, UpdatedAt: 2, Version: 99,
		Tags: []string{"preference"},
	})
	if err != nil {
		t.Fatalf("PutNode create: %v", err)
	}
	if created.ID != "n1" || created.Node.CreatedAt != 100 || created.Node.UpdatedAt != 100 || created.Node.Version != 1 {
		t.Fatalf("unexpected created node: %+v", created)
	}

	updated, err := s.PutNode("n1", record.Node{Type: "person", Title: "Samuel", Body: "updated"})
	if err != nil {
		t.Fatalf("PutNode update: %v", err)
	}
	if updated.Node.CreatedAt != 100 || updated.Node.UpdatedAt != 101 || updated.Node.Version != 2 {
		t.Fatalf("unexpected updated writer-owned fields: %+v", updated.Node)
	}
	got, ok := s.GetNode("person", "n1")
	if !ok || got.Node.Title != "Samuel" || got.Node.Body != "updated" {
		t.Fatalf("upsert did not replace current node: ok=%v got=%+v", ok, got)
	}
}

func TestPutEdgeUpsertIncrementsVersionAndAllowsDangling(t *testing.T) {
	s := New(WithNow(tickingClock(200)))

	created, err := s.PutEdge(record.Edge{FromNode: "missing-a", Relation: "prefers", ToNode: "missing-b", Strength: 0.7})
	if err != nil {
		t.Fatalf("PutEdge create dangling: %v", err)
	}
	if created.CreatedAt != 200 || created.UpdatedAt != 200 || created.Version != 1 {
		t.Fatalf("unexpected created edge: %+v", created)
	}

	updated, err := s.PutEdge(record.Edge{FromNode: "missing-a", Relation: "prefers", ToNode: "missing-b", Strength: 1.0})
	if err != nil {
		t.Fatalf("PutEdge update: %v", err)
	}
	if updated.CreatedAt != 200 || updated.UpdatedAt != 201 || updated.Version != 2 || updated.Strength != 1.0 {
		t.Fatalf("unexpected updated edge: %+v", updated)
	}
}

func TestGeneratedNodeIDIs16LowerHexAndValid(t *testing.T) {
	s := New(WithNow(tickingClock(1)))
	rec, err := s.PutNode("", record.Node{Type: "note", Title: "Generated"})
	if err != nil {
		t.Fatalf("PutNode generated ID: %v", err)
	}
	if ok := regexp.MustCompile(`^[0-9a-f]{16}$`).MatchString(string(rec.ID)); !ok {
		t.Fatalf("generated ID is not 16 lowercase hex chars: %q", rec.ID)
	}
	if _, err := keys.BuildNodeKey(rec.Node.Type, rec.ID); err != nil {
		t.Fatalf("generated ID did not satisfy key constraints: %v", err)
	}
}

func TestPutNodeRejectsInvalidIDsAndTags(t *testing.T) {
	tests := []struct {
		name string
		id   record.NodeID
		tags []string
	}{
		{name: "id with colon", id: "bad:id"},
		{name: "id too long", id: "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklm"},
		{name: "duplicate tags", id: "n1", tags: []string{"a", "a"}},
		{name: "uppercase tag", id: "n1", tags: []string{"Bad"}},
		{name: "tag with space", id: "n1", tags: []string{"bad tag"}},
		{name: "malformed tag leading underscore", id: "n1", tags: []string{"_bad"}},
		{name: "too many tags", id: "n1", tags: []string{
			"t00", "t01", "t02", "t03", "t04", "t05", "t06", "t07",
			"t08", "t09", "t10", "t11", "t12", "t13", "t14", "t15",
			"t16", "t17", "t18", "t19", "t20", "t21", "t22", "t23",
			"t24", "t25", "t26", "t27", "t28", "t29", "t30", "t31", "t32",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(WithNow(tickingClock(1)))
			_, err := s.PutNode(tt.id, record.Node{Type: "note", Title: "Bad", Tags: tt.tags})
			if err == nil {
				t.Fatalf("expected rejection")
			}
		})
	}
}

func TestStrictDeleteNotFoundAndSuccessfulDeletes(t *testing.T) {
	s := New(WithNow(tickingClock(1)))
	if err := s.DeleteNode("person", "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteNode missing error = %v, want ErrNotFound", err)
	}
	if err := s.DeleteEdge("a", "rel", "b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteEdge missing error = %v, want ErrNotFound", err)
	}
	if _, err := s.PutNode("n1", record.Node{Type: "person", Title: "Sam"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutEdge(record.Edge{FromNode: "a", Relation: "rel", ToNode: "b"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteNode("person", "n1"); err != nil {
		t.Fatalf("DeleteNode existing: %v", err)
	}
	if _, ok := s.GetNode("person", "n1"); ok {
		t.Fatalf("deleted node still present")
	}
	if err := s.DeleteEdge("a", "rel", "b"); err != nil {
		t.Fatalf("DeleteEdge existing: %v", err)
	}
	if _, ok := s.GetEdge("a", "rel", "b"); ok {
		t.Fatalf("deleted edge still present")
	}
}

func TestTypeChangeIsIdentityChange(t *testing.T) {
	s := New(WithNow(tickingClock(10)))
	if _, err := s.PutNode("same-id", record.Node{Type: "person", Title: "Sam"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutNode("same-id", record.Node{Type: "decision", Title: "Sam as decision"}); err != nil {
		t.Fatal(err)
	}
	person, ok := s.GetNode("person", "same-id")
	if !ok || person.Node.Title != "Sam" || person.Node.Version != 1 {
		t.Fatalf("original typed identity was not preserved: ok=%v rec=%+v", ok, person)
	}
	decision, ok := s.GetNode("decision", "same-id")
	if !ok || decision.Node.Title != "Sam as decision" || decision.Node.Version != 1 {
		t.Fatalf("new typed identity missing: ok=%v rec=%+v", ok, decision)
	}
	if got := len(s.Nodes()); got != 2 {
		t.Fatalf("len Nodes = %d, want 2", got)
	}
}

func TestPutEdgeRejectsInvalidKeyComponents(t *testing.T) {
	s := New(WithNow(tickingClock(1)))
	if _, err := s.PutEdge(record.Edge{FromNode: "bad:from", Relation: "rel", ToNode: "b"}); err == nil {
		t.Fatalf("expected invalid from_node rejection")
	}
	if _, err := s.PutEdge(record.Edge{FromNode: "a", Relation: "bad:rel", ToNode: "b"}); err == nil {
		t.Fatalf("expected invalid relation rejection")
	}
}
