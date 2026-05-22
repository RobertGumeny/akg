package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	akg "github.com/RobertGumeny/akg-go"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gen_delete_fixtures <output-dir>")
		os.Exit(1)
	}
	outDir := os.Args[1]

	fixtures := []struct {
		name string
		fn   func(path string) error
	}{
		{"m2-node-deleted-before-commit.akg", genNodeDeletedBeforeCommit},
		{"m2-node-deletion-survives-reopen.akg", genNodeDeletionSurvivesReopen},
		{"m2-edge-deleted-before-commit.akg", genEdgeDeletedBeforeCommit},
		{"m2-edge-deletion-survives-reopen.akg", genEdgeDeletionSurvivesReopen},
	}

	for _, fx := range fixtures {
		path := filepath.Join(outDir, fx.name)
		if err := fx.fn(path); err != nil {
			fmt.Fprintf(os.Stderr, "generate %s: %v\n", fx.name, err)
			os.Exit(1)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", fx.name, err)
			os.Exit(1)
		}
		sum := sha256.Sum256(data)
		fmt.Printf("%s  sha256:%s\n", fx.name, hex.EncodeToString(sum[:]))
	}
}

// genNodeDeletedBeforeCommit: PutNode + DeleteNode in a single commit.
// WAL: PUT_NODE@1, DELETE_NODE@2, COMMIT@3 → nextWALSeq=4
func genNodeDeletedBeforeCommit(path string) error {
	st, err := akg.Open(path)
	if err != nil {
		return err
	}
	if _, err := st.PutNode("note", "n1", akg.NodeFields{Title: "hello"}, nil); err != nil {
		return err
	}
	if err := st.DeleteNode("note", "n1"); err != nil {
		return err
	}
	return st.Close()
}

// genNodeDeletionSurvivesReopen: PutNode committed, then DeleteNode in a second commit.
// Final WAL: PUT_NODE@1, COMMIT@2, DELETE_NODE@3, COMMIT@4 → nextWALSeq=5
func genNodeDeletionSurvivesReopen(path string) error {
	st, err := akg.Open(path)
	if err != nil {
		return err
	}
	if _, err := st.PutNode("note", "n1", akg.NodeFields{Title: "hello"}, nil); err != nil {
		return err
	}
	if err := st.Close(); err != nil {
		return err
	}

	st2, err := akg.Open(path)
	if err != nil {
		return err
	}
	if err := st2.DeleteNode("note", "n1"); err != nil {
		return err
	}
	return st2.Close()
}

// genEdgeDeletedBeforeCommit: PutNode×2, PutEdge, DeleteEdge in a single commit.
// WAL: PUT_NODE@1, PUT_NODE@2, PUT_EDGE@3, DELETE_EDGE@4, COMMIT@5 → nextWALSeq=6
func genEdgeDeletedBeforeCommit(path string) error {
	st, err := akg.Open(path)
	if err != nil {
		return err
	}
	if _, err := st.PutNode("note", "n1", akg.NodeFields{Title: "one"}, nil); err != nil {
		return err
	}
	if _, err := st.PutNode("note", "n2", akg.NodeFields{Title: "two"}, nil); err != nil {
		return err
	}
	if err := st.PutEdge(akg.NodeRef{Type: "note", ID: "n1"}, "links_to", akg.NodeRef{Type: "note", ID: "n2"}, akg.EdgeFields{}); err != nil {
		return err
	}
	if err := st.DeleteEdge(akg.NodeRef{Type: "note", ID: "n1"}, "links_to", akg.NodeRef{Type: "note", ID: "n2"}); err != nil {
		return err
	}
	return st.Close()
}

// genEdgeDeletionSurvivesReopen: PutNode×2+PutEdge committed, then DeleteEdge in a second commit.
// Final WAL: PUT_NODE@1, PUT_NODE@2, PUT_EDGE@3, COMMIT@4, DELETE_EDGE@5, COMMIT@6 → nextWALSeq=7
func genEdgeDeletionSurvivesReopen(path string) error {
	st, err := akg.Open(path)
	if err != nil {
		return err
	}
	if _, err := st.PutNode("note", "n1", akg.NodeFields{Title: "one"}, nil); err != nil {
		return err
	}
	if _, err := st.PutNode("note", "n2", akg.NodeFields{Title: "two"}, nil); err != nil {
		return err
	}
	if err := st.PutEdge(akg.NodeRef{Type: "note", ID: "n1"}, "links_to", akg.NodeRef{Type: "note", ID: "n2"}, akg.EdgeFields{}); err != nil {
		return err
	}
	if err := st.Close(); err != nil {
		return err
	}

	st2, err := akg.Open(path)
	if err != nil {
		return err
	}
	if err := st2.DeleteEdge(akg.NodeRef{Type: "note", ID: "n1"}, "links_to", akg.NodeRef{Type: "note", ID: "n2"}); err != nil {
		return err
	}
	return st2.Close()
}
