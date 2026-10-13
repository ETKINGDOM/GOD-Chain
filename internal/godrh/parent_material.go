package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"unicode/utf8"
)

const (
	MaxParentTrustMaterialBytes = 4 << 10
	MaxParentProofMaterialBytes = 128 << 10
	parentTrustPurpose          = "god-chain-conditional-parent-trust"
	parentProofPurpose          = "god-chain-conditional-parent-proof"
)

var ErrParentMaterial = errors.New("offline parent material rejected; no RH finality or asset approval")

// Empty templates are NOT usable trust or proofs. They contain no populated
// checkpoint, genesis, fork version, clock, endpoint, committee or signature.
func ParentTrustTemplate() []byte {
	d := map[string]any{"purpose": parentTrustPurpose, "version": 1}
	for _, key := range parentTrustKeys[2:] {
		d[key] = ""
	}
	raw, _ := json.MarshalIndent(d, "", "  ")
	return append(raw, '\n')
}

func ParentProofTemplate(mode string) ([]byte, error) {
	if mode != "single-period" && mode != "adjacent-period" {
		return nil, ErrParentMaterial
	}
	header := map[string]string{"slot": "", "proposerIndex": "", "parentRoot": "", "stateRoot": "", "bodyRoot": ""}
	committee := map[string]any{"pubkeys": []string{}, "aggregatePubkey": ""}
	d := map[string]any{
		"purpose": parentProofPurpose, "version": 1, "mode": mode,
		"bootstrap": header, "attested": header, "finalized": header,
		"syncCommittee": committee, "committeeBranch": []string{},
		"finalityBranch": []string{}, "executionHashBranch": []string{},
		"participation": "", "signature": "", "signatureSlot": "", "finalizedExecutionHeader": "",
	}
	if mode == "adjacent-period" {
		d["nextSyncCommittee"], d["nextCommitteeBranch"] = committee, []string{}
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, ErrParentMaterial
	}
	return append(raw, '\n'), nil
}

var parentTrustKeys = []string{"purpose", "version", "checkpointRoot", "genesisValidatorsRoot", "checkpointSlot", "maxCheckpointAgeSlots", "forkName", "forkVersion", "forkStartSlot", "forkEndSlot"}
var parentProofKeys = []string{"purpose", "version", "mode", "bootstrap", "attested", "finalized", "syncCommittee", "committeeBranch", "finalityBranch", "executionHashBranch", "participation", "signature", "signatureSlot", "finalizedExecutionHeader"}

// Pins are independently reviewed SHA-256 values, not authentication of the
// source of a file or approval of its policy. Never infer them from a provider.
func ParentMaterialPin(text string) ([32]byte, error) {
	pin, err := SourceMaterialPin(text)
	if err != nil {
		return [32]byte{}, ErrParentMaterial
	}
	return pin, nil
}

// Decimal STRING wire values preserve uint64 precision, including values above
// the JavaScript safe-integer range. Leading zeroes, signs and exponents fail.
func ParentMaterialSlot(text string) (uint64, error) {
	if len(text) == 0 || len(text) > 20 || len(text) > 1 && text[0] == '0' {
		return 0, ErrParentMaterial
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return 0, ErrParentMaterial
		}
	}
	n, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, ErrParentMaterial
	}
	return n, nil
}

func parentMaterialUint(raw json.RawMessage) (uint64, error) {
	text, err := custodyText(raw)
	if err != nil {
		return 0, ErrParentMaterial
	}
	return ParentMaterialSlot(text)
}

func parentMaterialBytes(raw []byte, expected [32]byte, limit int) bool {
	return expected != [32]byte{} && len(raw) > 0 && len(raw) <= limit && utf8.Valid(raw) &&
		sha256.Sum256(raw) == expected && uniqueTokenSourceJSON(raw)
}

func parentMaterialPurpose(fields map[string]json.RawMessage, purpose string) bool {
	text, err := custodyText(fields["purpose"])
	return err == nil && text == purpose && bytes.Equal(bytes.TrimSpace(fields["version"]), []byte("1"))
}

// ParseParentTrustMaterial accepts no embedded clock or approval flag. Current
// slot is a separate explicit caller input. Provenance/weak subjectivity and
// real-network fork validity remain unverified even when the pin matches.
func ParseParentTrustMaterial(raw []byte, expected [32]byte, current uint64) (ParentFinalityTrust, error) {
	if !parentMaterialBytes(raw, expected, MaxParentTrustMaterialBytes) {
		return ParentFinalityTrust{}, ErrParentMaterial
	}
	f, err := custodyObject(raw, parentTrustKeys, nil)
	if err != nil || !parentMaterialPurpose(f, parentTrustPurpose) {
		return ParentFinalityTrust{}, ErrParentMaterial
	}
	t := ParentFinalityTrust{CurrentSlot: current}
	for _, item := range []struct {
		name string
		out  []byte
	}{{"checkpointRoot", t.CheckpointRoot[:]}, {"genesisValidatorsRoot", t.GenesisValidatorsRoot[:]}, {"forkVersion", t.ForkVersion[:]}} {
		value, err := custodyHex(f[item.name], len(item.out))
		if err != nil {
			return ParentFinalityTrust{}, ErrParentMaterial
		}
		copy(item.out, value)
	}
	for _, item := range []struct {
		name string
		out  *uint64
	}{{"checkpointSlot", &t.CheckpointSlot}, {"maxCheckpointAgeSlots", &t.MaxCheckpointAgeSlots}, {"forkStartSlot", &t.ForkStartSlot}, {"forkEndSlot", &t.ForkEndSlot}} {
		value, err := parentMaterialUint(f[item.name])
		if err != nil {
			return ParentFinalityTrust{}, ErrParentMaterial
		}
		*item.out = value
	}
	t.ForkName, err = custodyText(f["forkName"])
	if err != nil || !parentTrackingFresh(t, current) {
		return ParentFinalityTrust{}, ErrParentMaterial
	}
	return t, nil
}

func parentMaterialHeader(raw []byte) (ParentBeaconHeader, error) {
	f, err := custodyObject(raw, []string{"slot", "proposerIndex", "parentRoot", "stateRoot", "bodyRoot"}, nil)
	if err != nil {
		return ParentBeaconHeader{}, ErrParentMaterial
	}
	h := ParentBeaconHeader{}
	h.Slot, err = parentMaterialUint(f["slot"])
	if err != nil {
		return ParentBeaconHeader{}, ErrParentMaterial
	}
	h.ProposerIndex, err = parentMaterialUint(f["proposerIndex"])
	if err != nil {
		return ParentBeaconHeader{}, ErrParentMaterial
	}
	for _, item := range []struct {
		name string
		out  []byte
	}{{"parentRoot", h.ParentRoot[:]}, {"stateRoot", h.StateRoot[:]}, {"bodyRoot", h.BodyRoot[:]}} {
		value, err := custodyHex(f[item.name], len(item.out))
		if err != nil {
			return ParentBeaconHeader{}, ErrParentMaterial
		}
		copy(item.out, value)
	}
	return h, nil
}

// Decode at most the expected number of entries; do not allocate a slice of
// committees from an attacker-controlled array length. Null/scalar/extra fail.
func parentMaterialArray(raw []byte, count int, visit func(int, json.RawMessage) error) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	first, err := d.Token()
	if err != nil || first != json.Delim('[') {
		return ErrParentMaterial
	}
	for i := 0; i < count; i++ {
		if !d.More() {
			return ErrParentMaterial
		}
		var item json.RawMessage
		if d.Decode(&item) != nil || visit(i, item) != nil {
			return ErrParentMaterial
		}
	}
	if d.More() {
		return ErrParentMaterial
	}
	last, err := d.Token()
	if err != nil || last != json.Delim(']') {
		return ErrParentMaterial
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrParentMaterial
	}
	return nil
}

func parentMaterialCommittee(raw []byte, keys *[parentCommitteeSize][48]byte, aggregate *[48]byte) error {
	f, err := custodyObject(raw, []string{"pubkeys", "aggregatePubkey"}, nil)
	if err != nil {
		return ErrParentMaterial
	}
	err = parentMaterialArray(f["pubkeys"], parentCommitteeSize, func(i int, item json.RawMessage) error {
		key, err := custodyHex(item, 48)
		if err != nil {
			return ErrParentMaterial
		}
		copy(keys[i][:], key)
		return nil
	})
	if err != nil {
		return ErrParentMaterial
	}
	value, err := custodyHex(f["aggregatePubkey"], 48)
	if err != nil {
		return ErrParentMaterial
	}
	copy(aggregate[:], value)
	return nil
}

func parentMaterialBranch(raw []byte, count int) ([][32]byte, error) {
	out := make([][32]byte, count)
	err := parentMaterialArray(raw, count, func(i int, item json.RawMessage) error {
		value, err := custodyHex(item, 32)
		if err != nil {
			return ErrParentMaterial
		}
		copy(out[i][:], value)
		return nil
	})
	if err != nil {
		return nil, ErrParentMaterial
	}
	return out, nil
}

// ParentProofMaterial has no exported raw data, trust, pins or financial
// capability. It must pass the existing cryptographic verifier before review.
type ParentProofMaterial struct {
	fork     string
	proof    ParentFinalityProof
	next     *ParentNextCommitteeProof
	accepted bool
}

func (ParentProofMaterial) String() string               { return "parent proof material (redacted)" }
func (p ParentProofMaterial) GoString() string           { return p.String() }
func (ParentProofMaterial) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

func ParseParentProofMaterial(raw []byte, expected [32]byte, fork string) (ParentProofMaterial, error) {
	fail := func() (ParentProofMaterial, error) { return ParentProofMaterial{}, ErrParentMaterial }
	depth := 5
	if fork == "electra" {
		depth = 6
	} else if fork != "deneb" {
		return fail()
	}
	if !parentMaterialBytes(raw, expected, MaxParentProofMaterialBytes) {
		return fail()
	}
	f, err := custodyObject(raw, parentProofKeys, []string{"nextSyncCommittee", "nextCommitteeBranch"})
	if err != nil || !parentMaterialPurpose(f, parentProofPurpose) {
		return fail()
	}
	mode, err := custodyText(f["mode"])
	if err != nil || mode != "single-period" && mode != "adjacent-period" ||
		mode == "single-period" && len(f) != len(parentProofKeys) ||
		mode == "adjacent-period" && len(f) != len(parentProofKeys)+2 {
		return fail()
	}
	m := ParentProofMaterial{fork: fork}
	p := &m.proof
	for _, item := range []struct {
		name string
		out  *ParentBeaconHeader
	}{{"bootstrap", &p.Bootstrap}, {"attested", &p.Attested}, {"finalized", &p.Finalized}} {
		value, err := parentMaterialHeader(f[item.name])
		if err != nil {
			return fail()
		}
		*item.out = value
	}
	if parentMaterialCommittee(f["syncCommittee"], &p.Committee, &p.AggregatePubkey) != nil {
		return fail()
	}
	for _, item := range []struct {
		name  string
		count int
		out   *[][32]byte
	}{{"committeeBranch", depth, &p.CommitteeBranch}, {"finalityBranch", depth + 1, &p.FinalityBranch}, {"executionHashBranch", 9, &p.ExecutionHashBranch}} {
		value, err := parentMaterialBranch(f[item.name], item.count)
		if err != nil {
			return fail()
		}
		*item.out = value
	}
	for _, item := range []struct {
		name string
		out  []byte
	}{{"participation", p.Participation[:]}, {"signature", p.Signature[:]}} {
		value, err := custodyHex(f[item.name], len(item.out))
		if err != nil {
			return fail()
		}
		copy(item.out, value)
	}
	p.SignatureSlot, err = parentMaterialUint(f["signatureSlot"])
	if err != nil {
		return fail()
	}
	headerText, err := custodyText(f["finalizedExecutionHeader"])
	if err != nil || len(headerText) < 4 || len(headerText) > 2+2*4096 || (len(headerText)-2)%2 != 0 {
		return fail()
	}
	p.FinalizedExecutionHeader, err = custodyHex(f["finalizedExecutionHeader"], (len(headerText)-2)/2)
	if err != nil {
		return fail()
	}
	if mode == "adjacent-period" {
		m.next = &ParentNextCommitteeProof{}
		if parentMaterialCommittee(f["nextSyncCommittee"], &m.next.Committee, &m.next.AggregatePubkey) != nil {
			return fail()
		}
		m.next.Branch, err = parentMaterialBranch(f["nextCommitteeBranch"], depth)
		if err != nil {
			return fail()
		}
	}
	m.accepted = true
	return m, nil
}

type ParentMaterialReport struct {
	ParentCommitteeTransitionReport
	OfflineMaterialsPinned bool `json:"offlineMaterialsPinned"`
	OfflineOnly            bool `json:"offlineOnly"`
}

// ReviewParentMaterials is read-only and uses separate trust/proof pins and a
// separately supplied clock. Pins do not authenticate provenance. No fetched
// checkpoint, persisted head, signer, network, execution replay or asset gate.
// All input bytes are bounded/detached before caller context callbacks.
func ReviewParentMaterials(ctx context.Context, trustRaw, proofRaw []byte, trustPin, proofPin [32]byte, current uint64) (report ParentMaterialReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = ParentMaterialReport{}, ErrParentMaterial
		}
	}()
	fail := func() (ParentMaterialReport, error) { return ParentMaterialReport{}, ErrParentMaterial }
	if len(trustRaw) > MaxParentTrustMaterialBytes || len(proofRaw) > MaxParentProofMaterialBytes {
		return fail()
	}
	trustRaw, proofRaw = bytes.Clone(trustRaw), bytes.Clone(proofRaw)
	if ctx == nil || ctx.Err() != nil {
		return fail()
	}
	t, err := ParseParentTrustMaterial(trustRaw, trustPin, current)
	if err != nil {
		return fail()
	}
	m, err := ParseParentProofMaterial(proofRaw, proofPin, t.ForkName)
	if err != nil || !m.accepted || m.fork != t.ForkName {
		return fail()
	}
	var r ParentCommitteeTransitionReport
	if m.next != nil {
		r, err = VerifyParentCommitteeTransitionProof(ctx, t, ParentCommitteeTransitionProof{Finality: m.proof, Next: *m.next})
	} else {
		r.ParentFinalityReport, err = VerifyParentFinalityProof(ctx, t, m.proof)
	}
	if err != nil || ctx.Err() != nil {
		return fail()
	}
	return ParentMaterialReport{ParentCommitteeTransitionReport: r, OfflineMaterialsPinned: true, OfflineOnly: true}, nil
}

// Explicit private files only: owner-only POSIX leaf storage, no symlinks or
// hardlinks and separately retained whole-file pins. Unsupported ACL platforms
// refuse disk reads; the portable pure byte verifier remains available.
func ReviewPrivateParentMaterials(ctx context.Context, trustPath, proofPath string, trustPin, proofPin [32]byte, current uint64) (report ParentMaterialReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = ParentMaterialReport{}, ErrParentMaterial
		}
	}()
	if ctx == nil || ctx.Err() != nil {
		return ParentMaterialReport{}, ErrParentMaterial
	}
	trustRaw, err := readRHPrivateSnapshot(trustPath, MaxParentTrustMaterialBytes, trustPin)
	if err != nil || ctx.Err() != nil {
		return ParentMaterialReport{}, ErrParentMaterial
	}
	proofRaw, err := readRHPrivateSnapshot(proofPath, MaxParentProofMaterialBytes, proofPin)
	if err != nil {
		return ParentMaterialReport{}, ErrParentMaterial
	}
	return ReviewParentMaterials(ctx, trustRaw, proofRaw, trustPin, proofPin, current)
}
