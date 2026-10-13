package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"reflect"
	"sync"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
)

const MaxReturnOutboxBytes = 4 << 20
const returnOutboxPurpose = "GOD Chain private simulation return outbox v1"

var (
	ErrReturnOutboxState   = errors.New("simulation return outbox state or review rejected; no financial approval")
	ErrReturnOutboxStorage = errors.New("simulation return outbox storage unavailable or uncertain; close and independently review")
	ErrReturnOutboxUnknown = errors.New("simulation return handoff already consumed or uncertain; no retry, replacement or refund")
)

type ReturnOutboxStage string

const (
	ReturnOutboxEmpty    ReturnOutboxStage = "empty_simulation"
	ReturnOutboxPrepared ReturnOutboxStage = "prepared_unsigned_simulation"
	ReturnOutboxUnknown  ReturnOutboxStage = "handoff_unknown_simulation"
	ReturnOutboxMatched  ReturnOutboxStage = "native_attestation_matched_simulation"
)

// Exclusively owned single-slot storage. Successful Save atomically/durably
// retains detached bytes before returning. Errors/panics can leave old, new or
// partial state. Load is bounded; callbacks must not reenter this outbox/journal.
// Constructor success transfers Close ownership. No rollback/reset is supplied.
type ReturnOutboxStorage interface {
	Load() ([]byte, error)
	Save([]byte) error
	Close() error
}

type returnOutboxRecord struct {
	Proof      taskReceiptProofRecord
	Native     godnode.BridgeView
	Withdrawal godnode.BridgeWithdrawalView
	Handoff    *godnode.CommittedView
	Matched    *returnTerminalSnapshot
}
type returnOutboxState struct {
	Purpose  string
	Version  uint32
	Task     [32]byte
	Stage    ReturnOutboxStage
	Revision uint32
	Previous [32]byte
	Record   *returnOutboxRecord
}

type ReturnOutboxReport struct {
	Ticket                            uint64            `json:"ticket"`
	Kind                              string            `json:"kind"`
	Stage                             ReturnOutboxStage `json:"stage"`
	SimulationOnly                    bool              `json:"simulationOnly"`
	StorageUsable                     bool              `json:"storageUsable"`
	UnsignedProposalRetained          bool              `json:"unsignedProposalRetained"`
	SimulationHandoffRecorded         bool              `json:"simulationHandoffRecorded"`
	NeedsNativeStatusReview           bool              `json:"needsNativeStatusReview"`
	NativeAttestationRetained         bool              `json:"nativeAttestationRetained"`
	WriteOutcomeUncertain             bool              `json:"writeOutcomeUncertain"`
	RetryAllowed                      bool              `json:"retryAllowed"`
	NativeStateProven                 bool              `json:"nativeStateProven"`
	IndependentSourceFinalityVerified bool              `json:"independentSourceFinalityVerified"`
	PaymentVerified                   bool              `json:"paymentVerified"`
	RefundAuthorized                  bool              `json:"refundAuthorized"`
	SigningEnabled                    bool              `json:"signingEnabled"`
	BroadcastEnabled                  bool              `json:"broadcastEnabled"`
	RealAssetsReady                   bool              `json:"realAssetsReady"`
}

// Simulation-only unsigned resolution handoff, NOT a signed transaction,
// payment sender, source nonce journal or production execution queue. One
// exclusively owned slot prevents another handoff only within THIS slot.
// Different slots/tools and rollback of BOTH data and external pin are not
// fenced. It owns no provider, signer, approval keys or financial executor.
type ReturnSettlementOutbox struct {
	mu                        sync.Mutex
	journal                   *RelayJournal
	ticket                    uint64
	kind                      string
	store                     ReturnOutboxStorage
	state                     returnOutboxState
	usable, closed, uncertain bool
}

func (*ReturnSettlementOutbox) String() string                 { return "private simulation return outbox (redacted)" }
func (p *ReturnSettlementOutbox) GoString() string             { return p.String() }
func (p *ReturnSettlementOutbox) MarshalJSON() ([]byte, error) { return json.Marshal(p.Report()) }
func (p *ReturnSettlementOutbox) Report() ReturnOutboxReport {
	if p == nil {
		return ReturnOutboxReport{SimulationOnly: true}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.report()
}
func (p *ReturnSettlementOutbox) report() ReturnOutboxReport {
	usable := p.usable && !p.closed
	return ReturnOutboxReport{Ticket: p.ticket, Kind: p.kind, Stage: p.state.Stage, SimulationOnly: true, StorageUsable: usable,
		UnsignedProposalRetained:  usable && p.state.Record != nil,
		SimulationHandoffRecorded: usable && (p.state.Stage == ReturnOutboxUnknown || p.state.Stage == ReturnOutboxMatched),
		NeedsNativeStatusReview:   p.state.Stage == ReturnOutboxUnknown || p.uncertain,
		NativeAttestationRetained: usable && p.state.Stage == ReturnOutboxMatched, WriteOutcomeUncertain: p.uncertain}
}

func emptyReturnOutbox(task [32]byte) returnOutboxState {
	return returnOutboxState{Purpose: returnOutboxPurpose, Version: 1, Task: task, Stage: ReturnOutboxEmpty, Revision: 1}
}

// Reopen requires one independently retained exact latest-state checksum. It
// revalidates original task, complete-set proof and stored native view locally,
// writes NOTHING and does not contact providers or hand off on startup.
func NewReturnSettlementOutbox(ctx context.Context, journal *RelayJournal, ticket uint64, store ReturnOutboxStorage,
	create bool, resumePin ...[32]byte) (out *ReturnSettlementOutbox, err error) {
	defer func() {
		if recover() != nil {
			out, err = nil, ErrReturnOutboxStorage
		}
	}()
	if ctx == nil || ctx.Err() != nil || journal == nil || store == nil || !custodyAttemptResumeValid(create, resumePin) {
		return nil, ErrReturnOutboxState
	}
	journal.mu.Lock()
	defer journal.mu.Unlock()
	if reflect.TypeOf(store).Comparable() && any(store) == any(journal.store) {
		return nil, ErrReturnOutboxState
	}
	pin, kind, err := journal.taskReceiptProofPin(ctx, ticket)
	if err != nil || kind != string(godbridge.Paid) && kind != string(godbridge.Cancelled) {
		return nil, ErrReturnOutboxState
	}
	raw, err := store.Load()
	if err != nil {
		return nil, ErrReturnOutboxStorage
	}
	if len(raw) > MaxReturnOutboxBytes || ctx.Err() != nil {
		return nil, ErrReturnOutboxState
	}
	raw = bytes.Clone(raw)
	p := &ReturnSettlementOutbox{journal: journal, ticket: ticket, kind: kind, store: store, state: emptyReturnOutbox(pin), usable: true}
	if create {
		if len(raw) != 0 {
			return nil, ErrReturnOutboxState
		}
		if p.write(p.state) != nil {
			return nil, ErrReturnOutboxStorage
		}
	} else {
		if sha256.Sum256(raw) != resumePin[0] {
			return nil, ErrReturnOutboxState
		}
		p.state, err = decodeReturnOutbox(raw)
		if err != nil || p.state.Task != pin || p.verifyLocal(ctx) != nil {
			return nil, ErrReturnOutboxState
		}
	}
	return p, nil
}

// All operations take outbox then journal lock. Active source operations share
// one 30-second ceiling; mutex waiting is not context-interruptible. Adapters
// must honor cancellation and cannot reenter either API. No automatic loop.
func (p *ReturnSettlementOutbox) ready(ctx context.Context) bool {
	return p.usable && !p.closed && p.journal != nil && ctx != nil && ctx.Err() == nil
}
func (p *ReturnSettlementOutbox) checkStored(ctx context.Context) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrReturnOutboxStorage
		}
		if err == ErrReturnOutboxStorage {
			p.usable = false
		}
	}()
	raw, err := p.store.Load()
	if err != nil || len(raw) > MaxReturnOutboxBytes {
		return ErrReturnOutboxStorage
	}
	exact, err := encodeReturnOutbox(p.state)
	if err != nil || !bytes.Equal(raw, exact) {
		return ErrReturnOutboxStorage
	}
	if ctx.Err() != nil {
		return ErrReturnOutboxState
	}
	return nil
}

// Caller holds both locks; an outbox record never becomes a trust certificate.
func (p *ReturnSettlementOutbox) verifyLocal(ctx context.Context) error {
	pin, kind, err := p.journal.taskReceiptProofPin(ctx, p.ticket)
	if err != nil || pin != p.state.Task || kind != p.kind {
		return ErrReturnOutboxState
	}
	if p.state.Record == nil {
		return nil
	}
	r := p.state.Record
	witness, _, err := p.journal.prepareTaskReceiptInclusion(ctx, p.ticket, BlockMaterial(r.Proof.Material))
	if err != nil || !sameTaskReceiptWitness(witness, ReceiptInclusionWitness(r.Proof.Witness)) {
		return ErrReturnOutboxState
	}
	task := p.journal.state.Tasks[p.ticket-1]
	w := *task.Request.Withdrawal
	stored := storedReturnNativeSource{r.Native, r.Withdrawal}
	native, err := readNativeWithdrawalAt(ctx, p.journal.config, stored, w.Sequence, w.ID, r.Native.Commit.Height)
	if err != nil || !sameWithdrawal(native.withdrawal, w) {
		return ErrReturnOutboxState
	}
	if r.Handoff != nil && !returnCommitCovers(*r.Handoff, r.Native.Commit) {
		return ErrReturnOutboxState
	}
	if r.Matched != nil && (r.Handoff == nil || !returnTerminalShape(p.journal.config, r.Matched.Bridge,
		r.Matched.Withdrawal, w, godbridge.Status(p.kind), p.evidence(), *r.Handoff)) {
		return ErrReturnOutboxState
	}
	return nil
}
func (p *ReturnSettlementOutbox) evidence() godbridge.Evidence {
	task := p.journal.state.Tasks[p.ticket-1]
	return godbridge.Evidence{Height: task.ReceiptSet.Block.Height, BlockHash: task.ReceiptSet.Block.Hash,
		TransactionHash: task.Request.Transaction, LogIndex: task.Request.LogIndex}
}
func (p *ReturnSettlementOutbox) guard(ctx context.Context) error {
	if !p.ready(ctx) {
		return ErrReturnOutboxState
	}
	if err := p.checkStored(ctx); err != nil {
		return err
	}
	return p.verifyLocal(ctx)
}

// Retains the original complete RH material/proof and native pre-handoff view
// once. Exact re-review is idempotent, but cannot overwrite retained evidence.
// A newer sound native commit may recheck the same pending request; it never
// refreshes/replaces the saved original view or enables a financial operation.
func (p *ReturnSettlementOutbox) ReviewAndRetain(ctx context.Context, source ReceiptSetSource, native NativeBridgeReadSource) (report ReturnOutboxReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = ReturnOutboxReport{}, ErrReturnOutboxState
		}
	}()
	if p == nil || ctx == nil {
		return ReturnOutboxReport{}, ErrReturnOutboxState
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.journal.mu.Lock()
	defer p.journal.mu.Unlock()
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	if err := p.guard(readCtx); err != nil {
		return ReturnOutboxReport{}, err
	}
	if p.state.Stage == ReturnOutboxUnknown || p.state.Stage == ReturnOutboxMatched {
		return ReturnOutboxReport{}, ErrReturnOutboxUnknown
	}
	if source == nil || native == nil {
		return ReturnOutboxReport{}, ErrReturnOutboxState
	}
	r, err := p.journal.reviewReturnReceiptEvidence(readCtx, p.ticket, source, native)
	if err != nil {
		return ReturnOutboxReport{}, ErrReturnOutboxState
	}
	if err := p.guard(readCtx); err != nil {
		return ReturnOutboxReport{}, err
	}
	if p.state.Record != nil {
		if !p.reviewMatches(r) {
			return ReturnOutboxReport{}, ErrReturnOutboxState
		}
		return p.report(), nil
	}
	next, err := p.nextState(ReturnOutboxPrepared)
	if err != nil {
		return ReturnOutboxReport{}, err
	}
	next.Record = &returnOutboxRecord{Proof: taskReceiptProofRecord{storedProofMaterial(r.receipt.fetched.material), storedProofWitness(r.receipt.fetched.witness)},
		Native: r.native.bridge, Withdrawal: r.native.view}
	if p.write(next) != nil {
		return ReturnOutboxReport{}, ErrReturnOutboxStorage
	}
	// A completed atomic Save is acknowledged even if cancellation arrived
	// during it. This returns retention only, never an outgoing payload.
	return p.report(), nil
}
func (p *ReturnSettlementOutbox) reviewMatches(r ReturnReceiptEvidenceReview) bool {
	old := p.state.Record
	if old != nil && r.native.bridge.Commit.Height == old.Native.Commit.Height && r.native.bridge != old.Native {
		return false
	}
	return old != nil && r.ready && sameTaskReceiptMaterial(BlockMaterial(old.Proof.Material), r.receipt.fetched.material) &&
		sameTaskReceiptWitness(ReceiptInclusionWitness(old.Proof.Witness), r.receipt.fetched.witness) &&
		returnCommitCovers(r.native.bridge.Commit, old.Native.Commit)
}

// Return an UNSIGNED proposal once for an explicitly supplied local simulator.
// Fresh paired source/native review must match ALL retained evidence. Persist
// UNKNOWN before exposing anything. A lost acknowledgment/cancellation after
// Save never releases this slot, retries payment or authorizes a refund.
func (p *ReturnSettlementOutbox) TakeForSimulation(ctx context.Context, source ReceiptSetSource, native NativeBridgeReadSource) (proposal ResolutionObservation, err error) {
	defer func() {
		if recover() != nil {
			proposal, err = ResolutionObservation{}, ErrReturnOutboxState
		}
	}()
	if p == nil || ctx == nil {
		return ResolutionObservation{}, ErrReturnOutboxState
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.journal.mu.Lock()
	defer p.journal.mu.Unlock()
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	if err := p.guard(readCtx); err != nil {
		return ResolutionObservation{}, err
	}
	if p.state.Stage == ReturnOutboxUnknown || p.state.Stage == ReturnOutboxMatched {
		return ResolutionObservation{}, ErrReturnOutboxUnknown
	}
	if p.state.Stage != ReturnOutboxPrepared || source == nil || native == nil {
		return ResolutionObservation{}, ErrReturnOutboxState
	}
	r, err := p.journal.reviewReturnReceiptEvidence(readCtx, p.ticket, source, native)
	if err != nil || !p.reviewMatches(r) {
		return ResolutionObservation{}, ErrReturnOutboxState
	}
	if err := p.guard(readCtx); err != nil {
		return ResolutionObservation{}, err
	}
	proposal, err = r.receipt.ResolutionProposal()
	if err != nil {
		return ResolutionObservation{}, ErrReturnOutboxState
	}
	next, err := p.nextState(ReturnOutboxUnknown)
	if err != nil {
		return ResolutionObservation{}, err
	}
	commit := r.native.bridge.Commit
	next.Record.Handoff = &commit
	if p.write(next) != nil {
		return ResolutionObservation{}, ErrReturnOutboxStorage
	}
	if readCtx.Err() != nil {
		return ResolutionObservation{}, ErrReturnOutboxUnknown
	}
	return proposal, nil
}

// Check only the original native terminal attestation, twice at one commit.
// This does NOT refresh RH finality/receipt truth or verify native consensus.
// Pending, conflicting or missing native state never permits another handoff.
// A retained terminal match is immutable; later reads recheck it without writes.
func (p *ReturnSettlementOutbox) MatchNativeForSimulation(ctx context.Context, native NativeBridgeReadSource) (report ReturnOutboxReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = ReturnOutboxReport{}, ErrReturnOutboxState
		}
	}()
	if p == nil || ctx == nil {
		return ReturnOutboxReport{}, ErrReturnOutboxState
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.journal.mu.Lock()
	defer p.journal.mu.Unlock()
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	if err := p.guard(readCtx); err != nil {
		return ReturnOutboxReport{}, err
	}
	if native == nil || p.state.Stage != ReturnOutboxUnknown && p.state.Stage != ReturnOutboxMatched {
		return ReturnOutboxReport{}, ErrReturnOutboxState
	}
	w := *p.journal.state.Tasks[p.ticket-1].Request.Withdrawal
	outcome, evidence, handoff := godbridge.Status(p.kind), p.evidence(), *p.state.Record.Handoff
	a, err := readReturnTerminalAt(readCtx, p.journal.config, native, w, outcome, evidence, handoff, 0)
	if err != nil {
		return ReturnOutboxReport{}, ErrReturnOutboxState
	}
	b, err := readReturnTerminalAt(readCtx, p.journal.config, native, w, outcome, evidence, handoff, a.Bridge.Commit.Height)
	if err != nil || !sameReturnTerminal(a, b) {
		return ReturnOutboxReport{}, ErrReturnOutboxState
	}
	if err := p.guard(readCtx); err != nil {
		return ReturnOutboxReport{}, err
	}
	if p.state.Record.Matched != nil {
		if !returnTerminalCovers(a, *p.state.Record.Matched) {
			return ReturnOutboxReport{}, ErrReturnOutboxState
		}
		return p.report(), nil
	}
	next, err := p.nextState(ReturnOutboxMatched)
	if err != nil {
		return ReturnOutboxReport{}, err
	}
	next.Record.Matched = &a
	if p.write(next) != nil {
		return ReturnOutboxReport{}, ErrReturnOutboxStorage
	}
	return p.report(), nil
}

// PRIVATE recovery pin, independently retained outside this store. It is byte
// continuity, not authenticated origin or protection if both copies roll back.
func (p *ReturnSettlementOutbox) RetainedStatePin(ctx context.Context) (pin [32]byte, err error) {
	defer func() {
		if recover() != nil {
			pin, err = [32]byte{}, ErrReturnOutboxState
		}
	}()
	if p == nil || ctx == nil {
		return [32]byte{}, ErrReturnOutboxState
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.journal.mu.Lock()
	defer p.journal.mu.Unlock()
	if err := p.guard(ctx); err != nil {
		return [32]byte{}, err
	}
	raw, err := encodeReturnOutbox(p.state)
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(raw), nil
}
func (p *ReturnSettlementOutbox) nextState(stage ReturnOutboxStage) (returnOutboxState, error) {
	raw, err := encodeReturnOutbox(p.state)
	if err != nil {
		return returnOutboxState{}, err
	}
	next := p.state
	next.Stage, next.Revision, next.Previous = stage, next.Revision+1, sha256.Sum256(raw)
	if next.Record != nil {
		copy := *next.Record
		next.Record = &copy
	}
	return next, nil
}
func (p *ReturnSettlementOutbox) write(next returnOutboxState) (err error) {
	raw, err := encodeReturnOutbox(next)
	if err != nil {
		return err
	}
	defer func() {
		if recover() != nil {
			err = ErrReturnOutboxStorage
		}
		if err != nil {
			p.usable, p.uncertain = false, true
		}
	}()
	if p.store.Save(bytes.Clone(raw)) != nil {
		return ErrReturnOutboxStorage
	}
	p.state = next
	return nil
}
func (p *ReturnSettlementOutbox) Close() (err error) {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed, p.usable = true, false
	defer func() {
		if recover() != nil {
			err = ErrReturnOutboxStorage
		}
		if err != nil {
			p.uncertain = true
		}
	}()
	if p.store.Close() != nil {
		return ErrReturnOutboxStorage
	}
	return nil
}

func encodeReturnOutbox(s returnOutboxState) ([]byte, error) {
	if s.Purpose != returnOutboxPurpose || s.Version != 1 || s.Task == [32]byte{} {
		return nil, ErrReturnOutboxState
	}
	base := emptyReturnOutbox(s.Task)
	if s.Stage == ReturnOutboxEmpty {
		if s.Revision != 1 || s.Previous != [32]byte{} || s.Record != nil {
			return nil, ErrReturnOutboxState
		}
	} else {
		if s.Record == nil || len(s.Record.Proof.Material.Transactions) > MaxReceiptSetTransactions || len(s.Record.Proof.Material.Receipts) > MaxReceiptSetTransactions {
			return nil, ErrReturnOutboxState
		}
		r := s.Record
		if !validNativeCommit(r.Native.Commit, 0) || r.Withdrawal.Commit != r.Native.Commit ||
			r.Native.RealAssetsReady || r.Native.IndependentSourceFinalityVerified || r.Withdrawal.RealAssetsReady ||
			r.Withdrawal.IndependentSourceFinalityVerified || r.Withdrawal.SourceResolution != nil ||
			(r.Handoff != nil && !validNativeCommit(*r.Handoff, 0)) {
			return nil, ErrReturnOutboxState
		}
		if m := r.Matched; m != nil && (!validNativeCommit(m.Bridge.Commit, 0) || m.Withdrawal.Commit != m.Bridge.Commit ||
			m.Bridge.RealAssetsReady || m.Bridge.IndependentSourceFinalityVerified || m.Withdrawal.RealAssetsReady || m.Withdrawal.IndependentSourceFinalityVerified) {
			return nil, ErrReturnOutboxState
		}
		previous := base
		switch s.Stage {
		case ReturnOutboxPrepared:
			if s.Revision != 2 || s.Record.Handoff != nil || s.Record.Matched != nil {
				return nil, ErrReturnOutboxState
			}
		case ReturnOutboxUnknown:
			if s.Revision != 3 || s.Record.Handoff == nil || s.Record.Matched != nil {
				return nil, ErrReturnOutboxState
			}
			previous = s
			copy := *s.Record
			previous.Record = &copy
			previous.Stage, previous.Revision, previous.Record.Handoff = ReturnOutboxPrepared, 2, nil
			raw, _ := encodeReturnOutbox(base)
			previous.Previous = sha256.Sum256(raw)
		case ReturnOutboxMatched:
			if s.Revision != 4 || s.Record.Handoff == nil || s.Record.Matched == nil {
				return nil, ErrReturnOutboxState
			}
			prepared := s
			copy := *s.Record
			prepared.Record = &copy
			prepared.Stage, prepared.Revision, prepared.Record.Handoff, prepared.Record.Matched = ReturnOutboxPrepared, 2, nil, nil
			raw, _ := encodeReturnOutbox(base)
			prepared.Previous = sha256.Sum256(raw)
			raw, err := encodeReturnOutbox(prepared)
			if err != nil {
				return nil, err
			}
			previous = s
			copy = *s.Record
			previous.Record = &copy
			previous.Stage, previous.Revision, previous.Record.Matched, previous.Previous = ReturnOutboxUnknown, 3, nil, sha256.Sum256(raw)
		default:
			return nil, ErrReturnOutboxState
		}
		raw, err := encodeReturnOutbox(previous)
		if err != nil || s.Previous != sha256.Sum256(raw) {
			return nil, ErrReturnOutboxState
		}
	}
	raw, err := json.Marshal(s)
	if err != nil || len(raw) > MaxReturnOutboxBytes {
		return nil, ErrReturnOutboxState
	}
	return raw, nil
}
func decodeReturnOutbox(raw []byte) (returnOutboxState, error) {
	var s returnOutboxState
	if len(raw) == 0 || len(raw) > MaxReturnOutboxBytes || json.Unmarshal(raw, &s) != nil {
		return s, ErrReturnOutboxState
	}
	exact, err := encodeReturnOutbox(s)
	if err != nil || !bytes.Equal(raw, exact) {
		return returnOutboxState{}, ErrReturnOutboxState
	}
	return s, nil
}
