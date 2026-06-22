package akg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"math/bits"
	"sort"
)

const (
	headerSize       = 64
	sectionEntrySize = 17
	// currentMajor is 2: the tag-index key is type-qualified (t:{tag}:{type}:{id}).
	// Readers still accept major 1 (legacy t:{tag}:{id}) for read-compat — the
	// `buf[4] > currentMajor` gate in decodeHeader keeps major 1 readable — but
	// writers always emit major 2, so files self-upgrade on compaction.
	currentMajor      = 2
	currentMinor      = 0
	headerChecksumOff = 55
	checksumCRC32     = 0x01
	sectionData       = 0x01
	sectionBloom      = 0x02
	sectionWAL        = 0x03
	bloomBitsPerKey   = 10
	bloomHashCount    = 7
	bloomHashSeed     = 0
)

var (
	magic                   = [4]byte{'A', 'K', 'G', 0}
	errInvalidHeader        = errors.New("invalid header")
	errChecksumMismatch     = errors.New("checksum mismatch")
	errInvalidSectionTable  = errors.New("invalid section table")
	errInvalidSectionRanges = errors.New("invalid section ranges")
	errInvalidDataSection   = errors.New("invalid data section")
	errDuplicateDataKey     = errors.New("duplicate data key")
	errInvalidBloomSection  = errors.New("invalid bloom section")
)

type header struct {
	Major        uint8
	Minor        uint8
	SectionCount uint32
}

type section struct {
	Type   uint8
	Offset uint64
	Length uint64
}

type dataEntry struct {
	Key   []byte
	Value []byte
}

type container struct {
	// Major is the binary major from the file header. Read-side validation needs
	// it to select the version-correct tag-key shape (major 1 vs. major 2).
	Major uint8
	Data  []byte
	Bloom []byte
	WAL   []byte
}

func encodeHeader(h header) ([]byte, error) {
	buf := make([]byte, headerSize)
	copy(buf[0:4], magic[:])
	buf[4] = currentMajor
	buf[5] = h.Minor
	buf[6] = checksumCRC32
	binary.LittleEndian.PutUint32(buf[7:11], h.SectionCount)
	binary.LittleEndian.PutUint32(buf[headerChecksumOff:headerChecksumOff+4], headerChecksum(buf))
	return buf, nil
}

func decodeHeader(buf []byte) (header, error) {
	if len(buf) < headerSize || !bytes.Equal(buf[0:4], magic[:]) {
		return header{}, errInvalidHeader
	}
	for _, b := range buf[11:55] {
		if b != 0 {
			return header{}, errInvalidHeader
		}
	}
	for _, b := range buf[59:64] {
		if b != 0 {
			return header{}, errInvalidHeader
		}
	}
	want := binary.LittleEndian.Uint32(buf[headerChecksumOff : headerChecksumOff+4])
	if got := headerChecksum(buf); got != want {
		return header{}, errChecksumMismatch
	}
	if buf[4] > currentMajor || buf[6] != checksumCRC32 {
		return header{}, errInvalidHeader
	}
	return header{Major: buf[4], Minor: buf[5], SectionCount: binary.LittleEndian.Uint32(buf[7:11])}, nil
}

func headerChecksum(buf []byte) uint32 {
	copyBuf := make([]byte, headerSize)
	copy(copyBuf, buf[:headerSize])
	for i := headerChecksumOff; i < headerChecksumOff+4; i++ {
		copyBuf[i] = 0
	}
	return crc32.ChecksumIEEE(copyBuf)
}

func encodeSectionTable(sections []section) []byte {
	buf := make([]byte, len(sections)*sectionEntrySize)
	for i, s := range sections {
		off := i * sectionEntrySize
		buf[off] = s.Type
		binary.LittleEndian.PutUint64(buf[off+1:off+9], s.Offset)
		binary.LittleEndian.PutUint64(buf[off+9:off+17], s.Length)
	}
	return buf
}

func decodeSectionTable(buf []byte, count uint32) ([]section, error) {
	need := int(count) * sectionEntrySize
	if len(buf) != need {
		return nil, errInvalidSectionTable
	}
	sections := make([]section, count)
	for i := range sections {
		off := i * sectionEntrySize
		sections[i] = section{Type: buf[off], Offset: binary.LittleEndian.Uint64(buf[off+1 : off+9]), Length: binary.LittleEndian.Uint64(buf[off+9 : off+17])}
	}
	return sections, nil
}

func validateSections(sections []section, fileSize uint64) error {
	var data, bloom, wal int
	for _, s := range sections {
		switch s.Type {
		case sectionData:
			data++
			if s.Length < 4 {
				return errInvalidSectionTable
			}
		case sectionBloom:
			bloom++
			if s.Length <= 4 {
				return errInvalidSectionTable
			}
		case sectionWAL:
			wal++
			if s.Length != 0 && s.Length < 4 {
				return errInvalidSectionTable
			}
		}
		if s.Offset > fileSize || s.Length > fileSize-s.Offset {
			return errInvalidSectionRanges
		}
	}
	if data != 1 || bloom > 1 || wal > 1 {
		return errInvalidSectionTable
	}
	sorted := append([]section(nil), sections...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Offset < sorted[j].Offset })
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1].Offset+sorted[i-1].Length > sorted[i].Offset {
			return errInvalidSectionRanges
		}
	}
	return nil
}

func encodeSection(payload []byte) []byte {
	sum := make([]byte, 4)
	binary.LittleEndian.PutUint32(sum, crc32.ChecksumIEEE(payload))
	out := make([]byte, 0, len(payload)+4)
	out = append(out, payload...)
	out = append(out, sum...)
	return out
}

func decodeSection(section []byte) ([]byte, error) {
	if len(section) < 4 {
		return nil, errInvalidSectionTable
	}
	payload, got := section[:len(section)-4], section[len(section)-4:]
	want := make([]byte, 4)
	binary.LittleEndian.PutUint32(want, crc32.ChecksumIEEE(payload))
	if !bytes.Equal(got, want) {
		return nil, errChecksumMismatch
	}
	return payload, nil
}

func encodeDataEntries(entries []dataEntry) ([]byte, error) {
	entries = append([]dataEntry(nil), entries...)
	sort.Slice(entries, func(i, j int) bool { return bytes.Compare(entries[i].Key, entries[j].Key) < 0 })
	var total uint64
	for i, e := range entries {
		if i > 0 && bytes.Equal(entries[i-1].Key, e.Key) {
			return nil, errDuplicateDataKey
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

func decodeDataEntries(payload []byte) ([]dataEntry, error) {
	entries := []dataEntry{}
	for len(payload) > 0 {
		if len(payload) < 8 {
			return nil, errInvalidDataSection
		}
		keyLen := binary.LittleEndian.Uint32(payload[0:4])
		valueLen := binary.LittleEndian.Uint32(payload[4:8])
		payload = payload[8:]
		need := uint64(keyLen) + uint64(valueLen)
		if need > uint64(len(payload)) {
			return nil, errInvalidDataSection
		}
		key := append([]byte(nil), payload[:keyLen]...)
		value := append([]byte(nil), payload[keyLen:uint64(keyLen)+uint64(valueLen)]...)
		payload = payload[need:]
		if len(entries) > 0 {
			cmp := bytes.Compare(entries[len(entries)-1].Key, key)
			if cmp == 0 {
				return nil, errDuplicateDataKey
			}
			if cmp > 0 {
				return nil, errInvalidDataSection
			}
		}
		entries = append(entries, dataEntry{Key: key, Value: value})
	}
	return entries, nil
}

func encodeBloom(keys [][]byte) []byte {
	keyCount := uint64(len(keys))
	bitCount := keyCount * bloomBitsPerKey
	bitsLen := int((bitCount + 7) / 8)
	bitsArr := make([]byte, bitsLen)
	for _, key := range keys {
		if bitCount == 0 {
			break
		}
		h1, h2 := murmur3x64_128(key, bloomHashSeed)
		for i := uint8(0); i < bloomHashCount; i++ {
			idx := (h1 + uint64(i)*h2) % bitCount
			bitsArr[idx/8] |= 1 << (idx % 8)
		}
	}
	buf := make([]byte, 8+8+1+4+len(bitsArr))
	binary.LittleEndian.PutUint64(buf[0:8], keyCount)
	binary.LittleEndian.PutUint64(buf[8:16], bitCount)
	buf[16] = bloomHashCount
	binary.LittleEndian.PutUint32(buf[17:21], bloomHashSeed)
	copy(buf[21:], bitsArr)
	return buf
}

func decodeBloom(payload []byte) error {
	if len(payload) < 21 {
		return errInvalidBloomSection
	}
	keyCount := binary.LittleEndian.Uint64(payload[0:8])
	bitCount := binary.LittleEndian.Uint64(payload[8:16])
	hashCount := payload[16]
	hashSeed := binary.LittleEndian.Uint32(payload[17:21])
	bitsArr := payload[21:]
	if hashCount != bloomHashCount || hashSeed != bloomHashSeed || bitCount != keyCount*bloomBitsPerKey {
		return errInvalidBloomSection
	}
	if uint64(len(bitsArr)) != (bitCount+7)/8 {
		return errInvalidBloomSection
	}
	if extra := bitCount % 8; extra != 0 && len(bitsArr) > 0 {
		mask := byte(0xff << extra)
		if bitsArr[len(bitsArr)-1]&mask != 0 {
			return errInvalidBloomSection
		}
	}
	return nil
}

func murmur3x64_128(data []byte, seed uint32) (uint64, uint64) {
	const c1 uint64 = 0x87c37b91114253d5
	const c2 uint64 = 0x4cf5ad432745937f
	h1, h2 := uint64(seed), uint64(seed)
	nblocks := len(data) / 16
	for i := 0; i < nblocks; i++ {
		block := data[i*16:]
		k1 := binary.LittleEndian.Uint64(block[0:8])
		k2 := binary.LittleEndian.Uint64(block[8:16])
		k1 *= c1
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= c2
		h1 ^= k1
		h1 = bits.RotateLeft64(h1, 27)
		h1 += h2
		h1 = h1*5 + 0x52dce729
		k2 *= c2
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= c1
		h2 ^= k2
		h2 = bits.RotateLeft64(h2, 31)
		h2 += h1
		h2 = h2*5 + 0x38495ab5
	}
	var k1, k2 uint64
	tail := data[nblocks*16:]
	for i := 0; i < len(tail) && i < 8; i++ {
		k1 |= uint64(tail[i]) << (8 * i)
	}
	for i := 8; i < len(tail); i++ {
		k2 |= uint64(tail[i]) << (8 * (i - 8))
	}
	if k2 != 0 {
		k2 *= c2
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= c1
		h2 ^= k2
	}
	if k1 != 0 {
		k1 *= c1
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= c2
		h1 ^= k1
	}
	h1 ^= uint64(len(data))
	h2 ^= uint64(len(data))
	h1 += h2
	h2 += h1
	h1 = fmix64(h1)
	h2 = fmix64(h2)
	h1 += h2
	h2 += h1
	return h1, h2
}

func fmix64(k uint64) uint64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	k ^= k >> 33
	return k
}

func encodeContainer(c container) ([]byte, error) {
	sections := make([]section, 0, 3)
	payloads := make([][]byte, 0, 3)
	add := func(t uint8, p []byte) {
		sec := encodeSection(p)
		sections = append(sections, section{Type: t, Length: uint64(len(sec))})
		payloads = append(payloads, sec)
	}
	add(sectionData, c.Data)
	if c.Bloom != nil {
		add(sectionBloom, c.Bloom)
	}
	if c.WAL != nil {
		if len(c.WAL) == 0 {
			sections = append(sections, section{Type: sectionWAL, Length: 0})
			payloads = append(payloads, nil)
		} else {
			add(sectionWAL, c.WAL)
		}
	}
	off := uint64(headerSize + len(sections)*sectionEntrySize)
	for i := range sections {
		sections[i].Offset = off
		off += sections[i].Length
	}
	header, err := encodeHeader(header{Major: currentMajor, Minor: currentMinor, SectionCount: uint32(len(sections))})
	if err != nil {
		return nil, err
	}
	out := append(header, encodeSectionTable(sections)...)
	for _, p := range payloads {
		out = append(out, p...)
	}
	return out, nil
}

func decodeContainer(file []byte) (container, error) {
	h, err := decodeHeader(file)
	if err != nil {
		return container{}, err
	}
	tableEnd := headerSize + int(h.SectionCount)*sectionEntrySize
	if len(file) < tableEnd {
		return container{}, errInvalidSectionTable
	}
	sections, err := decodeSectionTable(file[headerSize:tableEnd], h.SectionCount)
	if err != nil {
		return container{}, err
	}
	if err := validateSections(sections, uint64(len(file))); err != nil {
		return container{}, err
	}
	c := container{Major: h.Major}
	for _, s := range sections {
		if s.Type == sectionWAL && s.Length == 0 {
			c.WAL = []byte{}
			continue
		}
		start, end := int(s.Offset), int(s.Offset+s.Length)
		payload, err := decodeSection(file[start:end])
		if err != nil {
			return container{}, err
		}
		switch s.Type {
		case sectionData:
			c.Data = payload
		case sectionBloom:
			c.Bloom = payload
		case sectionWAL:
			c.WAL = payload
		}
	}
	return c, nil
}
