//go:build go1.25 && (linux || darwin)

package godnode

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	cmtjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/privval"
	"golang.org/x/sys/unix"
)

// fileGuardProgress retains the original directory and immutable key inode.
// Progress may be atomically replaced by FilePV. Every read nevertheless checks
// the opened file against its current directory entry before and after reading.
type fileGuardProgress struct {
	root  *os.Root
	files SigningGuardFiles
	key   os.FileInfo
	pv    *privval.FilePV
	sync  func(*os.File) error
}

type fileBoundGuardStore struct {
	signingGuardStore
	progress *fileGuardProgress
}

// InitializeFileSigningGuard refuses unsafe or mismatched original files before
// retaining a requirement marker. It never adopts used progress or repairs files.
func InitializeFileSigningGuard(directory, chain string, genesis [32]byte, pv *privval.FilePV, files SigningGuardFiles, retainRequirement func() error) (result error) {
	if _, err := guardRecord(chain, genesis, pv); err != nil || pv.LastSignState.Height != 0 {
		return ErrSigningGuard
	}
	if !separateGuardFiles(directory, files.Home) {
		return ErrSigningGuard
	}
	p, err := openGuardProgress(files, pv)
	if err != nil {
		return ErrSigningGuard
	}
	defer func() {
		if p.root.Close() != nil {
			result = ErrSigningGuard
		}
	}()
	if retainRequirement == nil {
		return ErrSigningGuard
	}
	return InitializeBoundSigningGuard(directory, chain, genesis, pv, func() error {
		if p.check() != nil || retainRequirement() != nil || p.check() != nil {
			return ErrSigningGuard
		}
		return nil
	})
}

// OpenFileSigningGuard binds the original private files as well as the witness.
// It reads only on open/check. New signing progress must match and be file- and
// directory-synced before the witness advances or a signature is returned.
func OpenFileSigningGuard(directory, chain string, genesis [32]byte, pv *privval.FilePV, files SigningGuardFiles) (*GuardedSigner, error) {
	if _, err := guardRecord(chain, genesis, pv); err != nil {
		return nil, ErrSigningGuard
	}
	if !separateGuardFiles(directory, files.Home) {
		return nil, ErrSigningGuard
	}
	p, err := openGuardProgress(files, pv)
	if err != nil {
		return nil, ErrSigningGuard
	}
	g, err := OpenSigningGuard(directory, chain, genesis, pv)
	if err != nil {
		_ = p.root.Close()
		return nil, ErrSigningGuard
	}
	g.store = &fileBoundGuardStore{g.store, p}
	if g.current() != nil {
		_ = g.Close()
		return nil, ErrSigningGuard
	}
	return g, nil
}

func separateGuardFiles(directory, home string) bool {
	return filepath.IsAbs(directory) && filepath.Clean(directory) == directory && directory != string(filepath.Separator) &&
		directory != home && !strings.HasPrefix(directory, home+string(filepath.Separator)) && !strings.HasPrefix(home, directory+string(filepath.Separator))
}

func guardFileName(name string) bool {
	if len(name) == 0 || len(name) > 128 || name == "." || name == ".." {
		return false
	}
	for _, b := range []byte(name) {
		if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '.' || b == '-' || b == '_') {
			return false
		}
	}
	return true
}

func openGuardProgress(files SigningGuardFiles, pv *privval.FilePV) (*fileGuardProgress, error) {
	if pv == nil || !filepath.IsAbs(files.Home) || filepath.Clean(files.Home) != files.Home || files.Home == string(filepath.Separator) ||
		!guardFileName(files.KeyFile) || !guardFileName(files.StateFile) || files.KeyFile == files.StateFile {
		return nil, ErrSigningGuard
	}
	info, err := os.Lstat(files.Home)
	if err != nil || info.Mode() != os.ModeDir|0700 {
		return nil, ErrSigningGuard
	}
	root, err := os.OpenRoot(files.Home)
	if err != nil {
		return nil, ErrSigningGuard
	}
	p := &fileGuardProgress{root: root, files: files, pv: pv, sync: (*os.File).Sync}
	if p.check() != nil {
		_ = root.Close()
		return nil, ErrSigningGuard
	}
	return p, nil
}

func (p *fileGuardProgress) check() error {
	if privateGuardRoot(p.root, p.files.Home) != nil {
		return ErrSigningGuard
	}
	key, err := p.match(p.files.KeyFile, p.pv.Key, 4096, false, p.key)
	if err != nil {
		return ErrSigningGuard
	}
	if p.key == nil {
		p.key = key
	}
	if _, err := p.match(p.files.StateFile, p.pv.LastSignState, 16<<10, false, nil); err != nil || privateGuardRoot(p.root, p.files.Home) != nil {
		return ErrSigningGuard
	}
	return nil
}

// Compare compacted bytes rather than permissively decoding JSON. This accepts
// existing compact/indented FilePV encodings but not duplicate/unknown/aliased
// keys, omitted required fields or alternate scalar/base64 representations.
func (p *fileGuardProgress) match(name string, value any, limit int64, retain bool, original os.FileInfo) (os.FileInfo, error) {
	want, err := cmtjson.Marshal(value)
	defer clear(want)
	if err != nil || int64(len(want)) > limit {
		return nil, ErrSigningGuard
	}
	flags := os.O_RDONLY | unix.O_NOFOLLOW | unix.O_NONBLOCK
	if retain {
		flags = os.O_RDWR | unix.O_NOFOLLOW | unix.O_NONBLOCK
	}
	f, err := p.root.OpenFile(name, flags, 0)
	if err != nil {
		return nil, ErrSigningGuard
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() <= 0 || info.Size() > limit || checkGuardFile(p.root, name, f, info.Size()) != nil || original != nil && !os.SameFile(original, info) {
		return nil, ErrSigningGuard
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	defer clear(raw)
	var compact bytes.Buffer
	defer func() { clear(compact.Bytes()) }()
	if err != nil || int64(len(raw)) != info.Size() || json.Compact(&compact, raw) != nil || !bytes.Equal(compact.Bytes(), want) ||
		checkGuardFile(p.root, name, f, info.Size()) != nil {
		return nil, ErrSigningGuard
	}
	if retain && (p.sync(f) != nil || checkGuardFile(p.root, name, f, info.Size()) != nil) {
		return nil, ErrSigningGuard
	}
	if f.Close() != nil {
		return nil, ErrSigningGuard
	}
	return info, nil
}

func (p *fileGuardProgress) retain() error {
	if p.check() != nil {
		return ErrSigningGuard
	}
	if _, err := p.match(p.files.StateFile, p.pv.LastSignState, 16<<10, true, nil); err != nil {
		return ErrSigningGuard
	}
	d, err := p.root.Open(".")
	if err != nil {
		return ErrSigningGuard
	}
	err = p.sync(d)
	closeErr := d.Close()
	if err != nil || closeErr != nil || p.check() != nil {
		return ErrSigningGuard
	}
	return nil
}

func (s *fileBoundGuardStore) check(r signingGuardRecord) error {
	if s.signingGuardStore.check(r) != nil || s.progress.check() != nil {
		return ErrSigningGuard
	}
	return nil
}

func (s *fileBoundGuardStore) advance(old, next signingGuardRecord) error {
	if s.progress.retain() != nil || s.signingGuardStore.advance(old, next) != nil {
		return ErrSigningGuard
	}
	return nil
}

func (s *fileBoundGuardStore) close() error {
	err := s.signingGuardStore.close()
	if s.progress.root.Close() != nil || err != nil {
		return ErrSigningGuard
	}
	return nil
}
