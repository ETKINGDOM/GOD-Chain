package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"

	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

// Local simulation admission limits, not RH workload or protocol parameters.
const (
	MaxReceiptSetTransactions = 256
	MaxReceiptSetLogs         = 512
	MaxReceiptSetBytes        = 1 << 20
	maxSetHeaderBytes         = 4096
	maxSetItemBytes           = 128 << 10
)

// BlockMaterial is PRIVATE canonical execution-layer data. Receipts use their
// consensus binary envelopes, not JSON metadata or storage/network encodings.
// Transactions must not carry blob sidecars. All slices are untrusted inputs.
type BlockMaterial struct {
	Header       []byte
	Transactions [][]byte
	Receipts     [][]byte
}

func (BlockMaterial) String() string               { return "RH simulation block material (redacted)" }
func (m BlockMaterial) GoString() string           { return m.String() }
func (BlockMaterial) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

// ReceiptSetSource is a replaceable read-only adapter, not a finality verifier.
// It must honor context cancellation and never sign, mutate or broadcast. No
// adapter implies source finality or RH-specific encoding compatibility.
type ReceiptSetSource interface {
	Source
	BlockMaterial(context.Context, Block) (BlockMaterial, error)
}

type receiptSetDiscovery struct{ ReceiptSetSource }

// Private local record of a successful complete-set scan, NOT a transferable
// inclusion/finality certificate. Optional fields preserve earlier journal
// bytes. The checksum is corruption detection, not authentication of the owner.
type relayReceiptBinding struct {
	Block            Block
	TransactionIndex uint32
	Sequence         uint64
	EventDigest      [32]byte
}

func validRelayReceiptBinding(b *relayReceiptBinding, r relayRequest, cursor *relayCursor) bool {
	if b == nil {
		return true
	}
	return validDiscoveryBlock(b.Block) && b.TransactionIndex < MaxReceiptSetTransactions && r.LogIndex < MaxReceiptSetLogs &&
		b.Sequence != 0 && b.EventDigest != [32]byte{} && cursor != nil && validRelayCursor(cursor) &&
		b.Block.Height >= cursor.Origin.Height && b.Block.Height <= cursor.Through.Height &&
		(b.Block.Height != cursor.Origin.Height || b.Block.Hash == cursor.Origin.Hash) &&
		(b.Block.Height != cursor.Through.Height || b.Block.Hash == cursor.Through.Hash) &&
		(r.Withdrawal == nil || b.Sequence == r.Withdrawal.Sequence)
}

func receiptEventDigest(e SourceLog) [32]byte {
	// Explicit fields avoid the deliberately redacted SourceLog/ReceiptLog JSON
	// methods. Encoding is canonical, domain-separated and length-delimited.
	raw, _ := json.Marshal(struct {
		Purpose          string
		Block            Block
		Transaction      [32]byte
		TransactionIndex uint32
		Emitter          [20]byte
		Index            uint32
		Topics           [][32]byte
		Data             []byte
	}{"GOD Chain simulation receipt-set event binding v1", e.Block, e.TransactionHash, e.TransactionIndex,
		e.Log.Emitter, e.Log.Index, e.Log.Topics, append([]byte(nil), e.Log.Data...)})
	return sha256.Sum256(raw)
}

func (s receiptSetDiscovery) Logs(ctx context.Context, custody [20]byte, block Block) ([]SourceLog, error) {
	material, err := s.BlockMaterial(ctx, block)
	if err != nil || ctx.Err() != nil {
		return nil, ErrSource
	}
	return receiptSetEvents(ctx, material, custody, block)
}

// receiptSetEvents verifies complete ordered transaction and receipt tries
// AGAINST the requested header hash. It does not authenticate that header's
// canonical chain/finality, validate signatures, execute transactions or verify
// state/code roots. A provider can still fabricate a mutually consistent fork.
func receiptSetEvents(ctx context.Context, material BlockMaterial, custody [20]byte, block Block) ([]SourceLog, error) {
	if ctx == nil || ctx.Err() != nil || custody == [20]byte{} || !validDiscoveryBlock(block) {
		return nil, ErrSource
	}
	var err error
	material, err = detachReceiptSetMaterial(material)
	if err != nil {
		return nil, ErrSource
	}
	var header types.Header
	if rlp.DecodeBytes(material.Header, &header) != nil || header.Number == nil || !header.Number.IsUint64() ||
		header.Number.Uint64() != block.Height || [32]byte(header.Hash()) != block.Hash || header.GasUsed > header.GasLimit {
		return nil, ErrSource
	}
	encoded, err := rlp.EncodeToBytes(&header)
	if err != nil || !bytes.Equal(encoded, material.Header) {
		return nil, ErrSource
	}
	txs := make(types.Transactions, len(material.Transactions))
	receipts := make(types.Receipts, len(material.Receipts))
	seen := map[common.Hash]bool{}
	var cumulative uint64
	logCount := 0
	var events []SourceLog
	topics := bridgeEventTopics()
	for i := range txs {
		if ctx.Err() != nil {
			return nil, ErrSource
		}
		var tx types.Transaction
		if tx.UnmarshalBinary(material.Transactions[i]) != nil || tx.BlobTxSidecar() != nil {
			return nil, ErrSource
		}
		encoded, err := tx.MarshalBinary()
		if err != nil || !bytes.Equal(encoded, material.Transactions[i]) || seen[tx.Hash()] {
			return nil, ErrSource
		}
		seen[tx.Hash()], txs[i] = true, &tx
		var receipt types.Receipt
		if receipt.UnmarshalBinary(material.Receipts[i]) != nil || receipt.Type != tx.Type() ||
			len(receipt.PostState) != 0 || receipt.Status > types.ReceiptStatusSuccessful ||
			receipt.CumulativeGasUsed <= cumulative || receipt.CumulativeGasUsed > header.GasUsed ||
			len(receipt.Logs) > MaxReceiptLogs || len(receipt.Logs) > MaxReceiptSetLogs-logCount ||
			receipt.Status == types.ReceiptStatusFailed && len(receipt.Logs) != 0 {
			return nil, ErrSource
		}
		encoded, err = receipt.MarshalBinary()
		if err != nil || !bytes.Equal(encoded, material.Receipts[i]) {
			return nil, ErrSource
		}
		for _, log := range receipt.Logs {
			if log == nil || len(log.Topics) > 4 || len(log.Data) > MaxLogDataBytes {
				return nil, ErrSource
			}
			if [20]byte(log.Address) == custody && len(log.Topics) > 0 &&
				([32]byte(log.Topics[0]) == topics[0] || [32]byte(log.Topics[0]) == topics[1] || [32]byte(log.Topics[0]) == topics[2]) {
				if len(events) == MaxDiscoveryLogs {
					return nil, ErrSource
				}
				event := SourceLog{Block: block, TransactionHash: [32]byte(tx.Hash()), TransactionIndex: uint32(i),
					Log: ReceiptLog{Emitter: custody, Index: uint32(logCount), Data: bytes.Clone(log.Data)}}
				for _, topic := range log.Topics {
					event.Log.Topics = append(event.Log.Topics, [32]byte(topic))
				}
				events = append(events, event)
			}
			logCount++
		}
		if receipt.Bloom != types.CreateBloom(&receipt) {
			return nil, ErrSource
		}
		cumulative, receipts[i] = receipt.CumulativeGasUsed, &receipt
	}
	if ctx.Err() != nil || cumulative != header.GasUsed || header.Bloom != types.MergeBloom(receipts) ||
		header.TxHash != types.DeriveSha(txs, trie.NewStackTrie(nil)) ||
		header.ReceiptHash != types.DeriveSha(receipts, trie.NewStackTrie(nil)) || validateSourceLogs(events, custody, block) != nil || ctx.Err() != nil {
		return nil, ErrSource
	}
	return events, nil
}

// Both scanning and proof preparation bound input BEFORE allocation/decoding.
// Callers must not concurrently mutate their input slices. Returned material
// owns every slice and is never persisted or shared with an external adapter.
func detachReceiptSetMaterial(material BlockMaterial) (BlockMaterial, error) {
	if len(material.Header) == 0 || len(material.Header) > maxSetHeaderBytes ||
		len(material.Transactions) > MaxReceiptSetTransactions || len(material.Transactions) != len(material.Receipts) {
		return BlockMaterial{}, ErrSource
	}
	total := len(material.Header)
	for _, items := range [][][]byte{material.Transactions, material.Receipts} {
		for _, raw := range items {
			if len(raw) == 0 || len(raw) > maxSetItemBytes || len(raw) > MaxReceiptSetBytes-total {
				return BlockMaterial{}, ErrSource
			}
			total += len(raw)
		}
	}
	clone := func(items [][]byte) [][]byte {
		out := make([][]byte, len(items))
		for i := range items {
			out[i] = bytes.Clone(items[i])
		}
		return out
	}
	return BlockMaterial{Header: bytes.Clone(material.Header), Transactions: clone(material.Transactions), Receipts: clone(material.Receipts)}, nil
}

// These are local simulation admission bounds, not source-chain parameters.
const (
	MaxReceiptInclusionNodes = 16 // per ordered proof, including the root
	maxInclusionNodeBytes    = maxSetItemBytes + maxSetHeaderBytes
)

// ReceiptInclusionWitness contains PRIVATE canonical consensus envelopes and
// two root-to-leaf lists of hashed trie nodes. Embedded nodes stay in their
// parent, never as extra list entries. It carries no finality or state proof.
type ReceiptInclusionWitness struct {
	Header           []byte
	Transaction      []byte
	Receipt          []byte
	TransactionProof [][]byte
	ReceiptProof     [][]byte
}

func (ReceiptInclusionWitness) String() string               { return "RH simulation inclusion witness (redacted)" }
func (w ReceiptInclusionWitness) GoString() string           { return w.String() }
func (ReceiptInclusionWitness) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

// ReceiptInclusionRequest pins the header and transaction identity supplied by
// the caller. ReceiptLogOrdinal is an offset WITHIN that receipt, never the
// block-wide RPC log index used by bridge evidence or the relay journal.
type ReceiptInclusionRequest struct {
	Block             Block
	TransactionHash   [32]byte
	TransactionIndex  uint32
	ReceiptLogOrdinal uint32
	Custody           [20]byte
}

func (ReceiptInclusionRequest) String() string               { return "RH simulation inclusion request (redacted)" }
func (r ReceiptInclusionRequest) GoString() string           { return r.String() }
func (ReceiptInclusionRequest) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

// ReceiptLocalEvent deliberately is NOT a ReceiptLog, SourceLog or bridge
// Evidence. No block-wide index can be inferred from a single receipt proof.
// Explicit access is private operational data and must not be logged/published.
type ReceiptLocalEvent struct {
	Block             Block
	TransactionHash   [32]byte
	TransactionIndex  uint32
	ReceiptLogOrdinal uint32
	Emitter           [20]byte
	Topics            [][32]byte
	Data              []byte
}

func (ReceiptLocalEvent) String() string               { return "RH simulation receipt-local event (redacted)" }
func (e ReceiptLocalEvent) GoString() string           { return e.String() }
func (ReceiptLocalEvent) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type ReceiptInclusionObservation struct {
	event ReceiptLocalEvent
	ready bool
}

type ReceiptInclusionReport struct {
	HeaderMatched               bool `json:"headerMatched"`
	TransactionIncluded         bool `json:"transactionIncluded"`
	ReceiptIncluded             bool `json:"receiptIncluded"`
	ReceiptLocalLogSelected     bool `json:"receiptLocalLogSelected"`
	BlockLogIndexVerified       bool `json:"blockLogIndexVerified"`
	IndependentFinalityVerified bool `json:"independentFinalityVerified"`
	ApprovalReady               bool `json:"approvalReady"`
	RealAssetsReady             bool `json:"realAssetsReady"`
}

func (ReceiptInclusionObservation) String() string {
	return "RH simulation inclusion observation (redacted)"
}
func (o ReceiptInclusionObservation) GoString() string             { return o.String() }
func (o ReceiptInclusionObservation) MarshalJSON() ([]byte, error) { return json.Marshal(o.Report()) }
func (o ReceiptInclusionObservation) Report() ReceiptInclusionReport {
	return ReceiptInclusionReport{HeaderMatched: o.ready, TransactionIncluded: o.ready,
		ReceiptIncluded: o.ready, ReceiptLocalLogSelected: o.ready}
}
func (o ReceiptInclusionObservation) Event() (ReceiptLocalEvent, error) {
	if !o.ready {
		return ReceiptLocalEvent{}, ErrSource
	}
	e := o.event
	e.Topics, e.Data = append([][32]byte(nil), e.Topics...), bytes.Clone(e.Data)
	return e, nil
}

var ErrRelayReceiptProof = errors.New("RH simulation task proof preparation rejected; no journal change or financial approval")

// RelayReceiptProofObservation is a PRIVATE, ephemeral review against one
// retained complete-set task. It cannot be serialized as an evidence bundle;
// Selection returns only the receipt-local request needed to recheck its proof.
type RelayReceiptProofObservation struct {
	ticket    uint64
	kind      string
	cached    bool
	selection ReceiptInclusionRequest
	ready     bool
}

type RelayReceiptProofReport struct {
	Ticket                           uint64 `json:"ticket"`
	Kind                             string `json:"kind"`
	SimulationOnly                   bool   `json:"simulationOnly"`
	ReceiptSetBindingMatched         bool   `json:"receiptSetBindingMatched"`
	CompleteSetLogPositionMatched    bool   `json:"completeSetLogPositionMatched"`
	TaskRequestMatched               bool   `json:"taskRequestMatched"`
	CachedUnsignedObservationMatched bool   `json:"cachedUnsignedObservationMatched"`
	TransactionIncluded              bool   `json:"transactionIncluded"`
	ReceiptIncluded                  bool   `json:"receiptIncluded"`
	ProofPersisted                   bool   `json:"proofPersisted"`
	IndependentFinalityVerified      bool   `json:"independentFinalityVerified"`
	ApprovalReady                    bool   `json:"approvalReady"`
	RealAssetsReady                  bool   `json:"realAssetsReady"`
}

func (RelayReceiptProofObservation) String() string {
	return "RH simulation task proof review (redacted)"
}
func (o RelayReceiptProofObservation) GoString() string             { return o.String() }
func (o RelayReceiptProofObservation) MarshalJSON() ([]byte, error) { return json.Marshal(o.Report()) }
func (o RelayReceiptProofObservation) Report() RelayReceiptProofReport {
	if !o.ready {
		return RelayReceiptProofReport{}
	}
	return RelayReceiptProofReport{Ticket: o.ticket, Kind: o.kind, SimulationOnly: true,
		ReceiptSetBindingMatched: true, CompleteSetLogPositionMatched: true, TaskRequestMatched: true,
		CachedUnsignedObservationMatched: o.cached, TransactionIncluded: true, ReceiptIncluded: true}
}
func (o RelayReceiptProofObservation) Selection() (ReceiptInclusionRequest, error) {
	if !o.ready {
		return ReceiptInclusionRequest{}, ErrRelayReceiptProof
	}
	return o.selection, nil
}

// PrepareTaskReceiptInclusion generates and rechecks a proof for the EXACT
// event in a retained complete-set task, using caller-supplied complete data.
// It derives the receipt-local ordinal from ALL preceding receipt logs, then
// checks the block-wide index, original event digest, ABI/request and any cache.
// No source adapter is called: this does not recheck current provider anchors,
// authenticate headers/finality, code or terminal/native state. The whole set
// is discarded; the returned two proofs cannot carry block-wide index assurance.
// Manual/hint tasks are rejected, never promoted. This serialized read never
// saves proof bytes, advances the cursor/clock, claims work or changes retries,
// cache or financial authority, including for pending/exhausted tasks.
func (j *RelayJournal) PrepareTaskReceiptInclusion(ctx context.Context, ticket uint64, material BlockMaterial) (witness ReceiptInclusionWitness, observation RelayReceiptProofObservation, err error) {
	defer func() {
		if recover() != nil {
			witness, observation, err = ReceiptInclusionWitness{}, RelayReceiptProofObservation{}, ErrRelayReceiptProof
		}
	}()
	fail := func() (ReceiptInclusionWitness, RelayReceiptProofObservation, error) {
		return ReceiptInclusionWitness{}, RelayReceiptProofObservation{}, ErrRelayReceiptProof
	}
	if j == nil || ctx == nil {
		return fail()
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.prepareTaskReceiptInclusion(ctx, ticket, material)
}

// The caller holds j.mu throughout review, including a separate proof-store
// commit when requested. Never call an external adapter from this helper.
func (j *RelayJournal) prepareTaskReceiptInclusion(ctx context.Context, ticket uint64, material BlockMaterial) (witness ReceiptInclusionWitness, observation RelayReceiptProofObservation, err error) {
	defer func() {
		if recover() != nil {
			witness, observation, err = ReceiptInclusionWitness{}, RelayReceiptProofObservation{}, ErrRelayReceiptProof
		}
	}()
	fail := func() (ReceiptInclusionWitness, RelayReceiptProofObservation, error) {
		return ReceiptInclusionWitness{}, RelayReceiptProofObservation{}, ErrRelayReceiptProof
	}
	if !j.usable || j.closed || ctx.Err() != nil || ticket == 0 || ticket > uint64(len(j.state.Tasks)) || j.validate(j.state) != nil {
		return fail()
	}
	task := j.state.Tasks[ticket-1]
	if task.ReceiptSet == nil {
		return fail()
	}
	// Detach once before any further context hook or data-dependent work. Both
	// event matching and proof generation must consume this SAME private set.
	material, err = detachReceiptSetMaterial(material)
	if err != nil {
		return fail()
	}
	custody, err := address(j.config.private.CustodyContract)
	if err != nil {
		return fail()
	}
	events, err := receiptSetEvents(ctx, material, custody, task.ReceiptSet.Block)
	if err != nil {
		return fail()
	}
	var selected *SourceLog
	for i := range events {
		e := &events[i]
		if e.TransactionHash == task.Request.Transaction && e.Log.Index == task.Request.LogIndex {
			selected = e
			break
		}
	}
	if selected == nil || selected.TransactionIndex != task.ReceiptSet.TransactionIndex ||
		receiptEventDigest(*selected) != task.ReceiptSet.EventDigest {
		return fail()
	}
	digest, err := j.taskReceiptEventDigest(task, *selected, custody)
	if err != nil {
		return fail()
	}
	if task.Cache != nil && (!task.Cache.ReceiptSetMatched || task.Cache.Sequence != task.ReceiptSet.Sequence || task.Cache.Digest != digest ||
		task.Cache.Evidence != (godbridge.Evidence{Height: selected.Block.Height, BlockHash: selected.Block.Hash,
			TransactionHash: selected.TransactionHash, LogIndex: selected.Log.Index})) {
		return fail()
	}
	// The selected RPC index includes unrelated logs in earlier transactions.
	// Never pass it directly to the receipt-local inclusion verifier.
	preceding := uint32(0)
	for i := uint32(0); i < selected.TransactionIndex; i++ {
		var receipt types.Receipt
		if ctx.Err() != nil || receipt.UnmarshalBinary(material.Receipts[i]) != nil || uint64(preceding)+uint64(len(receipt.Logs)) > MaxReceiptSetLogs {
			return fail()
		}
		preceding += uint32(len(receipt.Logs))
	}
	if selected.Log.Index < preceding || selected.Log.Index-preceding >= MaxReceiptLogs {
		return fail()
	}
	request := ReceiptInclusionRequest{Block: selected.Block, TransactionHash: selected.TransactionHash,
		TransactionIndex: selected.TransactionIndex, ReceiptLogOrdinal: selected.Log.Index - preceding, Custody: custody}
	witness, inclusion, err := PrepareReceiptInclusion(ctx, material, request)
	if err != nil {
		return fail()
	}
	local, err := inclusion.Event()
	if err != nil || local.Block != selected.Block || local.TransactionHash != selected.TransactionHash ||
		local.TransactionIndex != selected.TransactionIndex || local.ReceiptLogOrdinal != request.ReceiptLogOrdinal || local.Emitter != selected.Log.Emitter ||
		len(local.Topics) != len(selected.Log.Topics) || !bytes.Equal(local.Data, selected.Log.Data) {
		return fail()
	}
	for i := range local.Topics {
		if local.Topics[i] != selected.Log.Topics[i] {
			return fail()
		}
	}
	if ctx.Err() != nil {
		return fail()
	}
	return witness, RelayReceiptProofObservation{ticket: ticket, kind: task.Request.Kind, cached: task.Cache != nil, selection: request, ready: true}, nil
}

// Recompute the unsigned proposal digest from retained request + exact event,
// not from provider metadata. This is a consistency check, not authorization.
func (j *RelayJournal) taskReceiptEventDigest(task relayTask, event SourceLog, custody [20]byte) ([32]byte, error) {
	protocol, err := j.config.Binding()
	if err != nil {
		return [32]byte{}, ErrRelayReceiptProof
	}
	if task.Request.Kind == "deposit" {
		d, err := decodeDeposit(event.Log, custody, Receipt{Block: event.Block, TransactionHash: event.TransactionHash})
		if err != nil || d.Sequence != task.ReceiptSet.Sequence || d.Recipient != task.Request.Recipient || d.Amount.String() != task.Request.Amount {
			return [32]byte{}, ErrRelayReceiptProof
		}
		return godbridge.DepositAttestationDigest(protocol, d)
	}
	outcome := godbridge.Status(task.Request.Kind)
	_, w, err := resolutionRequest(j.config, ResolutionRequest{TransactionHash: task.Request.Transaction, LogIndex: task.Request.LogIndex,
		Withdrawal: *task.Request.Withdrawal, Outcome: outcome})
	if err != nil || w.Sequence != task.ReceiptSet.Sequence || decodeResolutionEvent(event.Log, custody, w, outcome) != nil {
		return [32]byte{}, ErrRelayReceiptProof
	}
	return godbridge.ResolutionAttestationDigest(protocol, w, outcome, godbridge.Evidence{Height: event.Block.Height, BlockHash: event.Block.Hash,
		TransactionHash: event.TransactionHash, LogIndex: event.Log.Index})
}

// PrepareReceiptInclusion verifies a COMPLETE bounded block material snapshot,
// builds both selected proofs in uncommitted in-memory tries and independently
// rechecks them with VerifyReceiptInclusion before returning any PRIVATE data.
// This is not a source adapter, retained certificate or financial relay. The
// returned observation still lacks a block-wide log index and finality/approval;
// complete-set scan progress and existing task assurances are never changed.
func PrepareReceiptInclusion(ctx context.Context, material BlockMaterial, request ReceiptInclusionRequest) (witness ReceiptInclusionWitness, observation ReceiptInclusionObservation, err error) {
	defer func() {
		if recover() != nil {
			witness, observation, err = ReceiptInclusionWitness{}, ReceiptInclusionObservation{}, ErrSource
		}
	}()
	fail := func() (ReceiptInclusionWitness, ReceiptInclusionObservation, error) {
		return ReceiptInclusionWitness{}, ReceiptInclusionObservation{}, ErrSource
	}
	if ctx == nil || ctx.Err() != nil || !validDiscoveryBlock(request.Block) || request.TransactionHash == [32]byte{} || request.Custody == [20]byte{} ||
		request.TransactionIndex >= MaxReceiptSetTransactions || request.ReceiptLogOrdinal >= MaxReceiptLogs {
		return fail()
	}
	// Keep one detached snapshot for verification AND generation. Validation
	// of caller slices followed by generation from caller slices would race a
	// later change even though the old scan had already matched its roots.
	material, err = detachReceiptSetMaterial(material)
	if err != nil || uint64(request.TransactionIndex) >= uint64(len(material.Transactions)) {
		return fail()
	}
	if _, err = receiptSetEvents(ctx, material, request.Custody, request.Block); err != nil {
		return fail()
	}
	var header types.Header
	if rlp.DecodeBytes(material.Header, &header) != nil {
		return fail()
	}
	// Full-set verification already checked canonical consensus envelopes.
	// Check the request before doing additional trie construction work.
	var tx types.Transaction
	var receipt types.Receipt
	if tx.UnmarshalBinary(material.Transactions[request.TransactionIndex]) != nil || [32]byte(tx.Hash()) != request.TransactionHash ||
		receipt.UnmarshalBinary(material.Receipts[request.TransactionIndex]) != nil || receipt.Status != types.ReceiptStatusSuccessful ||
		uint64(request.ReceiptLogOrdinal) >= uint64(len(receipt.Logs)) {
		return fail()
	}
	selected := receipt.Logs[request.ReceiptLogOrdinal]
	topics := bridgeEventTopics()
	if [20]byte(selected.Address) != request.Custody || len(selected.Topics) == 0 ||
		([32]byte(selected.Topics[0]) != topics[0] && [32]byte(selected.Topics[0]) != topics[1] && [32]byte(selected.Topics[0]) != topics[2]) {
		return fail()
	}
	witness = ReceiptInclusionWitness{Header: bytes.Clone(material.Header), Transaction: bytes.Clone(material.Transactions[request.TransactionIndex]),
		Receipt: bytes.Clone(material.Receipts[request.TransactionIndex])}
	remaining := MaxReceiptSetBytes - len(witness.Header) - len(witness.Transaction) - len(witness.Receipt)
	var used int
	witness.TransactionProof, used, err = prepareInclusionProof(ctx, material.Transactions, request.TransactionIndex, header.TxHash, remaining)
	if err != nil {
		return fail()
	}
	witness.ReceiptProof, _, err = prepareInclusionProof(ctx, material.Receipts, request.TransactionIndex, header.ReceiptHash, remaining-used)
	if err != nil {
		return fail()
	}
	observation, err = VerifyReceiptInclusion(ctx, witness, request)
	if err != nil || ctx.Err() != nil {
		return fail()
	}
	return witness, observation, nil
}

// No database, resolver, Commit or external callback is used. With this pinned
// trie implementation, hashing retains all constructed nodes in memory. Root
// and key checks are repeated even though the complete material was checked.
func prepareInclusionProof(ctx context.Context, items [][]byte, index uint32, root common.Hash, budget int) ([][]byte, int, error) {
	if ctx == nil || ctx.Err() != nil || len(items) == 0 || len(items) > MaxReceiptSetTransactions || uint64(index) >= uint64(len(items)) || budget <= 0 || budget > MaxReceiptSetBytes {
		return nil, 0, ErrSource
	}
	tr := trie.NewEmpty(nil)
	for i, raw := range items {
		if ctx.Err() != nil || len(raw) == 0 || len(raw) > maxSetItemBytes || tr.Update(rlp.AppendUint64(nil, uint64(i)), raw) != nil {
			return nil, 0, ErrSource
		}
	}
	if tr.Hash() != root || ctx.Err() != nil {
		return nil, 0, ErrSource
	}
	writer := inclusionProofWriter{ctx: ctx, budget: budget}
	// The pinned Prove implementation does not propagate Put errors. A sticky
	// writer failure is mandatory; a truncated proof must never be returned.
	if tr.Prove(rlp.AppendUint64(nil, uint64(index)), &writer) != nil || writer.failed || len(writer.nodes) == 0 || ctx.Err() != nil {
		return nil, 0, ErrSource
	}
	return writer.nodes, writer.used, nil
}

type inclusionProofWriter struct {
	ctx          context.Context
	budget, used int
	nodes        [][]byte
	seen         map[common.Hash]bool
	failed       bool
}

func (w *inclusionProofWriter) Put(key, node []byte) error {
	if w.failed || w.ctx == nil || w.ctx.Err() != nil || len(key) != 32 || len(w.nodes) >= MaxReceiptInclusionNodes ||
		len(node) == 0 || len(node) > maxInclusionNodeBytes || len(node) > w.budget-w.used ||
		len(w.nodes) > 0 && len(node) < 32 || !validInclusionNode(node, 0) {
		w.failed = true
		return ErrSource
	}
	h := ethcrypto.Keccak256Hash(node)
	if !bytes.Equal(key, h[:]) || w.seen[h] {
		w.failed = true
		return ErrSource
	}
	if w.seen == nil {
		w.seen = map[common.Hash]bool{}
	}
	w.seen[h] = true
	w.nodes = append(w.nodes, bytes.Clone(node))
	w.used += len(node)
	return nil
}
func (w *inclusionProofWriter) Delete([]byte) error { w.failed = true; return ErrSource }

// VerifyReceiptInclusion checks both roots at the SAME RLP-encoded transaction
// position against an explicitly supplied header hash. The selected successful
// receipt contains a custody-topic log. It does NOT authenticate that header,
// finality, transaction signatures/execution, global log index, complete block,
// code/state, event ABI or native withdrawal state. No adapter, journal, signer
// or ledger is called. In particular, this result cannot release/approve GOD.
func VerifyReceiptInclusion(ctx context.Context, witness ReceiptInclusionWitness, request ReceiptInclusionRequest) (out ReceiptInclusionObservation, err error) {
	// Never expose decoder diagnostics or panic payloads containing private data.
	defer func() {
		if recover() != nil {
			out, err = ReceiptInclusionObservation{}, ErrSource
		}
	}()
	fail := func() (ReceiptInclusionObservation, error) { return ReceiptInclusionObservation{}, ErrSource }
	if ctx == nil || ctx.Err() != nil || !validDiscoveryBlock(request.Block) || request.TransactionHash == [32]byte{} ||
		request.Custody == [20]byte{} || request.TransactionIndex >= MaxReceiptSetTransactions || request.ReceiptLogOrdinal >= MaxReceiptLogs ||
		len(witness.Header) == 0 || len(witness.Header) > maxSetHeaderBytes ||
		len(witness.Transaction) == 0 || len(witness.Transaction) > maxSetItemBytes ||
		len(witness.Receipt) == 0 || len(witness.Receipt) > maxSetItemBytes {
		return fail()
	}
	total := len(witness.Header) + len(witness.Transaction) + len(witness.Receipt)
	for _, nodes := range [][][]byte{witness.TransactionProof, witness.ReceiptProof} {
		if len(nodes) == 0 || len(nodes) > MaxReceiptInclusionNodes {
			return fail()
		}
		for _, node := range nodes {
			if len(node) == 0 || len(node) > maxInclusionNodeBytes || len(node) > MaxReceiptSetBytes-total {
				return fail()
			}
			total += len(node)
		}
	}
	// Callers must not concurrently mutate input slices. Detach all admitted
	// input before parsing; no witness bytes are retained by the observation.
	clone := func(nodes [][]byte) [][]byte {
		out := make([][]byte, len(nodes))
		for i := range nodes {
			out[i] = bytes.Clone(nodes[i])
		}
		return out
	}
	witness.Header, witness.Transaction, witness.Receipt = bytes.Clone(witness.Header), bytes.Clone(witness.Transaction), bytes.Clone(witness.Receipt)
	witness.TransactionProof, witness.ReceiptProof = clone(witness.TransactionProof), clone(witness.ReceiptProof)
	var header types.Header
	if rlp.DecodeBytes(witness.Header, &header) != nil || header.Number == nil || !header.Number.IsUint64() ||
		header.Number.Uint64() != request.Block.Height || [32]byte(header.Hash()) != request.Block.Hash || header.GasUsed > header.GasLimit {
		return fail()
	}
	encoded, e := rlp.EncodeToBytes(&header)
	if e != nil || !bytes.Equal(encoded, witness.Header) {
		return fail()
	}
	var tx types.Transaction
	if tx.UnmarshalBinary(witness.Transaction) != nil || tx.BlobTxSidecar() != nil || [32]byte(tx.Hash()) != request.TransactionHash {
		return fail()
	}
	encoded, e = tx.MarshalBinary()
	if e != nil || !bytes.Equal(encoded, witness.Transaction) {
		return fail()
	}
	var receipt types.Receipt
	if receipt.UnmarshalBinary(witness.Receipt) != nil || receipt.Type != tx.Type() || len(receipt.PostState) != 0 ||
		receipt.Status != types.ReceiptStatusSuccessful || receipt.CumulativeGasUsed == 0 || receipt.CumulativeGasUsed > header.GasUsed ||
		len(receipt.Logs) > MaxReceiptLogs || uint64(request.ReceiptLogOrdinal) >= uint64(len(receipt.Logs)) {
		return fail()
	}
	encoded, e = receipt.MarshalBinary()
	if e != nil || !bytes.Equal(encoded, witness.Receipt) {
		return fail()
	}
	for _, log := range receipt.Logs {
		if log == nil || len(log.Topics) > 4 || len(log.Data) > MaxLogDataBytes {
			return fail()
		}
	}
	if receipt.Bloom != types.CreateBloom(&receipt) {
		return fail()
	}
	for i, bits := range receipt.Bloom {
		if header.Bloom[i]&bits != bits {
			return fail()
		}
	}
	key := rlp.AppendUint64(nil, uint64(request.TransactionIndex))
	if !inclusionValue(ctx, header.TxHash, key, witness.TransactionProof, witness.Transaction) ||
		!inclusionValue(ctx, header.ReceiptHash, key, witness.ReceiptProof, witness.Receipt) {
		return fail()
	}
	log := receipt.Logs[request.ReceiptLogOrdinal]
	topics := bridgeEventTopics()
	if [20]byte(log.Address) != request.Custody || len(log.Topics) == 0 ||
		([32]byte(log.Topics[0]) != topics[0] && [32]byte(log.Topics[0]) != topics[1] && [32]byte(log.Topics[0]) != topics[2]) || ctx.Err() != nil {
		return fail()
	}
	event := ReceiptLocalEvent{Block: request.Block, TransactionHash: request.TransactionHash, TransactionIndex: request.TransactionIndex,
		ReceiptLogOrdinal: request.ReceiptLogOrdinal, Emitter: request.Custody, Data: bytes.Clone(log.Data)}
	for _, topic := range log.Topics {
		event.Topics = append(event.Topics, [32]byte(topic))
	}
	return ReceiptInclusionObservation{event: event, ready: true}, nil
}

// Ordered reads enforce Keccak binding (the upstream proof DB must supply
// this), prohibit cycles/repeated/unused nodes and bound decoder work. Missing,
// absence, tampered and noncanonical-node proofs all fail with static errors.
type inclusionProofReader struct {
	ctx    context.Context
	nodes  [][]byte
	hashes []common.Hash
	next   int
}

func (r *inclusionProofReader) Has(key []byte) (bool, error) {
	if r.ctx.Err() != nil {
		return false, ErrSource
	}
	return r.next < len(r.nodes) && bytes.Equal(key, r.hashes[r.next][:]), nil
}
func (r *inclusionProofReader) Get(key []byte) ([]byte, error) {
	if r.ctx.Err() != nil || r.next == len(r.nodes) || !bytes.Equal(key, r.hashes[r.next][:]) {
		return nil, ErrSource
	}
	node := r.nodes[r.next]
	r.next++
	return node, nil
}
func inclusionValue(ctx context.Context, root common.Hash, key []byte, nodes [][]byte, expected []byte) bool {
	r := inclusionProofReader{ctx: ctx, nodes: nodes}
	seen := map[common.Hash]bool{}
	for i, node := range nodes {
		if ctx.Err() != nil || i > 0 && len(node) < 32 || !validInclusionNode(node, 0) {
			return false
		}
		h := ethcrypto.Keccak256Hash(node)
		if seen[h] {
			return false
		}
		seen[h] = true
		r.hashes = append(r.hashes, h)
	}
	value, err := trie.VerifyProof(root, key, &r)
	return err == nil && len(value) != 0 && bytes.Equal(value, expected) && r.next == len(nodes) && ctx.Err() == nil
}

// Check exact RLP boundaries and hex-prefix canonicality before the pinned
// decoder, including inline descendants. This is proof encoding validation,
// not complete source-block execution or validation of unseen hashed nodes.
func validInclusionNode(raw []byte, depth int) bool {
	if depth >= MaxReceiptInclusionNodes {
		return false
	}
	items, rest, err := rlp.SplitList(raw)
	if err != nil || len(rest) != 0 {
		return false
	}
	count, err := rlp.CountValues(items)
	if err != nil || count != 2 && count != 17 {
		return false
	}
	var parts []rlp.RawValue
	if rlp.DecodeBytes(raw, &parts) != nil {
		return false
	}
	ref := func(raw []byte, allowEmpty bool) bool {
		kind, value, rest, err := rlp.Split(raw)
		if err != nil || len(rest) != 0 {
			return false
		}
		if kind == rlp.List {
			return len(raw) < 32 && validInclusionNode(raw, depth+1)
		}
		return len(value) == 32 || allowEmpty && len(value) == 0
	}
	if len(parts) == 2 {
		var path []byte
		if rlp.DecodeBytes(parts[0], &path) != nil || len(path) == 0 || path[0]>>4 > 3 || path[0]&0x10 == 0 && path[0]&15 != 0 {
			return false
		}
		if path[0]&0x20 != 0 {
			var value []byte
			return rlp.DecodeBytes(parts[1], &value) == nil && len(value) != 0
		}
		return (len(path) > 1 || path[0]&0x10 != 0) && ref(parts[1], false)
	}
	if len(parts) != 17 {
		return false
	}
	children := 0
	for _, child := range parts[:16] {
		if !ref(child, true) {
			return false
		}
		if !bytes.Equal(child, []byte{0x80}) {
			children++
		}
	}
	var value []byte
	if rlp.DecodeBytes(parts[16], &value) != nil {
		return false
	}
	if len(value) != 0 {
		children++
	}
	return children >= 2
}
