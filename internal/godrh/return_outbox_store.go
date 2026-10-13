package godrh

import (
	"context"
	"os"
	"path/filepath"

	dbm "github.com/cosmos/cosmos-db"
)

// Dedicated private database and process lock. Refuses directory replacement
// or permission change on every read/write; no reset, rename or auto-repair.
// Uses the existing POSIX gate; Windows fails closed pending private ACLs.
type privateReturnOutboxStore struct {
	store            *privateRelayStore
	directory        string
	identity         os.FileInfo
	databaseIdentity os.FileInfo
}

func (s *privateReturnOutboxStore) check() error {
	i, err := os.Lstat(s.directory)
	if err != nil || !os.SameFile(s.identity, i) || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 || i.Mode().Perm()&0077 != 0 {
		return ErrReturnOutboxStorage
	}
	d, err := os.Lstat(filepath.Join(s.directory, returnOutboxDBName+dbm.DBFileSuffix))
	if err != nil || !os.SameFile(s.databaseIdentity, d) || !d.IsDir() || d.Mode()&os.ModeSymlink != 0 || d.Mode().Perm()&0077 != 0 {
		return ErrReturnOutboxStorage
	}
	return nil
}
func (s *privateReturnOutboxStore) Load() ([]byte, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	raw, err := s.store.Load()
	if err != nil || s.check() != nil {
		return nil, ErrReturnOutboxStorage
	}
	return raw, nil
}
func (s *privateReturnOutboxStore) Save(raw []byte) error {
	if s.check() != nil || s.store.Save(raw) != nil || s.check() != nil {
		return ErrReturnOutboxStorage
	}
	return nil
}
func (s *privateReturnOutboxStore) Close() error { return s.store.Close() }

// Opens only an existing trusted owner-only directory in private ignored
// storage. A caller independently retains the latest outbox pin on recovery.
// Opening never prepares a proposal, dispatches, signs or contacts a provider.
func OpenPrivateReturnSettlementOutbox(ctx context.Context, journal *RelayJournal, ticket uint64, directory string,
	create bool, resumePin ...[32]byte) (*ReturnSettlementOutbox, error) {
	if ctx == nil || journal == nil || !custodyAttemptResumeValid(create, resumePin) {
		return nil, ErrReturnOutboxState
	}
	journal.mu.Lock()
	_, kind, err := journal.taskReceiptProofPin(ctx, ticket)
	journal.mu.Unlock()
	if err != nil || kind != "paid" && kind != "cancelled" {
		return nil, ErrReturnOutboxState
	}
	identity, err := os.Lstat(directory)
	if err != nil {
		return nil, ErrReturnOutboxStorage
	}
	store, err := openPrivateRelayStore(directory, create, returnOutboxDBName)
	if err != nil {
		return nil, ErrReturnOutboxStorage
	}
	databaseIdentity, err := os.Lstat(filepath.Join(directory, returnOutboxDBName+dbm.DBFileSuffix))
	if err != nil {
		_ = store.Close()
		return nil, ErrReturnOutboxStorage
	}
	owned := &privateReturnOutboxStore{store: store, directory: directory, identity: identity, databaseIdentity: databaseIdentity}
	p, err := NewReturnSettlementOutbox(ctx, journal, ticket, owned, create, resumePin...)
	if err != nil {
		_ = owned.Close()
		return nil, err
	}
	return p, nil
}
