package godrh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
)

var ErrCustodyAccountFetch = errors.New("RH simulation read-only account proof fetch rejected; no dispatch or source-state authority")

// Explicit cancellation-honoring injection, never installed by fetch. A source
// must not concurrently mutate its returned bytes. Its finalized tag is a
// consistency claim, not independent header, ancestry or settlement evidence.
type CustodyAccountSource interface {
	ChainID(context.Context) (*big.Int, error)
	FinalizedBlock(context.Context) (Block, error)
	Block(context.Context, uint64) (Block, error)
	AccountProof(context.Context, [20]byte, Block) (CustodyAccountProof, error)
}

type CustodyAccountFetchReport struct {
	CustodyAccountProofReport
	SourceMaterialRead        bool `json:"sourceMaterialRead"`
	ProviderReferencesChecked bool `json:"providerReferencesChecked"`
}

// Ephemeral PRIVATE proof only. No snapshot store, nonce book, signer, server,
// retry worker or native ledger is available to this object or fetch API.
type FetchedCustodyAccount struct {
	block  Block
	proof  CustodyAccountProof
	report CustodyAccountFetchReport
	ready  bool
}

func (FetchedCustodyAccount) String() string {
	return "RH simulation fetched sender account proof (redacted)"
}
func (f FetchedCustodyAccount) GoString() string             { return f.String() }
func (f FetchedCustodyAccount) MarshalJSON() ([]byte, error) { return json.Marshal(f.Report()) }
func (f FetchedCustodyAccount) Report() CustodyAccountFetchReport {
	if !f.ready {
		return CustodyAccountFetchReport{}
	}
	return f.report
}
func (f FetchedCustodyAccount) Material() (Block, CustodyAccountProof, error) {
	if !f.ready {
		return Block{}, CustodyAccountProof{}, ErrCustodyAccountFetch
	}
	p, err := detachCustodyAccountProof(f.proof)
	if err != nil {
		return Block{}, CustodyAccountProof{}, ErrCustodyAccountFetch
	}
	return f.block, p, nil
}

// Explicit one-shot SIMULATION read against a caller-retained height/hash.
// Never chooses latest/pending, retries or acquires account-state authority.
// The deadline is cooperative for injected sources; the HTTP adapter also has
// per-request timeouts. A source that ignores cancellation violates this API.
func FetchCustodyAccountProofForSimulation(ctx context.Context, inputs CustodyTransactionInputs, block Block, source CustodyAccountSource) (fetched FetchedCustodyAccount, err error) {
	fail := func() (FetchedCustodyAccount, error) { return FetchedCustodyAccount{}, ErrCustodyAccountFetch }
	defer func() {
		if recover() != nil {
			fetched, err = fail()
		}
	}()
	if ctx == nil || ctx.Err() != nil || source == nil || !validDiscoveryBlock(block) {
		return fail()
	}
	checked, err := loadCustodyTransaction(inputs)
	if err != nil || !checked.report.SimulationConfiguration || ctx.Err() != nil {
		return fail()
	}
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	chainMatches := func() bool {
		chain, err := source.ChainID(readCtx)
		return err == nil && readCtx.Err() == nil && chain != nil && chain.Cmp(checked.chain) == 0
	}
	blockMatches := func(wanted Block) bool {
		if readCtx.Err() != nil || !validDiscoveryBlock(wanted) {
			return false
		}
		actual, err := source.Block(readCtx, wanted.Height)
		return err == nil && readCtx.Err() == nil && actual == wanted
	}
	if !chainMatches() {
		return fail()
	}
	checkpoint, err := source.FinalizedBlock(readCtx)
	if err != nil || readCtx.Err() != nil || !blockMatches(checkpoint) || block.Height > checkpoint.Height ||
		block.Height == checkpoint.Height && block != checkpoint || !blockMatches(block) {
		return fail()
	}
	raw, err := source.AccountProof(readCtx, checked.sender, block)
	if err != nil {
		return fail()
	}
	// Detach BEFORE any context/provider callback can reuse source buffers.
	proof, err := detachCustodyAccountProof(raw)
	if err != nil || readCtx.Err() != nil {
		return fail()
	}
	report, err := checkCustodyAccountProof(readCtx, checked, block, proof)
	if err != nil || !blockMatches(block) || !blockMatches(checkpoint) {
		return fail()
	}
	after, err := source.FinalizedBlock(readCtx)
	if err != nil || readCtx.Err() != nil || after.Height < checkpoint.Height ||
		after.Height == checkpoint.Height && after != checkpoint || !blockMatches(after) || !chainMatches() {
		return fail()
	}
	// Original whole-file pins, not newly computed replacements. No write/cache.
	again, err := loadCustodyTransaction(inputs)
	if err != nil || !again.report.SimulationConfiguration || !bytes.Equal(again.raw, checked.raw) || readCtx.Err() != nil {
		return fail()
	}
	return FetchedCustodyAccount{block: block, proof: proof, report: CustodyAccountFetchReport{CustodyAccountProofReport: report, SourceMaterialRead: true, ProviderReferencesChecked: true}, ready: true}, nil
}
