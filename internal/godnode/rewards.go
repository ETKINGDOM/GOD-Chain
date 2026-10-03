package godnode

import (
	"bytes"
	"encoding/json"
	"math/big"
	"sort"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	abci "github.com/cometbft/cometbft/abci/types"
	cmted "github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type signingValidator struct {
	Address []byte
	Power   int64
}
type signingSet struct {
	Height     int64
	Validators []signingValidator
}
type stakeWeight struct {
	Address []byte
	Shares  string
}
type eligibleStake struct {
	Consensus []byte
	Operator  []byte
	Weights   []stakeWeight
}
type contributionSnapshot struct {
	Height int64
	Time   time.Time
	Stake  []eligibleStake
}

func setKey(height int64) []byte { return append([]byte{2}, sdk.Uint64ToBigEndian(uint64(height))...) }

func (a *App) putSet(ctx sdk.Context, set signingSet) error {
	if set.Height <= 0 || len(set.Validators) == 0 || len(set.Validators) > 32 {
		return ErrBlock
	}
	sort.Slice(set.Validators, func(i, j int) bool { return bytes.Compare(set.Validators[i].Address, set.Validators[j].Address) < 0 })
	raw, err := json.Marshal(set)
	if err != nil {
		return err
	}
	ctx.KVStore(a.key).Set(setKey(set.Height), raw)
	return nil
}
func (a *App) getSet(ctx sdk.Context, height int64) (signingSet, error) {
	var set signingSet
	if json.Unmarshal(ctx.KVStore(a.key).Get(setKey(height)), &set) != nil || set.Height != height || len(set.Validators) == 0 || len(set.Validators) > 32 {
		return set, ErrBlock
	}
	return set, nil
}

func updatedSet(base signingSet, updates []abci.ValidatorUpdate, height int64) (signingSet, error) {
	set := signingSet{Height: height}
	powers := map[string]int64{}
	for _, v := range base.Validators {
		powers[string(v.Address)] = v.Power
	}
	seen := map[string]bool{}
	for _, u := range updates {
		pk := u.PubKey.GetEd25519()
		if len(pk) != 32 || u.Power < 0 {
			return set, ErrBlock
		}
		addr := cmted.PubKey(pk).Address()
		if seen[string(addr)] {
			return set, ErrBlock
		}
		seen[string(addr)] = true
		if u.Power == 0 {
			delete(powers, string(addr))
		} else {
			powers[string(addr)] = u.Power
		}
	}
	for addr, power := range powers {
		set.Validators = append(set.Validators, signingValidator{[]byte(addr), power})
	}
	return set, nil
}

func (a *App) initializeSigningSets(ctx sdk.Context, updates []abci.ValidatorUpdate) error {
	set, err := updatedSet(signingSet{}, updates, 1)
	if err != nil {
		return err
	}
	if err := a.putSet(ctx, set); err != nil {
		return err
	}
	set.Height = 2
	return a.putSet(ctx, set)
}

// The two-block validator update delay is explicit. LastCommit is validated
// against the set for the block actually signed, never the newly selected set.
func validateCommit(set signingSet, commit abci.CommitInfo) (map[string]bool, error) {
	if commit.Round < 0 || len(commit.Votes) != len(set.Validators) {
		return nil, ErrBlock
	}
	wanted := map[string]int64{}
	total, signed := new(big.Int), new(big.Int)
	for _, v := range set.Validators {
		if len(v.Address) != 20 || v.Power <= 0 || wanted[string(v.Address)] != 0 {
			return nil, ErrBlock
		}
		wanted[string(v.Address)] = v.Power
		total.Add(total, big.NewInt(v.Power))
	}
	signers := map[string]bool{}
	for _, vote := range commit.Votes {
		power, ok := wanted[string(vote.Validator.Address)]
		if !ok || power != vote.Validator.Power {
			return nil, ErrBlock
		}
		delete(wanted, string(vote.Validator.Address))
		switch vote.BlockIdFlag {
		case cmtproto.BlockIDFlagCommit:
			signers[string(vote.Validator.Address)] = true
			signed.Add(signed, big.NewInt(power))
		case cmtproto.BlockIDFlagAbsent, cmtproto.BlockIDFlagNil:
		default:
			return nil, ErrBlock
		}
	}
	if new(big.Int).Mul(signed, big.NewInt(3)).Cmp(new(big.Int).Mul(total, big.NewInt(2))) <= 0 {
		return nil, ErrBlock
	}
	return signers, nil
}

func (a *App) captureStake(ctx sdk.Context, height int64) error {
	set, err := a.getSet(ctx, height)
	if err != nil {
		return err
	}
	snapshot := contributionSnapshot{Height: height, Time: ctx.BlockTime().UTC()}
	for _, expected := range set.Validators {
		v, err := a.staking.GetValidatorByConsAddr(ctx, expected.Address)
		if err != nil {
			return err
		}
		if v.Jailed || !v.DelegatorShares.IsPositive() {
			continue
		}
		operator, err := validatorAddress(v.OperatorAddress)
		if err != nil {
			return err
		}
		self, err := a.staking.GetDelegation(ctx, sdk.AccAddress(operator), operator)
		if err != nil || v.TokensFromShares(self.Shares).TruncateInt().LT(minSelfStake()) {
			continue
		}
		delegations, err := a.staking.GetValidatorDelegations(ctx, operator)
		if err != nil || len(delegations) > 256 {
			return ErrBlock
		}
		eligible := eligibleStake{Consensus: expected.Address, Operator: operator}
		for _, d := range delegations {
			address, err := a.accounts.AddressCodec().StringToBytes(d.DelegatorAddress)
			if err != nil {
				return err
			}
			if !d.Shares.IsPositive() {
				return ErrBlock
			}
			eligible.Weights = append(eligible.Weights, stakeWeight{address, d.Shares.BigInt().String()})
		}
		sort.Slice(eligible.Weights, func(i, j int) bool {
			return bytes.Compare(eligible.Weights[i].Address, eligible.Weights[j].Address) < 0
		})
		snapshot.Stake = append(snapshot.Stake, eligible)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	ctx.KVStore(a.key).Set([]byte{3}, raw)
	return nil
}

func allocateContribution(budget sdkmath.Int, set signingSet, signers map[string]bool, snapshot contributionSnapshot) ([]godrewards.Allocation, error) {
	if budget.IsNil() || !budget.IsPositive() || budget.GT(godrewards.DailyGCeiling()) || len(set.Validators) == 0 || len(set.Validators) > 32 ||
		set.Height <= 0 || snapshot.Height != set.Height || len(snapshot.Stake) > len(set.Validators) {
		return nil, ErrBlock
	}
	totalPower := new(big.Int)
	powers := map[string]int64{}
	for _, v := range set.Validators {
		if len(v.Address) != 20 || v.Power <= 0 || powers[string(v.Address)] != 0 {
			return nil, ErrBlock
		}
		powers[string(v.Address)] = v.Power
		totalPower.Add(totalPower, big.NewInt(v.Power))
	}
	amounts := map[string]*big.Int{}
	add := func(addr []byte, n *big.Int) {
		if n.Sign() > 0 {
			if amounts[string(addr)] == nil {
				amounts[string(addr)] = new(big.Int)
			}
			amounts[string(addr)].Add(amounts[string(addr)], n)
		}
	}
	seen := map[string]bool{}
	for _, v := range snapshot.Stake {
		if seen[string(v.Consensus)] || powers[string(v.Consensus)] <= 0 || len(v.Operator) != 20 {
			return nil, ErrBlock
		}
		seen[string(v.Consensus)] = true
		if !signers[string(v.Consensus)] {
			continue
		}
		validatorBudget := new(big.Int).Mul(budget.BigInt(), big.NewInt(powers[string(v.Consensus)]))
		validatorBudget.Quo(validatorBudget, totalPower)
		operatorBudget := new(big.Int).Quo(new(big.Int).Set(validatorBudget), big.NewInt(10))
		delegatorBudget := new(big.Int).Sub(validatorBudget, operatorBudget)
		totalShares := new(big.Int)
		weights := make([]*big.Int, len(v.Weights))
		seenDelegators := map[string]bool{}
		for i, w := range v.Weights {
			n, ok := new(big.Int).SetString(w.Shares, 10)
			if !ok || n.Sign() <= 0 || n.BitLen() > 256 || len(w.Address) != 20 || seenDelegators[string(w.Address)] {
				return nil, ErrBlock
			}
			seenDelegators[string(w.Address)] = true
			weights[i] = n
			totalShares.Add(totalShares, n)
		}
		if totalShares.Sign() == 0 {
			return nil, ErrBlock
		}
		add(v.Operator, operatorBudget)
		for i, w := range v.Weights {
			n := new(big.Int).Mul(delegatorBudget, weights[i])
			n.Quo(n, totalShares)
			add(w.Address, n)
		}
	}
	addresses := make([]string, 0, len(amounts))
	for addr := range amounts {
		addresses = append(addresses, addr)
	}
	sort.Strings(addresses)
	allocations := make([]godrewards.Allocation, 0, len(addresses))
	for _, addr := range addresses {
		allocations = append(allocations, godrewards.Allocation{Recipient: []byte(addr), Amount: sdkmath.NewIntFromBigInt(amounts[addr])})
	}
	return allocations, nil
}

func (a *App) rewardCommit(ctx sdk.Context, req *abci.RequestFinalizeBlock) error {
	if req.Height == 1 {
		if len(req.DecidedLastCommit.Votes) != 0 {
			return ErrBlock
		}
		return nil
	}
	set, err := a.getSet(ctx, req.Height-1)
	if err != nil {
		return err
	}
	signers, err := validateCommit(set, req.DecidedLastCommit)
	if err != nil {
		return err
	}
	var snapshot contributionSnapshot
	if json.Unmarshal(ctx.KVStore(a.key).Get([]byte{3}), &snapshot) != nil || snapshot.Height != req.Height-1 {
		return ErrBlock
	}
	// A halted or old UTC day never gets catch-up emission in the new day.
	if snapshot.Time.UTC().Unix()/86400 != req.Time.UTC().Unix()/86400 {
		return nil
	}
	state, err := a.rewards.Snapshot(ctx)
	if err != nil {
		return err
	}
	budget := a.config.GPerSignedBlock
	if remaining := godrewards.DailyGCeiling().Sub(state.DayIssuedG); budget.GT(remaining) {
		budget = remaining
	}
	if budget.IsZero() {
		return nil
	}
	allocations, err := allocateContribution(budget, set, signers, snapshot)
	if err != nil {
		return err
	}
	return a.rewards.AccrueBlock(ctx, allocations)
}

func (a *App) preBlock(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
	m, err := a.metadata(ctx)
	if err != nil {
		return nil, err
	}
	if req.Height != m.Height+1 || req.Time.Before(m.Time) || !validTime(req.Time) {
		return nil, ErrBlock
	}
	if _, err := a.bridgeSnapshot(ctx); err != nil {
		return nil, err
	}
	cache, write := ctx.CacheContext()
	if err := a.rewards.BeginDay(cache); err != nil {
		return nil, err
	}
	if err := a.rewardCommit(cache, req); err != nil {
		return nil, err
	}
	if err := a.applyPenalties(cache, req); err != nil {
		return nil, err
	}
	if err := a.captureStake(cache, req.Height); err != nil {
		return nil, err
	}
	if req.Height > 2 {
		cache.KVStore(a.key).Delete(setKey(req.Height - 2))
	}
	m.Height, m.Time = req.Height, req.Time.UTC()
	if err := a.putMetadata(cache, m); err != nil {
		return nil, err
	}
	write()
	return &sdk.ResponsePreBlock{}, nil
}

func (a *App) endBlock(ctx sdk.Context) (sdk.EndBlock, error) {
	if err := a.evm.EndBlock(ctx); err != nil {
		return sdk.EndBlock{}, err
	}
	if err := a.fees.EndBlock(ctx); err != nil {
		return sdk.EndBlock{}, err
	}
	if err := a.rewards.CollectGasFees(ctx); err != nil {
		return sdk.EndBlock{}, err
	}
	updates, err := a.staking.EndBlocker(ctx)
	if err != nil {
		return sdk.EndBlock{}, err
	}
	previous, err := a.getSet(ctx, ctx.BlockHeight()+1)
	if err != nil {
		return sdk.EndBlock{}, err
	}
	set, err := updatedSet(previous, updates, ctx.BlockHeight()+2)
	if err != nil {
		return sdk.EndBlock{}, err
	}
	if err := a.putSet(ctx, set); err != nil {
		return sdk.EndBlock{}, err
	}
	if !a.bank.GetSupply(ctx, godrewards.GodDenom).Amount.Equal(godrewards.FixedGodSupply()) {
		return sdk.EndBlock{}, ErrSupply
	}
	if _, err := a.bridgeSnapshot(ctx); err != nil {
		return sdk.EndBlock{}, err
	}
	return sdk.EndBlock{ValidatorUpdates: updates}, nil
}
