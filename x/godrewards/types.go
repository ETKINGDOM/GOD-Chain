// Package godrewards implements the prototype G ledger using God SDK storage.
// Keeper calls are trusted internal APIs, not authenticated transaction handlers.
package godrewards

import (
	"errors"
	"math/big"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	ModuleName    = "godrewards"
	StoreKey      = ModuleName
	GodDenom      = "agod"
	PendingModule = "god_pending_fees"
	PoolModule    = "god_reward_pool"
	ReserveModule = "god_bridge_reserve"
	Decimals      = 18
)

var (
	ErrInvalidAmount      = errors.New("amount must be positive and valid")
	ErrInvalidAccount     = errors.New("invalid account")
	ErrNotInitialized     = errors.New("reward keeper is not initialized")
	ErrAlreadyInitialized = errors.New("reward keeper is already initialized")
	ErrSupplyInvariant    = errors.New("GOD fixed-supply invariant violated")
	ErrGInvariant         = errors.New("G outstanding-supply invariant violated")
	ErrTimeRegression     = errors.New("consensus time regressed")
	ErrDuplicateBlock     = errors.New("reward block was already accounted for")
	ErrDailyCeiling       = errors.New("network daily G ceiling exceeded")
	ErrEmptyPool          = errors.New("no settled GOD is available for redemption")
	ErrInsufficientG      = errors.New("insufficient spendable G")
	ErrMinimumOutput      = errors.New("GOD output is below the requested minimum")
	ErrExpired            = errors.New("redemption deadline passed")
	ErrInvalidLock        = errors.New("invalid or already-used G lock")
	ErrUnauthorizedLock   = errors.New("G lock belongs to another account")
	ErrProtectedAccount   = errors.New("protocol account cannot act as a participant")
	ErrReentrant          = errors.New("redemption is already in progress")
)

// Constructors return fresh values: callers cannot mutate consensus constants.
func Unit() sdkmath.Int {
	return sdkmath.NewIntFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(Decimals), nil))
}
func FixedGodSupply() sdkmath.Int    { return Unit().MulRaw(1_000_000_000) }
func DailyGCeiling() sdkmath.Int     { return Unit().MulRaw(10_000) }
func MinimumRedemption() sdkmath.Int { return Unit() }

func positive(n sdkmath.Int) bool           { return !n.IsNil() && n.IsPositive() }
func validAccount(addr sdk.AccAddress) bool { return len(addr) == 20 }
func day(t time.Time) int64                 { return t.UTC().Unix() / 86400 }

// Quote uses a wide intermediate, not bounded Int.Mul, to avoid overflow before
// division. Output is bounded by P when 0 < g <= T.
func Quote(pool, offered, total sdkmath.Int) (sdkmath.Int, error) {
	if !positive(pool) || !positive(total) {
		return sdkmath.ZeroInt(), ErrEmptyPool
	}
	if !positive(offered) || offered.GT(total) {
		return sdkmath.ZeroInt(), ErrInvalidAmount
	}
	output := new(big.Int).Mul(pool.BigInt(), offered.BigInt())
	output.Quo(output, total.BigInt())
	if output.Sign() == 0 {
		return sdkmath.ZeroInt(), ErrEmptyPool
	}
	return sdkmath.NewIntFromBigInt(output), nil
}

// Allocation must be derived by a reviewed consensus/staking adapter. This
// prototype does not accept reward amounts from a public transaction or RPC.
type Allocation struct {
	Recipient sdk.AccAddress
	Amount    sdkmath.Int
}

type Redemption struct {
	Sender      sdk.AccAddress
	Beneficiary sdk.AccAddress
	G           sdkmath.Int
	MinGodOut   sdkmath.Int
	Deadline    time.Time
}

type Snapshot struct {
	PoolGod          sdkmath.Int `json:"poolGodSmallestUnits"`
	PendingGod       sdkmath.Int `json:"pendingGodSmallestUnits"`
	OutstandingG     sdkmath.Int `json:"outstandingGSmallestUnits"`
	PendingG         sdkmath.Int `json:"pendingGSmallestUnits"`
	DayIssuedG       sdkmath.Int `json:"dayIssuedGSmallestUnits"`
	ActiveDay        int64       `json:"activeUtcDay"`
	LastRewardHeight int64       `json:"lastRewardHeight"`
}
