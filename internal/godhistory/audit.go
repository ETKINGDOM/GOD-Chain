//go:build go1.25

package godhistory

import (
	"bytes"
	"context"
	"encoding/binary"
	"strconv"
	"time"

	bolt "go.etcd.io/bbolt"
)

// AuditReport describes logical consistency of one retained storage snapshot,
// not freshness, authenticated source history or complete address membership.
// It deliberately includes no address, transaction, bundle or path value.
type AuditReport struct {
	Version                int    `json:"version"`
	Scope                  string `json:"scope"`
	StorageConsistent      bool   `json:"storageConsistent"`
	FirstHeight            string `json:"firstHeight"`
	IndexedThrough         string `json:"indexedThrough"`
	Blocks                 string `json:"blocks"`
	Transactions           string `json:"transactions"`
	HashEntries            string `json:"hashEntries"`
	AddressMemberships     string `json:"addressMemberships"`
	Unsupported            string `json:"unsupportedTransactions"`
	StoredSourceReconciled bool   `json:"storedSourceReconciled"`
	LastSourceCheck        string `json:"lastSourceCheck"`
	FreshSourceChecked     bool   `json:"freshSourceChecked"`
	CompleteHistory        bool   `json:"completeHistory"`
	PublicRoute            bool   `json:"publicRoute"`
	Synthetic              bool   `json:"synthetic"`
	RealAssets             bool   `json:"realAssets"`
}

func auditRecord(raw, key []byte, c Config, through int64) (Record, error) {
	var r Record
	if len(key) != 12 || len(raw) == 0 || len(raw) > 512 {
		return r, ErrHistory
	}
	h := binary.BigEndian.Uint64(key[:8])
	i := binary.BigEndian.Uint32(key[8:])
	if h < uint64(c.FirstHeight) || h > uint64(through) || i >= BlockLimit || exact(raw, &r) != nil || r.Height != strconv.FormatUint(h, 10) || r.Index != strconv.FormatUint(uint64(i), 10) || !hash(r.ConsensusHash) {
		return Record{}, ErrHistory
	}
	switch r.Kind {
	case "native":
		if r.CompatibleHash != "" || r.Execution != "sdk-succeeded" && r.Execution != "sdk-failed" {
			return Record{}, ErrHistory
		}
	case "ethereum":
		if !hash(r.CompatibleHash) || r.Execution != "succeeded" && r.Execution != "failed" && r.Execution != "sdk-failed" {
			return Record{}, ErrHistory
		}
	default:
		return Record{}, ErrHistory
	}
	return r, nil
}

// Audit scans a privately opened read-only copy, using constant-sized working
// state and a thirty-second work deadline. It checks every declared block and
// transaction plus both reverse indexes; orphan/extra keys and missing declared
// block/transaction/hash keys fail closed.
// It neither calls a source nor writes a verification flag, repairs data,
// restores a file, enables queries or asserts bbolt physical-page integrity.
// Stored compact records cannot reconstruct original participant semantics or
// authenticate consistently rewritten data; a trusted backup digest and fresh
// source/anchor reconciliation remain separate restore requirements.
func (x *Index) Audit(ctx context.Context) (report AuditReport, err error) {
	if ctx == nil || !x.readOnly || !x.mu.TryLock() {
		return report, ErrHistory
	}
	defer x.mu.Unlock()
	defer func() {
		if recover() != nil {
			report = AuditReport{}
			err = ErrHistory
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	err = x.db.View(func(tx *bolt.Tx) error {
		if ctx.Err() != nil {
			return ErrHistory
		}
		bucketCount := 0
		if tx.ForEach(func(name []byte, _ *bolt.Bucket) error {
			if ctx.Err() != nil || !bytes.Equal(name, metaBucket) && !bytes.Equal(name, blocksBucket) && !bytes.Equal(name, recordsBucket) && !bytes.Equal(name, addressesBucket) && !bytes.Equal(name, hashesBucket) {
				return ErrHistory
			}
			bucketCount++
			return nil
		}) != nil || bucketCount != 5 {
			return ErrHistory
		}
		m := tx.Bucket(metaBucket).Cursor()
		k, v := m.First()
		if !bytes.Equal(k, []byte("state")) || len(v) == 0 || len(v) > 2048 {
			return ErrHistory
		}
		if k, _ = m.Next(); k != nil {
			return ErrHistory
		}
		s, e := x.state(tx)
		if e != nil {
			return ErrHistory
		}
		blocks := tx.Bucket(blocksBucket).Cursor()
		records := tx.Bucket(recordsBucket).Cursor()
		rk, rv := records.First()
		var count, transactions, unsupported uint64
		var previousHash string
		var previousTime time.Time
		for bk, bv := blocks.First(); bk != nil; bk, bv = blocks.Next() {
			if ctx.Err() != nil || count >= s.Blocks || len(bk) != 8 || len(bv) == 0 || len(bv) > 512 {
				return ErrHistory
			}
			h := s.Config.FirstHeight + int64(count)
			if !bytes.Equal(bk, heightKey(h)) {
				return ErrHistory
			}
			b, e := storedBlock(tx, h)
			when, te := stamp(b.Time)
			if e != nil || te != nil || h == 1 && b.Parent != "0x" || count > 0 && (b.Parent != previousHash || when.Before(previousTime)) || b.Unsupported < unsupported || b.Unsupported-unsupported > uint64(b.Transactions) {
				return ErrHistory
			}
			for i := 0; i < b.Transactions; i++ {
				if ctx.Err() != nil || transactions >= s.Transactions || !bytes.Equal(rk, tuple(h, i)) {
					return ErrHistory
				}
				r, e := auditRecord(rv, rk, s.Config, s.Height)
				if e != nil || !bytes.Equal(tx.Bucket(hashesBucket).Get([]byte(r.ConsensusHash)), rk) {
					return ErrHistory
				}
				transactions++
				rk, rv = records.Next()
			}
			count++
			unsupported, previousHash, previousTime = b.Unsupported, b.Hash, when
		}
		if count != s.Blocks || transactions != s.Transactions || unsupported != s.Unsupported || rk != nil {
			return ErrHistory
		}
		var hashes, memberships uint64
		hc := tx.Bucket(hashesBucket).Cursor()
		for hk, hv := hc.First(); hk != nil; hk, hv = hc.Next() {
			if ctx.Err() != nil || hashes >= s.Transactions || len(hk) != 66 || !hash(string(hk)) {
				return ErrHistory
			}
			r, e := auditRecord(tx.Bucket(recordsBucket).Get(hv), hv, s.Config, s.Height)
			if e != nil || r.ConsensusHash != string(hk) {
				return ErrHistory
			}
			hashes++
		}
		if hashes != s.Transactions {
			return ErrHistory
		}
		ac := tx.Bucket(addressesBucket).Cursor()
		for ak, av := ac.First(); ak != nil; ak, av = ac.Next() {
			if ctx.Err() != nil || memberships >= s.Transactions*128 || len(ak) != 32 || len(av) != 1 || av[0] < 1 || av[0] > 3 {
				return ErrHistory
			}
			if _, e := auditRecord(tx.Bucket(recordsBucket).Get(ak[20:]), ak[20:], s.Config, s.Height); e != nil {
				return ErrHistory
			}
			memberships++
		}
		if ctx.Err() != nil || memberships < s.Transactions-s.Unsupported {
			return ErrHistory
		}
		report = AuditReport{
			Version: 1, Scope: "retained-storage-only", StorageConsistent: true,
			FirstHeight: strconv.FormatInt(s.Config.FirstHeight, 10), IndexedThrough: strconv.FormatInt(s.Height, 10),
			Blocks: strconv.FormatUint(count, 10), Transactions: strconv.FormatUint(transactions, 10),
			HashEntries: strconv.FormatUint(hashes, 10), AddressMemberships: strconv.FormatUint(memberships, 10),
			Unsupported: strconv.FormatUint(unsupported, 10), StoredSourceReconciled: s.Verified,
			LastSourceCheck: s.CheckedAt, Synthetic: true,
		}
		return nil
	})
	if err != nil {
		return AuditReport{}, ErrHistory
	}
	return report, nil
}
