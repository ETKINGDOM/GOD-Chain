//go:build go1.25 && (linux || darwin)

package godnode

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/cometbft/cometbft/privval"
	"golang.org/x/sys/unix"
)

type diskSigningGuard struct {
	root      *os.Root
	lease     *os.File
	directory string
	base      string
}

// InitializeSigningGuard exclusively creates a witness for a never-used key.
// The directory must already exist privately, outside every node backup scope.
// Existing or nonzero progress is never adopted, overwritten or reset.
func InitializeSigningGuard(directory, chain string, genesis [32]byte, pv *privval.FilePV) error {
	return InitializeBoundSigningGuard(directory, chain, genesis, pv, func() error { return nil })
}

// InitializeBoundSigningGuard calls retainRequirement under the key lease,
// after validating fresh material but before creating the witness. The caller
// must durably require that witness before this returns success. A partial
// initialization must never allow an unguarded start; no files are repaired.
func InitializeBoundSigningGuard(directory, chain string, genesis [32]byte, pv *privval.FilePV, retainRequirement func() error) error {
	r, err := guardRecord(chain, genesis, pv)
	if err != nil || pv.LastSignState.Height != 0 || retainRequirement == nil {
		return ErrSigningGuard
	}
	s, err := openGuardStore(directory, r.Key, true)
	if err != nil {
		return err
	}
	defer s.close()
	if _, err := s.root.Lstat(s.base + ".json"); !os.IsNotExist(err) {
		return ErrSigningGuard
	}
	if retainRequirement() != nil {
		return ErrSigningGuard
	}
	return s.write(r)
}

// OpenSigningGuard only accepts the exact retained FilePV progress. Missing,
// stale, ahead, corrupt or uncertain material is refused without repair.
// Only initialization may create a lease file; reopening requires an existing
// retained lease, so a missing lease cannot establish a new exclusion scope.
func OpenSigningGuard(directory, chain string, genesis [32]byte, pv *privval.FilePV) (*GuardedSigner, error) {
	r, err := guardRecord(chain, genesis, pv)
	if err != nil {
		return nil, err
	}
	s, err := openGuardStore(directory, r.Key, false)
	if err != nil {
		return nil, err
	}
	if s.check(r) != nil {
		_ = s.close()
		return nil, ErrSigningGuard
	}
	return &GuardedSigner{pv: pv, record: r, store: s}, nil
}

func openGuardStore(directory string, key [32]byte, initialize bool) (*diskSigningGuard, error) {
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory || directory == string(filepath.Separator) {
		return nil, ErrSigningGuard
	}
	info, err := os.Lstat(directory)
	if err != nil || info.Mode() != os.ModeDir|0700 {
		return nil, ErrSigningGuard
	}
	r, err := os.OpenRoot(directory)
	if err != nil {
		return nil, ErrSigningGuard
	}
	s := &diskSigningGuard{root: r, directory: directory, base: hex.EncodeToString(key[:])}
	success := false
	defer func() {
		if !success {
			_ = s.close()
		}
	}()
	if s.privateRoot() != nil {
		return nil, ErrSigningGuard
	}
	flags := os.O_RDWR | unix.O_NOFOLLOW | unix.O_NONBLOCK
	if initialize {
		flags |= os.O_CREATE
	}
	f, err := r.OpenFile(s.base+".lock", flags, 0600)
	if err != nil {
		return nil, ErrSigningGuard
	}
	s.lease = f
	if checkGuardFile(r, s.base+".lock", f, 0) != nil || unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil {
		return nil, ErrSigningGuard
	}
	if _, err := r.Lstat(s.base + ".pending"); !os.IsNotExist(err) {
		return nil, ErrSigningGuard
	}
	success = true
	return s, nil
}

func (s *diskSigningGuard) privateRoot() error {
	return privateGuardRoot(s.root, s.directory)
}

func privateGuardRoot(root *os.Root, directory string) error {
	f, err := root.Open(".")
	if err != nil {
		return ErrSigningGuard
	}
	defer f.Close()
	info, err := f.Stat()
	listed, e := os.Lstat(directory)
	var meta unix.Stat_t
	if err != nil || e != nil || info.Mode() != os.ModeDir|0700 || !os.SameFile(info, listed) || unix.Fstat(int(f.Fd()), &meta) != nil || meta.Uid != uint32(os.Geteuid()) {
		return ErrSigningGuard
	}
	return nil
}

func checkGuardFile(root *os.Root, name string, f *os.File, size int64) error {
	info, err := f.Stat()
	listed, e := root.Lstat(name)
	var meta unix.Stat_t
	if err != nil || e != nil || info.Mode() != 0600 || info.Size() != size || !os.SameFile(info, listed) || unix.Fstat(int(f.Fd()), &meta) != nil || meta.Uid != uint32(os.Geteuid()) || meta.Nlink != 1 {
		return ErrSigningGuard
	}
	return nil
}

func (s *diskSigningGuard) check(r signingGuardRecord) error {
	if s.privateRoot() != nil || checkGuardFile(s.root, s.base+".lock", s.lease, 0) != nil {
		return ErrSigningGuard
	}
	if _, err := s.root.Lstat(s.base + ".pending"); !os.IsNotExist(err) {
		return ErrSigningGuard
	}
	want, _ := json.Marshal(r)
	f, err := s.root.OpenFile(s.base+".json", os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return ErrSigningGuard
	}
	defer f.Close()
	if checkGuardFile(s.root, s.base+".json", f, int64(len(want))) != nil {
		return ErrSigningGuard
	}
	raw, err := io.ReadAll(io.LimitReader(f, 2049))
	if err != nil || len(raw) > 2048 || !bytes.Equal(raw, want) {
		return ErrSigningGuard
	}
	return nil
}

func (s *diskSigningGuard) advance(old, next signingGuardRecord) error {
	if old.Key != next.Key || old.Chain != next.Chain || old.Genesis != next.Genesis || s.check(old) != nil {
		return ErrSigningGuard
	}
	return s.write(next)
}

func (s *diskSigningGuard) write(r signingGuardRecord) error {
	if s.privateRoot() != nil || checkGuardFile(s.root, s.base+".lock", s.lease, 0) != nil {
		return ErrSigningGuard
	}
	raw, err := json.Marshal(r)
	if err != nil || len(raw) > 2048 {
		return ErrSigningGuard
	}
	f, err := s.root.OpenFile(s.base+".pending", os.O_WRONLY|os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return ErrSigningGuard
	}
	n, err := f.Write(raw)
	if err == nil && n == len(raw) {
		err = f.Sync()
	} else {
		err = ErrSigningGuard
	}
	if checkGuardFile(s.root, s.base+".pending", f, int64(len(raw))) != nil {
		err = ErrSigningGuard
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil || s.root.Rename(s.base+".pending", s.base+".json") != nil {
		return ErrSigningGuard
	}
	d, err := s.root.Open(".")
	if err != nil {
		return ErrSigningGuard
	}
	err = d.Sync()
	closeErr = d.Close()
	if err != nil || closeErr != nil || s.check(r) != nil {
		return ErrSigningGuard
	}
	return nil
}

func (s *diskSigningGuard) close() error {
	var err error
	if s.lease != nil {
		err = s.lease.Close()
	}
	if s.root != nil {
		if e := s.root.Close(); e != nil {
			err = e
		}
	}
	return err
}
