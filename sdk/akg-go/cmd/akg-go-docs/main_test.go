package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden snapshot files")

func runCmd(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	code := run(args, &outBuf, &errBuf)
	return outBuf.String(), errBuf.String(), code
}

func checkGolden(t *testing.T, goldenFile, got string) {
	t.Helper()
	path := filepath.Join("testdata", goldenFile)
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file missing (run with -update to create): %v\npath: %s", err, path)
	}
	if got != string(want) {
		t.Errorf("output mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", goldenFile, got, want)
	}
}

func TestOverview(t *testing.T) {
	stdout, _, code := runCmd(t, "overview")
	if code != 0 {
		t.Fatalf("overview exited %d", code)
	}
	checkGolden(t, "overview.golden", stdout)
}

func TestExplainPutNode(t *testing.T) {
	stdout, _, code := runCmd(t, "explain", "PutNode")
	if code != 0 {
		t.Fatalf("explain PutNode exited %d", code)
	}
	checkGolden(t, "explain_putnode.golden", stdout)
}

func TestSearchCommit(t *testing.T) {
	stdout, _, code := runCmd(t, "search", "commit")
	if code != 0 {
		t.Fatalf("search commit exited %d", code)
	}
	checkGolden(t, "search_commit.golden", stdout)
}

func TestDumpJSON(t *testing.T) {
	stdout, _, code := runCmd(t, "dump", "--format", "json")
	if code != 0 {
		t.Fatalf("dump --format json exited %d", code)
	}
	checkGolden(t, "dump_json.golden", stdout)
}

func TestOverviewHasAPISymbolSection(t *testing.T) {
	stdout, _, code := runCmd(t, "overview")
	if code != 0 {
		t.Fatalf("overview exited %d", code)
	}
	if !bytes.Contains([]byte(stdout), []byte("## api_symbol")) {
		t.Error("overview output missing ## api_symbol section")
	}
}

func TestExplainPutNodeContent(t *testing.T) {
	stdout, _, code := runCmd(t, "explain", "PutNode")
	if code != 0 {
		t.Fatalf("explain PutNode exited %d", code)
	}
	if !bytes.Contains([]byte(stdout), []byte("PutNode")) {
		t.Error("explain PutNode output missing 'PutNode'")
	}
	if !bytes.Contains([]byte(stdout), []byte("### ")) {
		t.Error("explain PutNode output missing relation heading")
	}
}

func TestSearchDeleteContainsDeleteNodeAndEdge(t *testing.T) {
	stdout, _, code := runCmd(t, "search", "delete")
	if code != 0 {
		t.Fatalf("search delete exited %d", code)
	}
	if !bytes.Contains([]byte(stdout), []byte("DeleteNode")) {
		t.Error("search delete output missing 'DeleteNode'")
	}
	if !bytes.Contains([]byte(stdout), []byte("DeleteEdge")) {
		t.Error("search delete output missing 'DeleteEdge'")
	}
}

func TestDumpJSONIsValidJSON(t *testing.T) {
	stdout, _, code := runCmd(t, "dump", "--format", "json")
	if code != 0 {
		t.Fatalf("dump --format json exited %d", code)
	}
	if !bytes.Contains([]byte(stdout), []byte("\"nodes\"")) {
		t.Error("dump json output missing nodes field")
	}
	if !bytes.Contains([]byte(stdout), []byte("\"edges\"")) {
		t.Error("dump json output missing edges field")
	}
}

func TestExplainUnknownExits1(t *testing.T) {
	_, stderr, code := runCmd(t, "explain", "UnknownSymbol")
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !bytes.Contains([]byte(stderr), []byte("Not found")) {
		t.Errorf("expected 'Not found' in stderr, got: %s", stderr)
	}
}
