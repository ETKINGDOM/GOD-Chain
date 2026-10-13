package godnode

import (
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ethereum/go-ethereum/common"
)

type StakingPositionView struct {
	Delegation DelegationView `json:"delegation"`
	Unbonding  UnbondingView  `json:"unbonding"`
}
type StakingSnapshotView struct {
	Network         NetworkView           `json:"network"`
	Account         AccountView           `json:"account"`
	Page            ValidatorsView        `json:"page"`
	Positions       []StakingPositionView `json:"positions"`
	ContractAccount bool                  `json:"contractAccount"`
}

// QueryStakingSnapshot holds the same application read lock throughout one
// bounded latest-committed display. FinalizeBlock/Commit cannot split these
// account, directory, delegation and exit reads across two versions. No bank,
// rewards, settlement, staking, signing progress or node setting is written.
// This operator view is not a light-client proof or transaction authorization.
func (a *App) QueryStakingSnapshot(owner string, operators []string) (view StakingSnapshotView, err error) {
	if a == nil {
		return view, ErrLifecycle
	}
	if len(operators) < 1 || len(operators) > ValidatorPageLimit {
		return view, ErrQueryAddress
	}
	raw, e := godaddress.FromNative(owner)
	if e != nil {
		return view, ErrQueryAddress
	}
	canonical, e := godaddress.ToNative(raw)
	if e != nil || canonical != owner {
		return view, ErrQueryAddress
	}
	seen := map[string]bool{}
	for _, operator := range operators {
		if _, e := queryValidatorAddress(operator); e != nil || seen[operator] {
			return view, ErrQueryAddress
		}
		seen[operator] = true
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	defer func() {
		if recover() != nil {
			view, err = StakingSnapshotView{}, ErrQueryState
		}
	}()
	n, e := a.queryNetwork(0)
	if e != nil {
		return view, e
	}
	account, e := a.queryAccount(owner, n.Commit.Height)
	if e != nil {
		return view, e
	}
	page, e := a.queryValidators("", n.Commit.Height)
	if e != nil {
		return view, e
	}
	positions := make([]StakingPositionView, len(operators))
	for i, operator := range operators {
		d, e := a.queryDelegation(owner, operator, n.Commit.Height)
		if e != nil {
			return view, e
		}
		u, e := a.queryUnbonding(owner, operator, n.Commit.Height)
		if e != nil {
			return view, e
		}
		if d.Commit != n.Commit || u.Commit != n.Commit {
			return view, ErrQueryState
		}
		positions[i] = StakingPositionView{d, u}
	}
	ctx, c, e := a.committedContext(n.Commit.Height)
	if e != nil || c != n.Commit || account.Commit != n.Commit || page.Commit != n.Commit {
		return view, ErrQueryState
	}
	address := common.BytesToAddress(raw)
	contract := len(a.evm.GetCode(ctx, a.evm.GetCodeHash(ctx, address))) != 0
	return StakingSnapshotView{n, account, page, positions, contract}, nil
}
