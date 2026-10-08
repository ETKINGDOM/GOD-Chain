package godnode

import (
	"errors"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type UnbondingEntryView struct {
	ID             uint64    `json:"id,string"`
	CreationHeight int64     `json:"creationHeight,string"`
	CompletionTime time.Time `json:"completionTime"`
	InitialGod     string    `json:"initialBalanceGod"`
	BalanceGod     string    `json:"balanceGod"`
	HoldCount      int64     `json:"onHoldRefCount,string"`
}

type UnbondingView struct {
	Commit     CommittedView        `json:"commit"`
	Delegator  string               `json:"delegator"`
	Validator  string               `json:"validator"`
	Delay      int64                `json:"unbondingSeconds,string"`
	MaxEntries uint32               `json:"maxEntries"`
	Entries    []UnbondingEntryView `json:"entries"`
}

// QueryUnbonding reads one owner/operator pair from committed state only. It
// neither scans delegators nor completes exits. Empty results are not payment
// evidence; timestamps alone do not prove a returned, spendable bank balance.
func (a *App) QueryUnbonding(owner, validator string, height int64) (view UnbondingView, err error) {
	if a == nil {
		return view, ErrLifecycle
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	defer func() {
		if recover() != nil {
			view, err = UnbondingView{}, ErrQueryState
		}
	}()
	del, e := queryAccountAddress(owner)
	if e != nil {
		return view, e
	}
	val, e := sdk.ValAddressFromBech32(validator)
	if e != nil || len(val) != godaddress.AccountBytes || val.String() != validator {
		return view, ErrQueryAddress
	}
	ctx, commit, e := a.committedContext(height)
	if e != nil {
		return view, e
	}
	params, e := a.staking.GetParams(ctx)
	// These are existing genesis policy, not a new economic configuration.
	if e != nil || params.MaxEntries != 7 || params.UnbondingTime != 21*24*time.Hour {
		return view, ErrQueryState
	}
	native, e := godaddress.ToNative(del)
	if e != nil {
		return view, ErrQueryAddress
	}
	view = UnbondingView{Commit: commit, Delegator: native, Validator: validator, Delay: int64(params.UnbondingTime / time.Second), MaxEntries: params.MaxEntries, Entries: []UnbondingEntryView{}}
	ubd, e := a.staking.GetUnbondingDelegation(ctx, del, val)
	if errors.Is(e, stakingtypes.ErrNoUnbondingDelegation) {
		return view, nil
	}
	if e != nil || ubd.DelegatorAddress != native || ubd.ValidatorAddress != validator || len(ubd.Entries) > 7 {
		return UnbondingView{}, ErrQueryState
	}
	ids := map[uint64]bool{}
	for _, entry := range ubd.Entries {
		if entry.UnbondingId == 0 || ids[entry.UnbondingId] || entry.CreationHeight <= 0 || entry.CreationHeight > commit.Height || entry.CompletionTime.IsZero() || entry.UnbondingOnHoldRefCount < 0 || entry.InitialBalance.IsNil() || !entry.InitialBalance.IsPositive() || entry.InitialBalance.GT(godrewards.FixedGodSupply()) || entry.Balance.IsNil() || entry.Balance.IsNegative() || entry.Balance.GT(entry.InitialBalance) {
			return UnbondingView{}, ErrQueryState
		}
		ids[entry.UnbondingId] = true
		view.Entries = append(view.Entries, UnbondingEntryView{entry.UnbondingId, entry.CreationHeight, entry.CompletionTime.UTC(), entry.InitialBalance.String(), entry.Balance.String(), entry.UnbondingOnHoldRefCount})
	}
	return view, nil
}
