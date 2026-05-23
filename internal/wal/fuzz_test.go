package wal

import (
	"testing"

	"github.com/RobertGumeny/akg/internal/record"
)

func FuzzDecodeRecord(f *testing.F) {
	payload, _ := record.EncodeNodePayload(record.Node{Type: "note", Title: "hello", CreatedAt: 1, UpdatedAt: 1})
	valid, _ := EncodeRecord(Record{Sequence: 1, Operation: OpPutNode, Payload: payload})
	commit, _ := EncodeRecord(Record{Sequence: 2, Operation: OpCommit})
	f.Add([]byte{})
	f.Add([]byte{1, 0, 0, 0})
	f.Add(valid)
	f.Add(commit)

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _ = DecodeRecord(data)
	})
}

func FuzzDecodeRecordsStrict(f *testing.F) {
	payload, _ := record.EncodeNodePayload(record.Node{Type: "note", Title: "hello", CreatedAt: 1, UpdatedAt: 1})
	valid, _ := EncodeRecords([]Record{{Sequence: 1, Operation: OpPutNode, Payload: payload}, {Sequence: 2, Operation: OpCommit}})
	f.Add([]byte{})
	f.Add([]byte{1, 0, 0, 0})
	f.Add(valid)

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeRecordsStrict(data)
	})
}

func FuzzCommittedRecords(f *testing.F) {
	payload, _ := record.EncodeNodePayload(record.Node{Type: "note", Title: "hello", CreatedAt: 1, UpdatedAt: 1})
	valid, _ := EncodeRecords([]Record{{Sequence: 1, Operation: OpPutNode, Payload: payload}, {Sequence: 2, Operation: OpCommit}})
	uncommitted, _ := EncodeRecord(Record{Sequence: 3, Operation: OpPutNode, Payload: payload})
	f.Add([]byte{})
	f.Add([]byte{1, 0, 0, 0})
	f.Add(valid)
	f.Add(append(append([]byte(nil), valid...), uncommitted...))

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = CommittedRecords(data)
	})
}

func FuzzReplayCommitted(f *testing.F) {
	payload, _ := record.EncodeNodePayload(record.Node{Type: "note", Title: "hello", CreatedAt: 1, UpdatedAt: 1})
	valid, _ := EncodeRecords([]Record{{Sequence: 1, Operation: OpPutNode, Payload: payload}, {Sequence: 2, Operation: OpCommit}})
	f.Add([]byte{})
	f.Add([]byte{1, 0, 0, 0})
	f.Add(valid)

	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ReplayCommitted(data)
	})
}
