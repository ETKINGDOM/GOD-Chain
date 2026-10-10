//go:build !go1.25 || (!linux && !darwin)

package godrh

import "context"

func OpenPrivateCustodyNonceBook(context.Context, CustodyNonceInputs, string, bool, ...[32]byte) (*CustodyNonceBook, error) {
	return nil, ErrCustodyNonceStorage
}
