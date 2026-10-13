package godrh

import (
	"context"
	"errors"
	"time"
)

var ErrCustodySubmitUnknown = errors.New("RH local custody submission outcome unknown; retain the original journal and do not resend")

// An explicitly injected LOCAL simulator, not an installed financial transport.
// It owns detached signed bytes and may retain them for one pending admission.
// It must honor context, make no hidden retries, and not invoke another sender
// with the same key. Callbacks run without journal locks; malicious adapters
// are outside this boundary. A nil error means admission at most, not payment.
type CustodySimulationSubmitter interface {
	SubmitLocalCustody(context.Context, []byte) error
}

type CustodySubmissionReport struct {
	SimulationOnly         bool `json:"simulationOnly"`
	SubmissionAttempted    bool `json:"submissionAttempted"`
	AdmissionAcknowledged  bool `json:"admissionAcknowledged"`
	DispatchOutcomeUnknown bool `json:"dispatchOutcomeUnknown"`
	RetryAllowed           bool `json:"retryAllowed"`
	NonceReleased          bool `json:"nonceReleased"`
	FeeReservationReleased bool `json:"feeReservationReleased"`
	PaymentVerified        bool `json:"paymentVerified"`
	BroadcastEnabled       bool `json:"broadcastEnabled"`
	RealAssetsReady        bool `json:"realAssetsReady"`
}

// SubmitForSimulation composes the existing sender-wide nonce/Gas journal with
// one bounded local admission. It requires an already reviewed, signed and
// reserved envelope. UNKNOWN is synchronously saved and read back before the
// adapter sees bytes. Error, panic, cancellation, lost acknowledgment or even
// successful admission cannot release the reservation or permit a second call.
// The adapter's bytes are never cleared after it returns: pending admission
// may still own them. No signer, endpoint, production mode or CLI is installed.
// The ten-second active context is cooperative, not an interruptible lock or
// a fence against an adapter that ignores cancellation.
func (b *CustodyNonceBook) SubmitForSimulation(ctx context.Context, i CustodyTransactionInputs, submitter CustodySimulationSubmitter) (report CustodySubmissionReport, err error) {
	report.SimulationOnly = true
	defer func() {
		if recover() != nil {
			report.DispatchOutcomeUnknown = true
			err = ErrCustodySubmitUnknown
		}
	}()
	if ctx == nil || ctx.Err() != nil || submitter == nil {
		return report, ErrCustodyNonceState
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	wire, err := b.TakeForSimulation(sendCtx, i)
	if err != nil {
		report.DispatchOutcomeUnknown = errors.Is(err, ErrCustodyNonceUnknown) || errors.Is(err, ErrCustodyNonceStorage)
		return report, err
	}
	report.DispatchOutcomeUnknown = true
	if sendCtx.Err() != nil {
		return report, ErrCustodySubmitUnknown
	}
	report.SubmissionAttempted = true
	if submitter.SubmitLocalCustody(sendCtx, wire) != nil || sendCtx.Err() != nil {
		return report, ErrCustodySubmitUnknown
	}
	report.AdmissionAcknowledged = true
	return report, nil
}

// RecoverForSimulation has no submission capability. It reads the exact
// original UNKNOWN transaction once, checks the complete receipt set, then
// rechecks and durably retains its observation in the SAME shared book. The
// returned private material can be separately retained; the journal only stores
// a digest. Provider agreement is not independent source finality or payment.
// Cancellation/write uncertainty returns no ready result even if a new record
// was retained; explicit exact recovery is idempotent, never redispatch.
func (b *CustodyNonceBook) RecoverForSimulation(ctx context.Context, i CustodyTransactionInputs, source CustodyReceiptSource) (fetched FetchedCustodyReceipt, err error) {
	defer func() {
		if recover() != nil {
			fetched, err = FetchedCustodyReceipt{}, ErrCustodyReceiptFetch
		}
	}()
	if ctx == nil || ctx.Err() != nil || source == nil {
		return FetchedCustodyReceipt{}, ErrCustodyReceiptFetch
	}
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	f, err := b.FetchReceiptForSimulation(readCtx, i, source)
	if err != nil {
		return FetchedCustodyReceipt{}, err
	}
	block, material, err := f.Material()
	if err != nil || readCtx.Err() != nil {
		return FetchedCustodyReceipt{}, ErrCustodyReceiptFetch
	}
	report, err := b.ReconcileForSimulation(readCtx, i, block, material)
	if err != nil {
		return FetchedCustodyReceipt{}, err
	}
	if readCtx.Err() != nil {
		return FetchedCustodyReceipt{}, ErrCustodyReceiptFetch
	}
	f.report.CustodyReconciliationReport = report
	return f, nil
}
