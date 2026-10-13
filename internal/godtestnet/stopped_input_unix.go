//go:build go1.25 && (linux || darwin)

package godtestnet

import (
	"crypto/sha256"
	"io"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func stoppedPrivateFile(info os.FileInfo, limit int64) bool {
	meta, ok := info.Sys().(*syscall.Stat_t)
	return ok && info.Mode() == 0600 && info.Size() > 0 && info.Size() <= limit &&
		meta.Uid == uint32(os.Geteuid()) && meta.Nlink == 1
}

func readStoppedFile(root *os.Root, name string, limit int64) (stoppedInput, error) {
	first, err := root.Lstat(name)
	if err != nil || !stoppedPrivateFile(first, limit) {
		return stoppedInput{}, ErrUpgradeCheck
	}
	f, err := root.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return stoppedInput{}, ErrUpgradeCheck
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !stoppedPrivateFile(opened, limit) || !sameStoppedFile(first, opened) {
		return stoppedInput{}, ErrUpgradeCheck
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	defer clear(raw)
	last, lastErr := f.Stat()
	listed, listErr := root.Lstat(name)
	if err != nil || lastErr != nil || listErr != nil || int64(len(raw)) != first.Size() ||
		!stoppedPrivateFile(last, limit) || !stoppedPrivateFile(listed, limit) ||
		!sameStoppedFile(first, last) || !sameStoppedFile(first, listed) || f.Close() != nil {
		return stoppedInput{}, ErrUpgradeCheck
	}
	return stoppedInput{listed, sha256.Sum256(raw)}, nil
}
