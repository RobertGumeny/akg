package akg

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// tempGlob matches the temp files writeFileAtomicRename creates.
func tempGlob(t *testing.T, dir, base string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "."+base+".akg.tmp-*"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	return matches
}

// TestCommitRenameFailureLeavesPriorFileIntact verifies that when the atomic
// rename fails mid-commit, the previously committed file is untouched and no
// temp file is left behind.
func TestCommitRenameFailureLeavesPriorFileIntact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.akg")
	base := filepath.Base(path)

	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "first"}, nil); err != nil {
		t.Fatalf("PutNode n1: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("baseline Commit: %v", err)
	}

	baseline, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile baseline: %v", err)
	}

	// Stage a second mutation, then make the rename fail on commit.
	if _, err := st.PutNode("note", "n2", NodeFields{Title: "second"}, nil); err != nil {
		t.Fatalf("PutNode n2: %v", err)
	}
	saved := osRename
	osRename = func(_, _ string) error { return errors.New("injected rename failure") }
	defer func() { osRename = saved }()

	if err := st.Commit(); err == nil {
		t.Fatal("expected Commit to fail when rename fails")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after: %v", err)
	}
	if !bytes.Equal(baseline, after) {
		t.Fatalf("prior committed file was modified by a failed commit: %d vs %d bytes", len(baseline), len(after))
	}
	if leftovers := tempGlob(t, dir, base); len(leftovers) != 0 {
		t.Fatalf("temp files left behind after failed commit: %v", leftovers)
	}
}

// TestOpenToleratesStrayTempFile verifies that a leftover atomic-write temp file
// in the store directory does not interfere with opening the store.
func TestOpenToleratesStrayTempFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "store.akg")
	base := filepath.Base(path)

	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := st.PutNode("note", "n1", NodeFields{Title: "hello"}, nil); err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	stray := filepath.Join(dir, "."+base+".akg.tmp-deadbeefdeadbeef")
	if err := os.WriteFile(stray, []byte("garbage that is not a valid container"), 0o600); err != nil {
		t.Fatalf("write stray temp: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("Open with stray temp present: %v", err)
	}
	node, err := reopened.GetNode("note", "n1")
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if node == nil {
		t.Fatal("committed node missing after reopen with stray temp present")
	}
}
