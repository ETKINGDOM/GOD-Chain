package godrh

import (
	"bytes"
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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
// HTTP/RH-specific material adapter is supplied by this milestone.
type ReceiptSetSource interface {
	Source
	BlockMaterial(context.Context, Block) (BlockMaterial, error)
}

type receiptSetDiscovery struct{ ReceiptSetSource }

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
	if ctx == nil || ctx.Err() != nil || custody == [20]byte{} || !validDiscoveryBlock(block) ||
		len(material.Header) == 0 || len(material.Header) > maxSetHeaderBytes ||
		len(material.Transactions) > MaxReceiptSetTransactions || len(material.Transactions) != len(material.Receipts) {
		return nil, ErrSource
	}
	// Bound before decoding/allocation, then detach the whole input. No adapter
	// callback occurs until all generated events have detached topics and data.
	total := len(material.Header)
	for _, items := range [][][]byte{material.Transactions, material.Receipts} {
		for _, raw := range items {
			if len(raw) == 0 || len(raw) > maxSetItemBytes || len(raw) > MaxReceiptSetBytes-total {
				return nil, ErrSource
			}
			total += len(raw)
		}
	}
	material.Header = bytes.Clone(material.Header)
	clone := func(items [][]byte) [][]byte {
		out := make([][]byte, len(items))
		for i := range items {
			out[i] = bytes.Clone(items[i])
		}
		return out
	}
	material.Transactions, material.Receipts = clone(material.Transactions), clone(material.Receipts)
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
