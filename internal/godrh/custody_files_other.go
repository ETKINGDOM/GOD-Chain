//go:build !go1.25 || (!linux && !darwin)

package godrh

// No unverified disk/ACL fallback. Pure request construction remains separate.
func readCustodyPrivateSnapshot(string, int64, [32]byte) ([]byte, error) {
	return nil, ErrCustodyWorkflow
}

func readRHPrivateSnapshot(string, int64, [32]byte) ([]byte, error) {
	return nil, ErrCustodyWorkflow
}

func readRHPrivateBytes(string, int64, [32]byte) ([]byte, error) {
	return nil, ErrCustodyWorkflow
}

func writeCustodyPrivateFile(string, []byte) error   { return ErrCustodyWorkflow }
func writeRHPrivateFile(string, []byte, int64) error { return ErrCustodyWorkflow }

func custodySigningOutputAvailable(string) error { return ErrCustodySigning }
