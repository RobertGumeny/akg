package akg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// goldenParityPath is the cross-SDK byte-parity golden for the canonical
// commit-append sequence. The reference SDK (internal/store), akg-go, and akg-ts
// each reproduce the sequence and must produce byte-identical output. If any one
// diverges on the write path — re-materializing Data, encoding the WAL
// differently, or emitting a different container — its test fails against this
// golden. Regenerate with WRITE_PARITY_GOLDEN=1 (and re-run the other two suites).
func goldenParityPath() string {
	return filepath.Join("..", "..", "testdata", "behavior", "parity-commit-append.akg")
}

// parityAppendSequence applies the canonical commit-append sequence to a fresh
// store at path with a constant clock, then returns the resulting file bytes.
// Constant (not ticking) so the byte output cannot depend on how many times an
// implementation samples its clock per mutation: every record is stamped
// 1_000_000. The sequence is: PutNode n1 / Commit / PutNode n2 / Commit, which
// must leave Data+Bloom empty and grow the WAL to four records.
func parityAppendSequence(t *testing.T, path string) []byte {
	t.Helper()
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	st.state.now = func() timestampMicros { return 1_000_000 }
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "One"}, nil); err != nil {
		t.Fatalf("PutNode n1: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit 1: %v", err)
	}
	if _, err := st.PutNode("note", "n2", NodeFields{Title: "Two"}, nil); err != nil {
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
	got := parityAppendSequence(t, filepath.Join(t.TempDir(), "out.akg"))

	if os.Getenv("WRITE_PARITY_GOLDEN") != "" {
		if err := os.WriteFile(goldenParityPath(), got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("wrote parity golden (%d bytes)", len(got))
		return
	}

	want, err := os.ReadFile(goldenParityPath())
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("commit-append bytes diverge from cross-SDK golden (got %d bytes, want %d)", len(got), len(want))
	}
}

// TestCommitDoesNotRematerializeData proves CONF-3's logical-append contract from
// a non-empty baseline: after compacting one node into the Data section, a
// single-record commit must leave Data and Bloom byte-identical and only grow the
// WAL. The pre-CONF-3 akg-go behavior re-materialized Data every commit, which
// this catches.
func TestCommitDoesNotRematerializeData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rematerialize.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	st.state.now = func() timestampMicros { return 1_000_000 }
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "One"}, nil); err != nil {
		t.Fatalf("PutNode n1: %v", err)
	}
	if err := st.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read before: %v", err)
	}
	cBefore, err := decodeContainer(before)
	if err != nil {
		t.Fatalf("decode before: %v", err)
	}

	if _, err := st.PutNode("note", "n2", NodeFields{Title: "Two"}, nil); err != nil {
		t.Fatalf("PutNode n2: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	cAfter, err := decodeContainer(after)
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

	// The appended node must survive a reopen (it lives only in the WAL now).
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	re, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if n, err := re.GetNode("note", "n2"); err != nil || n == nil {
		t.Fatalf("appended node n2 not durable after reopen (node=%v err=%v)", n, err)
	}
}
