package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strconv"
	"time"
)

const (
	custodySigningReserved = "signing_reserved_simulation"
	custodySigningUnknown  = "signing_unknown_simulation"
	custodySigningRetained = "signed_retained_simulation"
)

// Only the current intent occupies the live snapshot; earlier intents remain
// in the immutable hash-linked history. Bound intents refer to an existing
// exact signed entry. A pending intent consumes that next nonce and Gas budget.
type custodySigningIntent struct {
	RequestSHA256    string `json:"requestSHA256"`
	PlanSHA256       string `json:"planSHA256"`
	UnsignedSHA256   string `json:"unsignedSHA256"`
	OutputPathSHA256 string `json:"outputPathSHA256"`
	Nonce            string `json:"nonce"`
	MaximumGasCost   string `json:"maximumGasCost"`
	Stage            string `json:"stage"`
	EnvelopeSHA256   string `json:"envelopeSHA256,omitempty"`
	TransactionHash  string `json:"transactionHash,omitempty"`
}

func custodySigningPending(s custodyNonceState) bool {
	return s.SigningIntent != nil && s.SigningIntent.Stage != custodySigningRetained
}
func sameCustodySigningState(a, b custodyNonceState) bool {
	return a.SigningRevision == b.SigningRevision && (a.SigningIntent == nil && b.SigningIntent == nil ||
		a.SigningIntent != nil && b.SigningIntent != nil && *a.SigningIntent == *b.SigningIntent)
}
func sameCustodySigningIntent(a, b custodySigningIntent) bool {
	a.Stage, b.Stage = custodySigningReserved, custodySigningReserved
	a.EnvelopeSHA256, b.EnvelopeSHA256, a.TransactionHash, b.TransactionHash = "", "", "", ""
	return a == b
}

func custodySigningStateCost(s custodyNonceState) (*big.Int, error) {
	bad := func() (*big.Int, error) { return nil, ErrCustodyNonceState }
	if s.SigningIntent == nil {
		if s.SigningRevision != 0 {
			return bad()
		}
		return new(big.Int), nil
	}
	i := s.SigningIntent
	if s.SigningRevision == 0 || s.SigningRevision > 2*MaxCustodyNonceEntries {
		return bad()
	}
	for _, pin := range []string{i.RequestSHA256, i.PlanSHA256, i.UnsignedSHA256, i.OutputPathSHA256} {
		if _, err := SourceMaterialPin(pin); err != nil {
			return bad()
		}
	}
	nonce, err := custodyNonceCounter(i.Nonce)
	if err != nil {
		return bad()
	}
	start, err := custodyNonceCounter(s.StartingNonce)
	if err != nil {
		return bad()
	}
	cost, err := custodyNonceAmount(i.MaximumGasCost)
	if err != nil {
		return bad()
	}
	if i.Stage == custodySigningRetained {
		if s.SigningRevision%2 != 0 || s.SigningRevision > uint32(2*len(s.Entries)) {
			return bad()
		}
		for _, pin := range []string{i.EnvelopeSHA256, i.TransactionHash} {
			if _, err := SourceMaterialPin(pin); err != nil {
				return bad()
			}
		}
		for _, e := range s.Entries {
			if e.Nonce == i.Nonce && e.MaximumGasCost == i.MaximumGasCost && e.Binding.RequestSHA256 == i.RequestSHA256 && e.Binding.PlanSHA256 == i.PlanSHA256 &&
				e.Binding.EnvelopeSHA256 == i.EnvelopeSHA256 && e.Binding.TransactionHash == i.TransactionHash {
				return new(big.Int), nil
			}
		}
		return bad()
	}
	if (i.Stage != custodySigningReserved && i.Stage != custodySigningUnknown) ||
		(i.Stage == custodySigningReserved) != (s.SigningRevision%2 == 1) || s.SigningRevision > uint32(2*(len(s.Entries)+1)) ||
		len(s.Entries) == MaxCustodyNonceEntries || nonce != start+uint64(len(s.Entries)) || i.EnvelopeSHA256 != "" || i.TransactionHash != "" {
		return bad()
	}
	for _, e := range s.Entries {
		if e.Binding.RequestSHA256 == i.RequestSHA256 {
			return bad()
		}
	}
	return cost, nil
}

func custodySigningStepTransition(old, next custodyNonceState) bool {
	if next.SigningRevision != old.SigningRevision+1 || next.SigningIntent == nil {
		return false
	}
	if old.SigningIntent == nil || old.SigningIntent.Stage == custodySigningRetained {
		return next.SigningIntent.Stage == custodySigningReserved
	}
	return old.SigningIntent.Stage == custodySigningReserved && next.SigningIntent.Stage == custodySigningUnknown &&
		sameCustodySigningIntent(*old.SigningIntent, *next.SigningIntent)
}
func custodySigningRetentionTransition(old, next custodyNonceState) bool {
	return old.SigningIntent != nil && next.SigningIntent != nil && old.SigningIntent.Stage == custodySigningUnknown && next.SigningIntent.Stage == custodySigningRetained &&
		old.SigningRevision == next.SigningRevision && sameCustodySigningIntent(*old.SigningIntent, *next.SigningIntent)
}

func (b *CustodyNonceBook) signingCandidate(i CustodySigningInputs, output string) (custodySigningIntent, error) {
	if i.CallInputs.ConfigurationSHA256 != b.inputs.ConfigurationSHA256 || !custodyNonceSeparated(b.directory, CustodyTransactionInputs{CallInputs: i.CallInputs, PlanPath: i.PlanPath, TransactionPath: output}) {
		return custodySigningIntent{}, ErrCustodyNonceState
	}
	call, plan, unsigned, err := loadCustodySigning(i)
	if err != nil || plan.sender != b.plan.sender {
		return custodySigningIntent{}, ErrCustodyNonceState
	}
	chain, _, _, err := call.Payload()
	if err != nil || chain.Cmp(b.chain) != 0 {
		return custodySigningIntent{}, ErrCustodyNonceState
	}
	u, o := sha256.Sum256(unsigned), sha256.Sum256([]byte(output))
	return custodySigningIntent{RequestSHA256: hex.EncodeToString(i.CallInputs.RequestSHA256[:]), PlanSHA256: hex.EncodeToString(i.PlanSHA256[:]),
		UnsignedSHA256: hex.EncodeToString(u[:]), OutputPathSHA256: hex.EncodeToString(o[:]), Nonce: strconv.FormatUint(plan.nonce, 10),
		MaximumGasCost: new(big.Int).Mul(new(big.Int).SetUint64(plan.gas), plan.fee).String(), Stage: custodySigningReserved}, nil
}

// Reserve the exact next unsigned intent in the SAME sender-wide book. No
// signature or output is created here. Existing exact intents are idempotent;
// another request/output, occupied nonce, gap or shared-budget overrun refuses.
func (b *CustodyNonceBook) ReserveSigningForSimulation(ctx context.Context, i CustodySigningInputs, output string) error {
	if b == nil {
		return ErrCustodyNonceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready(ctx) {
		return ErrCustodyNonceState
	}
	if err := b.recheck(ctx); err != nil {
		return err
	}
	wanted, err := b.signingCandidate(i, output)
	if err != nil || ctx.Err() != nil {
		return ErrCustodyNonceState
	}
	if old := b.state.SigningIntent; old != nil && sameCustodySigningIntent(*old, wanted) {
		return nil
	}
	if b.signingActive || custodySigningPending(b.state) || custodySigningOutputAvailable(output) != nil {
		return ErrCustodyNonceState
	}
	next := b.nextState()
	next.SigningRevision++
	next.SigningIntent = &wanted
	if _, err := encodeCustodyNonce(next); err != nil || !custodyNonceTransition(b.state, next) || ctx.Err() != nil {
		return ErrCustodyNonceState
	}
	return b.write(next)
}

func (b *CustodyNonceBook) consumeSigning(ctx context.Context, i CustodySigningInputs, output string) (report CustodySigningReport, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready(ctx) {
		return report, ErrCustodyNonceState
	}
	if err := b.recheck(ctx); err != nil {
		return report, err
	}
	wanted, err := b.signingCandidate(i, output)
	old := b.state.SigningIntent
	if err != nil || old == nil || !sameCustodySigningIntent(*old, wanted) {
		return report, ErrCustodyNonceState
	}
	report = CustodySigningReport{SimulationOnly: true, ExactUnsignedEnvelopePrepared: true, SharedNonceReserved: true, DurableSigningAttempt: true}
	if b.signingActive || old.Stage != custodySigningReserved {
		report.LocalSigningAttemptConsumed, report.SigningOutcomeUnknown = true, true
		return report, ErrCustodySigningUnknown
	}
	if custodySigningOutputAvailable(output) != nil || ctx.Err() != nil {
		return report, ErrCustodyNonceState
	}
	next := b.nextState()
	next.SigningRevision++
	next.SigningIntent.Stage = custodySigningUnknown
	if err := b.write(next); err != nil {
		report.DurableSigningAttempt = false
		report.SigningOutcomeUnknown = true
		return report, err
	}
	report.LocalSigningAttemptConsumed, report.SigningOutcomeUnknown = true, true
	if ctx.Err() != nil {
		return report, ErrCustodySigningUnknown
	}
	b.signingActive = true
	return report, nil
}

// UNKNOWN is synchronized/read back BEFORE a local signer receives unsigned
// bytes. Callbacks run without book locks. A panic/lost response/cancel stops,
// never signs again. Successful retained signed bytes are then atomically bound
// to the existing signed nonce entry; submission remains a separate operation.
func (b *CustodyNonceBook) SignReservedForSimulation(ctx context.Context, i CustodySigningInputs, output string, signer LocalCustodySimulationSigner) (inputs CustodyTransactionInputs, report CustodySigningReport, err error) {
	active := false
	defer func() {
		if recover() != nil {
			inputs, err = CustodyTransactionInputs{}, ErrCustodySigningUnknown
			report.SigningOutcomeUnknown = true
		}
		if active {
			b.mu.Lock()
			b.signingActive = false
			b.mu.Unlock()
		}
	}()
	if b == nil || ctx == nil || ctx.Err() != nil || signer == nil {
		return inputs, report, ErrCustodyNonceState
	}
	signCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	report, err = b.consumeSigning(signCtx, i, output)
	if err != nil {
		return inputs, report, err
	}
	active = true
	a, err := PrepareCustodySigningForSimulation(signCtx, i)
	if err != nil {
		return inputs, report, ErrCustodySigningUnknown
	}
	signed, local, err := a.SignForSimulation(signCtx, output, signer)
	local.DurableSigningAttempt, local.SharedNonceReserved = true, true
	if err != nil {
		local.SigningOutcomeUnknown = true
		return inputs, local, ErrCustodySigningUnknown
	}
	report, err = b.reconcileSigned(signCtx, signed, true)
	if err != nil {
		local.SigningOutcomeUnknown = true
		return inputs, local, err
	}
	return signed, report, nil
}

// Explicit exact signed-file recovery, never a signer call, file search or
// automatic adoption. Supply the original private file pin independently.
// UNKNOWN can bind only its original request/unsigned envelope/output/nonce and
// fee reservation. Repeated exact binding is read-only; uncertainty cannot
// release the slot or permit a fresh signer invocation.
func (b *CustodyNonceBook) ReconcileSignedForSimulation(ctx context.Context, i CustodyTransactionInputs) (report CustodySigningReport, err error) {
	return b.reconcileSigned(ctx, i, false)
}

func (b *CustodyNonceBook) reconcileSigned(ctx context.Context, i CustodyTransactionInputs, activeOwner bool) (report CustodySigningReport, err error) {
	if b == nil {
		return report, ErrCustodyNonceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.signingActive && !activeOwner {
		return report, ErrCustodySigningUnknown
	}
	if !b.ready(ctx) {
		return report, ErrCustodyNonceState
	}
	if err := b.recheck(ctx); err != nil {
		return report, err
	}
	wanted, err := b.signingCandidate(CustodySigningInputs{CallInputs: i.CallInputs, PlanPath: i.PlanPath, PlanSHA256: i.PlanSHA256}, i.TransactionPath)
	old := b.state.SigningIntent
	if err != nil || old == nil || old.Stage == custodySigningReserved || !sameCustodySigningIntent(*old, wanted) {
		return report, ErrCustodyNonceState
	}
	entry, tx, err := b.candidate(i)
	if err != nil || entry.Nonce != old.Nonce || entry.MaximumGasCost != old.MaximumGasCost || ctx.Err() != nil {
		return report, ErrCustodyNonceState
	}
	wanted.Stage, wanted.EnvelopeSHA256, wanted.TransactionHash = custodySigningRetained, entry.Binding.EnvelopeSHA256, entry.Binding.TransactionHash
	if old.Stage == custodySigningRetained {
		if *old != wanted {
			return report, ErrCustodyNonceState
		}
	} else {
		next := b.nextState()
		next.SigningIntent = &wanted
		next.Entries = append(next.Entries, entry)
		if _, err := encodeCustodyNonce(next); err != nil || !custodyNonceTransition(b.state, next) {
			return report, ErrCustodyNonceState
		}
		if err := b.write(next); err != nil {
			return report, err
		}
	}
	// The reviewer owns exact bytes; rechecking the file after retention also
	// guards against a concurrently changed private file before acknowledgment.
	again, err := loadCustodyTransaction(i)
	if err != nil || !bytes.Equal(again.raw, tx.raw) || ctx.Err() != nil {
		return report, ErrCustodySigningUnknown
	}
	return CustodySigningReport{SimulationOnly: true, ExactUnsignedEnvelopePrepared: true, LocalSigningAttemptConsumed: true,
		SignerResultMatched: true, PrivateEnvelopeRetained: true, DurableSigningAttempt: true, SharedNonceReserved: true}, nil
}
