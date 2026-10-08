package godnode

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	storetypes "cosmossdk.io/store/types"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var (
	ErrQueryHeight  = errors.New("only the latest committed query height is supported")
	ErrQueryAddress = errors.New("query account address rejected")
	ErrQueryState   = errors.New("committed query state unavailable")
)

// CommittedView identifies the application version read, not an authenticated
// consensus or bridge proof. JSON integers used by signing clients are strings
// to avoid losing precision in JavaScript. Results contain no mutable SDK data.
type CommittedView struct {
	Height     int64     `json:"height,string"`
	Time       time.Time `json:"time"`
	AppHash    string    `json:"appHash"`
	Synthetic  bool      `json:"synthetic"`
	RealAssets bool      `json:"realAssets"`
}

type NetworkView struct {
	Commit           CommittedView `json:"commit"`
	ChainID          string        `json:"chainId"`
	EVMChainID       uint64        `json:"evmChainId,string"`
	NativeDenom      string        `json:"nativeDenom"`
	Decimals         uint8         `json:"decimals"`
	TotalGod         string        `json:"totalGod"`
	RewardPoolGod    string        `json:"rewardPoolGod"`
	PendingRewardGod string        `json:"pendingRewardGod"`
	OutstandingG     string        `json:"outstandingG"`
	PendingG         string        `json:"pendingG"`
}

// AccountView separates the three queried G buckets. PendingEarnedG is not
// claimable, and none of these fields purports to include application locks.
// BalanceGod is the bank balance, not a spendability or transaction simulation.
// Exists refers to the auth account; a query never creates an absent account.
type AccountView struct {
	Commit         CommittedView `json:"commit"`
	NativeAddress  string        `json:"nativeAddress"`
	EVMAddress     string        `json:"evmAddress"`
	Exists         bool          `json:"exists"`
	ModuleAccount  bool          `json:"moduleAccount"`
	AccountNumber  uint64        `json:"accountNumber,string"`
	Sequence       uint64        `json:"sequence,string"`
	BalanceGod     string        `json:"balanceGod"`
	SpendableG     string        `json:"spendableG"`
	UnclaimedG     string        `json:"unclaimedG"`
	PendingEarnedG string        `json:"pendingEarnedG"`
}

type DelegationView struct {
	Commit        CommittedView `json:"commit"`
	Delegator     string        `json:"delegator"`
	Validator     string        `json:"validator"`
	Exists        bool          `json:"exists"`
	Shares        string        `json:"shares"`
	GodEquivalent string        `json:"godEquivalentSmallestUnits"`
}

// ValidatorView is one detached staking record, not a consensus-membership,
// uptime, operator-ownership or geography proof. Bonded status alone does not
// establish signing in a particular block. No arbitrary description is exposed.
type ValidatorView struct {
	Commit            CommittedView `json:"commit"`
	Validator         string        `json:"validator"`
	Exists            bool          `json:"exists"`
	Status            string        `json:"status"`
	Jailed            bool          `json:"jailed"`
	Tombstoned        bool          `json:"tombstoned"`
	TokensGod         string        `json:"tokensGodSmallestUnits"`
	DelegatorShares   string        `json:"delegatorShares"`
	CommissionRate    string        `json:"commissionRate"`
	MinSelfDelegation string        `json:"minSelfDelegationGodSmallestUnits"`
}

func queryValidatorAddress(text string) (sdk.ValAddress, error) {
	// Reject oversized input before codec work, keeper access or allocation.
	const length = len(godaddress.ValidatorOperatorPrefix) + 1 + godaddress.AccountBytes*8/5 + 6
	if len(text) != length {
		return nil, ErrQueryAddress
	}
	raw, err := validatorAddress(text)
	if err != nil {
		return nil, ErrQueryAddress
	}
	return sdk.ValAddress(raw), nil
}

// QueryValidator reads exactly one canonical operator at the latest committed
// version. It scans no validator/delegator collections and mutates no stake,
// signing progress, reward or sequence. Unknown operators are explicit absence.
func (a *App) QueryValidator(operator string, height int64) (ValidatorView, error) {
	if a == nil {
		return ValidatorView{}, ErrLifecycle
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.queryValidator(operator, height)
}

func (a *App) queryValidator(operator string, height int64) (view ValidatorView, err error) {
	defer func() {
		if recover() != nil {
			view, err = ValidatorView{}, ErrQueryState
		}
	}()
	address, e := queryValidatorAddress(operator)
	if e != nil {
		return view, e
	}
	ctx, commit, e := a.committedContext(height)
	if e != nil {
		return view, e
	}
	v, e := a.staking.GetValidator(ctx, address)
	if errors.Is(e, stakingtypes.ErrNoValidatorFound) {
		return ValidatorView{Commit: commit, Validator: operator, Status: "absent", TokensGod: "0", DelegatorShares: "0", CommissionRate: "0", MinSelfDelegation: "0"}, nil
	}
	if e != nil {
		return ValidatorView{}, ErrQueryState
	}
	return a.validatorView(ctx, commit, operator, v)
}

func (a *App) validatorView(ctx sdk.Context, commit CommittedView, operator string, v stakingtypes.Validator) (ValidatorView, error) {
	if v.OperatorAddress != operator || v.Tokens.IsNil() || v.Tokens.IsNegative() || v.Tokens.GT(godrewards.FixedGodSupply()) ||
		v.DelegatorShares.IsNil() || v.DelegatorShares.IsNegative() || v.DelegatorShares.BigInt().BitLen() > 256 ||
		v.Commission.Rate.IsNil() || !v.Commission.Rate.Equal(commission()) || v.MinSelfDelegation.IsNil() ||
		!v.MinSelfDelegation.Equal(minSelfStake()) {
		return ValidatorView{}, ErrQueryState
	}
	var status string
	switch v.Status {
	case stakingtypes.Bonded:
		status = "bonded"
	case stakingtypes.Unbonding:
		status = "unbonding"
	case stakingtypes.Unbonded:
		status = "unbonded"
	default:
		return ValidatorView{}, ErrQueryState
	}
	address, e := queryValidatorAddress(operator)
	if e != nil {
		return ValidatorView{}, ErrQueryState
	}
	consensus, e := v.GetConsAddr()
	if e != nil || len(consensus) != godaddress.AccountBytes {
		return ValidatorView{}, ErrQueryState
	}
	info, e := a.slashing.GetValidatorSigningInfo(ctx, consensus)
	if e != nil {
		return ValidatorView{}, ErrQueryState
	}
	return ValidatorView{Commit: commit, Validator: operator, Exists: true, Status: status,
		Jailed: v.Jailed, Tombstoned: info.Tombstoned || ctx.KVStore(a.key).Has(penaltyKey(6, address)),
		TokensGod: v.Tokens.String(), DelegatorShares: v.DelegatorShares.String(),
		CommissionRate: v.Commission.Rate.String(), MinSelfDelegation: v.MinSelfDelegation.String()}, nil
}

const ValidatorPageLimit = 8

// ValidatorsView enumerates registration records in raw operator-key order,
// not by stake, eligibility or consensus power. NextAfter is an exclusive
// cursor, valid only at this exact latest commit. An advanced tip requires a
// new first page; clients must never combine pages from different commits.
// No total-count scan, descriptions, contacts or consensus keys are exposed.
type ValidatorsView struct {
	Commit     CommittedView   `json:"commit"`
	After      string          `json:"after"`
	Limit      int             `json:"limit"`
	NextAfter  string          `json:"nextAfter"`
	Validators []ValidatorView `json:"validators"`
}

func (a *App) QueryValidators(after string, height int64) (ValidatorsView, error) {
	if a == nil {
		return ValidatorsView{}, ErrLifecycle
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.queryValidators(after, height)
}

func (a *App) queryValidators(after string, height int64) (view ValidatorsView, err error) {
	defer func() {
		if recover() != nil {
			view, err = ValidatorsView{}, ErrQueryState
		}
	}()
	start := append([]byte(nil), stakingtypes.ValidatorsKey...)
	if after != "" {
		address, e := queryValidatorAddress(after)
		if e != nil {
			return ValidatorsView{}, e
		}
		if height <= 0 {
			return ValidatorsView{}, ErrQueryHeight
		}
		// Exact keys have fixed length. Appending zero seeks strictly past the
		// cursor without walking earlier entries or relying on its existence.
		start = append(stakingtypes.GetValidatorKey(address), 0)
	}
	ctx, commit, e := a.committedContext(height)
	if e != nil {
		return ValidatorsView{}, e
	}
	view = ValidatorsView{Commit: commit, After: after, Limit: ValidatorPageLimit, Validators: make([]ValidatorView, 0, ValidatorPageLimit)}
	iterator := ctx.KVStore(a.stakingKey).Iterator(start, storetypes.PrefixEndBytes(stakingtypes.ValidatorsKey))
	defer func() {
		if iterator.Close() != nil {
			view, err = ValidatorsView{}, ErrQueryState
		}
	}()
	for ; iterator.Valid(); iterator.Next() {
		key := iterator.Key()
		if len(key) != 2+godaddress.AccountBytes || key[0] != stakingtypes.ValidatorsKey[0] || key[1] != godaddress.AccountBytes {
			return ValidatorsView{}, ErrQueryState
		}
		address := sdk.ValAddress(append([]byte(nil), key[2:]...))
		operator := address.String()
		if _, e := queryValidatorAddress(operator); e != nil || !bytes.Equal(key, stakingtypes.GetValidatorKey(address)) {
			return ValidatorsView{}, ErrQueryState
		}
		// At most eight records and one look-ahead key are read per request.
		if len(view.Validators) == ValidatorPageLimit {
			view.NextAfter = view.Validators[len(view.Validators)-1].Validator
			break
		}
		v, e := a.staking.GetValidator(ctx, address)
		if e != nil {
			return ValidatorsView{}, ErrQueryState
		}
		record, e := a.validatorView(ctx, commit, operator, v)
		if e != nil {
			return ValidatorsView{}, e
		}
		view.Validators = append(view.Validators, record)
	}
	// The pinned SDK cache-merge iterator reports Error at normal exhaustion.
	// Valid controls EOF; reads/panics and Close failures still fail closed.
	return view, nil
}

// QueryDelegation is one bounded committed lookup. GOD equivalent is truncated
// from shares at that height, not spendable funds or an unbonding completion.
func (a *App) QueryDelegation(owner, validator string, height int64) (view DelegationView, err error) {
	if a == nil {
		return view, ErrLifecycle
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	defer func() {
		if recover() != nil {
			view, err = DelegationView{}, ErrQueryState
		}
	}()
	delegator, e := queryAccountAddress(owner)
	if e != nil {
		return view, e
	}
	val, e := sdk.ValAddressFromBech32(validator)
	if e != nil || val.String() != validator {
		return view, ErrQueryAddress
	}
	ctx, commit, e := a.committedContext(height)
	if e != nil {
		return view, e
	}
	native, e := godaddress.ToNative(delegator)
	if e != nil {
		return view, ErrQueryAddress
	}
	view = DelegationView{Commit: commit, Delegator: native, Validator: validator, Shares: "0", GodEquivalent: "0"}
	d, e := a.staking.GetDelegation(ctx, delegator, val)
	if errors.Is(e, stakingtypes.ErrNoDelegation) {
		return view, nil
	}
	if e != nil {
		return DelegationView{}, ErrQueryState
	}
	v, e := a.staking.GetValidator(ctx, val)
	if e != nil {
		return DelegationView{}, ErrQueryState
	}
	view.Exists = true
	view.Shares = d.Shares.String()
	view.GodEquivalent = v.TokensFromShares(d.Shares).TruncateInt().String()
	return view, nil
}

// committedContext must run under a.mu. Explicit versions avoid CheckTx and
// FinalizeBlock's working stores, including while the next commit is pending.
// Timestamp and chain identity come from the bound committed metadata, never a
// wall clock or the SDK's possibly newer check/finalize header.
func (a *App) committedContext(height int64) (sdk.Context, CommittedView, error) {
	if !a.usable() || !a.initialized || a.base.LastBlockHeight() <= 0 {
		return sdk.Context{}, CommittedView{}, ErrLifecycle
	}
	latest := a.base.LastBlockHeight()
	if height < 0 || height != 0 && height != latest {
		return sdk.Context{}, CommittedView{}, ErrQueryHeight
	}
	id := a.base.LastCommitID()
	if id.Version != latest || len(id.Hash) != 32 {
		return sdk.Context{}, CommittedView{}, ErrQueryState
	}
	ctx, err := a.base.CreateQueryContext(latest, false)
	if err != nil {
		return sdk.Context{}, CommittedView{}, ErrQueryState
	}
	m, err := a.metadata(ctx)
	if err != nil || m.Height != latest {
		return sdk.Context{}, CommittedView{}, ErrQueryState
	}
	ctx = ctx.WithChainID(a.config.ChainID).WithBlockHeight(latest).WithBlockTime(m.Time.UTC())
	return ctx, CommittedView{Height: latest, Time: m.Time.UTC(), AppHash: hex.EncodeToString(id.Hash), Synthetic: true, RealAssets: false}, nil
}

// QueryNetwork reads the latest committed version only. Zero selects latest;
// an exact current height can pin related requests, rejecting a changed tip.
// No listener, proof, transaction submission or activation is supplied here.
func (a *App) QueryNetwork(height int64) (NetworkView, error) {
	if a == nil {
		return NetworkView{}, ErrLifecycle
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.queryNetwork(height)
}

func (a *App) queryNetwork(height int64) (view NetworkView, err error) {
	// Corrupt storage and query-gas failures must not expose panic payloads or
	// escape as a partial result. A read failure does not write or retry state.
	defer func() {
		if recover() != nil {
			view, err = NetworkView{}, ErrQueryState
		}
	}()
	ctx, commit, err := a.committedContext(height)
	if err != nil {
		return NetworkView{}, err
	}
	rewards, err := a.rewards.Snapshot(ctx)
	if err != nil {
		return NetworkView{}, ErrQueryState
	}
	return NetworkView{Commit: commit, ChainID: a.config.ChainID, EVMChainID: a.config.EVMChainID,
		NativeDenom: godrewards.GodDenom, Decimals: godrewards.Decimals,
		TotalGod:      a.bank.GetSupply(ctx, godrewards.GodDenom).Amount.String(),
		RewardPoolGod: rewards.PoolGod.String(), PendingRewardGod: rewards.PendingGod.String(),
		OutstandingG: rewards.OutstandingG.String(), PendingG: rewards.PendingG.String()}, nil
}

func queryAccountAddress(text string) (sdk.AccAddress, error) {
	var raw []byte
	var err error
	if strings.HasPrefix(text, "0x") {
		raw, err = godaddress.FromEVM(text)
	} else {
		raw, err = godaddress.FromNative(text)
	}
	if err != nil {
		return nil, ErrQueryAddress
	}
	return sdk.AccAddress(raw), nil
}

// QueryAccount accepts either reviewed encoding of the same 20-byte account.
// It returns no signing keys, contract storage or faith content.
func (a *App) QueryAccount(text string, height int64) (AccountView, error) {
	if a == nil {
		return AccountView{}, ErrLifecycle
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.queryAccount(text, height)
}

func (a *App) queryAccount(text string, height int64) (view AccountView, err error) {
	defer func() {
		if recover() != nil {
			view, err = AccountView{}, ErrQueryState
		}
	}()
	owner, err := queryAccountAddress(text)
	if err != nil {
		return AccountView{}, err
	}
	ctx, commit, err := a.committedContext(height)
	if err != nil {
		return AccountView{}, err
	}
	// Check the reward module's initialized, fixed-supply and time guards
	// without invoking day settlement or scanning account/lock collections.
	if _, err := a.rewards.Snapshot(ctx); err != nil {
		return AccountView{}, ErrQueryState
	}
	native, err := godaddress.ToNative(owner)
	if err != nil {
		return AccountView{}, ErrQueryState
	}
	evm, err := godaddress.ToEVM(owner)
	if err != nil {
		return AccountView{}, ErrQueryState
	}
	view = AccountView{Commit: commit, NativeAddress: native, EVMAddress: evm,
		BalanceGod: a.bank.GetBalance(ctx, owner, godrewards.GodDenom).Amount.String(),
		SpendableG: a.rewards.SpendableG(ctx, owner).String(), UnclaimedG: a.rewards.UnclaimedG(ctx, owner).String(),
		PendingEarnedG: a.rewards.PendingEarnedG(ctx, owner).String()}
	if account := a.accounts.GetAccount(ctx, owner); account != nil {
		view.Exists = true
		_, view.ModuleAccount = account.(sdk.ModuleAccountI)
		view.AccountNumber, view.Sequence = account.GetAccountNumber(), account.GetSequence()
	}
	return view, nil
}
