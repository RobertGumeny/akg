package store

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"github.com/earendil-works/akg/internal/format"
	"github.com/earendil-works/akg/internal/record"
	"github.com/earendil-works/akg/internal/state"
	"github.com/earendil-works/akg/internal/wal"
)

const (
	walEntryFlushThreshold = 1000
	walByteFlushThreshold  = 10 * 1024 * 1024
)

var (
	ErrInvalidWALReplay = errors.New("invalid wal replay")
	ErrBloomMismatch    = errors.New("bloom mismatch")
)

type pendingWALRecord struct {
	op      wal.Operation
	payload []byte
}

// Store is the minimal internal file-level store state produced by ordinary
// create/open. It intentionally exposes only live authoritative state plus WAL
// bookkeeping needed by later commit/compaction tasks.
type Store struct {
	path                  string
	state                 *state.State
	pending               []pendingWALRecord
	nextWALSequence       wal.SequenceNumber
	uncompactedWALEntries int
	uncompactedWALBytes   int
	walAppendBytes        int
	walAppendEntries      int
}

func Path(s *Store) string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *Store) State() *state.State {
	if s == nil {
		return nil
	}
	return s.state
}

func (s *Store) NextWALSequence() wal.SequenceNumber {
	if s == nil {
		return 0
	}
	return s.nextWALSequence
}

func (s *Store) UncompactedWALEntries() int {
	if s == nil {
		return 0
	}
	return s.uncompactedWALEntries
}

func (s *Store) UncompactedWALBytes() int {
	if s == nil {
		return 0
	}
	return s.uncompactedWALBytes
}

func (s *Store) walThresholdExceeded() bool {
	if s == nil {
		return false
	}
	return s.uncompactedWALEntries >= walEntryFlushThreshold || s.uncompactedWALBytes >= walByteFlushThreshold
}

func (s *Store) PutNode(id record.NodeID, n record.Node) (state.NodeRecord, error) {
	if s == nil || s.state == nil {
		return state.NodeRecord{}, state.ErrInvalidInput
	}
	rec, err := s.state.PutNode(id, n)
	if err != nil {
		return state.NodeRecord{}, err
	}
	payload, err := record.EncodeNodePutPayload(record.NodePut{ID: rec.ID, Node: rec.Node})
	if err != nil {
		return state.NodeRecord{}, err
	}
	s.pending = append(s.pending, pendingWALRecord{op: wal.OpPutNode, payload: payload})
	return rec, nil
}

func (s *Store) PutEdge(e record.Edge) (record.Edge, error) {
	if s == nil || s.state == nil {
		return record.Edge{}, state.ErrInvalidInput
	}
	edge, err := s.state.PutEdge(e)
	if err != nil {
		return record.Edge{}, err
	}
	payload, err := record.EncodeEdgePayload(edge)
	if err != nil {
		return record.Edge{}, err
	}
	s.pending = append(s.pending, pendingWALRecord{op: wal.OpPutEdge, payload: payload})
	return edge, nil
}

func (s *Store) DeleteNode(typeName string, id record.NodeID) error {
	if s == nil || s.state == nil {
		return state.ErrInvalidInput
	}
	if err := s.state.DeleteNode(typeName, id); err != nil {
		return err
	}
	payload, err := record.EncodeNodeDeletePayload(record.NodeDelete{Type: typeName, ID: id})
	if err != nil {
		return err
	}
	s.pending = append(s.pending, pendingWALRecord{op: wal.OpDeleteNode, payload: payload})
	return nil
}

func (s *Store) DeleteEdge(from record.NodeID, relation record.Relation, to record.NodeID) error {
	if s == nil || s.state == nil {
		return state.ErrInvalidInput
	}
	if err := s.state.DeleteEdge(from, relation, to); err != nil {
		return err
	}
	payload, err := record.EncodeEdgeDeletePayload(record.EdgeDelete{FromNode: from, Relation: relation, ToNode: to})
	if err != nil {
		return err
	}
	s.pending = append(s.pending, pendingWALRecord{op: wal.OpDeleteEdge, payload: payload})
	return nil
}

func (s *Store) Commit() error {
	if s == nil {
		return state.ErrInvalidInput
	}
	if len(s.pending) == 0 {
		return nil
	}
	file, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	container, _, err := format.DecodeContainer(file)
	if err != nil {
		return err
	}
	if s.walAppendBytes > len(container.WAL) {
		return ErrInvalidWALReplay
	}
	walPrefix := append([]byte(nil), container.WAL[:s.walAppendBytes]...)
	records := make([]wal.Record, 0, len(s.pending)+1)
	next := s.nextWALSequence
	for _, p := range s.pending {
		records = append(records, wal.Record{Sequence: next, Operation: p.op, Payload: append([]byte(nil), p.payload...)})
		next++
	}
	records = append(records, wal.Record{Sequence: next, Operation: wal.OpCommit})
	encoded, err := wal.EncodeRecords(records)
	if err != nil {
		return err
	}
	newWAL := append(walPrefix, encoded...)
	newFile, _, err := format.EncodeContainer(format.Container{Data: container.Data, Bloom: container.Bloom, WAL: newWAL})
	if err != nil {
		return err
	}
	if err := writeFileSync(s.path, newFile); err != nil {
		return err
	}
	s.pending = nil
	s.nextWALSequence = next + 1
	s.uncompactedWALEntries = s.walAppendEntries + len(records)
	s.uncompactedWALBytes = len(newWAL)
	s.walAppendEntries = s.uncompactedWALEntries
	s.walAppendBytes = s.uncompactedWALBytes
	return nil
}

func (s *Store) Close() error {
	return s.Commit()
}

// Create writes a new AKG file containing an empty Data section, deterministic
// empty Bloom state, and an empty WAL section, then opens it through the same
// validation path used for existing files.
func Create(path string) (*Store, error) {
	data, err := format.EncodeDataEntries(nil)
	if err != nil {
		return nil, err
	}
	bloom, err := format.EncodeBloom(nil)
	if err != nil {
		return nil, err
	}
	file, _, err := format.EncodeContainer(format.Container{Data: data, Bloom: bloom, WAL: []byte{}})
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, file, 0o666); err != nil {
		return nil, err
	}
	return Open(path)
}

// Open decodes and validates an AKG file, hydrates live Data entries into
// authoritative state, replays committed WAL records through the last valid
// COMMIT, and ignores trailing uncommitted WAL records.
func Open(path string) (*Store, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	st, err := openBytes(file)
	if err != nil {
		return nil, err
	}
	st.path = path
	return st, nil
}

// Validate verifies that path opens under ordinary strict validation semantics
// without changing the file or exposing extra API surface.
func Validate(path string) error {
	_, err := Open(path)
	return err
}

func openBytes(file []byte) (*Store, error) {
	container, _, err := format.DecodeContainer(file)
	if err != nil {
		return nil, err
	}
	entries, err := format.DecodeDataEntries(container.Data)
	if err != nil {
		return nil, err
	}
	if err := validateBloom(container.Bloom, entries); err != nil {
		return nil, err
	}
	st, err := HydrateDataEntries(entries)
	if err != nil {
		return nil, err
	}
	info, err := inspectWAL(container.WAL)
	if err != nil {
		return nil, err
	}
	if err := replayWAL(st, info.committed); err != nil {
		return nil, err
	}
	return &Store{
		state:                 st,
		nextWALSequence:       info.next,
		uncompactedWALEntries: info.entries,
		uncompactedWALBytes:   len(container.WAL),
		walAppendBytes:        info.appendBytes,
		walAppendEntries:      info.appendEntries,
	}, nil
}

func validateBloom(payload []byte, entries []format.DataEntry) error {
	if payload == nil {
		return nil
	}
	if _, err := format.DecodeBloom(payload); err != nil {
		return err
	}
	keys := make([][]byte, len(entries))
	for i, entry := range entries {
		keys[i] = entry.Key
	}
	want, err := format.EncodeBloom(keys)
	if err != nil {
		return err
	}
	if !bytes.Equal(payload, want) {
		return ErrBloomMismatch
	}
	return nil
}

type walInfo struct {
	committed     []wal.Record
	next          wal.SequenceNumber
	entries       int
	appendBytes   int
	appendEntries int
}

func inspectWAL(payload []byte) (walInfo, error) {
	info := walInfo{next: 1}
	var all []wal.Record
	lastCommit := -1
	pos := 0
	lastCommitEnd := 0
	for len(payload) > 0 {
		r, n, err := wal.DecodeRecord(payload)
		if err != nil {
			if lastCommit >= 0 {
				break
			}
			return walInfo{}, err
		}
		all = append(all, r)
		info.entries++
		if r.Sequence >= info.next {
			info.next = r.Sequence + 1
		}
		if r.Operation == wal.OpCommit {
			lastCommit = len(all) - 1
			lastCommitEnd = pos + n
		}
		pos += n
		payload = payload[n:]
	}
	if lastCommit < 0 {
		info.appendBytes = 0
		info.appendEntries = 0
		return info, nil
	}
	info.appendBytes = lastCommitEnd
	info.appendEntries = lastCommit + 1
	for _, r := range all[:lastCommit+1] {
		if err := wal.ValidatePayload(r); err != nil {
			return walInfo{}, err
		}
		if r.Operation != wal.OpCommit {
			info.committed = append(info.committed, r)
		}
	}
	return info, nil
}

func writeFileSync(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	n, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if n != len(data) {
		return errors.New("short file write")
	}
	if syncErr != nil {
		return syncErr
	}
	if closeErr != nil {
		return closeErr
	}
	if dir, err := os.Open(filepath.Dir(path)); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

func replayWAL(s *state.State, records []wal.Record) error {
	for _, r := range records {
		switch r.Operation {
		case wal.OpPutNode:
			put, err := record.DecodeNodePutPayload(r.Payload)
			if err != nil {
				return err
			}
			if err := s.LoadNodeRecord(state.NodeRecord{ID: put.ID, Node: put.Node}); err != nil {
				return err
			}
		case wal.OpPutEdge:
			edge, err := record.DecodeEdgePayload(r.Payload)
			if err != nil {
				return err
			}
			if err := s.LoadEdge(edge); err != nil {
				return err
			}
		case wal.OpDeleteNode:
			del, err := record.DecodeNodeDeletePayload(r.Payload)
			if err != nil {
				return err
			}
			if err := s.DeleteNode(del.Type, del.ID); err != nil {
				return err
			}
		case wal.OpDeleteEdge:
			del, err := record.DecodeEdgeDeletePayload(r.Payload)
			if err != nil {
				return err
			}
			if err := s.DeleteEdge(del.FromNode, del.Relation, del.ToNode); err != nil {
				return err
			}
		default:
			return ErrInvalidWALReplay
		}
	}
	return nil
}
