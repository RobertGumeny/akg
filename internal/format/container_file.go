package format

import "errors"

var ErrMissingSection = errors.New("missing section")

// Container contains decoded known AKG section payloads plus the binary major
// from the file header, which read-side validation needs to select the
// version-correct tag-key shape (major 1 vs. major 2).
type Container struct {
	Major uint8
	Data  []byte
	Bloom []byte
	WAL   []byte
}

// EncodeContainer writes a Milestone-1 whole container with Data, optional Bloom, and optional WAL sections.
func EncodeContainer(c Container) ([]byte, []Section, error) {
	sections := make([]Section, 0, 3)
	payloads := make([][]byte, 0, 3)
	add := func(t SectionType, p []byte) error {
		sec, err := EncodeSection(p, ChecksumCRC32)
		if err != nil {
			return err
		}
		sections = append(sections, Section{Type: t, Length: uint64(len(sec))})
		payloads = append(payloads, sec)
		return nil
	}
	if err := add(SectionData, c.Data); err != nil {
		return nil, nil, err
	}
	if c.Bloom != nil {
		if err := add(SectionBloom, c.Bloom); err != nil {
			return nil, nil, err
		}
	}
	if c.WAL != nil {
		if len(c.WAL) == 0 {
			sections = append(sections, Section{Type: SectionWAL, Length: 0})
			payloads = append(payloads, nil)
		} else if err := add(SectionWAL, c.WAL); err != nil {
			return nil, nil, err
		}
	}
	off := uint64(HeaderSize + len(sections)*SectionEntrySize)
	for i := range sections {
		sections[i].Offset = off
		off += sections[i].Length
	}
	header, err := EncodeHeader(Header{Major: CurrentMajor, Minor: CurrentMinor, ChecksumAlgorithm: ChecksumCRC32, SectionCount: uint32(len(sections))})
	if err != nil {
		return nil, nil, err
	}
	out := append(header, EncodeSectionTable(sections)...)
	for _, p := range payloads {
		out = append(out, p...)
	}
	return out, sections, nil
}

// DecodeContainer validates a whole AKG v1 container and returns known section payloads.
func DecodeContainer(file []byte) (Container, []Section, error) {
	h, err := DecodeHeader(file)
	if err != nil {
		return Container{}, nil, err
	}
	tableEnd := HeaderSize + int(h.SectionCount)*SectionEntrySize
	if len(file) < tableEnd {
		return Container{}, nil, ErrInvalidSectionTable
	}
	sections, err := DecodeSectionTable(file[HeaderSize:tableEnd], h.SectionCount)
	if err != nil {
		return Container{}, nil, err
	}
	if err := ValidateSections(sections, uint64(len(file)), h.ChecksumAlgorithm); err != nil {
		return Container{}, nil, err
	}
	c := Container{Major: h.Major}
	for _, s := range sections {
		if s.Type == SectionWAL && s.Length == 0 {
			c.WAL = []byte{}
			continue
		}
		start, end := int(s.Offset), int(s.Offset+s.Length)
		payload, err := DecodeSection(file[start:end], h.ChecksumAlgorithm)
		if err != nil {
			return Container{}, nil, err
		}
		switch s.Type {
		case SectionData:
			c.Data = payload
		case SectionBloom:
			c.Bloom = payload
		case SectionWAL:
			c.WAL = payload
		}
	}
	return c, sections, nil
}
