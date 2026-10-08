//go:build linux || darwin

package godhistory

import (
	"os"
	"syscall"
)

func owner(info os.FileInfo, directory bool) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid()) && info.Mode()&os.ModeSymlink == 0 && (directory && info.IsDir() && info.Mode().Perm() == 0700 || !directory && info.Mode().IsRegular() && info.Mode().Perm() == 0600 && stat.Nlink == 1)
}
func noFollow() int { return syscall.O_NOFOLLOW }
