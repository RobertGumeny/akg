package wal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"

	"github.com/earendil-works/akg/internal/record"
)

const recordHeaderSize = 13

var (
	ErrInvalidRecord    = errors.New("invalid wal record")
	ErrChecksumMismatch = errors.New("wal checksum mismatch")
	ErrUnknownOperation = errors.New("unknown wal operation")
)

// EncodeRecord encodes one WAL record with a CRC32 over sequence, operation, length, and payload.
func EncodeRecord(r Record) ([]byte, error) {
	if !r.Operation.Valid() {
		return nil, ErrUnknownOperation
	}
	if r.Operation == OpCommit && len(r.Payload) != 0 {
		return nil, ErrInvalidRecord
	}
	buf := make([]byte, recordHeaderSize+len(r.Payload)+4)
	binary.LittleEndian.PutUint64(buf[0:8], uint64(r.Sequence))
	buf[8] = byte(r.Operation)
	binary.LittleEndian.PutUint32(buf[9:13], uint32(len(r.Payload)))
	copy(buf[13:], r.Payload)
	binary.LittleEndian.PutUint32(buf[13+len(r.Payload):], crc32.ChecksumIEEE(buf[:13+len(r.Payload)]))
	return buf, nil
}

// DecodeRecord decodes one complete WAL record from the front of buf.
func DecodeRecord(buf []byte) (Record, int, error) {
	if len(buf) < recordHeaderSize {
		return Record{}, 0, ErrInvalidRecord
	}
	seq := binary.LittleEndian.Uint64(buf[0:8])
	op := Operation(buf[8])
	if !op.Valid() {
		return Record{}, 0, ErrUnknownOperation
	}
	length := binary.LittleEndian.Uint32(buf[9:13])
	need := recordHeaderSize + int(length) + 4
	if uint64(length) > uint64(len(buf)-recordHeaderSize) || len(buf) < need {
		return Record{}, 0, ErrInvalidRecord
	}
	got := binary.LittleEndian.Uint32(buf[recordHeaderSize+int(length) : need])
	want := crc32.ChecksumIEEE(buf[:recordHeaderSize+int(length)])
	if got != want {
		return Record{}, 0, ErrChecksumMismatch
	}
	if op == OpCommit && length != 0 {
		return Record{}, 0, ErrInvalidRecord
	}
	payload := append([]byte(nil), buf[recordHeaderSize:recordHeaderSize+int(length)]...)
	return Record{Sequence: SequenceNumber(seq), Operation: op, Payload: payload}, need, nil
}

// EncodeRecords encodes records in order.
func EncodeRecords(records []Record) ([]byte, error) {
	var out bytes.Buffer
	for _, r := range records {
		b, err := EncodeRecord(r)
		if err != nil {
			return nil, err
		}
		out.Write(b)
	}
	return out.Bytes(), nil
}

// DecodeRecordsStrict decodes the entire WAL payload and rejects any malformed record.
func DecodeRecordsStrict(payload []byte) ([]Record, error) {
	var records []Record
	for len(payload) > 0 {
		r, n, err := DecodeRecord(payload)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
		payload = payload[n:]
	}
	return records, nil
}

// CommittedRecords returns records through the last valid COMMIT, excluding COMMIT records.
// Malformed bytes after the last valid COMMIT are ignored as trailing uncommitted WAL.
func CommittedRecords(payload []byte) ([]Record, error) {
	var all []Record
	lastCommit := -1
	for len(payload) > 0 {
		r, n, err := DecodeRecord(payload)
		if err != nil {
			if lastCommit >= 0 {
				break
			}
			return nil, err
		}
		all = append(all, r)
		if r.Operation == OpCommit {
			lastCommit = len(all) - 1
		}
		payload = payload[n:]
	}
	if lastCommit < 0 {
		return nil, nil
	}
	committed := make([]Record, 0, lastCommit)
	for _, r := range all[:lastCommit+1] {
		if r.Operation != OpCommit {
			committed = append(committed, r)
		}
	}
	return committed, nil
}

// ValidatePayload applies operation-specific MessagePack payload validation.
func ValidatePayload(r Record) error {
	switch r.Operation {
	case OpPutNode:
		_, err := record.DecodeNodePutPayload(r.Payload)
		return err
	case OpPutEdge:
		_, err := record.DecodeEdgePayload(r.Payload)
		return err
	case OpDeleteNode:
		_, err := record.DecodeNodeDeletePayload(r.Payload)
		return err
	case OpDeleteEdge:
		_, err := record.DecodeEdgeDeletePayload(r.Payload)
		return err
	case OpCommit:
		if len(r.Payload) != 0 {
			return ErrInvalidRecord
		}
		return nil
	default:
		return ErrUnknownOperation
	}
}

// ReplayCommitted validates and returns operation records through the last valid COMMIT.
func ReplayCommitted(payload []byte) ([]Record, error) {
	recs, err := CommittedRecords(payload)
	if err != nil {
		return nil, err
	}
	for _, r := range recs {
		if err := ValidatePayload(r); err != nil {
			return nil, err
		}
	}
	return recs, nil
}
