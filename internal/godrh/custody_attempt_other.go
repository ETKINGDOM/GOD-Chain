//go:build !go1.25 || (!linux && !darwin)

package godrh

import "context"

func OpenPrivateCustodyAttempt(context.Context, CustodyTransactionInputs, string, bool, ...[32]byte) (*CustodyAttempt, error) {
	return nil, ErrCustodyAttemptStorage
}
