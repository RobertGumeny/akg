package akg

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
)

const walRecordHeaderSize = 13

type walSequenceNumber uint64
type walOperation uint8

type walRecord struct {
	Sequence  walSequenceNumber
	Operation walOperation
	Payload   []byte
}

const (
	walOpPutNode    walOperation = 0x01
	walOpDeleteNode walOperation = 0x02
	walOpPutEdge    walOperation = 0x03
	walOpDeleteEdge walOperation = 0x04
	walOpCommit     walOperation = 0x05
)

var (
	errInvalidWALRecord    = errors.New("invalid wal record")
	errWALChecksumMismatch = errors.New("wal checksum mismatch")
	errUnknownWALOperation = errors.New("unknown wal operation")
)

func (op walOperation) valid() bool {
	switch op {
	case walOpPutNode, walOpDeleteNode, walOpPutEdge, walOpDeleteEdge, walOpCommit:
		return true
	default:
		return false
	}
}

func encodeWALRecord(r walRecord) ([]byte, error) {
	if !r.Operation.valid() {
		return nil, errUnknownWALOperation
	}
	if r.Operation == walOpCommit && len(r.Payload) != 0 {
		return nil, errInvalidWALRecord
	}
	buf := make([]byte, walRecordHeaderSize+len(r.Payload)+4)
	binary.LittleEndian.PutUint64(buf[0:8], uint64(r.Sequence))
	buf[8] = byte(r.Operation)
	binary.LittleEndian.PutUint32(buf[9:13], uint32(len(r.Payload)))
	copy(buf[13:], r.Payload)
	binary.LittleEndian.PutUint32(buf[13+len(r.Payload):], crc32.ChecksumIEEE(buf[:13+len(r.Payload)]))
	return buf, nil
}

func decodeWALRecord(buf []byte) (walRecord, int, error) {
	if len(buf) < walRecordHeaderSize {
		return walRecord{}, 0, errInvalidWALRecord
	}
	seq := binary.LittleEndian.Uint64(buf[0:8])
	op := walOperation(buf[8])
	if !op.valid() {
		return walRecord{}, 0, errUnknownWALOperation
	}
	length := binary.LittleEndian.Uint32(buf[9:13])
	need := walRecordHeaderSize + int(length) + 4
	if uint64(length) > uint64(len(buf)-walRecordHeaderSize) || len(buf) < need {
		return walRecord{}, 0, errInvalidWALRecord
	}
	got := binary.LittleEndian.Uint32(buf[walRecordHeaderSize+int(length) : need])
	want := crc32.ChecksumIEEE(buf[:walRecordHeaderSize+int(length)])
	if got != want {
		return walRecord{}, 0, errWALChecksumMismatch
	}
	if op == walOpCommit && length != 0 {
		return walRecord{}, 0, errInvalidWALRecord
	}
	payload := append([]byte(nil), buf[walRecordHeaderSize:walRecordHeaderSize+int(length)]...)
	return walRecord{Sequence: walSequenceNumber(seq), Operation: op, Payload: payload}, need, nil
}

func encodeWALRecords(records []walRecord) ([]byte, error) {
	var out bytes.Buffer
	for _, r := range records {
		b, err := encodeWALRecord(r)
		if err != nil {
			return nil, err
		}
		out.Write(b)
	}
	return out.Bytes(), nil
}

func validateWALPayload(r walRecord) error {
	switch r.Operation {
	case walOpPutNode:
		_, err := decodeNodePutPayload(r.Payload)
		return err
	case walOpDeleteNode:
		_, err := decodeNodeDeletePayload(r.Payload)
		return err
	case walOpPutEdge:
		_, err := decodeEdgePutPayload(r.Payload)
		return err
	case walOpDeleteEdge:
		_, err := decodeEdgeDeletePayload(r.Payload)
		return err
	case walOpCommit:
		if len(r.Payload) != 0 {
			return errInvalidWALRecord
		}
		return nil
	default:
		return errUnknownWALOperation
	}
}
