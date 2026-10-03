package godbridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sort"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// These are bounded prototype inputs, not approved mainnet genesis limits.
const MaxBootstrapDeposits = 256
const MaxBootstrapValidators = 32

var ErrBootstrap = errors.New("invalid or missing bridge bootstrap reconciliation")
var bootstrapKey = []byte{3}

// BootstrapCheckpoint is a federated statement over finalized source state,
// NOT a storage proof or an implemented RH observer. CreditedGod must be the
// custody ledger's credit, never its raw token balance (which can contain gifts).
// NoWithdrawalOutcomes asserts no prior payment OR cancellation anywhere.
type BootstrapCheckpoint struct {
	Height               uint64
	BlockHash            [32]byte
	NextDeposit          uint64
	CreditedGod          sdkmath.Int
	NoWithdrawalOutcomes bool
}

type BootstrapDeposit struct {
	Deposit   Deposit
	Approvals [][]byte
}

type BootstrapValidator struct {
	Owner        [20]byte
	ConsensusKey [32]byte
	SelfStake    sdkmath.Int
}

type BootstrapFeeBudget struct {
	Owner  [20]byte
	Amount sdkmath.Int
}

// BootstrapPlan describes initial deposit-funded account balances and budgets.
// It does not install validators, reserve/spend fee budgets or start consensus.
// The node's optional certificate separately checks validator consent and
// node-policy binding; this plan alone never authorizes staking. Independent
// source evidence and authenticated node transaction mounting remain release
// requirements.
// No populated plan belongs in public source.
type BootstrapPlan struct {
	Version     uint32
	GenesisTime time.Time
	Checkpoint  BootstrapCheckpoint
	Deposits    []BootstrapDeposit
	Validators  []BootstrapValidator
	FeeBudgets  []BootstrapFeeBudget
}

type BootstrapRecord struct {
	Version         uint32      `json:"version"`
	Digest          [32]byte    `json:"digest"`
	GenesisTime     time.Time   `json:"genesisTime"`
	SourceHeight    uint64      `json:"sourceHeight"`
	SourceBlockHash [32]byte    `json:"sourceBlockHash"`
	Deposits        uint32      `json:"deposits"`
	Validators      uint32      `json:"validators"`
	Accounts        uint32      `json:"accounts"`
	ReleasedGod     sdkmath.Int `json:"initialReleasedGodSmallestUnits"`
}

func bootstrapAmount(n sdkmath.Int) bool {
	return !n.IsNil() && n.IsPositive() && n.LTE(ExposureLimit())
}

func (k Keeper) canonicalBootstrap(input BootstrapPlan) (BootstrapPlan, error) {
	if len(k.binding) == 0 || input.Version != 1 || !validTime(input.GenesisTime) ||
		input.Checkpoint.Height == 0 || input.Checkpoint.BlockHash == [32]byte{} ||
		!input.Checkpoint.NoWithdrawalOutcomes || !bootstrapAmount(input.Checkpoint.CreditedGod) ||
		len(input.Deposits) == 0 || len(input.Deposits) > MaxBootstrapDeposits ||
		input.Checkpoint.NextDeposit != uint64(len(input.Deposits))+1 ||
		len(input.Validators) == 0 || len(input.Validators) > MaxBootstrapValidators ||
		len(input.FeeBudgets) == 0 || len(input.FeeBudgets) > MaxBootstrapDeposits {
		return BootstrapPlan{}, ErrBootstrap
	}
	p := input
	p.GenesisTime = p.GenesisTime.UTC()
	p.Checkpoint.CreditedGod = sdkmath.NewIntFromBigInt(input.Checkpoint.CreditedGod.BigInt())
	p.Deposits = make([]BootstrapDeposit, len(input.Deposits))
	for i, d := range input.Deposits {
		if _, err := k.DepositDigest(d.Deposit); err != nil || len(d.Approvals) < Threshold || len(d.Approvals) > SignerCount {
			return BootstrapPlan{}, ErrBootstrap
		}
		p.Deposits[i].Deposit = d.Deposit
		p.Deposits[i].Deposit.Amount = sdkmath.NewIntFromBigInt(d.Deposit.Amount.BigInt())
		for _, s := range d.Approvals {
			if len(s) != 65 || s[64] > 1 {
				return BootstrapPlan{}, ErrBootstrap
			}
			p.Deposits[i].Approvals = append(p.Deposits[i].Approvals, append([]byte(nil), s...))
		}
	}
	p.Validators = append([]BootstrapValidator(nil), input.Validators...)
	p.FeeBudgets = append([]BootstrapFeeBudget(nil), input.FeeBudgets...)
	sort.Slice(p.Deposits, func(i, j int) bool { return p.Deposits[i].Deposit.Sequence < p.Deposits[j].Deposit.Sequence })
	sort.Slice(p.Validators, func(i, j int) bool { return bytes.Compare(p.Validators[i].Owner[:], p.Validators[j].Owner[:]) < 0 })
	sort.Slice(p.FeeBudgets, func(i, j int) bool { return bytes.Compare(p.FeeBudgets[i].Owner[:], p.FeeBudgets[j].Owner[:]) < 0 })
	balances := map[[20]byte]sdkmath.Int{}
	events := map[string]bool{}
	total := sdkmath.ZeroInt()
	var previous Evidence
	for i, entry := range p.Deposits {
		d := entry.Deposit
		if d.Sequence != uint64(i)+1 || d.Evidence.Height > p.Checkpoint.Height ||
			d.Evidence.Height == p.Checkpoint.Height && d.Evidence.BlockHash != p.Checkpoint.BlockHash ||
			i > 0 && (d.Evidence.Height < previous.Height || d.Evidence.Height == previous.Height && d.Evidence.BlockHash != previous.BlockHash) {
			return BootstrapPlan{}, ErrBootstrap
		}
		event := string(k.eventKey(d.Evidence))
		if events[event] {
			return BootstrapPlan{}, ErrBootstrap
		}
		events[event] = true
		previous = d.Evidence
		total = total.Add(d.Amount)
		if total.GT(ExposureLimit()) {
			return BootstrapPlan{}, ErrBootstrap
		}
		balance, found := balances[d.Recipient]
		if !found {
			balance = sdkmath.ZeroInt()
		}
		balances[d.Recipient] = balance.Add(d.Amount)
	}
	if !total.Equal(p.Checkpoint.CreditedGod) || len(balances) != len(p.FeeBudgets) {
		return BootstrapPlan{}, ErrBootstrap
	}
	remaining := map[[20]byte]sdkmath.Int{}
	for i, b := range p.FeeBudgets {
		balance, found := balances[b.Owner]
		if !found || b.Owner == [20]byte{} || !bootstrapAmount(b.Amount) || b.Amount.GT(balance) || i > 0 && p.FeeBudgets[i-1].Owner == b.Owner {
			return BootstrapPlan{}, ErrBootstrap
		}
		p.FeeBudgets[i].Amount = sdkmath.NewIntFromBigInt(b.Amount.BigInt())
		remaining[b.Owner] = balance.Sub(b.Amount)
	}
	seenKeys := map[[32]byte]bool{}
	for i, v := range p.Validators {
		balance, found := remaining[v.Owner]
		if !found || v.Owner == [20]byte{} || v.ConsensusKey == [32]byte{} || seenKeys[v.ConsensusKey] ||
			!bootstrapAmount(v.SelfStake) || v.SelfStake.LT(godrewards.Unit().MulRaw(1000)) || v.SelfStake.GT(balance) ||
			i > 0 && p.Validators[i-1].Owner == v.Owner {
			return BootstrapPlan{}, ErrBootstrap
		}
		seenKeys[v.ConsensusKey] = true
		p.Validators[i].SelfStake = sdkmath.NewIntFromBigInt(v.SelfStake.BigInt())
	}
	return p, nil
}

func (k Keeper) bootstrapDigest(p BootstrapPlan) [32]byte {
	b := k.domain("genesis-bootstrap")
	configuration := sha256.Sum256(k.binding)
	b.Write(configuration[:])
	b.u32(p.Version)
	b.clock(p.GenesisTime)
	b.u64(p.Checkpoint.Height)
	b.Write(p.Checkpoint.BlockHash[:])
	b.u64(p.Checkpoint.NextDeposit)
	b.amount(p.Checkpoint.CreditedGod)
	b.WriteByte(1) // Valid plans assert no withdrawal outcomes.
	b.u32(uint32(len(p.Deposits)))
	for _, d := range p.Deposits {
		h, _ := k.DepositDigest(d.Deposit)
		b.Write(h[:])
	}
	b.u32(uint32(len(p.Validators)))
	for _, v := range p.Validators {
		b.Write(v.Owner[:])
		b.Write(v.ConsensusKey[:])
		b.amount(v.SelfStake)
	}
	b.u32(uint32(len(p.FeeBudgets)))
	for _, fee := range p.FeeBudgets {
		b.Write(fee.Owner[:])
		b.amount(fee.Amount)
	}
	return sha256.Sum256(b.Bytes())
}

// BootstrapDigest signs a complete canonical plan, including checkpoint,
// validator keys, fee budgets and configuration. Deposit approvals are verified
// separately; their byte order is not part of the plan's content identity.
func (k Keeper) BootstrapDigest(plan BootstrapPlan) ([32]byte, error) {
	if !k.IsConfigured() {
		return [32]byte{}, ErrConfig
	}
	p, err := k.canonicalBootstrap(plan)
	if err != nil {
		return [32]byte{}, err
	}
	return k.bootstrapDigest(p), nil
}

// CanonicalConfig validates and detaches the immutable attestation policy.
// It supplies neither operational addresses nor independent source evidence.
func CanonicalConfig(config Config) (Config, error) { return config.canonical() }

func bootstrapVerifier(config Config) (Keeper, error) {
	config, err := config.canonical()
	if err != nil {
		return Keeper{}, err
	}
	binding, err := json.Marshal(config)
	if err != nil {
		return Keeper{}, ErrConfig
	}
	return Keeper{config: config, binding: binding}, nil
}

// DepositAttestationDigest is the configured deposit digest without ledger
// dependencies. It is only a signing proposal, not approval or source proof.
func DepositAttestationDigest(config Config, deposit Deposit) ([32]byte, error) {
	k, err := bootstrapVerifier(config)
	if err != nil {
		return [32]byte{}, err
	}
	return k.DepositDigest(deposit)
}

// AuthorizationAttestationDigest builds the exact configured withdrawal
// authorization packet without a ledger or state writes. It does not establish
// delay eligibility, queue order, source truth or signature approval.
func AuthorizationAttestationDigest(config Config, withdrawal Withdrawal) ([32]byte, error) {
	k, err := bootstrapVerifier(config)
	if err != nil {
		return [32]byte{}, err
	}
	return k.AuthorizationDigest(withdrawal)
}

// ResolutionAttestationDigest builds a paid/cancelled receipt proposal, not
// proof that such a receipt exists or is final. A cancellation request is a
// different action and cannot authorize a refund through this helper.
func ResolutionAttestationDigest(config Config, withdrawal Withdrawal, status Status, evidence Evidence) ([32]byte, error) {
	k, err := bootstrapVerifier(config)
	if err != nil {
		return [32]byte{}, err
	}
	return k.ResolutionDigest(withdrawal, status, evidence)
}

// PauseAttestationDigest builds an unsigned monotonic control proposal. It
// verifies configuration and packet structure, not caller authority or nonce
// freshness, and never changes bridge state or returns a usable ledger keeper.
func PauseAttestationDigest(config Config, nonce uint64, intake, outflow bool) ([32]byte, error) {
	k, err := bootstrapVerifier(config)
	if err != nil {
		return [32]byte{}, err
	}
	return k.PauseDigest(nonce, intake, outflow)
}

// CanonicalBootstrap performs bounded structural reconciliation without a
// ledger, account keeper or state writes. A digest is NOT signature approval
// or proof of backing. Returned slices and amounts are detached from input.
func CanonicalBootstrap(config Config, plan BootstrapPlan) (BootstrapPlan, [32]byte, error) {
	k, err := bootstrapVerifier(config)
	if err != nil {
		return BootstrapPlan{}, [32]byte{}, err
	}
	p, err := k.canonicalBootstrap(plan)
	if err != nil {
		return BootstrapPlan{}, [32]byte{}, err
	}
	return p, k.bootstrapDigest(p), nil
}

// VerifyAttestation verifies a bounded, distinct configured quorum over an
// already domain-separated digest. Callers own that digest's protocol domain.
// It verifies signatures, never the truth or finality of their source claim.
func VerifyAttestation(config Config, digest [32]byte, approvals [][]byte) error {
	k, err := bootstrapVerifier(config)
	if err != nil {
		return err
	}
	if digest == [32]byte{} {
		return ErrApprovals
	}
	return k.verify(digest, approvals)
}

// ReviewBootstrap checks every deposit and complete-plan approval without any
// SDK context or writes. Node genesis can use this gate before initialization;
// InitBootstrap remains the separate atomic ledger initialization API.
func ReviewBootstrap(config Config, plan BootstrapPlan, approvals [][]byte) (BootstrapPlan, [32]byte, error) {
	p, digest, err := CanonicalBootstrap(config, plan)
	if err != nil {
		return BootstrapPlan{}, [32]byte{}, err
	}
	k, err := bootstrapVerifier(config)
	if err != nil {
		return BootstrapPlan{}, [32]byte{}, err
	}
	if err := k.verify(digest, approvals); err != nil {
		return BootstrapPlan{}, [32]byte{}, err
	}
	for _, d := range p.Deposits {
		h, err := k.DepositDigest(d.Deposit)
		if err != nil {
			return BootstrapPlan{}, [32]byte{}, err
		}
		if err := k.verify(h, d.Approvals); err != nil {
			return BootstrapPlan{}, [32]byte{}, err
		}
	}
	return p, digest, nil
}

// InitBootstrap is an initialization-only trusted API, never a participant
// message or a ledger migration. Each source deposit and the complete initial
// plan require quorum approval. All initialization and release writes share one
// cache. No fees/stake are manufactured; this does not prove RH finality.
func (k Keeper) InitBootstrap(ctx sdk.Context, plan BootstrapPlan, approvals [][]byte) error {
	if !k.IsConfigured() || ctx.BlockHeight() != 0 || ctx.ChainID() != k.config.NativeChain || !ctx.BlockTime().Equal(plan.GenesisTime) {
		return ErrBootstrap
	}
	if k.store(ctx).Has(stateKey) {
		return ErrInitialized
	}
	p, err := k.canonicalBootstrap(plan)
	if err != nil {
		return err
	}
	digest := k.bootstrapDigest(p)
	if err := k.verify(digest, approvals); err != nil {
		return err
	}
	// Fresh participant accounts only: do not reinterpret a preexisting account
	// or vesting lock as immediately available initial stake/fee funding.
	for _, fee := range p.FeeBudgets {
		if k.accounts.GetAccount(ctx, fee.Owner[:]) != nil {
			return ErrBootstrap
		}
	}
	cache, write := ctx.CacheContext()
	if err := k.Init(cache); err != nil {
		return err
	}
	for _, d := range p.Deposits {
		if err := k.AcceptDeposit(cache, d.Deposit, d.Approvals); err != nil {
			return err
		}
	}
	balances := map[[20]byte]sdkmath.Int{}
	for _, d := range p.Deposits {
		n, found := balances[d.Deposit.Recipient]
		if !found {
			n = sdkmath.ZeroInt()
		}
		balances[d.Deposit.Recipient] = n.Add(d.Deposit.Amount)
	}
	for _, fee := range p.FeeBudgets {
		owner, balance := fee.Owner, balances[fee.Owner]
		a, ok := k.accounts.GetAccount(cache, owner[:]).(*authtypes.BaseAccount)
		if !ok || a == nil || a.GetSequence() != 0 || a.GetPubKey() != nil || !k.bank.GetBalance(cache, owner[:], godrewards.GodDenom).Amount.Equal(balance) {
			return ErrBootstrap
		}
	}
	s, err := k.Snapshot(cache)
	if err != nil || !s.ReleasedGod.Equal(p.Checkpoint.CreditedGod) || !s.PendingGod.IsZero() || s.HeadSequence != 1 || s.NextSequence != 1 {
		return ErrBootstrap
	}
	r := BootstrapRecord{Version: 1, Digest: digest, GenesisTime: p.GenesisTime, SourceHeight: p.Checkpoint.Height,
		SourceBlockHash: p.Checkpoint.BlockHash, Deposits: uint32(len(p.Deposits)), Validators: uint32(len(p.Validators)),
		Accounts: uint32(len(p.FeeBudgets)), ReleasedGod: s.ReleasedGod}
	raw, err := json.Marshal(r)
	if err != nil {
		return ErrBootstrap
	}
	ledger, err := k.getState(cache)
	if err != nil {
		return err
	}
	ledger.BootstrapRecordHash = sha256.Sum256(raw)
	stateBytes, err := json.Marshal(ledger)
	if err != nil {
		return ErrBootstrap
	}
	cache.KVStore(k.key).Set(bootstrapKey, raw)
	cache.KVStore(k.key).Set(stateKey, stateBytes)
	if err := k.CheckInvariants(cache); err != nil {
		return err
	}
	write()
	return nil
}

// Bootstrap reads the immutable initial reconciliation record, not current
// backing or spendable fee/stake balances. Subsequent deposits/fees change N.
func (k Keeper) Bootstrap(ctx sdk.Context) (BootstrapRecord, error) {
	if !k.IsConfigured() {
		return BootstrapRecord{}, ErrBootstrap
	}
	if _, err := k.getState(ctx); err != nil {
		return BootstrapRecord{}, err
	}
	return k.bootstrapRecord(ctx)
}

func (k Keeper) bootstrapRecord(ctx sdk.Context) (BootstrapRecord, error) {
	var r BootstrapRecord
	raw := k.store(ctx).Get(bootstrapKey)
	if len(raw) == 0 || len(raw) > 4096 || json.Unmarshal(raw, &r) != nil || r.Version != 1 || r.Digest == [32]byte{} ||
		!validTime(r.GenesisTime) || r.SourceHeight == 0 || r.SourceBlockHash == [32]byte{} ||
		r.Deposits == 0 || r.Deposits > MaxBootstrapDeposits || r.Validators == 0 || r.Validators > MaxBootstrapValidators ||
		r.Accounts == 0 || r.Accounts > MaxBootstrapDeposits || r.Validators > r.Accounts || r.Accounts > r.Deposits || !bootstrapAmount(r.ReleasedGod) {
		return BootstrapRecord{}, ErrBootstrap
	}
	return r, nil
}
