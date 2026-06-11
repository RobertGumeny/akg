package akg

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestDocManifestMatchesExportedSurface defends DOC-2: it ties docs/manifest.json
// (the source the doc-graph is generated from) to the akg package's actual
// exported surface. CI already guarantees the .akg graph matches the manifest;
// this guarantees the manifest matches reality. Add an exported symbol without
// documenting it — or document a symbol that no longer exists — and this fails.
//
// Two curated maps capture the deliberate, reviewed exceptions:
//   - docManifestAliases maps a manifest id to the real identifier it documents,
//     for the one case where a bare name collides (the Snapshot type vs. the
//     Store.Snapshot method).
//   - undocumentedExports lists exported identifiers intentionally absent from the
//     README-derived doc-graph. Growing this list is a deliberate choice a
//     reviewer sees; it is not a silent escape hatch.
func TestDocManifestMatchesExportedSurface(t *testing.T) {
	// manifest id -> real exported identifier it stands for.
	docManifestAliases := map[string]string{
		"SnapshotType": "Snapshot", // documents the Snapshot type; "Snapshot" documents Store.Snapshot()
	}
	// Exported but intentionally outside the doc-graph.
	undocumentedExports := map[string]string{
		"SetTestNow":            "test-only deterministic-clock seam used by the docs generator",
		"OpenBytes":             "read-only in-memory open; not part of the README-documented surface",
		"UncompactedWALBytes":   "WAL introspection accessor; not part of the README-documented surface",
		"UncompactedWALEntries": "WAL introspection accessor; not part of the README-documented surface",
	}

	exported := exportedSurface(t)
	documented := documentedSymbols(t)

	// Map documented ids through the alias table to their real identifiers.
	docReal := map[string]bool{}
	for id := range documented {
		if real, ok := docManifestAliases[id]; ok {
			docReal[real] = true
		} else {
			docReal[id] = true
		}
	}

	// Every exported symbol must be documented or explicitly allow-listed.
	var undocumented []string
	for name := range exported {
		if docReal[name] {
			continue
		}
		if _, ok := undocumentedExports[name]; ok {
			continue
		}
		undocumented = append(undocumented, name)
	}
	if len(undocumented) > 0 {
		sort.Strings(undocumented)
		t.Errorf("exported symbols missing from docs/manifest.json: %v\n"+
			"Document them in the manifest (and regenerate the doc-graph), or add them to "+
			"undocumentedExports with a reason.", undocumented)
	}

	// Every documented symbol must map to a real exported identifier.
	var phantom []string
	for name := range docReal {
		if !exported[name] {
			phantom = append(phantom, name)
		}
	}
	if len(phantom) > 0 {
		sort.Strings(phantom)
		t.Errorf("docs/manifest.json documents symbols that are not exported by the akg package: %v\n"+
			"Remove them from the manifest, or fix the docManifestAliases mapping.", phantom)
	}

	// Guard the allow-list/alias maps against rot: an entry that no longer matches
	// a real exported symbol is itself drift.
	for name, reason := range undocumentedExports {
		if !exported[name] {
			t.Errorf("undocumentedExports lists %q (%s) but it is no longer exported — drop it", name, reason)
		}
	}
	for id, real := range docManifestAliases {
		if !documented[id] {
			t.Errorf("docManifestAliases maps manifest id %q but no such id is in the manifest — drop it", id)
		}
		if !exported[real] {
			t.Errorf("docManifestAliases maps %q -> %q but %q is not exported — fix it", id, real, real)
		}
	}
}

// exportedSurface parses the akg package source (excluding test files) and returns
// the set of exported identifiers: top-level funcs, types, vars, consts, and
// exported methods of exported types (by bare method name).
func exportedSurface(t *testing.T) map[string]bool {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}
	pkg, ok := pkgs["akg"]
	if !ok {
		t.Fatalf("akg package not found in current directory")
	}

	exportedTypes := map[string]bool{}
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			if gd, ok := decl.(*ast.GenDecl); ok && gd.Tok == token.TYPE {
				for _, spec := range gd.Specs {
					ts := spec.(*ast.TypeSpec)
					if ts.Name.IsExported() {
						exportedTypes[ts.Name.Name] = true
					}
				}
			}
		}
	}

	surface := map[string]bool{}
	for _, file := range pkg.Files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if !d.Name.IsExported() {
					continue
				}
				if d.Recv == nil {
					surface[d.Name.Name] = true // top-level func
					continue
				}
				if exportedTypes[receiverTypeName(d.Recv)] {
					surface[d.Name.Name] = true // method of an exported type
				}
			case *ast.GenDecl:
				switch d.Tok {
				case token.TYPE:
					for _, spec := range d.Specs {
						ts := spec.(*ast.TypeSpec)
						if ts.Name.IsExported() {
							surface[ts.Name.Name] = true
						}
					}
				case token.VAR, token.CONST:
					for _, spec := range d.Specs {
						vs := spec.(*ast.ValueSpec)
						for _, name := range vs.Names {
							if name.IsExported() {
								surface[name.Name] = true
							}
						}
					}
				}
			}
		}
	}
	return surface
}

// receiverTypeName returns the base type name of a method receiver, unwrapping a
// pointer receiver (e.g. (s *Store) -> "Store").
func receiverTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	switch t := recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// documentedSymbols returns the set of api_symbol ids declared in docs/manifest.json.
func documentedSymbols(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("docs", "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m struct {
		Nodes []struct {
			ID string `json:"id"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	const prefix = "api_symbol:"
	out := map[string]bool{}
	for _, n := range m.Nodes {
		if strings.HasPrefix(n.ID, prefix) {
			out[strings.TrimPrefix(n.ID, prefix)] = true
		}
	}
	return out
}
