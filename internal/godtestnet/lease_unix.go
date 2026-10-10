//go:build go1.25 && (linux || darwin)

package godtestnet

import (
	"os"

	"golang.org/x/sys/unix"
)

const nodeLeaseFile = ".god-node-lease"

// Hold one advisory lease for this exact retained home through engine shutdown
// and application close. Never truncate, remove or replace the lease file: doing
// so could let a second cooperating process lock a different inode. This is not
// remote signer fencing, protection from the host owner or older executables.
func acquireNodeLease(root *os.Root) (*os.File, error) {
	f, err := root.OpenFile(nodeLeaseFile, os.O_RDWR|os.O_CREATE|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil {
		return nil, ErrPrivate
	}
	ok := false
	defer func() {
		if !ok {
			_ = f.Close()
		}
	}()
	info, err := f.Stat()
	var meta unix.Stat_t
	if err != nil || info.Mode() != 0600 || info.Size() != 0 || unix.Fstat(int(f.Fd()), &meta) != nil ||
		meta.Uid != uint32(os.Geteuid()) || meta.Nlink != 1 || unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) != nil {
		return nil, ErrPrivate
	}
	listed, err := root.Lstat(nodeLeaseFile)
	if err != nil || listed.Mode() != 0600 || !os.SameFile(info, listed) {
		return nil, ErrPrivate
	}
	ok = true
	return f, nil
}
