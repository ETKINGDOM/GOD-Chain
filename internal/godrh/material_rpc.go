package godrh

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

// Local admission bounds, not RH protocol/workload guarantees. Hex and JSON
// framing have a separate bounded budget; ordinary RPC reads keep their limit.
const maxMaterialRPCBytes = 2*MaxReceiptSetBytes + 8192

// HTTPReceiptSetSource opts into exactly two additional read methods on the
// explicitly configured private endpoint. It neither enables a server's debug
// namespace nor certifies that RH supports these methods or encodings. No log
// hint fallback, signer, broadcast, storage or background loop is supplied.
type HTTPReceiptSetSource struct {
	source  *HTTPSource
	custody [20]byte
}

func (*HTTPReceiptSetSource) String() string {
	return "GOD Chain RH read-only material source (redacted)"
}
func (s *HTTPReceiptSetSource) GoString() string { return s.String() }
func (*HTTPReceiptSetSource) MarshalJSON() ([]byte, error) {
	return []byte(`{"readOnly":true,"redacted":true}`), nil
}

func NewHTTPReceiptSetSource(c Config) (*HTTPReceiptSetSource, error) {
	source, err := NewHTTPSource(c)
	if err != nil {
		return nil, err
	}
	custody, err := address(c.private.CustodyContract)
	if err != nil {
		source.Close()
		return nil, ErrConfig
	}
	source.rawReads = true
	return &HTTPReceiptSetSource{source: source, custody: custody}, nil
}

func (s *HTTPReceiptSetSource) base() *HTTPSource {
	if s == nil {
		return nil
	}
	return s.source
}
func (s *HTTPReceiptSetSource) Close() { s.base().Close() }
func (s *HTTPReceiptSetSource) ChainID(ctx context.Context) (*big.Int, error) {
	return s.base().ChainID(ctx)
}
func (s *HTTPReceiptSetSource) FinalizedBlock(ctx context.Context) (Block, error) {
	return s.base().FinalizedBlock(ctx)
}
func (s *HTTPReceiptSetSource) Block(ctx context.Context, height uint64) (Block, error) {
	return s.base().Block(ctx, height)
}
func (s *HTTPReceiptSetSource) Code(ctx context.Context, target [20]byte, block [32]byte) ([]byte, error) {
	return s.base().Code(ctx, target, block)
}
func (s *HTTPReceiptSetSource) Call(ctx context.Context, target [20]byte, input []byte, block [32]byte) ([]byte, error) {
	return s.base().Call(ctx, target, input, block)
}
func (s *HTTPReceiptSetSource) Receipt(ctx context.Context, tx [32]byte) (Receipt, error) {
	return s.base().Receipt(ctx, tx)
}

// BlockMaterial rechecks the provider's height/hash before and after two hash-
// pinned reads. The pinned backend looks these raw methods up by hash without
// independently enforcing canonicality. These guards detect inconsistent views,
// NOT authentic headers, ancestry, finality, execution or state/code truth.
func (s *HTTPReceiptSetSource) BlockMaterial(ctx context.Context, block Block) (material BlockMaterial, err error) {
	defer func() {
		if recover() != nil {
			material, err = BlockMaterial{}, ErrSource
		}
	}()
	if s == nil || s.source == nil || s.custody == [20]byte{} || ctx == nil || ctx.Err() != nil || !validDiscoveryBlock(block) {
		return BlockMaterial{}, ErrSource
	}
	ctx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	before, err := s.Block(ctx, block.Height)
	if err != nil || before != block || ctx.Err() != nil {
		return BlockMaterial{}, ErrSource
	}
	params := []any{"0x" + hex.EncodeToString(block.Hash[:])}
	wire, err := s.source.rpc(ctx, "debug_getRawBlock", params)
	if err != nil {
		return BlockMaterial{}, ErrSource
	}
	raw, err := rpcData(wire, MaxReceiptSetBytes)
	if err != nil {
		return BlockMaterial{}, ErrSource
	}
	material, err = parseRawMaterialBlock(ctx, raw, block)
	if err != nil {
		return BlockMaterial{}, ErrSource
	}
	wire, err = s.source.rpc(ctx, "debug_getRawReceipts", params)
	if err != nil {
		return BlockMaterial{}, ErrSource
	}
	material.Receipts, err = parseRawMaterialReceipts(ctx, wire, material)
	if err != nil {
		return BlockMaterial{}, ErrSource
	}
	if _, err = receiptSetEvents(ctx, material, s.custody, block); err != nil {
		return BlockMaterial{}, ErrSource
	}
	after, err := s.Block(ctx, block.Height)
	if err != nil || after != block || ctx.Err() != nil {
		return BlockMaterial{}, ErrSource
	}
	return material, nil
}

// Check counts and envelope sizes BEFORE decoding nested transaction/header
// objects. Items are temporary views into private bounded raw bytes.
func boundedRLPList(raw []byte, count, itemBytes int) ([]rlp.RawValue, error) {
	content, rest, err := rlp.SplitList(raw)
	if err != nil || len(rest) != 0 {
		return nil, ErrSource
	}
	var items []rlp.RawValue
	for len(content) != 0 {
		_, _, rest, err = rlp.Split(content)
		size := len(content) - len(rest)
		if err != nil || len(items) == count || size <= 0 || size > itemBytes {
			return nil, ErrSource
		}
		items = append(items, rlp.RawValue(content[:size]))
		content = rest
	}
	return items, nil
}

func parseRawMaterialBlock(ctx context.Context, raw []byte, block Block) (BlockMaterial, error) {
	if ctx == nil || ctx.Err() != nil || len(raw) == 0 || len(raw) > MaxReceiptSetBytes || !validDiscoveryBlock(block) {
		return BlockMaterial{}, ErrSource
	}
	fields, err := boundedRLPList(raw, 4, MaxReceiptSetBytes)
	if err != nil || len(fields) < 3 || len(fields[0]) > maxSetHeaderBytes {
		return BlockMaterial{}, ErrSource
	}
	items, err := boundedRLPList(fields[1], MaxReceiptSetTransactions, maxSetItemBytes+4)
	if err != nil {
		return BlockMaterial{}, ErrSource
	}
	if _, err = boundedRLPList(fields[2], 2, maxSetHeaderBytes); err != nil {
		return BlockMaterial{}, ErrSource
	}
	if len(fields) == 4 {
		if _, err = boundedRLPList(fields[3], 16, 128); err != nil {
			return BlockMaterial{}, ErrSource
		}
	}
	var decoded types.Block
	if rlp.DecodeBytes(raw, &decoded) != nil || decoded.Number() == nil || !decoded.Number().IsUint64() ||
		decoded.NumberU64() != block.Height || [32]byte(decoded.Hash()) != block.Hash || len(decoded.Transactions()) != len(items) {
		return BlockMaterial{}, ErrSource
	}
	canonical, err := rlp.EncodeToBytes(&decoded)
	if err != nil || !bytes.Equal(canonical, raw) {
		return BlockMaterial{}, ErrSource
	}
	header := decoded.Header()
	if types.CalcUncleHash(decoded.Uncles()) != header.UncleHash ||
		(header.WithdrawalsHash == nil) != (len(fields) == 3) ||
		header.WithdrawalsHash != nil && types.DeriveSha(decoded.Withdrawals(), trie.NewStackTrie(nil)) != *header.WithdrawalsHash {
		return BlockMaterial{}, ErrSource
	}
	material := BlockMaterial{Header: bytes.Clone(fields[0]), Transactions: make([][]byte, 0, len(items))}
	total := len(material.Header)
	for _, tx := range decoded.Transactions() {
		if ctx.Err() != nil || tx == nil || tx.BlobTxSidecar() != nil {
			return BlockMaterial{}, ErrSource
		}
		envelope, err := tx.MarshalBinary()
		if err != nil || len(envelope) == 0 || len(envelope) > maxSetItemBytes || len(envelope) > MaxReceiptSetBytes-total {
			return BlockMaterial{}, ErrSource
		}
		total += len(envelope)
		material.Transactions = append(material.Transactions, envelope)
	}
	if ctx.Err() != nil {
		return BlockMaterial{}, ErrSource
	}
	return material, nil
}

func parseRawMaterialReceipts(ctx context.Context, raw []byte, material BlockMaterial) ([][]byte, error) {
	if ctx == nil || ctx.Err() != nil || len(raw) == 0 || len(raw) > maxMaterialRPCBytes || !utf8.Valid(raw) ||
		len(material.Header) == 0 || len(material.Header) > maxSetHeaderBytes || len(material.Transactions) > MaxReceiptSetTransactions {
		return nil, ErrSource
	}
	total := len(material.Header)
	for _, tx := range material.Transactions {
		if len(tx) == 0 || len(tx) > maxSetItemBytes || len(tx) > MaxReceiptSetBytes-total {
			return nil, ErrSource
		}
		total += len(tx)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('[') {
		return nil, ErrSource
	}
	receipts := make([][]byte, 0, len(material.Transactions))
	for decoder.More() {
		if ctx.Err() != nil || len(receipts) == len(material.Transactions) {
			return nil, ErrSource
		}
		var item json.RawMessage
		if decoder.Decode(&item) != nil {
			return nil, ErrSource
		}
		budget := min(maxSetItemBytes, MaxReceiptSetBytes-total)
		envelope, err := rpcData(item, budget)
		if err != nil || len(envelope) == 0 {
			return nil, ErrSource
		}
		total += len(envelope)
		receipts = append(receipts, envelope)
	}
	last, err := decoder.Token()
	if err != nil || last != json.Delim(']') || len(receipts) != len(material.Transactions) {
		return nil, ErrSource
	}
	if _, err = decoder.Token(); err != io.EOF || ctx.Err() != nil {
		return nil, ErrSource
	}
	return receipts, nil
}

var _ ReceiptSetSource = (*HTTPReceiptSetSource)(nil)
var _ ReceiptSource = (*HTTPReceiptSetSource)(nil)
