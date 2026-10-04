package godrh

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const relayDBName = "god_readonly_relay"
const taskProofDBName = "god_task_receipt_proof"

type privateRelayStore struct{ db dbm.DB }

func (s *privateRelayStore) Load() ([]byte, error) { return s.db.Get([]byte{1}) }
func (s *privateRelayStore) Save(raw []byte) error { return s.db.SetSync([]byte{1}, raw) }
func (s *privateRelayStore) Close() error          { return s.db.Close() }

// OpenPrivateRelayJournal uses the already pinned pure-Go database's exclusive
// process lock and synchronous atomic writes. The caller supplies an existing
// owner-only directory in ignored storage. No directory, key or state is reset.
// Windows disk use fails closed pending an owner-only ACL implementation.
func OpenPrivateRelayJournal(c Config, directory string, create bool) (*RelayJournal, error) {
	if _, err := relayBinding(c); err != nil {
		return nil, ErrRelayState
	}
	store, err := openPrivateRelayStore(directory, create, relayDBName)
	if err != nil {
		return nil, err
	}
	j, err := NewRelayJournal(c, store, create)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	return j, nil
}

// A dedicated private directory/database for ONE immutable task proof slot.
// Reuses the journal's existing POSIX directory/lock gate; Windows fails closed.
// No journal payload, signing state or task retry budget is stored here.
func OpenPrivateTaskReceiptProofStore(ctx context.Context, journal *RelayJournal, ticket uint64, directory string, create bool) (*TaskReceiptProofStore, error) {
	if journal == nil || ctx == nil {
		return nil, ErrTaskReceiptProofState
	}
	journal.mu.Lock()
	_, _, err := journal.taskReceiptProofPin(ctx, ticket)
	journal.mu.Unlock()
	if err != nil {
		return nil, ErrTaskReceiptProofState
	}
	store, err := openPrivateRelayStore(directory, create, taskProofDBName)
	if err != nil {
		return nil, ErrTaskReceiptProofStorage
	}
	p, err := NewTaskReceiptProofStore(ctx, journal, ticket, store, create)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	return p, nil
}

// Names are private constants, never user-provided path components. This
// checks the final directory and its existing entries, not every ancestor or
// hostile filesystem races. Caller supplies trusted ignored owner-only storage.
func openPrivateRelayStore(directory string, create bool, name string) (*privateRelayStore, error) {
	if name != relayDBName && name != taskProofDBName {
		return nil, ErrRelayStorage
	}
	if runtime.GOOS == "windows" || !filepath.IsAbs(directory) || filepath.Clean(directory) == string(filepath.Separator) {
		return nil, ErrRelayStorage
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return nil, ErrRelayStorage
	}
	entries, err := os.ReadDir(directory)
	if err != nil || create && len(entries) != 0 || !create && (len(entries) != 1 || entries[0].Name() != name+dbm.DBFileSuffix) {
		return nil, ErrRelayStorage
	}
	if !create {
		path := filepath.Join(directory, name+dbm.DBFileSuffix)
		stored, err := os.Lstat(path)
		if err != nil || !stored.IsDir() || stored.Mode()&os.ModeSymlink != 0 || stored.Mode().Perm()&0077 != 0 {
			return nil, ErrRelayStorage
		}
		files, err := os.ReadDir(path)
		if err != nil || len(files) == 0 {
			return nil, ErrRelayStorage
		}
		for _, file := range files {
			info, err := os.Lstat(filepath.Join(path, file.Name()))
			if err != nil || !info.Mode().IsRegular() {
				return nil, ErrRelayStorage
			}
		}
	}
	db, err := dbm.NewGoLevelDBWithOpts(name, directory, &opt.Options{ErrorIfMissing: !create, ErrorIfExist: create})
	if err != nil {
		return nil, ErrRelayStorage
	}
	ok := false
	defer func() {
		if !ok {
			_ = db.Close()
		}
	}()
	opened, err := os.Lstat(directory)
	if err != nil || !os.SameFile(info, opened) || opened.Mode().Perm()&0077 != 0 {
		return nil, ErrRelayStorage
	}
	if create && os.Chmod(filepath.Join(directory, name+dbm.DBFileSuffix), 0700) != nil {
		return nil, ErrRelayStorage
	}
	ok = true
	return &privateRelayStore{db}, nil
}
