package godrh

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/big"

	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

type Block struct {
	Height uint64
	Hash   [32]byte
}

// Source is replaceable and read-only. A provider's finalized tag is a claim,
// not independent verification of L1 settlement or an approval to release GOD.
type Source interface {
	ChainID(context.Context) (*big.Int, error)
	FinalizedBlock(context.Context) (Block, error)
	Block(context.Context, uint64) (Block, error)
	Code(context.Context, [20]byte, [32]byte) ([]byte, error)
	Call(context.Context, [20]byte, []byte, [32]byte) ([]byte, error)
}

// Probe pins all contract reads to one explicit provider-reported block hash.
// It never signs, sends transactions, reads node stores or enables real assets.
func Probe(ctx context.Context, c Config, source Source) Report {
	r := c.Report()
	if !r.ConfigurationReady || source == nil || ctx == nil || ctx.Err() != nil {
		return r
	}
	r.SourceChecked = true
	add := func(name string, passed bool) bool {
		r.Checks = append(r.Checks, Check{name, passed})
		return passed
	}
	expectedChain, _ := chainID(c.private.SourceChainID)
	actualChain, err := source.ChainID(ctx)
	if !add("source_chain_id", err == nil && actualChain != nil && actualChain.Cmp(expectedChain) == 0) {
		return r
	}
	block, err := source.FinalizedBlock(ctx)
	if !add("provider_finalized_checkpoint", err == nil && block.Height > 0 && block.Hash != [32]byte{}) {
		return r
	}
	token, _ := address(c.private.TokenContract)
	custody, _ := address(c.private.CustodyContract)
	for _, contract := range []struct {
		name    string
		address [20]byte
		pin     string
	}{{"token_runtime_code", token, c.private.ExpectedTokenCodeHash}, {"custody_runtime_code", custody, c.private.ExpectedCustodyCodeHash}} {
		code, err := source.Code(ctx, contract.address, block.Hash)
		expected, _ := hash(contract.pin)
		if !add(contract.name, err == nil && len(code) > 0 && len(code) <= maxCodeBytes && ethcrypto.Keccak256Hash(code) == expected) {
			return r
		}
	}
	call := func(target [20]byte, signature string) ([]byte, bool) {
		output, err := source.Call(ctx, target, selector(signature), block.Hash)
		return output, err == nil
	}
	for _, expected := range []struct {
		name, signature string
		target          [20]byte
		value           *big.Int
	}{
		{"token_decimals", "decimals()", token, big.NewInt(godrewards.Decimals)},
		{"fixed_token_supply", "totalSupply()", token, godrewards.FixedGodSupply().BigInt()},
		{"custody_fixed_supply", "FIXED_SUPPLY()", custody, godrewards.FixedGodSupply().BigInt()},
		{"custody_exposure_limit", "EXPOSURE_LIMIT()", custody, godbridge.ExposureLimit().BigInt()},
		{"custody_transfer_limit", "TRANSFER_LIMIT()", custody, godbridge.TransferLimit().BigInt()},
		{"custody_outflow_limit", "OUTFLOW_LIMIT()", custody, godbridge.OutflowLimit().BigInt()},
		{"custody_withdrawal_delay", "DELAY()", custody, big.NewInt(int64(godbridge.WithdrawalDelay.Seconds()))},
		{"custody_queue_bound", "MAX_SPAN()", custody, big.NewInt(godbridge.MaxOpenWithdrawals)},
	} {
		output, ok := call(expected.target, expected.signature)
		if !add(expected.name, ok && len(output) == 32 && new(big.Int).SetBytes(output).Cmp(expected.value) == 0) {
			return r
		}
	}
	output, ok := call(custody, "token()")
	if !add("custody_token", ok && len(output) == 32 && bytes.Equal(output[:12], make([]byte, 12)) && bytes.Equal(output[12:], token[:])) {
		return r
	}
	binding, _ := c.Binding()
	output, ok = call(custody, "assetID()")
	if !add("custody_asset_binding", ok && bytes.Equal(output, binding.AssetID[:])) {
		return r
	}
	for _, expected := range []struct{ signature, value string }{{"sourceChain()", binding.SourceChain}, {"nativeChain()", binding.NativeChain}} {
		output, ok := call(custody, expected.signature)
		if !add("custody_"+expected.signature[:len(expected.signature)-2], ok && abiText(output, expected.value)) {
			return r
		}
	}
	for i, signer := range binding.Signers {
		input := append(selector("signers(uint256)"), make([]byte, 32)...)
		binary.BigEndian.PutUint64(input[len(input)-8:], uint64(i))
		output, err := source.Call(ctx, custody, input, block.Hash)
		if !add("custody_signer", err == nil && len(output) == 32 && bytes.Equal(output[:12], make([]byte, 12)) && bytes.Equal(output[12:], signer[:])) {
			return r
		}
	}
	// Recheck network and the checkpoint. Every contract read above used the
	// first hash with requireCanonical; a changed or inconsistent view rejects.
	chainAgain, chainErr := source.ChainID(ctx)
	blockAgain, blockErr := source.Block(ctx, block.Height)
	if !add("consistent_provider_view", ctx.Err() == nil && chainErr == nil && blockErr == nil && chainAgain != nil && chainAgain.Cmp(expectedChain) == 0 && blockAgain == block) {
		return r
	}
	r.ReadOnlyProbePassed = true
	return r
}

func selector(signature string) []byte { return ethcrypto.Keccak256([]byte(signature))[:4] }

func abiText(raw []byte, expected string) bool {
	if len(expected) == 0 || len(expected) > 50 {
		return false
	}
	padded := (len(expected) + 31) / 32 * 32
	if len(raw) != 64+padded || new(big.Int).SetBytes(raw[:32]).Cmp(big.NewInt(32)) != 0 ||
		new(big.Int).SetBytes(raw[32:64]).Cmp(big.NewInt(int64(len(expected)))) != 0 {
		return false
	}
	return bytes.Equal(raw[64:64+len(expected)], []byte(expected)) && bytes.Equal(raw[64+len(expected):], make([]byte, padded-len(expected)))
}
