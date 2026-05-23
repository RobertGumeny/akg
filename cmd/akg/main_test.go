package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RobertGumeny/akg"
)

func TestCLIValidateSucceedsAndFailsClearly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "valid.akg")
	if _, err := akg.Create(path); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"validate", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("validate code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "valid") {
		t.Fatalf("validate stdout = %q", stdout.String())
	}

	badPath := filepath.Join(t.TempDir(), "bad.akg")
	if err := os.WriteFile(badPath, []byte("not an akg file"), 0o666); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"validate", badPath}, &stdout, &stderr); code == 0 {
		t.Fatalf("validate corrupt file succeeded")
	}
	if !strings.Contains(stderr.String(), "validation failed") {
		t.Fatalf("corrupt validation stderr = %q", stderr.String())
	}
}

func TestCLIInspectShowsOnlyCurrentState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inspect.akg")
	st, err := akg.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutNode("n1", akg.Node{Type: "note", Title: "old"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutNode("n1", akg.Node{Type: "note", Title: "current"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutNode("n2", akg.Node{Type: "note", Title: "deleted"}); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteNode("note", "n2"); err != nil {
		t.Fatal(err)
	}
	if err := st.Commit(); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := run([]string{"inspect", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("inspect code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var out inspectOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("inspect JSON: %v\n%s", err, stdout.String())
	}
	if out.NodeCount != 1 || len(out.Nodes) != 1 || out.Nodes[0].ID != "n1" || out.Nodes[0].Node.Title != "current" {
		t.Fatalf("inspect exposed stale/deleted state: %#v", out)
	}
}

func TestCLICompact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "compact.akg")
	st, err := akg.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutNode("n1", akg.Node{Type: "note", Title: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Commit(); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"compact", path}, &stdout, &stderr); code != 0 {
		t.Fatalf("compact code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if err := akg.Validate(path); err != nil {
		t.Fatalf("Validate compacted: %v", err)
	}
}
