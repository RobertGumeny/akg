package akg_test

import (
	"path/filepath"
	"testing"

	"github.com/RobertGumeny/akg-format"
)

func TestPublicAPIExposesOnlyCurrentLogicalState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "api.akg")
	st, err := akg.Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := st.PutNode("n1", akg.Node{Type: "note", Title: "old"}); err != nil {
		t.Fatalf("PutNode old: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit old: %v", err)
	}
	if _, err := st.PutNode("n1", akg.Node{Type: "note", Title: "current"}); err != nil {
		t.Fatalf("PutNode current: %v", err)
	}
	if _, err := st.PutNode("n2", akg.Node{Type: "note", Title: "deleted"}); err != nil {
		t.Fatalf("PutNode deleted: %v", err)
	}
	if err := st.DeleteNode("note", "n2"); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit current: %v", err)
	}

	reopened, err := akg.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	nodes := reopened.ListNodes()
	if len(nodes) != 1 {
		t.Fatalf("ListNodes returned stale/deleted records: %#v", nodes)
	}
	got, ok := reopened.GetNode("note", "n1")
	if !ok || got.Node.Title != "current" {
		t.Fatalf("GetNode = %#v, %v; want current n1", got, ok)
	}
	if _, ok := reopened.GetNode("note", "n2"); ok {
		t.Fatalf("deleted node is visible through public API")
	}
}

func TestPublicAPICompactPreservesCurrentState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compact.akg")
	st, err := akg.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutNode("a", akg.Node{Type: "note", Title: "A"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutNode("b", akg.Node{Type: "note", Title: "B"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutEdge(akg.Edge{FromNode: "a", Relation: "links", ToNode: "b"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	reopened, err := akg.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(reopened.ListNodes()) != 2 || len(reopened.ListEdges()) != 1 {
		t.Fatalf("unexpected compacted state: nodes=%#v edges=%#v", reopened.ListNodes(), reopened.ListEdges())
	}
}
