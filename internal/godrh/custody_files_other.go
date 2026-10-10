//go:build !go1.25 || (!linux && !darwin)

package godrh

// No unverified disk/ACL fallback. Pure request construction remains separate.
func readCustodyPrivateSnapshot(string, int64, [32]byte) ([]byte, error) {
	return nil, ErrCustodyWorkflow
}

func writeCustodyPrivateFile(string, []byte) error { return ErrCustodyWorkflow }
