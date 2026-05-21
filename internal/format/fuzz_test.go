package format

import "testing"

func FuzzDecodeHeader(f *testing.F) {
	valid, _ := EncodeHeader(Header{Major: CurrentMajor, Minor: CurrentMinor, ChecksumAlgorithm: ChecksumCRC32, SectionCount: 1})
	f.Add([]byte{})
	f.Add([]byte("AKG\x00"))
	f.Add(valid)

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeHeader(data)
	})
}

func FuzzDecodeSectionTable(f *testing.F) {
	f.Add([]byte{}, uint8(0))
	f.Add(EncodeSectionTable([]Section{{Type: SectionData, Offset: HeaderSize + SectionEntrySize, Length: 12}}), uint8(1))
	f.Add([]byte{byte(SectionData)}, uint8(1))

	f.Fuzz(func(t *testing.T, data []byte, count uint8) {
		_, _ = DecodeSectionTable(data, uint32(count))
	})
}

func FuzzDecodeSection(f *testing.F) {
	valid, _ := EncodeSection([]byte("payload"), ChecksumCRC32)
	f.Add([]byte{})
	f.Add([]byte{1, 0, 0, 0})
	f.Add(valid)

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeSection(data, ChecksumCRC32)
	})
}

func FuzzDecodeDataEntries(f *testing.F) {
	valid, _ := EncodeDataEntries([]DataEntry{{Key: []byte("a"), Value: []byte("one")}, {Key: []byte("b"), Value: nil}})
	f.Add([]byte{})
	f.Add([]byte{1, 0, 0, 0})
	f.Add([]byte{1, 0, 0, 0, 0, 0, 0, 0, 'a'})
	f.Add(valid)

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeDataEntries(data)
	})
}

func FuzzDecodeBloom(f *testing.F) {
	valid, _ := EncodeBloom([][]byte{[]byte("a"), []byte("b")})
	f.Add([]byte{})
	f.Add([]byte{1, 0, 0, 0})
	f.Add(valid)

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeBloom(data)
	})
}

func FuzzDecodeContainer(f *testing.F) {
	data, _ := EncodeDataEntries([]DataEntry{{Key: []byte("a"), Value: []byte("one")}})
	bloom, _ := EncodeBloom([][]byte{[]byte("a")})
	valid, _, _ := EncodeContainer(Container{Data: data, Bloom: bloom, WAL: []byte{}})
	f.Add([]byte{})
	f.Add([]byte("AKG\x00"))
	f.Add(valid)

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _ = DecodeContainer(data)
	})
}
