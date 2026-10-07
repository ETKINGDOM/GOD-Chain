//go:build go1.25 && linux

package goddeploy

import "golang.org/x/sys/unix"

func hostDiskUnit(stat *unix.Statfs_t) (uint64, error) {
	return hostLinuxUnit(stat.Bsize, stat.Frsize)
}
