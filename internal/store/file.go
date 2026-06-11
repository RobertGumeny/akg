package store

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RobertGumeny/akg/internal/format"
	"github.com/RobertGumeny/akg/internal/record"
	"github.com/RobertGumeny/akg/internal/state"
	"github.com/RobertGumeny/akg/internal/wal"
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

func (s *Store) DeleteEdge(fromType string, from record.NodeID, relation record.Relation, toType string, to record.NodeID) error {
	if s == nil || s.state == nil {
		return state.ErrInvalidInput
	}
	if err := s.state.DeleteEdge(fromType, from, relation, toType, to); err != nil {
		return err
	}
	payload, err := record.EncodeEdgeDeletePayload(record.EdgeDelete{FromType: fromType, FromNode: from, Relation: relation, ToType: toType, ToNode: to})
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
	if err := writeFileAtomic(s.path, newFile); err != nil {
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

// Compact performs explicit whole-file compaction for path. It first opens the
// file through ordinary strict semantics (including committed WAL replay), then
// atomically replaces it with a fresh container containing only live Data,
// regenerated Bloom, and an empty WAL. Crash safety relies on writing and
// fsyncing a same-directory temporary file before os.Rename, then fsyncing the
// directory after replacement; a crash may leave the old file or the new file,
// plus at most a removable temporary file.
func Compact(path string) error {
	st, err := Open(path)
	if err != nil {
		return err
	}
	file, err := encodeCompactedFile(st.state)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, file)
}

// Compact rewrites this store's file using explicit compaction. Outstanding
// staged mutations are committed first so the compacted file preserves the
// store's currently visible logical state.
func (s *Store) Compact() error {
	if s == nil {
		return state.ErrInvalidInput
	}
	if err := s.Commit(); err != nil {
		return err
	}
	if err := Compact(s.path); err != nil {
		return err
	}
	reopened, err := Open(s.path)
	if err != nil {
		return err
	}
	*s = *reopened
	return nil
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
	var prev wal.SequenceNumber
	for i, r := range all[:lastCommit+1] {
		if i > 0 && r.Sequence <= prev {
			return walInfo{}, wal.ErrInvalidRecord
		}
		prev = r.Sequence
		if err := wal.ValidatePayload(r); err != nil {
			return walInfo{}, err
		}
		if r.Operation != wal.OpCommit {
			info.committed = append(info.committed, r)
		}
	}
	return info, nil
}

func encodeCompactedFile(s *state.State) ([]byte, error) {
	entries, err := MaterializeDataEntries(s)
	if err != nil {
		return nil, err
	}
	data, err := format.EncodeDataEntries(entries)
	if err != nil {
		return nil, err
	}
	keys := make([][]byte, len(entries))
	for i, entry := range entries {
		keys[i] = entry.Key
	}
	bloom, err := format.EncodeBloom(keys)
	if err != nil {
		return nil, err
	}
	file, _, err := format.EncodeContainer(format.Container{Data: data, Bloom: bloom, WAL: []byte{}})
	if err != nil {
		return nil, err
	}
	return file, nil
}

// osRename is a seam so tests can inject a rename failure.
var osRename = os.Rename

// writeFileAtomic durably replaces path with data using a crash-atomic
// sequence: write a same-directory temp file, fsync it, rename it over the
// target, then fsync the directory. A crash before the rename leaves the prior
// committed file fully intact; the rename itself is atomic. The temp is removed
// on any error before the rename.
//
// The temp is created with the existing file's mode (or 0o666 for a new file)
// via O_CREATE|O_EXCL, letting the kernel apply umask. There is no explicit
// chmod, so umask is honored for new files. This matches the akg-go and akg-ts
// application SDKs byte-for-byte.
func writeFileAtomic(path string, data []byte) error {
	dirPath := filepath.Dir(path)
	base := filepath.Base(path)

	mode := os.FileMode(0o666)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, tmpPath, err := createTempExcl(dirPath, base, mode)
	if err != nil {
		return fmt.Errorf("akg: atomic write to %q: %w", path, err)
	}

	n, writeErr := tmp.Write(data)
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if writeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("akg: atomic write to %q: %w", path, writeErr)
	}
	if n != len(data) {
		os.Remove(tmpPath)
		return fmt.Errorf("akg: atomic write to %q: short write (%d of %d bytes)", path, n, len(data))
	}
	if syncErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("akg: atomic write to %q: %w", path, syncErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("akg: atomic write to %q: %w", path, closeErr)
	}

	if err := osRename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("akg: atomic write to %q: %w", path, err)
	}

	if dir, err := os.Open(dirPath); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
	return nil
}

// createTempExcl creates a uniquely named temp file in dir with the given mode,
// using O_CREATE|O_EXCL so the kernel applies umask and an existing file is
// never clobbered. The name is ".<base>.akg.tmp-<random hex>".
func createTempExcl(dir, base string, mode os.FileMode) (*os.File, string, error) {
	for i := 0; i < 10000; i++ {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			return nil, "", err
		}
		name := filepath.Join(dir, "."+base+".akg.tmp-"+hex.EncodeToString(b[:]))
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return nil, "", err
		}
		return f, name, nil
	}
	return nil, "", fmt.Errorf("could not create a unique temp file in %q", dir)
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
			if err := s.DeleteEdge(del.FromType, del.FromNode, del.Relation, del.ToType, del.ToNode); err != nil {
				return err
			}
		default:
			return ErrInvalidWALReplay
		}
	}
	return nil
}
