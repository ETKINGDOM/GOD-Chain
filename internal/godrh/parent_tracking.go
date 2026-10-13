package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"sync"
)

const (
	MaxParentTrackingSteps     = 16
	MaxParentTrackingBytes     = 4 << 20
	maxParentTrackingStepBytes = 256 << 10
	parentTrackingPurpose      = "GOD Chain conditional parent finality tracking v1"
)

var (
	ErrParentTracking         = errors.New("conditional parent tracking input, freshness or retained pin rejected; no RH approval")
	ErrParentTrackingStorage  = errors.New("conditional parent tracking storage unavailable or write uncertain; close and independently review retained pins")
	ErrParentTrackingConflict = errors.New("conditional parent tracking conflicts with retained head or prepared revision; no head replacement")
)

// ParentTrackingStorage must be exclusively owned, bounded on Load, and retain
// each successful Save synchronously and atomically. Save errors/panics can
// leave old OR new data; no retry/reset/rollback is supplied. Callbacks must not
// reenter the tracker. Constructor success transfers ownership of Close.
type ParentTrackingStorage interface {
	Load() ([]byte, error)
	Save([]byte) error
	Close() error
}

// Private encoding aliases deliberately do not inherit diagnostic redaction.
// These bytes contain operational roots and proof material, never public JSON.
type parentStoredHeader ParentBeaconHeader
type parentStoredTrust ParentFinalityTrust
type parentStoredProof struct {
	Bootstrap, Attested, Finalized                       parentStoredHeader
	Committee                                            [parentCommitteeSize][48]byte
	AggregatePubkey                                      [48]byte
	CommitteeBranch, FinalityBranch, ExecutionHashBranch [][32]byte
	Participation                                        [64]byte
	Signature                                            [96]byte
	SignatureSlot                                        uint64
	FinalizedExecutionHeader                             []byte
}
type parentStoredNext ParentNextCommitteeProof
type parentTrackingStep struct {
	CurrentSlot uint64
	Proof       parentStoredProof
	Next        *parentStoredNext
}
type parentTrackingState struct {
	Purpose string
	Version uint32
	Origin  parentStoredTrust
	Steps   []parentTrackingStep
}

func parentStoreProof(p ParentFinalityProof) parentStoredProof {
	return parentStoredProof{parentStoredHeader(p.Bootstrap), parentStoredHeader(p.Attested), parentStoredHeader(p.Finalized),
		p.Committee, p.AggregatePubkey, p.CommitteeBranch, p.FinalityBranch, p.ExecutionHashBranch,
		p.Participation, p.Signature, p.SignatureSlot, p.FinalizedExecutionHeader}
}
func (p parentStoredProof) proof() ParentFinalityProof {
	return ParentFinalityProof{ParentBeaconHeader(p.Bootstrap), ParentBeaconHeader(p.Attested), ParentBeaconHeader(p.Finalized),
		p.Committee, p.AggregatePubkey, p.CommitteeBranch, p.FinalityBranch, p.ExecutionHashBranch,
		p.Participation, p.Signature, p.SignatureSlot, p.FinalizedExecutionHeader}
}

func encodeParentTracking(s parentTrackingState) ([]byte, error) {
	if s.Purpose != parentTrackingPurpose || s.Version != 1 || len(s.Steps) > MaxParentTrackingSteps {
		return nil, ErrParentTracking
	}
	for _, step := range s.Steps {
		p := step.Proof
		if len(p.CommitteeBranch) > 6 || len(p.FinalityBranch) > 7 || len(p.ExecutionHashBranch) != 9 ||
			len(p.FinalizedExecutionHeader) == 0 || len(p.FinalizedExecutionHeader) > maxSetHeaderBytes ||
			(step.Next != nil && len(step.Next.Branch) > 6) {
			return nil, ErrParentTracking
		}
		encoded, err := json.Marshal(step)
		if err != nil || len(encoded) > maxParentTrackingStepBytes {
			return nil, ErrParentTracking
		}
	}
	raw, err := json.Marshal(s)
	if err != nil || len(raw) > MaxParentTrackingBytes {
		return nil, ErrParentTracking
	}
	return raw, nil
}

func decodeParentTracking(raw []byte) (parentTrackingState, error) {
	var s parentTrackingState
	if len(raw) == 0 || len(raw) > MaxParentTrackingBytes {
		return s, ErrParentTracking
	}
	// Do NOT unmarshal []parentTrackingStep directly: millions of tiny `{}`
	// elements could allocate enormous fixed-width committee arrays before a
	// post-decode count check. Decode one bounded raw step at a time instead.
	var wire struct {
		Purpose string
		Version uint32
		Origin  parentStoredTrust
		Steps   json.RawMessage
	}
	if json.Unmarshal(raw, &wire) != nil || wire.Purpose != parentTrackingPurpose || wire.Version != 1 {
		return s, ErrParentTracking
	}
	s.Purpose, s.Version, s.Origin = wire.Purpose, wire.Version, wire.Origin
	d := json.NewDecoder(bytes.NewReader(wire.Steps))
	open, err := d.Token()
	if err != nil || open != json.Delim('[') {
		return s, ErrParentTracking
	}
	for d.More() {
		if len(s.Steps) >= MaxParentTrackingSteps {
			return parentTrackingState{}, ErrParentTracking
		}
		var item json.RawMessage
		if d.Decode(&item) != nil || len(item) > maxParentTrackingStepBytes {
			return parentTrackingState{}, ErrParentTracking
		}
		var step parentTrackingStep
		if json.Unmarshal(item, &step) != nil {
			return parentTrackingState{}, ErrParentTracking
		}
		s.Steps = append(s.Steps, step)
	}
	close, err := d.Token()
	if err != nil || close != json.Delim(']') || len(s.Steps) == 0 {
		return parentTrackingState{}, ErrParentTracking
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return parentTrackingState{}, ErrParentTracking
	}
	canonical, err := encodeParentTracking(s)
	if err != nil || !bytes.Equal(raw, canonical) {
		return parentTrackingState{}, ErrParentTracking
	}
	return s, nil
}

func parentTrackingFresh(t ParentFinalityTrust, current uint64) bool {
	return t.CheckpointRoot != [32]byte{} && t.GenesisValidatorsRoot != [32]byte{} && t.ForkVersion != [4]byte{} &&
		t.CheckpointSlot > 0 && t.MaxCheckpointAgeSlots > 0 && t.MaxCheckpointAgeSlots <= parentPeriodSlots &&
		(t.ForkName == "deneb" || t.ForkName == "electra") && t.ForkStartSlot%32 == 0 && t.ForkEndSlot%32 == 0 &&
		t.ForkEndSlot > t.ForkStartSlot && t.CheckpointSlot >= t.ForkStartSlot && t.CheckpointSlot < t.ForkEndSlot &&
		t.CurrentSlot >= t.CheckpointSlot && current >= t.CurrentSlot && current < t.ForkEndSlot &&
		current-t.CheckpointSlot <= t.MaxCheckpointAgeSlots
}

// Original anchor age is checked independently of the derived head. Advancing
// a cryptographically verified head NEVER renews the original trust lifetime.
func parentTrackingReplay(ctx context.Context, s parentTrackingState, current uint64) (ParentBeaconHeader, uint64, error) {
	origin := ParentFinalityTrust(s.Origin)
	if ctx == nil || !parentTrackingFresh(origin, current) || ctx.Err() != nil {
		return ParentBeaconHeader{}, 0, ErrParentTracking
	}
	head := ParentBeaconHeader{Slot: origin.CheckpointSlot}
	root, observed := origin.CheckpointRoot, origin.CurrentSlot
	for _, step := range s.Steps {
		if step.CurrentSlot < observed || step.CurrentSlot > current || !parentTrackingFresh(origin, step.CurrentSlot) {
			return ParentBeaconHeader{}, 0, ErrParentTracking
		}
		t := origin
		t.CheckpointRoot, t.CheckpointSlot, t.CurrentSlot = root, head.Slot, step.CurrentSlot
		var next *ParentNextCommitteeProof
		if step.Next != nil {
			n := ParentNextCommitteeProof(*step.Next)
			next = &n
		}
		p := step.Proof.proof()
		if _, err := verifyParentFinalityProof(ctx, t, p, next); err != nil {
			return ParentBeaconHeader{}, 0, ErrParentTracking
		}
		head, root, observed = p.Finalized, parentHeaderRoot(p.Finalized), step.CurrentSlot
	}
	if ctx.Err() != nil {
		return ParentBeaconHeader{}, 0, ErrParentTracking
	}
	return head, observed, nil
}

type ParentTrackingReport struct {
	StorageUsable                    bool `json:"storageUsable"`
	WriteOutcomeUncertain            bool `json:"writeOutcomeUncertain"`
	RetainedUpdates                  int  `json:"retainedUpdates"`
	ConditionalHeadRetained          bool `json:"conditionalHeadRetained"`
	AnchorIndependentlyAuthenticated bool `json:"anchorIndependentlyAuthenticated"`
	RHSourceFinalityVerified         bool `json:"rhSourceFinalityVerified"`
	SigningEnabled                   bool `json:"signingEnabled"`
	BroadcastEnabled                 bool `json:"broadcastEnabled"`
	RealAssetsReady                  bool `json:"realAssetsReady"`
}

// ParentFinalityTracker retains a bounded proof chain under ONE externally
// supplied anchor, fork interval and clock policy. It is not a full Ethereum
// light client or an RH verifier. Same-slot conflicts, rollbacks, skipped
// periods, changed bindings, expired origin trust and all fork transitions
// fail closed. No optimistic/timeout head, automatic refresh or network exists.
type ParentFinalityTracker struct {
	mu                        sync.Mutex
	store                     ParentTrackingStorage
	state                     parentTrackingState
	raw                       []byte
	pin                       [32]byte
	head                      ParentBeaconHeader
	observed                  uint64
	usable, closed, uncertain bool
}

func (*ParentFinalityTracker) String() string {
	return "conditional parent finality tracker (redacted)"
}
func (p *ParentFinalityTracker) GoString() string             { return p.String() }
func (p *ParentFinalityTracker) MarshalJSON() ([]byte, error) { return json.Marshal(p.Report()) }
func (p *ParentFinalityTracker) Report() ParentTrackingReport {
	if p == nil {
		return ParentTrackingReport{}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return ParentTrackingReport{StorageUsable: p.usable && !p.closed, WriteOutcomeUncertain: p.uncertain,
		RetainedUpdates: len(p.state.Steps), ConditionalHeadRetained: p.usable && !p.closed && len(p.state.Steps) > 0}
}

// expectedPin on reopen MUST be retained separately from this storage. A
// checksum read from the same possibly rolled-back file is not an anti-rollback
// anchor. Reopen replays EVERY signature/inclusion proof from the original
// checkpoint, under fresh caller clock input, and writes nothing. Creating is
// allowed only with empty storage and a zero pin; no initial write occurs.
func NewParentFinalityTracker(ctx context.Context, origin ParentFinalityTrust, current uint64, expectedPin [32]byte, store ParentTrackingStorage, create bool) (out *ParentFinalityTracker, err error) {
	defer func() {
		if recover() != nil {
			out, err = nil, ErrParentTrackingStorage
		}
	}()
	if ctx == nil || store == nil || !parentTrackingFresh(origin, current) || ctx.Err() != nil {
		return nil, ErrParentTracking
	}
	raw, err := store.Load()
	if err != nil {
		return nil, ErrParentTrackingStorage
	}
	if len(raw) > MaxParentTrackingBytes {
		return nil, ErrParentTracking
	}
	raw = bytes.Clone(raw)
	s := parentTrackingState{Purpose: parentTrackingPurpose, Version: 1, Origin: parentStoredTrust(origin)}
	if create {
		if len(raw) != 0 || expectedPin != [32]byte{} {
			return nil, ErrParentTracking
		}
	} else {
		if len(raw) == 0 || expectedPin == [32]byte{} || sha256.Sum256(raw) != expectedPin {
			return nil, ErrParentTracking
		}
		s, err = decodeParentTracking(raw)
		if err != nil || s.Origin != parentStoredTrust(origin) {
			return nil, ErrParentTracking
		}
	}
	head, _, err := parentTrackingReplay(ctx, s, current)
	if err != nil {
		return nil, err
	}
	return &ParentFinalityTracker{store: store, state: s, raw: raw, pin: expectedPin, head: head, observed: current, usable: true}, nil
}

// Explicit PRIVATE export only, not a diagnostic field or financial authority.
func (p *ParentFinalityTracker) RetentionPin() ([32]byte, error) {
	if p == nil {
		return [32]byte{}, ErrParentTracking
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.usable || p.closed {
		return [32]byte{}, ErrParentTrackingStorage
	}
	return p.pin, nil
}

type ParentTrackingCandidate struct {
	owner      *ParentFinalityTracker
	prior, pin [32]byte
	raw        []byte
	state      parentTrackingState
	head       ParentBeaconHeader
	current    uint64
}

func (*ParentTrackingCandidate) String() string {
	return "conditional parent tracking candidate (redacted)"
}
func (p *ParentTrackingCandidate) GoString() string { return p.String() }
func (*ParentTrackingCandidate) MarshalJSON() ([]byte, error) {
	return []byte(`{"redacted":true}`), nil
}

// Retain this PRIVATE planned pin BEFORE Commit, alongside the old pin, for
// explicit old/new outcome recovery if a synchronous storage write fails.
func (p *ParentTrackingCandidate) RetentionPin() [32]byte {
	if p == nil {
		return [32]byte{}
	}
	return p.pin
}

func (p *ParentFinalityTracker) checkStored() (err error) {
	defer func() {
		if recover() != nil {
			err = ErrParentTrackingStorage
		}
		if err != nil {
			p.usable = false
		}
	}()
	raw, err := p.store.Load()
	if err != nil || len(raw) > MaxParentTrackingBytes || !bytes.Equal(raw, p.raw) {
		return ErrParentTrackingStorage
	}
	return nil
}

// Review is read-only. It owns all variable bytes BEFORE any context callback,
// checks the exact retained storage revision and verifies the next head against
// the retained finalized state. Caller must not mutate slices during entry.
// A same-slot replay is refused, not treated as a new head or successful write.
func (p *ParentFinalityTracker) Review(ctx context.Context, current uint64, proof ParentFinalityProof, next *ParentNextCommitteeProof) (candidate *ParentTrackingCandidate, err error) {
	if p == nil {
		return nil, ErrParentTracking
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() {
		if recover() != nil {
			candidate, err = nil, ErrParentTracking
		}
	}()
	if !p.usable || p.closed {
		return nil, ErrParentTrackingStorage
	}
	if len(p.state.Steps) >= MaxParentTrackingSteps {
		return nil, ErrParentTracking
	}
	step := parentTrackingStep{CurrentSlot: current, Proof: parentStoreProof(proof)}
	if next != nil {
		n := parentStoredNext(*next)
		step.Next = &n
	}
	s := p.state
	s.Steps = append(append([]parentTrackingStep(nil), p.state.Steps...), step)
	raw, err := encodeParentTracking(s)
	if err != nil {
		return nil, err
	}
	// Canonical private codec detaches all nested slices, including prior steps.
	s, err = decodeParentTracking(raw)
	if err != nil {
		return nil, err
	}
	if ctx == nil || ctx.Err() != nil || !parentTrackingFresh(ParentFinalityTrust(s.Origin), current) || current < p.observed {
		return nil, ErrParentTracking
	}
	if proof.Finalized.Slot <= p.head.Slot {
		return nil, ErrParentTrackingConflict
	}
	if err := p.checkStored(); err != nil {
		return nil, err
	}
	head, _, err := parentTrackingReplay(ctx, s, current)
	if err != nil {
		return nil, err
	}
	return &ParentTrackingCandidate{owner: p, prior: p.pin, pin: sha256.Sum256(raw), raw: raw, state: s, head: head, current: current}, nil
}

// Commit accepts ONLY an unmodified candidate prepared by this live tracker at
// its current revision and the same explicitly supplied clock slot. Caller
// retains both pins before this write. Any Save error/panic disables the instance;
// reopening either retained outcome requires its separately retained pin and a
// full replay. No retry, stale-candidate overwrite or cancellation rollback.
func (p *ParentFinalityTracker) Commit(ctx context.Context, current uint64, c *ParentTrackingCandidate) (err error) {
	if p == nil {
		return ErrParentTracking
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.usable || p.closed {
		return ErrParentTrackingStorage
	}
	if c == nil || c.owner != p || c.prior != p.pin {
		return ErrParentTrackingConflict
	}
	if ctx == nil || current != c.current || !parentTrackingFresh(ParentFinalityTrust(p.state.Origin), current) {
		return ErrParentTracking
	}
	// Context panics before the write do not create an uncertain write outcome.
	if err := parentTrackingContext(ctx); err != nil {
		return err
	}
	if err := p.checkStored(); err != nil {
		return err
	}
	if err := parentTrackingContext(ctx); err != nil {
		return err
	}
	defer func() {
		if recover() != nil {
			err = ErrParentTrackingStorage
		}
		if err != nil {
			p.usable, p.uncertain = false, true
		}
	}()
	if p.store.Save(bytes.Clone(c.raw)) != nil {
		return ErrParentTrackingStorage
	}
	p.state, p.raw, p.pin, p.head, p.observed = c.state, bytes.Clone(c.raw), c.pin, c.head, current
	return nil
}

func parentTrackingContext(ctx context.Context) (err error) {
	defer func() {
		if recover() != nil {
			err = ErrParentTracking
		}
	}()
	if ctx.Err() != nil {
		return ErrParentTracking
	}
	return nil
}

func (p *ParentFinalityTracker) Close() (err error) {
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
			err = ErrParentTrackingStorage
		}
	}()
	if p.store.Close() != nil {
		return ErrParentTrackingStorage
	}
	return nil
}
