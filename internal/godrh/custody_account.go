package godrh

import (
	"bytes"
	"context"
	"errors"
	"math"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

// Local proof admission bounds, not certified RH encoding/workload policy.
const (
	MaxCustodyAccountProofNodes = 65
	MaxCustodyAccountProofBytes = 64 << 10
	maxCustodyAccountNodeBytes  = 1024
)

var ErrCustodyAccountProof = errors.New("RH offline sender account proof rejected; no source finality or dispatch approval")

// Private canonical header and root-to-leaf hashed account-trie nodes. Embedded
// nodes stay inside their parents. The sender/key comes ONLY from the exact
// independently pinned signed custody envelope, not provider metadata.
type CustodyAccountProof struct {
	Header       []byte
	AccountNodes [][]byte
}

func (CustodyAccountProof) String() string               { return "RH sender account proof (redacted)" }
func (p CustodyAccountProof) GoString() string           { return p.String() }
func (CustodyAccountProof) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type CustodyAccountProofReport struct {
	Transaction                    CustodyTransactionReport `json:"transaction"`
	HeaderMatched                  bool                     `json:"headerMatched"`
	SenderAccountIncluded          bool                     `json:"senderAccountIncluded"`
	CanonicalAccountEncoding       bool                     `json:"canonicalAccountEncoding"`
	NonceAtHeaderMatches           bool                     `json:"nonceAtHeaderMatches"`
	ExecutionGasBudgetAtHeaderFits bool                     `json:"executionGasBudgetAtHeaderFits"`
	EmptyCodeHashAtHeader          bool                     `json:"emptyCodeHashAtHeader"`
	IndependentHeaderVerified      bool                     `json:"independentHeaderVerified"`
	SourceChainStateVerified       bool                     `json:"sourceChainStateVerified"`
	LatestAccountStateVerified     bool                     `json:"latestAccountStateVerified"`
	PendingNonceVerified           bool                     `json:"pendingNonceVerified"`
	TotalSourceFeesVerified        bool                     `json:"totalSourceFeesVerified"`
	SigningEnabled                 bool                     `json:"signingEnabled"`
	BroadcastEnabled               bool                     `json:"broadcastEnabled"`
	RealAssetsReady                bool                     `json:"realAssetsReady"`
}

func detachCustodyAccountProof(p CustodyAccountProof) (CustodyAccountProof, error) {
	if len(p.Header) == 0 || len(p.Header) > maxSetHeaderBytes || len(p.AccountNodes) == 0 || len(p.AccountNodes) > MaxCustodyAccountProofNodes {
		return CustodyAccountProof{}, ErrCustodyAccountProof
	}
	total := len(p.Header)
	for _, node := range p.AccountNodes {
		if len(node) == 0 || len(node) > maxCustodyAccountNodeBytes || len(node) > MaxCustodyAccountProofBytes-total {
			return CustodyAccountProof{}, ErrCustodyAccountProof
		}
		total += len(node)
	}
	owned := CustodyAccountProof{Header: bytes.Clone(p.Header), AccountNodes: make([][]byte, len(p.AccountNodes))}
	for i, node := range p.AccountNodes {
		owned.AccountNodes[i] = bytes.Clone(node)
	}
	return owned, nil
}

// Conditional proof check ONLY: the caller's block pin authenticates neither
// chain identity nor header origin/finality/freshness. It has no provider,
// journal, signer, disk output or ledger capability. Never authorizes dispatch.
// Callers must not concurrently mutate the proof or checked transaction.
func checkCustodyAccountProof(ctx context.Context, checked checkedCustodyTransaction, block Block, proof CustodyAccountProof) (report CustodyAccountProofReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = CustodyAccountProofReport{}, ErrCustodyAccountProof
		}
	}()
	fail := func() (CustodyAccountProofReport, error) { return CustodyAccountProofReport{}, ErrCustodyAccountProof }
	if ctx == nil || ctx.Err() != nil || !validDiscoveryBlock(block) || checked.sender == [20]byte{} || checked.nonce == math.MaxUint64 ||
		checked.maximumGasCost == nil || checked.maximumGasCost.Sign() <= 0 || checked.maximumGasCost.BitLen() > 256 {
		return fail()
	}
	proof, err = detachCustodyAccountProof(proof)
	if err != nil || ctx.Err() != nil {
		return fail()
	}
	account, err := custodyAccountValue(ctx, block, proof, checked.sender)
	if err != nil || !bytes.Equal(account.CodeHash, types.EmptyCodeHash[:]) ||
		account.Nonce != checked.nonce || account.Balance.ToBig().Cmp(checked.maximumGasCost) < 0 {
		return fail()
	}
	return CustodyAccountProofReport{Transaction: checked.report, HeaderMatched: true, SenderAccountIncluded: true,
		CanonicalAccountEncoding: true, NonceAtHeaderMatches: true, ExecutionGasBudgetAtHeaderFits: true, EmptyCodeHashAtHeader: true}, nil
}

// Internal decoder shared with the opt-in RPC adapter. The caller has already
// admitted/detached bounds and checked context, block and sender. It returns
// PRIVATE values solely to compare RPC metadata, never source authenticity.
func custodyAccountValue(ctx context.Context, block Block, proof CustodyAccountProof, sender [20]byte) (types.StateAccount, error) {
	fail := func() (types.StateAccount, error) { return types.StateAccount{}, ErrCustodyAccountProof }
	var header types.Header
	if rlp.DecodeBytes(proof.Header, &header) != nil || header.Number == nil || !header.Number.IsUint64() ||
		header.Number.Uint64() != block.Height || [32]byte(header.Hash()) != block.Hash || header.Root == (common.Hash{}) || header.GasUsed > header.GasLimit {
		return fail()
	}
	canonical, err := rlp.EncodeToBytes(&header)
	if err != nil || !bytes.Equal(canonical, proof.Header) {
		return fail()
	}
	// The upstream verifier expects the proof DB to enforce node-hash binding.
	// Reuse the strictly ordered reader, but check every admitted account node
	// independently before decoding. No duplicate, cyclic or unused nodes.
	reader := inclusionProofReader{ctx: ctx, nodes: proof.AccountNodes}
	seen := map[common.Hash]bool{}
	for i, node := range proof.AccountNodes {
		if ctx.Err() != nil || i > 0 && len(node) < 32 || !validInclusionNode(node, 0) {
			return fail()
		}
		hash := ethcrypto.Keccak256Hash(node)
		if seen[hash] {
			return fail()
		}
		seen[hash] = true
		reader.hashes = append(reader.hashes, hash)
	}
	value, err := trie.VerifyProof(header.Root, ethcrypto.Keccak256(sender[:]), &reader)
	if err != nil || len(value) == 0 || reader.next != len(proof.AccountNodes) || ctx.Err() != nil {
		return fail()
	}
	var account types.StateAccount
	if rlp.DecodeBytes(value, &account) != nil || account.Balance == nil || account.Root == (common.Hash{}) ||
		len(account.CodeHash) != 32 {
		return fail()
	}
	canonical, err = rlp.EncodeToBytes(&account)
	if err != nil || !bytes.Equal(canonical, value) || ctx.Err() != nil {
		return fail()
	}
	return account, nil
}

// Offline, opt-in review against independently retained original file pins.
// Simulation and production-mode diagnostics are both allowed; neither mode
// promotes the supplied header to authenticated RH state. A block header has
// no chain ID. Matching nonce/balance at it says nothing about pending senders,
// other fee reservations, later state or RH/L1 fees. No values are returned.
func VerifyCustodyAccountProof(ctx context.Context, inputs CustodyTransactionInputs, block Block, proof CustodyAccountProof) (report CustodyAccountProofReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = CustodyAccountProofReport{}, ErrCustodyAccountProof
		}
	}()
	fail := func() (CustodyAccountProofReport, error) { return CustodyAccountProofReport{}, ErrCustodyAccountProof }
	if ctx == nil || ctx.Err() != nil {
		return fail()
	}
	checked, err := loadCustodyTransaction(inputs)
	if err != nil || ctx.Err() != nil {
		return fail()
	}
	report, err = checkCustodyAccountProof(ctx, checked, block, proof)
	if err != nil || ctx.Err() != nil {
		return fail()
	}
	// Recheck exact original files after verification. Do not let concurrently
	// replaced private input become a new signer, nonce, envelope or fee plan.
	again, err := loadCustodyTransaction(inputs)
	if err != nil || !bytes.Equal(again.raw, checked.raw) || ctx.Err() != nil {
		return fail()
	}
	return report, nil
}
