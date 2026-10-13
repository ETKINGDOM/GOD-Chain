package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
)

const MaxNativeReturnAttemptBytes = 16 << 10
const nativeReturnAttemptPurpose = "GOD Chain private simulation native return attempt v1"

var (
	ErrNativeReturnAttemptState   = errors.New("simulation native return attempt or original outbox binding rejected")
	ErrNativeReturnAttemptStorage = errors.New("simulation native return attempt storage unavailable or uncertain; stop and independently review")
	ErrNativeReturnAttemptUnknown = errors.New("simulation native return dispatch consumed or uncertain; no retry, replacement or sequence release")
)

type NativeReturnAttemptStage string

const (
	NativeReturnAttemptSigned  NativeReturnAttemptStage = "signed_retained_simulation"
	NativeReturnAttemptUnknown NativeReturnAttemptStage = "dispatch_unknown_simulation"
	NativeReturnAttemptMatched NativeReturnAttemptStage = "native_attestation_matched_simulation"
)

// One exclusively owned simulation slot. Save must synchronously and atomically
// retain detached bytes. Failure/panic may leave old, new or incomplete state.
// Load is bounded. Callbacks cannot reenter attempt/outbox/journal APIs.
type NativeReturnAttemptStorage interface {
	Load() ([]byte, error)
	Save([]byte) error
	Close() error
}

type nativeReturnAttemptState struct {
	Purpose           string                   `json:"purpose"`
	Version           uint32                   `json:"version"`
	Stage             NativeReturnAttemptStage `json:"stage"`
	Revision          uint32                   `json:"revision"`
	Previous          string                   `json:"previous"`
	BindingPin        string                   `json:"bindingPin"`
	RequestPin        string                   `json:"requestPin"`
	PlanPin           string                   `json:"planPin"`
	OriginalOutboxPin string                   `json:"originalOutboxPin"`
	TransactionHash   string                   `json:"transactionHash"`
	Envelope          string                   `json:"envelope"`
	MatchedOutboxPin  string                   `json:"matchedOutboxPin"`
}

type NativeReturnAttemptReport struct {
	Transaction                NativeResolutionTransactionReport `json:"transaction"`
	Stage                      NativeReturnAttemptStage          `json:"stage"`
	SimulationOnly             bool                              `json:"simulationOnly"`
	StorageUsable              bool                              `json:"storageUsable"`
	SignedEnvelopeRetained     bool                              `json:"signedEnvelopeRetained"`
	DispatchOutcomeUnknown     bool                              `json:"dispatchOutcomeUnknown"`
	NativeAttestationRetained  bool                              `json:"nativeAttestationRetained"`
	NeedsManualReview          bool                              `json:"needsManualReview"`
	WriteOutcomeUncertain      bool                              `json:"writeOutcomeUncertain"`
	RetryAllowed               bool                              `json:"retryAllowed"`
	SequenceReleased           bool                              `json:"sequenceReleased"`
	TransactionInclusionProven bool                              `json:"transactionInclusionProven"`
	NativeStateProven          bool                              `json:"nativeStateProven"`
	PaymentVerified            bool                              `json:"paymentVerified"`
	RefundAuthorized           bool                              `json:"refundAuthorized"`
	SigningEnabled             bool                              `json:"signingEnabled"`
	BroadcastEnabled           bool                              `json:"broadcastEnabled"`
	RealAssetsReady            bool                              `json:"realAssetsReady"`
}

// Retains already signed bytes after an original consumed simulation outbox.
// It has no signer, sender, provider, financial retry or sequence allocator.
// It fences THIS slot only, not different stores or rollback of store+pin.
type NativeReturnAttempt struct {
	*nativeReturnAttemptCore
}

// A copy of the public handle shares the same mutex/state rather than copying
// a mutex or losing value-format redaction. Only the constructor creates a core.
type nativeReturnAttemptCore struct {
	mu                        sync.Mutex
	outbox                    *ReturnSettlementOutbox
	transaction               NativeResolutionTransaction
	store                     NativeReturnAttemptStorage
	state                     nativeReturnAttemptState
	usable, closed, uncertain bool
}

func (NativeReturnAttempt) String() string {
	return "private simulation native return attempt (redacted)"
}
func (a NativeReturnAttempt) GoString() string             { return a.String() }
func (a NativeReturnAttempt) MarshalJSON() ([]byte, error) { return json.Marshal((&a).Report()) }
func (a *NativeReturnAttempt) Report() NativeReturnAttemptReport {
	if a == nil || a.nativeReturnAttemptCore == nil {
		return NativeReturnAttemptReport{SimulationOnly: true}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	usable := a.usable && !a.closed
	return NativeReturnAttemptReport{Transaction: a.transaction.Report(), Stage: a.state.Stage, SimulationOnly: true,
		StorageUsable: usable, SignedEnvelopeRetained: usable, DispatchOutcomeUnknown: a.state.Stage == NativeReturnAttemptUnknown || a.uncertain,
		NativeAttestationRetained: usable && a.state.Stage == NativeReturnAttemptMatched,
		NeedsManualReview:         !usable || a.uncertain || a.state.Stage == NativeReturnAttemptUnknown, WriteOutcomeUncertain: a.uncertain}
}

func nativeAttemptBase(s nativeReturnAttemptState) nativeReturnAttemptState {
	s.Stage, s.Revision, s.Previous, s.MatchedOutboxPin = NativeReturnAttemptSigned, 1, "", ""
	return s
}
func nativeAttemptHash(s nativeReturnAttemptState) string {
	raw, _ := json.Marshal(s)
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func encodeNativeReturnAttempt(s nativeReturnAttemptState) ([]byte, error) {
	bad := func() ([]byte, error) { return nil, ErrNativeReturnAttemptState }
	if s.Purpose != nativeReturnAttemptPurpose || s.Version != 1 || len(s.Envelope) == 0 || len(s.Envelope) > 2*MaxNativeResolutionTransactionBytes {
		return bad()
	}
	for _, pin := range []string{s.BindingPin, s.RequestPin, s.PlanPin, s.OriginalOutboxPin, s.TransactionHash} {
		if _, err := SourceMaterialPin(pin); err != nil {
			return bad()
		}
	}
	wire, err := hex.DecodeString(s.Envelope)
	if err != nil || hex.EncodeToString(wire) != s.Envelope {
		return bad()
	}
	hash := sha256.Sum256(wire)
	if hex.EncodeToString(hash[:]) != s.TransactionHash {
		return bad()
	}
	signed := nativeAttemptBase(s)
	unknown := signed
	unknown.Stage, unknown.Revision, unknown.Previous = NativeReturnAttemptUnknown, 2, nativeAttemptHash(signed)
	switch s.Stage {
	case NativeReturnAttemptSigned:
		if s != signed {
			return bad()
		}
	case NativeReturnAttemptUnknown:
		if s != unknown {
			return bad()
		}
	case NativeReturnAttemptMatched:
		if s.Revision != 3 || s.Previous != nativeAttemptHash(unknown) {
			return bad()
		}
		if _, err := SourceMaterialPin(s.MatchedOutboxPin); err != nil {
			return bad()
		}
	default:
		return bad()
	}
	raw, err := json.Marshal(s)
	if err != nil || len(raw) > MaxNativeReturnAttemptBytes {
		return bad()
	}
	return raw, nil
}
func decodeNativeReturnAttempt(raw []byte) (nativeReturnAttemptState, error) {
	var s nativeReturnAttemptState
	if len(raw) == 0 || len(raw) > MaxNativeReturnAttemptBytes || json.Unmarshal(raw, &s) != nil {
		return nativeReturnAttemptState{}, ErrNativeReturnAttemptState
	}
	exact, err := encodeNativeReturnAttempt(s)
	if err != nil || !bytes.Equal(raw, exact) {
		return nativeReturnAttemptState{}, ErrNativeReturnAttemptState
	}
	return s, nil
}

// Caller holds outbox then journal lock. No provider is consulted. Match means
// agreement with retained operator views, NOT inclusion of this transaction.
func nativeReturnOutboxLink(ctx context.Context, p *ReturnSettlementOutbox, t NativeResolutionTransaction) (pin [32]byte, err error) {
	defer func() {
		if recover() != nil {
			pin, err = [32]byte{}, ErrNativeReturnAttemptState
		}
	}()
	if p == nil || p.journal == nil || p.journal.config.private.Mode != "simulation" || p.guard(ctx) != nil ||
		(p.state.Stage != ReturnOutboxUnknown && p.state.Stage != ReturnOutboxMatched) {
		return pin, ErrNativeReturnAttemptState
	}
	binding, err := p.journal.config.Binding()
	if err != nil || binding != t.binding {
		return pin, ErrNativeReturnAttemptState
	}
	task := p.journal.state.Tasks[p.ticket-1]
	if task.Request.Withdrawal == nil || !sameWithdrawal(*task.Request.Withdrawal, t.request.Withdrawal) ||
		p.kind != string(t.request.Outcome) || p.evidence() != t.request.Evidence {
		return pin, ErrNativeReturnAttemptState
	}
	if p.state.Stage == ReturnOutboxMatched {
		return p.state.Previous, nil
	}
	raw, err := encodeReturnOutbox(p.state)
	if err != nil {
		return pin, ErrNativeReturnAttemptState
	}
	return sha256.Sum256(raw), nil
}

// Reopen needs one independently retained latest attempt pin and the original
// reviewed signed inputs. It writes nothing, does not sign/fetch/re-dispatch,
// and does not own/close the outbox or journal. Success alone owns store.Close.
func NewNativeReturnAttempt(ctx context.Context, outbox *ReturnSettlementOutbox, transaction NativeResolutionTransaction,
	store NativeReturnAttemptStorage, create bool, resumePin ...[32]byte) (out *NativeReturnAttempt, err error) {
	defer func() {
		if recover() != nil {
			out, err = nil, ErrNativeReturnAttemptStorage
		}
	}()
	if ctx == nil || ctx.Err() != nil || outbox == nil || outbox.journal == nil || store == nil || !custodyAttemptResumeValid(create, resumePin) {
		return nil, ErrNativeReturnAttemptState
	}
	outbox.mu.Lock()
	defer outbox.mu.Unlock()
	outbox.journal.mu.Lock()
	defer outbox.journal.mu.Unlock()
	if reflect.TypeOf(store).Comparable() && (any(store) == any(outbox.store) || any(store) == any(outbox.journal.store)) {
		return nil, ErrNativeReturnAttemptState
	}
	transaction, err = ReviewNativeResolutionTransaction(transaction.binding, transaction.request, transaction.plan, transaction.raw)
	if err != nil {
		return nil, ErrNativeReturnAttemptState
	}
	link, err := nativeReturnOutboxLink(ctx, outbox, transaction)
	if err != nil || create && outbox.state.Stage != ReturnOutboxUnknown {
		return nil, ErrNativeReturnAttemptState
	}
	encode := func(v [32]byte) string { return hex.EncodeToString(v[:]) }
	s := nativeReturnAttemptState{Purpose: nativeReturnAttemptPurpose, Version: 1, Stage: NativeReturnAttemptSigned, Revision: 1,
		BindingPin: encode(transaction.bindingPin), RequestPin: encode(transaction.requestPin), PlanPin: encode(transaction.planPin),
		OriginalOutboxPin: encode(link), TransactionHash: encode(transaction.hash), Envelope: hex.EncodeToString(transaction.raw)}
	raw, err := store.Load()
	if err != nil || len(raw) > MaxNativeReturnAttemptBytes {
		return nil, ErrNativeReturnAttemptStorage
	}
	if ctx.Err() != nil {
		return nil, ErrNativeReturnAttemptState
	}
	a := &NativeReturnAttempt{&nativeReturnAttemptCore{outbox: outbox, transaction: transaction, store: store, state: s, usable: true}}
	if create {
		if len(raw) != 0 {
			return nil, ErrNativeReturnAttemptState
		}
		if a.write(s) != nil {
			return nil, ErrNativeReturnAttemptStorage
		}
	} else {
		if sha256.Sum256(raw) != resumePin[0] {
			return nil, ErrNativeReturnAttemptState
		}
		saved, err := decodeNativeReturnAttempt(bytes.Clone(raw))
		if err != nil || nativeAttemptBase(saved) != s {
			return nil, ErrNativeReturnAttemptState
		}
		if saved.Stage == NativeReturnAttemptMatched && (outbox.state.Stage != ReturnOutboxMatched || saved.MatchedOutboxPin != nativeAttemptHashOfOutbox(outbox)) {
			return nil, ErrNativeReturnAttemptState
		}
		a.state = saved
	}
	return a, nil
}
func nativeAttemptHashOfOutbox(p *ReturnSettlementOutbox) string {
	raw, _ := encodeReturnOutbox(p.state)
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}
func (a *NativeReturnAttempt) write(s nativeReturnAttemptState) (err error) {
	raw, err := encodeNativeReturnAttempt(s)
	if err != nil {
		return err
	}
	defer func() {
		if recover() != nil {
			err = ErrNativeReturnAttemptStorage
		}
		if err != nil {
			a.usable, a.uncertain = false, true
		}
	}()
	if a.store.Save(bytes.Clone(raw)) != nil {
		return ErrNativeReturnAttemptStorage
	}
	retained, err := a.store.Load()
	if err != nil || !bytes.Equal(raw, retained) {
		return ErrNativeReturnAttemptStorage
	}
	a.state = s
	return nil
}
func (a *NativeReturnAttempt) checkStored(ctx context.Context) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrNativeReturnAttemptStorage
		}
		if err != nil {
			a.usable = false
		}
	}()
	raw, err := a.store.Load()
	if err != nil || len(raw) > MaxNativeReturnAttemptBytes {
		return ErrNativeReturnAttemptStorage
	}
	exact, err := encodeNativeReturnAttempt(a.state)
	if err != nil || !bytes.Equal(exact, raw) {
		return ErrNativeReturnAttemptStorage
	}
	if ctx.Err() != nil {
		return ErrNativeReturnAttemptState
	}
	return nil
}

// Caller holds all three locks in order: attempt, outbox, journal.
func (a *NativeReturnAttempt) guard(ctx context.Context) error {
	if !a.usable || a.closed || ctx == nil || ctx.Err() != nil {
		return ErrNativeReturnAttemptState
	}
	if err := a.checkStored(ctx); err != nil {
		return err
	}
	t, err := ReviewNativeResolutionTransaction(a.transaction.binding, a.transaction.request, a.transaction.plan, a.transaction.raw)
	if err != nil || t.hash != a.transaction.hash || t.bindingPin != a.transaction.bindingPin || t.requestPin != a.transaction.requestPin || t.planPin != a.transaction.planPin {
		return ErrNativeReturnAttemptState
	}
	pin, err := nativeReturnOutboxLink(ctx, a.outbox, t)
	if err != nil || hex.EncodeToString(pin[:]) != a.state.OriginalOutboxPin {
		return ErrNativeReturnAttemptState
	}
	if a.state.Stage == NativeReturnAttemptMatched && (a.outbox.state.Stage != ReturnOutboxMatched || nativeAttemptHashOfOutbox(a.outbox) != a.state.MatchedOutboxPin) {
		return ErrNativeReturnAttemptState
	}
	return nil
}

// Exact signed bytes ONCE, for an explicitly supplied LOCAL simulator. UNKNOWN
// is durably saved before exposure. A lost acknowledgment/cancellation after
// Save never allows another take, replacement or sequence release in this slot.
func (a *NativeReturnAttempt) TakeForSimulation(ctx context.Context) ([]byte, error) {
	if a == nil || a.nativeReturnAttemptCore == nil || ctx == nil {
		return nil, ErrNativeReturnAttemptState
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.outbox.mu.Lock()
	defer a.outbox.mu.Unlock()
	a.outbox.journal.mu.Lock()
	defer a.outbox.journal.mu.Unlock()
	if err := a.guard(ctx); err != nil {
		return nil, err
	}
	if a.state.Stage != NativeReturnAttemptSigned || a.outbox.state.Stage == ReturnOutboxMatched {
		return nil, ErrNativeReturnAttemptUnknown
	}
	next := a.state
	next.Stage, next.Revision, next.Previous = NativeReturnAttemptUnknown, 2, nativeAttemptHash(a.state)
	if ctx.Err() != nil {
		return nil, ErrNativeReturnAttemptState
	}
	if err := a.write(next); err != nil {
		return nil, err
	}
	if ctx.Err() != nil {
		return nil, ErrNativeReturnAttemptUnknown
	}
	return bytes.Clone(a.transaction.raw), nil
}

// Retain only the exact original outbox's existing terminal match. No provider
// read, financial transition or claim that THIS outer transaction was included.
func (a *NativeReturnAttempt) MatchOutboxForSimulation(ctx context.Context) (NativeReturnAttemptReport, error) {
	if a == nil || a.nativeReturnAttemptCore == nil || ctx == nil {
		return NativeReturnAttemptReport{}, ErrNativeReturnAttemptState
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.outbox.mu.Lock()
	defer a.outbox.mu.Unlock()
	a.outbox.journal.mu.Lock()
	defer a.outbox.journal.mu.Unlock()
	if err := a.guard(ctx); err != nil {
		return NativeReturnAttemptReport{}, err
	}
	if a.outbox.state.Stage != ReturnOutboxMatched || a.state.Stage == NativeReturnAttemptSigned {
		return NativeReturnAttemptReport{}, ErrNativeReturnAttemptState
	}
	if a.state.Stage == NativeReturnAttemptUnknown {
		next := a.state
		next.Stage, next.Revision, next.Previous = NativeReturnAttemptMatched, 3, nativeAttemptHash(a.state)
		next.MatchedOutboxPin = nativeAttemptHashOfOutbox(a.outbox)
		if ctx.Err() != nil {
			return NativeReturnAttemptReport{}, ErrNativeReturnAttemptState
		}
		if err := a.write(next); err != nil {
			return NativeReturnAttemptReport{}, err
		}
	}
	// Report without relocking this mutex.
	return NativeReturnAttemptReport{Transaction: a.transaction.Report(), Stage: a.state.Stage, SimulationOnly: true,
		StorageUsable: true, SignedEnvelopeRetained: true, NativeAttestationRetained: true}, nil
}

func (a *NativeReturnAttempt) RetainedStatePin(ctx context.Context) ([32]byte, error) {
	if a == nil || a.nativeReturnAttemptCore == nil || ctx == nil {
		return [32]byte{}, ErrNativeReturnAttemptState
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable || a.closed || ctx.Err() != nil {
		return [32]byte{}, ErrNativeReturnAttemptState
	}
	if err := a.checkStored(ctx); err != nil {
		return [32]byte{}, err
	}
	raw, err := encodeNativeReturnAttempt(a.state)
	if err != nil || ctx.Err() != nil {
		return [32]byte{}, ErrNativeReturnAttemptState
	}
	return sha256.Sum256(raw), nil
}

func (a *NativeReturnAttempt) Close() (err error) {
	if a == nil || a.nativeReturnAttemptCore == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed, a.usable = true, false
	defer func() {
		if recover() != nil {
			err = ErrNativeReturnAttemptStorage
		}
		if err != nil {
			a.uncertain = true
		}
	}()
	if a.store.Close() != nil {
		return ErrNativeReturnAttemptStorage
	}
	return nil
}
