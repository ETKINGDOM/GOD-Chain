package godrh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
)

var ErrTaskReceiptFetch = errors.New("RH read-only task material fetch rejected; no journal change or financial approval")

// FetchedTaskReceiptProof is PRIVATE, ephemeral material and proof for one
// immutable complete-set task. Provider agreement within the fetch is not
// authenticated source truth or authority for later signing/asset movement.
// There is no journal/source/store pointer or future freshness guarantee.
type FetchedTaskReceiptProof struct {
	material   BlockMaterial
	witness    ReceiptInclusionWitness
	review     RelayReceiptProofObservation
	checkpoint Block // last provider checkpoint, private and unauthenticated
	ready      bool
}

type TaskReceiptFetchReport struct {
	RelayReceiptProofReport
	SourceMaterialRead        bool `json:"sourceMaterialRead"`
	ProviderReferencesChecked bool `json:"providerReferencesChecked"`
}

func (FetchedTaskReceiptProof) String() string {
	return "RH simulation fetched task proof (redacted)"
}
func (f FetchedTaskReceiptProof) GoString() string             { return f.String() }
func (f FetchedTaskReceiptProof) MarshalJSON() ([]byte, error) { return json.Marshal(f.Report()) }
func (f FetchedTaskReceiptProof) Report() TaskReceiptFetchReport {
	if !f.ready {
		return TaskReceiptFetchReport{}
	}
	return TaskReceiptFetchReport{RelayReceiptProofReport: f.review.Report(), SourceMaterialRead: true, ProviderReferencesChecked: true}
}

// Material returns a detached private snapshot for explicit separate retention.
// The proof store must still recheck it against its original task; fetch flags
// are not saved certificates. Never log/publish any of these raw bytes.
func (f FetchedTaskReceiptProof) Material() (BlockMaterial, error) {
	if !f.ready {
		return BlockMaterial{}, ErrTaskReceiptFetch
	}
	material, err := detachReceiptSetMaterial(f.material)
	if err != nil {
		return BlockMaterial{}, ErrTaskReceiptFetch
	}
	return material, nil
}

// Proof returns new owned bytes and the existing receipt-local selection/review.
// The selected two paths still cannot carry complete-set global log counts.
func (f FetchedTaskReceiptProof) Proof() (ReceiptInclusionWitness, RelayReceiptProofObservation, error) {
	if !f.ready {
		return ReceiptInclusionWitness{}, RelayReceiptProofObservation{}, ErrTaskReceiptFetch
	}
	clone := func(nodes [][]byte) [][]byte {
		out := make([][]byte, len(nodes))
		for i, node := range nodes {
			out[i] = bytes.Clone(node)
		}
		return out
	}
	w := f.witness
	w.Header, w.Transaction, w.Receipt = bytes.Clone(w.Header), bytes.Clone(w.Transaction), bytes.Clone(w.Receipt)
	w.TransactionProof, w.ReceiptProof = clone(w.TransactionProof), clone(w.ReceiptProof)
	return w, f.review, nil
}

// FetchTaskReceiptProof is an explicit one-shot READ, not scheduled observation
// work. It never calls Receipt/Code/Call/Logs, writes a store, claims a task or
// changes stage/cache/clock/cursor/retries. Exhausted tasks are not reactivated.
// Only tasks originally bound by complete-set scanning can select material.
//
// The journal lock is held through the read and proof checks. Adapters must
// honor cancellation and must not reenter journal/proof-store APIs. The active
// read has the existing 30-second context ceiling; mutex wait is not context-
// interruptible. Provider pre/postflight guards do NOT verify authentic finality,
// code/terminal/native state or execution and do not replace ObserveNext.
func (j *RelayJournal) FetchTaskReceiptProof(ctx context.Context, ticket uint64, source ReceiptSetSource) (fetched FetchedTaskReceiptProof, err error) {
	defer func() {
		if recover() != nil {
			fetched, err = FetchedTaskReceiptProof{}, ErrTaskReceiptFetch
		}
	}()
	fail := func() (FetchedTaskReceiptProof, error) { return FetchedTaskReceiptProof{}, ErrTaskReceiptFetch }
	if j == nil || ctx == nil || source == nil {
		return fail()
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.fetchTaskReceiptProof(ctx, ticket, source)
}

// The caller holds j.mu. This shared read also serves explicit retained-proof
// source review without recursively acquiring the journal lock.
func (j *RelayJournal) fetchTaskReceiptProof(ctx context.Context, ticket uint64, source ReceiptSetSource) (FetchedTaskReceiptProof, error) {
	fail := func() (FetchedTaskReceiptProof, error) { return FetchedTaskReceiptProof{}, ErrTaskReceiptFetch }
	pin, _, err := j.taskReceiptProofPin(ctx, ticket)
	if err != nil {
		return fail()
	}
	binding := *j.state.Tasks[ticket-1].ReceiptSet
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	before, err := j.observationAnchors(readCtx, source, &binding)
	if err != nil {
		return fail()
	}
	raw, err := source.BlockMaterial(readCtx, binding.Block)
	if err != nil {
		return fail()
	}
	// Detach immediately, BEFORE any further context hook or provider callback.
	// Task review and the returned retention snapshot use this same private data.
	material, err := detachReceiptSetMaterial(raw)
	if err != nil || readCtx.Err() != nil {
		return fail()
	}
	witness, review, err := j.prepareTaskReceiptInclusion(readCtx, ticket, material)
	if err != nil {
		return fail()
	}
	after, err := j.observationAnchors(readCtx, source, &binding, before)
	if err != nil {
		return fail()
	}
	afterPin, _, err := j.taskReceiptProofPin(readCtx, ticket)
	if err != nil || afterPin != pin || readCtx.Err() != nil {
		return fail()
	}
	return FetchedTaskReceiptProof{material: material, witness: witness, review: review, checkpoint: after, ready: true}, nil
}
