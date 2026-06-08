package akg

import (
	"os"
	"path/filepath"
)

// Compact commits any pending mutations and rewrites the store file to contain
// only live records, discarding all tombstones and prior WAL history.
//
// The auto-commit runs first: if it fails, compaction does not run. After a
// successful compaction the logical graph content is unchanged, the open store
// remains fully usable, and the file contains no WAL section.
//
// Compaction is always caller-triggered; it is never automatic.
func (s *Store) Compact() error {
	if s == nil || s.closed {
		return ErrInvalidInput
	}
	if err := s.Commit(); err != nil {
		return err
	}
	entries, err := materializeDataEntries(s.state)
	if err != nil {
		return err
	}
	data, err := encodeDataEntries(entries)
	if err != nil {
		return err
	}
	keys := make([][]byte, len(entries))
	for i, e := range entries {
		keys[i] = e.Key
	}
	file, err := encodeContainer(container{Data: data, Bloom: encodeBloom(keys)})
	if err != nil {
		return err
	}
	if err := writeFileAtomicRename(s.path, file); err != nil {
		return err
	}
	s.committedWAL = nil
	s.nextWALSeq = 1
	s.uncompactedWALBytes = 0
	s.pendingBytes = 0
	return nil
}

func writeFileAtomicRename(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".akg-compact-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_, writeErr := tmp.Write(data)
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if writeErr != nil {
		os.Remove(tmpName)
		return writeErr
	}
	if syncErr != nil {
		os.Remove(tmpName)
		return syncErr
	}
	if closeErr != nil {
		os.Remove(tmpName)
		return closeErr
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
