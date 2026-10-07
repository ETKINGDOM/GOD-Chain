//go:build go1.25 && (linux || darwin)

package goddeploy

import (
	"os"

	"golang.org/x/sys/unix"
)

func readHostMetrics(dir *os.File) (hostMetrics, bool, error) {
	var disk unix.Statfs_t
	var limits unix.Rlimit
	if unix.Fstatfs(int(dir.Fd()), &disk) != nil || unix.Getrlimit(unix.RLIMIT_NOFILE, &limits) != nil {
		return hostMetrics{}, false, ErrHost
	}
	unit, err := hostDiskUnit(&disk)
	if err != nil {
		return hostMetrics{}, false, ErrHost
	}
	available, err := hostCapacity(disk.Bavail, unit)
	if err != nil {
		return hostMetrics{}, false, ErrHost
	}
	return hostMetrics{available, limits.Cur}, os.Geteuid() > 0, nil
}
