package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmttypes "github.com/cometbft/cometbft/types"
)

const MaxLaunchAssemblyBytes = 1 << 20
const launchAssemblyPurpose = "god-chain-synthetic-launch-assembly-v1"

var ErrLaunchAssembly = errors.New("private launch assembly rejected; no production activation or verified source funding")

// Typed encoding/json also accepts case-folded aliases. Require exact canonical
// keys at every object depth; duplicate/trailing/depth rejection is separate.
func launchAssemblyExactKeys(raw, canonical []byte) bool {
	decode := func(b []byte) (any, error) {
		d := json.NewDecoder(bytes.NewReader(b))
		d.UseNumber()
		var v any
		err := d.Decode(&v)
		return v, err
	}
	a, err := decode(raw)
	if err != nil {
		return false
	}
	b, err := decode(canonical)
	if err != nil {
		return false
	}
	var same func(any, any) bool
	same = func(a, b any) bool {
		switch x := a.(type) {
		case map[string]any:
			y, ok := b.(map[string]any)
			if !ok || len(x) != len(y) {
				return false
			}
			for k, v := range x {
				w, ok := y[k]
				if !ok || !same(v, w) {
					return false
				}
			}
			return true
		case []any:
			y, ok := b.([]any)
			if !ok || len(x) != len(y) {
				return false
			}
			for i, v := range x {
				if !same(v, y[i]) {
					return false
				}
			}
			return true
		default:
			switch b.(type) {
			case map[string]any, []any:
				return false
			}
			return true
		}
	}
	return same(a, b)
}

// Explicit private operator inputs. No monetary values, chain IDs, populated
// contracts, endpoints or signing keys are defaulted by the template.
type launchAssemblyDocument struct {
	Purpose             string                           `json:"purpose"`
	Version             uint32                           `json:"version"`
	Economics           json.RawMessage                  `json:"economics"`
	Runtime             godnode.Config                   `json:"runtime"`
	Bootstrap           godbridge.BootstrapPlan          `json:"bootstrap"`
	PlanApprovals       [][]byte                         `json:"planApprovals"`
	NodeApprovals       [][]byte                         `json:"nodeApprovals"`
	Consents            []godnode.BridgeValidatorConsent `json:"consents"`
	ConsensusParams     *cmttypes.ConsensusParams        `json:"consensusParams"`
	BlockIntervalMillis uint32                           `json:"blockIntervalMillis"`
}

type launchCandidateArtifact struct {
	Version             uint32          `json:"version"`
	Mode                string          `json:"mode"`
	InputSHA256         [32]byte        `json:"inputSha256"`
	Runtime             godnode.Config  `json:"runtime"`
	Genesis             json.RawMessage `json:"genesis"`
	BlockIntervalMillis uint32          `json:"blockIntervalMillis"`
}

func launchCandidateBytes(ctx context.Context, raw []byte, pin [32]byte) ([]byte, godnode.Config, *cmttypes.GenesisDoc, time.Duration, LaunchAssemblyReport, error) {
	c, r, err := ReviewLaunchAssembly(ctx, raw, pin)
	if err != nil {
		return nil, godnode.Config{}, nil, 0, LaunchAssemblyReport{}, ErrLaunchAssembly
	}
	runtime, genesis, err := c.Payload(ctx)
	if err != nil {
		return nil, godnode.Config{}, nil, 0, LaunchAssemblyReport{}, ErrLaunchAssembly
	}
	g, err := cmtjson.Marshal(genesis)
	var d launchAssemblyDocument
	if err != nil || json.Unmarshal(raw, &d) != nil {
		return nil, godnode.Config{}, nil, 0, LaunchAssemblyReport{}, ErrLaunchAssembly
	}
	artifact, err := json.Marshal(launchCandidateArtifact{1, "synthetic-bridge-launch-candidate", pin, runtime, g, d.BlockIntervalMillis})
	if err != nil || len(artifact) > MaxLaunchAssemblyBytes || ctx.Err() != nil {
		return nil, godnode.Config{}, nil, 0, LaunchAssemblyReport{}, ErrLaunchAssembly
	}
	return artifact, runtime, genesis, time.Duration(d.BlockIntervalMillis) * time.Millisecond, r, nil
}

// ReadPrivateLaunchCandidate re-derives the exact staged bytes from the original
// independently pinned, fully signed input. The artifact's own input hash or a
// supplied report is never authority. Both original files are re-read before
// returning detached prototype inputs. This opens no database/listener, holds
// no operator key and does not authenticate real source backing.
func ReadPrivateLaunchCandidate(ctx context.Context, input string, inputPin [32]byte, artifact string, artifactPin [32]byte) (runtime godnode.Config, genesis *cmttypes.GenesisDoc, interval time.Duration, report LaunchAssemblyReport, err error) {
	bad := func() (godnode.Config, *cmttypes.GenesisDoc, time.Duration, LaunchAssemblyReport, error) {
		return godnode.Config{}, nil, 0, LaunchAssemblyReport{}, ErrLaunchAssembly
	}
	defer func() {
		if recover() != nil {
			runtime, genesis, interval, report, err = bad()
		}
	}()
	if ctx == nil || ctx.Err() != nil || input == artifact {
		return bad()
	}
	raw, err := readRHPrivateBytes(input, MaxLaunchAssemblyBytes, inputPin)
	if err != nil {
		return bad()
	}
	retained, err := readRHPrivateBytes(artifact, MaxLaunchAssemblyBytes, artifactPin)
	if err != nil {
		return bad()
	}
	want, runtime, genesis, interval, report, err := launchCandidateBytes(ctx, raw, inputPin)
	if err != nil || !bytes.Equal(want, retained) {
		return bad()
	}
	inputAgain, e := readRHPrivateBytes(input, MaxLaunchAssemblyBytes, inputPin)
	artifactAgain, f := readRHPrivateBytes(artifact, MaxLaunchAssemblyBytes, artifactPin)
	if e != nil || f != nil || !bytes.Equal(inputAgain, raw) || !bytes.Equal(artifactAgain, retained) || ctx.Err() != nil {
		return bad()
	}
	report.CandidateStaged = true
	return runtime, genesis, interval, report, nil
}

func LaunchAssemblyTemplate() []byte {
	// Null runtime/plan/consensus are deliberate incomplete placeholders. The
	// template cannot pass review or create an executable candidate.
	raw, _ := json.MarshalIndent(struct {
		Purpose             string                           `json:"purpose"`
		Version             uint32                           `json:"version"`
		Economics           json.RawMessage                  `json:"economics"`
		Runtime             any                              `json:"runtime"`
		Bootstrap           any                              `json:"bootstrap"`
		PlanApprovals       [][]byte                         `json:"planApprovals"`
		NodeApprovals       [][]byte                         `json:"nodeApprovals"`
		Consents            []godnode.BridgeValidatorConsent `json:"consents"`
		ConsensusParams     any                              `json:"consensusParams"`
		BlockIntervalMillis uint32                           `json:"blockIntervalMillis"`
	}{Purpose: launchAssemblyPurpose, Version: 1, Economics: LaunchEconomicsTemplate(), PlanApprovals: [][]byte{}, NodeApprovals: [][]byte{}, Consents: []godnode.BridgeValidatorConsent{}}, "", "  ")
	return append(raw, '\n')
}

type LaunchAssemblyReport struct {
	Economics                 godrhLaunchEconomicsSummary `json:"economics"`
	Node                      godnode.BridgeLaunchReport  `json:"node"`
	EconomicRuntimeMatched    bool                        `json:"economicRuntimeMatched"`
	SingleHostFounderDeclared bool                        `json:"singleHostFounderDeclared"`
	CandidateStaged           bool                        `json:"candidateStaged"`
	ProductionStartEnabled    bool                        `json:"productionStartEnabled"`
	RealAssetsReady           bool                        `json:"realAssetsReady"`
}

// Redacted summary, not a second economics calculator or new policy defaults.
type godrhLaunchEconomicsSummary struct {
	ArithmeticConsistent  bool   `json:"arithmeticConsistent"`
	InitialValidators     uint64 `json:"initialValidators"`
	OwnerApproved         bool   `json:"ownerApproved"`
	SourceBackingVerified bool   `json:"sourceBackingVerified"`
}

func prepareLaunchAssembly(ctx context.Context, raw []byte, pin [32]byte) (p godnode.BridgeLaunchProposal, d launchAssemblyDocument, r LaunchAssemblyReport, err error) {
	defer func() {
		if recover() != nil {
			p, d, r, err = godnode.BridgeLaunchProposal{}, launchAssemblyDocument{}, LaunchAssemblyReport{}, ErrLaunchAssembly
		}
	}()
	bad := func() (godnode.BridgeLaunchProposal, launchAssemblyDocument, LaunchAssemblyReport, error) {
		return godnode.BridgeLaunchProposal{}, launchAssemblyDocument{}, LaunchAssemblyReport{}, ErrLaunchAssembly
	}
	if len(raw) == 0 || len(raw) > MaxLaunchAssemblyBytes {
		return bad()
	}
	raw = bytes.Clone(raw)
	if ctx == nil || ctx.Err() != nil || pin == [32]byte{} || sha256.Sum256(raw) != pin || !uniqueTokenSourceJSON(raw) {
		return bad()
	}
	fields := []string{"purpose", "version", "economics", "runtime", "bootstrap", "planApprovals", "nodeApprovals", "consents", "consensusParams", "blockIntervalMillis"}
	if _, err := custodyObject(raw, fields, nil); err != nil {
		return bad()
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&d) != nil || d.Purpose != launchAssemblyPurpose || d.Version != 1 || decoder.Decode(new(any)) != io.EOF {
		return bad()
	}
	canonical, err := json.Marshal(d)
	if err != nil || !launchAssemblyExactKeys(raw, canonical) {
		return bad()
	}
	economic, err := ReviewLaunchEconomics(ctx, d.Economics, sha256.Sum256(d.Economics))
	if err != nil || !economic.ArithmeticConsistent {
		return bad()
	}
	var e launchEconomicsDocument
	if json.Unmarshal(d.Economics, &e) != nil {
		return bad()
	}
	seconds, err := ParentMaterialSlot(e.AssumedBlockSeconds)
	if err != nil || seconds > 6 || uint64(d.BlockIntervalMillis) != seconds*1000 || !d.Runtime.Prototype || !d.Runtime.RequireValidatorProof || d.Runtime.BridgeApprovalGas == 0 ||
		d.Runtime.GPerSignedBlock.IsNil() || d.Runtime.Policy.MinFeePerGas.IsNil() || d.Runtime.GPerSignedBlock.String() != e.GPerSignedBlockSmallestUnits ||
		d.Runtime.Policy.MinFeePerGas.String() != e.MinFeePerGasSmallestUnits || uint64(len(d.Bootstrap.Validators)) != economic.InitialValidators || e.RedemptionSeedGODSmallestUnits != "0" {
		return bad()
	}
	startup, ok := sdkmath.NewIntFromString(e.StartupGODSmallestUnits)
	if !ok {
		return bad()
	}
	working, ok := sdkmath.NewIntFromString(e.GasWorkingGODSmallestUnits)
	if !ok {
		return bad()
	}
	p, err = godnode.PrepareBridgeLaunch(ctx, d.Runtime, d.Bootstrap, d.PlanApprovals, d.ConsensusParams, godnode.BridgeLaunchBudget{MaximumStartupGod: startup, WorkingGasGod: working, BlockInterval: time.Duration(d.BlockIntervalMillis) * time.Millisecond})
	if err != nil || ctx.Err() != nil {
		return bad()
	}
	r = LaunchAssemblyReport{Economics: godrhLaunchEconomicsSummary{ArithmeticConsistent: true, InitialValidators: economic.InitialValidators}, Node: p.Report(), EconomicRuntimeMatched: true, SingleHostFounderDeclared: true}
	return p, d, r, nil
}

// Explicit private digest allows independent offline consent preparation. No
// signature, key, proof acquisition or financial authority is furnished here.
func LaunchAssemblyNodeDigest(ctx context.Context, raw []byte, pin [32]byte) ([32]byte, error) {
	p, _, _, err := prepareLaunchAssembly(ctx, raw, pin)
	if err != nil {
		return [32]byte{}, err
	}
	digest, err := p.NodeDigest()
	if err != nil {
		return [32]byte{}, ErrLaunchAssembly
	}
	return digest, nil
}

// Compose complete economic/funding/runtime/owner/quorum authorization into
// actual detached node inputs, not a collection of unrelated passing reports.
func ReviewLaunchAssembly(ctx context.Context, raw []byte, pin [32]byte) (candidate godnode.BridgeLaunchCandidate, report LaunchAssemblyReport, err error) {
	defer func() {
		if recover() != nil {
			candidate, report, err = godnode.BridgeLaunchCandidate{}, LaunchAssemblyReport{}, ErrLaunchAssembly
		}
	}()
	p, d, r, err := prepareLaunchAssembly(ctx, raw, pin)
	if err != nil {
		return candidate, report, err
	}
	c, err := p.Complete(ctx, d.NodeApprovals, d.Consents)
	if err != nil || ctx.Err() != nil {
		return candidate, report, ErrLaunchAssembly
	}
	r.Node = c.Report()
	return c, r, nil
}

func ReviewPrivateLaunchAssembly(ctx context.Context, path string, pin [32]byte) (report LaunchAssemblyReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = LaunchAssemblyReport{}, ErrLaunchAssembly
		}
	}()
	raw, err := readRHPrivateBytes(path, MaxLaunchAssemblyBytes, pin)
	if err != nil {
		return report, ErrLaunchAssembly
	}
	_, r, err := ReviewLaunchAssembly(ctx, raw, pin)
	if err != nil {
		return report, err
	}
	again, err := readRHPrivateBytes(path, MaxLaunchAssemblyBytes, pin)
	if err != nil || !bytes.Equal(raw, again) || ctx.Err() != nil {
		return report, ErrLaunchAssembly
	}
	return r, nil
}

// New private staging artifact only. The current runtime remains prototype;
// ordinary mainnet start/bridge/mint and public peering remain disabled.
func StagePrivateLaunchAssembly(ctx context.Context, input string, pin [32]byte, output string) (report LaunchAssemblyReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = LaunchAssemblyReport{}, ErrLaunchAssembly
		}
	}()
	if input == output {
		return report, ErrLaunchAssembly
	}
	raw, err := readRHPrivateBytes(input, MaxLaunchAssemblyBytes, pin)
	if err != nil {
		return report, ErrLaunchAssembly
	}
	artifact, _, _, _, r, err := launchCandidateBytes(ctx, raw, pin)
	if err != nil {
		return report, ErrLaunchAssembly
	}
	again, err := readRHPrivateBytes(input, MaxLaunchAssemblyBytes, pin)
	if err != nil || !bytes.Equal(raw, again) || ctx.Err() != nil {
		return report, ErrLaunchAssembly
	}
	if writeLaunchAssemblyPrivateFile(output, artifact) != nil {
		return report, ErrLaunchAssembly
	}
	retained, err := readRHPrivateBytes(output, MaxLaunchAssemblyBytes, sha256.Sum256(artifact))
	if err != nil || !bytes.Equal(retained, artifact) || ctx.Err() != nil {
		return report, ErrLaunchAssembly
	}
	r.CandidateStaged = true
	return r, nil
}
