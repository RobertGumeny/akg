package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/RobertGumeny/akg-format/internal/format"
)

func TestMilestone2ConformanceFixturesOpen(t *testing.T) {
	cases := []struct {
		name      string
		nodes     int
		edges     int
		wantWAL   bool
		nextSeq   int
		absentKey string
	}{
		{name: "m2-empty-create.akg", nodes: 0, edges: 0, nextSeq: 1},
		{name: "m2-minimal-node.akg", nodes: 1, edges: 0, nextSeq: 1},
		{name: "m2-full-node.akg", nodes: 1, edges: 0, nextSeq: 1},
		{name: "m2-single-edge.akg", nodes: 2, edges: 1, nextSeq: 1},
		{name: "m2-small-graph.akg", nodes: 3, edges: 2, nextSeq: 1},
		{name: "m2-committed-wal-replay.akg", nodes: 2, edges: 1, wantWAL: true, nextSeq: 10},
		{name: "m2-uncommitted-wal-tail.akg", nodes: 2, edges: 1, wantWAL: true, nextSeq: 11},
		{name: "m2-compacted.akg", nodes: 1, edges: 0, nextSeq: 1, absentKey: "n:note:old"},
		{name: "m2-deletes-before-compaction.akg", nodes: 1, edges: 0, wantWAL: true, nextSeq: 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, err := Open(conformancePath(tc.name))
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if got := len(st.State().Nodes()); got != tc.nodes {
				t.Fatalf("nodes = %d, want %d", got, tc.nodes)
			}
			if got := len(st.State().Edges()); got != tc.edges {
				t.Fatalf("edges = %d, want %d", got, tc.edges)
			}
			if int(st.NextWALSequence()) != tc.nextSeq {
				t.Fatalf("next WAL sequence = %d, want %d", st.NextWALSequence(), tc.nextSeq)
			}
			if tc.wantWAL == (st.UncompactedWALEntries() == 0) {
				t.Fatalf("WAL entry presence = %t, want %t", st.UncompactedWALEntries() != 0, tc.wantWAL)
			}
			if tc.absentKey != "" {
				if _, ok := st.State().GetNode("note", "old"); ok {
					t.Fatalf("deleted node from %s survived open", tc.absentKey)
				}
			}
		})
	}
}

func TestMilestone2ConformanceRejectionFixtures(t *testing.T) {
	cases := []struct {
		name string
		want error
	}{
		{name: "m2-reject-malformed-committed-wal.akg"},
		{name: "m2-reject-derived-index-mismatch.akg", want: ErrDerivedIndexMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Open(conformancePath(tc.name))
			if err == nil {
				t.Fatal("Open succeeded, want rejection")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("Open error = %v, want %v", err, tc.want)
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
