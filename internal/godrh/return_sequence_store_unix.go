//go:build go1.25 && (linux || darwin)

package godrh

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	dbm "github.com/cosmos/cosmos-db"
)

type privateNativeReturnSequenceStore struct {
	store                      *privateRelayStore
	directory                  string
	identity, databaseIdentity os.FileInfo
}

func (s *privateNativeReturnSequenceStore) check() error {
	i, err := os.Lstat(s.directory)
	if err != nil || !custodyDirectoryInfo(i) || !os.SameFile(i, s.identity) {
		return ErrNativeReturnSequenceStorage
	}
	d, err := os.Lstat(filepath.Join(s.directory, nativeReturnSequenceDBName+dbm.DBFileSuffix))
	if err != nil || !custodyDirectoryInfo(d) || !os.SameFile(d, s.databaseIdentity) {
		return ErrNativeReturnSequenceStorage
	}
	return nil
}
func (s *privateNativeReturnSequenceStore) Load() ([]byte, error) {
	if s.check() != nil {
		return nil, ErrNativeReturnSequenceStorage
	}
	raw, err := s.store.Load()
	if err != nil || len(raw) > MaxNativeReturnSequenceBytes || s.check() != nil {
		return nil, ErrNativeReturnSequenceStorage
	}
	return raw, nil
}
func (s *privateNativeReturnSequenceStore) Save(raw []byte) error {
	if len(raw) > MaxNativeReturnSequenceBytes || s.check() != nil || s.store.Save(raw) != nil || s.check() != nil {
		return ErrNativeReturnSequenceStorage
	}
	return nil
}
func (s *privateNativeReturnSequenceStore) Close() error { return s.store.Close() }

// One private, owner-only database with exclusive process lock and atomic Sync.
// No ancestor/hostile-owner audit or protection against a rollback of BOTH book
// and independently retained pin. Unknown entries cannot be reset or repaired.
// Does not read a key, outbox, chain account or provider, and cannot broadcast.
func OpenPrivateNativeReturnSequenceBook(ctx context.Context, c godbridge.Config, p NativeReturnSequencePlan, directory string, create bool, resumePin ...[32]byte) (*NativeReturnSequenceBook, error) {
	if ctx == nil || ctx.Err() != nil || !custodyAttemptResumeValid(create, resumePin) {
		return nil, ErrNativeReturnSequenceState
	}
	if _, err := nativeSequenceHeader(c, p); err != nil {
		return nil, err
	}
	if len(directory) > 4096 || !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return nil, ErrNativeReturnSequenceStorage
	}
	identity, err := os.Lstat(directory)
	if err != nil || !custodyDirectoryInfo(identity) {
		return nil, ErrNativeReturnSequenceStorage
	}
	store, err := openPrivateRelayStore(directory, create, nativeReturnSequenceDBName)
	if err != nil {
		return nil, ErrNativeReturnSequenceStorage
	}
	databaseIdentity, err := os.Lstat(filepath.Join(directory, nativeReturnSequenceDBName+dbm.DBFileSuffix))
	if err != nil || !custodyDirectoryInfo(databaseIdentity) {
		_ = store.Close()
		return nil, ErrNativeReturnSequenceStorage
	}
	owned := &privateNativeReturnSequenceStore{store, directory, identity, databaseIdentity}
	b, err := NewNativeReturnSequenceBook(ctx, c, p, owned, create, resumePin...)
	if err != nil {
		_ = owned.Close()
		return nil, err
	}
	return b, nil
}
