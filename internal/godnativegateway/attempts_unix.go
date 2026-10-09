//go:build linux || darwin

package godnativegateway

import (
	"os"
	"syscall"
)

func attemptOwner(info os.FileInfo, directory bool) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid()) && info.Mode()&os.ModeSymlink == 0 && (directory && info.IsDir() && info.Mode().Perm() == 0700 || !directory && info.Mode().IsRegular() && info.Mode().Perm() == 0600 && stat.Nlink == 1)
}
func attemptNoFollow() int { return syscall.O_NOFOLLOW }
