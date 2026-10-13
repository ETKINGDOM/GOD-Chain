//go:build go1.25 && (linux || darwin)

package godrh

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const creationJournalLease = ".god-custody-creation-simulation-lease"

var creationJournalNames = [3]string{"creation-prepared.json", "creation-dispatch-unknown.json", "creation-observation.json"}

type privateCreationJournal struct {
	root      *os.Root
	lease     *os.File
	directory string
	identity  os.FileInfo
	closed    bool
}

func (*privateCreationJournal) String() string {
	return "RH private creation simulation storage (redacted)"
}
func (s *privateCreationJournal) GoString() string           { return s.String() }
func (*privateCreationJournal) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

func (s *privateCreationJournal) inventory() (int, error) {
	if s == nil || s.closed || s.root == nil || s.lease == nil {
		return 0, ErrCustodyCreationJournal
	}
	current, err := os.Lstat(s.directory)
	if err != nil || !custodyDirectoryInfo(current) || !os.SameFile(s.identity, current) {
		return 0, ErrCustodyCreationJournal
	}
	info, err := custodyFileInfo(s.lease, 1, true)
	if err != nil {
		return 0, ErrCustodyCreationJournal
	}
	listed, err := s.root.Lstat(creationJournalLease)
	if err != nil || listed.Mode() != 0600 || !os.SameFile(info, listed) {
		return 0, ErrCustodyCreationJournal
	}
	f, err := s.root.Open(".")
	if err != nil {
		return 0, ErrCustodyCreationJournal
	}
	defer f.Close()
	entries, err := f.ReadDir(5)
	if err != nil && err != io.EOF || len(entries) < 1 || len(entries) > 4 {
		return 0, ErrCustodyCreationJournal
	}
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.Name()] = true
	}
	if !seen[creationJournalLease] {
		return 0, ErrCustodyCreationJournal
	}
	count := len(entries) - 1
	for i := 0; i < count; i++ {
		if !seen[creationJournalNames[i]] {
			return 0, ErrCustodyCreationJournal
		}
	}
	return count, nil
}
func (s *privateCreationJournal) read(name string) ([]byte, error) {
	f, err := s.root.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, ErrCustodyCreationJournal
	}
	defer f.Close()
	before, err := custodyFileInfo(f, maxCustodyCreationJournalBytes, false)
	if err != nil {
		return nil, ErrCustodyCreationJournal
	}
	raw, err := io.ReadAll(io.LimitReader(f, maxCustodyCreationJournalBytes+1))
	if err != nil || int64(len(raw)) != before.Size() {
		return nil, ErrCustodyCreationJournal
	}
	after, err := custodyFileInfo(f, maxCustodyCreationJournalBytes, false)
	if err != nil || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return nil, ErrCustodyCreationJournal
	}
	listed, err := s.root.Lstat(name)
	if err != nil || listed.Mode() != 0600 || !os.SameFile(after, listed) {
		return nil, ErrCustodyCreationJournal
	}
	return raw, nil
}
func (s *privateCreationJournal) Load() ([][]byte, error) {
	count, err := s.inventory()
	if err != nil {
		return nil, ErrCustodyCreationJournal
	}
	rows := make([][]byte, count)
	for i := range rows {
		rows[i], err = s.read(creationJournalNames[i])
		if err != nil {
			return nil, ErrCustodyCreationJournal
		}
	}
	second, err := s.inventory()
	if err != nil || second != count {
		return nil, ErrCustodyCreationJournal
	}
	return rows, nil
}
func (s *privateCreationJournal) Append(raw []byte) error {
	count, err := s.inventory()
	if err != nil || count >= 3 {
		return ErrCustodyCreationJournal
	}
	snapshot, err := creationSnapshot(raw)
	if err != nil || snapshot.Sequence != uint32(count+1) {
		return ErrCustodyCreationJournal
	}
	rows, err := s.Load()
	if err != nil || len(rows) != count {
		return ErrCustodyCreationJournal
	}
	if count > 0 {
		_, latest, err := creationJournalRows(rows, snapshot)
		if err != nil || snapshot.PreviousSHA256 != creationHexPin(sha256.Sum256(latest)) {
			return ErrCustodyCreationJournal
		}
	}
	f, err := s.root.OpenFile(creationJournalNames[count], os.O_WRONLY|os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil {
		return ErrCustodyCreationJournal
	}
	defer f.Close()
	if _, err := custodyFileInfo(f, maxCustodyCreationJournalBytes, true); err != nil {
		return ErrCustodyCreationJournal
	}
	n, err := f.Write(bytes.Clone(raw))
	if err != nil || n != len(raw) || f.Sync() != nil {
		return ErrCustodyCreationJournal
	}
	info, err := custodyFileInfo(f, maxCustodyCreationJournalBytes, false)
	if err != nil || info.Size() != int64(len(raw)) || f.Close() != nil {
		return ErrCustodyCreationJournal
	}
	listed, err := s.root.Lstat(creationJournalNames[count])
	if err != nil || !os.SameFile(info, listed) || listed.Mode() != 0600 {
		return ErrCustodyCreationJournal
	}
	dir, err := s.root.Open(".")
	if err != nil {
		return ErrCustodyCreationJournal
	}
	defer dir.Close()
	if dir.Sync() != nil || dir.Close() != nil {
		return ErrCustodyCreationJournal
	}
	retained, err := s.read(creationJournalNames[count])
	if err != nil || !bytes.Equal(raw, retained) {
		return ErrCustodyCreationJournal
	}
	return nil
}
func (s *privateCreationJournal) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	var first error
	if s.lease != nil {
		if unix.Flock(int(s.lease.Fd()), unix.LOCK_UN) != nil {
			first = ErrCustodyCreationJournal
		}
		if s.lease.Close() != nil {
			first = ErrCustodyCreationJournal
		}
	}
	if s.root != nil && s.root.Close() != nil {
		first = ErrCustodyCreationJournal
	}
	return first
}

// Existing private owner-only leaf directory. Create requires it to be empty;
// reopen never creates a missing lease or repairs incomplete history. Success
// grants lifetime ownership until Close, not distributed/global nonce control.
func OpenPrivateCustodyCreationJournal(directory string, create bool) (CustodyCreationJournalStorage, error) {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory || len(directory) > 4096 {
		return nil, ErrCustodyCreationJournal
	}
	before, err := os.Lstat(directory)
	if err != nil || !custodyDirectoryInfo(before) {
		return nil, ErrCustodyCreationJournal
	}
	r, err := os.OpenRoot(directory)
	if err != nil {
		return nil, ErrCustodyCreationJournal
	}
	s := &privateCreationJournal{root: r, directory: directory, identity: before}
	failed := true
	defer func() {
		if failed {
			_ = s.Close()
		}
	}()
	opened, err := r.Stat(".")
	if err != nil || !os.SameFile(before, opened) || !custodyDirectoryInfo(opened) {
		return nil, ErrCustodyCreationJournal
	}
	if create {
		f, err := r.Open(".")
		if err != nil {
			return nil, ErrCustodyCreationJournal
		}
		entries, err := f.ReadDir(1)
		_ = f.Close()
		if err != io.EOF || len(entries) != 0 {
			return nil, ErrCustodyCreationJournal
		}
	}
	flags := os.O_RDWR | unix.O_NOFOLLOW | unix.O_NONBLOCK
	if create {
		flags |= os.O_CREATE | os.O_EXCL
	}
	s.lease, err = r.OpenFile(creationJournalLease, flags, 0600)
	if err != nil {
		return nil, ErrCustodyCreationJournal
	}
	if _, err := custodyFileInfo(s.lease, 1, true); err != nil || unix.Flock(int(s.lease.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil {
		return nil, ErrCustodyCreationJournal
	}
	if create {
		if s.lease.Sync() != nil {
			return nil, ErrCustodyCreationJournal
		}
		f, err := r.Open(".")
		if err != nil {
			return nil, ErrCustodyCreationJournal
		}
		defer f.Close()
		if f.Sync() != nil {
			return nil, ErrCustodyCreationJournal
		}
	}
	if _, err := s.inventory(); err != nil {
		return nil, ErrCustodyCreationJournal
	}
	failed = false
	return s, nil
}
