//go:build go1.25 && (linux || darwin)

package godrh

import (
	"bytes"
	"golang.org/x/sys/unix"
	"os"
)

// Existing trusted owner-only leaf boundary; no overwrite, repair, cleanup or
// implicit directory creation. An interrupted artifact remains for review.
func writeLaunchAssemblyPrivateFile(path string, raw []byte) error {
	if len(raw) == 0 || len(raw) > MaxLaunchAssemblyBytes {
		return ErrLaunchAssembly
	}
	r, leaf, err := custodyPrivateRoot(path)
	if err != nil {
		return ErrLaunchAssembly
	}
	defer r.Close()
	f, err := r.OpenFile(leaf, os.O_WRONLY|os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil {
		return ErrLaunchAssembly
	}
	defer f.Close()
	if _, err := custodyFileInfo(f, MaxLaunchAssemblyBytes, true); err != nil {
		return ErrLaunchAssembly
	}
	if n, err := f.Write(bytes.Clone(raw)); err != nil || n != len(raw) || f.Sync() != nil {
		return ErrLaunchAssembly
	}
	info, err := custodyFileInfo(f, MaxLaunchAssemblyBytes, false)
	if err != nil || info.Size() != int64(len(raw)) {
		return ErrLaunchAssembly
	}
	listed, err := r.Lstat(leaf)
	if err != nil || !os.SameFile(info, listed) || listed.Mode() != 0600 || f.Close() != nil {
		return ErrLaunchAssembly
	}
	dir, err := r.Open(".")
	if err != nil {
		return ErrLaunchAssembly
	}
	defer dir.Close()
	if dir.Sync() != nil || dir.Close() != nil {
		return ErrLaunchAssembly
	}
	return nil
}
