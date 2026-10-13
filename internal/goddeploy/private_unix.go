//go:build go1.25 && (linux || darwin)

package goddeploy

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func privateInfo(info os.FileInfo) bool {
	meta, ok := info.Sys().(*syscall.Stat_t)
	return ok && meta.Uid == uint32(os.Geteuid()) && info.Mode().Perm()&0077 == 0 &&
		(info.IsDir() || info.Mode().IsRegular() && meta.Nlink == 1)
}

func openPrivateRegular(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
}
