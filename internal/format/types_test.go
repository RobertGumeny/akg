package format

import "testing"

func TestSectionTypeConstants(t *testing.T) {
	tests := []struct {
		name string
		got  SectionType
		want uint8
	}{
		{"Data", SectionData, 0x01},
		{"Bloom", SectionBloom, 0x02},
		{"WAL", SectionWAL, 0x03},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if uint8(tt.got) != tt.want {
				t.Fatalf("section type = %#02x, want %#02x", uint8(tt.got), tt.want)
			}
		})
	}
}

func TestChecksumAlgorithmConstants(t *testing.T) {
	tests := []struct {
		name string
		got  ChecksumAlgorithm
		want uint8
	}{
		{"CRC32", ChecksumCRC32, 0x01},
		{"SHA256", ChecksumSHA256, 0x02},
		{"BLAKE3", ChecksumBLAKE3, 0x03},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if uint8(tt.got) != tt.want {
				t.Fatalf("checksum algorithm = %#02x, want %#02x", uint8(tt.got), tt.want)
			}
		})
	}
}
