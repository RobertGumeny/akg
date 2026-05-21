package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/RobertGumeny/akg-format/internal/format"
	"github.com/RobertGumeny/akg-format/internal/record"
)

func TestConformanceManifestSync(t *testing.T) {
	manifest := loadConformanceManifest(t)
	seen := map[string]bool{}
	for _, fixture := range manifest.Fixtures {
		if fixture.Path == "" {
			t.Fatal("manifest fixture has empty path")
		}
		if fixture.Purpose == "" {
			t.Fatalf("manifest fixture %q has empty purpose", fixture.Path)
		}
		if fixture.ExpectedResult != "accept" && fixture.ExpectedResult != "reject" {
			t.Fatalf("manifest fixture %q result = %q, want accept or reject", fixture.Path, fixture.ExpectedResult)
		}
		if fixture.ExpectedResult == "reject" && fixture.ExpectedErrorCategory == "" {
			t.Fatalf("manifest rejection fixture %q has empty expected_error_category", fixture.Path)
		}
		if seen[fixture.Path] {
			t.Fatalf("manifest fixture %q appears more than once", fixture.Path)
		}
		seen[fixture.Path] = true
		if _, err := os.Stat(conformancePath(fixture.Path)); err != nil {
			t.Fatalf("manifest references missing fixture %q: %v", fixture.Path, err)
		}
	}

	matches, err := filepath.Glob(conformancePath("*.akg"))
	if err != nil {
		t.Fatal(err)
	}
	for _, match := range matches {
		name := filepath.Base(match)
		if !seen[name] {
			t.Fatalf("fixture %q is missing from manifest", name)
		}
	}
	if len(seen) != len(matches) {
		t.Fatalf("manifest has %d fixtures, filesystem has %d .akg fixtures", len(seen), len(matches))
	}
}

func TestMilestone2ConformanceFixturesOpen(t *testing.T) {
	for _, tc := range loadConformanceManifest(t).Fixtures {
		if tc.ValidationScope != "store" || tc.ExpectedResult != "accept" {
			continue
		}
		t.Run(tc.Path, func(t *testing.T) {
			if tc.StoreExpectation == nil {
				t.Fatalf("store accept fixture %q is missing store_expectation", tc.Path)
			}
			st, err := Open(conformancePath(tc.Path))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if got := len(st.State().Nodes()); got != tc.StoreExpectation.Nodes {
				t.Fatalf("nodes = %d, want %d", got, tc.StoreExpectation.Nodes)
			}
			if got := len(st.State().Edges()); got != tc.StoreExpectation.Edges {
				t.Fatalf("edges = %d, want %d", got, tc.StoreExpectation.Edges)
			}
			if int(st.NextWALSequence()) != tc.StoreExpectation.NextWALSequence {
				t.Fatalf("next WAL sequence = %d, want %d", st.NextWALSequence(), tc.StoreExpectation.NextWALSequence)
			}
			if tc.StoreExpectation.HasUncompactedWAL == (st.UncompactedWALEntries() == 0) {
				t.Fatalf("WAL entry presence = %t, want %t", st.UncompactedWALEntries() != 0, tc.StoreExpectation.HasUncompactedWAL)
			}
			if tc.StoreExpectation.AbsentNode != nil {
				absent := tc.StoreExpectation.AbsentNode
				if _, ok := st.State().GetNode(absent.Type, record.NodeID(absent.ID)); ok {
					t.Fatalf("deleted node n:%s:%s survived open", absent.Type, absent.ID)
				}
			}
		})
	}
}

func TestMilestone2ConformanceRejectionFixtures(t *testing.T) {
	for _, tc := range loadConformanceManifest(t).Fixtures {
		if tc.ValidationScope != "store" || tc.ExpectedResult != "reject" {
			continue
		}
		t.Run(tc.Path, func(t *testing.T) {
			_, err := Open(conformancePath(tc.Path))
			if err == nil {
				t.Fatal("Open succeeded, want rejection")
			}
			if want := conformanceErrorCategory(tc.ExpectedErrorCategory); want != nil && !errors.Is(err, want) {
				t.Fatalf("Open error = %v, want category %q (%v)", err, tc.ExpectedErrorCategory, want)
			}
		})
	}
}

func TestStoreOpenToleratesUnknownStructurallyValidSection(t *testing.T) {
	path := tempPath(t)
	entries := []format.DataEntry{{Key: []byte("n:note:n1"), Value: nodePayloadWithUnknownField()}, {Key: []byte("ts:7:n:note:n1")}}
	data := mustData(t, entries)
	bloom := mustBloom(t, [][]byte{[]byte("n:note:n1"), []byte("ts:7:n:note:n1")})
	file := encodeStoreTestContainer(t,
		testSection{typ: format.SectionData, payload: data},
		testSection{typ: 0x99, payload: []byte("future-section")},
		testSection{typ: format.SectionBloom, payload: bloom},
		testSection{typ: format.SectionWAL, zeroLength: true},
	)
	if err := os.WriteFile(path, file, 0o666); err != nil {
		t.Fatal(err)
	}
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open rejected unknown structurally valid section: %v", err)
	}
	if _, ok := st.State().GetNode("note", "n1"); !ok {
		t.Fatal("known Data section was not hydrated around unknown section")
	}
}

func conformancePath(name string) string {
	return filepath.Join("..", "..", "testdata", "conformance", name)
}

func loadConformanceManifest(t *testing.T) conformanceManifest {
	t.Helper()
	data, err := os.ReadFile(conformancePath("manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest conformanceManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != 1 {
		t.Fatalf("manifest version = %d, want 1", manifest.Version)
	}
	return manifest
}

func conformanceErrorCategory(category string) error {
	switch category {
	case "derived_index_mismatch":
		return ErrDerivedIndexMismatch
	default:
		return nil
	}
}

type conformanceManifest struct {
	Version  int                  `json:"version"`
	Fixtures []conformanceFixture `json:"fixtures"`
}

type conformanceFixture struct {
	Path                  string                  `json:"path"`
	Purpose               string                  `json:"purpose"`
	ExpectedResult        string                  `json:"expected_result"`
	ExpectedErrorCategory string                  `json:"expected_error_category"`
	ValidationScope       string                  `json:"validation_scope"`
	StoreExpectation      *conformanceStoreExpect `json:"store_expectation"`
}

type conformanceStoreExpect struct {
	Nodes             int                    `json:"nodes"`
	Edges             int                    `json:"edges"`
	HasUncompactedWAL bool                   `json:"has_uncompacted_wal"`
	NextWALSequence   int                    `json:"next_wal_sequence"`
	AbsentNode        *conformanceAbsentNode `json:"absent_node"`
}

type conformanceAbsentNode struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type testSection struct {
	typ        format.SectionType
	payload    []byte
	zeroLength bool
}

func encodeStoreTestContainer(t *testing.T, payloads ...testSection) []byte {
	t.Helper()
	sections := make([]format.Section, len(payloads))
	encoded := make([][]byte, len(payloads))
	for i, p := range payloads {
		sections[i].Type = p.typ
		if p.zeroLength {
			continue
		}
		sec, err := format.EncodeSection(p.payload, format.ChecksumCRC32)
		if err != nil {
			t.Fatal(err)
		}
		encoded[i] = sec
		sections[i].Length = uint64(len(sec))
	}
	pos := uint64(format.HeaderSize + len(sections)*format.SectionEntrySize)
	for i := range sections {
		sections[i].Offset = pos
		pos += sections[i].Length
	}
	header, err := format.EncodeHeader(format.Header{Major: format.CurrentMajor, Minor: format.CurrentMinor, ChecksumAlgorithm: format.ChecksumCRC32, SectionCount: uint32(len(sections))})
	if err != nil {
		t.Fatal(err)
	}
	file := append(header, format.EncodeSectionTable(sections)...)
	for _, sec := range encoded {
		file = append(file, sec...)
	}
	return file
}
