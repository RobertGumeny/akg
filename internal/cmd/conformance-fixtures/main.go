package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"

	"github.com/RobertGumeny/akg-format/internal/format"
	"github.com/RobertGumeny/akg-format/internal/keys"
	"github.com/RobertGumeny/akg-format/internal/record"
	"github.com/RobertGumeny/akg-format/internal/store"
	"github.com/RobertGumeny/akg-format/internal/wal"
)

func main() {
	dir := flag.String("dir", "testdata/conformance", "conformance fixture directory")
	printHashes := flag.Bool("print-hashes", false, "print current fixture sha256 values")
	writeTask3 := flag.Bool("write-task3-rejections", false, "write deterministic Milestone 3 rejection fixtures")
	writeUnknownSectionAccept := flag.Bool("write-unknown-section-accept", false, "write deterministic accepted fixture with a structurally valid unknown section")
	flag.Parse()

	if *writeTask3 {
		if err := writeTask3Rejections(*dir); err != nil {
			fatal(err)
		}
	}
	if *writeUnknownSectionAccept {
		if err := os.WriteFile(filepath.Join(*dir, "m3-unknown-section-tolerated.akg"), unknownSectionAcceptedContainer(), 0o666); err != nil {
			fatal(err)
		}
	}

	manifest, err := loadManifest(filepath.Join(*dir, "manifest.json"))
	if err != nil {
		fatal(err)
	}
	for _, fixture := range manifest.Fixtures {
		path := filepath.Join(*dir, fixture.Path)
		data, err := os.ReadFile(path)
		if err != nil {
			fatal(fmt.Errorf("read %s: %w", fixture.Path, err))
		}
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])
		if *printHashes {
			fmt.Printf("%s  %s\n", hash, fixture.Path)
		}
		if fixture.SHA256 == "" {
			fatal(fmt.Errorf("%s: manifest sha256 is empty", fixture.Path))
		}
		if hash != fixture.SHA256 {
			fatal(fmt.Errorf("%s: sha256 %s, want %s", fixture.Path, hash, fixture.SHA256))
		}
		if fixture.ValidationScope == "store" {
			err := store.Validate(path)
			if fixture.ExpectedResult == "accept" && err != nil {
				fatal(fmt.Errorf("%s: validate rejected accepted fixture: %w", fixture.Path, err))
			}
			if fixture.ExpectedResult == "reject" && err == nil {
				fatal(fmt.Errorf("%s: validate accepted rejection fixture", fixture.Path))
			}
		}
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func loadManifest(path string) (manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, err
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, err
	}
	if m.Version != 1 {
		return manifest{}, fmt.Errorf("manifest version = %d, want 1", m.Version)
	}
	return m, nil
}

type manifest struct {
	Version  int       `json:"version"`
	Fixtures []fixture `json:"fixtures"`
}

type fixture struct {
	Path            string `json:"path"`
	ExpectedResult  string `json:"expected_result"`
	ValidationScope string `json:"validation_scope"`
	SHA256          string `json:"sha256"`
}

func writeTask3Rejections(dir string) error {
	empty, err := emptyContainer()
	if err != nil {
		return err
	}
	fixtures := map[string][]byte{}

	fixtures["m3-reject-wrong-magic.akg"] = corrupt(copyBytes(empty), 0, 'X')
	fixtures["m3-reject-unsupported-major-version.akg"] = mustHeaderContainer(2, []format.Section{{Type: format.SectionData, Offset: format.HeaderSize + format.SectionEntrySize, Length: 4}}, [][]byte{mustSection(nil)})
	fixtures["m3-reject-bad-header-checksum.akg"] = corrupt(copyBytes(empty), 55, 0xff)
	fixtures["m3-reject-bad-section-checksum.akg"] = corrupt(copyBytes(empty), len(empty)-1, 0xff)
	fixtures["m3-reject-duplicate-data-sections.akg"] = containerFromSections([]testSection{{typ: format.SectionData, payload: nil}, {typ: format.SectionData, payload: nil}})
	fixtures["m3-reject-overlapping-sections.akg"] = overlappingContainer()
	fixtures["m3-reject-malformed-bloom.akg"] = containerFromSections([]testSection{{typ: format.SectionData, payload: nil}, {typ: format.SectionBloom, payload: []byte{0x01}}, {typ: format.SectionWAL, zeroLength: true}})
	fixtures["m3-reject-invalid-wal-opcode.akg"] = containerWithWAL(unknownOpcodeWAL())
	fixtures["m3-reject-invalid-wal-put-node-payload.akg"] = containerWithWAL(committedWAL(wal.OpPutNode, []byte{0xc1}))
	fixtures["m3-reject-invalid-wal-put-node-utf8-payload.akg"] = containerWithWAL(committedWAL(wal.OpPutNode, invalidUTF8NodePutPayload()))
	fixtures["m3-reject-invalid-wal-delete-node-payload.akg"] = containerWithWAL(committedWAL(wal.OpDeleteNode, []byte{0xc1}))
	fixtures["m3-reject-invalid-wal-put-edge-payload.akg"] = containerWithWAL(committedWAL(wal.OpPutEdge, []byte{0xc1}))
	fixtures["m3-reject-invalid-wal-delete-edge-payload.akg"] = containerWithWAL(committedWAL(wal.OpDeleteEdge, []byte{0xc1}))
	fixtures["m3-reject-malformed-committed-wal-checksum.akg"] = containerWithWAL(corrupt(committedWAL(wal.OpPutNode, validNodePutPayload()), 3, 0xff))
	fixtures["m3-reject-duplicate-wal-sequence.akg"] = containerWithWAL(nonIncreasingSequenceWAL(1, 1))
	fixtures["m3-reject-decreasing-wal-sequence.akg"] = containerWithWAL(nonIncreasingSequenceWAL(2, 1))
	fixtures["m3-reject-invalid-node-data-payload.akg"] = containerFromDataEntries([]format.DataEntry{{Key: []byte("n:note:n1"), Value: []byte{0xc1}}})
	fixtures["m3-reject-invalid-node-data-utf8-payload.akg"] = containerFromDataEntries([]format.DataEntry{{Key: []byte("n:note:n1"), Value: invalidUTF8NodePayload()}})
	fixtures["m3-reject-missing-derived-tag-index.akg"] = missingDerivedTagContainer()

	names := make([]string, 0, len(fixtures))
	for name := range fixtures {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), fixtures[name], 0o666); err != nil {
			return err
		}
	}
	return nil
}

type testSection struct {
	typ        format.SectionType
	payload    []byte
	zeroLength bool
}

func emptyContainer() ([]byte, error) {
	data, err := format.EncodeDataEntries(nil)
	if err != nil {
		return nil, err
	}
	bloom, err := format.EncodeBloom(nil)
	if err != nil {
		return nil, err
	}
	file, _, err := format.EncodeContainer(format.Container{Data: data, Bloom: bloom, WAL: []byte{}})
	return file, err
}

func containerFromSections(payloads []testSection) []byte {
	sections := make([]format.Section, len(payloads))
	encoded := make([][]byte, len(payloads))
	for i, p := range payloads {
		sections[i].Type = p.typ
		if p.zeroLength {
			continue
		}
		sec := mustSection(p.payload)
		encoded[i] = sec
		sections[i].Length = uint64(len(sec))
	}
	pos := uint64(format.HeaderSize + len(sections)*format.SectionEntrySize)
	for i := range sections {
		sections[i].Offset = pos
		pos += sections[i].Length
	}
	return mustHeaderContainer(format.CurrentMajor, sections, encoded)
}

func mustHeaderContainer(major uint8, sections []format.Section, encoded [][]byte) []byte {
	header, err := format.EncodeHeader(format.Header{Major: major, Minor: format.CurrentMinor, ChecksumAlgorithm: format.ChecksumCRC32, SectionCount: uint32(len(sections))})
	if err != nil {
		panic(err)
	}
	out := append(header, format.EncodeSectionTable(sections)...)
	for _, p := range encoded {
		out = append(out, p...)
	}
	return out
}

func overlappingContainer() []byte {
	data := mustSection(nil)
	bloomPayload, err := format.EncodeBloom(nil)
	if err != nil {
		panic(err)
	}
	bloom := mustSection(bloomPayload)
	off := uint64(format.HeaderSize + 2*format.SectionEntrySize)
	sections := []format.Section{{Type: format.SectionData, Offset: off, Length: uint64(len(data))}, {Type: format.SectionBloom, Offset: off + 1, Length: uint64(len(bloom))}}
	return mustHeaderContainer(format.CurrentMajor, sections, [][]byte{data, bloom})
}

func containerWithWAL(walPayload []byte) []byte {
	bloom, err := format.EncodeBloom(nil)
	if err != nil {
		panic(err)
	}
	return containerFromSections([]testSection{{typ: format.SectionData, payload: nil}, {typ: format.SectionBloom, payload: bloom}, {typ: format.SectionWAL, payload: walPayload}})
}

func containerFromDataEntries(entries []format.DataEntry) []byte {
	data, err := format.EncodeDataEntries(entries)
	if err != nil {
		panic(err)
	}
	keysForBloom := make([][]byte, len(entries))
	for i, entry := range entries {
		keysForBloom[i] = entry.Key
	}
	bloom, err := format.EncodeBloom(keysForBloom)
	if err != nil {
		panic(err)
	}
	return containerFromSections([]testSection{{typ: format.SectionData, payload: data}, {typ: format.SectionBloom, payload: bloom}, {typ: format.SectionWAL, zeroLength: true}})
}

func missingDerivedTagContainer() []byte {
	node := record.Node{Type: "note", Title: "Tagged", Tags: []string{"red"}, CreatedAt: 1, UpdatedAt: 1}
	payload, err := record.EncodeNodePayload(node)
	if err != nil {
		panic(err)
	}
	nodeKey, err := keys.BuildNodeKey("note", "n1")
	if err != nil {
		panic(err)
	}
	timeKey, err := keys.BuildTemporalNodeKey(1, "note", "n1")
	if err != nil {
		panic(err)
	}
	return containerFromDataEntries([]format.DataEntry{{Key: nodeKey, Value: payload}, {Key: timeKey}})
}

func unknownSectionAcceptedContainer() []byte {
	node := record.Node{Type: "note", Title: "Unknown section tolerated", CreatedAt: 1, UpdatedAt: 1}
	payload, err := record.EncodeNodePayload(node)
	if err != nil {
		panic(err)
	}
	nodeKey, err := keys.BuildNodeKey("note", "n1")
	if err != nil {
		panic(err)
	}
	timeKey, err := keys.BuildTemporalNodeKey(1, "note", "n1")
	if err != nil {
		panic(err)
	}
	data, err := format.EncodeDataEntries([]format.DataEntry{{Key: nodeKey, Value: payload}, {Key: timeKey}})
	if err != nil {
		panic(err)
	}
	bloom, err := format.EncodeBloom([][]byte{nodeKey, timeKey})
	if err != nil {
		panic(err)
	}
	return containerFromSections([]testSection{
		{typ: format.SectionData, payload: data},
		{typ: 0x99, payload: []byte("future-section")},
		{typ: format.SectionBloom, payload: bloom},
		{typ: format.SectionWAL, zeroLength: true},
	})
}

func validNodePutPayload() []byte {
	payload, err := record.EncodeNodePutPayload(record.NodePut{ID: "n1", Node: record.Node{Type: "note", Title: "One", CreatedAt: 1, UpdatedAt: 1}})
	if err != nil {
		panic(err)
	}
	return payload
}

func invalidUTF8NodePayload() []byte {
	return []byte{0x83,
		0xa4, 't', 'y', 'p', 'e', 0xa4, 'n', 'o', 't', 'e',
		0xa5, 't', 'i', 't', 'l', 'e', 0xa2, 'o', 'k',
		0xa4, 'm', 'e', 't', 'a', 0x81, 0xa1, 'k', 0x91, 0xa1, 0xff,
	}
}

func invalidUTF8NodePutPayload() []byte {
	return []byte{0x84,
		0xa4, 't', 'y', 'p', 'e', 0xa4, 'n', 'o', 't', 'e',
		0xa5, 't', 'i', 't', 'l', 'e', 0xa2, 'o', 'k',
		0xa4, 'm', 'e', 't', 'a', 0x81, 0xa1, 'k', 0x91, 0xa1, 0xff,
		0xa2, 'i', 'd', 0xa2, 'n', '1',
	}
}

func committedWAL(op wal.Operation, payload []byte) []byte {
	records := []wal.Record{{Sequence: 1, Operation: op, Payload: payload}, {Sequence: 2, Operation: wal.OpCommit}}
	out, err := wal.EncodeRecords(records)
	if err != nil {
		panic(err)
	}
	return out
}

func unknownOpcodeWAL() []byte {
	buf := make([]byte, 17)
	binary.LittleEndian.PutUint64(buf[0:8], 1)
	buf[8] = 0xff
	binary.LittleEndian.PutUint32(buf[9:13], 0)
	binary.LittleEndian.PutUint32(buf[13:17], crc32.ChecksumIEEE(buf[:13]))
	return buf
}

func nonIncreasingSequenceWAL(first, second wal.SequenceNumber) []byte {
	records := []wal.Record{{Sequence: first, Operation: wal.OpPutNode, Payload: validNodePutPayload()}, {Sequence: second, Operation: wal.OpCommit}}
	out, err := wal.EncodeRecords(records)
	if err != nil {
		panic(err)
	}
	return out
}

func mustSection(payload []byte) []byte {
	sec, err := format.EncodeSection(payload, format.ChecksumCRC32)
	if err != nil {
		panic(err)
	}
	return sec
}

func copyBytes(b []byte) []byte {
	return append([]byte(nil), b...)
}

func corrupt(b []byte, idx int, mask byte) []byte {
	b[idx] ^= mask
	return b
}
