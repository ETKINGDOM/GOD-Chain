package godrh

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"unicode/utf8"
)

// Bounded JSON/hex framing, independent of the decoded 64 KiB proof budget.
const maxCustodyAccountRPCBytes = 2*MaxCustodyAccountProofBytes + 8192

// Explicit SIMULATION adapter with exactly four read methods. It does not
// enable any remote debug namespace or certify RH support for these methods.
// Hash-reference refusal, unavailable state/header or non-MPT data fails closed
// without a height/latest fallback, signer, writer, persistence or retry loop.
type HTTPCustodyAccountSource struct{ source *HTTPSource }

func (*HTTPCustodyAccountSource) String() string {
	return "GOD Chain RH simulation read-only account source (redacted)"
}
func (s *HTTPCustodyAccountSource) GoString() string { return s.String() }
func (*HTTPCustodyAccountSource) MarshalJSON() ([]byte, error) {
	return []byte(`{"readOnly":true,"redacted":true}`), nil
}

func NewHTTPCustodyAccountSource(c Config) (*HTTPCustodyAccountSource, error) {
	if c.private.Mode != "simulation" {
		return nil, ErrConfig
	}
	source, err := NewHTTPSource(c)
	if err != nil {
		return nil, err
	}
	source.accountReads = true
	return &HTTPCustodyAccountSource{source: source}, nil
}
func (s *HTTPCustodyAccountSource) base() *HTTPSource {
	if s == nil {
		return nil
	}
	return s.source
}
func (s *HTTPCustodyAccountSource) Close() { s.base().Close() }
func (s *HTTPCustodyAccountSource) ChainID(ctx context.Context) (*big.Int, error) {
	return s.base().ChainID(ctx)
}
func (s *HTTPCustodyAccountSource) FinalizedBlock(ctx context.Context) (Block, error) {
	return s.base().FinalizedBlock(ctx)
}
func (s *HTTPCustodyAccountSource) Block(ctx context.Context, height uint64) (Block, error) {
	return s.base().Block(ctx, height)
}

func (s *HTTPCustodyAccountSource) AccountProof(ctx context.Context, sender [20]byte, block Block) (proof CustodyAccountProof, err error) {
	fail := func() (CustodyAccountProof, error) { return CustodyAccountProof{}, ErrSource }
	defer func() {
		if recover() != nil {
			proof, err = fail()
		}
	}()
	if s == nil || s.source == nil || ctx == nil || ctx.Err() != nil || sender == [20]byte{} || !validDiscoveryBlock(block) {
		return fail()
	}
	ctx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	before, err := s.Block(ctx, block.Height)
	if err != nil || before != block || ctx.Err() != nil {
		return fail()
	}
	// The pinned backend's raw-header hash lookup does not itself enforce
	// canonicality. Height/hash guards detect inconsistent views, not finality.
	raw, err := s.source.rpc(ctx, "debug_getRawHeader", []any{"0x" + hex.EncodeToString(block.Hash[:])})
	if err != nil {
		return fail()
	}
	header, err := rpcData(raw, maxSetHeaderBytes)
	if err != nil || len(header) == 0 || ctx.Err() != nil {
		return fail()
	}
	raw, err = s.source.rpc(ctx, "eth_getProof", []any{"0x" + hex.EncodeToString(sender[:]), []string{}, blockReference(block.Hash)})
	if err != nil {
		return fail()
	}
	proof, err = parseCustodyAccountRPC(ctx, raw, header, sender, block)
	if err != nil {
		return fail()
	}
	after, err := s.Block(ctx, block.Height)
	if err != nil || after != block || ctx.Err() != nil {
		return fail()
	}
	return proof, nil
}

// Metadata must agree EXACTLY with the root-proven canonical account. It is
// never a substitute for the proof or an independently authenticated header.
func parseCustodyAccountRPC(ctx context.Context, raw, header []byte, sender [20]byte, block Block) (proof CustodyAccountProof, err error) {
	fail := func() (CustodyAccountProof, error) { return CustodyAccountProof{}, ErrSource }
	defer func() {
		if recover() != nil {
			proof, err = fail()
		}
	}()
	if ctx == nil || ctx.Err() != nil || sender == [20]byte{} || !validDiscoveryBlock(block) || len(header) == 0 || len(header) > maxSetHeaderBytes || len(raw) == 0 || len(raw) > maxCustodyAccountRPCBytes || !utf8.Valid(raw) {
		return fail()
	}
	f, err := custodyObject(raw, []string{"address", "accountProof", "balance", "codeHash", "nonce", "storageHash", "storageProof"}, nil)
	if err != nil {
		return fail()
	}
	text, err := rpcText(f["address"])
	accountAddress, parseErr := address(text)
	if err != nil || parseErr != nil || accountAddress != sender {
		return fail()
	}
	storageDecoder := json.NewDecoder(bytes.NewReader(f["storageProof"]))
	start, err := storageDecoder.Token()
	if err != nil || start != json.Delim('[') || storageDecoder.More() {
		return fail()
	}
	end, err := storageDecoder.Token()
	if err != nil || end != json.Delim(']') {
		return fail()
	}
	if _, err = storageDecoder.Token(); err != io.EOF {
		return fail()
	}
	text, err = rpcText(f["balance"])
	balance, parseErr := quantity(text)
	if err != nil || parseErr != nil {
		return fail()
	}
	nonce, err := rpcUint(f["nonce"], 64)
	if err != nil {
		return fail()
	}
	code, err := rpcHash(f["codeHash"])
	if err != nil {
		return fail()
	}
	storage, err := rpcHash(f["storageHash"])
	if err != nil {
		return fail()
	}
	d := json.NewDecoder(bytes.NewReader(f["accountProof"]))
	first, err := d.Token()
	if err != nil || first != json.Delim('[') {
		return fail()
	}
	proof.Header = bytes.Clone(header)
	total := len(header)
	for d.More() {
		if ctx.Err() != nil || len(proof.AccountNodes) == MaxCustodyAccountProofNodes {
			return fail()
		}
		var item json.RawMessage
		if d.Decode(&item) != nil {
			return fail()
		}
		node, err := rpcData(item, min(maxCustodyAccountNodeBytes, MaxCustodyAccountProofBytes-total))
		if err != nil || len(node) == 0 {
			return fail()
		}
		total += len(node)
		proof.AccountNodes = append(proof.AccountNodes, node)
	}
	last, err := d.Token()
	if err != nil || last != json.Delim(']') || len(proof.AccountNodes) == 0 {
		return fail()
	}
	if _, err = d.Token(); err != io.EOF || ctx.Err() != nil {
		return fail()
	}
	account, err := custodyAccountValue(ctx, block, proof, sender)
	if err != nil || account.Nonce != nonce || account.Balance.ToBig().Cmp(balance) != 0 || !bytes.Equal(account.CodeHash, code[:]) || [32]byte(account.Root) != storage || ctx.Err() != nil {
		return fail()
	}
	return proof, nil
}

var _ CustodyAccountSource = (*HTTPCustodyAccountSource)(nil)
