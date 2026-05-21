package akg_test

import (
	"path/filepath"
	"reflect"
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

func TestPublicAPIReadHelpersStayMinimal(t *testing.T) {
	storeType := reflect.TypeOf((*akg.Store)(nil))
	allowed := map[string]struct{}{
		"Close":      {},
		"Commit":     {},
		"Compact":    {},
		"DeleteEdge": {},
		"DeleteNode": {},
		"GetEdge":    {},
		"GetNode":    {},
		"ListEdges":  {},
		"ListNodes":  {},
		"PutEdge":    {},
		"PutNode":    {},
	}

	for i := 0; i < storeType.NumMethod(); i++ {
		method := storeType.Method(i).Name
		if _, ok := allowed[method]; !ok {
			t.Fatalf("unexpected exported Store method %q; v1 read helpers are exact lookup plus whole-state lists only", method)
		}
		delete(allowed, method)
	}
	if len(allowed) != 0 {
		t.Fatalf("missing expected Store methods: %#v", allowed)
	}
}

func TestPublicAPIValidateCreateCommitOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lifecycle.akg")
	st, err := akg.Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := akg.Validate(path); err != nil {
		t.Fatalf("Validate empty file: %v", err)
	}
	if _, err := st.PutNode("a", akg.Node{Type: "note", Title: "A"}); err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := akg.Validate(path); err != nil {
		t.Fatalf("Validate committed file: %v", err)
	}

	reopened, err := akg.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if node, ok := reopened.GetNode("note", "a"); !ok || node.Node.Title != "A" {
		t.Fatalf("GetNode = %#v, %v", node, ok)
	}
}

func TestPublicAPIEdgeReadsAndDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "edges.akg")
	st, err := akg.Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := st.PutNode("a", akg.Node{Type: "note", Title: "A"}); err != nil {
		t.Fatalf("PutNode a: %v", err)
	}
	if _, err := st.PutNode("b", akg.Node{Type: "note", Title: "B"}); err != nil {
		t.Fatalf("PutNode b: %v", err)
	}
	if _, err := st.PutEdge(akg.Edge{FromNode: "a", Relation: "links", ToNode: "b"}); err != nil {
		t.Fatalf("PutEdge: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	reopened, err := akg.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if edge, ok := reopened.GetEdge("a", "links", "b"); !ok || edge.Relation != "links" {
		t.Fatalf("GetEdge = %#v, %v", edge, ok)
	}
	if edges := reopened.ListEdges(); len(edges) != 1 {
		t.Fatalf("ListEdges = %#v, want one edge", edges)
	}
	if err := reopened.DeleteEdge("a", "links", "b"); err != nil {
		t.Fatalf("DeleteEdge: %v", err)
	}
	if err := reopened.Commit(); err != nil {
		t.Fatalf("Commit delete: %v", err)
	}

	deleted, err := akg.Open(path)
	if err != nil {
		t.Fatalf("Open after delete: %v", err)
	}
	if _, ok := deleted.GetEdge("a", "links", "b"); ok {
		t.Fatalf("deleted edge is visible")
	}
	if edges := deleted.ListEdges(); len(edges) != 0 {
		t.Fatalf("ListEdges after delete = %#v, want no edges", edges)
	}
}

func TestPublicAPICloseCommitsPendingMutations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "close.akg")
	st, err := akg.Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := st.PutNode("a", akg.Node{Type: "note", Title: "A"}); err != nil {
		t.Fatalf("PutNode a: %v", err)
	}
	if _, err := st.PutNode("b", akg.Node{Type: "note", Title: "B"}); err != nil {
		t.Fatalf("PutNode b: %v", err)
	}
	if err := st.DeleteNode("note", "b"); err != nil {
		t.Fatalf("DeleteNode b: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := akg.Open(path)
	if err != nil {
		t.Fatalf("Open after Close: %v", err)
	}
	if _, ok := reopened.GetNode("note", "b"); ok {
		t.Fatalf("deleted node is visible after Close")
	}
	if nodes := reopened.ListNodes(); len(nodes) != 1 {
		t.Fatalf("ListNodes after Close = %#v, want one node", nodes)
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
		t.Fatalf("Store.Compact: %v", err)
	}
	if err := akg.Compact(path); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if err := akg.Validate(path); err != nil {
		t.Fatalf("Validate compacted file: %v", err)
	}
	reopened, err := akg.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(reopened.ListNodes()) != 2 || len(reopened.ListEdges()) != 1 {
		t.Fatalf("unexpected compacted state: nodes=%#v edges=%#v", reopened.ListNodes(), reopened.ListEdges())
	}
}
