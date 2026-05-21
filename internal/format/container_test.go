package format

import (
	"bytes"
	"encoding/binary"
	"errors"
	"reflect"
	"testing"
)

func TestHeaderEncodeDecodeLittleEndian(t *testing.T) {
	h := Header{Major: 1, Minor: 0, ChecksumAlgorithm: ChecksumCRC32, SectionCount: 0x01020304}
	buf, err := EncodeHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if len(buf) != HeaderSize {
		t.Fatalf("header len = %d", len(buf))
	}
	if string(buf[:4]) != "AKG\x00" {
		t.Fatalf("bad magic %q", buf[:4])
	}
	if got := buf[7:11]; !bytes.Equal(got, []byte{0x04, 0x03, 0x02, 0x01}) {
		t.Fatalf("section count bytes = % x", got)
	}
	got, err := DecodeHeader(buf)
	if err != nil {
		t.Fatal(err)
	}
	if got != h {
		t.Fatalf("decoded header = %+v, want %+v", got, h)
	}
}

func TestHeaderDecodeRejectsLevel4Corruption(t *testing.T) {
	buf, _ := EncodeHeader(Header{Major: 1, ChecksumAlgorithm: ChecksumCRC32})

	wrongMagic := append([]byte(nil), buf...)
	wrongMagic[0] = 'X'
	if _, err := DecodeHeader(wrongMagic); !errors.Is(err, ErrInvalidHeader) {
		t.Fatalf("wrong magic err = %v", err)
	}

	badChecksum := append([]byte(nil), buf...)
	badChecksum[4] = 2
	if _, err := DecodeHeader(badChecksum); !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("bad checksum err = %v", err)
	}

	unsupportedMajor := append([]byte(nil), buf...)
	unsupportedMajor[4] = CurrentMajor + 1
	binary.LittleEndian.PutUint32(unsupportedMajor[55:59], headerChecksum(unsupportedMajor))
	if _, err := DecodeHeader(unsupportedMajor); !errors.Is(err, ErrInvalidHeader) {
		t.Fatalf("unsupported major err = %v", err)
	}

	reserved := append([]byte(nil), buf...)
	reserved[11] = 1
	binary.LittleEndian.PutUint32(reserved[55:59], headerChecksum(reserved))
	if _, err := DecodeHeader(reserved); !errors.Is(err, ErrInvalidHeader) {
		t.Fatalf("reserved err = %v", err)
	}

	reservedChecksumAlgorithm := append([]byte(nil), buf...)
	reservedChecksumAlgorithm[6] = byte(ChecksumSHA256)
	binary.LittleEndian.PutUint32(reservedChecksumAlgorithm[55:59], headerChecksum(reservedChecksumAlgorithm))
	if _, err := DecodeHeader(reservedChecksumAlgorithm); !errors.Is(err, ErrInvalidHeader) {
		t.Fatalf("reserved checksum algorithm err = %v", err)
	}
}

func TestEncodeHeaderRejectsReservedChecksumAlgorithms(t *testing.T) {
	for _, alg := range []ChecksumAlgorithm{ChecksumSHA256, ChecksumBLAKE3} {
		if _, err := EncodeHeader(Header{Major: CurrentMajor, ChecksumAlgorithm: alg}); !errors.Is(err, ErrInvalidHeader) {
			t.Fatalf("EncodeHeader(%#02x) err = %v, want ErrInvalidHeader", uint8(alg), err)
		}
	}
}

func TestSectionTableEncodeDecodeLittleEndian(t *testing.T) {
	sections := []Section{{Type: SectionData, Offset: 0x0102030405060708, Length: 0x1112131415161718}}
	buf := EncodeSectionTable(sections)
	if len(buf) != SectionEntrySize {
		t.Fatalf("section table len = %d", len(buf))
	}
	want := []byte{0x01, 0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01, 0x18, 0x17, 0x16, 0x15, 0x14, 0x13, 0x12, 0x11}
	if !bytes.Equal(buf, want) {
		t.Fatalf("section table bytes = % x", buf)
	}
	got, err := DecodeSectionTable(buf, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != sections[0] {
		t.Fatalf("decoded section = %+v", got[0])
	}
}

func TestSectionValidationCardinalityAndRanges(t *testing.T) {
	valid := []Section{{Type: SectionData, Offset: 100, Length: 5}, {Type: SectionWAL, Offset: 200, Length: 0}, {Type: 0x99, Offset: 201, Length: 4}}
	if err := ValidateSections(valid, 300, ChecksumCRC32); err != nil {
		t.Fatalf("valid sections rejected: %v", err)
	}

	tests := []struct {
		name     string
		sections []Section
	}{
		{"missing data", []Section{{Type: SectionWAL, Offset: 0, Length: 0}}},
		{"duplicate data", []Section{{Type: SectionData, Offset: 0, Length: 5}, {Type: SectionData, Offset: 10, Length: 5}}},
		{"duplicate bloom", []Section{{Type: SectionData, Offset: 0, Length: 5}, {Type: SectionBloom, Offset: 10, Length: 5}, {Type: SectionBloom, Offset: 20, Length: 5}}},
		{"duplicate wal", []Section{{Type: SectionData, Offset: 0, Length: 5}, {Type: SectionWAL, Offset: 10, Length: 0}, {Type: SectionWAL, Offset: 20, Length: 0}}},
		{"zero bloom payload", []Section{{Type: SectionData, Offset: 0, Length: 5}, {Type: SectionBloom, Offset: 10, Length: 4}}},
		{"overlap", []Section{{Type: SectionData, Offset: 0, Length: 10}, {Type: 0x99, Offset: 9, Length: 1}}},
		{"out of file", []Section{{Type: SectionData, Offset: 95, Length: 6}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateSections(tt.sections, 100, ChecksumCRC32); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestDecodeContainerRejectsLevel4SectionTableCorruption(t *testing.T) {
	data, _ := EncodeDataEntries([]DataEntry{{Key: []byte("a"), Value: []byte("v")}})
	bloom, _ := EncodeBloom([][]byte{[]byte("a")})
	wal := []byte{}

	tests := []struct {
		name     string
		payloads []typedPayload
	}{
		{"missing Data section", []typedPayload{{typ: SectionBloom, payload: bloom}}},
		{"duplicate Data section", []typedPayload{{typ: SectionData, payload: data}, {typ: SectionData, payload: data}}},
		{"duplicate Bloom section", []typedPayload{{typ: SectionData, payload: data}, {typ: SectionBloom, payload: bloom}, {typ: SectionBloom, payload: bloom}}},
		{"duplicate WAL section", []typedPayload{{typ: SectionData, payload: data}, {typ: SectionWAL, zeroLength: true}, {typ: SectionWAL, zeroLength: true}}},
		{"zero-length Data section", []typedPayload{{typ: SectionData, zeroLength: true}}},
		{"zero-length Bloom section", []typedPayload{{typ: SectionData, payload: data}, {typ: SectionBloom, zeroLength: true}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := encodeTestContainer(t, tt.payloads...)
			if _, _, err := DecodeContainer(file); err == nil {
				t.Fatal("expected DecodeContainer error")
			}
		})
	}

	t.Run("overlapping sections", func(t *testing.T) {
		file := encodeTestContainer(t, typedPayload{typ: SectionData, payload: data}, typedPayload{typ: SectionBloom, payload: bloom})
		binary.LittleEndian.PutUint64(file[HeaderSize+SectionEntrySize+1:HeaderSize+SectionEntrySize+9], uint64(HeaderSize+2*SectionEntrySize))
		if _, _, err := DecodeContainer(file); !errors.Is(err, ErrInvalidSectionRanges) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("out-of-file section", func(t *testing.T) {
		file := encodeTestContainer(t, typedPayload{typ: SectionData, payload: data})
		binary.LittleEndian.PutUint64(file[HeaderSize+1:HeaderSize+9], uint64(len(file)+1))
		if _, _, err := DecodeContainer(file); !errors.Is(err, ErrInvalidSectionRanges) {
			t.Fatalf("err = %v", err)
		}
	})

	t.Run("zero-length WAL accepted", func(t *testing.T) {
		file := encodeTestContainer(t, typedPayload{typ: SectionData, payload: data}, typedPayload{typ: SectionWAL, payload: wal, zeroLength: true})
		c, _, err := DecodeContainer(file)
		if err != nil {
			t.Fatalf("zero-length WAL rejected: %v", err)
		}
		if c.WAL == nil || len(c.WAL) != 0 {
			t.Fatalf("decoded WAL = %#v, want empty slice", c.WAL)
		}
	})

	t.Run("unknown structurally valid section accepted", func(t *testing.T) {
		unknown := []byte("future-section")
		file := encodeTestContainer(t, typedPayload{typ: SectionData, payload: data}, typedPayload{typ: 0x99, payload: unknown})
		c, _, err := DecodeContainer(file)
		if err != nil {
			t.Fatalf("unknown section rejected: %v", err)
		}
		if !bytes.Equal(c.Data, data) {
			t.Fatalf("data payload changed")
		}
	})
}

func TestSectionChecksumRejectsReservedAlgorithms(t *testing.T) {
	payload := []byte("payload")
	for _, alg := range []ChecksumAlgorithm{ChecksumSHA256, ChecksumBLAKE3} {
		if _, err := EncodeSection(payload, alg); !errors.Is(err, ErrInvalidSectionTable) {
			t.Fatalf("EncodeSection(%#02x) err = %v, want ErrInvalidSectionTable", uint8(alg), err)
		}
		if _, err := DecodeSection(append(payload, make([]byte, 32)...), alg); !errors.Is(err, ErrInvalidSectionTable) {
			t.Fatalf("DecodeSection(%#02x) err = %v, want ErrInvalidSectionTable", uint8(alg), err)
		}
	}
}

func TestSectionChecksumRoundTrip(t *testing.T) {
	payload := []byte("payload")
	section, err := EncodeSection(payload, ChecksumCRC32)
	if err != nil {
		t.Fatal(err)
	}
	if len(section) != len(payload)+4 {
		t.Fatalf("section len = %d", len(section))
	}
	got, err := DecodeSection(section, ChecksumCRC32)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload = %q", got)
	}
	file := append([]byte("prefix"), section...)
	sections := []Section{{Type: SectionData, Offset: 6, Length: uint64(len(section))}, {Type: SectionWAL, Offset: uint64(6 + len(section)), Length: 0}}
	if err := ValidateSectionChecksums(file, sections, ChecksumCRC32); err != nil {
		t.Fatalf("section checksum validation rejected file: %v", err)
	}

	payloadBitFlipped := append([]byte(nil), section...)
	payloadBitFlipped[0] ^= 0xff
	if _, err := DecodeSection(payloadBitFlipped, ChecksumCRC32); !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("payload bit checksum err = %v", err)
	}

	checksumBitFlipped := append([]byte(nil), section...)
	checksumBitFlipped[len(checksumBitFlipped)-1] ^= 0x01
	if _, err := DecodeSection(checksumBitFlipped, ChecksumCRC32); !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("checksum bit err = %v", err)
	}
}

func TestDataEntriesEncodeDecodeSortedLittleEndian(t *testing.T) {
	payload, err := EncodeDataEntries([]DataEntry{
		{Key: []byte("b"), Value: []byte("two")},
		{Key: []byte("a"), Value: nil},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := []byte{1, 0, 0, 0, 0, 0, 0, 0, 'a', 1, 0, 0, 0, 3, 0, 0, 0, 'b'}
	if !bytes.Equal(payload[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("payload prefix = % x", payload[:len(wantPrefix)])
	}
	entries, err := DecodeDataEntries(payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(entries[0].Key) != "a" || len(entries[0].Value) != 0 || string(entries[1].Key) != "b" || string(entries[1].Value) != "two" {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestDataEntriesLevel5RoundTripProperties(t *testing.T) {
	cases := [][]DataEntry{
		{{Key: []byte("b"), Value: []byte("two")}, {Key: []byte("a"), Value: nil}, {Key: []byte("c"), Value: []byte{0, 1, 2}}},
		{{Key: []byte{0x00}, Value: []byte("zero")}, {Key: []byte{0xff}, Value: []byte("high")}, {Key: []byte{0x01, 0x00}, Value: []byte("middle")}},
		{{Key: []byte("prefix")}, {Key: []byte("prefix-longer"), Value: []byte("v")}},
	}
	for _, entries := range cases {
		encoded, err := EncodeDataEntries(entries)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodeDataEntries(encoded)
		if err != nil {
			t.Fatal(err)
		}
		reencoded, err := EncodeDataEntries(decoded)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(reencoded, encoded) {
			t.Fatalf("data round trip was not stable: % x != % x", reencoded, encoded)
		}
		for i := 1; i < len(decoded); i++ {
			if bytes.Compare(decoded[i-1].Key, decoded[i].Key) >= 0 {
				t.Fatalf("decoded keys are not bytewise sorted: %q then %q", decoded[i-1].Key, decoded[i].Key)
			}
		}
	}
	if _, err := EncodeDataEntries([]DataEntry{{Key: []byte("dup")}, {Key: []byte("other")}, {Key: []byte("dup")}}); !errors.Is(err, ErrDuplicateDataKey) {
		t.Fatalf("duplicate input keys err = %v", err)
	}
}

func TestContainerLevel5RoundTripProperties(t *testing.T) {
	data, err := EncodeDataEntries([]DataEntry{{Key: []byte("b"), Value: []byte("two")}, {Key: []byte("a"), Value: []byte("one")}})
	if err != nil {
		t.Fatal(err)
	}
	bloom, err := EncodeBloom([][]byte{[]byte("a"), []byte("b")})
	if err != nil {
		t.Fatal(err)
	}
	container := Container{Data: data, Bloom: bloom, WAL: []byte{}}
	encoded, sections, err := EncodeContainer(container)
	if err != nil {
		t.Fatal(err)
	}
	decoded, decodedSections, err := DecodeContainer(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decodedSections, sections) {
		t.Fatalf("sections = %+v, want %+v", decodedSections, sections)
	}
	reencoded, reencodedSections, err := EncodeContainer(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reencoded, encoded) {
		t.Fatal("container encode/decode/encode was not stable")
	}
	if !reflect.DeepEqual(reencodedSections, sections) {
		t.Fatalf("reencoded sections = %+v, want %+v", reencodedSections, sections)
	}

	unknown := []byte("future-section")
	withUnknown := encodeTestContainer(t, typedPayload{typ: SectionData, payload: data}, typedPayload{typ: 0x99, payload: unknown}, typedPayload{typ: SectionBloom, payload: bloom})
	decoded, decodedSections, err = DecodeContainer(withUnknown)
	if err != nil {
		t.Fatalf("unknown structurally valid section rejected: %v", err)
	}
	if !bytes.Equal(decoded.Data, data) || !bytes.Equal(decoded.Bloom, bloom) {
		t.Fatal("known sections were not decoded around unknown section")
	}
	if len(decodedSections) != 3 || decodedSections[1].Type != 0x99 {
		t.Fatalf("decoded sections = %+v, want unknown section preserved in table", decodedSections)
	}
}

func TestDataEntriesRejectMalformedDuplicateAndUnsorted(t *testing.T) {
	if _, err := EncodeDataEntries([]DataEntry{{Key: []byte("a")}, {Key: []byte("a")}}); !errors.Is(err, ErrDuplicateDataKey) {
		t.Fatalf("duplicate encode err = %v", err)
	}

	tests := []struct {
		name    string
		payload []byte
	}{
		{"truncated entry header", []byte{1, 0, 0, 0}},
		{"declared key length beyond payload", []byte{2, 0, 0, 0, 0, 0, 0, 0, 'k'}},
		{"declared value length beyond payload", []byte{1, 0, 0, 0, 2, 0, 0, 0, 'k'}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeDataEntries(tt.payload); !errors.Is(err, ErrInvalidDataSection) {
				t.Fatalf("err = %v", err)
			}
		})
	}

	dup, _ := EncodeDataEntries([]DataEntry{{Key: []byte("a")}, {Key: []byte("b")}})
	copy(dup[9+8:9+8+1], []byte("a"))
	if _, err := DecodeDataEntries(dup); !errors.Is(err, ErrDuplicateDataKey) {
		t.Fatalf("duplicate decode err = %v", err)
	}
	unsorted := append([]byte(nil), dup...)
	unsorted[8] = 'b'
	unsorted[17] = 'a'
	if _, err := DecodeDataEntries(unsorted); !errors.Is(err, ErrInvalidDataSection) {
		t.Fatalf("unsorted err = %v", err)
	}
}

type typedPayload struct {
	typ        SectionType
	payload    []byte
	zeroLength bool
}

func encodeTestContainer(t *testing.T, payloads ...typedPayload) []byte {
	t.Helper()
	sections := make([]Section, len(payloads))
	encoded := make([][]byte, len(payloads))
	for i, p := range payloads {
		sections[i].Type = p.typ
		if p.zeroLength {
			sections[i].Length = 0
			continue
		}
		sec, err := EncodeSection(p.payload, ChecksumCRC32)
		if err != nil {
			t.Fatal(err)
		}
		encoded[i] = sec
		sections[i].Length = uint64(len(sec))
	}
	pos := uint64(HeaderSize + len(sections)*SectionEntrySize)
	for i := range sections {
		sections[i].Offset = pos
		pos += sections[i].Length
	}
	header, err := EncodeHeader(Header{Major: CurrentMajor, Minor: CurrentMinor, ChecksumAlgorithm: ChecksumCRC32, SectionCount: uint32(len(sections))})
	if err != nil {
		t.Fatal(err)
	}
	file := append(header, EncodeSectionTable(sections)...)
	for _, sec := range encoded {
		file = append(file, sec...)
	}
	return file
}
