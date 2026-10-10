//go:build go1.25

package godhistory

import (
	"context"
	"strconv"
	"time"

	bolt "go.etcd.io/bbolt"
)

type BackfillOptions struct {
	ThroughHeight         int64
	ThroughHash           string
	BatchSize, MaxBatches int
	Pause                 time.Duration
}

// Fixed counters/coverage only. No source identity, target hash, addresses,
// payloads, provider errors or filesystem paths are reported.
type BackfillReport struct {
	Synthetic               bool   `json:"synthetic"`
	RealAssets              bool   `json:"realAssets"`
	PublicRoute             bool   `json:"publicRoute"`
	CompleteHistory         bool   `json:"completeHistory"`
	SupportedSummariesOnly  bool   `json:"supportedSummariesOnly"`
	FromGenesis             bool   `json:"fromGenesis"`
	TargetReached           bool   `json:"targetReached"`
	SourceReconciled        bool   `json:"sourceReconciled"`
	From                    string `json:"from"`
	TargetHeight            string `json:"targetHeight"`
	IndexedThrough          string `json:"indexedThrough"`
	RetainedBlocks          string `json:"retainedBlocks"`
	RetainedTransactions    string `json:"retainedTransactions"`
	UnsupportedTransactions string `json:"unsupportedTransactions"`
	BatchesCompleted        int    `json:"batchesCompleted"`
	NewBlocks               int    `json:"newBlocks"`
	Reason                  string `json:"reason"`
}

func (o BackfillOptions) Validate(c Config) error {
	if !c.valid() || o.ThroughHeight < c.FirstHeight || !hash(o.ThroughHash) ||
		uint64(o.ThroughHeight-c.FirstHeight)+1 > c.MaxBlocks || o.BatchSize < 1 || o.BatchSize > BatchLimit ||
		o.MaxBatches < 1 || o.MaxBatches > 32 || o.Pause < time.Second || o.Pause > 30*time.Second {
		return ErrHistory
	}
	return nil
}

func backfillWait(ctx context.Context, pause time.Duration) error {
	timer := time.NewTimer(pause)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ErrHistory
	case <-timer.C:
		return nil
	}
}

// Backfill appends at most 32 bounded batches toward an explicit immutable
// target. It never follows a growing tip forever or changes configured limits.
// Every batch rechecks both its prior checkpoint and the selected target; only
// complete blocks survive interruption. It uses no signing keys and writes no
// chain state. Index writes remain confined to the explicit private workspace.
// Reaching the target is declared-range coverage, not authenticated finality,
// an archive of all transaction semantics or a complete account/NFT history.
func (x *Index) Backfill(ctx context.Context, source Source, o BackfillOptions) (BackfillReport, error) {
	return x.backfill(ctx, source, o, backfillWait)
}

// The wait seam is private to deterministic scheduling/cancellation fixtures.
// Exported Backfill uses real timers, a 30-minute total and five-minute batches.
func (x *Index) backfill(ctx context.Context, source Source, o BackfillOptions, wait func(context.Context, time.Duration) error) (r BackfillReport, err error) {
	r = BackfillReport{Synthetic: true, SupportedSummariesOnly: true, Reason: "configuration"}
	if x == nil || ctx == nil || ctx.Err() != nil || source == nil || wait == nil || x.readOnly || o.Validate(x.config) != nil {
		return r, ErrHistory
	}
	x.mu.Lock()
	defer x.mu.Unlock()
	defer func() {
		if recover() != nil {
			r.TargetReached, r.SourceReconciled, r.Reason = false, false, "unavailable"
			err = ErrHistory
		}
	}()
	var s State
	readState := func() error {
		return x.db.View(func(tx *bolt.Tx) error { var e error; s, e = x.state(tx); return e })
	}
	if readState() != nil || s.Height > o.ThroughHeight {
		return r, ErrHistory
	}
	r.From, r.TargetHeight, r.FromGenesis = strconv.FormatInt(x.config.FirstHeight, 10), strconv.FormatInt(o.ThroughHeight, 10), x.config.FirstHeight == 1
	updateCounts := func() {
		r.IndexedThrough = strconv.FormatInt(s.Height, 10)
		r.RetainedBlocks, r.RetainedTransactions, r.UnsupportedTransactions = strconv.FormatUint(s.Blocks, 10), strconv.FormatUint(s.Transactions, 10), strconv.FormatUint(s.Unsupported, 10)
	}
	updateCounts()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	initial := s.Blocks
	var anchor backfillAnchor
	for i := 0; i < o.MaxBatches; i++ {
		// Refuse queries throughout any interrupted batch or inter-batch wait.
		// The last fully reconciled successful/budgeted exit enables them again.
		s.Verified = false
		if x.db.Update(func(tx *bolt.Tx) error { return putState(tx, s) }) != nil {
			r.SourceReconciled, r.Reason = false, "storage-unavailable"
			return r, ErrHistory
		}
		if i > 0 && wait(ctx, o.Pause) != nil || ctx.Err() != nil {
			r.SourceReconciled, r.Reason = false, "cancelled"
			return r, ErrHistory
		}
		batchCtx, cancelBatch := context.WithTimeout(ctx, 5*time.Minute)
		_, e := x.syncLocked(batchCtx, source, o.BatchSize, o.ThroughHeight, o.ThroughHash, &anchor)
		cancelBatch()
		if readState() != nil {
			r.SourceReconciled, r.Reason = false, "unavailable"
			return r, ErrHistory
		}
		updateCounts()
		r.NewBlocks = int(s.Blocks - initial)
		if e != nil {
			r.SourceReconciled, r.Reason = false, "source-or-storage-unavailable"
			return r, ErrHistory
		}
		r.BatchesCompleted++
		r.SourceReconciled = s.Verified
		if s.Height == o.ThroughHeight {
			r.TargetReached, r.Reason = true, "target-range-covered"
			return r, nil
		}
	}
	r.Reason = "batch-budget-reached"
	return r, nil
}
