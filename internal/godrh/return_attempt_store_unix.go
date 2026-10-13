//go:build go1.25 && (linux || darwin)

package godrh

import (
	"context"
	"os"
	"path/filepath"

	dbm "github.com/cosmos/cosmos-db"
)

// Existing private database/atomic Sync/lock boundary, with caller-owned exact
// mode-0700 leaf identities rechecked. Not an all-ancestor or hostile-owner audit.
type privateNativeReturnAttemptStore struct {
	store                      *privateRelayStore
	directory                  string
	identity, databaseIdentity os.FileInfo
}

func (s *privateNativeReturnAttemptStore) check() error {
	i, err := os.Lstat(s.directory)
	if err != nil || !custodyDirectoryInfo(i) || !os.SameFile(i, s.identity) {
		return ErrNativeReturnAttemptStorage
	}
	d, err := os.Lstat(filepath.Join(s.directory, nativeReturnAttemptDBName+dbm.DBFileSuffix))
	if err != nil || !custodyDirectoryInfo(d) || !os.SameFile(d, s.databaseIdentity) {
		return ErrNativeReturnAttemptStorage
	}
	return nil
}
func (s *privateNativeReturnAttemptStore) Load() ([]byte, error) {
	if s.check() != nil {
		return nil, ErrNativeReturnAttemptStorage
	}
	raw, err := s.store.Load()
	if err != nil || s.check() != nil {
		return nil, ErrNativeReturnAttemptStorage
	}
	return raw, nil
}
func (s *privateNativeReturnAttemptStore) Save(raw []byte) error {
	if s.check() != nil || s.store.Save(raw) != nil || s.check() != nil {
		return ErrNativeReturnAttemptStorage
	}
	return nil
}
func (s *privateNativeReturnAttemptStore) Close() error { return s.store.Close() }

// Private signed bearer storage; never a public route, auto-repair or startup
// sender. Only accepts an existing empty owned directory for creation, or the
// original database plus independently retained latest pin for recovery.
func OpenPrivateNativeReturnAttempt(ctx context.Context, outbox *ReturnSettlementOutbox, transaction NativeResolutionTransaction,
	directory string, create bool, resumePin ...[32]byte) (*NativeReturnAttempt, error) {
	if ctx == nil || ctx.Err() != nil || outbox == nil || outbox.journal == nil || !custodyAttemptResumeValid(create, resumePin) {
		return nil, ErrNativeReturnAttemptState
	}
	checked, err := ReviewNativeResolutionTransaction(transaction.binding, transaction.request, transaction.plan, transaction.raw)
	if err != nil {
		return nil, ErrNativeReturnAttemptState
	}
	outbox.mu.Lock()
	outbox.journal.mu.Lock()
	_, err = nativeReturnOutboxLink(ctx, outbox, checked)
	refuse := err != nil || create && outbox.state.Stage != ReturnOutboxUnknown
	outbox.journal.mu.Unlock()
	outbox.mu.Unlock()
	if refuse {
		return nil, ErrNativeReturnAttemptState
	}
	if len(directory) > 4096 || !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return nil, ErrNativeReturnAttemptStorage
	}
	identity, err := os.Lstat(directory)
	if err != nil || !custodyDirectoryInfo(identity) {
		return nil, ErrNativeReturnAttemptStorage
	}
	store, err := openPrivateRelayStore(directory, create, nativeReturnAttemptDBName)
	if err != nil {
		return nil, ErrNativeReturnAttemptStorage
	}
	databaseIdentity, err := os.Lstat(filepath.Join(directory, nativeReturnAttemptDBName+dbm.DBFileSuffix))
	if err != nil || !custodyDirectoryInfo(databaseIdentity) {
		_ = store.Close()
		return nil, ErrNativeReturnAttemptStorage
	}
	owned := &privateNativeReturnAttemptStore{store, directory, identity, databaseIdentity}
	a, err := NewNativeReturnAttempt(ctx, outbox, transaction, owned, create, resumePin...)
	if err != nil {
		_ = owned.Close()
		return nil, err
	}
	return a, nil
}
