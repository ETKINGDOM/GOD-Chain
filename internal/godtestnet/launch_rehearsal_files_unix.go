//go:build go1.25 && (linux || darwin)

package godtestnet

import (
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Launch inputs require exact owner-only files, not the looser legacy testnet
// permissions boundary. No symlink, hard-link, device, FIFO or owner fallback.
func openLaunchRoot(home string) (*os.Root, error) {
	if !filepath.IsAbs(home) || filepath.Clean(home) != home || home == string(filepath.Separator) {
		return nil, ErrLaunchRehearsal
	}
	r, err := openPrivate(home)
	if err != nil {
		return nil, ErrLaunchRehearsal
	}
	if !launchRootMatches(r, home) {
		_ = r.Close()
		return nil, ErrLaunchRehearsal
	}
	return r, nil
}

func launchRootMatches(r *os.Root, home string) bool {
	listed, err := os.Lstat(home)
	opened, e := r.Stat(".")
	if err != nil || e != nil || listed.Mode() != os.ModeDir|0700 || !os.SameFile(listed, opened) {
		return false
	}
	f, err := r.Open(".")
	if err != nil {
		return false
	}
	defer f.Close()
	actual, err := f.Stat()
	var meta unix.Stat_t
	return err == nil && os.SameFile(actual, opened) && unix.Fstat(int(f.Fd()), &meta) == nil && meta.Uid == uint32(os.Geteuid())
}

func readLaunchFile(r *os.Root, name string, limit int64) ([]byte, error) {
	f, err := r.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, ErrLaunchRehearsal
	}
	defer f.Close()
	before, err := f.Stat()
	var meta unix.Stat_t
	if err != nil || before.Mode() != 0600 || before.Size() <= 0 || before.Size() > limit ||
		unix.Fstat(int(f.Fd()), &meta) != nil || meta.Uid != uint32(os.Geteuid()) || meta.Nlink != 1 {
		return nil, ErrLaunchRehearsal
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	after, e := f.Stat()
	listed, l := r.Lstat(name)
	var current unix.Stat_t
	if err != nil || e != nil || l != nil || int64(len(raw)) != before.Size() || after.Mode() != 0600 || after.Size() != before.Size() ||
		!os.SameFile(before, after) || !os.SameFile(after, listed) || listed.Mode() != 0600 ||
		unix.Fstat(int(f.Fd()), &current) != nil || current.Uid != meta.Uid || current.Nlink != 1 || f.Close() != nil {
		clear(raw)
		return nil, ErrLaunchRehearsal
	}
	return raw, nil
}

func launchDataDirectory(r *os.Root, name string) error {
	if r.Mkdir(name, 0700) != nil {
		// Existing directories must pass the same check, never be chmod-repaired.
		if _, err := r.Lstat(name); err != nil {
			return ErrLaunchRehearsal
		}
	}
	i, err := r.Lstat(name)
	if err != nil || i.Mode() != os.ModeDir|0700 {
		return ErrLaunchRehearsal
	}
	f, err := r.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_DIRECTORY, 0)
	if err != nil {
		return ErrLaunchRehearsal
	}
	defer f.Close()
	actual, err := f.Stat()
	var meta unix.Stat_t
	if err != nil || !os.SameFile(i, actual) || unix.Fstat(int(f.Fd()), &meta) != nil || meta.Uid != uint32(os.Geteuid()) {
		return ErrLaunchRehearsal
	}
	return nil
}
