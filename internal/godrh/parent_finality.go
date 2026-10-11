package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/bits"

	bls "github.com/consensys/gnark-crypto/ecc/bls12-381"
	"github.com/ethereum/go-ethereum/beacon/merkle"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	parentPeriodSlots   = 8192
	parentCommitteeSize = 512
	parentSupermajority = 342
	// execution_payload is body field 9 (gindex 25); block_hash is payload
	// field 12, in a 32-leaf Deneb/Electra container: (25 << 5) | 12.
	parentExecutionHashIndex = 812
	parentBLSDomain          = "BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_POP_"
)

var ErrParentFinalityProof = errors.New("offline parent finality proof rejected; no RH finality or asset approval")

// ParentBeaconHeader is the five-field BeaconBlockHeader, not an execution
// header or RPC finalized tag. Operational roots never enter public defaults.
type ParentBeaconHeader struct {
	Slot, ProposerIndex             uint64
	ParentRoot, StateRoot, BodyRoot [32]byte
}

func (ParentBeaconHeader) String() string               { return "parent beacon header (redacted)" }
func (h ParentBeaconHeader) GoString() string           { return h.String() }
func (ParentBeaconHeader) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

// ParentFinalityTrust MUST come from separately retained, reviewed material,
// never from the same provider/proof. This API cannot authenticate its origin,
// clock, fork schedule or weak-subjectivity freshness. Start/End are an explicit
// inclusive/exclusive fork interval; no inferred, future or fallback fork.
type ParentFinalityTrust struct {
	CheckpointRoot, GenesisValidatorsRoot              [32]byte
	CheckpointSlot, CurrentSlot, MaxCheckpointAgeSlots uint64
	ForkName                                           string
	ForkVersion                                        [4]byte
	ForkStartSlot, ForkEndSlot                         uint64
}

func (ParentFinalityTrust) String() string               { return "parent finality trust input (redacted)" }
func (p ParentFinalityTrust) GoString() string           { return p.String() }
func (ParentFinalityTrust) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

// ParentFinalityProof has only fixed-width committee/signature/header fields
// and exactly sized Merkle branches plus one bounded canonical RLP header. No
// transport/parser/store/signer is provided. Repeated committee keys are valid
// protocol positions and are counted with multiplicity, not deduplicated.
type ParentFinalityProof struct {
	Bootstrap, Attested, Finalized                       ParentBeaconHeader
	Committee                                            [parentCommitteeSize][48]byte
	AggregatePubkey                                      [48]byte
	CommitteeBranch, FinalityBranch, ExecutionHashBranch [][32]byte
	Participation                                        [64]byte
	Signature                                            [96]byte
	SignatureSlot                                        uint64
	FinalizedExecutionHeader                             []byte
}

func (ParentFinalityProof) String() string               { return "parent finality proof (redacted)" }
func (p ParentFinalityProof) GoString() string           { return p.String() }
func (ParentFinalityProof) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type ParentFinalityReport struct {
	CheckpointMatched                 bool `json:"checkpointMatched"`
	CommitteeIncluded                 bool `json:"committeeIncluded"`
	CommitteeAggregateMatched         bool `json:"committeeAggregateMatched"`
	SupermajoritySignatureVerified    bool `json:"supermajoritySignatureVerified"`
	FinalizedHeaderIncluded           bool `json:"finalizedHeaderIncluded"`
	CanonicalExecutionHashIncluded    bool `json:"canonicalExecutionHashIncluded"`
	ConditionalParentFinalityVerified bool `json:"conditionalParentFinalityVerified"`
	AnchorIndependentlyAuthenticated  bool `json:"anchorIndependentlyAuthenticated"`
	ForkScheduleIndependentlyVerified bool `json:"forkScheduleIndependentlyVerified"`
	ClockIndependentlyVerified        bool `json:"clockIndependentlyVerified"`
	SourceChainIdentityVerified       bool `json:"sourceChainIdentityVerified"`
	RHBatchBindingVerified            bool `json:"rhBatchBindingVerified"`
	RHSourceFinalityVerified          bool `json:"rhSourceFinalityVerified"`
	ExecutionReplayed                 bool `json:"executionReplayed"`
	SigningEnabled                    bool `json:"signingEnabled"`
	BroadcastEnabled                  bool `json:"broadcastEnabled"`
	RealAssetsReady                   bool `json:"realAssetsReady"`
}

func parentPair(a, b [32]byte) [32]byte {
	var raw [64]byte
	copy(raw[:32], a[:])
	copy(raw[32:], b[:])
	return sha256.Sum256(raw[:])
}

func parentHeaderRoot(h ParentBeaconHeader) [32]byte {
	var leaves [8][32]byte
	binary.LittleEndian.PutUint64(leaves[0][:8], h.Slot)
	binary.LittleEndian.PutUint64(leaves[1][:8], h.ProposerIndex)
	leaves[2], leaves[3], leaves[4] = h.ParentRoot, h.StateRoot, h.BodyRoot
	for n := 8; n > 1; n /= 2 {
		for i := 0; i < n/2; i++ {
			leaves[i] = parentPair(leaves[2*i], leaves[2*i+1])
		}
	}
	return leaves[0]
}

func parentKeyRoot(k [48]byte) [32]byte {
	var a, b [32]byte
	copy(a[:], k[:32])
	copy(b[:16], k[32:])
	return parentPair(a, b)
}

func parentCommitteeRoot(p *ParentFinalityProof) [32]byte {
	var leaves [parentCommitteeSize][32]byte
	for i, k := range p.Committee {
		leaves[i] = parentKeyRoot(k)
	}
	for n := len(leaves); n > 1; n /= 2 {
		for i := 0; i < n/2; i++ {
			leaves[i] = parentPair(leaves[2*i], leaves[2*i+1])
		}
	}
	return parentPair(leaves[0], parentKeyRoot(p.AggregatePubkey))
}

func parentSigningRoot(t ParentFinalityTrust, h ParentBeaconHeader) [32]byte {
	var version, domain [32]byte
	copy(version[:4], t.ForkVersion[:])
	forkData := parentPair(version, t.GenesisValidatorsRoot)
	domain[0] = 7 // DOMAIN_SYNC_COMMITTEE, not a transaction signature domain.
	copy(domain[4:], forkData[:28])
	return parentPair(parentHeaderRoot(h), domain)
}

func parentBranch(root [32]byte, index uint64, branch [][32]byte, leaf [32]byte) bool {
	if len(branch) != bits.Len64(index)-1 {
		return false
	}
	values := make(merkle.Values, len(branch))
	for i, item := range branch {
		values[i] = merkle.Value(item)
	}
	return merkle.VerifyProof(common.Hash(root), index, values, merkle.Value(leaf)) == nil
}

func parentG1(k [48]byte) (bls.G1Affine, bool) {
	var p bls.G1Affine
	n, err := p.SetBytes(k[:]) // checks curve AND prime-order subgroup
	return p, err == nil && n == len(k) && !p.IsInfinity() && p.Bytes() == k
}

func parentSignature(ctx context.Context, t ParentFinalityTrust, p *ParentFinalityProof) bool {
	var all, selected bls.G1Affine
	for i, raw := range p.Committee {
		if ctx.Err() != nil {
			return false
		}
		key, valid := parentG1(raw)
		if !valid {
			return false
		}
		all.Add(&all, &key)
		if p.Participation[i/8]&(1<<(i%8)) != 0 {
			selected.Add(&selected, &key)
		}
	}
	aggregate, valid := parentG1(p.AggregatePubkey)
	if !valid || all.IsInfinity() || selected.IsInfinity() || !all.Equal(&aggregate) {
		return false
	}
	var signature bls.G2Affine
	n, err := signature.SetBytes(p.Signature[:]) // subgroup check before pairing
	if err != nil || n != len(p.Signature) || signature.IsInfinity() || signature.Bytes() != p.Signature || ctx.Err() != nil {
		return false
	}
	root := parentSigningRoot(t, p.Attested)
	message, err := bls.HashToG2(root[:], []byte(parentBLSDomain))
	if err != nil || message.IsInfinity() {
		return false
	}
	_, _, generator, _ := bls.Generators()
	generator.Neg(&generator)
	ok, err := bls.PairingCheck([]bls.G1Affine{selected, generator}, []bls.G2Affine{message, signature})
	return err == nil && ok && ctx.Err() == nil
}

// VerifyParentFinalityProof is a read-only, bounded, single-period/single-fork
// light-client check under EXPLICIT caller trust. It never obtains an anchor or
// clock from a provider. Deneb/Electra only; transitions and all other forks are
// rejected. A proved Ethereum parent block is NOT a proved RH block: RH batch
// origin, data availability, deterministic execution or confirmed assertions,
// token/custody proofs and one-for-one backing remain separate requirements.
// No activation flag, financial authorization, file or raw material is returned.
// Callers must not concurrently mutate slices during entry/copy.
func VerifyParentFinalityProof(ctx context.Context, t ParentFinalityTrust, p ParentFinalityProof) (report ParentFinalityReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = ParentFinalityReport{}, ErrParentFinalityProof
		}
	}()
	fail := func() (ParentFinalityReport, error) { return ParentFinalityReport{}, ErrParentFinalityProof }
	committeeIndex, finalityIndex := uint64(54), uint64(105)
	switch t.ForkName {
	case "deneb":
	case "electra":
		committeeIndex, finalityIndex = 86, 169
	default:
		return fail()
	}
	if ctx == nil || t.CheckpointRoot == [32]byte{} || t.GenesisValidatorsRoot == [32]byte{} || t.ForkVersion == [4]byte{} ||
		t.CheckpointSlot == 0 || p.Bootstrap.Slot != t.CheckpointSlot || t.ForkStartSlot%32 != 0 || t.ForkEndSlot%32 != 0 || t.ForkEndSlot <= t.ForkStartSlot ||
		t.MaxCheckpointAgeSlots == 0 || t.MaxCheckpointAgeSlots > parentPeriodSlots || t.CurrentSlot < t.CheckpointSlot || t.CurrentSlot-t.CheckpointSlot > t.MaxCheckpointAgeSlots ||
		p.Finalized.Slot <= t.CheckpointSlot || p.Attested.Slot < p.Finalized.Slot || p.SignatureSlot <= p.Attested.Slot || t.CurrentSlot < p.SignatureSlot ||
		len(p.CommitteeBranch) != bits.Len64(committeeIndex)-1 || len(p.FinalityBranch) != bits.Len64(finalityIndex)-1 || len(p.ExecutionHashBranch) != 9 ||
		len(p.FinalizedExecutionHeader) == 0 || len(p.FinalizedExecutionHeader) > maxSetHeaderBytes {
		return fail()
	}
	// Own every variable-length byte before ANY caller context callback. Fixed
	// arrays and trust policy already passed by value. No cached mutable roots.
	p.CommitteeBranch = append([][32]byte(nil), p.CommitteeBranch...)
	p.FinalityBranch = append([][32]byte(nil), p.FinalityBranch...)
	p.ExecutionHashBranch = append([][32]byte(nil), p.ExecutionHashBranch...)
	p.FinalizedExecutionHeader = bytes.Clone(p.FinalizedExecutionHeader)
	if ctx.Err() != nil {
		return fail()
	}
	for _, slot := range []uint64{t.CheckpointSlot, p.Finalized.Slot, p.Attested.Slot, p.SignatureSlot - 1, p.SignatureSlot, t.CurrentSlot} {
		if slot < t.ForkStartSlot || slot >= t.ForkEndSlot || slot/parentPeriodSlots != t.CheckpointSlot/parentPeriodSlots {
			return fail()
		}
	}
	for _, h := range []ParentBeaconHeader{p.Bootstrap, p.Attested, p.Finalized} {
		if h.ParentRoot == [32]byte{} || h.StateRoot == [32]byte{} || h.BodyRoot == [32]byte{} {
			return fail()
		}
	}
	var participants int
	for _, b := range p.Participation {
		participants += bits.OnesCount8(b)
	}
	if participants < parentSupermajority || parentHeaderRoot(p.Bootstrap) != t.CheckpointRoot ||
		!parentBranch(p.Bootstrap.StateRoot, committeeIndex, p.CommitteeBranch, parentCommitteeRoot(&p)) ||
		!parentBranch(p.Attested.StateRoot, finalityIndex, p.FinalityBranch, parentHeaderRoot(p.Finalized)) {
		return fail()
	}
	var header types.Header
	if rlp.DecodeBytes(p.FinalizedExecutionHeader, &header) != nil || header.Number == nil || !header.Number.IsUint64() ||
		header.Number.Sign() <= 0 || header.Root == (common.Hash{}) || header.GasUsed > header.GasLimit {
		return fail()
	}
	canonical, encodeErr := rlp.EncodeToBytes(&header)
	if encodeErr != nil || !bytes.Equal(canonical, p.FinalizedExecutionHeader) ||
		!parentBranch(p.Finalized.BodyRoot, parentExecutionHashIndex, p.ExecutionHashBranch, [32]byte(header.Hash())) ||
		!parentSignature(ctx, t, &p) || ctx.Err() != nil {
		return fail()
	}
	return ParentFinalityReport{CheckpointMatched: true, CommitteeIncluded: true, CommitteeAggregateMatched: true,
		SupermajoritySignatureVerified: true, FinalizedHeaderIncluded: true, CanonicalExecutionHashIncluded: true, ConditionalParentFinalityVerified: true}, nil
}
