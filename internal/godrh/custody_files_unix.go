//go:build go1.25 && (linux || darwin)

package godrh

import (
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

func custodyDirectoryInfo(info os.FileInfo) bool {
	if info == nil || info.Mode() != os.ModeDir|0700 {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}

// Caller-owned trusted private leaf storage, not an all-ancestor audit or
// protection from its owner. os.Root anchors relative operations after open.
func custodyPrivateRoot(filePath string) (*os.Root, string, error) {
	if len(filePath) == 0 || len(filePath) > 4096 || !filepath.IsAbs(filePath) || filepath.Clean(filePath) != filePath {
		return nil, "", ErrCustodyWorkflow
	}
	directory, leaf := filepath.Dir(filePath), filepath.Base(filePath)
	if directory == string(filepath.Separator) || leaf == "." || leaf == ".." {
		return nil, "", ErrCustodyWorkflow
	}
	before, err := os.Lstat(directory)
	if err != nil || !custodyDirectoryInfo(before) {
		return nil, "", ErrCustodyWorkflow
	}
	r, err := os.OpenRoot(directory)
	if err != nil {
		return nil, "", ErrCustodyWorkflow
	}
	opened, err := r.Stat(".")
	if err != nil || !custodyDirectoryInfo(opened) || !os.SameFile(before, opened) {
		_ = r.Close()
		return nil, "", ErrCustodyWorkflow
	}
	return r, leaf, nil
}

func custodyFileInfo(f *os.File, limit int64, empty bool) (os.FileInfo, error) {
	info, err := f.Stat()
	var stat unix.Stat_t
	if err != nil || info.Mode() != 0600 || unix.Fstat(int(f.Fd()), &stat) != nil || stat.Uid != uint32(os.Geteuid()) || stat.Nlink != 1 ||
		info.Size() > limit || empty && info.Size() != 0 || !empty && info.Size() <= 0 {
		return nil, ErrCustodyWorkflow
	}
	return info, nil
}

func readCustodyPrivateSnapshot(filePath string, limit int64, expected [32]byte) ([]byte, error) {
	if expected == [32]byte{} || limit <= 0 || limit > MaxCustodyRequestBytes {
		return nil, ErrCustodyWorkflow
	}
	r, leaf, err := custodyPrivateRoot(filePath)
	if err != nil {
		return nil, ErrCustodyWorkflow
	}
	defer r.Close()
	f, err := r.OpenFile(leaf, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, ErrCustodyWorkflow
	}
	defer f.Close()
	before, err := custodyFileInfo(f, limit, false)
	if err != nil {
		return nil, ErrCustodyWorkflow
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil || int64(len(raw)) != before.Size() || sha256.Sum256(raw) != expected {
		return nil, ErrCustodyWorkflow
	}
	after, err := custodyFileInfo(f, limit, false)
	if err != nil || after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return nil, ErrCustodyWorkflow
	}
	listed, err := r.Lstat(leaf)
	if err != nil || listed.Mode() != 0600 || !os.SameFile(after, listed) {
		return nil, ErrCustodyWorkflow
	}
	return raw, nil
}

func writeCustodyPrivateFile(filePath string, raw []byte) error {
	if len(raw) == 0 || len(raw) > MaxCustodyPacketBytes {
		return ErrCustodyWorkflow
	}
	r, leaf, err := custodyPrivateRoot(filePath)
	if err != nil {
		return ErrCustodyWorkflow
	}
	defer r.Close()
	f, err := r.OpenFile(leaf, os.O_WRONLY|os.O_CREATE|os.O_EXCL|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0600)
	if err != nil {
		return ErrCustodyWorkflow
	}
	defer f.Close()
	if _, err := custodyFileInfo(f, MaxCustodyPacketBytes, true); err != nil {
		return ErrCustodyWorkflow
	}
	n, err := f.Write(raw)
	if err != nil || n != len(raw) || f.Sync() != nil {
		return ErrCustodyWorkflow
	}
	info, err := custodyFileInfo(f, MaxCustodyPacketBytes, false)
	if err != nil || info.Size() != int64(len(raw)) {
		return ErrCustodyWorkflow
	}
	listed, err := r.Lstat(leaf)
	if err != nil || listed.Mode() != 0600 || !os.SameFile(info, listed) || f.Close() != nil {
		return ErrCustodyWorkflow
	}
	dir, err := r.Open(".")
	if err != nil {
		return ErrCustodyWorkflow
	}
	defer dir.Close()
	if dir.Sync() != nil || dir.Close() != nil {
		return ErrCustodyWorkflow
	}
	return nil
}
