package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"sync"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
)

const MaxNativeReturnSequenceEntries = 64
const MaxNativeReturnSequenceBytes = 768 << 10
const nativeReturnSequencePurpose = "GOD Chain shared simulation native return sequence v1"

var (
	ErrNativeReturnSequenceState   = errors.New("shared simulation native return sequence or original outbox rejected")
	ErrNativeReturnSequenceStorage = errors.New("shared simulation native return sequence storage uncertain; stop and independently review")
	ErrNativeReturnSequenceUnknown = errors.New("shared simulation native return already handed off; no retry, replacement or release")
)

// Explicit simulation seed/budget, NOT authenticated account state or an
// approved production fee policy. All cooperating return operations must use
// one exact book for this sender/binding. Other tools/stores are not fenced.
type NativeReturnSequencePlan struct {
	Sender                          string
	AccountNumber, StartingSequence uint64
	MaximumReservedFee              sdkmath.Int
}

func (NativeReturnSequencePlan) String() string {
	return "private native return sequence plan (redacted)"
}
func (p NativeReturnSequencePlan) GoString() string { return p.String() }
func (NativeReturnSequencePlan) MarshalJSON() ([]byte, error) {
	return []byte(`{"redacted":true}`), nil
}

type nativeReturnSequenceEntry struct {
	Attempt        nativeReturnAttemptState `json:"attempt"`
	WithdrawalID   string                   `json:"withdrawalId"`
	Sequence       string                   `json:"sequence"`
	MaximumFeeCost string                   `json:"maximumFeeCost"`
}
type nativeReturnSequenceState struct {
	Purpose            string                        `json:"purpose"`
	Version            uint32                        `json:"version"`
	Revision           uint32                        `json:"revision"`
	Previous           string                        `json:"previous"`
	BindingPin         string                        `json:"bindingPin"`
	Sender             string                        `json:"sender"`
	AccountNumber      string                        `json:"accountNumber"`
	StartingSequence   string                        `json:"startingSequence"`
	MaximumReservedFee string                        `json:"maximumReservedFee"`
	Entries            []nativeReturnSequenceEntry   `json:"entries"`
	Reviews            []nativeReturnExecutionRecord `json:"reviews,omitempty"`
}

// Storage is exclusive for the book's lifetime. Save must synchronously retain
// detached exact bytes; errors/panics may leave old/new/incomplete state. No
// callback reentrancy, auto-repair, independent writer or implicit repinning.
type NativeReturnSequenceStorage interface {
	Load() ([]byte, error)
	Save([]byte) error
	Close() error
}
type NativeReturnSequenceReport struct {
	Reservations                int  `json:"reservations"`
	UnknownDispatches           int  `json:"unknownDispatches"`
	ConditionalExecutionReviews int  `json:"conditionalExecutionReviews"`
	SimulationOnly              bool `json:"simulationOnly"`
	SharedJournalOnly           bool `json:"sharedJournalOnly"`
	StorageUsable               bool `json:"storageUsable"`
	WriteOutcomeUncertain       bool `json:"writeOutcomeUncertain"`
	AccountStateAuthenticated   bool `json:"accountStateAuthenticated"`
	SequenceProgressConfirmed   bool `json:"sequenceProgressConfirmed"`
	TransactionInclusionProven  bool `json:"transactionInclusionProven"`
	FeesReleased                bool `json:"feesReleased"`
	SigningEnabled              bool `json:"signingEnabled"`
	BroadcastEnabled            bool `json:"broadcastEnabled"`
	RealAssetsReady             bool `json:"realAssetsReady"`
}

// Copies share the same mutex/state. This is a cooperating sender-wide
// simulation journal, not a financial relayer or a distributed exclusion lock.
type NativeReturnSequenceBook struct{ *nativeReturnSequenceCore }
type nativeReturnSequenceCore struct {
	mu                        sync.Mutex
	store                     NativeReturnSequenceStorage
	state                     nativeReturnSequenceState
	usable, closed, uncertain bool
}

func (NativeReturnSequenceBook) String() string {
	return "private shared native return sequence (redacted)"
}
func (b NativeReturnSequenceBook) GoString() string             { return b.String() }
func (b NativeReturnSequenceBook) MarshalJSON() ([]byte, error) { return json.Marshal((&b).Report()) }
func (b *NativeReturnSequenceBook) Report() NativeReturnSequenceReport {
	r := NativeReturnSequenceReport{SimulationOnly: true, SharedJournalOnly: true}
	if b == nil || b.nativeReturnSequenceCore == nil {
		return r
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	r.Reservations = len(b.state.Entries)
	r.ConditionalExecutionReviews = len(b.state.Reviews)
	r.StorageUsable = b.usable && !b.closed
	r.WriteOutcomeUncertain = b.uncertain
	for _, e := range b.state.Entries {
		if e.Attempt.Stage == NativeReturnAttemptUnknown {
			r.UnknownDispatches++
		}
	}
	return r
}
func nativeSequenceCounter(s string) (uint64, error) {
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil || strconv.FormatUint(n, 10) != s {
		return 0, ErrNativeReturnSequenceState
	}
	return n, nil
}
func nativeSequenceBudget(s string) (*big.Int, error) {
	n, err := custodyNonceAmount(s)
	ceiling := new(big.Int).Mul(big.NewInt(MaxNativeResolutionFeeSmallestUnits), big.NewInt(MaxNativeReturnSequenceEntries))
	if err != nil || n.Cmp(ceiling) > 0 {
		return nil, ErrNativeReturnSequenceState
	}
	return n, nil
}
func nativeSequenceHeader(c godbridge.Config, p NativeReturnSequencePlan) (nativeReturnSequenceState, error) {
	c, err := godbridge.CanonicalConfig(c)
	if err != nil || p.MaximumReservedFee.IsNil() || p.StartingSequence == math.MaxUint64 {
		return nativeReturnSequenceState{}, ErrNativeReturnSequenceState
	}
	if _, err := godaddress.FromNative(p.Sender); err != nil {
		return nativeReturnSequenceState{}, ErrNativeReturnSequenceState
	}
	if _, err := nativeSequenceBudget(p.MaximumReservedFee.String()); err != nil {
		return nativeReturnSequenceState{}, err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return nativeReturnSequenceState{}, ErrNativeReturnSequenceState
	}
	pin := sha256.Sum256(raw)
	return nativeReturnSequenceState{Purpose: nativeReturnSequencePurpose, Version: 1, Revision: 1, BindingPin: hex.EncodeToString(pin[:]), Sender: p.Sender, AccountNumber: strconv.FormatUint(p.AccountNumber, 10), StartingSequence: strconv.FormatUint(p.StartingSequence, 10), MaximumReservedFee: p.MaximumReservedFee.String(), Entries: []nativeReturnSequenceEntry{}}, nil
}
func nativeSequenceSameHeader(a, b nativeReturnSequenceState) bool {
	return a.Purpose == b.Purpose && a.Version == b.Version && a.BindingPin == b.BindingPin && a.Sender == b.Sender && a.AccountNumber == b.AccountNumber && a.StartingSequence == b.StartingSequence && a.MaximumReservedFee == b.MaximumReservedFee
}
func encodeNativeReturnSequence(s nativeReturnSequenceState) ([]byte, error) {
	bad := func() ([]byte, error) { return nil, ErrNativeReturnSequenceState }
	if s.Purpose != nativeReturnSequencePurpose || s.Version != 1 || s.Entries == nil || len(s.Entries) > MaxNativeReturnSequenceEntries {
		return bad()
	}
	if _, err := SourceMaterialPin(s.BindingPin); err != nil {
		return bad()
	}
	if _, err := godaddress.FromNative(s.Sender); err != nil {
		return bad()
	}
	if _, err := nativeSequenceCounter(s.AccountNumber); err != nil {
		return bad()
	}
	start, err := nativeSequenceCounter(s.StartingSequence)
	if err != nil || start == math.MaxUint64 || uint64(len(s.Entries)) > math.MaxUint64-start {
		return bad()
	}
	budget, err := nativeSequenceBudget(s.MaximumReservedFee)
	if err != nil {
		return bad()
	}
	unknown := 0
	total := new(big.Int)
	withdrawals, outboxes, transactions := map[string]bool{}, map[string]bool{}, map[string]bool{}
	reservedSeen := false
	for index, e := range s.Entries {
		sequence, err := nativeSequenceCounter(e.Sequence)
		if err != nil || sequence != start+uint64(index) || sequence == math.MaxUint64 {
			return bad()
		}
		if _, err := SourceMaterialPin(e.WithdrawalID); err != nil {
			return bad()
		}
		if e.Attempt.BindingPin != s.BindingPin || (e.Attempt.Stage != NativeReturnAttemptSigned && e.Attempt.Stage != NativeReturnAttemptUnknown) {
			return bad()
		}
		if _, err := encodeNativeReturnAttempt(e.Attempt); err != nil {
			return bad()
		}
		if withdrawals[e.WithdrawalID] || outboxes[e.Attempt.OriginalOutboxPin] || transactions[e.Attempt.TransactionHash] {
			return bad()
		}
		withdrawals[e.WithdrawalID], outboxes[e.Attempt.OriginalOutboxPin], transactions[e.Attempt.TransactionHash] = true, true, true
		if e.Attempt.Stage == NativeReturnAttemptSigned {
			reservedSeen = true
		} else {
			if reservedSeen {
				return bad()
			}
			unknown++
		}
		cost, err := custodyNonceAmount(e.MaximumFeeCost)
		if err != nil || cost.Cmp(big.NewInt(MaxNativeResolutionFeeSmallestUnits)) > 0 {
			return bad()
		}
		total.Add(total, cost)
	}
	if !validNativeReturnExecutionRecords(s) || total.Cmp(budget) > 0 || s.Revision != uint32(1+len(s.Entries)+unknown+len(s.Reviews)) {
		return bad()
	}
	if s.Revision == 1 {
		if s.Previous != "" {
			return bad()
		}
	} else {
		if _, err := SourceMaterialPin(s.Previous); err != nil {
			return bad()
		}
	}
	raw, err := json.Marshal(s)
	if err != nil || len(raw) > MaxNativeReturnSequenceBytes {
		return bad()
	}
	return raw, nil
}
func decodeNativeReturnSequence(raw []byte) (nativeReturnSequenceState, error) {
	var s nativeReturnSequenceState
	if len(raw) == 0 || len(raw) > MaxNativeReturnSequenceBytes || json.Unmarshal(raw, &s) != nil {
		return s, ErrNativeReturnSequenceState
	}
	exact, err := encodeNativeReturnSequence(s)
	if err != nil || !bytes.Equal(raw, exact) {
		return s, ErrNativeReturnSequenceState
	}
	return s, nil
}

// Recovery needs the original immutable header and one independently retained
// latest state pin. No outbox ownership, signing, provider or automatic dispatch.
func NewNativeReturnSequenceBook(ctx context.Context, c godbridge.Config, p NativeReturnSequencePlan, store NativeReturnSequenceStorage, create bool, resumePin ...[32]byte) (book *NativeReturnSequenceBook, err error) {
	defer func() {
		if recover() != nil {
			book, err = nil, ErrNativeReturnSequenceStorage
		}
	}()
	if ctx == nil || ctx.Err() != nil || store == nil || !custodyAttemptResumeValid(create, resumePin) {
		return nil, ErrNativeReturnSequenceState
	}
	wanted, err := nativeSequenceHeader(c, p)
	if err != nil {
		return nil, err
	}
	raw, err := store.Load()
	if err != nil || len(raw) > MaxNativeReturnSequenceBytes {
		return nil, ErrNativeReturnSequenceStorage
	}
	if ctx.Err() != nil {
		return nil, ErrNativeReturnSequenceState
	}
	b := &NativeReturnSequenceBook{&nativeReturnSequenceCore{store: store, state: wanted, usable: true}}
	if create {
		if len(raw) != 0 {
			return nil, ErrNativeReturnSequenceState
		}
		if err = b.write(wanted); err != nil {
			return nil, err
		}
	} else {
		if sha256.Sum256(raw) != resumePin[0] {
			return nil, ErrNativeReturnSequenceState
		}
		s, err := decodeNativeReturnSequence(bytes.Clone(raw))
		if err != nil || !nativeSequenceSameHeader(s, wanted) {
			return nil, ErrNativeReturnSequenceState
		}
		b.state = s
	}
	return b, nil
}
func (b *NativeReturnSequenceBook) write(s nativeReturnSequenceState) (err error) {
	raw, err := encodeNativeReturnSequence(s)
	if err != nil {
		return err
	}
	defer func() {
		if recover() != nil {
			err = ErrNativeReturnSequenceStorage
		}
		if err != nil {
			b.usable, b.uncertain = false, true
		}
	}()
	if b.store.Save(bytes.Clone(raw)) != nil {
		return ErrNativeReturnSequenceStorage
	}
	retained, err := b.store.Load()
	if err != nil || !bytes.Equal(raw, retained) {
		return ErrNativeReturnSequenceStorage
	}
	b.state = s
	return nil
}
func (b *NativeReturnSequenceBook) check(ctx context.Context) (err error) {
	if ctx == nil || ctx.Err() != nil || !b.usable || b.closed {
		return ErrNativeReturnSequenceState
	}
	defer func() {
		if recover() != nil {
			err = ErrNativeReturnSequenceStorage
		}
		if err != nil {
			b.usable = false
		}
	}()
	raw, err := b.store.Load()
	exact, e := encodeNativeReturnSequence(b.state)
	if err != nil || e != nil || !bytes.Equal(raw, exact) {
		return ErrNativeReturnSequenceStorage
	}
	if ctx.Err() != nil {
		return ErrNativeReturnSequenceState
	}
	return nil
}
func (b *NativeReturnSequenceBook) next() nativeReturnSequenceState {
	next := b.state
	next.Entries = append([]nativeReturnSequenceEntry{}, b.state.Entries...)
	next.Reviews = append([]nativeReturnExecutionRecord{}, b.state.Reviews...)
	raw, _ := encodeNativeReturnSequence(b.state)
	pin := sha256.Sum256(raw)
	next.Previous = hex.EncodeToString(pin[:])
	next.Revision++
	return next
}

// Locks are book -> original outbox -> original journal. Reuses the complete
// checked envelope and original consumed-outbox correspondence, not supplied IDs.
func (b *NativeReturnSequenceBook) candidate(ctx context.Context, p *ReturnSettlementOutbox, t NativeResolutionTransaction) (nativeReturnSequenceEntry, NativeResolutionTransaction, error) {
	return b.executionCandidate(ctx, p, t, false)
}

func (b *NativeReturnSequenceBook) executionCandidate(ctx context.Context, p *ReturnSettlementOutbox, t NativeResolutionTransaction, allowMatched bool) (nativeReturnSequenceEntry, NativeResolutionTransaction, error) {
	bad := func() (nativeReturnSequenceEntry, NativeResolutionTransaction, error) {
		return nativeReturnSequenceEntry{}, NativeResolutionTransaction{}, ErrNativeReturnSequenceState
	}
	if p == nil || p.journal == nil {
		return bad()
	}
	if reflect.TypeOf(b.store).Comparable() && (any(b.store) == any(p.store) || any(b.store) == any(p.journal.store)) {
		return bad()
	}
	t, err := ReviewNativeResolutionTransaction(t.binding, t.request, t.plan, t.raw)
	if err != nil {
		return bad()
	}
	link, err := nativeReturnOutboxLink(ctx, p, t)
	if err != nil || (p.state.Stage != ReturnOutboxUnknown && !(allowMatched && p.state.Stage == ReturnOutboxMatched)) || t.request.Sender != b.state.Sender || hex.EncodeToString(t.bindingPin[:]) != b.state.BindingPin || strconv.FormatUint(t.plan.AccountNumber, 10) != b.state.AccountNumber {
		return bad()
	}
	hexPin := func(v [32]byte) string { return hex.EncodeToString(v[:]) }
	s := nativeReturnAttemptState{Purpose: nativeReturnAttemptPurpose, Version: 1, Stage: NativeReturnAttemptSigned, Revision: 1, BindingPin: hexPin(t.bindingPin), RequestPin: hexPin(t.requestPin), PlanPin: hexPin(t.planPin), OriginalOutboxPin: hexPin(link), TransactionHash: hexPin(t.hash), Envelope: hex.EncodeToString(t.raw)}
	return nativeReturnSequenceEntry{Attempt: s, WithdrawalID: hexPin(t.request.Withdrawal.ID), Sequence: strconv.FormatUint(t.plan.Sequence, 10), MaximumFeeCost: t.plan.MaximumFeeCost.String()}, t, nil
}
func nativeSequenceSameEntry(a, b nativeReturnSequenceEntry) bool {
	a.Attempt = nativeAttemptBase(a.Attempt)
	b.Attempt = nativeAttemptBase(b.Attempt)
	return a == b
}
func (b *NativeReturnSequenceBook) NextSequence(ctx context.Context) (uint64, error) {
	if b == nil || b.nativeReturnSequenceCore == nil {
		return 0, ErrNativeReturnSequenceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.check(ctx); err != nil {
		return 0, err
	}
	start, _ := nativeSequenceCounter(b.state.StartingSequence)
	n := start + uint64(len(b.state.Entries))
	if len(b.state.Entries) == MaxNativeReturnSequenceEntries || n == math.MaxUint64 {
		return 0, ErrNativeReturnSequenceState
	}
	return n, nil
}

// Reserves one consecutive sequence and maximum fee for the exact original
// withdrawal. No gap skipping, repricing, conflicting outcome or fee recycling.
func (b *NativeReturnSequenceBook) Reserve(ctx context.Context, p *ReturnSettlementOutbox, t NativeResolutionTransaction) error {
	if b == nil || b.nativeReturnSequenceCore == nil || p == nil || p.journal == nil {
		return ErrNativeReturnSequenceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.journal.mu.Lock()
	defer p.journal.mu.Unlock()
	if err := b.check(ctx); err != nil {
		return err
	}
	wanted, _, err := b.candidate(ctx, p, t)
	if err != nil {
		return err
	}
	for _, e := range b.state.Entries {
		if nativeSequenceSameEntry(e, wanted) {
			return nil
		}
		if e.WithdrawalID == wanted.WithdrawalID || e.Attempt.OriginalOutboxPin == wanted.Attempt.OriginalOutboxPin || e.Attempt.TransactionHash == wanted.Attempt.TransactionHash {
			return ErrNativeReturnSequenceState
		}
	}
	start, _ := nativeSequenceCounter(b.state.StartingSequence)
	if len(b.state.Entries) == MaxNativeReturnSequenceEntries || wanted.Sequence != strconv.FormatUint(start+uint64(len(b.state.Entries)), 10) || ctx.Err() != nil {
		return ErrNativeReturnSequenceState
	}
	next := b.next()
	next.Entries = append(next.Entries, wanted)
	return b.write(next)
}

// Exact already-signed bytes once, in sequence order, ONLY for a supplied local
// simulator. UNKNOWN is durable before bytes escape. Earlier UNKNOWN does NOT
// prove sequence consumption; later envelopes may stall. No live broadcast API.
func (b *NativeReturnSequenceBook) TakeForSimulation(ctx context.Context, p *ReturnSettlementOutbox, t NativeResolutionTransaction) ([]byte, error) {
	if b == nil || b.nativeReturnSequenceCore == nil || p == nil || p.journal == nil {
		return nil, ErrNativeReturnSequenceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.journal.mu.Lock()
	defer p.journal.mu.Unlock()
	if err := b.check(ctx); err != nil {
		return nil, err
	}
	wanted, checked, err := b.candidate(ctx, p, t)
	if err != nil {
		return nil, err
	}
	for i, e := range b.state.Entries {
		if !nativeSequenceSameEntry(e, wanted) {
			continue
		}
		if e.Attempt.Stage == NativeReturnAttemptUnknown {
			return nil, ErrNativeReturnSequenceUnknown
		}
		for _, prior := range b.state.Entries[:i] {
			if prior.Attempt.Stage != NativeReturnAttemptUnknown {
				return nil, ErrNativeReturnSequenceState
			}
		}
		if ctx.Err() != nil {
			return nil, ErrNativeReturnSequenceState
		}
		next := b.next()
		a := &next.Entries[i].Attempt
		a.Stage, a.Revision, a.Previous = NativeReturnAttemptUnknown, 2, nativeAttemptHash(e.Attempt)
		if err := b.write(next); err != nil {
			return nil, err
		}
		if ctx.Err() != nil {
			return nil, ErrNativeReturnSequenceUnknown
		}
		return bytes.Clone(checked.raw), nil
	}
	return nil, ErrNativeReturnSequenceState
}
func (b *NativeReturnSequenceBook) RetainedStatePin(ctx context.Context) ([32]byte, error) {
	if b == nil || b.nativeReturnSequenceCore == nil {
		return [32]byte{}, ErrNativeReturnSequenceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.check(ctx); err != nil {
		return [32]byte{}, err
	}
	raw, err := encodeNativeReturnSequence(b.state)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(raw), nil
}
func (b *NativeReturnSequenceBook) Close() (err error) {
	if b == nil || b.nativeReturnSequenceCore == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed, b.usable = true, false
	defer func() {
		if recover() != nil {
			err = ErrNativeReturnSequenceStorage
		}
	}()
	if b.store.Close() != nil {
		return ErrNativeReturnSequenceStorage
	}
	return nil
}
