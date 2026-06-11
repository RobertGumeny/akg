package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/RobertGumeny/akg/internal/format"
	"github.com/RobertGumeny/akg/internal/record"
	"github.com/RobertGumeny/akg/internal/state"
)

// goldenParityPath is the cross-SDK byte-parity golden for the canonical
// commit-append sequence shared by all three first-party implementations
// (reference, akg-go, akg-ts). See sdk/akg-go/commit_parity_test.go for the
// authoritative description; this is the reference SDK's reproduction.
func goldenParityPath() string {
	return filepath.Join("..", "..", "testdata", "behavior", "parity-commit-append.akg")
}

// referenceParityAppend applies the canonical commit-append sequence to a fresh
// reference store at path with a constant clock (every record stamped 1_000_000),
// then returns the resulting file bytes: PutNode n1 / Commit / PutNode n2 /
// Commit, which must leave Data+Bloom empty and grow the WAL to four records.
func referenceParityAppend(t *testing.T, path string) []byte {
	t.Helper()
	st, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Swap in a constant clock so the WAL timestamps match the cross-SDK golden.
	st.state = state.New(state.WithNow(fixedClock(1_000_000)))
	if _, err := st.PutNode("n1", record.Node{Type: "note", Title: "One"}); err != nil {
		t.Fatalf("PutNode n1: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit 1: %v", err)
	}
	if _, err := st.PutNode("n2", record.Node{Type: "note", Title: "Two"}); err != nil {
		t.Fatalf("PutNode n2: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit 2: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read store file: %v", err)
	}
	return got
}

func TestCommitAppendByteParity(t *testing.T) {
	want, err := os.ReadFile(goldenParityPath())
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	got := referenceParityAppend(t, filepath.Join(t.TempDir(), "out.akg"))
	if !bytes.Equal(got, want) {
		t.Fatalf("commit-append bytes diverge from cross-SDK golden (got %d bytes, want %d)", len(got), len(want))
	}
}

// TestCommitDoesNotRematerializeData proves CONF-3's logical-append contract from
// a non-empty baseline: after compacting one node into the Data section, a
// single-record commit must leave Data and Bloom byte-identical and only grow the
// WAL.
func TestCommitDoesNotRematerializeData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rematerialize.akg")
	base := state.New(state.WithNow(fixedClock(1_000_000)))
	if _, err := base.PutNode("n1", record.Node{Type: "note", Title: "One"}); err != nil {
		t.Fatalf("PutNode n1: %v", err)
	}
	writeStoreFile(t, path, base, nil) // compacted baseline, no WAL

	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	cBefore, _, err := format.DecodeContainer(before)
	if err != nil {
		t.Fatalf("decode before: %v", err)
	}

	if _, err := st.PutNode("n2", record.Node{Type: "note", Title: "Two"}); err != nil {
		t.Fatalf("PutNode n2: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	cAfter, _, err := format.DecodeContainer(after)
	if err != nil {
		t.Fatalf("decode after: %v", err)
	}

	if !bytes.Equal(cBefore.Data, cAfter.Data) {
		t.Fatal("Data section changed on commit — store re-materialized Data instead of appending to the WAL")
	}
	if !bytes.Equal(cBefore.Bloom, cAfter.Bloom) {
		t.Fatal("Bloom section changed on commit — Bloom is a Data index and must only change on compaction")
	}
	if len(cAfter.WAL) <= len(cBefore.WAL) {
		t.Fatalf("WAL did not grow on commit (before=%d after=%d)", len(cBefore.WAL), len(cAfter.WAL))
	}

	if _, ok := st.State().GetNode("note", "n2"); !ok {
		t.Fatal("appended node n2 not visible after commit")
	}
}
