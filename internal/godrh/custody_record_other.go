//go:build !go1.25 || (!linux && !darwin)

package godrh

import "context"

func checkCustodyNewOutput(string) error { return ErrCustodyRecordFile }
func OpenPrivateCustodyRecord(context.Context, CustodyFileInputs, string, string, bool) (*CustodyRecord, error) {
	return nil, ErrCustodyRecordStorage
}
