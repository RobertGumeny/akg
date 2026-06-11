package akg

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
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

// osRename is a seam so tests can inject a rename failure.
var osRename = os.Rename

// writeFileAtomicRename durably replaces path with data using a crash-atomic
// sequence: write a same-directory temp file, fsync it, rename it over the
// target, then fsync the directory. A crash before the rename leaves the prior
// committed file fully intact; the rename itself is atomic. The temp is removed
// on any error before the rename.
//
// Permissions match the TypeScript SDK: the temp is created with the existing
// file's mode (or 0o666 for a new file) via O_CREATE|O_EXCL, letting the kernel
// apply umask. There is no explicit chmod, so umask is honored for new files.
func writeFileAtomicRename(path string, data []byte) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	mode := os.FileMode(0o666)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, tmpName, err := createTempExcl(dir, base, mode)
	if err != nil {
		return fmt.Errorf("akg: atomic write to %q: %w", path, err)
	}

	n, writeErr := tmp.Write(data)
	syncErr := tmp.Sync()
	closeErr := tmp.Close()
	if writeErr != nil {
		os.Remove(tmpName)
		return fmt.Errorf("akg: atomic write to %q: %w", path, writeErr)
	}
	if n != len(data) {
		os.Remove(tmpName)
		return fmt.Errorf("akg: atomic write to %q: short write (%d of %d bytes)", path, n, len(data))
	}
	if syncErr != nil {
		os.Remove(tmpName)
		return fmt.Errorf("akg: atomic write to %q: %w", path, syncErr)
	}
	if closeErr != nil {
		os.Remove(tmpName)
		return fmt.Errorf("akg: atomic write to %q: %w", path, closeErr)
	}

	if err := osRename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("akg: atomic write to %q: %w", path, err)
	}

	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
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
