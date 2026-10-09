//go:build !linux && !darwin

package godnativegateway

import "os"

// Other ownership/ACL paths remain unaccepted; compilation is not activation.
func attemptOwner(os.FileInfo, bool) bool { return false }
func attemptNoFollow() int                { return 0 }
