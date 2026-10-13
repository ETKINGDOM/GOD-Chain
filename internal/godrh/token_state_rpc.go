package godrh

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

const maxTokenStateRPCBytes = 2*MaxContractStateProofBytes + (32 << 10)

// The caller retains this execution header separately. A matching header hash
// is integrity, not RH/L1 source identity, fork policy or independent finality.
func tokenStateHeaderMatches(raw []byte, block Block) bool {
	if len(raw) == 0 || len(raw) > maxSetHeaderBytes || !validDiscoveryBlock(block) {
		return false
	}
	var h types.Header
	if rlp.DecodeBytes(raw, &h) != nil || h.Number == nil || !h.Number.IsUint64() || h.Number.Uint64() != block.Height ||
		[32]byte(h.Hash()) != block.Hash || h.Root == (common.Hash{}) || h.GasUsed > h.GasLimit {
		return false
	}
	canonical, err := rlp.EncodeToBytes(&h)
	return err == nil && bytes.Equal(raw, canonical)
}

// Proof response keys may use either canonical uint256 QUANTITY or the
// requested 32-byte DATA form. Both must encode the exact requested raw key.
func tokenStateRPCKey(raw []byte) ([32]byte, error) {
	var key [32]byte
	text, err := rpcText(raw)
	if err != nil || strings.ToLower(text) != text {
		return key, ErrSource
	}
	if len(text) == 66 {
		value, err := custodyHex(raw, 32)
		if err != nil {
			return key, ErrSource
		}
		copy(key[:], value)
		return key, nil
	}
	n, err := quantity(text)
	if err != nil {
		return key, ErrSource
	}
	n.FillBytes(key[:])
	return key, nil
}

// Closed EIP-1186 response framing plus actual canonical account/storage proof
// verification. Provider metadata must match the proved account, never replace
// it. Runtime comes from a separate exact-hash code read, not a response field.
func parseTokenStateRPC(ctx context.Context, raw, header, runtime []byte, block Block, binding ContractStateBinding) (proof ContractStateProof, err error) {
	fail := func() (ContractStateProof, error) { return ContractStateProof{}, ErrSource }
	defer func() {
		if recover() != nil {
			proof, err = fail()
		}
	}()
	if len(raw) == 0 || len(raw) > maxTokenStateRPCBytes || !utf8.Valid(raw) || !uniqueTokenSourceJSON(raw) ||
		len(header) == 0 || len(header) > maxSetHeaderBytes || len(runtime) == 0 || len(runtime) > maxCodeBytes ||
		binding.Address == [20]byte{} || binding.RuntimeHash == [32]byte{} || len(binding.Slots) > MaxContractStateSlots {
		return fail()
	}
	// Own variable caller bytes before ANY context callback.
	raw, header, runtime = bytes.Clone(raw), bytes.Clone(header), bytes.Clone(runtime)
	binding.Slots = append([]ContractStorageBinding(nil), binding.Slots...)
	if ctx == nil || ctx.Err() != nil || !tokenStateHeaderMatches(header, block) {
		return fail()
	}
	f, err := custodyObject(raw, []string{"address", "accountProof", "balance", "codeHash", "nonce", "storageHash", "storageProof"}, nil)
	if err != nil {
		return fail()
	}
	// Reuse the bounded account/metadata verifier without ignoring supplied
	// storage: every original storage entry is checked one-for-one below.
	accountFields := make(map[string]json.RawMessage, len(f))
	for k, v := range f {
		accountFields[k] = v
	}
	accountFields["storageProof"] = json.RawMessage(`[]`)
	accountRaw, err := json.Marshal(accountFields)
	if err != nil {
		return fail()
	}
	a, err := parseCustodyAccountRPC(ctx, accountRaw, header, binding.Address, block)
	if err != nil {
		return fail()
	}
	p := ContractStateProof{Header: a.Header, AccountNodes: a.AccountNodes, Runtime: runtime, StorageNodes: make([][][]byte, len(binding.Slots))}
	total := len(p.Header) + len(p.Runtime)
	for _, node := range p.AccountNodes {
		total += len(node)
	}
	err = parentMaterialArray(f["storageProof"], len(binding.Slots), func(i int, item json.RawMessage) error {
		fields, err := custodyObject(item, []string{"key", "value", "proof"}, nil)
		if err != nil {
			return ErrSource
		}
		key, err := tokenStateRPCKey(fields["key"])
		text, textErr := rpcText(fields["value"])
		value, valueErr := quantity(text)
		if err != nil || textErr != nil || valueErr != nil || key != binding.Slots[i].Key {
			return ErrSource
		}
		var word [32]byte
		value.FillBytes(word[:])
		if word != binding.Slots[i].Value {
			return ErrSource
		}
		return tokenStateArray(fields["proof"], MaxCustodyAccountProofNodes, func(item json.RawMessage) error {
			if ctx.Err() != nil {
				return ErrSource
			}
			node, err := tokenStateData(item, min(maxCustodyAccountNodeBytes, MaxContractStateProofBytes-total))
			if err != nil {
				return ErrSource
			}
			total += len(node)
			p.StorageNodes[i] = append(p.StorageNodes[i], node)
			return nil
		})
	})
	if err != nil || ctx.Err() != nil {
		return fail()
	}
	if _, err := VerifyContractStateProof(ctx, block, binding, p); err != nil {
		return fail()
	}
	return p, nil
}

// Only this configured token and four read methods. No eth_call, logs,
// receipts, raw-header/debug method, signer, broadcaster, persistence or retry.
type HTTPTokenStateSource struct {
	source  *HTTPSource
	token   [20]byte
	runtime [32]byte
}

func (*HTTPTokenStateSource) String() string {
	return "GOD Chain conditional read-only token state source (redacted)"
}
func (s *HTTPTokenStateSource) GoString() string { return s.String() }
func (*HTTPTokenStateSource) MarshalJSON() ([]byte, error) {
	return []byte(`{"readOnly":true,"redacted":true}`), nil
}

func NewHTTPTokenStateSource(c Config) (*HTTPTokenStateSource, error) {
	r := c.TokenReport()
	if !r.ConnectionReady || !r.RuntimePinConfigured {
		return nil, ErrConfig
	}
	s, err := newHTTPSource(c.private.RPCEndpoint)
	if err != nil {
		return nil, ErrSource
	}
	s.stateReads = true
	token, _ := address(c.private.TokenContract)
	runtime, _ := hash(c.private.ExpectedTokenCodeHash)
	return &HTTPTokenStateSource{source: s, token: token, runtime: runtime}, nil
}

func (s *HTTPTokenStateSource) base() *HTTPSource {
	if s == nil {
		return nil
	}
	return s.source
}
func (s *HTTPTokenStateSource) Close() { s.base().Close() }
func (s *HTTPTokenStateSource) ChainID(ctx context.Context) (*big.Int, error) {
	return s.base().ChainID(ctx)
}
func (s *HTTPTokenStateSource) Block(ctx context.Context, height uint64) (Block, error) {
	return s.base().Block(ctx, height)
}

func (s *HTTPTokenStateSource) ContractStateProof(ctx context.Context, block Block, b ContractStateBinding, header []byte) (proof ContractStateProof, err error) {
	fail := func() (ContractStateProof, error) { return ContractStateProof{}, ErrSource }
	defer func() {
		if recover() != nil {
			proof, err = fail()
		}
	}()
	if s == nil || s.source == nil || b.Address != s.token || b.RuntimeHash != s.runtime || len(b.Slots) > MaxContractStateSlots ||
		len(header) == 0 || len(header) > maxSetHeaderBytes {
		return fail()
	}
	header = bytes.Clone(header)
	b.Slots = append([]ContractStorageBinding(nil), b.Slots...)
	seen := map[[32]byte]bool{}
	for _, slot := range b.Slots {
		if seen[slot.Key] {
			return fail()
		}
		seen[slot.Key] = true
	}
	if ctx == nil || ctx.Err() != nil || !tokenStateHeaderMatches(header, block) {
		return fail()
	}
	ctx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	before, err := s.Block(ctx, block.Height)
	if err != nil || before != block {
		return fail()
	}
	runtime, err := s.source.Code(ctx, s.token, block.Hash)
	if err != nil || tokenRuntimeDigest(runtime) != s.runtime {
		return fail()
	}
	keys := []string{}
	for _, slot := range b.Slots {
		keys = append(keys, "0x"+hex.EncodeToString(slot.Key[:]))
	}
	raw, err := s.source.rpc(ctx, "eth_getProof", []any{"0x" + hex.EncodeToString(s.token[:]), keys, blockReference(block.Hash)})
	if err != nil {
		return fail()
	}
	p, err := parseTokenStateRPC(ctx, raw, header, runtime, block, b)
	if err != nil {
		return fail()
	}
	after, err := s.Block(ctx, block.Height)
	if err != nil || after != block || ctx.Err() != nil {
		return fail()
	}
	return p, nil
}

var _ TokenStateSource = (*HTTPTokenStateSource)(nil)
