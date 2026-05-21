package store

import (
	"errors"
	"os"
	"testing"

	"github.com/earendil-works/akg/internal/format"
	"github.com/earendil-works/akg/internal/record"
	"github.com/earendil-works/akg/internal/state"
	"github.com/earendil-works/akg/internal/wal"
)

func TestCreateOpenValidateEmptyFile(t *testing.T) {
	path := tempPath(t)
	st, err := Create(path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(st.State().Nodes()) != 0 || len(st.State().Edges()) != 0 {
		t.Fatalf("created store is not empty")
	}
	if st.NextWALSequence() != 1 || st.UncompactedWALEntries() != 0 || st.UncompactedWALBytes() != 0 {
		t.Fatalf("unexpected WAL bookkeeping: next=%d entries=%d bytes=%d", st.NextWALSequence(), st.UncompactedWALEntries(), st.UncompactedWALBytes())
	}
	if err := Validate(path); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestOpenCleanCompactedFileExposesLiveState(t *testing.T) {
	path := tempPath(t)
	s := state.New(state.WithNow(fixedClock(10)))
	if _, err := s.PutNode("n1", record.Node{Type: "note", Title: "A", Tags: []string{"topic"}}); err != nil {
		t.Fatal(err)
	}
	writeStoreFile(t, path, s, nil)

	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, ok := st.State().GetNode("note", "n1"); !ok {
		t.Fatalf("node from clean Data section missing after open")
	}
}

func TestOpenAppliesCommittedWALAndIgnoresUncommittedTail(t *testing.T) {
	path := tempPath(t)
	base := state.New(state.WithNow(fixedClock(1)))
	if _, err := base.PutNode("n1", record.Node{Type: "note", Title: "A"}); err != nil {
		t.Fatal(err)
	}
	committedNode := nodePutPayload("n2", record.Node{Type: "note", Title: "B", CreatedAt: 2, UpdatedAt: 2, Version: 1})
	committedEdge := mustEdgePayload(t, record.Edge{FromNode: "n1", Relation: "links", ToNode: "n2", CreatedAt: 3, UpdatedAt: 3, Version: 1})
	uncommittedEdge := mustEdgePayload(t, record.Edge{FromNode: "n2", Relation: "links", ToNode: "n3", CreatedAt: 4, UpdatedAt: 4, Version: 1})
	walPayload := mustWAL(t, []wal.Record{
		{Sequence: 7, Operation: wal.OpPutNode, Payload: committedNode},
		{Sequence: 8, Operation: wal.OpPutEdge, Payload: committedEdge},
		{Sequence: 9, Operation: wal.OpCommit},
		{Sequence: 10, Operation: wal.OpPutEdge, Payload: uncommittedEdge},
	})
	walPayload = append(walPayload, 0x01, 0x02) // malformed trailing uncommitted tail
	writeStoreFile(t, path, base, walPayload)

	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, ok := st.State().GetNode("note", "n2"); !ok {
		t.Fatalf("committed WAL node was not replayed")
	}
	if _, ok := st.State().GetEdge("n1", "links", "n2"); !ok {
		t.Fatalf("committed WAL edge was not replayed")
	}
	if _, ok := st.State().GetEdge("n2", "links", "n3"); ok {
		t.Fatalf("trailing uncommitted WAL edge was replayed")
	}
	if st.NextWALSequence() != 11 || st.UncompactedWALEntries() != 4 || st.UncompactedWALBytes() != len(walPayload) {
		t.Fatalf("unexpected WAL bookkeeping: next=%d entries=%d bytes=%d", st.NextWALSequence(), st.UncompactedWALEntries(), st.UncompactedWALBytes())
	}
}

func TestOpenWithNoValidCommitAppliesNoWALMutations(t *testing.T) {
	path := tempPath(t)
	base := state.New()
	walPayload := mustWAL(t, []wal.Record{{Sequence: 1, Operation: wal.OpPutEdge, Payload: mustEdgePayload(t, record.Edge{FromNode: "a", Relation: "r", ToNode: "b"})}})
	writeStoreFile(t, path, base, walPayload)

	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(st.State().Edges()) != 0 {
		t.Fatalf("uncommitted WAL mutation was applied")
	}
	if st.NextWALSequence() != 2 || st.UncompactedWALEntries() != 1 {
		t.Fatalf("unexpected WAL bookkeeping: next=%d entries=%d", st.NextWALSequence(), st.UncompactedWALEntries())
	}
}

func TestOpenRejectsMalformedCommittedWALAndInvalidPayload(t *testing.T) {
	base := state.New()

	malformedPath := tempPath(t)
	bad := mustWAL(t, []wal.Record{{Sequence: 1, Operation: wal.OpPutEdge, Payload: mustEdgePayload(t, record.Edge{FromNode: "a", Relation: "r", ToNode: "b"})}, {Sequence: 2, Operation: wal.OpCommit}})
	bad[3] ^= 0xff
	writeStoreFile(t, malformedPath, base, bad)
	if _, err := Open(malformedPath); err == nil {
		t.Fatalf("expected malformed committed WAL rejection")
	}

	invalidPayloadPath := tempPath(t)
	invalid := mustWAL(t, []wal.Record{{Sequence: 1, Operation: wal.OpPutEdge, Payload: []byte{0x80}}, {Sequence: 2, Operation: wal.OpCommit}})
	writeStoreFile(t, invalidPayloadPath, base, invalid)
	if _, err := Open(invalidPayloadPath); err == nil {
		t.Fatalf("expected invalid committed WAL payload rejection")
	}
}

func TestCommitPersistsMutationViaWALReplay(t *testing.T) {
	path := tempPath(t)
	st, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutNode("n1", record.Node{Type: "note", Title: "A"}); err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	if err := st.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if _, ok := st.State().GetNode("note", "n1"); !ok {
		t.Fatalf("committed in-memory node missing")
	}
	if st.NextWALSequence() != 3 || st.UncompactedWALEntries() != 2 || st.UncompactedWALBytes() == 0 {
		t.Fatalf("unexpected WAL bookkeeping after commit: next=%d entries=%d bytes=%d", st.NextWALSequence(), st.UncompactedWALEntries(), st.UncompactedWALBytes())
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, ok := reopened.State().GetNode("note", "n1"); !ok {
		t.Fatalf("committed node did not survive reopen")
	}
	records := readWALRecords(t, path)
	if len(records) != 2 || records[0].Operation != wal.OpPutNode || records[1].Operation != wal.OpCommit || len(records[1].Payload) != 0 {
		t.Fatalf("unexpected WAL records after commit: %#v", records)
	}
}

func TestUncommittedMutationDoesNotReopen(t *testing.T) {
	path := tempPath(t)
	st, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutNode("n1", record.Node{Type: "note", Title: "A"}); err != nil {
		t.Fatalf("PutNode: %v", err)
	}
	if _, ok := st.State().GetNode("note", "n1"); !ok {
		t.Fatalf("pending node should be visible in current store state")
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, ok := reopened.State().GetNode("note", "n1"); ok {
		t.Fatalf("uncommitted node was applied after reopen")
	}
}

func TestMultipleCommittedBatchesReplayInSequenceAcrossSessions(t *testing.T) {
	path := tempPath(t)
	st, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutNode("n1", record.Node{Type: "note", Title: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Commit(); err != nil {
		t.Fatal(err)
	}
	st, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutNode("n2", record.Node{Type: "note", Title: "B"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutEdge(record.Edge{FromNode: "n1", Relation: "links", ToNode: "n2"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Commit(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, ok := reopened.State().GetNode("note", "n1"); !ok {
		t.Fatalf("first batch node missing")
	}
	if _, ok := reopened.State().GetNode("note", "n2"); !ok {
		t.Fatalf("second batch node missing")
	}
	if _, ok := reopened.State().GetEdge("n1", "links", "n2"); !ok {
		t.Fatalf("second batch edge missing")
	}
	records := readWALRecords(t, path)
	if len(records) != 5 {
		t.Fatalf("WAL record count = %d, want 5", len(records))
	}
	for i, r := range records {
		want := wal.SequenceNumber(i + 1)
		if r.Sequence != want {
			t.Fatalf("record %d sequence = %d, want %d", i, r.Sequence, want)
		}
	}
	if reopened.NextWALSequence() != 6 {
		t.Fatalf("next WAL sequence = %d, want 6", reopened.NextWALSequence())
	}
}

func TestCloseCommitsOutstandingMutation(t *testing.T) {
	path := tempPath(t)
	st, err := Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutNode("n1", record.Node{Type: "note", Title: "A"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reopened.State().GetNode("note", "n1"); !ok {
		t.Fatalf("Close did not commit outstanding node")
	}
}

func TestCommitDiscardsIgnoredWALTailBeforeAppending(t *testing.T) {
	path := tempPath(t)
	base := state.New()
	committed := nodePutPayload("n1", record.Node{Type: "note", Title: "A", CreatedAt: 1, UpdatedAt: 1, Version: 1})
	uncommitted := nodePutPayload("n2", record.Node{Type: "note", Title: "B", CreatedAt: 2, UpdatedAt: 2, Version: 1})
	walPayload := mustWAL(t, []wal.Record{
		{Sequence: 1, Operation: wal.OpPutNode, Payload: committed},
		{Sequence: 2, Operation: wal.OpCommit},
		{Sequence: 3, Operation: wal.OpPutNode, Payload: uncommitted},
	})
	writeStoreFile(t, path, base, walPayload)
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.PutNode("n3", record.Node{Type: "note", Title: "C"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Commit(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reopened.State().GetNode("note", "n2"); ok {
		t.Fatalf("previous uncommitted tail became committed")
	}
	if _, ok := reopened.State().GetNode("note", "n3"); !ok {
		t.Fatalf("new committed node missing")
	}
}

func TestWALThresholdDetection(t *testing.T) {
	if (&Store{uncompactedWALEntries: 999, uncompactedWALBytes: walByteFlushThreshold - 1}).walThresholdExceeded() {
		t.Fatalf("threshold fired early")
	}
	if !(&Store{uncompactedWALEntries: 1000}).walThresholdExceeded() {
		t.Fatalf("entry threshold did not fire")
	}
	if !(&Store{uncompactedWALBytes: walByteFlushThreshold}).walThresholdExceeded() {
		t.Fatalf("byte threshold did not fire")
	}
}

func TestOpenRejectsInvalidDataBloomAndContainer(t *testing.T) {
	checksumPath := tempPath(t)
	writeStoreFile(t, checksumPath, state.New(), nil)
	corrupt, err := os.ReadFile(checksumPath)
	if err != nil {
		t.Fatal(err)
	}
	corrupt[len(corrupt)-1] ^= 0xff
	if err := os.WriteFile(checksumPath, corrupt, 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(checksumPath); !errors.Is(err, format.ErrChecksumMismatch) {
		t.Fatalf("Open checksum error = %v, want ErrChecksumMismatch", err)
	}

	path := tempPath(t)
	file, _, err := format.EncodeContainer(format.Container{Data: []byte{0x01}, Bloom: mustBloom(t, nil), WAL: []byte{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, file, 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); !errors.Is(err, format.ErrInvalidDataSection) {
		t.Fatalf("Open invalid Data error = %v, want ErrInvalidDataSection", err)
	}

	bloomPath := tempPath(t)
	file, _, err = format.EncodeContainer(format.Container{Data: mustData(t, nil), Bloom: []byte{0x01}, WAL: []byte{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bloomPath, file, 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(bloomPath); err == nil {
		t.Fatalf("expected invalid Bloom rejection")
	}

	containerPath := tempPath(t)
	if err := os.WriteFile(containerPath, []byte("not-akg"), 0o666); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(containerPath); !errors.Is(err, format.ErrInvalidHeader) {
		t.Fatalf("Open invalid container error = %v, want ErrInvalidHeader", err)
	}
}

func tempPath(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "*.akg")
	if err != nil {
		t.Fatal(err)
	}
	path := f.Name()
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func readWALRecords(t *testing.T, path string) []wal.Record {
	t.Helper()
	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	container, _, err := format.DecodeContainer(file)
	if err != nil {
		t.Fatal(err)
	}
	records, err := wal.DecodeRecordsStrict(container.WAL)
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func writeStoreFile(t *testing.T, path string, s *state.State, walPayload []byte) {
	t.Helper()
	entries, err := MaterializeDataEntries(s)
	if err != nil {
		t.Fatal(err)
	}
	data := mustData(t, entries)
	keys := make([][]byte, len(entries))
	for i, entry := range entries {
		keys[i] = entry.Key
	}
	file, _, err := format.EncodeContainer(format.Container{Data: data, Bloom: mustBloom(t, keys), WAL: walPayload})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, file, 0o666); err != nil {
		t.Fatal(err)
	}
}

func mustData(t *testing.T, entries []format.DataEntry) []byte {
	t.Helper()
	data, err := format.EncodeDataEntries(entries)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustBloom(t *testing.T, keys [][]byte) []byte {
	t.Helper()
	bloom, err := format.EncodeBloom(keys)
	if err != nil {
		t.Fatal(err)
	}
	return bloom
}

func nodePutPayload(id string, n record.Node) []byte {
	var out []byte
	out = append(out, 0x85)
	appendMsgpackString(&out, "created_at")
	out = append(out, byte(n.CreatedAt))
	appendMsgpackString(&out, "id")
	appendMsgpackString(&out, id)
	appendMsgpackString(&out, "title")
	appendMsgpackString(&out, n.Title)
	appendMsgpackString(&out, "type")
	appendMsgpackString(&out, n.Type)
	appendMsgpackString(&out, "updated_at")
	out = append(out, byte(n.UpdatedAt))
	return out
}

func mustEdgePayload(t *testing.T, e record.Edge) []byte {
	t.Helper()
	p, err := record.EncodeEdgePayload(e)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustWAL(t *testing.T, records []wal.Record) []byte {
	t.Helper()
	payload, err := wal.EncodeRecords(records)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}
