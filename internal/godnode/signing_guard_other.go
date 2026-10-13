//go:build !go1.25 || (!linux && !darwin)

package godnode

import "github.com/cometbft/cometbft/privval"

// Owner-only descriptor, locking and durable directory acceptance are required.
// Unsupported hosts never silently use an in-memory or unlocked substitute.
func InitializeSigningGuard(string, string, [32]byte, *privval.FilePV) error { return ErrSigningGuard }
func OpenSigningGuard(string, string, [32]byte, *privval.FilePV) (*GuardedSigner, error) {
	return nil, ErrSigningGuard
}

func InitializeBoundSigningGuard(string, string, [32]byte, *privval.FilePV, func() error) error {
	return ErrSigningGuard
}

func InitializeFileSigningGuard(string, string, [32]byte, *privval.FilePV, SigningGuardFiles, func() error) error {
	return ErrSigningGuard
}

func OpenFileSigningGuard(string, string, [32]byte, *privval.FilePV, SigningGuardFiles) (*GuardedSigner, error) {
	return nil, ErrSigningGuard
}
