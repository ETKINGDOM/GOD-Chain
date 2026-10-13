package godrh

import (
	"context"
	"encoding/hex"
	"errors"
	"math"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
)

var ErrNativeReturnExecution = errors.New("simulation native execution observation rejected or conflicts; retain original attempt for independent review")
var ErrNativeReturnSubmitUnknown = errors.New("simulation native submission outcome unknown; do not retry, replace or release reservations")

// Optional v1 observations, preserving all old exact encodings. Only identity
// digests are retained; separately retain the full private material and trust
// to recheck it. This digest is not a transferable proof or financial permit.
type nativeReturnExecutionRecord struct {
	TransactionHash string `json:"transactionHash"`
	MaterialPin     string `json:"materialPin"`
	TrustPin        string `json:"trustPin"`
	Height          int64  `json:"height,string"`
	Confirmation    int64  `json:"confirmationHeight,string"`
	Index           uint32 `json:"index"`
	Code            uint32 `json:"code"`
}

func validNativeReturnExecutionRecords(s nativeReturnSequenceState) bool {
	if len(s.Reviews) > len(s.Entries) {
		return false
	}
	seen := map[string]bool{}
	for _, r := range s.Reviews {
		if r.Height <= 0 || r.Height == math.MaxInt64 || r.Confirmation != r.Height+1 || r.Index >= godnode.MaxNativeExecutionTransactions || seen[r.TransactionHash] {
			return false
		}
		for _, pin := range []string{r.TransactionHash, r.MaterialPin, r.TrustPin} {
			if _, err := SourceMaterialPin(pin); err != nil {
				return false
			}
		}
		found := false
		for _, e := range s.Entries {
			if e.Attempt.TransactionHash == r.TransactionHash && e.Attempt.Stage == NativeReturnAttemptUnknown {
				found = true
			}
		}
		if !found {
			return false
		}
		seen[r.TransactionHash] = true
	}
	return true
}

type NativeReturnExecutionReport struct {
	Execution                 godnode.NativeExecutionReport `json:"execution"`
	ObservationRetained       bool                          `json:"observationRetained"`
	OriginalAttemptMatched    bool                          `json:"originalAttemptMatched"`
	NativeAttestationMatched  bool                          `json:"nativeAttestationMatched"`
	SequenceProgressConfirmed bool                          `json:"sequenceProgressConfirmed"`
	RetryAllowed              bool                          `json:"retryAllowed"`
	FeesReleased              bool                          `json:"feesReleased"`
	RefundAuthorized          bool                          `json:"refundAuthorized"`
	PaymentVerified           bool                          `json:"paymentVerified"`
	BroadcastEnabled          bool                          `json:"broadcastEnabled"`
	RealAssetsReady           bool                          `json:"realAssetsReady"`
}

// Reconcile the exact dispatched SDK envelope, not merely an outbox's terminal
// withdrawal status. The independently reviewed confirmation header/validator
// set bounds this conditional native proof. Neither native account progression
// nor authentic RH funding/finality, fees or financial recovery is inferred.
// Locks follow book -> original outbox -> original journal. No provider call.
func (b *NativeReturnSequenceBook) ReconcileExecutionForSimulation(ctx context.Context, p *ReturnSettlementOutbox, t NativeResolutionTransaction,
	trust godnode.NativeExecutionTrust, material godnode.NativeExecutionMaterial, index uint32) (report NativeReturnExecutionReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = NativeReturnExecutionReport{}, ErrNativeReturnExecution
		}
	}()
	if b == nil || b.nativeReturnSequenceCore == nil || p == nil || p.journal == nil || ctx == nil || ctx.Err() != nil {
		return report, ErrNativeReturnExecution
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.journal.mu.Lock()
	defer p.journal.mu.Unlock()
	if err := b.check(ctx); err != nil {
		return report, err
	}
	wanted, checked, err := b.executionCandidate(ctx, p, t, true)
	if err != nil || trust.ChainID != checked.binding.NativeChain || p.state.Record == nil || p.state.Record.Handoff == nil {
		return report, ErrNativeReturnExecution
	}
	found := false
	for _, e := range b.state.Entries {
		if nativeSequenceSameEntry(e, wanted) && e.Attempt.Stage == NativeReturnAttemptUnknown {
			found = true
		}
	}
	if !found {
		return report, ErrNativeReturnExecution
	}
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	review, err := godnode.ReviewNativeExecution(readCtx, checked.raw, checked.plan.Gas, trust, material, index)
	r := review.Report()
	if err != nil || !r.ConditionalInclusionMatched || r.Height <= p.state.Record.Handoff.Height || readCtx.Err() != nil {
		return report, ErrNativeReturnExecution
	}
	materialPin, trustPin, err := review.RetainedIdentities()
	if err != nil {
		return report, ErrNativeReturnExecution
	}
	observation := nativeReturnExecutionRecord{hex.EncodeToString(checked.hash[:]), hex.EncodeToString(materialPin[:]), hex.EncodeToString(trustPin[:]), r.Height, r.ConfirmationHeight, index, r.ExecutionCode}
	for _, prior := range b.state.Reviews {
		if prior.TransactionHash == observation.TransactionHash {
			if prior != observation {
				return report, ErrNativeReturnExecution
			}
			return NativeReturnExecutionReport{Execution: r, ObservationRetained: true, OriginalAttemptMatched: true, NativeAttestationMatched: p.state.Stage == ReturnOutboxMatched}, nil
		}
	}
	next := b.next()
	next.Reviews = append(next.Reviews, observation)
	if ctx.Err() != nil {
		return report, ErrNativeReturnExecution
	}
	if err := b.write(next); err != nil {
		return report, err
	}
	if ctx.Err() != nil {
		// The observation can be durable while acknowledgment is lost. A later
		// explicit exact recheck is idempotent; never turn cancellation into a
		// fresh send or use it as permission to discard the retained record.
		return report, ErrNativeReturnExecution
	}
	return NativeReturnExecutionReport{Execution: r, ObservationRetained: true, OriginalAttemptMatched: true, NativeAttestationMatched: p.state.Stage == ReturnOutboxMatched}, nil
}

// A caller-supplied LOCAL simulator only. No network endpoint or signer is
// supplied. The adapter owns detached bytes and may retain them for its single
// pending admission (for example the local node's mempool). It must honor
// context and avoid hidden resubmissions. Callbacks run without journal locks;
// a malicious adapter is not fenced by this simulation API.
// Successful SubmitLocalNative means at most admission, never confirmation.
type NativeReturnSimulationSubmitter interface {
	SubmitLocalNative(context.Context, []byte) error
}

type NativeReturnSubmissionReport struct {
	SimulationOnly         bool `json:"simulationOnly"`
	SubmissionAttempted    bool `json:"submissionAttempted"`
	AdmissionAcknowledged  bool `json:"admissionAcknowledged"`
	DispatchOutcomeUnknown bool `json:"dispatchOutcomeUnknown"`
	RetryAllowed           bool `json:"retryAllowed"`
	SequenceReleased       bool `json:"sequenceReleased"`
	FeesReleased           bool `json:"feesReleased"`
	RealAssetsReady        bool `json:"realAssetsReady"`
}

// Retain UNKNOWN and verify readback BEFORE one bounded explicit local call.
// A provider error, panic, timeout, cancellation or lost acknowledgment never
// retries that envelope. Recovery uses exact execution evidence, not a send.
func (b *NativeReturnSequenceBook) SubmitForSimulation(ctx context.Context, p *ReturnSettlementOutbox, t NativeResolutionTransaction, submitter NativeReturnSimulationSubmitter) (report NativeReturnSubmissionReport, err error) {
	report.SimulationOnly = true
	defer func() {
		if recover() != nil {
			report.DispatchOutcomeUnknown = true
			err = ErrNativeReturnSubmitUnknown
		}
	}()
	if ctx == nil || ctx.Err() != nil || submitter == nil {
		return report, ErrNativeReturnSequenceState
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	wire, err := b.TakeForSimulation(sendCtx, p, t)
	if err != nil {
		report.DispatchOutcomeUnknown = errors.Is(err, ErrNativeReturnSequenceUnknown) || errors.Is(err, ErrNativeReturnSequenceStorage)
		return report, err
	}
	// Ownership passes to the adapter. Never clear/mutate its slice after an
	// admission acknowledgment: the node may still hold it in its mempool.
	report.DispatchOutcomeUnknown = true
	if sendCtx.Err() != nil {
		return report, ErrNativeReturnSubmitUnknown
	}
	report.SubmissionAttempted = true
	if submitter.SubmitLocalNative(sendCtx, wire) != nil || sendCtx.Err() != nil {
		return report, ErrNativeReturnSubmitUnknown
	}
	report.AdmissionAcknowledged = true
	return report, nil
}
