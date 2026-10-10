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

const custodyAttemptLease = ".god-simulation-attempt-lease"
const custodyAttemptReservation = "simulation-reservation.json"
const custodyAttemptDispatch = "simulation-dispatch-unknown.json"

// Two immutable exclusive stages, not an atomic replacement database or a
// production nonce allocator. Partial writes fail closed without cleanup.
type privateCustodyAttemptStore struct {
	root      *os.Root
	lease     *os.File
	directory string
	identity  os.FileInfo
	loaded    *custodyAttemptState
	closed    bool
}

func (s *privateCustodyAttemptStore) verify() error {
	if s.closed || s.root == nil || s.lease == nil {
		return ErrCustodyAttemptStorage
	}
	current, err := os.Lstat(s.directory)
	if err != nil || !custodyDirectoryInfo(current) || !os.SameFile(s.identity, current) {
		return ErrCustodyAttemptStorage
	}
	info, err := custodyFileInfo(s.lease, 1, true)
	if err != nil {
		return ErrCustodyAttemptStorage
	}
	listed, err := s.root.Lstat(custodyAttemptLease)
	if err != nil || listed.Mode() != 0600 || !os.SameFile(info, listed) {
		return ErrCustodyAttemptStorage
	}
	dir, err := s.root.Open(".")
	if err != nil {
		return ErrCustodyAttemptStorage
	}
	entries, readErr := dir.ReadDir(4)
	closeErr := dir.Close()
	if (readErr != nil && readErr != io.EOF) || closeErr != nil || len(entries) > 3 {
		return ErrCustodyAttemptStorage
	}
	for _, e := range entries {
		if e.Name() != custodyAttemptLease && e.Name() != custodyAttemptReservation && e.Name() != custodyAttemptDispatch {
			return ErrCustodyAttemptStorage
		}
	}
	return nil
}

func (s *privateCustodyAttemptStore) read(name string) ([]byte, error) {
	f, err := s.root.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, ErrCustodyAttemptStorage
	}
	defer f.Close()
	before, err := custodyFileInfo(f, MaxCustodyAttemptBytes, false)
	if err != nil {
		return nil, ErrCustodyAttemptStorage
	}
	raw, err := io.ReadAll(io.LimitReader(f, MaxCustodyAttemptBytes+1))
	if err != nil || int64(len(raw)) != before.Size() {
		return nil, ErrCustodyAttemptStorage
	}
	after, err := custodyFileInfo(f, MaxCustodyAttemptBytes, false)
	if err != nil || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return nil, ErrCustodyAttemptStorage
	}
	listed, err := s.root.Lstat(name)
	if err != nil || listed.Mode() != 0600 || !os.SameFile(after, listed) {
		return nil, ErrCustodyAttemptStorage
	}
	return raw, nil
}

func (s *privateCustodyAttemptStore) Load() ([]byte, error) {
	if s.verify() != nil {
		return nil, ErrCustodyAttemptStorage
	}
	_, err := s.root.Lstat(custodyAttemptReservation)
	if os.IsNotExist(err) {
		if _, err := s.root.Lstat(custodyAttemptDispatch); !os.IsNotExist(err) {
			return nil, ErrCustodyAttemptStorage
		}
		return nil, nil
	}
	if err != nil {
		return nil, ErrCustodyAttemptStorage
	}
	raw, err := s.read(custodyAttemptReservation)
	if err != nil {
		return nil, ErrCustodyAttemptStorage
	}
	state, err := decodeCustodyAttempt(raw)
	if err != nil || state.Stage != CustodyAttemptReserved {
		return nil, ErrCustodyAttemptStorage
	}
	_, err = s.root.Lstat(custodyAttemptDispatch)
	if !os.IsNotExist(err) {
		if err != nil {
			return nil, ErrCustodyAttemptStorage
		}
		nextRaw, err := s.read(custodyAttemptDispatch)
		if err != nil {
			return nil, ErrCustodyAttemptStorage
		}
		next, err := decodeCustodyAttempt(nextRaw)
		if err != nil || next.Stage != CustodyAttemptUnknown || !sameCustodyAttempt(state, next) {
			return nil, ErrCustodyAttemptStorage
		}
		state, raw = next, nextRaw
	}
	s.loaded = &state
	return bytes.Clone(raw), nil
}

func (s *privateCustodyAttemptStore) Save(raw []byte) error {
	if s.verify() != nil {
		return ErrCustodyAttemptStorage
	}
	next, err := decodeCustodyAttempt(bytes.Clone(raw))
	if err != nil {
		return ErrCustodyAttemptStorage
	}
	name := custodyAttemptReservation
	if s.loaded == nil {
		if next.Stage != CustodyAttemptReserved {
			return ErrCustodyAttemptStorage
		}
	} else {
		if s.loaded.Stage != CustodyAttemptReserved || next.Stage != CustodyAttemptUnknown || !sameCustodyAttempt(*s.loaded, next) {
			return ErrCustodyAttemptStorage
		}
		name = custodyAttemptDispatch
	}
	if writeCustodyPrivateFile(filepath.Join(s.directory, name), bytes.Clone(raw)) != nil || s.verify() != nil {
		return ErrCustodyAttemptStorage
	}
	retained, err := s.read(name)
	if err != nil || !bytes.Equal(raw, retained) {
		return ErrCustodyAttemptStorage
	}
	s.loaded = &next
	return nil
}

func (s *privateCustodyAttemptStore) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	a, b := s.lease.Close(), s.root.Close()
	if a != nil || b != nil {
		return ErrCustodyAttemptStorage
	}
	return nil
}

// Existing empty owner-controlled 0700 directory. Input files must live in
// separate private directories. The retained lease fences cooperating users
// of this exact slot only, not independent stores, old tools or remote hosts.
func OpenPrivateCustodyAttempt(ctx context.Context, inputs CustodyTransactionInputs, directory string, create bool, resumePin ...[32]byte) (*CustodyAttempt, error) {
	if ctx == nil || ctx.Err() != nil || !custodyAttemptResumeValid(create, resumePin) {
		return nil, ErrCustodyAttemptState
	}
	if _, _, err := custodyAttemptCandidate(inputs); err != nil {
		return nil, ErrCustodyAttemptState
	}
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return nil, ErrCustodyAttemptStorage
	}
	for _, path := range []string{inputs.CallInputs.ConfigurationPath, inputs.CallInputs.RequestPath, inputs.PlanPath, inputs.TransactionPath} {
		if filepath.Dir(path) == directory {
			return nil, ErrCustodyAttemptStorage
		}
	}
	r, _, err := custodyPrivateRoot(filepath.Join(directory, custodyAttemptReservation))
	if err != nil {
		return nil, ErrCustodyAttemptStorage
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
		return nil, ErrCustodyAttemptStorage
	}
	if create {
		dir, err := r.Open(".")
		if err != nil {
			return nil, ErrCustodyAttemptStorage
		}
		entries, readErr := dir.ReadDir(1)
		closeErr := dir.Close()
		if (readErr != nil && readErr != io.EOF) || closeErr != nil || len(entries) != 0 {
			return nil, ErrCustodyAttemptStorage
		}
	}
	flags := os.O_RDWR | unix.O_NOFOLLOW | unix.O_NONBLOCK
	if create {
		flags |= os.O_CREATE | os.O_EXCL
	}
	lease, err = r.OpenFile(custodyAttemptLease, flags, 0600)
	if err != nil {
		return nil, ErrCustodyAttemptStorage
	}
	if _, err = custodyFileInfo(lease, 1, true); err != nil || unix.Flock(int(lease.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil {
		return nil, ErrCustodyAttemptStorage
	}
	if create && lease.Sync() != nil {
		return nil, ErrCustodyAttemptStorage
	}
	store := &privateCustodyAttemptStore{root: r, lease: lease, directory: directory, identity: identity}
	if store.verify() != nil {
		return nil, ErrCustodyAttemptStorage
	}
	a, err := NewCustodyAttempt(ctx, inputs, store, create, resumePin...)
	if err != nil {
		return nil, err
	}
	ok = true
	return a, nil
}
