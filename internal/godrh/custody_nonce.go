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
	"strconv"
	"sync"
	"unicode/utf8"
)

// Finite simulation journal, not a production account-state or fee oracle.
const MaxCustodyNonceEntries = 64
const MaxCustodyNonceBytes = 64 << 10
const custodyNoncePurpose = "GOD Chain shared simulation nonce journal v1"

var (
	ErrCustodyNonceState   = errors.New("RH shared simulation nonce binding or reservation rejected")
	ErrCustodyNonceStorage = errors.New("RH shared simulation nonce storage uncertain; stop and independently review")
	ErrCustodyNonceUnknown = errors.New("RH shared simulation nonce already handed off; no retry or release")
)

type CustodyNonceInputs struct {
	ConfigurationPath   string
	ConfigurationSHA256 [32]byte
	AccountPlanPath     string
	AccountPlanSHA256   [32]byte
}

func (CustodyNonceInputs) String() string               { return "RH shared simulation nonce inputs (redacted)" }
func (i CustodyNonceInputs) GoString() string           { return i.String() }
func (CustodyNonceInputs) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type custodyNoncePlan struct {
	sender [20]byte
	start  uint64
	budget *big.Int
}

func custodyNonceCounter(text string) (uint64, error) {
	n, err := strconv.ParseUint(text, 10, 64)
	if err != nil || len(text) > 20 || strconv.FormatUint(n, 10) != text || n == math.MaxUint64 {
		return 0, ErrCustodyNonceState
	}
	return n, nil
}
func custodyNonceAmount(text string) (*big.Int, error) {
	if len(text) == 0 || len(text) > 78 || text[0] == '0' {
		return nil, ErrCustodyNonceState
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return nil, ErrCustodyNonceState
		}
	}
	n, ok := new(big.Int).SetString(text, 10)
	if !ok || n.Sign() <= 0 || n.BitLen() > 256 {
		return nil, ErrCustodyNonceState
	}
	return n, nil
}
func parseCustodyNoncePlan(raw []byte) (custodyNoncePlan, error) {
	bad := func() (custodyNoncePlan, error) { return custodyNoncePlan{}, ErrCustodyNonceState }
	if len(raw) == 0 || len(raw) > 4096 || !utf8.Valid(raw) {
		return bad()
	}
	f, err := custodyObject(raw, []string{"version", "sender", "startingNonce", "maximumReservedGasCost"}, nil)
	if err != nil || !bytes.Equal(bytes.TrimSpace(f["version"]), []byte("1")) {
		return bad()
	}
	sender, err := custodyHex(f["sender"], 20)
	if err != nil {
		return bad()
	}
	var p custodyNoncePlan
	copy(p.sender[:], sender)
	start, err := custodyText(f["startingNonce"])
	if err != nil {
		return bad()
	}
	p.start, err = custodyNonceCounter(start)
	if err != nil {
		return bad()
	}
	budget, err := custodyText(f["maximumReservedGasCost"])
	if err != nil {
		return bad()
	}
	p.budget, err = custodyNonceAmount(budget)
	if err != nil || p.sender == [20]byte{} {
		return bad()
	}
	return p, nil
}

type custodyNonceEntry struct {
	Binding        custodyAttemptState `json:"binding"`
	Nonce          string              `json:"nonce"`
	MaximumGasCost string              `json:"maximumGasCost"`
}
type custodyNonceState struct {
	Purpose                string               `json:"purpose"`
	Version                uint32               `json:"version"`
	Sequence               uint32               `json:"sequence"`
	PreviousSHA256         string               `json:"previousSHA256"`
	ConfigurationSHA256    string               `json:"configurationSHA256"`
	AccountPlanSHA256      string               `json:"accountPlanSHA256"`
	StartingNonce          string               `json:"startingNonce"`
	MaximumReservedGasCost string               `json:"maximumReservedGasCost"`
	Entries                []custodyNonceEntry  `json:"entries"`
	Reviews                []custodyNonceReview `json:"reviews,omitempty"`
}

func encodeCustodyNonce(s custodyNonceState) ([]byte, error) {
	if s.Purpose != custodyNoncePurpose || s.Version != 1 || s.Entries == nil || len(s.Entries) > MaxCustodyNonceEntries {
		return nil, ErrCustodyNonceState
	}
	unknown := 0
	for _, e := range s.Entries {
		if e.Binding.Stage == CustodyAttemptUnknown {
			unknown++
		}
	}
	if len(s.Reviews) > len(s.Entries) || s.Sequence != uint32(1+len(s.Entries)+unknown+len(s.Reviews)) {
		return nil, ErrCustodyNonceState
	}
	if s.Sequence == 1 {
		if s.PreviousSHA256 != "" {
			return nil, ErrCustodyNonceState
		}
	} else if _, err := SourceMaterialPin(s.PreviousSHA256); err != nil {
		return nil, ErrCustodyNonceState
	}
	for _, p := range []string{s.ConfigurationSHA256, s.AccountPlanSHA256} {
		if _, err := SourceMaterialPin(p); err != nil {
			return nil, ErrCustodyNonceState
		}
	}
	start, err := custodyNonceCounter(s.StartingNonce)
	if err != nil || uint64(len(s.Entries)) > math.MaxUint64-start {
		return nil, ErrCustodyNonceState
	}
	budget, err := custodyNonceAmount(s.MaximumReservedGasCost)
	if err != nil {
		return nil, ErrCustodyNonceState
	}
	total := new(big.Int)
	requests, envelopes, hashes := map[string]bool{}, map[string]bool{}, map[string]bool{}
	reservedSeen := false
	for index, e := range s.Entries {
		n, err := custodyNonceCounter(e.Nonce)
		if err != nil || n != start+uint64(index) || e.Binding.ConfigurationSHA256 != s.ConfigurationSHA256 {
			return nil, ErrCustodyNonceState
		}
		if _, err := encodeCustodyAttempt(e.Binding); err != nil {
			return nil, ErrCustodyNonceState
		}
		if requests[e.Binding.RequestSHA256] || envelopes[e.Binding.EnvelopeSHA256] || hashes[e.Binding.TransactionHash] {
			return nil, ErrCustodyNonceState
		}
		requests[e.Binding.RequestSHA256], envelopes[e.Binding.EnvelopeSHA256], hashes[e.Binding.TransactionHash] = true, true, true
		if e.Binding.Stage == CustodyAttemptReserved {
			reservedSeen = true
		} else if reservedSeen {
			return nil, ErrCustodyNonceState
		}
		cost, err := custodyNonceAmount(e.MaximumGasCost)
		if err != nil {
			return nil, ErrCustodyNonceState
		}
		total.Add(total, cost)
	}
	if total.Cmp(budget) > 0 {
		return nil, ErrCustodyNonceState
	}
	seenReviews := map[string]bool{}
	for _, r := range s.Reviews {
		if _, err := SourceMaterialPin(r.MaterialSHA256); err != nil || seenReviews[r.TransactionHash] {
			return nil, ErrCustodyNonceState
		}
		matched := false
		for _, e := range s.Entries {
			if e.Binding.TransactionHash == r.TransactionHash && e.Binding.Stage == CustodyAttemptUnknown {
				matched = true
			}
		}
		if !matched {
			return nil, ErrCustodyNonceState
		}
		seenReviews[r.TransactionHash] = true
	}
	raw, err := json.Marshal(s)
	if err != nil || len(raw) > MaxCustodyNonceBytes {
		return nil, ErrCustodyNonceState
	}
	return raw, nil
}
func decodeCustodyNonce(raw []byte) (custodyNonceState, error) {
	var s custodyNonceState
	if len(raw) == 0 || len(raw) > MaxCustodyNonceBytes || json.Unmarshal(raw, &s) != nil {
		return custodyNonceState{}, ErrCustodyNonceState
	}
	exact, err := encodeCustodyNonce(s)
	if err != nil || !bytes.Equal(exact, raw) {
		return custodyNonceState{}, ErrCustodyNonceState
	}
	return s, nil
}
func custodyNonceHeaderEqual(a, b custodyNonceState) bool {
	return a.Purpose == b.Purpose && a.Version == b.Version && a.ConfigurationSHA256 == b.ConfigurationSHA256 && a.AccountPlanSHA256 == b.AccountPlanSHA256 && a.StartingNonce == b.StartingNonce && a.MaximumReservedGasCost == b.MaximumReservedGasCost
}

// Exactly one reservation, reserved->unknown change OR immutable local review.
// Never delete/release/reprice or replace an earlier observation.
func custodyNonceTransition(old, next custodyNonceState) bool {
	raw, err := encodeCustodyNonce(old)
	if err != nil {
		return false
	}
	if _, err := encodeCustodyNonce(next); err != nil {
		return false
	}
	pin := sha256.Sum256(raw)
	if !custodyNonceHeaderEqual(old, next) || next.Sequence != old.Sequence+1 || next.PreviousSHA256 != hex.EncodeToString(pin[:]) {
		return false
	}
	if len(next.Entries) == len(old.Entries)+1 {
		if !sameCustodyNonceReviews(old.Reviews, next.Reviews) {
			return false
		}
		for n, e := range old.Entries {
			if next.Entries[n] != e {
				return false
			}
		}
		return next.Entries[len(old.Entries)].Binding.Stage == CustodyAttemptReserved
	}
	if len(next.Entries) != len(old.Entries) {
		return false
	}
	if len(next.Reviews) == len(old.Reviews)+1 {
		for n, e := range old.Entries {
			if next.Entries[n] != e {
				return false
			}
		}
		return sameCustodyNonceReviews(old.Reviews, next.Reviews[:len(old.Reviews)])
	}
	if !sameCustodyNonceReviews(old.Reviews, next.Reviews) {
		return false
	}
	changed := 0
	for n, e := range old.Entries {
		x := next.Entries[n]
		if e == x {
			continue
		}
		if e.Nonce != x.Nonce || e.MaximumGasCost != x.MaximumGasCost || !sameCustodyAttempt(e.Binding, x.Binding) || e.Binding.Stage != CustodyAttemptReserved || x.Binding.Stage != CustodyAttemptUnknown {
			return false
		}
		changed++
	}
	return changed == 1
}

// Exclusive lifetime ownership of ONE account journal. Save success must
// synchronously retain exact detached bytes; errors can retain old/new/partial
// state. No reentrancy, repair, implicit reset or another independent writer.
type CustodyNonceStorage interface {
	Load() ([]byte, error)
	Save([]byte) error
	Close() error
}
type CustodyNonceReport struct {
	Reservations              int  `json:"reservations"`
	UnknownDispatches         int  `json:"unknownDispatches"`
	LocalReceiptReviews       int  `json:"localReceiptReviews"`
	StorageUsable             bool `json:"storageUsable"`
	WriteOutcomeUncertain     bool `json:"writeOutcomeUncertain"`
	SharedJournalOnly         bool `json:"sharedJournalOnly"`
	NonceAvailabilityVerified bool `json:"nonceAvailabilityVerified"`
	TotalSourceFeesVerified   bool `json:"totalSourceFeesVerified"`
	NonceReleased             bool `json:"nonceReleased"`
	PaymentVerified           bool `json:"paymentVerified"`
	BroadcastEnabled          bool `json:"broadcastEnabled"`
	RealAssetsReady           bool `json:"realAssetsReady"`
}
type CustodyNonceBook struct {
	mu                        sync.Mutex
	inputs                    CustodyNonceInputs
	store                     CustodyNonceStorage
	state                     custodyNonceState
	plan                      custodyNoncePlan
	chain                     *big.Int
	directory                 string // disk adapter's private input-separation guard
	usable, closed, uncertain bool
}

func (*CustodyNonceBook) String() string                 { return "RH shared simulation nonce journal (redacted)" }
func (b *CustodyNonceBook) GoString() string             { return b.String() }
func (b *CustodyNonceBook) MarshalJSON() ([]byte, error) { return json.Marshal(b.Report()) }
func (b *CustodyNonceBook) Report() CustodyNonceReport {
	if b == nil {
		return CustodyNonceReport{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	u := 0
	for _, e := range b.state.Entries {
		if e.Binding.Stage == CustodyAttemptUnknown {
			u++
		}
	}
	return CustodyNonceReport{Reservations: len(b.state.Entries), UnknownDispatches: u, LocalReceiptReviews: len(b.state.Reviews), StorageUsable: b.usable && !b.closed, WriteOutcomeUncertain: b.uncertain, SharedJournalOnly: true}
}
func loadCustodyNonceInputs(i CustodyNonceInputs) (custodyNonceState, custodyNoncePlan, *big.Int, error) {
	bad := func() (custodyNonceState, custodyNoncePlan, *big.Int, error) {
		return custodyNonceState{}, custodyNoncePlan{}, nil, ErrCustodyNonceState
	}
	raw, err := readCustodyPrivateSnapshot(i.ConfigurationPath, maxConfigBytes, i.ConfigurationSHA256)
	if err != nil {
		return bad()
	}
	c, err := Parse(raw)
	if err != nil || c.private.Mode != "simulation" {
		return bad()
	}
	if _, err := c.Binding(); err != nil {
		return bad()
	}
	chain, err := chainID(c.private.SourceChainID)
	if err != nil {
		return bad()
	}
	raw, err = readCustodyPrivateSnapshot(i.AccountPlanPath, 4096, i.AccountPlanSHA256)
	if err != nil {
		return bad()
	}
	p, err := parseCustodyNoncePlan(raw)
	if err != nil {
		return bad()
	}
	token, err := address(c.private.TokenContract)
	if err != nil || p.sender == token {
		return bad()
	}
	custody, err := address(c.private.CustodyContract)
	if err != nil || p.sender == custody {
		return bad()
	}
	s := custodyNonceState{Purpose: custodyNoncePurpose, Version: 1, Sequence: 1, ConfigurationSHA256: hex.EncodeToString(i.ConfigurationSHA256[:]), AccountPlanSHA256: hex.EncodeToString(i.AccountPlanSHA256[:]), StartingNonce: strconv.FormatUint(p.start, 10), MaximumReservedGasCost: p.budget.String(), Entries: []custodyNonceEntry{}}
	return s, p, chain, nil
}

// Reopen requires one independently retained latest exact-state pin. The
// reviewed starting nonce is only a simulation seed, NOT a chain observation.
func NewCustodyNonceBook(ctx context.Context, i CustodyNonceInputs, store CustodyNonceStorage, create bool, resumePin ...[32]byte) (book *CustodyNonceBook, err error) {
	defer func() {
		if recover() != nil {
			book, err = nil, ErrCustodyNonceStorage
		}
	}()
	if ctx == nil || ctx.Err() != nil || store == nil || !custodyAttemptResumeValid(create, resumePin) {
		return nil, ErrCustodyNonceState
	}
	wanted, plan, chain, err := loadCustodyNonceInputs(i)
	if err != nil {
		return nil, ErrCustodyNonceState
	}
	raw, err := store.Load()
	if err != nil {
		return nil, ErrCustodyNonceStorage
	}
	if ctx.Err() != nil {
		return nil, ErrCustodyNonceState
	}
	b := &CustodyNonceBook{inputs: i, store: store, state: wanted, plan: plan, chain: chain, usable: true}
	if create {
		if len(raw) != 0 {
			return nil, ErrCustodyNonceState
		}
		if b.write(wanted) != nil {
			return nil, ErrCustodyNonceStorage
		}
	} else {
		if sha256.Sum256(raw) != resumePin[0] {
			return nil, ErrCustodyNonceState
		}
		s, err := decodeCustodyNonce(raw)
		if err != nil || !custodyNonceHeaderEqual(s, wanted) {
			return nil, ErrCustodyNonceState
		}
		b.state = s
	}
	return b, nil
}
func (b *CustodyNonceBook) write(s custodyNonceState) (err error) {
	raw, err := encodeCustodyNonce(s)
	if err != nil {
		return err
	}
	defer func() {
		if recover() != nil {
			err = ErrCustodyNonceStorage
		}
		if err != nil {
			b.usable, b.uncertain = false, true
		}
	}()
	if b.store.Save(bytes.Clone(raw)) != nil {
		return ErrCustodyNonceStorage
	}
	retained, err := b.store.Load()
	if err != nil || !bytes.Equal(retained, raw) {
		return ErrCustodyNonceStorage
	}
	b.state = s
	return nil
}
func (b *CustodyNonceBook) recheck(ctx context.Context) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrCustodyNonceStorage
		}
		if err != nil {
			b.usable = false
		}
	}()
	raw, err := b.store.Load()
	wanted, e := encodeCustodyNonce(b.state)
	if err != nil || e != nil || !bytes.Equal(raw, wanted) {
		return ErrCustodyNonceStorage
	}
	head, _, _, err := loadCustodyNonceInputs(b.inputs)
	if err != nil || !custodyNonceHeaderEqual(head, b.state) {
		return ErrCustodyNonceState
	}
	if ctx.Err() != nil {
		return ErrCustodyNonceState
	}
	return nil
}
func (b *CustodyNonceBook) ready(ctx context.Context) bool {
	return ctx != nil && ctx.Err() == nil && b.usable && !b.closed
}
func (b *CustodyNonceBook) nextState() custodyNonceState {
	next := b.state
	next.Entries = append([]custodyNonceEntry{}, b.state.Entries...)
	next.Reviews = append([]custodyNonceReview(nil), b.state.Reviews...)
	raw, _ := encodeCustodyNonce(b.state)
	pin := sha256.Sum256(raw)
	next.PreviousSHA256 = hex.EncodeToString(pin[:])
	next.Sequence++
	return next
}

// Private hint, never an on-chain availability certificate. Reserve rechecks
// it atomically; two callers using this same hint cannot both claim the nonce.
func (b *CustodyNonceBook) NextNonce(ctx context.Context) (uint64, error) {
	if b == nil {
		return 0, ErrCustodyNonceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready(ctx) {
		return 0, ErrCustodyNonceState
	}
	if err := b.recheck(ctx); err != nil {
		return 0, err
	}
	n := b.plan.start + uint64(len(b.state.Entries))
	if len(b.state.Entries) == MaxCustodyNonceEntries || n == math.MaxUint64 {
		return 0, ErrCustodyNonceState
	}
	return n, nil
}
func (b *CustodyNonceBook) candidate(i CustodyTransactionInputs) (custodyNonceEntry, checkedCustodyTransaction, error) {
	if i.CallInputs.ConfigurationSHA256 != b.inputs.ConfigurationSHA256 || !custodyNonceSeparated(b.directory, i) {
		return custodyNonceEntry{}, checkedCustodyTransaction{}, ErrCustodyNonceState
	}
	s, tx, err := custodyAttemptCandidate(i)
	if err != nil || tx.sender != b.plan.sender || tx.chain.Cmp(b.chain) != 0 {
		return custodyNonceEntry{}, checkedCustodyTransaction{}, ErrCustodyNonceState
	}
	return custodyNonceEntry{Binding: s, Nonce: strconv.FormatUint(tx.nonce, 10), MaximumGasCost: tx.maximumGasCost.String()}, tx, nil
}

// Each exact request/envelope owns one consecutive nonce and its worst-case
// execution-Gas budget for this book's lifetime. No refunds or gap skipping.
func (b *CustodyNonceBook) Reserve(ctx context.Context, i CustodyTransactionInputs) error {
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
	wanted, tx, err := b.candidate(i)
	if err != nil || ctx.Err() != nil {
		return ErrCustodyNonceState
	}
	for _, e := range b.state.Entries {
		if sameCustodyAttempt(e.Binding, wanted.Binding) && e.Nonce == wanted.Nonce && e.MaximumGasCost == wanted.MaximumGasCost {
			return nil
		}
		if e.Binding.RequestSHA256 == wanted.Binding.RequestSHA256 || e.Binding.EnvelopeSHA256 == wanted.Binding.EnvelopeSHA256 || e.Binding.TransactionHash == wanted.Binding.TransactionHash {
			return ErrCustodyNonceState
		}
	}
	if len(b.state.Entries) == MaxCustodyNonceEntries || tx.nonce != b.plan.start+uint64(len(b.state.Entries)) || ctx.Err() != nil {
		return ErrCustodyNonceState
	}
	next := b.nextState()
	next.Entries = append(next.Entries, wanted)
	if _, err := encodeCustodyNonce(next); err != nil {
		return ErrCustodyNonceState
	}
	return b.write(next)
}

// Signed bytes handed once, in nonce order, ONLY for an explicit local fixture.
// Earlier UNKNOWN may not have executed; later transactions can stall. This is
// conservative unique-nonce pipelining, NOT confirmed nonce progression.
func (b *CustodyNonceBook) TakeForSimulation(ctx context.Context, i CustodyTransactionInputs) ([]byte, error) {
	if b == nil {
		return nil, ErrCustodyNonceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready(ctx) {
		return nil, ErrCustodyNonceState
	}
	if err := b.recheck(ctx); err != nil {
		return nil, err
	}
	wanted, tx, err := b.candidate(i)
	if err != nil {
		return nil, ErrCustodyNonceState
	}
	for index, e := range b.state.Entries {
		if !sameCustodyAttempt(e.Binding, wanted.Binding) || e.Nonce != wanted.Nonce || e.MaximumGasCost != wanted.MaximumGasCost {
			continue
		}
		if e.Binding.Stage == CustodyAttemptUnknown {
			return nil, ErrCustodyNonceUnknown
		}
		for _, earlier := range b.state.Entries[:index] {
			if earlier.Binding.Stage != CustodyAttemptUnknown {
				return nil, ErrCustodyNonceState
			}
		}
		if ctx.Err() != nil {
			return nil, ErrCustodyNonceState
		}
		next := b.nextState()
		next.Entries[index].Binding.Stage = CustodyAttemptUnknown
		if err := b.write(next); err != nil {
			return nil, err
		}
		if ctx.Err() != nil {
			return nil, ErrCustodyNonceUnknown
		}
		return bytes.Clone(tx.raw), nil
	}
	return nil, ErrCustodyNonceState
}
func (b *CustodyNonceBook) RetainedStatePin(ctx context.Context) ([32]byte, error) {
	if b == nil {
		return [32]byte{}, ErrCustodyNonceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready(ctx) {
		return [32]byte{}, ErrCustodyNonceState
	}
	if err := b.recheck(ctx); err != nil {
		return [32]byte{}, err
	}
	raw, _ := encodeCustodyNonce(b.state)
	return sha256.Sum256(raw), nil
}
func (b *CustodyNonceBook) Close() (err error) {
	if b == nil {
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
			err = ErrCustodyNonceStorage
		}
		if err != nil {
			b.uncertain = true
		}
	}()
	if b.store.Close() != nil {
		return ErrCustodyNonceStorage
	}
	return nil
}
