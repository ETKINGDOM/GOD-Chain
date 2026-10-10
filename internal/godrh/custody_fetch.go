package godrh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/core/types"
)

var ErrCustodyReceiptFetch = errors.New("RH read-only custody receipt fetch rejected; no financial resolution or dispatch retry")

// Explicit read-only injection. Neither the adapter nor its finalized tag is
// an independent source-finality verifier. No transport is installed by fetch.
type CustodyReceiptSource interface {
	ReceiptSetSource
	Receipt(context.Context, [32]byte) (Receipt, error)
}

type CustodyReceiptFetchReport struct {
	CustodyReconciliationReport
	SourceMaterialRead        bool `json:"sourceMaterialRead"`
	ReceiptMetadataMatched    bool `json:"receiptMetadataMatched"`
	ProviderReferencesChecked bool `json:"providerReferencesChecked"`
}

// Ephemeral PRIVATE material. Ordinary formatting exposes only closed gates
// and local checks. Fetch never retains a review, resolves unknown or signs.
type FetchedCustodyReceipt struct {
	block    Block
	material BlockMaterial
	report   CustodyReceiptFetchReport
	ready    bool
}

func (FetchedCustodyReceipt) String() string {
	return "RH simulation fetched custody receipt (redacted)"
}
func (f FetchedCustodyReceipt) GoString() string             { return f.String() }
func (f FetchedCustodyReceipt) MarshalJSON() ([]byte, error) { return json.Marshal(f.Report()) }
func (f FetchedCustodyReceipt) Report() CustodyReceiptFetchReport {
	if !f.ready {
		return CustodyReceiptFetchReport{}
	}
	return f.report
}

// Explicit detached access for separate PRIVATE retention/local review. These
// bytes are not a transferable proof or authority to mutate a native ledger.
func (f FetchedCustodyReceipt) Material() (Block, BlockMaterial, error) {
	if !f.ready {
		return Block{}, BlockMaterial{}, ErrCustodyReceiptFetch
	}
	m, err := detachReceiptSetMaterial(f.material)
	if err != nil {
		return Block{}, BlockMaterial{}, ErrCustodyReceiptFetch
	}
	return f.block, m, nil
}

func detachCustodyReceipt(r Receipt, transaction [32]byte) (Receipt, error) {
	if transaction == [32]byte{} || r.TransactionHash != transaction || !validDiscoveryBlock(r.Block) || r.TransactionIndex >= MaxReceiptSetTransactions || len(r.Logs) > MaxReceiptLogs || !r.Success && len(r.Logs) != 0 {
		return Receipt{}, ErrCustodyReceiptFetch
	}
	for n, l := range r.Logs {
		if l.Removed || l.Emitter == [20]byte{} || l.Index >= MaxReceiptSetLogs || len(l.Topics) > 4 || len(l.Data) > MaxLogDataBytes || n > 0 && l.Index <= r.Logs[n-1].Index {
			return Receipt{}, ErrCustodyReceiptFetch
		}
	}
	out := r
	out.Logs = make([]ReceiptLog, len(r.Logs))
	for n, l := range r.Logs {
		out.Logs[n] = l
		out.Logs[n].Topics = append([][32]byte(nil), l.Topics...)
		out.Logs[n].Data = bytes.Clone(l.Data)
	}
	return out, nil
}

func sameCustodyReceipt(a, b Receipt) bool {
	if a.Block != b.Block || a.TransactionHash != b.TransactionHash || a.TransactionIndex != b.TransactionIndex || a.Success != b.Success || len(a.Logs) != len(b.Logs) {
		return false
	}
	for n, l := range a.Logs {
		x := b.Logs[n]
		if l.Emitter != x.Emitter || l.Index != x.Index || l.Removed != x.Removed || len(l.Topics) != len(x.Topics) || !bytes.Equal(l.Data, x.Data) {
			return false
		}
		for i, topic := range l.Topics {
			if x.Topics[i] != topic {
				return false
			}
		}
	}
	return true
}

// Called only AFTER complete-set/root/exact-envelope checks. Match every
// selected log and its block-wide index, derived from all preceding receipts.
func custodyReceiptMaterialMatches(m BlockMaterial, r Receipt, raw []byte) bool {
	index := int(r.TransactionIndex)
	if index >= len(m.Transactions) || index >= len(m.Receipts) || !bytes.Equal(m.Transactions[index], raw) {
		return false
	}
	var offset uint32
	for n := 0; n <= index; n++ {
		var receipt types.Receipt
		if receipt.UnmarshalBinary(m.Receipts[n]) != nil {
			return false
		}
		if n != index {
			offset += uint32(len(receipt.Logs))
			continue
		}
		if (receipt.Status == types.ReceiptStatusSuccessful) != r.Success || len(receipt.Logs) != len(r.Logs) {
			return false
		}
		for ordinal, l := range receipt.Logs {
			x := r.Logs[ordinal]
			if l == nil || [20]byte(l.Address) != x.Emitter || offset+uint32(ordinal) != x.Index || len(l.Topics) != len(x.Topics) || !bytes.Equal(l.Data, x.Data) {
				return false
			}
			for i, topic := range l.Topics {
				if [32]byte(topic) != x.Topics[i] {
					return false
				}
			}
		}
	}
	return true
}

// One bounded read, not a poller/retry loop. Caller must explicitly provide a
// cancellation-honoring adapter that NEVER reenters book APIs. b.mu is held for
// the whole read; mutex acquisition is not context-interruptible. Provider
// references are consistency claims, NOT ancestry, code/state or RH/L1 finality.
func (b *CustodyNonceBook) FetchReceiptForSimulation(ctx context.Context, i CustodyTransactionInputs, source CustodyReceiptSource) (fetched FetchedCustodyReceipt, err error) {
	fail := func() (FetchedCustodyReceipt, error) { return FetchedCustodyReceipt{}, ErrCustodyReceiptFetch }
	defer func() {
		if recover() != nil {
			fetched, err = fail()
		}
	}()
	if b == nil || source == nil || ctx == nil || ctx.Err() != nil {
		return fail()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready(ctx) || b.recheck(ctx) != nil {
		return fail()
	}
	wanted, checked, err := b.candidate(i)
	if err != nil {
		return fail()
	}
	matched := false
	for _, e := range b.state.Entries {
		matched = matched || e.Binding.Stage == CustodyAttemptUnknown && sameCustodyAttempt(e.Binding, wanted.Binding) && e.Nonce == wanted.Nonce && e.MaximumGasCost == wanted.MaximumGasCost
	}
	if !matched {
		return fail()
	}
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	chainMatches := func() bool {
		chain, err := source.ChainID(readCtx)
		return err == nil && readCtx.Err() == nil && chain != nil && chain.Cmp(b.chain) == 0
	}
	blockMatches := func(block Block) bool {
		if readCtx.Err() != nil || !validDiscoveryBlock(block) {
			return false
		}
		actual, err := source.Block(readCtx, block.Height)
		return err == nil && readCtx.Err() == nil && actual == block
	}
	if !chainMatches() {
		return fail()
	}
	checkpoint, err := source.FinalizedBlock(readCtx)
	if err != nil || readCtx.Err() != nil || !blockMatches(checkpoint) {
		return fail()
	}
	receipt, err := source.Receipt(readCtx, checked.hash)
	if err != nil {
		return fail()
	}
	// Own all slices BEFORE the next provider callback can reuse its buffers.
	receipt, err = detachCustodyReceipt(receipt, checked.hash)
	if err != nil || readCtx.Err() != nil || receipt.Block.Height > checkpoint.Height || !blockMatches(receipt.Block) {
		return fail()
	}
	raw, err := source.BlockMaterial(readCtx, receipt.Block)
	if err != nil {
		return fail()
	}
	material, err := detachReceiptSetMaterial(raw)
	if err != nil || readCtx.Err() != nil {
		return fail()
	}
	review, report, err := checkCustodyExecution(readCtx, i, checked, receipt.Block, material)
	if err != nil || !custodyReceiptMaterialMatches(material, receipt, checked.raw) {
		return fail()
	}
	for _, retained := range b.state.Reviews {
		if retained.TransactionHash == review.TransactionHash && retained != review {
			return fail()
		}
	}
	afterReceipt, err := source.Receipt(readCtx, checked.hash)
	if err != nil {
		return fail()
	}
	afterReceipt, err = detachCustodyReceipt(afterReceipt, checked.hash)
	if err != nil || readCtx.Err() != nil || !sameCustodyReceipt(receipt, afterReceipt) || !blockMatches(receipt.Block) || !blockMatches(checkpoint) {
		return fail()
	}
	after, err := source.FinalizedBlock(readCtx)
	if err != nil || readCtx.Err() != nil || after.Height < checkpoint.Height || after.Height == checkpoint.Height && after != checkpoint || !blockMatches(after) || !chainMatches() {
		return fail()
	}
	// Recheck original private files/head after ALL callbacks; no write/cache.
	if b.recheck(readCtx) != nil {
		return fail()
	}
	if _, _, err := b.candidate(i); err != nil || readCtx.Err() != nil {
		return fail()
	}
	return FetchedCustodyReceipt{block: receipt.Block, material: material, report: CustodyReceiptFetchReport{CustodyReconciliationReport: report, SourceMaterialRead: true, ReceiptMetadataMatched: true, ProviderReferencesChecked: true}, ready: true}, nil
}

var _ CustodyReceiptSource = (*HTTPReceiptSetSource)(nil)
