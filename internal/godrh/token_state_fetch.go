package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math/big"
	"unicode/utf8"
)

const MaxTokenStateReadRequestBytes = 16 << 10
const tokenStateReadPurpose = "GOD Chain private conditional token state read v1"

var ErrTokenStateRead = errors.New("conditional read-only token state rejected; no independent RH finality, permissions or asset approval")

func TokenStateReadTemplate() []byte {
	raw, _ := json.MarshalIndent(map[string]any{"purpose": tokenStateReadPurpose, "version": 1, "header": "", "slots": []any{}}, "", "  ")
	return append(raw, '\n')
}

// Explicit retained header and source-reviewed slot expectations. Empty slots
// are code/account-only review, never proof of immutable supply or permissions.
type TokenStateReadRequest struct {
	header   []byte
	slots    []ContractStorageBinding
	accepted bool
}

func (TokenStateReadRequest) String() string {
	return "private conditional token state read request (redacted)"
}
func (r TokenStateReadRequest) GoString() string           { return r.String() }
func (TokenStateReadRequest) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

func ParseTokenStateReadRequest(raw []byte, pin [32]byte) (TokenStateReadRequest, error) {
	fail := func() (TokenStateReadRequest, error) { return TokenStateReadRequest{}, ErrTokenStateRead }
	if len(raw) == 0 || len(raw) > MaxTokenStateReadRequestBytes || pin == [32]byte{} || sha256.Sum256(raw) != pin ||
		!utf8.Valid(raw) || !uniqueTokenSourceJSON(raw) {
		return fail()
	}
	f, err := custodyObject(raw, []string{"purpose", "version", "header", "slots"}, nil)
	if err != nil || !parentMaterialPurpose(f, tokenStateReadPurpose) {
		return fail()
	}
	header, err := tokenStateData(f["header"], maxSetHeaderBytes)
	if err != nil {
		return fail()
	}
	slots := []ContractStorageBinding{}
	seen := map[[32]byte]bool{}
	err = tokenStateArray(f["slots"], MaxContractStateSlots, func(raw json.RawMessage) error {
		f, err := custodyObject(raw, []string{"key", "value"}, nil)
		if err != nil {
			return ErrTokenStateRead
		}
		slot := ContractStorageBinding{}
		for _, field := range []struct {
			name string
			out  []byte
		}{{"key", slot.Key[:]}, {"value", slot.Value[:]}} {
			word, err := custodyHex(f[field.name], 32)
			if err != nil {
				return ErrTokenStateRead
			}
			copy(field.out, word)
		}
		if seen[slot.Key] {
			return ErrTokenStateRead
		}
		seen[slot.Key] = true
		slots = append(slots, slot)
		return nil
	})
	if err != nil {
		return fail()
	}
	return TokenStateReadRequest{header: header, slots: slots, accepted: true}, nil
}

// Cancellation-honoring read-only injection, not a signer, head selector or
// finalized-tag authority. Implementations must not mutate returned material.
type TokenStateSource interface {
	ChainID(context.Context) (*big.Int, error)
	Block(context.Context, uint64) (Block, error)
	ContractStateProof(context.Context, Block, ContractStateBinding, []byte) (ContractStateProof, error)
}

type TokenStateReadReport struct {
	TokenStateReport
	SourceMaterialRead           bool `json:"sourceMaterialRead"`
	ProviderChainAndBlockMatched bool `json:"providerChainAndBlockMatched"`
}

type FetchedTokenState struct {
	block   Block
	binding ContractStateBinding
	proof   ContractStateProof
	report  TokenStateReadReport
	ready   bool
}

func (FetchedTokenState) String() string                 { return "conditional fetched token state (redacted)" }
func (s FetchedTokenState) GoString() string             { return s.String() }
func (s FetchedTokenState) MarshalJSON() ([]byte, error) { return json.Marshal(s.Report()) }
func (s FetchedTokenState) Report() TokenStateReadReport {
	if !s.ready {
		return TokenStateReadReport{}
	}
	return s.report
}
func (s FetchedTokenState) Material() (Block, ContractStateBinding, ContractStateProof, error) {
	if !s.ready {
		return Block{}, ContractStateBinding{}, ContractStateProof{}, ErrTokenStateRead
	}
	b, p, err := detachContractState(s.binding, s.proof)
	if err != nil {
		return Block{}, ContractStateBinding{}, ContractStateProof{}, ErrTokenStateRead
	}
	return s.block, b, p, nil
}

// One bounded conditional read at the caller's retained block. No latest,
// pending, height-only proof fallback, finalized claim, retry or persistence.
// Cooperative injected sources must honor the deadline; HTTP additionally
// enforces per-request timeouts. Never yields release/signing authority.
func FetchTokenStateProof(ctx context.Context, c Config, bundle TokenSourceBundle, compiler [32]byte, block Block, request TokenStateReadRequest, source TokenStateSource) (fetched FetchedTokenState, err error) {
	fail := func() (FetchedTokenState, error) { return FetchedTokenState{}, ErrTokenStateRead }
	defer func() {
		if recover() != nil {
			fetched, err = fail()
		}
	}()
	if !request.accepted || len(request.header) == 0 || len(request.header) > maxSetHeaderBytes || len(request.slots) > MaxContractStateSlots {
		return fail()
	}
	request.header = bytes.Clone(request.header)
	request.slots = append([]ContractStorageBinding(nil), request.slots...)
	if ctx == nil || ctx.Err() != nil || source == nil || !tokenStateHeaderMatches(request.header, block) ||
		!CheckTokenSourceMaterials(c, bundle, compiler).OfflineMaterialsMatched {
		return fail()
	}
	token, err := address(c.private.TokenContract)
	if err != nil {
		return fail()
	}
	runtime, err := hash(c.private.ExpectedTokenCodeHash)
	if err != nil {
		return fail()
	}
	chain, err := chainID(c.private.SourceChainID)
	if err != nil {
		return fail()
	}
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	chainMatches := func() bool {
		n, err := source.ChainID(readCtx)
		return err == nil && readCtx.Err() == nil && n != nil && n.Cmp(chain) == 0
	}
	blockMatches := func() bool {
		b, err := source.Block(readCtx, block.Height)
		return err == nil && readCtx.Err() == nil && b == block
	}
	if !chainMatches() || !blockMatches() {
		return fail()
	}
	binding := ContractStateBinding{Address: token, RuntimeHash: runtime, Slots: request.slots}
	// Give the source its own slots/header, then detach the result before
	// invoking ANY source/context callback again.
	input := binding
	input.Slots = append([]ContractStorageBinding(nil), binding.Slots...)
	raw, err := source.ContractStateProof(readCtx, block, input, bytes.Clone(request.header))
	if err != nil {
		return fail()
	}
	binding, proof, err := detachContractState(binding, raw)
	if err != nil || !bytes.Equal(proof.Header, request.header) || readCtx.Err() != nil {
		return fail()
	}
	r, err := VerifyTokenStateProof(readCtx, c, bundle, compiler, block, binding.Slots, proof)
	if err != nil || !blockMatches() || !chainMatches() {
		return fail()
	}
	return FetchedTokenState{block: block, binding: binding, proof: proof,
		report: TokenStateReadReport{TokenStateReport: r, SourceMaterialRead: true, ProviderChainAndBlockMatched: true}, ready: true}, nil
}

type TokenStateReadFiles struct {
	ConfigPath, BundlePath, RequestPath                string
	ConfigPin, BundlePin, RequestPin, ExpectedCompiler [32]byte
}

func (TokenStateReadFiles) String() string               { return "private token state read files (redacted)" }
func (f TokenStateReadFiles) GoString() string           { return f.String() }
func (TokenStateReadFiles) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type TokenStateReadWorkflowReport struct {
	PrivateMaterialsPinned bool                 `json:"privateMaterialsPinned"`
	ConditionalRead        TokenStateReadReport `json:"conditionalRead"`
	PrivateFilesUnchanged  bool                 `json:"privateFilesUnchanged"`
	SigningEnabled         bool                 `json:"signingEnabled"`
	BroadcastEnabled       bool                 `json:"broadcastEnabled"`
	RealAssetsReady        bool                 `json:"realAssetsReady"`
}

// Explicit network-read workflow. Files are owner-only pinned snapshots and
// rechecked after reading. Configured production HTTPS is allowed for diagnosis
// only; this does not configure a bridge, fund custody or authorize activation.
func ReviewPrivateTokenStateRead(ctx context.Context, f TokenStateReadFiles, block Block) (report TokenStateReadWorkflowReport, err error) {
	fail := func() (TokenStateReadWorkflowReport, error) { return TokenStateReadWorkflowReport{}, ErrTokenStateRead }
	defer func() {
		if recover() != nil {
			report, err = fail()
		}
	}()
	if ctx == nil || ctx.Err() != nil || !validDiscoveryBlock(block) {
		return fail()
	}
	paths := []string{f.ConfigPath, f.BundlePath, f.RequestPath}
	pins := [][32]byte{f.ConfigPin, f.BundlePin, f.RequestPin}
	limits := []int64{maxConfigBytes, MaxTokenSourceBundleBytes, MaxTokenStateReadRequestBytes}
	raw := make([][]byte, len(paths))
	for i := range paths {
		raw[i], err = readRHPrivateBytes(paths[i], limits[i], pins[i])
		if err != nil || ctx.Err() != nil {
			return fail()
		}
	}
	c, err := Parse(raw[0])
	if err != nil {
		return fail()
	}
	b, err := ParseTokenSourceBundle(raw[1], f.BundlePin)
	if err != nil {
		return fail()
	}
	r, err := ParseTokenStateReadRequest(raw[2], f.RequestPin)
	if err != nil || !tokenStateHeaderMatches(r.header, block) || !CheckTokenSourceMaterials(c, b, f.ExpectedCompiler).OfflineMaterialsMatched {
		return fail()
	}
	s, err := NewHTTPTokenStateSource(c)
	if err != nil {
		return fail()
	}
	defer s.Close()
	fetched, err := FetchTokenStateProof(ctx, c, b, f.ExpectedCompiler, block, r, s)
	if err != nil {
		return fail()
	}
	for i := range paths {
		again, err := readRHPrivateBytes(paths[i], limits[i], pins[i])
		if err != nil || !bytes.Equal(raw[i], again) || ctx.Err() != nil {
			return fail()
		}
	}
	return TokenStateReadWorkflowReport{PrivateMaterialsPinned: true, ConditionalRead: fetched.Report(), PrivateFilesUnchanged: true}, nil
}
