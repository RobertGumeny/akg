package format

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"sort"
)

const (
	HeaderSize        = 64
	SectionEntrySize  = 17
	// CurrentMajor is 2: the tag-index key is type-qualified (t:{tag}:{type}:{id}).
	// Readers still accept major 1 (the legacy t:{tag}:{id} shape) for read-compat
	// — the `h.Major > CurrentMajor` gate below deliberately keeps major 1 readable
	// — but writers always emit major 2, so files self-upgrade on compaction.
	CurrentMajor      = 2
	CurrentMinor      = 0
	headerChecksumOff = 55
)

var magic = [4]byte{'A', 'K', 'G', 0}

var (
	ErrInvalidHeader        = errors.New("invalid header")
	ErrChecksumMismatch     = errors.New("checksum mismatch")
	ErrInvalidSectionTable  = errors.New("invalid section table")
	ErrInvalidSectionRanges = errors.New("invalid section ranges")
	ErrInvalidDataSection   = errors.New("invalid data section")
	ErrDuplicateDataKey     = errors.New("duplicate data key")
)

// Header is the fixed 64-byte AKG container header.
type Header struct {
	Major             uint8
	Minor             uint8
	ChecksumAlgorithm ChecksumAlgorithm
	SectionCount      uint32
}

// DataEntry is one flat key/value entry in a Data section payload.
type DataEntry struct {
	Key   []byte
	Value []byte
}

// EncodeHeader encodes h as the canonical 64-byte little-endian AKG header and
// writes the header CRC32 checksum over the header with the checksum field zeroed.
func EncodeHeader(h Header) ([]byte, error) {
	if h.Major == 0 {
		h.Major = CurrentMajor
	}
	if h.ChecksumAlgorithm == 0 {
		h.ChecksumAlgorithm = ChecksumCRC32
	}
	if h.ChecksumAlgorithm != ChecksumCRC32 {
		return nil, ErrInvalidHeader
	}
	buf := make([]byte, HeaderSize)
	copy(buf[0:4], magic[:])
	buf[4] = h.Major
	buf[5] = h.Minor
	buf[6] = byte(h.ChecksumAlgorithm)
	binary.LittleEndian.PutUint32(buf[7:11], h.SectionCount)
	binary.LittleEndian.PutUint32(buf[headerChecksumOff:headerChecksumOff+4], headerChecksum(buf))
	return buf, nil
}

// DecodeHeader validates and decodes the fixed 64-byte AKG header.
func DecodeHeader(buf []byte) (Header, error) {
	if len(buf) < HeaderSize || !bytes.Equal(buf[0:4], magic[:]) {
		return Header{}, ErrInvalidHeader
	}
	if !reservedHeaderBytesZero(buf) {
		return Header{}, ErrInvalidHeader
	}
	want := binary.LittleEndian.Uint32(buf[headerChecksumOff : headerChecksumOff+4])
	if got := headerChecksum(buf); got != want {
		return Header{}, ErrChecksumMismatch
	}
	h := Header{
		Major:             buf[4],
		Minor:             buf[5],
		ChecksumAlgorithm: ChecksumAlgorithm(buf[6]),
		SectionCount:      binary.LittleEndian.Uint32(buf[7:11]),
	}
	if h.Major > CurrentMajor || checksumSize(h.ChecksumAlgorithm) == 0 {
		return Header{}, ErrInvalidHeader
	}
	return h, nil
}

func reservedHeaderBytesZero(buf []byte) bool {
	for _, b := range buf[11:55] {
		if b != 0 {
			return false
		}
	}
	for _, b := range buf[59:64] {
		if b != 0 {
			return false
		}
	}
	return true
}

func headerChecksum(buf []byte) uint32 {
	copyBuf := make([]byte, HeaderSize)
	copy(copyBuf, buf[:HeaderSize])
	for i := headerChecksumOff; i < headerChecksumOff+4; i++ {
		copyBuf[i] = 0
	}
	return crc32.ChecksumIEEE(copyBuf)
}

// EncodeSectionTable encodes section descriptors as repeated 17-byte entries.
func EncodeSectionTable(sections []Section) []byte {
	buf := make([]byte, len(sections)*SectionEntrySize)
	for i, s := range sections {
		off := i * SectionEntrySize
		buf[off] = byte(s.Type)
		binary.LittleEndian.PutUint64(buf[off+1:off+9], s.Offset)
		binary.LittleEndian.PutUint64(buf[off+9:off+17], s.Length)
	}
	return buf
}

// DecodeSectionTable decodes exactly count section descriptors.
func DecodeSectionTable(buf []byte, count uint32) ([]Section, error) {
	need := int(count) * SectionEntrySize
	if len(buf) != need {
		return nil, ErrInvalidSectionTable
	}
	sections := make([]Section, count)
	for i := range sections {
		off := i * SectionEntrySize
		sections[i] = Section{
			Type:   SectionType(buf[off]),
			Offset: binary.LittleEndian.Uint64(buf[off+1 : off+9]),
			Length: binary.LittleEndian.Uint64(buf[off+9 : off+17]),
		}
	}
	return sections, nil
}

// ValidateSections enforces section cardinality and range invariants. fileSize is
// the complete container byte length.
func ValidateSections(sections []Section, fileSize uint64, alg ChecksumAlgorithm) error {
	cs := uint64(checksumSize(alg))
	if cs == 0 {
		return ErrInvalidSectionTable
	}
	var data, bloom, wal int
	for _, s := range sections {
		switch s.Type {
		case SectionData:
			data++
			if s.Length < cs {
				return ErrInvalidSectionTable
			}
		case SectionBloom:
			bloom++
			if s.Length <= cs {
				return ErrInvalidSectionTable
			}
		case SectionWAL:
			wal++
			if s.Length != 0 && s.Length < cs {
				return ErrInvalidSectionTable
			}
		}
		if s.Offset > fileSize || s.Length > fileSize-s.Offset {
			return ErrInvalidSectionRanges
		}
	}
	if data != 1 || bloom > 1 || wal > 1 {
		return ErrInvalidSectionTable
	}
	sorted := append([]Section(nil), sections...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Offset < sorted[j].Offset })
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1].Offset+sorted[i-1].Length > sorted[i].Offset {
			return ErrInvalidSectionRanges
		}
	}
	return nil
}

// Checksum returns the AKG v1 section checksum for payload. AKG v1 requires
// CRC32; SHA-256 and BLAKE3 algorithm IDs are reserved for future versions.
func Checksum(payload []byte, alg ChecksumAlgorithm) ([]byte, error) {
	if alg != ChecksumCRC32 {
		return nil, ErrInvalidSectionTable
	}
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, crc32.ChecksumIEEE(payload))
	return out, nil
}

func checksumSize(alg ChecksumAlgorithm) int {
	switch alg {
	case ChecksumCRC32:
		return 4
	default:
		return 0
	}
}

// EncodeSection returns payload followed by its checksum bytes.
func EncodeSection(payload []byte, alg ChecksumAlgorithm) ([]byte, error) {
	sum, err := Checksum(payload, alg)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(payload)+len(sum))
	out = append(out, payload...)
	out = append(out, sum...)
	return out, nil
}

// ValidateSectionChecksums validates checksums for every non-empty section
// range described by the section table. A zero-length WAL section has no payload
// or trailing checksum bytes and is skipped.
func ValidateSectionChecksums(file []byte, sections []Section, alg ChecksumAlgorithm) error {
	for _, s := range sections {
		if s.Type == SectionWAL && s.Length == 0 {
			continue
		}
		if s.Offset > uint64(len(file)) || s.Length > uint64(len(file))-s.Offset {
			return ErrInvalidSectionRanges
		}
		start, end := int(s.Offset), int(s.Offset+s.Length)
		if _, err := DecodeSection(file[start:end], alg); err != nil {
			return err
		}
	}
	return nil
}

// DecodeSection validates section checksum bytes and returns the payload.
func DecodeSection(section []byte, alg ChecksumAlgorithm) ([]byte, error) {
	cs := checksumSize(alg)
	if cs == 0 || len(section) < cs {
		return nil, ErrInvalidSectionTable
	}
	payload, got := section[:len(section)-cs], section[len(section)-cs:]
	want, err := Checksum(payload, alg)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(got, want) {
		return nil, ErrChecksumMismatch
	}
	return payload, nil
}

// EncodeDataEntries encodes entries sorted by raw bytewise key order. Duplicate
// keys are rejected before encoding.
func EncodeDataEntries(entries []DataEntry) ([]byte, error) {
	entries = append([]DataEntry(nil), entries...)
	sort.Slice(entries, func(i, j int) bool { return bytes.Compare(entries[i].Key, entries[j].Key) < 0 })
	var total uint64
	for i, e := range entries {
		if i > 0 && bytes.Equal(entries[i-1].Key, e.Key) {
			return nil, ErrDuplicateDataKey
		}
		total += 8 + uint64(len(e.Key)) + uint64(len(e.Value))
	}
	buf := make([]byte, 0, total)
	var lens [8]byte
	for _, e := range entries {
		binary.LittleEndian.PutUint32(lens[0:4], uint32(len(e.Key)))
		binary.LittleEndian.PutUint32(lens[4:8], uint32(len(e.Value)))
		buf = append(buf, lens[:]...)
		buf = append(buf, e.Key...)
		buf = append(buf, e.Value...)
	}
	return buf, nil
}

// DecodeDataEntries decodes the flat Data-section payload and rejects malformed,
// truncated, unsorted, or duplicate-key entry streams.
func DecodeDataEntries(payload []byte) ([]DataEntry, error) {
	entries := []DataEntry{}
	for len(payload) > 0 {
		if len(payload) < 8 {
			return nil, ErrInvalidDataSection
		}
		keyLen := binary.LittleEndian.Uint32(payload[0:4])
		valueLen := binary.LittleEndian.Uint32(payload[4:8])
		payload = payload[8:]
		need := uint64(keyLen) + uint64(valueLen)
		if need > uint64(len(payload)) {
			return nil, ErrInvalidDataSection
		}
		key := append([]byte(nil), payload[:keyLen]...)
		value := append([]byte(nil), payload[keyLen:uint64(keyLen)+uint64(valueLen)]...)
		payload = payload[need:]
		if len(entries) > 0 {
			cmp := bytes.Compare(entries[len(entries)-1].Key, key)
			if cmp == 0 {
				return nil, ErrDuplicateDataKey
			}
			if cmp > 0 {
				return nil, ErrInvalidDataSection
			}
		}
		entries = append(entries, DataEntry{Key: key, Value: value})
	}
	return entries, nil
}
