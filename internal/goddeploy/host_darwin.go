//go:build go1.25 && darwin

package goddeploy

import "golang.org/x/sys/unix"

func hostDiskUnit(stat *unix.Statfs_t) (uint64, error) {
	if stat.Bsize == 0 {
		return 0, ErrHost
	}
	return uint64(stat.Bsize), nil
}
