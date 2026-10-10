//go:build go1.25 && (linux || darwin)

package godrh

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const custodyNonceLease = ".god-shared-simulation-nonce-lease"
const custodyNonceMaxSnapshots = 1 + 3*MaxCustodyNonceEntries

func custodyNonceSnapshotName(sequence uint32) string {
	return fmt.Sprintf("simulation-state-%03d.json", sequence)
}

// Bounded append-only, exclusive immutable snapshots with a hash-linked
// transition chain. Not a crash-atomic replacement database or distributed lock.
type privateCustodyNonceStore struct {
	root      *os.Root
	lease     *os.File
	directory string
	identity  os.FileInfo
	closed    bool
}

func (s *privateCustodyNonceStore) inventory() (int, error) {
	if s.closed || s.root == nil || s.lease == nil {
		return 0, ErrCustodyNonceStorage
	}
	current, err := os.Lstat(s.directory)
	if err != nil || !custodyDirectoryInfo(current) || !os.SameFile(s.identity, current) {
		return 0, ErrCustodyNonceStorage
	}
	info, err := custodyFileInfo(s.lease, 1, true)
	if err != nil {
		return 0, ErrCustodyNonceStorage
	}
	listed, err := s.root.Lstat(custodyNonceLease)
	if err != nil || listed.Mode() != 0600 || !os.SameFile(info, listed) {
		return 0, ErrCustodyNonceStorage
	}
	dir, err := s.root.Open(".")
	if err != nil {
		return 0, ErrCustodyNonceStorage
	}
	entries, readErr := dir.ReadDir(custodyNonceMaxSnapshots + 2)
	closeErr := dir.Close()
	if (readErr != nil && readErr != io.EOF) || closeErr != nil || len(entries) > custodyNonceMaxSnapshots+1 {
		return 0, ErrCustodyNonceStorage
	}
	seen := map[string]bool{}
	for _, e := range entries {
		seen[e.Name()] = true
	}
	if !seen[custodyNonceLease] {
		return 0, ErrCustodyNonceStorage
	}
	count := len(entries) - 1
	for n := 1; n <= count; n++ {
		if !seen[custodyNonceSnapshotName(uint32(n))] {
			return 0, ErrCustodyNonceStorage
		}
	}
	return count, nil
}
func (s *privateCustodyNonceStore) read(name string) ([]byte, error) {
	f, err := s.root.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, ErrCustodyNonceStorage
	}
	defer f.Close()
	before, err := custodyFileInfo(f, MaxCustodyNonceBytes, false)
	if err != nil {
		return nil, ErrCustodyNonceStorage
	}
	raw, err := io.ReadAll(io.LimitReader(f, MaxCustodyNonceBytes+1))
	if err != nil || int64(len(raw)) != before.Size() {
		return nil, ErrCustodyNonceStorage
	}
	after, err := custodyFileInfo(f, MaxCustodyNonceBytes, false)
	if err != nil || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return nil, ErrCustodyNonceStorage
	}
	listed, err := s.root.Lstat(name)
	if err != nil || listed.Mode() != 0600 || !os.SameFile(after, listed) || f.Close() != nil {
		return nil, ErrCustodyNonceStorage
	}
	return raw, nil
}
func (s *privateCustodyNonceStore) Load() ([]byte, error) {
	count, err := s.inventory()
	if err != nil {
		return nil, ErrCustodyNonceStorage
	}
	var previous custodyNonceState
	var latest []byte
	for n := 1; n <= count; n++ {
		raw, err := s.read(custodyNonceSnapshotName(uint32(n)))
		if err != nil {
			return nil, ErrCustodyNonceStorage
		}
		next, err := decodeCustodyNonce(raw)
		if err != nil || next.Sequence != uint32(n) || n > 1 && !custodyNonceTransition(previous, next) {
			return nil, ErrCustodyNonceStorage
		}
		previous, latest = next, raw
	}
	if after, err := s.inventory(); err != nil || after != count {
		return nil, ErrCustodyNonceStorage
	}
	return bytes.Clone(latest), nil
}
func (s *privateCustodyNonceStore) Save(raw []byte) error {
	next, err := decodeCustodyNonce(raw)
	if err != nil {
		return ErrCustodyNonceStorage
	}
	oldRaw, err := s.Load()
	if err != nil {
		return ErrCustodyNonceStorage
	}
	if len(oldRaw) == 0 {
		if next.Sequence != 1 {
			return ErrCustodyNonceStorage
		}
	} else {
		old, err := decodeCustodyNonce(oldRaw)
		if err != nil || !custodyNonceTransition(old, next) {
			return ErrCustodyNonceStorage
		}
	}
	name := custodyNonceSnapshotName(next.Sequence)
	f, err := s.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil {
		return ErrCustodyNonceStorage
	}
	defer f.Close()
	if _, err := custodyFileInfo(f, MaxCustodyNonceBytes, true); err != nil {
		return ErrCustodyNonceStorage
	}
	n, err := f.Write(bytes.Clone(raw))
	if err != nil || n != len(raw) || f.Sync() != nil {
		return ErrCustodyNonceStorage
	}
	info, err := custodyFileInfo(f, MaxCustodyNonceBytes, false)
	if err != nil || info.Size() != int64(len(raw)) {
		return ErrCustodyNonceStorage
	}
	listed, err := s.root.Lstat(name)
	if err != nil || listed.Mode() != 0600 || !os.SameFile(info, listed) || f.Close() != nil {
		return ErrCustodyNonceStorage
	}
	dir, err := s.root.Open(".")
	if err != nil {
		return ErrCustodyNonceStorage
	}
	defer dir.Close()
	if dir.Sync() != nil || dir.Close() != nil {
		return ErrCustodyNonceStorage
	}
	retained, err := s.Load()
	if err != nil || !bytes.Equal(raw, retained) {
		return ErrCustodyNonceStorage
	}
	return nil
}
func (s *privateCustodyNonceStore) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	a, b := s.lease.Close(), s.root.Close()
	if a != nil || b != nil {
		return ErrCustodyNonceStorage
	}
	return nil
}

// All cooperating requests for this sender/source must use THIS exact book.
// Other directories/tools/hosts are not fenced. Never reset or repin a financial
// retry. Existing owner-only empty leaf on create; retained lease on reopen.
func OpenPrivateCustodyNonceBook(ctx context.Context, i CustodyNonceInputs, directory string, create bool, resumePin ...[32]byte) (*CustodyNonceBook, error) {
	if ctx == nil || ctx.Err() != nil || !custodyAttemptResumeValid(create, resumePin) {
		return nil, ErrCustodyNonceState
	}
	if _, _, _, err := loadCustodyNonceInputs(i); err != nil {
		return nil, ErrCustodyNonceState
	}
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory || filepath.Dir(i.ConfigurationPath) == directory || filepath.Dir(i.AccountPlanPath) == directory {
		return nil, ErrCustodyNonceStorage
	}
	r, _, err := custodyPrivateRoot(filepath.Join(directory, custodyNonceLease))
	if err != nil {
		return nil, ErrCustodyNonceStorage
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
	identity, err := r.Stat(".")
	if err != nil {
		return nil, ErrCustodyNonceStorage
	}
	if create {
		dir, err := r.Open(".")
		if err != nil {
			return nil, ErrCustodyNonceStorage
		}
		entries, readErr := dir.ReadDir(1)
		closeErr := dir.Close()
		if (readErr != nil && readErr != io.EOF) || closeErr != nil || len(entries) != 0 {
			return nil, ErrCustodyNonceStorage
		}
	}
	flags := os.O_RDWR | unix.O_NOFOLLOW | unix.O_NONBLOCK
	if create {
		flags |= os.O_CREATE | os.O_EXCL
	}
	lease, err = r.OpenFile(custodyNonceLease, flags, 0600)
	if err != nil {
		return nil, ErrCustodyNonceStorage
	}
	if _, err = custodyFileInfo(lease, 1, true); err != nil || unix.Flock(int(lease.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil {
		return nil, ErrCustodyNonceStorage
	}
	if create && lease.Sync() != nil {
		return nil, ErrCustodyNonceStorage
	}
	store := &privateCustodyNonceStore{root: r, lease: lease, directory: directory, identity: identity}
	book, err := NewCustodyNonceBook(ctx, i, store, create, resumePin...)
	if err != nil {
		return nil, err
	}
	book.directory = directory
	ok = true
	return book, nil
}
