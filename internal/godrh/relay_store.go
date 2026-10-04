package godrh

import (
	"os"
	"path/filepath"
	"runtime"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const relayDBName = "god_readonly_relay"

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
	if runtime.GOOS == "windows" || !filepath.IsAbs(directory) || filepath.Clean(directory) == string(filepath.Separator) {
		return nil, ErrRelayStorage
	}
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return nil, ErrRelayStorage
	}
	entries, err := os.ReadDir(directory)
	if err != nil || create && len(entries) != 0 || !create && (len(entries) != 1 || entries[0].Name() != relayDBName+dbm.DBFileSuffix) {
		return nil, ErrRelayStorage
	}
	if !create {
		path := filepath.Join(directory, relayDBName+dbm.DBFileSuffix)
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
	db, err := dbm.NewGoLevelDBWithOpts(relayDBName, directory, &opt.Options{ErrorIfMissing: !create, ErrorIfExist: create})
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
	if create && os.Chmod(filepath.Join(directory, relayDBName+dbm.DBFileSuffix), 0700) != nil {
		return nil, ErrRelayStorage
	}
	j, err := NewRelayJournal(c, &privateRelayStore{db}, create)
	if err != nil {
		return nil, err
	}
	ok = true
	return j, nil
}
