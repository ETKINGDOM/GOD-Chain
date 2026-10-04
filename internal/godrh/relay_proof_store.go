package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
)

// One immutable private proof slot per exclusively owned store. These are
// simulation admission bounds, not production storage/retention parameters.
const MaxTaskReceiptProofBytes = 3 << 20

const taskReceiptProofPurpose = "GOD Chain private task receipt proof v1"

var (
	ErrTaskReceiptProofState   = errors.New("RH private task proof state or binding rejected")
	ErrTaskReceiptProofStorage = errors.New("RH private task proof storage failed; reopen and review before continuing")
	ErrTaskReceiptProofMissing = errors.New("RH private task proof slot is empty")
	ErrTaskReceiptProofSource  = errors.New("RH retained task proof source check rejected; no storage change or financial approval")
)

// TaskReceiptProofStore owns one private slot for one immutable complete-set
// task. The supplied RelayStore has the same exclusive, atomic durable-write
// contract as the journal, but MUST be separate storage. Store callbacks must
// not reenter this proof store or its journal. It owns no source/signer/ledger;
// explicit source checking borrows an adapter only for that call.
type TaskReceiptProofStore struct {
	mu                        sync.Mutex
	journal                   *RelayJournal
	ticket                    uint64
	kind                      string
	store                     RelayStore
	state                     taskReceiptProofState
	usable, closed, uncertain bool
}

// Internal storage types deliberately omit public redacted JSON methods. Raw data
// is encoded ONLY in the private store, never in reports or diagnostics.
type storedProofMaterial BlockMaterial
type storedProofWitness ReceiptInclusionWitness
type taskReceiptProofRecord struct {
	Material storedProofMaterial
	Witness  storedProofWitness
}
type taskReceiptProofState struct {
	Purpose string
	Version uint32
	Task    [32]byte
	Record  *taskReceiptProofRecord
}

type TaskReceiptProofStoreReport struct {
	Ticket                      uint64 `json:"ticket"`
	Kind                        string `json:"kind"`
	SimulationOnly              bool   `json:"simulationOnly"`
	StorageUsable               bool   `json:"storageUsable"`
	ProofRetained               bool   `json:"proofRetained"`
	WriteOutcomeUncertain       bool   `json:"writeOutcomeUncertain"`
	IndependentFinalityVerified bool   `json:"independentFinalityVerified"`
	ApprovalReady               bool   `json:"approvalReady"`
	SigningEnabled              bool   `json:"signingEnabled"`
	BroadcastEnabled            bool   `json:"broadcastEnabled"`
	RealAssetsReady             bool   `json:"realAssetsReady"`
}

func (*TaskReceiptProofStore) String() string {
	return "RH private simulation task proof store (redacted)"
}
func (p *TaskReceiptProofStore) GoString() string             { return p.String() }
func (p *TaskReceiptProofStore) MarshalJSON() ([]byte, error) { return json.Marshal(p.Report()) }
func (p *TaskReceiptProofStore) Report() TaskReceiptProofStoreReport {
	if p == nil {
		return TaskReceiptProofStoreReport{SimulationOnly: true}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.report()
}
func (p *TaskReceiptProofStore) report() TaskReceiptProofStoreReport {
	usable := p.usable && !p.closed
	return TaskReceiptProofStoreReport{Ticket: p.ticket, Kind: p.kind, SimulationOnly: true, StorageUsable: usable,
		ProofRetained: usable && p.state.Record != nil, WriteOutcomeUncertain: p.uncertain}
}

// NewTaskReceiptProofStore never initializes over existing/missing state. A
// reopened populated slot reruns complete-set task review and proof generation,
// compares every saved witness byte, and writes NOTHING on reopen. Success
// takes ownership of store.Close; after failure the caller owns the store.
// Neither checksum nor task pin authenticates a malicious owner or rollback.
func NewTaskReceiptProofStore(ctx context.Context, journal *RelayJournal, ticket uint64, store RelayStore, create bool) (out *TaskReceiptProofStore, err error) {
	defer func() {
		if recover() != nil {
			out, err = nil, ErrTaskReceiptProofStorage
		}
	}()
	if ctx == nil || journal == nil || store == nil {
		return nil, ErrTaskReceiptProofState
	}
	journal.mu.Lock()
	defer journal.mu.Unlock()
	if reflect.TypeOf(store).Comparable() && store == journal.store {
		return nil, ErrTaskReceiptProofState
	}
	pin, kind, err := journal.taskReceiptProofPin(ctx, ticket)
	if err != nil {
		return nil, ErrTaskReceiptProofState
	}
	raw, err := store.Load()
	if err != nil {
		return nil, ErrTaskReceiptProofStorage
	}
	if len(raw) > MaxTaskReceiptProofBytes {
		return nil, ErrTaskReceiptProofState
	}
	raw = bytes.Clone(raw)
	p := &TaskReceiptProofStore{journal: journal, ticket: ticket, kind: kind, store: store, usable: true}
	if create {
		if len(raw) != 0 || ctx.Err() != nil {
			return nil, ErrTaskReceiptProofState
		}
		if p.write(taskReceiptProofState{Purpose: taskReceiptProofPurpose, Version: 1, Task: pin}) != nil {
			return nil, ErrTaskReceiptProofStorage
		}
		return p, nil
	}
	state, err := decodeTaskReceiptProof(raw)
	if err != nil || state.Task != pin {
		return nil, ErrTaskReceiptProofState
	}
	p.state = state
	if state.Record != nil {
		if _, _, err = p.reverify(ctx); err != nil {
			return nil, ErrTaskReceiptProofState
		}
	}
	if ctx.Err() != nil {
		return nil, ErrTaskReceiptProofState
	}
	return p, nil
}

// The caller holds j.mu. Mutable stage/cache/cursor/retries are NOT pinned;
// the original request and complete-set event binding are pinned in full.
func (j *RelayJournal) taskReceiptProofPin(ctx context.Context, ticket uint64) (pin [32]byte, kind string, err error) {
	defer func() {
		if recover() != nil {
			pin, kind, err = [32]byte{}, "", ErrTaskReceiptProofState
		}
	}()
	if ctx == nil || ctx.Err() != nil || !j.usable || j.closed || ticket == 0 || ticket > uint64(len(j.state.Tasks)) || j.validate(j.state) != nil {
		return [32]byte{}, "", ErrTaskReceiptProofState
	}
	task := j.state.Tasks[ticket-1]
	if task.ReceiptSet == nil {
		return [32]byte{}, "", ErrTaskReceiptProofState
	}
	raw, err := json.Marshal(struct {
		Purpose    string
		Binding    [32]byte
		Ticket     uint64
		Request    relayRequest
		ReceiptSet relayReceiptBinding
	}{taskReceiptProofPurpose, j.state.Binding, ticket, task.Request, *task.ReceiptSet})
	if err != nil {
		return [32]byte{}, "", ErrTaskReceiptProofState
	}
	return sha256.Sum256(raw), task.Request.Kind, nil
}

// PrepareAndSave may populate the slot once. An exact repeat is idempotent;
// no replacement, deletion, task promotion or journal write is supplied.
// Cancellation before Save prevents writes. Once atomic Save succeeds, the
// commit is acknowledged even if cancellation arrives DURING that write.
func (p *TaskReceiptProofStore) PrepareAndSave(ctx context.Context, material BlockMaterial) (report TaskReceiptProofStoreReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = TaskReceiptProofStoreReport{}, ErrTaskReceiptProofState
		}
	}()
	if p == nil || ctx == nil {
		return TaskReceiptProofStoreReport{}, ErrTaskReceiptProofState
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.usable || p.closed {
		return TaskReceiptProofStoreReport{}, ErrTaskReceiptProofState
	}
	p.journal.mu.Lock()
	defer p.journal.mu.Unlock()
	pin, _, err := p.journal.taskReceiptProofPin(ctx, p.ticket)
	if err != nil || pin != p.state.Task {
		return TaskReceiptProofStoreReport{}, ErrTaskReceiptProofState
	}
	material, err = detachReceiptSetMaterial(material)
	if err != nil {
		return TaskReceiptProofStoreReport{}, ErrTaskReceiptProofState
	}
	witness, _, err := p.journal.prepareTaskReceiptInclusion(ctx, p.ticket, material)
	if err != nil {
		return TaskReceiptProofStoreReport{}, ErrTaskReceiptProofState
	}
	next := taskReceiptProofState{Purpose: taskReceiptProofPurpose, Version: 1, Task: pin,
		Record: &taskReceiptProofRecord{Material: storedProofMaterial(material), Witness: storedProofWitness(witness)}}
	encoded, err := encodeTaskReceiptProof(next)
	if err != nil || ctx.Err() != nil {
		return TaskReceiptProofStoreReport{}, ErrTaskReceiptProofState
	}
	if p.state.Record != nil {
		old, err := encodeTaskReceiptProof(p.state)
		if err != nil || !bytes.Equal(encoded, old) {
			return TaskReceiptProofStoreReport{}, ErrTaskReceiptProofState
		}
		return p.report(), nil
	}
	if p.write(next) != nil {
		return TaskReceiptProofStoreReport{}, ErrTaskReceiptProofStorage
	}
	return p.report(), nil
}

// Read reruns review against the CURRENT journal's immutable task and any
// current unsigned cache, but performs no source read, store write or retry.
// Returned data is newly generated/detached. Its review remains ephemeral;
// storage retention is reported separately, never as financial approval.
func (p *TaskReceiptProofStore) Read(ctx context.Context) (witness ReceiptInclusionWitness, observation RelayReceiptProofObservation, err error) {
	defer func() {
		if recover() != nil {
			witness, observation, err = ReceiptInclusionWitness{}, RelayReceiptProofObservation{}, ErrTaskReceiptProofState
		}
	}()
	if p == nil || ctx == nil {
		return ReceiptInclusionWitness{}, RelayReceiptProofObservation{}, ErrTaskReceiptProofState
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.usable || p.closed {
		return ReceiptInclusionWitness{}, RelayReceiptProofObservation{}, ErrTaskReceiptProofState
	}
	p.journal.mu.Lock()
	defer p.journal.mu.Unlock()
	pin, _, err := p.journal.taskReceiptProofPin(ctx, p.ticket)
	if err != nil || pin != p.state.Task {
		return ReceiptInclusionWitness{}, RelayReceiptProofObservation{}, ErrTaskReceiptProofState
	}
	if p.state.Record == nil {
		return ReceiptInclusionWitness{}, RelayReceiptProofObservation{}, ErrTaskReceiptProofMissing
	}
	return p.reverify(ctx)
}

// Both locks are held (or construction has exclusive ownership).
func (p *TaskReceiptProofStore) reverify(ctx context.Context) (ReceiptInclusionWitness, RelayReceiptProofObservation, error) {
	w, o, err := p.journal.prepareTaskReceiptInclusion(ctx, p.ticket, BlockMaterial(p.state.Record.Material))
	if err != nil || !sameTaskReceiptWitness(w, ReceiptInclusionWitness(p.state.Record.Witness)) || ctx.Err() != nil {
		return ReceiptInclusionWitness{}, RelayReceiptProofObservation{}, ErrTaskReceiptProofState
	}
	return w, o, nil
}

// TaskReceiptProofSourceReview is a PRIVATE, ephemeral comparison of one
// retained proof with a newly read complete set. It owns no store/source pointer
// and conveys no authenticated finality, future freshness or signing authority.
type TaskReceiptProofSourceReview struct {
	fetched FetchedTaskReceiptProof
	matched bool
}

type TaskReceiptProofSourceReport struct {
	TaskReceiptFetchReport
	RetainedProofMatched bool `json:"retainedProofMatched"`
	SigningEnabled       bool `json:"signingEnabled"`
	BroadcastEnabled     bool `json:"broadcastEnabled"`
}

func (TaskReceiptProofSourceReview) String() string {
	return "RH simulation retained task proof source review (redacted)"
}
func (r TaskReceiptProofSourceReview) GoString() string { return r.String() }
func (r TaskReceiptProofSourceReview) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Report())
}
func (r TaskReceiptProofSourceReview) Report() TaskReceiptProofSourceReport {
	if !r.matched || !r.fetched.ready {
		return TaskReceiptProofSourceReport{}
	}
	return TaskReceiptProofSourceReport{TaskReceiptFetchReport: r.fetched.Report(), RetainedProofMatched: true}
}

// Explicit access returns new owned PRIVATE bytes, never a certificate.
func (r TaskReceiptProofSourceReview) Material() (BlockMaterial, error) {
	if !r.matched || !r.fetched.ready {
		return BlockMaterial{}, ErrTaskReceiptProofSource
	}
	return r.fetched.Material()
}
func (r TaskReceiptProofSourceReview) Proof() (ReceiptInclusionWitness, RelayReceiptProofObservation, error) {
	if !r.matched || !r.fetched.ready {
		return ReceiptInclusionWitness{}, RelayReceiptProofObservation{}, ErrTaskReceiptProofSource
	}
	return r.fetched.Proof()
}

// RecheckSource explicitly verifies the loaded private record, fetches the
// original task's material once with provider-reference guards and compares ALL
// material and witness bytes. It never reloads/writes storage, replaces a slot,
// creates a cache or consumes attempts. A failed source does not disable sound
// local storage. It does NOT replace ObserveNext's code/terminal state checks.
//
// Locks remain store-then-journal through the active bounded read. Source
// adapters must honor context and never reenter either API from callbacks.
// Waiting for either mutex is not context-interruptible. Success is relative
// provider agreement during one call, not independently authenticated finality.
func (p *TaskReceiptProofStore) RecheckSource(ctx context.Context, source ReceiptSetSource) (review TaskReceiptProofSourceReview, err error) {
	defer func() {
		if recover() != nil {
			review, err = TaskReceiptProofSourceReview{}, ErrTaskReceiptProofSource
		}
	}()
	fail := func() (TaskReceiptProofSourceReview, error) {
		return TaskReceiptProofSourceReview{}, ErrTaskReceiptProofSource
	}
	if p == nil || ctx == nil || source == nil {
		return fail()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.usable || p.closed || p.journal == nil || p.state.Record == nil {
		return fail()
	}
	p.journal.mu.Lock()
	defer p.journal.mu.Unlock()
	if ctx.Err() != nil {
		return fail()
	}
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	pin, _, err := p.journal.taskReceiptProofPin(readCtx, p.ticket)
	if err != nil || pin != p.state.Task || p.state.Purpose != taskReceiptProofPurpose || p.state.Version != 1 {
		return fail()
	}
	if _, _, err = p.reverify(readCtx); err != nil {
		return fail()
	}
	fetched, err := p.journal.fetchTaskReceiptProof(readCtx, p.ticket, source)
	if err != nil || !sameTaskReceiptMaterial(fetched.material, BlockMaterial(p.state.Record.Material)) ||
		!sameTaskReceiptWitness(fetched.witness, ReceiptInclusionWitness(p.state.Record.Witness)) || readCtx.Err() != nil {
		return fail()
	}
	return TaskReceiptProofSourceReview{fetched: fetched, matched: true}, nil
}

func sameTaskReceiptMaterial(a, b BlockMaterial) bool {
	if !bytes.Equal(a.Header, b.Header) || len(a.Transactions) != len(b.Transactions) || len(a.Receipts) != len(b.Receipts) {
		return false
	}
	for i := range a.Transactions {
		if !bytes.Equal(a.Transactions[i], b.Transactions[i]) {
			return false
		}
	}
	for i := range a.Receipts {
		if !bytes.Equal(a.Receipts[i], b.Receipts[i]) {
			return false
		}
	}
	return true
}

func sameTaskReceiptWitness(a, b ReceiptInclusionWitness) bool {
	if !bytes.Equal(a.Header, b.Header) || !bytes.Equal(a.Transaction, b.Transaction) || !bytes.Equal(a.Receipt, b.Receipt) ||
		len(a.TransactionProof) != len(b.TransactionProof) || len(a.ReceiptProof) != len(b.ReceiptProof) {
		return false
	}
	for i := range a.TransactionProof {
		if !bytes.Equal(a.TransactionProof[i], b.TransactionProof[i]) {
			return false
		}
	}
	for i := range a.ReceiptProof {
		if !bytes.Equal(a.ReceiptProof[i], b.ReceiptProof[i]) {
			return false
		}
	}
	return true
}

// Any write error/panic is an UNKNOWN outcome; stop using this instance.
func (p *TaskReceiptProofStore) write(next taskReceiptProofState) (err error) {
	defer func() {
		if recover() != nil {
			p.usable, p.uncertain, err = false, true, ErrTaskReceiptProofStorage
		}
	}()
	raw, err := encodeTaskReceiptProof(next)
	if err != nil {
		return ErrTaskReceiptProofState
	}
	if p.store.Save(raw) != nil {
		p.usable, p.uncertain = false, true
		return ErrTaskReceiptProofStorage
	}
	p.state = next
	return nil
}

func (p *TaskReceiptProofStore) Close() (err error) {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() {
		if recover() != nil {
			err = ErrTaskReceiptProofStorage
		}
	}()
	if p.closed {
		return nil
	}
	p.closed, p.usable = true, false
	if p.store.Close() != nil {
		return ErrTaskReceiptProofStorage
	}
	return nil
}

func encodeTaskReceiptProof(state taskReceiptProofState) ([]byte, error) {
	if state.Purpose != taskReceiptProofPurpose || state.Version != 1 || state.Task == [32]byte{} {
		return nil, ErrTaskReceiptProofState
	}
	payload, err := json.Marshal(state)
	if err != nil || len(payload) > MaxTaskReceiptProofBytes-sha256.Size {
		return nil, ErrTaskReceiptProofState
	}
	sum := sha256.Sum256(payload)
	return append(payload, sum[:]...), nil
}

func decodeTaskReceiptProof(raw []byte) (taskReceiptProofState, error) {
	if len(raw) <= sha256.Size || len(raw) > MaxTaskReceiptProofBytes {
		return taskReceiptProofState{}, ErrTaskReceiptProofState
	}
	payload := raw[:len(raw)-sha256.Size]
	sum := sha256.Sum256(payload)
	if !bytes.Equal(sum[:], raw[len(payload):]) {
		return taskReceiptProofState{}, ErrTaskReceiptProofState
	}
	var state taskReceiptProofState
	if json.Unmarshal(payload, &state) != nil {
		return taskReceiptProofState{}, ErrTaskReceiptProofState
	}
	canonical, err := encodeTaskReceiptProof(state)
	if err != nil || !bytes.Equal(canonical, raw) {
		return taskReceiptProofState{}, ErrTaskReceiptProofState
	}
	return state, nil
}
