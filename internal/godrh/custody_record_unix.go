//go:build go1.25 && (linux || darwin)

package godrh

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const custodyRecordLease = ".god-custody-record-lease"
const custodyReservationFile = "reservation.json"
const custodyRetentionFile = "retained.json"

// Two immutable, exclusive files: original reservation then optional retention.
// No rename, truncation, deletion, rollback, repair or temporary-file cleanup.
// Partial files fail closed and require external review, not a silent reset.
type privateCustodyRecordStore struct {
	root      *os.Root
	lease     *os.File
	directory string
	identity  os.FileInfo
	loaded    *custodyRecordState
	closed    bool
}

func checkCustodyNewOutput(path string) error {
	r, leaf, err := custodyPrivateRoot(path)
	if err != nil {
		return ErrCustodyRecordFile
	}
	defer r.Close()
	if _, err = r.Lstat(leaf); !os.IsNotExist(err) {
		return ErrCustodyRecordFile
	}
	return nil
}

func (s *privateCustodyRecordStore) verify() error {
	if s.closed || s.root == nil || s.lease == nil {
		return ErrCustodyRecordStorage
	}
	current, err := os.Lstat(s.directory)
	if err != nil || !custodyDirectoryInfo(current) || !os.SameFile(s.identity, current) {
		return ErrCustodyRecordStorage
	}
	info, err := custodyFileInfo(s.lease, 1, true)
	if err != nil {
		return ErrCustodyRecordStorage
	}
	listed, err := s.root.Lstat(custodyRecordLease)
	if err != nil || listed.Mode() != 0600 || !os.SameFile(info, listed) {
		return ErrCustodyRecordStorage
	}
	dir, err := s.root.Open(".")
	if err != nil {
		return ErrCustodyRecordStorage
	}
	entries, readErr := dir.ReadDir(4)
	closeErr := dir.Close()
	if readErr != nil && readErr != io.EOF || closeErr != nil || len(entries) > 3 {
		return ErrCustodyRecordStorage
	}
	for _, e := range entries {
		if e.Name() != custodyRecordLease && e.Name() != custodyReservationFile && e.Name() != custodyRetentionFile {
			return ErrCustodyRecordStorage
		}
	}
	return nil
}

func (s *privateCustodyRecordStore) read(name string) ([]byte, error) {
	f, err := s.root.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, ErrCustodyRecordStorage
	}
	defer f.Close()
	before, err := custodyFileInfo(f, MaxCustodyRecordBytes, false)
	if err != nil {
		return nil, ErrCustodyRecordStorage
	}
	raw, err := io.ReadAll(io.LimitReader(f, MaxCustodyRecordBytes+1))
	if err != nil || int64(len(raw)) != before.Size() {
		return nil, ErrCustodyRecordStorage
	}
	after, err := custodyFileInfo(f, MaxCustodyRecordBytes, false)
	if err != nil || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return nil, ErrCustodyRecordStorage
	}
	listed, err := s.root.Lstat(name)
	if err != nil || listed.Mode() != 0600 || !os.SameFile(after, listed) {
		return nil, ErrCustodyRecordStorage
	}
	return raw, nil
}

func (s *privateCustodyRecordStore) Load() ([]byte, error) {
	if s.verify() != nil {
		return nil, ErrCustodyRecordStorage
	}
	_, err := s.root.Lstat(custodyReservationFile)
	if os.IsNotExist(err) {
		if _, retainedErr := s.root.Lstat(custodyRetentionFile); !os.IsNotExist(retainedErr) {
			return nil, ErrCustodyRecordStorage
		}
		return nil, nil
	}
	if err != nil {
		return nil, ErrCustodyRecordStorage
	}
	raw, err := s.read(custodyReservationFile)
	if err != nil {
		return nil, ErrCustodyRecordStorage
	}
	state, err := decodeCustodyRecord(raw)
	if err != nil || state.Stage != CustodyReserved {
		return nil, ErrCustodyRecordStorage
	}
	_, err = s.root.Lstat(custodyRetentionFile)
	if !os.IsNotExist(err) {
		if err != nil {
			return nil, ErrCustodyRecordStorage
		}
		retained, readErr := s.read(custodyRetentionFile)
		if readErr != nil {
			return nil, ErrCustodyRecordStorage
		}
		next, decodeErr := decodeCustodyRecord(retained)
		if decodeErr != nil || next.Stage != CustodyRetained || !sameCustodyRecord(state, next) {
			return nil, ErrCustodyRecordStorage
		}
		state, raw = next, retained
	}
	s.loaded = &state
	return bytes.Clone(raw), nil
}

func (s *privateCustodyRecordStore) Save(raw []byte) error {
	if s.verify() != nil {
		return ErrCustodyRecordStorage
	}
	next, err := decodeCustodyRecord(bytes.Clone(raw))
	if err != nil {
		return ErrCustodyRecordStorage
	}
	name := custodyReservationFile
	if s.loaded == nil {
		if next.Stage != CustodyReserved {
			return ErrCustodyRecordStorage
		}
	} else {
		if s.loaded.Stage != CustodyReserved || next.Stage != CustodyRetained || !sameCustodyRecord(*s.loaded, next) {
			return ErrCustodyRecordStorage
		}
		name = custodyRetentionFile
	}
	if writeCustodyPrivateFile(filepath.Join(s.directory, name), bytes.Clone(raw)) != nil || s.verify() != nil {
		return ErrCustodyRecordStorage
	}
	retained, err := s.read(name)
	if err != nil || !bytes.Equal(raw, retained) {
		return ErrCustodyRecordStorage
	}
	s.loaded = &next
	return nil
}

func (s *privateCustodyRecordStore) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	leaseErr, rootErr := s.lease.Close(), s.root.Close()
	if leaseErr != nil || rootErr != nil {
		return ErrCustodyRecordStorage
	}
	return nil
}

// Existing empty private directory for create; existing intact reservation for
// reopen. One cooperating-process lease is retained as an inode and never reset.
// Trusted leaf storage only: not hostile-owner/all-ancestor/rollback protection.
func OpenPrivateCustodyRecord(ctx context.Context, inputs CustodyFileInputs, outputPath, directory string, create bool) (*CustodyRecord, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, ErrCustodyRecordState
	}
	if _, _, err := custodyRecordCandidate(inputs, outputPath); err != nil {
		return nil, ErrCustodyRecordState
	}
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return nil, ErrCustodyRecordStorage
	}
	if filepath.Dir(outputPath) == directory {
		return nil, ErrCustodyRecordStorage
	}
	if create && checkCustodyNewOutput(outputPath) != nil {
		return nil, ErrCustodyRecordState
	}
	r, _, err := custodyPrivateRoot(filepath.Join(directory, custodyReservationFile))
	if err != nil {
		return nil, ErrCustodyRecordStorage
	}
	ok := false
	var lease *os.File
	defer func() {
		if !ok {
			if lease != nil {
				_ = lease.Close()
			}
			_ = r.Close()
		}
	}()
	info, err := r.Stat(".")
	if err != nil {
		return nil, ErrCustodyRecordStorage
	}
	if create {
		dir, openErr := r.Open(".")
		if openErr != nil {
			return nil, ErrCustodyRecordStorage
		}
		entries, readErr := dir.ReadDir(1)
		closeErr := dir.Close()
		if readErr != nil && readErr != io.EOF || closeErr != nil || len(entries) != 0 {
			return nil, ErrCustodyRecordStorage
		}
	}
	flags := os.O_RDWR | unix.O_NOFOLLOW | unix.O_NONBLOCK
	if create {
		flags |= os.O_CREATE | os.O_EXCL
	}
	lease, err = r.OpenFile(custodyRecordLease, flags, 0600)
	if err != nil {
		return nil, ErrCustodyRecordStorage
	}
	if _, err = custodyFileInfo(lease, 1, true); err != nil || unix.Flock(int(lease.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil {
		return nil, ErrCustodyRecordStorage
	}
	if create && lease.Sync() != nil {
		return nil, ErrCustodyRecordStorage
	}
	store := &privateCustodyRecordStore{root: r, lease: lease, directory: directory, identity: info}
	if store.verify() != nil {
		return nil, ErrCustodyRecordStorage
	}
	record, err := NewCustodyRecord(ctx, inputs, outputPath, store, create)
	if err != nil {
		return nil, err
	}
	ok = true
	return record, nil
}
