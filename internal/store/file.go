package store

import (
	"bytes"
	"errors"
	"os"

	"github.com/earendil-works/akg/internal/format"
	"github.com/earendil-works/akg/internal/record"
	"github.com/earendil-works/akg/internal/state"
	"github.com/earendil-works/akg/internal/wal"
)

var (
	ErrInvalidWALReplay = errors.New("invalid wal replay")
	ErrBloomMismatch    = errors.New("bloom mismatch")
)

// Store is the minimal internal file-level store state produced by ordinary
// create/open. It intentionally exposes only live authoritative state plus WAL
// bookkeeping needed by later commit/compaction tasks.
type Store struct {
	path                  string
	state                 *state.State
	nextWALSequence       wal.SequenceNumber
	uncompactedWALEntries int
	uncompactedWALBytes   int
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
	committed []wal.Record
	next      wal.SequenceNumber
	entries   int
}

func inspectWAL(payload []byte) (walInfo, error) {
	info := walInfo{next: 1}
	var all []wal.Record
	lastCommit := -1
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
		}
		payload = payload[n:]
	}
	if lastCommit < 0 {
		return info, nil
	}
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
