package wal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"testing"

	"github.com/earendil-works/akg/internal/record"
)

func TestWALRoundTripAllOperationsAndPayloadValidation(t *testing.T) {
	nodePayload, err := record.EncodeNodePayload(record.Node{Type: "note", Title: "hello", CreatedAt: 1, UpdatedAt: 2})
	if err != nil {
		t.Fatal(err)
	}
	edgePayload, err := record.EncodeEdgePayload(record.Edge{FromNode: "a", Relation: "links", ToNode: "b", CreatedAt: 3, UpdatedAt: 4})
	if err != nil {
		t.Fatal(err)
	}
	deleteNodePayload, err := record.EncodeNodeDeletePayload(record.NodeDelete{Type: "note", ID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	deleteEdgePayload, err := record.EncodeEdgeDeletePayload(record.EdgeDelete{FromNode: "a", Relation: "links", ToNode: "b"})
	if err != nil {
		t.Fatal(err)
	}
	records := []Record{
		{Sequence: 1, Operation: OpPutNode, Payload: nodePayload},
		{Sequence: 2, Operation: OpPutEdge, Payload: edgePayload},
		{Sequence: 3, Operation: OpDeleteNode, Payload: deleteNodePayload},
		{Sequence: 4, Operation: OpDeleteEdge, Payload: deleteEdgePayload},
		{Sequence: 5, Operation: OpCommit},
	}
	payload, err := EncodeRecords(records)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeRecordsStrict(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(records) {
		t.Fatalf("decoded %d records, want %d", len(got), len(records))
	}
	for i := range got {
		if got[i].Sequence != records[i].Sequence || got[i].Operation != records[i].Operation || !bytes.Equal(got[i].Payload, records[i].Payload) {
			t.Fatalf("record %d = %#v, want %#v", i, got[i], records[i])
		}
		if err := ValidatePayload(got[i]); err != nil {
			t.Fatalf("payload %d rejected: %v", i, err)
		}

		encodedOne, err := EncodeRecord(records[i])
		if err != nil {
			t.Fatal(err)
		}
		decodedOne, n, err := DecodeRecord(encodedOne)
		if err != nil {
			t.Fatal(err)
		}
		if n != len(encodedOne) {
			t.Fatalf("DecodeRecord consumed %d bytes, want %d", n, len(encodedOne))
		}
		if decodedOne.Sequence != records[i].Sequence || decodedOne.Operation != records[i].Operation || !bytes.Equal(decodedOne.Payload, records[i].Payload) {
			t.Fatalf("single record round trip = %#v, want %#v", decodedOne, records[i])
		}
		wantCRC := crc32.ChecksumIEEE(encodedOne[:len(encodedOne)-4])
		if gotCRC := binary.LittleEndian.Uint32(encodedOne[len(encodedOne)-4:]); gotCRC != wantCRC {
			t.Fatalf("CRC32 bytes are not little-endian: got %08x want %08x", gotCRC, wantCRC)
		}
	}
}

func TestWALDeletePayloadsTolerateUnknownReadFields(t *testing.T) {
	deleteNodeWithExtra := []byte{0x83, 0xa2, 'i', 'd', 0xa1, 'n', 0xa5, 'e', 'x', 't', 'r', 'a', 0xc3, 0xa4, 't', 'y', 'p', 'e', 0xa4, 'n', 'o', 't', 'e'}
	deleteEdgeWithExtra := []byte{0x84, 0xa9, 'f', 'r', 'o', 'm', '_', 'n', 'o', 'd', 'e', 0xa1, 'a', 0xa8, 'r', 'e', 'l', 'a', 't', 'i', 'o', 'n', 0xa5, 'l', 'i', 'n', 'k', 's', 0xa7, 't', 'o', '_', 'n', 'o', 'd', 'e', 0xa1, 'b', 0xa7, 'i', 'g', 'n', 'o', 'r', 'e', 'd', 0xcc, 0x2a}
	for _, r := range []Record{{Operation: OpDeleteNode, Payload: deleteNodeWithExtra}, {Operation: OpDeleteEdge, Payload: deleteEdgeWithExtra}} {
		if err := ValidatePayload(r); err != nil {
			t.Fatalf("delete with extra field rejected: %v", err)
		}
	}
	missingNodeID := []byte{0x81, 0xa4, 't', 'y', 'p', 'e', 0xa4, 'n', 'o', 't', 'e'}
	if err := ValidatePayload(Record{Operation: OpDeleteNode, Payload: missingNodeID}); !errors.Is(err, record.ErrMissingRequiredField) {
		t.Fatalf("missing delete id err = %v", err)
	}
	missingEdgeToNode := []byte{0x82, 0xa9, 'f', 'r', 'o', 'm', '_', 'n', 'o', 'd', 'e', 0xa1, 'a', 0xa8, 'r', 'e', 'l', 'a', 't', 'i', 'o', 'n', 0xa5, 'l', 'i', 'n', 'k', 's'}
	if err := ValidatePayload(Record{Operation: OpDeleteEdge, Payload: missingEdgeToNode}); !errors.Is(err, record.ErrMissingRequiredField) {
		t.Fatalf("missing delete edge field err = %v", err)
	}
}

func TestWALRejectsLevel4RecordCorruption(t *testing.T) {
	nodePayload, _ := record.EncodeNodePayload(record.Node{Type: "note", Title: "ok", CreatedAt: 1, UpdatedAt: 1})
	valid, _ := EncodeRecord(Record{Sequence: 1, Operation: OpPutNode, Payload: nodePayload})

	tests := []struct {
		name string
		buf  []byte
		want error
	}{
		{"truncated record header", valid[:recordHeaderSize-1], ErrInvalidRecord},
		{"declared payload length beyond available bytes", corruptWAL(valid, func(b []byte) { binary.LittleEndian.PutUint32(b[9:13], uint32(len(nodePayload)+1)) }), ErrInvalidRecord},
		{"bad WAL checksum", corruptWAL(valid, func(b []byte) { b[len(b)-1] ^= 0x01 }), ErrChecksumMismatch},
		{"unknown operation code", corruptWAL(valid, func(b []byte) { b[8] = 0xff }), ErrUnknownOperation},
		{"COMMIT with non-empty payload", encodeRawWALRecord(2, OpCommit, []byte("not-empty")), ErrInvalidRecord},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := DecodeRecord(tt.buf); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestWALReplayCommittedTailBehavior(t *testing.T) {
	nodePayload, _ := record.EncodeNodePayload(record.Node{Type: "note", Title: "committed", CreatedAt: 1, UpdatedAt: 1})
	uncommittedPayload, _ := record.EncodeNodePayload(record.Node{Type: "note", Title: "uncommitted", CreatedAt: 2, UpdatedAt: 2})
	committed, _ := EncodeRecords([]Record{{Sequence: 1, Operation: OpPutNode, Payload: nodePayload}, {Sequence: 2, Operation: OpCommit}})
	uncommitted, _ := EncodeRecord(Record{Sequence: 3, Operation: OpPutNode, Payload: uncommittedPayload})
	payload := append(committed, uncommitted...)

	replayed, err := ReplayCommitted(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) != 1 || replayed[0].Sequence != 1 {
		t.Fatalf("replayed = %#v", replayed)
	}

	multiCommit, _ := EncodeRecords([]Record{
		{Sequence: 1, Operation: OpPutNode, Payload: nodePayload},
		{Sequence: 2, Operation: OpCommit},
		{Sequence: 3, Operation: OpPutNode, Payload: uncommittedPayload},
		{Sequence: 4, Operation: OpCommit},
		{Sequence: 5, Operation: OpPutNode, Payload: nodePayload},
	})
	replayed, err = ReplayCommitted(multiCommit)
	if err != nil {
		t.Fatal(err)
	}
	if len(replayed) != 2 || replayed[0].Sequence != 1 || replayed[1].Sequence != 3 {
		t.Fatalf("replay should return records before last valid COMMIT only, excluding COMMITs: %#v", replayed)
	}
	payload = append(committed, []byte{0x01, 0x02, 0x03}...)
	if _, err := ReplayCommitted(payload); err != nil {
		t.Fatalf("trailing malformed uncommitted tail was not ignored: %v", err)
	}

	badCommitted, _ := EncodeRecords([]Record{{Sequence: 1, Operation: OpDeleteNode, Payload: missingNodeIDPayload()}, {Sequence: 2, Operation: OpCommit}})
	if _, err := ReplayCommitted(badCommitted); !errors.Is(err, record.ErrMissingRequiredField) {
		t.Fatalf("malformed committed WAL err = %v", err)
	}

	badEdgeDeleteCommitted, _ := EncodeRecords([]Record{{Sequence: 1, Operation: OpDeleteEdge, Payload: missingEdgeDeleteFieldPayload()}, {Sequence: 2, Operation: OpCommit}})
	if _, err := ReplayCommitted(badEdgeDeleteCommitted); !errors.Is(err, record.ErrMissingRequiredField) {
		t.Fatalf("malformed committed edge delete WAL err = %v", err)
	}

	malformedBeforeCommit := append([]byte(nil), committed...)
	malformedBeforeCommit[len(nodePayload)+recordHeaderSize] ^= 0x01
	if _, err := ReplayCommitted(malformedBeforeCommit); !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("malformed record before commit err = %v", err)
	}
}

func missingNodeIDPayload() []byte {
	return []byte{0x81, 0xa4, 't', 'y', 'p', 'e', 0xa4, 'n', 'o', 't', 'e'}
}

func missingEdgeDeleteFieldPayload() []byte {
	return []byte{0x82, 0xa9, 'f', 'r', 'o', 'm', '_', 'n', 'o', 'd', 'e', 0xa1, 'a', 0xa8, 'r', 'e', 'l', 'a', 't', 'i', 'o', 'n', 0xa5, 'l', 'i', 'n', 'k', 's'}
}

func corruptWAL(in []byte, fn func([]byte)) []byte {
	out := append([]byte(nil), in...)
	fn(out)
	return out
}

func encodeRawWALRecord(seq uint64, op Operation, payload []byte) []byte {
	buf := make([]byte, recordHeaderSize+len(payload)+4)
	binary.LittleEndian.PutUint64(buf[0:8], seq)
	buf[8] = byte(op)
	binary.LittleEndian.PutUint32(buf[9:13], uint32(len(payload)))
	copy(buf[recordHeaderSize:], payload)
	binary.LittleEndian.PutUint32(buf[recordHeaderSize+len(payload):], crc32.ChecksumIEEE(buf[:recordHeaderSize+len(payload)]))
	return buf
}
