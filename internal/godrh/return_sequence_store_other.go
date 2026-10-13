//go:build !go1.25 || (!linux && !darwin)

package godrh

import (
	"context"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
)

// No unverified filesystem/ACL fallback.
func OpenPrivateNativeReturnSequenceBook(context.Context, godbridge.Config, NativeReturnSequencePlan, string, bool, ...[32]byte) (*NativeReturnSequenceBook, error) {
	return nil, ErrNativeReturnSequenceStorage
}
