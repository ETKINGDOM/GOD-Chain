package godrh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	"github.com/ethereum/go-ethereum/core/types"
)

var ErrTaskReceiptEvidence = errors.New("RH read-only task evidence review rejected; no journal change or financial approval")

// TaskReceiptEvidenceReview owns PRIVATE ephemeral proof and unsigned proposal
// data. It has no source, journal, signer or store pointer. Agreement during one
// read does not authenticate source finality, contract/native state or backing.
type TaskReceiptEvidenceReview struct {
	fetched    FetchedTaskReceiptProof
	deposit    DepositObservation
	resolution ResolutionObservation
	ready      bool
}

type TaskReceiptEvidenceReport struct {
	TaskReceiptFetchReport
	ReceiptDerivedFromCompleteSet bool `json:"receiptDerivedFromCompleteSet"`
	ContractViewsChecked          bool `json:"contractViewsChecked"`
	TerminalViewMatched           bool `json:"terminalViewMatched"`
	UnsignedProposalReady         bool `json:"unsignedProposalReady"`
	ContractStateProven           bool `json:"contractStateProven"`
	NativeWithdrawalStateVerified bool `json:"nativeWithdrawalStateVerified"`
	SigningEnabled                bool `json:"signingEnabled"`
	BroadcastEnabled              bool `json:"broadcastEnabled"`
}

func (TaskReceiptEvidenceReview) String() string {
	return "RH simulation task evidence review (redacted)"
}
func (r TaskReceiptEvidenceReview) GoString() string             { return r.String() }
func (r TaskReceiptEvidenceReview) MarshalJSON() ([]byte, error) { return json.Marshal(r.Report()) }
func (r TaskReceiptEvidenceReview) Report() TaskReceiptEvidenceReport {
	if !r.ready {
		return TaskReceiptEvidenceReport{}
	}
	return TaskReceiptEvidenceReport{TaskReceiptFetchReport: r.fetched.Report(),
		ReceiptDerivedFromCompleteSet: true, ContractViewsChecked: true,
		TerminalViewMatched: r.resolution.ready, UnsignedProposalReady: true}
}

// Explicit access returns detached PRIVATE bytes, not a retained certificate or
// future freshness guarantee. Proof storage must independently review its input.
func (r TaskReceiptEvidenceReview) Material() (BlockMaterial, error) {
	if !r.ready {
		return BlockMaterial{}, ErrTaskReceiptEvidence
	}
	return r.fetched.Material()
}
func (r TaskReceiptEvidenceReview) Proof() (ReceiptInclusionWitness, RelayReceiptProofObservation, error) {
	if !r.ready {
		return ReceiptInclusionWitness{}, RelayReceiptProofObservation{}, ErrTaskReceiptEvidence
	}
	return r.fetched.Proof()
}
func (r TaskReceiptEvidenceReview) DepositProposal() (DepositObservation, error) {
	if !r.ready || !r.deposit.ready {
		return DepositObservation{}, ErrTaskReceiptEvidence
	}
	o := r.deposit
	o.deposit.Amount = sdkmath.NewIntFromBigInt(o.deposit.Amount.BigInt())
	return o, nil
}
func (r TaskReceiptEvidenceReview) ResolutionProposal() (ResolutionObservation, error) {
	if !r.ready || !r.resolution.ready {
		return ResolutionObservation{}, ErrTaskReceiptEvidence
	}
	o := r.resolution
	o.withdrawal.Amount = sdkmath.NewIntFromBigInt(o.withdrawal.Amount.BigInt())
	return o, nil
}

// ReviewTaskReceiptEvidence performs one explicit simulation-only read under
// j.mu. It fetches a complete set once, verifies the original task/proof, derives
// the receipt from that SAME set and runs the existing contract/terminal checks.
// It never asks the provider for a second JSON Receipt or log hint, promotes a
// task, writes storage, changes retries/caches/clock or enables asset movement.
//
// The entire active operation has the existing 30-second ceiling; mutex wait
// is not interruptible. Adapters must honor context cancellation and must not
// reenter this journal or its proof stores. Code and view calls are provider
// claims, not state proofs. Independently authenticated headers/finality,
// native withdrawal state, operator approval and financial relaying are absent.
func (j *RelayJournal) ReviewTaskReceiptEvidence(ctx context.Context, ticket uint64, source ReceiptSetSource) (review TaskReceiptEvidenceReview, err error) {
	defer func() {
		if recover() != nil {
			review, err = TaskReceiptEvidenceReview{}, ErrTaskReceiptEvidence
		}
	}()
	fail := func() (TaskReceiptEvidenceReview, error) { return TaskReceiptEvidenceReview{}, ErrTaskReceiptEvidence }
	if j == nil || ctx == nil || ctx.Err() != nil || source == nil {
		return fail()
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	pin, _, err := j.taskReceiptProofPin(readCtx, ticket)
	if err != nil {
		return fail()
	}
	fetched, err := j.fetchTaskReceiptProof(readCtx, ticket, source)
	if err != nil {
		return fail()
	}
	receipt, err := derivedTaskReceipt(readCtx, fetched)
	if err != nil {
		return fail()
	}
	// Carry the last fetch checkpoint into EVERY subsequent finalized read;
	// even a temporary retreat followed by recovery must fail this review.
	adapter := &taskEvidenceSource{Source: source, receipt: receipt, checkpoint: fetched.checkpoint}
	task := j.state.Tasks[ticket-1]
	cache, err := j.observe(readCtx, adapter, task.Request, task.ReceiptSet)
	if err != nil || cache == nil || !cache.ReceiptSetMatched || cache.Sequence != task.ReceiptSet.Sequence ||
		cache.Evidence != (godbridge.Evidence{Height: receipt.Block.Height, BlockHash: receipt.Block.Hash,
			TransactionHash: receipt.TransactionHash, LogIndex: task.Request.LogIndex}) {
		return fail()
	}
	if _, err = j.observationAnchors(readCtx, adapter, task.ReceiptSet, fetched.checkpoint, cache.Checkpoint); err != nil {
		return fail()
	}
	afterPin, _, err := j.taskReceiptProofPin(readCtx, ticket)
	if err != nil || afterPin != pin || readCtx.Err() != nil {
		return fail()
	}
	out := TaskReceiptEvidenceReview{fetched: fetched, ready: true}
	if task.Request.Kind == "deposit" {
		amount, ok := sdkmath.NewIntFromString(task.Request.Amount)
		if !ok {
			return fail()
		}
		out.deposit = DepositObservation{deposit: godbridge.Deposit{Sequence: cache.Sequence,
			Recipient: task.Request.Recipient, Amount: amount, Evidence: cache.Evidence},
			digest: cache.Digest, checkpoint: cache.Checkpoint, ready: true}
	} else {
		w := *task.Request.Withdrawal
		w.Amount = sdkmath.NewIntFromBigInt(w.Amount.BigInt())
		out.resolution = ResolutionObservation{withdrawal: w, outcome: godbridge.Status(task.Request.Kind),
			evidence: cache.Evidence, digest: cache.Digest, checkpoint: cache.Checkpoint, ready: true}
	}
	return out, nil
}

// Only called with a verified, detached full set. Binary receipts do not carry
// RPC block/transaction/log metadata: derive it from the same verified set,
// including every earlier receipt's logs, never from the two selected proofs.
func derivedTaskReceipt(ctx context.Context, f FetchedTaskReceiptProof) (Receipt, error) {
	s := f.review.selection
	if ctx == nil || ctx.Err() != nil || !f.ready || int(s.TransactionIndex) >= len(f.material.Receipts) {
		return Receipt{}, ErrTaskReceiptEvidence
	}
	var tx types.Transaction
	if tx.UnmarshalBinary(f.material.Transactions[s.TransactionIndex]) != nil || [32]byte(tx.Hash()) != s.TransactionHash {
		return Receipt{}, ErrTaskReceiptEvidence
	}
	index := uint32(0)
	for i := uint32(0); i <= s.TransactionIndex; i++ {
		var r types.Receipt
		if ctx.Err() != nil || r.UnmarshalBinary(f.material.Receipts[i]) != nil ||
			len(r.Logs) > MaxReceiptLogs || uint64(index)+uint64(len(r.Logs)) > MaxReceiptSetLogs {
			return Receipt{}, ErrTaskReceiptEvidence
		}
		if i != s.TransactionIndex {
			index += uint32(len(r.Logs))
			continue
		}
		out := Receipt{Block: s.Block, TransactionHash: s.TransactionHash,
			TransactionIndex: s.TransactionIndex, Success: r.Status == types.ReceiptStatusSuccessful}
		for _, log := range r.Logs {
			if log == nil {
				return Receipt{}, ErrTaskReceiptEvidence
			}
			l := ReceiptLog{Emitter: [20]byte(log.Address), Index: index, Data: bytes.Clone(log.Data)}
			for _, topic := range log.Topics {
				l.Topics = append(l.Topics, [32]byte(topic))
			}
			out.Logs = append(out.Logs, l)
			index++
		}
		return out, nil
	}
	return Receipt{}, ErrTaskReceiptEvidence
}

type taskEvidenceSource struct {
	Source
	receipt    Receipt
	checkpoint Block
}

func (s *taskEvidenceSource) FinalizedBlock(ctx context.Context) (Block, error) {
	b, err := s.Source.FinalizedBlock(ctx)
	if err != nil || ctx.Err() != nil || !checkpointCovers(b, s.checkpoint) {
		return Block{}, ErrTaskReceiptEvidence
	}
	s.checkpoint = b
	return b, nil
}
func (s *taskEvidenceSource) Receipt(ctx context.Context, transaction [32]byte) (Receipt, error) {
	if ctx.Err() != nil || transaction != s.receipt.TransactionHash {
		return Receipt{}, ErrTaskReceiptEvidence
	}
	r := s.receipt
	r.Logs = append([]ReceiptLog(nil), r.Logs...)
	for i := range r.Logs {
		r.Logs[i].Data = bytes.Clone(r.Logs[i].Data)
		r.Logs[i].Topics = append([][32]byte(nil), r.Logs[i].Topics...)
	}
	return r, nil
}
