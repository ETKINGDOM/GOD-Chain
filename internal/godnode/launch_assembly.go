package godnode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	abci "github.com/cometbft/cometbft/abci/types"
	cmted "github.com/cometbft/cometbft/crypto/ed25519"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmttypes "github.com/cometbft/cometbft/types"
)

var ErrLaunchAssembly = errors.New("bridge launch candidate assembly rejected; no production start or source backing is authorized")

// Explicit candidate limits, never mainnet defaults or measured funding. This
// assembly admits an empty redemption pool: no declared pool seed is fabricated.
type BridgeLaunchBudget struct {
	MaximumStartupGod sdkmath.Int
	WorkingGasGod     sdkmath.Int
	BlockInterval     time.Duration
}
type BridgeLaunchReport struct {
	PrototypeOnly             bool `json:"prototypeOnly"`
	Validators                int  `json:"validators"`
	Deposits                  int  `json:"deposits"`
	Accounts                  int  `json:"accounts"`
	FundingReconciled         bool `json:"fundingReconciled"`
	WorkingGasReconciled      bool `json:"workingGasReconciled"`
	ValidatorProofRequired    bool `json:"validatorProofRequired"`
	EmptyRedemptionPool       bool `json:"emptyRedemptionPool"`
	NodeAuthorizationVerified bool `json:"nodeAuthorizationVerified"`
	SourceBackingVerified     bool `json:"sourceBackingVerified"`
	SigningEnabled            bool `json:"signingEnabled"`
	StartEnabled              bool `json:"startEnabled"`
	RealAssetsReady           bool `json:"realAssetsReady"`
}

// Owns detached private configuration/genesis. Ordinary formatting and JSON
// expose only the redacted report; explicit Payload is for private staging.
type BridgeLaunchProposal struct {
	config  Config
	genesis *cmttypes.GenesisDoc
	digest  [32]byte
	report  BridgeLaunchReport
}
type BridgeLaunchCandidate struct{ proposal BridgeLaunchProposal }

func (BridgeLaunchProposal) String() string                 { return "private bridge launch proposal (redacted)" }
func (p BridgeLaunchProposal) GoString() string             { return p.String() }
func (p BridgeLaunchProposal) MarshalJSON() ([]byte, error) { return json.Marshal(p.report) }
func (p BridgeLaunchProposal) Report() BridgeLaunchReport   { return p.report }
func (BridgeLaunchCandidate) String() string {
	return "private authorized synthetic launch candidate (redacted)"
}
func (c BridgeLaunchCandidate) GoString() string             { return c.String() }
func (c BridgeLaunchCandidate) MarshalJSON() ([]byte, error) { return json.Marshal(c.Report()) }
func (c BridgeLaunchCandidate) Report() BridgeLaunchReport   { return c.proposal.report }

func cloneLaunchConfig(c Config) (Config, error) {
	raw, err := json.Marshal(c)
	if err != nil || len(raw) > 16<<10 {
		return Config{}, ErrLaunchAssembly
	}
	var out Config
	if json.Unmarshal(raw, &out) != nil || out.validate() != nil {
		return Config{}, ErrLaunchAssembly
	}
	return out, nil
}
func cloneLaunchGenesis(g *cmttypes.GenesisDoc) (*cmttypes.GenesisDoc, error) {
	if g == nil {
		return nil, ErrLaunchAssembly
	}
	raw, err := cmtjson.Marshal(g)
	if err != nil || len(raw) > 2<<20 {
		return nil, ErrLaunchAssembly
	}
	var out cmttypes.GenesisDoc
	if cmtjson.Unmarshal(raw, &out) != nil {
		return nil, ErrLaunchAssembly
	}
	return &out, nil
}
func launchInitRequest(g *cmttypes.GenesisDoc) (*abci.RequestInitChain, error) {
	if g == nil || g.ConsensusParams == nil || g.InitialHeight != 1 || g.ValidateAndComplete() != nil {
		return nil, ErrLaunchAssembly
	}
	cp := g.ConsensusParams.ToProto()
	req := &abci.RequestInitChain{Time: g.GenesisTime, ChainId: g.ChainID, InitialHeight: 1, AppStateBytes: bytes.Clone(g.AppState), ConsensusParams: &cp}
	for _, v := range g.Validators {
		if v.PubKey == nil || v.PubKey.Type() != "ed25519" || len(v.PubKey.Bytes()) != 32 {
			return nil, ErrLaunchAssembly
		}
		req.Validators = append(req.Validators, abci.Ed25519ValidatorUpdate(v.PubKey.Bytes(), v.Power))
	}
	return req, nil
}

// Pure assembly from an already quorum-approved synthetic bootstrap plan.
// Balances and consensus validators are derived, never entered independently.
// No key generation/signing, database, listener, provider or production flag.
func PrepareBridgeLaunch(ctx context.Context, c Config, plan godbridge.BootstrapPlan, planApprovals [][]byte, consensus *cmttypes.ConsensusParams, budget BridgeLaunchBudget) (proposal BridgeLaunchProposal, err error) {
	defer func() {
		if recover() != nil {
			proposal, err = BridgeLaunchProposal{}, ErrLaunchAssembly
		}
	}()
	if ctx == nil || ctx.Err() != nil || !c.Prototype || !c.RequireValidatorProof || c.BridgeGenesis == nil || c.BridgeApprovalGas == 0 || consensus == nil ||
		budget.MaximumStartupGod.IsNil() || budget.WorkingGasGod.IsNil() || !budget.MaximumStartupGod.IsPositive() || !budget.WorkingGasGod.IsPositive() ||
		budget.MaximumStartupGod.GT(godbridge.ExposureLimit()) || budget.BlockInterval < 100*time.Millisecond || budget.BlockInterval > 6*time.Second {
		return proposal, ErrLaunchAssembly
	}
	c, err = cloneLaunchConfig(c)
	if err != nil {
		return BridgeLaunchProposal{}, err
	}
	binding, err := godbridge.CanonicalConfig(*c.BridgeGenesis)
	if err != nil {
		return BridgeLaunchProposal{}, ErrLaunchAssembly
	}
	c.BridgeGenesis = &binding
	p, _, err := godbridge.ReviewBootstrap(binding, plan, planApprovals)
	if err != nil {
		return BridgeLaunchProposal{}, ErrLaunchAssembly
	}
	fees, stake := sdkmath.ZeroInt(), sdkmath.ZeroInt()
	for _, f := range p.FeeBudgets {
		fees = fees.Add(f.Amount)
	}
	for _, v := range p.Validators {
		stake = stake.Add(v.SelfStake)
	}
	if !fees.Equal(budget.WorkingGasGod) || !stake.Add(fees).Equal(p.Checkpoint.CreditedGod) || p.Checkpoint.CreditedGod.GT(budget.MaximumStartupGod) {
		return BridgeLaunchProposal{}, ErrLaunchAssembly
	}
	cpRaw, err := json.Marshal(consensus)
	if err != nil || len(cpRaw) > 4096 {
		return BridgeLaunchProposal{}, ErrLaunchAssembly
	}
	var cp cmttypes.ConsensusParams
	if json.Unmarshal(cpRaw, &cp) != nil {
		return BridgeLaunchProposal{}, ErrLaunchAssembly
	}
	g := Genesis{Version: 1, Prototype: true, Bridge: &BridgeGenesisCertificate{Version: 1, Plan: p}}
	for _, a := range planApprovals {
		g.Bridge.PlanApprovals = append(g.Bridge.PlanApprovals, bytes.Clone(a))
	}
	balances := map[[20]byte]sdkmath.Int{}
	for _, d := range p.Deposits {
		n, ok := balances[d.Deposit.Recipient]
		if !ok {
			n = sdkmath.ZeroInt()
		}
		balances[d.Deposit.Recipient] = n.Add(d.Deposit.Amount)
	}
	for owner, amount := range balances {
		address, e := godaddress.ToNative(owner[:])
		if e != nil {
			return BridgeLaunchProposal{}, ErrLaunchAssembly
		}
		g.Balances = append(g.Balances, GenesisBalance{Address: address, Amount: amount.String()})
	}
	sort.Slice(g.Balances, func(i, j int) bool { return g.Balances[i].Address < g.Balances[j].Address })
	genesis := &cmttypes.GenesisDoc{ChainID: c.ChainID, GenesisTime: p.GenesisTime, InitialHeight: 1, ConsensusParams: &cp}
	for _, v := range p.Validators {
		owner, e := godaddress.ToNative(v.Owner[:])
		if e != nil {
			return BridgeLaunchProposal{}, ErrLaunchAssembly
		}
		pub := bytes.Clone(v.ConsensusKey[:])
		g.Validators = append(g.Validators, GenesisValidator{Owner: owner, PublicKey: pub, Stake: v.SelfStake.String()})
		genesis.Validators = append(genesis.Validators, cmttypes.GenesisValidator{PubKey: cmted.PubKey(bytes.Clone(pub)), Power: v.SelfStake.Quo(godrewards.Unit()).Int64()})
	}
	sort.Slice(g.Validators, func(i, j int) bool { return g.Validators[i].Owner < g.Validators[j].Owner })
	genesis.AppState, err = json.Marshal(g)
	if err != nil {
		return BridgeLaunchProposal{}, ErrLaunchAssembly
	}
	req, err := launchInitRequest(genesis)
	if err != nil {
		return BridgeLaunchProposal{}, err
	}
	digest, err := BridgeGenesisDigest(c, req)
	if err != nil || ctx.Err() != nil {
		return BridgeLaunchProposal{}, ErrLaunchAssembly
	}
	return BridgeLaunchProposal{config: c, genesis: genesis, digest: digest, report: BridgeLaunchReport{PrototypeOnly: true, Validators: len(p.Validators), Deposits: len(p.Deposits), Accounts: len(balances), FundingReconciled: true, WorkingGasReconciled: true, ValidatorProofRequired: true, EmptyRedemptionPool: true}}, nil
}

// Explicit private challenge. It is not a signature, source proof or vote.
func (p BridgeLaunchProposal) NodeDigest() ([32]byte, error) {
	if !p.report.FundingReconciled || p.digest == [32]byte{} {
		return [32]byte{}, ErrLaunchAssembly
	}
	return p.digest, nil
}

// Completes only with independently supplied quorum signatures and exact owner
// consent/consensus-key possession. No automatic signer or consent fabrication.
func (p BridgeLaunchProposal) Complete(ctx context.Context, nodeApprovals [][]byte, consents []BridgeValidatorConsent) (candidate BridgeLaunchCandidate, err error) {
	defer func() {
		if recover() != nil {
			candidate, err = BridgeLaunchCandidate{}, ErrLaunchAssembly
		}
	}()
	if ctx == nil || ctx.Err() != nil || p.digest == [32]byte{} || !p.report.FundingReconciled {
		return candidate, ErrLaunchAssembly
	}
	c, err := cloneLaunchConfig(p.config)
	if err != nil {
		return candidate, err
	}
	g, err := cloneLaunchGenesis(p.genesis)
	if err != nil {
		return candidate, err
	}
	state, err := decodeGenesis(g.AppState)
	if err != nil || state.Bridge == nil {
		return candidate, ErrLaunchAssembly
	}
	state.Bridge.NodeApprovals = nil
	state.Bridge.Consents = nil
	for _, a := range nodeApprovals {
		state.Bridge.NodeApprovals = append(state.Bridge.NodeApprovals, bytes.Clone(a))
	}
	for _, v := range consents {
		v.OwnerSignature = bytes.Clone(v.OwnerSignature)
		v.ConsensusSignature = bytes.Clone(v.ConsensusSignature)
		state.Bridge.Consents = append(state.Bridge.Consents, v)
	}
	g.AppState, err = json.Marshal(state)
	if err != nil {
		return candidate, ErrLaunchAssembly
	}
	req, err := launchInitRequest(g)
	if err != nil || validateGenesisRequest(req, c) != nil {
		return candidate, ErrLaunchAssembly
	}
	digest, err := BridgeGenesisDigest(c, req)
	if err != nil || digest != p.digest || ctx.Err() != nil {
		return candidate, ErrLaunchAssembly
	}
	p.config, p.genesis = c, g
	p.report.NodeAuthorizationVerified = true
	return BridgeLaunchCandidate{proposal: p}, nil
}

// Detached exact runtime/genesis for an explicitly private synthetic rehearsal.
// This does not install, start, sign, enable public peering or attest RH backing.
func (c BridgeLaunchCandidate) Payload(ctx context.Context) (config Config, genesis *cmttypes.GenesisDoc, err error) {
	defer func() {
		if recover() != nil {
			config, genesis, err = Config{}, nil, ErrLaunchAssembly
		}
	}()
	if ctx == nil || ctx.Err() != nil || !c.proposal.report.NodeAuthorizationVerified {
		return Config{}, nil, ErrLaunchAssembly
	}
	config, err = cloneLaunchConfig(c.proposal.config)
	if err != nil {
		return Config{}, nil, err
	}
	genesis, err = cloneLaunchGenesis(c.proposal.genesis)
	if err != nil {
		return Config{}, nil, err
	}
	req, err := launchInitRequest(genesis)
	if err != nil || validateGenesisRequest(req, config) != nil || ctx.Err() != nil {
		return Config{}, nil, ErrLaunchAssembly
	}
	return config, genesis, nil
}
