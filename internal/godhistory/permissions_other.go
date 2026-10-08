//go:build !linux && !darwin

package godhistory

import "os"

// Owner-only ACL behavior is not accepted on other target runtimes.
func owner(os.FileInfo, bool) bool { return false }
func noFollow() int                { return 0 }
