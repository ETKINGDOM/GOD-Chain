//go:build !go1.25 || (!linux && !darwin)

package godrh

func OpenPrivateCustodyCreationJournal(string, bool) (CustodyCreationJournalStorage, error) {
	return nil, ErrCustodyCreationJournal
}
