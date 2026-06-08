package akg

import (
	"fmt"
	"path/filepath"
	"testing"
)

// TestAutoFlushCommitsAtEntryThreshold verifies the writer-side safety valve:
// once buffered mutations cross walEntryFlushThreshold, the store commits them
// durably without an explicit Commit/Close. A second store opened on the same
// path must therefore observe the records.
func TestAutoFlushCommitsAtEntryThreshold(t *testing.T) {
	path := filepath.Join(t.TempDir(), "autoflush.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Below the threshold nothing is flushed: a fresh open sees an empty graph.
	for i := 0; i < 10; i++ {
		if _, err := st.PutNode("note", fmt.Sprintf("n%d", i), NodeFields{Title: "t"}, nil); err != nil {
			t.Fatalf("PutNode %d: %v", i, err)
		}
	}
	if got := st.UncompactedWALEntries(); got != 0 {
		t.Fatalf("UncompactedWALEntries before flush = %d, want 0", got)
	}
	if peek, err := Open(path); err != nil {
		t.Fatalf("Open peek: %v", err)
	} else if nodes, _ := peek.ListNodes(""); len(nodes) != 0 {
		t.Fatalf("uncommitted nodes visible before flush: got %d, want 0", len(nodes))
	}

	// Cross the threshold; the valve must auto-commit.
	for i := 10; i < walEntryFlushThreshold; i++ {
		if _, err := st.PutNode("note", fmt.Sprintf("n%d", i), NodeFields{Title: "t"}, nil); err != nil {
			t.Fatalf("PutNode %d: %v", i, err)
		}
	}
	if got := st.UncompactedWALEntries(); got == 0 {
		t.Fatalf("UncompactedWALEntries after threshold = 0, want > 0 (valve did not fire)")
	}

	peek, err := Open(path)
	if err != nil {
		t.Fatalf("Open after flush: %v", err)
	}
	nodes, err := peek.ListNodes("")
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != walEntryFlushThreshold {
		t.Fatalf("auto-flushed node count = %d, want %d", len(nodes), walEntryFlushThreshold)
	}
}

// TestAutoFlushResetsAfterCompact confirms the uncompacted-WAL counters that
// drive the valve reset to zero after compaction.
func TestAutoFlushResetsAfterCompact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "autoflush-compact.akg")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "t"}, nil); err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if st.UncompactedWALEntries() == 0 || st.UncompactedWALBytes() == 0 {
		t.Fatalf("expected non-zero uncompacted WAL counters after commit")
	}
	if err := st.Compact(); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if got := st.UncompactedWALEntries(); got != 0 {
		t.Fatalf("UncompactedWALEntries after compact = %d, want 0", got)
	}
	if got := st.UncompactedWALBytes(); got != 0 {
		t.Fatalf("UncompactedWALBytes after compact = %d, want 0", got)
	}
}
