package format

// SectionType identifies an AKG container section.
type SectionType uint8

const (
	SectionData  SectionType = 0x01
	SectionBloom SectionType = 0x02
	SectionWAL   SectionType = 0x03
)

// Section describes one AKG section-table entry.
type Section struct {
	Type   SectionType
	Offset uint64
	Length uint64
}

// ChecksumAlgorithm identifies the file checksum algorithm declared in the
// container header.
type ChecksumAlgorithm uint8

const (
	ChecksumCRC32  ChecksumAlgorithm = 0x01
	ChecksumSHA256 ChecksumAlgorithm = 0x02
	ChecksumBLAKE3 ChecksumAlgorithm = 0x03
)
