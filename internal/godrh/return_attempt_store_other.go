//go:build !go1.25 || (!linux && !darwin)

package godrh

import "context"

// No unverified disk/ACL fallback. Offline envelope review remains separate.
func OpenPrivateNativeReturnAttempt(context.Context, *ReturnSettlementOutbox, NativeResolutionTransaction,
	string, bool, ...[32]byte) (*NativeReturnAttempt, error) {
	return nil, ErrNativeReturnAttemptStorage
}
