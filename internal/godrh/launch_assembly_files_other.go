//go:build !go1.25 || (!linux && !darwin)

package godrh

// No unreviewed private-file/ACL fallback.
func writeLaunchAssemblyPrivateFile(string, []byte) error { return ErrLaunchAssembly }
