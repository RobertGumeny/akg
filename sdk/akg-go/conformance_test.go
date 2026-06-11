package akg

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type conformanceManifest struct {
	Fixtures []conformanceFixture `json:"fixtures"`
}

type conformanceFixture struct {
	Path                  string            `json:"path"`
	ExpectedResult        string            `json:"expected_result"`
	ExpectedErrorCategory string            `json:"expected_error_category"`
	ValidationScope       string            `json:"validation_scope"`
	StoreExpectation      *storeExpectation `json:"store_expectation"`
}

type storeExpectation struct {
	Nodes             int         `json:"nodes"`
	Edges             int         `json:"edges"`
	HasUncompactedWAL bool        `json:"has_uncompacted_wal"`
	NextWALSequence   uint64      `json:"next_wal_sequence"`
	AbsentNode        *absentNode `json:"absent_node"`
}

type absentNode struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

func TestConformance(t *testing.T) {
	fixtureDir := filepath.Join("..", "..", "testdata", "conformance")
	manifestData, err := os.ReadFile(filepath.Join(fixtureDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest conformanceManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	for _, fx := range manifest.Fixtures {
		fx := fx
		t.Run(fx.Path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(fixtureDir, fx.Path))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			var openErr error
			var st *Store
			if fx.ValidationScope == "format" {
				_, openErr = decodeContainer(data)
			} else {
				st, openErr = openBytes(data)
			}

			switch fx.ExpectedResult {
			case "accept":
				if openErr != nil {
					t.Fatalf("expected accept, got error: %v", openErr)
				}
				if fx.StoreExpectation != nil && st != nil {
					checkStoreExpectation(t, st, fx.StoreExpectation)
				}
			case "reject":
				if openErr == nil {
					t.Fatal("expected reject, got nil error")
				}
				got := errorCategory(openErr)
				if got != fx.ExpectedErrorCategory {
					t.Fatalf("expected error category %q, got %q (error: %v)", fx.ExpectedErrorCategory, got, openErr)
				}
			default:
				t.Fatalf("unknown expected_result: %q", fx.ExpectedResult)
			}
		})
	}
}

func checkStoreExpectation(t *testing.T, st *Store, exp *storeExpectation) {
	t.Helper()
	if got := len(st.state.nodes); got != exp.Nodes {
		t.Errorf("node count: want %d, got %d", exp.Nodes, got)
	}
	if got := len(st.state.edges); got != exp.Edges {
		t.Errorf("edge count: want %d, got %d", exp.Edges, got)
	}
	if got := len(st.committedWAL) > 0; got != exp.HasUncompactedWAL {
		t.Errorf("has_uncompacted_wal: want %v, got %v", exp.HasUncompactedWAL, got)
	}
	if got := uint64(st.nextWALSeq); got != exp.NextWALSequence {
		t.Errorf("next_wal_sequence: want %d, got %d", exp.NextWALSequence, got)
	}
	if exp.AbsentNode != nil {
		node, err := st.GetNode(exp.AbsentNode.Type, exp.AbsentNode.ID)
		if err != nil {
			t.Errorf("GetNode for absent check: %v", err)
		} else if node != nil {
			t.Errorf("expected node {type=%q id=%q} to be absent", exp.AbsentNode.Type, exp.AbsentNode.ID)
		}
	}
}

func errorCategory(err error) string {
	switch {
	case errors.Is(err, errInvalidHeader):
		return "invalid_header"
	case errors.Is(err, errChecksumMismatch):
		return "checksum_mismatch"
	case errors.Is(err, errInvalidSectionRanges):
		return "invalid_section_ranges"
	case errors.Is(err, errInvalidBloomSection):
		return "invalid_bloom_section"
	case errors.Is(err, errInvalidSectionTable):
		return "invalid_section_table"
	case errors.Is(err, errInvalidWALPayload):
		return "invalid_wal_payload"
	case errors.Is(err, errWALChecksumMismatch):
		return "wal_checksum_mismatch"
	case errors.Is(err, errUnknownWALOperation):
		return "unknown_wal_operation"
	case errors.Is(err, errInvalidWALRecord):
		return "invalid_wal_record"
	case errors.Is(err, errDerivedIndexMismatch):
		return "derived_index_mismatch"
	case errors.Is(err, errInvalidDataPayload):
		return "invalid_data_payload"
	case errors.Is(err, errMalformedKey):
		return "malformed_key"
	default:
		return "unknown"
	}
}
