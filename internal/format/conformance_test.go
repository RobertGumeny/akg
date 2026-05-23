package format

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RobertGumeny/akg/internal/wal"
)

const conformanceFixture = "m1-data-bloom-wal.akg"

func TestConformanceFixtureLevel3Validation(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "conformance", conformanceFixture)
	file, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("header", func(t *testing.T) {
		if HeaderSize != 64 {
			t.Fatalf("HeaderSize = %d, want 64", HeaderSize)
		}
		if len(file) < HeaderSize {
			t.Fatalf("fixture is smaller than header: %d bytes", len(file))
		}
		if !bytes.Equal(file[:4], []byte("AKG\x00")) {
			t.Fatalf("magic = %q, want AKG\\0", file[:4])
		}
		if file[4] != CurrentMajor || file[5] != CurrentMinor {
			t.Fatalf("version = %d.%d, want %d.%d", file[4], file[5], CurrentMajor, CurrentMinor)
		}
		if ChecksumAlgorithm(file[6]) != ChecksumCRC32 {
			t.Fatalf("checksum algorithm = %d, want CRC32", file[6])
		}
		if got := binary.LittleEndian.Uint32(file[7:11]); got != 3 {
			t.Fatalf("section count = %d, want 3", got)
		}
		assertHexBytes(t, "header bytes", file[:HeaderSize], "414b4700010001030000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000d24492430000000000")
	})

	container, sections, err := DecodeContainer(file)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("section table", func(t *testing.T) {
		wantSections := []Section{
			{Type: SectionData, Offset: 115, Length: 473},
			{Type: SectionBloom, Offset: 588, Length: 34},
			{Type: SectionWAL, Offset: 622, Length: 349},
		}
		if len(sections) != len(wantSections) {
			t.Fatalf("decoded %d sections, want %d", len(sections), len(wantSections))
		}
		for i := range wantSections {
			if sections[i] != wantSections[i] {
				t.Fatalf("section %d = %+v, want %+v", i, sections[i], wantSections[i])
			}
		}
		assertHexBytes(t, "section table bytes", file[HeaderSize:int(wantSections[0].Offset)], "017300000000000000d901000000000000024c020000000000002200000000000000036e020000000000005d01000000000000")
	})

	entries, err := DecodeDataEntries(container.Data)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("data", func(t *testing.T) {
		if len(entries) != 7 {
			t.Fatalf("decoded %d data entries, want 7", len(entries))
		}
		wantKeys := []string{
			"e:note:x1:links_to:note:x2",
			"ei:note:x2:links_to:note:x1",
			"n:note:x1",
			"n:note:x2",
			"ts:3000000:e:note:x1:links_to:note:x2",
			"ts:3000000:n:note:x1",
			"ts:3000000:n:note:x2",
		}
		for i, want := range wantKeys {
			if got := string(entries[i].Key); got != want {
				t.Fatalf("entry %d key = %q, want %q", i, got, want)
			}
			if i > 0 && bytes.Compare(entries[i-1].Key, entries[i].Key) >= 0 {
				t.Fatalf("data keys are not strictly sorted: %q then %q", entries[i-1].Key, entries[i].Key)
			}
		}
		assertHexBytes(t, "first Data entry prefix", container.Data[:19], "1a0000008b000000653a6e6f74653a78313a6c")
	})

	bf, err := DecodeBloom(container.Bloom)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("bloom", func(t *testing.T) {
		if bf.KeyCount != 7 || bf.BitCount != 70 || bf.HashFunctionCount != BloomHashFunctionCnt || bf.HashSeed != BloomHashSeed || len(bf.Bits) != 9 {
			t.Fatalf("unexpected Bloom params: key_count=%d bit_count=%d hash_count=%d seed=%d bits_len=%d", bf.KeyCount, bf.BitCount, bf.HashFunctionCount, bf.HashSeed, len(bf.Bits))
		}
		for _, e := range entries {
			if !bf.MayContain(e.Key) {
				t.Fatalf("fixture bloom misses %q", e.Key)
			}
		}
		assertHexBytes(t, "Bloom fixed-parameter prefix", container.Bloom[:21], "070000000000000046000000000000000700000000")
	})

	t.Run("wal", func(t *testing.T) {
		records, err := wal.DecodeRecordsStrict(container.WAL)
		if err != nil {
			t.Fatal(err)
		}
		if len(records) != 4 {
			t.Fatalf("decoded %d WAL records, want 4", len(records))
		}
		wantOps := []wal.Operation{wal.OpPutNode, wal.OpPutNode, wal.OpPutEdge, wal.OpCommit}
		for i, want := range wantOps {
			if records[i].Sequence != wal.SequenceNumber(i+1) || records[i].Operation != want {
				t.Fatalf("record %d = seq %d op %d, want seq %d op %d", i, records[i].Sequence, records[i].Operation, i+1, want)
			}
		}
		committed, err := wal.ReplayCommitted(container.WAL)
		if err != nil {
			t.Fatal(err)
		}
		if len(committed) != 3 || committed[0].Operation != wal.OpPutNode || committed[1].Operation != wal.OpPutNode || committed[2].Operation != wal.OpPutEdge {
			t.Fatalf("unexpected replay records: %#v", committed)
		}
		assertHexBytes(t, "first WAL record header", container.WAL[:13], "01000000000000000145000000")
	})

	t.Run("whole container round trip", func(t *testing.T) {
		reencoded, _, err := EncodeContainer(container)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(reencoded, file) {
			t.Fatal("fixture whole-container re-encode changed bytes")
		}
	})
}

func assertHexBytes(t *testing.T, name string, got []byte, wantHex string) {
	t.Helper()
	want, err := hex.DecodeString(strings.ReplaceAll(wantHex, " ", ""))
	if err != nil {
		t.Fatalf("invalid %s hex literal: %v", name, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s = %x, want %x", name, got, want)
	}
}
