package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	akg "github.com/RobertGumeny/akg/sdk/akg-go"
)

func runShowCmd(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	code := runShow(args, &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

// writeFixture builds a small opponent/pattern graph in a temp .akg file, the
// same shape the durable poker agent produces, and returns its path.
func writeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "memory.akg")
	store, err := akg.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	opp, err := store.PutNode("opponent", "villain", akg.NodeFields{
		Title: "villain",
		Body:  "Villain is loose-aggressive (VPIP 68%, PFR 30%).",
	}, nil)
	if err != nil {
		t.Fatalf("put opponent: %v", err)
	}
	pat, err := store.PutNode("pattern", "folds-to-cbet", akg.NodeFields{
		Title: "Folds to flop c-bets",
		Body:  "Villain folded to hero flop c-bet 36/75 times.",
	}, nil)
	if err != nil {
		t.Fatalf("put pattern: %v", err)
	}
	if err := store.PutEdge(opp, "shows_pattern", pat, akg.EdgeFields{}); err != nil {
		t.Fatalf("put edge: %v", err)
	}
	if err := store.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return path
}

func TestShowRendersGroupedKnowledge(t *testing.T) {
	stdout, _, code := runShowCmd(t, writeFixture(t))
	if code != 0 {
		t.Fatalf("show exited %d", code)
	}
	for _, want := range []string{
		"2 nodes / 1 edges",
		"OPPONENT (1)",
		"Villain is loose-aggressive (VPIP 68%, PFR 30%).",
		"PATTERN (1)",
		"Folds to flop c-bets",
		"EDGES (1)",
		"opponent/villain -shows_pattern-> pattern/folds-to-cbet",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("show output missing %q\n--- got ---\n%s", want, stdout)
		}
	}
}

func TestShowJSONEmitsSnapshot(t *testing.T) {
	stdout, _, code := runShowCmd(t, "--json", writeFixture(t))
	if code != 0 {
		t.Fatalf("show --json exited %d", code)
	}
	if !strings.Contains(stdout, "\"nodes\"") || !strings.Contains(stdout, "\"edges\"") {
		t.Errorf("show --json missing nodes/edges fields\n%s", stdout)
	}
}

func TestShowMissingFileExits1(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.akg")
	_, stderr, code := runShowCmd(t, missing)
	if code != 1 {
		t.Fatalf("expected exit 1 for missing file, got %d", code)
	}
	if !strings.Contains(stderr, "cannot read") {
		t.Errorf("expected 'cannot read' in stderr, got: %s", stderr)
	}
}

func TestShowCollapsesLargeTypes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "memory.akg")
	store, err := akg.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for i := 0; i < collapseThreshold+3; i++ {
		id := "h" + string(rune('a'+i))
		if _, err := store.PutNode("hand", id, akg.NodeFields{Title: "Hand " + id}, nil); err != nil {
			t.Fatalf("put hand: %v", err)
		}
	}
	if err := store.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	collapsed, _, code := runShowCmd(t, path)
	if code != 0 {
		t.Fatalf("show exited %d", code)
	}
	if !strings.Contains(collapsed, "more (pass --all to show every node)") {
		t.Errorf("expected collapse summary for a large type, got:\n%s", collapsed)
	}

	all, _, code := runShowCmd(t, "--all", path)
	if code != 0 {
		t.Fatalf("show --all exited %d", code)
	}
	if strings.Contains(all, "more (pass --all") {
		t.Errorf("--all should not collapse, got:\n%s", all)
	}
}
