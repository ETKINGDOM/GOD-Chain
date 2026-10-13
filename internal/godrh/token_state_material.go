package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	MaxTokenStateMaterialBytes = 2*MaxContractStateProofBytes + (32 << 10)
	tokenStatePurpose          = "GOD Chain private conditional token state v1"
)

var ErrTokenStateMaterial = errors.New("offline token state material rejected; no independent RH finality, permissions or asset approval")

// Deliberately unusable until the operator retains real proof bytes and
// separately reviews their block pin and slot layout. No slot is inferred.
func TokenStateMaterialTemplate() []byte {
	raw, _ := json.MarshalIndent(map[string]any{
		"purpose": tokenStatePurpose, "version": 1, "header": "", "runtime": "",
		"accountNodes": []string{}, "slots": []any{},
	}, "", "  ")
	return append(raw, '\n')
}

// The expected block is a separate operator input, not a claim inside the
// proof file and not a provider's finalized tag. No trust is derived here.
func TokenStateBlock(height, digest string) (Block, error) {
	n, err := ParentMaterialSlot(height)
	if err != nil || n == 0 || len(digest) != 66 || !strings.HasPrefix(digest, "0x") {
		return Block{}, ErrTokenStateMaterial
	}
	h, err := SourceMaterialPin(digest[2:])
	if err != nil {
		return Block{}, ErrTokenStateMaterial
	}
	return Block{Height: n, Hash: h}, nil
}

type TokenStateMaterial struct {
	proof    ContractStateProof
	slots    []ContractStorageBinding
	accepted bool
}

func (TokenStateMaterial) String() string               { return "conditional token state material (redacted)" }
func (m TokenStateMaterial) GoString() string           { return m.String() }
func (TokenStateMaterial) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

func tokenStateData(raw []byte, maximum int) ([]byte, error) {
	text, err := custodyText(raw)
	if err != nil || len(text) < 4 || len(text) > 2+2*maximum || len(text)%2 != 0 ||
		!strings.HasPrefix(text, "0x") || strings.ToLower(text) != text {
		return nil, ErrTokenStateMaterial
	}
	value, err := hex.DecodeString(text[2:])
	if err != nil {
		return nil, ErrTokenStateMaterial
	}
	return value, nil
}

// Bounded streaming array decoding avoids allocating the declared array
// length. Empty arrays are allowed only where the proof semantics permit.
func tokenStateArray(raw []byte, maximum int, visit func(json.RawMessage) error) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	first, err := d.Token()
	if err != nil || first != json.Delim('[') {
		return ErrTokenStateMaterial
	}
	for count := 0; d.More(); count++ {
		if count == maximum {
			return ErrTokenStateMaterial
		}
		var item json.RawMessage
		if d.Decode(&item) != nil || visit(item) != nil {
			return ErrTokenStateMaterial
		}
	}
	last, err := d.Token()
	if err != nil || last != json.Delim(']') {
		return ErrTokenStateMaterial
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrTokenStateMaterial
	}
	return nil
}

// Exact separately pinned JSON, with bounded canonical lowercase hex data.
// The JSON seal is integrity, not authentication. Actual account/code/slot
// verification still runs against the separate block in the workflow below.
func ParseTokenStateMaterial(raw []byte, expected [32]byte) (TokenStateMaterial, error) {
	fail := func() (TokenStateMaterial, error) { return TokenStateMaterial{}, ErrTokenStateMaterial }
	if expected == [32]byte{} || len(raw) == 0 || len(raw) > MaxTokenStateMaterialBytes ||
		sha256.Sum256(raw) != expected || !utf8.Valid(raw) || !uniqueTokenSourceJSON(raw) {
		return fail()
	}
	f, err := custodyObject(raw, []string{"purpose", "version", "header", "runtime", "accountNodes", "slots"}, nil)
	if err != nil || !parentMaterialPurpose(f, tokenStatePurpose) {
		return fail()
	}
	p := ContractStateProof{}
	if p.Header, err = tokenStateData(f["header"], maxSetHeaderBytes); err != nil {
		return fail()
	}
	if p.Runtime, err = tokenStateData(f["runtime"], maxCodeBytes); err != nil {
		return fail()
	}
	total, accountBytes := len(p.Header)+len(p.Runtime), len(p.Header)
	err = tokenStateArray(f["accountNodes"], MaxCustodyAccountProofNodes, func(item json.RawMessage) error {
		node, err := tokenStateData(item, min(maxCustodyAccountNodeBytes, MaxCustodyAccountProofBytes-accountBytes))
		if err != nil || len(node) > MaxContractStateProofBytes-total {
			return ErrTokenStateMaterial
		}
		total, accountBytes = total+len(node), accountBytes+len(node)
		p.AccountNodes = append(p.AccountNodes, node)
		return nil
	})
	if err != nil || len(p.AccountNodes) == 0 {
		return fail()
	}
	slots := []ContractStorageBinding{}
	seen := map[[32]byte]bool{}
	err = tokenStateArray(f["slots"], MaxContractStateSlots, func(item json.RawMessage) error {
		fields, err := custodyObject(item, []string{"key", "value", "nodes"}, nil)
		if err != nil {
			return ErrTokenStateMaterial
		}
		s := ContractStorageBinding{}
		for _, field := range []struct {
			name string
			out  []byte
		}{{"key", s.Key[:]}, {"value", s.Value[:]}} {
			value, err := custodyHex(fields[field.name], 32)
			if err != nil {
				return ErrTokenStateMaterial
			}
			copy(field.out, value)
		}
		if seen[s.Key] {
			return ErrTokenStateMaterial
		}
		seen[s.Key] = true
		nodes := [][]byte{}
		err = tokenStateArray(fields["nodes"], MaxCustodyAccountProofNodes, func(item json.RawMessage) error {
			node, err := tokenStateData(item, min(maxCustodyAccountNodeBytes, MaxContractStateProofBytes-total))
			if err != nil {
				return ErrTokenStateMaterial
			}
			total += len(node)
			nodes = append(nodes, node)
			return nil
		})
		if err != nil {
			return ErrTokenStateMaterial
		}
		slots = append(slots, s)
		p.StorageNodes = append(p.StorageNodes, nodes)
		return nil
	})
	if err != nil || total > MaxContractStateProofBytes {
		return fail()
	}
	return TokenStateMaterial{proof: p, slots: slots, accepted: true}, nil
}

type TokenStateFiles struct {
	ConfigPath, BundlePath, ProofPath                string
	ConfigPin, BundlePin, ProofPin, ExpectedCompiler [32]byte
}

func (TokenStateFiles) String() string               { return "private token state files (redacted)" }
func (f TokenStateFiles) GoString() string           { return f.String() }
func (TokenStateFiles) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type TokenStateWorkflowReport struct {
	PrivateMaterialsPinned bool             `json:"privateMaterialsPinned"`
	ConditionalState       TokenStateReport `json:"conditionalState"`
	PrivateFilesUnchanged  bool             `json:"privateFilesUnchanged"`
	SigningEnabled         bool             `json:"signingEnabled"`
	BroadcastEnabled       bool             `json:"broadcastEnabled"`
	RealAssetsReady        bool             `json:"realAssetsReady"`
}

// No compiler, provider, signer, journal or bridge keeper is used. Read-only
// owner-only POSIX snapshots are rechecked before a redacted success report.
func ReviewPrivateTokenState(ctx context.Context, files TokenStateFiles, block Block) (report TokenStateWorkflowReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = TokenStateWorkflowReport{}, ErrTokenStateMaterial
		}
	}()
	if ctx == nil || ctx.Err() != nil || files.ExpectedCompiler == [32]byte{} || !validDiscoveryBlock(block) {
		return TokenStateWorkflowReport{}, ErrTokenStateMaterial
	}
	paths := []string{files.ConfigPath, files.BundlePath, files.ProofPath}
	pins := [][32]byte{files.ConfigPin, files.BundlePin, files.ProofPin}
	limits := []int64{maxConfigBytes, MaxTokenSourceBundleBytes, MaxTokenStateMaterialBytes}
	raw := make([][]byte, len(paths))
	for i := range paths {
		raw[i], err = readRHPrivateBytes(paths[i], limits[i], pins[i])
		if err != nil || ctx.Err() != nil {
			return TokenStateWorkflowReport{}, ErrTokenStateMaterial
		}
	}
	c, err := Parse(raw[0])
	if err != nil {
		return TokenStateWorkflowReport{}, ErrTokenStateMaterial
	}
	b, err := ParseTokenSourceBundle(raw[1], files.BundlePin)
	if err != nil {
		return TokenStateWorkflowReport{}, ErrTokenStateMaterial
	}
	m, err := ParseTokenStateMaterial(raw[2], files.ProofPin)
	if err != nil || !m.accepted {
		return TokenStateWorkflowReport{}, ErrTokenStateMaterial
	}
	r, err := VerifyTokenStateProof(ctx, c, b, files.ExpectedCompiler, block, m.slots, m.proof)
	if err != nil {
		return TokenStateWorkflowReport{}, ErrTokenStateMaterial
	}
	for i := range paths {
		again, err := readRHPrivateBytes(paths[i], limits[i], pins[i])
		if err != nil || !bytes.Equal(raw[i], again) || ctx.Err() != nil {
			return TokenStateWorkflowReport{}, ErrTokenStateMaterial
		}
	}
	return TokenStateWorkflowReport{PrivateMaterialsPinned: true, ConditionalState: r, PrivateFilesUnchanged: true}, nil
}
