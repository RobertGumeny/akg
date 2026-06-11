//go:build unix

package akg

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestCommitPreservesFilePermissions verifies a commit preserves the target
// file's existing permission bits. Umask is pinned to 0 so the assertion is
// deterministic regardless of the test environment. This test relies on
// syscall.Umask, so it is unix-only; the cross-platform crash tests live in
// atomic_test.go.
func TestCommitPreservesFilePermissions(t *testing.T) {
	old := syscall.Umask(0)
	defer syscall.Umask(old)

	dir := t.TempDir()
	path := filepath.Join(dir, "store.akg")

	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "first"}, nil); err != nil {
		t.Fatalf("PutNode n1: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatalf("Chmod: %v", err)
	}

	if _, err := st.PutNode("note", "n2", NodeFields{Title: "second"}, nil); err != nil {
		t.Fatalf("PutNode n2: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("second Commit: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("permissions not preserved across commit: got %o, want 0640", got)
	}
}
