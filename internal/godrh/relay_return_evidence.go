package godrh

import (
	"context"
	"encoding/json"
	"errors"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
)

var ErrReturnReceiptEvidence = errors.New("paired return evidence review rejected; no journal change, proof or financial approval")

// A transient unsigned review, not a retained authorization or trusted proof.
// No task/cache/retry/clock/cursor change, signing, broadcasting or asset movement.
type ReturnReceiptEvidenceReview struct {
	receipt TaskReceiptEvidenceReview
	native  NativeWithdrawalObservation
	ready   bool
}
type ReturnReceiptEvidenceReport struct {
	TaskReceiptEvidenceReport
	NativeCommittedViewMatched bool `json:"nativeCommittedViewMatched"`
	NativeSameCommitRechecked  bool `json:"nativeSameCommitRechecked"`
	NativeBindingMatched       bool `json:"nativeBindingMatched"`
	OriginalWithdrawalMatched  bool `json:"originalWithdrawalMatched"`
}

func (ReturnReceiptEvidenceReview) String() string {
	return "paired simulation return evidence (redacted)"
}
func (r ReturnReceiptEvidenceReview) GoString() string             { return r.String() }
func (r ReturnReceiptEvidenceReview) MarshalJSON() ([]byte, error) { return json.Marshal(r.Report()) }
func (r ReturnReceiptEvidenceReview) Report() ReturnReceiptEvidenceReport {
	if !r.ready {
		return ReturnReceiptEvidenceReport{}
	}
	return ReturnReceiptEvidenceReport{TaskReceiptEvidenceReport: r.receipt.Report(),
		NativeCommittedViewMatched: true, NativeSameCommitRechecked: true, NativeBindingMatched: true, OriginalWithdrawalMatched: true}
}
func (r ReturnReceiptEvidenceReview) ReceiptReview() (TaskReceiptEvidenceReview, error) {
	if !r.ready {
		return TaskReceiptEvidenceReview{}, ErrReturnReceiptEvidence
	}
	return r.receipt, nil
}
func (r ReturnReceiptEvidenceReview) NativeSnapshot() (godbridge.Withdrawal, godnode.CommittedView, error) {
	if !r.ready {
		return godbridge.Withdrawal{}, godnode.CommittedView{}, ErrReturnReceiptEvidence
	}
	return r.native.Snapshot()
}

// Read one retained Paid/Cancelled task against a supplied native committed
// view before AND after complete RH receipt/terminal review. Every native read
// must still expose the original unfinished withdrawal at the exact same
// commit. Native or RH inconsistency invalidates the entire result, never a
// partial proposal. This deliberately refuses already-resolved native records.
// Operator RPC agreement is NOT independent native state or source finality.
// The lock and active 30-second ceiling match ReviewTaskReceiptEvidence.
func (j *RelayJournal) ReviewReturnReceiptEvidence(ctx context.Context, ticket uint64, source ReceiptSetSource, native NativeBridgeReadSource) (out ReturnReceiptEvidenceReview, err error) {
	defer func() {
		if recover() != nil {
			out, err = ReturnReceiptEvidenceReview{}, ErrReturnReceiptEvidence
		}
	}()
	bad := func() (ReturnReceiptEvidenceReview, error) {
		return ReturnReceiptEvidenceReview{}, ErrReturnReceiptEvidence
	}
	if j == nil || ctx == nil || ctx.Err() != nil || source == nil || native == nil {
		return bad()
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.reviewReturnReceiptEvidence(ctx, ticket, source, native)
}

// Caller holds j.mu; the durable simulation outbox uses the same review.
func (j *RelayJournal) reviewReturnReceiptEvidence(ctx context.Context, ticket uint64, source ReceiptSetSource, native NativeBridgeReadSource) (ReturnReceiptEvidenceReview, error) {
	bad := func() (ReturnReceiptEvidenceReview, error) {
		return ReturnReceiptEvidenceReview{}, ErrReturnReceiptEvidence
	}
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	pin, _, err := j.taskReceiptProofPin(readCtx, ticket)
	if err != nil {
		return bad()
	}
	task := j.state.Tasks[ticket-1]
	if task.Request.Kind != string(godbridge.Paid) && task.Request.Kind != string(godbridge.Cancelled) || task.Request.Withdrawal == nil {
		return bad()
	}
	// Detach before invoking either replaceable source.
	w := *task.Request.Withdrawal
	w.Amount = sdkmath.NewIntFromBigInt(w.Amount.BigInt())
	before, err := ReadNativeWithdrawal(readCtx, j.config, native, w.Sequence, w.ID)
	if err != nil || !sameWithdrawal(before.withdrawal, w) {
		return bad()
	}
	receipt, err := j.reviewTaskReceiptEvidence(readCtx, ticket, source)
	if err != nil || !receipt.resolution.ready || !sameWithdrawal(receipt.resolution.withdrawal, w) {
		return bad()
	}
	after, err := readNativeWithdrawalAt(readCtx, j.config, native, w.Sequence, w.ID, before.bridge.Commit.Height)
	if err != nil || !sameNativeWithdrawal(before, after) {
		return bad()
	}
	afterPin, _, err := j.taskReceiptProofPin(readCtx, ticket)
	if err != nil || afterPin != pin || readCtx.Err() != nil {
		return bad()
	}
	return ReturnReceiptEvidenceReview{receipt: receipt, native: before, ready: true}, nil
}
