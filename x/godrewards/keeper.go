package godrewards

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// Bank is implemented by the existing God SDK bank keeper. This module has no
// GOD mint/burn permission and never substitutes staking or reserve balances
// for reward liquidity.
type Bank interface {
	GetBalance(context.Context, sdk.AccAddress, string) sdk.Coin
	GetSupply(context.Context, string) sdk.Coin
	SendCoinsFromAccountToModule(context.Context, sdk.AccAddress, string, sdk.Coins) error
	SendCoinsFromModuleToAccount(context.Context, string, sdk.AccAddress, sdk.Coins) error
	SendCoinsFromModuleToModule(context.Context, string, string, sdk.Coins) error
}

type ModuleAccounts interface {
	GetModuleAddress(string) sdk.AccAddress
	GetAccount(context.Context, sdk.AccAddress) sdk.AccountI
}

type Keeper struct {
	key      *storetypes.KVStoreKey
	bank     Bank
	accounts ModuleAccounts
}

var (
	initializedKey  = []byte{1}
	dayKey          = []byte{2}
	lastTimeKey     = []byte{3}
	lastHeightKey   = []byte{4}
	dailyKey        = []byte{5}
	outstandingKey  = []byte{6}
	pendingTotalKey = []byte{7}
	redeemingKey    = []byte{8}
	spendablePrefix = []byte{20}
	unclaimedPrefix = []byte{21}
	earnedPrefix    = []byte{22}
	lockPrefix      = []byte{23}
	usedLockPrefix  = []byte{24}
)

func NewKeeper(key *storetypes.KVStoreKey, bank Bank, accounts ModuleAccounts) (Keeper, error) {
	if key == nil || bank == nil || accounts == nil {
		return Keeper{}, fmt.Errorf("keeper dependencies are required")
	}
	seen := map[string]bool{}
	for _, name := range []string{PendingModule, PoolModule, ReserveModule, authtypes.FeeCollectorName} {
		addr := accounts.GetModuleAddress(name)
		if !validAccount(addr) || seen[string(addr)] {
			return Keeper{}, fmt.Errorf("required module account is not registered: %s", name)
		}
		seen[string(addr)] = true
	}
	return Keeper{key: key, bank: bank, accounts: accounts}, nil
}

func (k Keeper) store(ctx sdk.Context) storetypes.KVStore   { return ctx.KVStore(k.key) }
func (k Keeper) sub(ctx sdk.Context, p []byte) prefix.Store { return prefix.NewStore(k.store(ctx), p) }

func getInt(store storetypes.KVStore, key []byte) sdkmath.Int {
	bytes := store.Get(key)
	if bytes == nil {
		return sdkmath.ZeroInt()
	}
	n, ok := sdkmath.NewIntFromString(string(bytes))
	if !ok || n.IsNegative() {
		panic("corrupt nonnegative reward amount")
	}
	return n
}

func putInt(store storetypes.KVStore, key []byte, n sdkmath.Int) {
	if n.IsNil() || n.IsNegative() {
		panic("invalid reward amount")
	}
	if n.IsZero() {
		store.Delete(key)
		return
	}
	store.Set(key, []byte(n.String()))
}

func getNumber(store storetypes.KVStore, key []byte) int64 {
	bytes := store.Get(key)
	if bytes == nil {
		return 0
	}
	if len(bytes) != 8 {
		panic("corrupt reward metadata")
	}
	return int64(binary.BigEndian.Uint64(bytes))
}

func putNumber(store storetypes.KVStore, key []byte, n int64) {
	if n < 0 {
		panic("negative reward metadata")
	}
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uint64(n))
	store.Set(key, bytes)
}

func checkedAdd(a, b sdkmath.Int) (sdkmath.Int, error) {
	n := new(big.Int).Add(a.BigInt(), b.BigInt())
	if n.Sign() < 0 || n.BitLen() > 256 {
		return sdkmath.ZeroInt(), ErrGInvariant
	}
	return sdkmath.NewIntFromBigInt(n), nil
}

func (k Keeper) checkSupply(ctx sdk.Context) error {
	if !k.bank.GetSupply(ctx, GodDenom).Amount.Equal(FixedGodSupply()) {
		return ErrSupplyInvariant
	}
	return nil
}

func (k Keeper) protected(ctx sdk.Context, addr sdk.AccAddress) bool {
	for _, name := range []string{PendingModule, PoolModule, ReserveModule, authtypes.FeeCollectorName} {
		if addr.Equals(k.accounts.GetModuleAddress(name)) {
			return true
		}
	}
	_, module := k.accounts.GetAccount(ctx, addr).(sdk.ModuleAccountI)
	return module
}

func previousTime(store storetypes.KVStore) time.Time {
	bytes := store.Get(lastTimeKey)
	if bytes == nil {
		return time.Time{}
	}
	value, err := time.Parse(time.RFC3339Nano, string(bytes))
	if err != nil {
		panic("corrupt reward consensus time")
	}
	return value
}

func putTime(store storetypes.KVStore, value time.Time) {
	store.Set(lastTimeKey, []byte(value.UTC().Format(time.RFC3339Nano)))
}

func (k Keeper) ready(ctx sdk.Context) error {
	if !k.store(ctx).Has(initializedKey) {
		return ErrNotInitialized
	}
	// Guard every mutating entry point, not just Redeem: an adapter callback
	// must not transfer or unlock the balance while a payout is in progress.
	if k.store(ctx).Has(redeemingKey) {
		return ErrReentrant
	}
	if ctx.BlockTime().Unix() <= 0 || ctx.BlockTime().Before(previousTime(k.store(ctx))) {
		return ErrTimeRegression
	}
	return k.checkSupply(ctx)
}

// Init only initializes G bookkeeping against an already initialized fixed
// bank supply. It creates no GOD and releases no bridge reserve. Real backing
// verification is deliberately outside this prototype and remains disabled.
func (k Keeper) Init(ctx sdk.Context) error {
	if k.store(ctx).Has(initializedKey) {
		return ErrAlreadyInitialized
	}
	iterator := k.store(ctx).Iterator(nil, nil)
	nonempty := iterator.Valid()
	iterator.Close()
	if nonempty {
		return ErrGInvariant
	}
	if ctx.BlockTime().Unix() <= 0 {
		return ErrTimeRegression
	}
	if err := k.checkSupply(ctx); err != nil {
		return err
	}
	if !k.bank.GetBalance(ctx, k.accounts.GetModuleAddress(PoolModule), GodDenom).IsZero() ||
		!k.bank.GetBalance(ctx, k.accounts.GetModuleAddress(PendingModule), GodDenom).IsZero() {
		return fmt.Errorf("initial reward liquidity must be zero")
	}
	cache, write := ctx.CacheContext()
	k.store(cache).Set(initializedKey, []byte{1})
	putNumber(k.store(cache), dayKey, day(ctx.BlockTime()))
	putTime(k.store(cache), ctx.BlockTime())
	write()
	return nil
}

// BeginDay settles the previously earned period once before new-period work.
// Skipped days do not earn G. Both pending GOD and G move in one cache commit.
func (k Keeper) BeginDay(ctx sdk.Context) error {
	if err := k.ready(ctx); err != nil {
		return err
	}
	cache, write := ctx.CacheContext()
	store := k.store(cache)
	current := day(ctx.BlockTime())
	active := getNumber(store, dayKey)
	if current < active {
		return ErrTimeRegression
	}
	if current > active {
		pending := k.bank.GetBalance(cache, k.accounts.GetModuleAddress(PendingModule), GodDenom)
		if pending.IsPositive() {
			if err := k.bank.SendCoinsFromModuleToModule(cache, PendingModule, PoolModule, sdk.NewCoins(pending)); err != nil {
				return err
			}
		}
		earned := k.sub(cache, earnedPrefix)
		unclaimed := k.sub(cache, unclaimedPrefix)
		iterator := earned.Iterator(nil, nil)
		var keys [][]byte
		sum := sdkmath.ZeroInt()
		for ; iterator.Valid(); iterator.Next() {
			key := append([]byte(nil), iterator.Key()...)
			amount := getInt(earned, key)
			updated, err := checkedAdd(getInt(unclaimed, key), amount)
			if err != nil {
				iterator.Close()
				return err
			}
			putInt(unclaimed, key, updated)
			sum, err = checkedAdd(sum, amount)
			if err != nil {
				iterator.Close()
				return err
			}
			keys = append(keys, key)
		}
		iterator.Close()
		if !sum.Equal(getInt(store, pendingTotalKey)) {
			return ErrGInvariant
		}
		updated, err := checkedAdd(getInt(store, outstandingKey), sum)
		if err != nil {
			return err
		}
		putInt(store, outstandingKey, updated)
		for _, key := range keys {
			earned.Delete(key)
		}
		putInt(store, pendingTotalKey, sdkmath.ZeroInt())
		putInt(store, dailyKey, sdkmath.ZeroInt())
		putNumber(store, dayKey, current)
	}
	putTime(store, ctx.BlockTime())
	if err := k.checkSupply(cache); err != nil {
		return err
	}
	write()
	return nil
}

// AccrueBlock is a trusted consensus-adapter API, not a public mint endpoint.
// The stake/signature-to-allocation algorithm is not selected or wired here.
// A caller cannot reuse an accounted height or exceed the daily network cap.
func (k Keeper) AccrueBlock(ctx sdk.Context, allocations []Allocation) error {
	if err := k.ready(ctx); err != nil {
		return err
	}
	cache, write := ctx.CacheContext()
	if err := k.BeginDay(cache); err != nil {
		return err
	}
	store := k.store(cache)
	if ctx.BlockHeight() <= 0 || ctx.BlockHeight() <= getNumber(store, lastHeightKey) {
		return ErrDuplicateBlock
	}
	total := sdkmath.ZeroInt()
	for _, allocation := range allocations {
		if !validAccount(allocation.Recipient) {
			return ErrInvalidAccount
		}
		if k.protected(ctx, allocation.Recipient) {
			return ErrProtectedAccount
		}
		if !positive(allocation.Amount) {
			return ErrInvalidAmount
		}
		if allocation.Amount.GT(DailyGCeiling()) {
			return ErrDailyCeiling
		}
		total = total.Add(allocation.Amount)
		if total.GT(DailyGCeiling()) {
			return ErrDailyCeiling
		}
	}
	issued := getInt(store, dailyKey).Add(total)
	if issued.GT(DailyGCeiling()) {
		return ErrDailyCeiling
	}
	earned := k.sub(cache, earnedPrefix)
	for _, allocation := range allocations {
		updated, err := checkedAdd(getInt(earned, allocation.Recipient), allocation.Amount)
		if err != nil {
			return err
		}
		putInt(earned, allocation.Recipient, updated)
	}
	pending, err := checkedAdd(getInt(store, pendingTotalKey), total)
	if err != nil {
		return err
	}
	putInt(store, pendingTotalKey, pending)
	putInt(store, dailyKey, issued)
	putNumber(store, lastHeightKey, ctx.BlockHeight())
	write()
	return nil
}

func (k Keeper) CollectGasFees(ctx sdk.Context) error {
	if err := k.ready(ctx); err != nil {
		return err
	}
	cache, write := ctx.CacheContext()
	if err := k.BeginDay(cache); err != nil {
		return err
	}
	coin := k.bank.GetBalance(cache, k.accounts.GetModuleAddress(authtypes.FeeCollectorName), GodDenom)
	if coin.IsPositive() {
		if err := k.bank.SendCoinsFromModuleToModule(cache, authtypes.FeeCollectorName, PendingModule, sdk.NewCoins(coin)); err != nil {
			return err
		}
	}
	write()
	return nil
}

// Donate voluntarily moves existing GOD into pending funds, never minting GOD.
// Transaction entry points must authenticate the donating account.
func (k Keeper) Donate(ctx sdk.Context, sender sdk.AccAddress, amount sdkmath.Int) error {
	if err := k.ready(ctx); err != nil {
		return err
	}
	if !validAccount(sender) {
		return ErrInvalidAccount
	}
	if k.protected(ctx, sender) {
		return ErrProtectedAccount
	}
	if !positive(amount) {
		return ErrInvalidAmount
	}
	cache, write := ctx.CacheContext()
	if err := k.BeginDay(cache); err != nil {
		return err
	}
	if err := k.bank.SendCoinsFromAccountToModule(cache, sender, PendingModule, sdk.NewCoins(sdk.NewCoin(GodDenom, amount))); err != nil {
		return err
	}
	if err := k.checkSupply(cache); err != nil {
		return err
	}
	write()
	return nil
}

func (k Keeper) SpendableG(ctx sdk.Context, owner sdk.AccAddress) sdkmath.Int {
	return getInt(k.sub(ctx, spendablePrefix), owner)
}
func (k Keeper) UnclaimedG(ctx sdk.Context, owner sdk.AccAddress) sdkmath.Int {
	return getInt(k.sub(ctx, unclaimedPrefix), owner)
}

// PendingEarnedG is a read-only view of the current period's contribution
// allocation. It is not settled, claimable or redeemable G.
func (k Keeper) PendingEarnedG(ctx sdk.Context, owner sdk.AccAddress) sdkmath.Int {
	return getInt(k.sub(ctx, earnedPrefix), owner)
}

// Claim changes the location of settled G, not its total supply.
func (k Keeper) Claim(ctx sdk.Context, owner sdk.AccAddress) error {
	if err := k.ready(ctx); err != nil {
		return err
	}
	if !validAccount(owner) {
		return ErrInvalidAccount
	}
	if k.protected(ctx, owner) {
		return ErrProtectedAccount
	}
	cache, write := ctx.CacheContext()
	if err := k.BeginDay(cache); err != nil {
		return err
	}
	unclaimed := k.sub(cache, unclaimedPrefix)
	amount := getInt(unclaimed, owner)
	if amount.IsZero() {
		return ErrInsufficientG
	}
	updated, err := checkedAdd(k.SpendableG(cache, owner), amount)
	if err != nil {
		return err
	}
	putInt(k.sub(cache, spendablePrefix), owner, updated)
	putInt(unclaimed, owner, sdkmath.ZeroInt())
	write()
	return nil
}

func (k Keeper) TransferG(ctx sdk.Context, from, to sdk.AccAddress, amount sdkmath.Int) error {
	if err := k.ready(ctx); err != nil {
		return err
	}
	if !validAccount(from) || !validAccount(to) {
		return ErrInvalidAccount
	}
	if k.protected(ctx, from) || k.protected(ctx, to) {
		return ErrProtectedAccount
	}
	if !positive(amount) {
		return ErrInvalidAmount
	}
	cache, write := ctx.CacheContext()
	if err := k.BeginDay(cache); err != nil {
		return err
	}
	balances := k.sub(cache, spendablePrefix)
	available := getInt(balances, from)
	if amount.GT(available) {
		return ErrInsufficientG
	}
	putInt(balances, from, available.Sub(amount))
	updated, err := checkedAdd(getInt(balances, to), amount)
	if err != nil {
		return err
	}
	putInt(balances, to, updated)
	write()
	return nil
}

type gLock struct {
	Owner  []byte      `json:"owner"`
	Amount sdkmath.Int `json:"amount"`
}

func validLockID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, char := range id {
		if !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') && char != '-' && char != '_' {
			return false
		}
	}
	return true
}

// Scope identifiers to their owner. A participant must not reserve another
// participant's application lock identifier by using the same text first.
func lockKey(owner sdk.AccAddress, id string) []byte {
	key := make([]byte, len(owner)+len(id))
	copy(key, owner)
	copy(key[len(owner):], id)
	return key
}

// Locks retain their outstanding redemption share but cannot be redeemed until
// unlocked. Used identifiers are retained to prevent stale unlock/relock replay.
func (k Keeper) LockG(ctx sdk.Context, owner sdk.AccAddress, id string, amount sdkmath.Int) error {
	if err := k.ready(ctx); err != nil {
		return err
	}
	if !validAccount(owner) {
		return ErrInvalidAccount
	}
	if k.protected(ctx, owner) {
		return ErrProtectedAccount
	}
	if !validLockID(id) {
		return ErrInvalidLock
	}
	if !positive(amount) {
		return ErrInvalidAmount
	}
	cache, write := ctx.CacheContext()
	if err := k.BeginDay(cache); err != nil {
		return err
	}
	key := lockKey(owner, id)
	if k.sub(cache, usedLockPrefix).Has(key) {
		return ErrInvalidLock
	}
	available := k.SpendableG(cache, owner)
	if amount.GT(available) {
		return ErrInsufficientG
	}
	bytes, err := json.Marshal(gLock{Owner: append([]byte(nil), owner...), Amount: amount})
	if err != nil {
		return err
	}
	k.sub(cache, lockPrefix).Set(key, bytes)
	k.sub(cache, usedLockPrefix).Set(key, []byte{1})
	putInt(k.sub(cache, spendablePrefix), owner, available.Sub(amount))
	write()
	return nil
}

func (k Keeper) UnlockG(ctx sdk.Context, owner sdk.AccAddress, id string) error {
	if err := k.ready(ctx); err != nil {
		return err
	}
	if !validAccount(owner) {
		return ErrInvalidAccount
	}
	if k.protected(ctx, owner) {
		return ErrProtectedAccount
	}
	if !validLockID(id) {
		return ErrInvalidLock
	}
	cache, write := ctx.CacheContext()
	if err := k.BeginDay(cache); err != nil {
		return err
	}
	locks := k.sub(cache, lockPrefix)
	key := lockKey(owner, id)
	bytes := locks.Get(key)
	if bytes == nil {
		return ErrInvalidLock
	}
	var lock gLock
	if err := json.Unmarshal(bytes, &lock); err != nil {
		return ErrGInvariant
	}
	if !owner.Equals(sdk.AccAddress(lock.Owner)) {
		return ErrUnauthorizedLock
	}
	if !positive(lock.Amount) {
		return ErrGInvariant
	}
	updated, err := checkedAdd(k.SpendableG(cache, owner), lock.Amount)
	if err != nil {
		return err
	}
	putInt(k.sub(cache, spendablePrefix), owner, updated)
	locks.Delete(key)
	write()
	return nil
}

// Redeem must be called by a message server that authenticates Sender. This
// keeper alone is not transaction authentication. All payout/burn writes are
// atomic in a single God SDK cache; errors leave both bank and G state intact.
func (k Keeper) Redeem(ctx sdk.Context, request Redemption) (sdkmath.Int, error) {
	zero := sdkmath.ZeroInt()
	if err := k.ready(ctx); err != nil {
		return zero, err
	}
	if !validAccount(request.Sender) || !validAccount(request.Beneficiary) {
		return zero, ErrInvalidAccount
	}
	if k.protected(ctx, request.Sender) || k.protected(ctx, request.Beneficiary) {
		return zero, ErrProtectedAccount
	}
	if k.store(ctx).Has(redeemingKey) {
		return zero, ErrReentrant
	}
	if !positive(request.G) || request.G.LT(MinimumRedemption()) || !positive(request.MinGodOut) {
		return zero, ErrInvalidAmount
	}
	if request.Deadline.IsZero() || ctx.BlockTime().After(request.Deadline) {
		return zero, ErrExpired
	}
	cache, write := ctx.CacheContext()
	if err := k.BeginDay(cache); err != nil {
		return zero, err
	}
	available := k.SpendableG(cache, request.Sender)
	if request.G.GT(available) {
		return zero, ErrInsufficientG
	}
	store := k.store(cache)
	total := getInt(store, outstandingKey)
	pool := k.bank.GetBalance(cache, k.accounts.GetModuleAddress(PoolModule), GodDenom).Amount
	output, err := Quote(pool, request.G, total)
	if err != nil {
		return zero, err
	}
	if output.LT(request.MinGodOut) {
		return zero, ErrMinimumOutput
	}
	store.Set(redeemingKey, []byte{1})
	if err := k.bank.SendCoinsFromModuleToAccount(cache, PoolModule, request.Beneficiary, sdk.NewCoins(sdk.NewCoin(GodDenom, output))); err != nil {
		return zero, err
	}
	putInt(k.sub(cache, spendablePrefix), request.Sender, available.Sub(request.G))
	putInt(store, outstandingKey, total.Sub(request.G))
	if err := k.checkSupply(cache); err != nil {
		return zero, err
	}
	store.Delete(redeemingKey)
	write()
	return output, nil
}

func (k Keeper) Snapshot(ctx sdk.Context) (Snapshot, error) {
	if err := k.ready(ctx); err != nil {
		return Snapshot{}, err
	}
	store := k.store(ctx)
	return Snapshot{
		PoolGod:      k.bank.GetBalance(ctx, k.accounts.GetModuleAddress(PoolModule), GodDenom).Amount,
		PendingGod:   k.bank.GetBalance(ctx, k.accounts.GetModuleAddress(PendingModule), GodDenom).Amount,
		OutstandingG: getInt(store, outstandingKey), PendingG: getInt(store, pendingTotalKey),
		DayIssuedG: getInt(store, dailyKey), ActiveDay: getNumber(store, dayKey), LastRewardHeight: getNumber(store, lastHeightKey),
	}, nil
}

// CheckInvariants is an exhaustive diagnostic, not an O(1) per-block operation.
// Its cost and the day-boundary settlement cost require load testing before
// ordinary-computer support or production gas bounds can be claimed.
func (k Keeper) CheckInvariants(ctx sdk.Context) error {
	if err := k.ready(ctx); err != nil {
		return err
	}
	if k.store(ctx).Has(redeemingKey) {
		return ErrReentrant
	}
	sum := sdkmath.ZeroInt()
	for _, p := range [][]byte{spendablePrefix, unclaimedPrefix} {
		store := k.sub(ctx, p)
		iterator := store.Iterator(nil, nil)
		for ; iterator.Valid(); iterator.Next() {
			owner := sdk.AccAddress(iterator.Key())
			if !validAccount(owner) || k.protected(ctx, owner) || !positive(getInt(store, owner)) {
				iterator.Close()
				return ErrGInvariant
			}
			var err error
			sum, err = checkedAdd(sum, getInt(store, iterator.Key()))
			if err != nil {
				iterator.Close()
				return err
			}
		}
		iterator.Close()
	}
	iterator := k.sub(ctx, lockPrefix).Iterator(nil, nil)
	for ; iterator.Valid(); iterator.Next() {
		var lock gLock
		if err := json.Unmarshal(iterator.Value(), &lock); err != nil || !validAccount(sdk.AccAddress(lock.Owner)) || k.protected(ctx, sdk.AccAddress(lock.Owner)) || !positive(lock.Amount) || len(iterator.Key()) <= 20 || !validLockID(string(iterator.Key()[20:])) || !sdk.AccAddress(iterator.Key()[:20]).Equals(sdk.AccAddress(lock.Owner)) || !k.sub(ctx, usedLockPrefix).Has(iterator.Key()) {
			iterator.Close()
			return ErrGInvariant
		}
		var err error
		sum, err = checkedAdd(sum, lock.Amount)
		if err != nil {
			iterator.Close()
			return err
		}
	}
	iterator.Close()
	if !sum.Equal(getInt(k.store(ctx), outstandingKey)) {
		return ErrGInvariant
	}
	earned := k.sub(ctx, earnedPrefix)
	iterator = earned.Iterator(nil, nil)
	pending := sdkmath.ZeroInt()
	for ; iterator.Valid(); iterator.Next() {
		owner := sdk.AccAddress(iterator.Key())
		if !validAccount(owner) || k.protected(ctx, owner) || !positive(getInt(earned, owner)) {
			iterator.Close()
			return ErrGInvariant
		}
		var err error
		pending, err = checkedAdd(pending, getInt(earned, iterator.Key()))
		if err != nil {
			iterator.Close()
			return err
		}
	}
	iterator.Close()
	issued := getInt(k.store(ctx), dailyKey)
	if !pending.Equal(getInt(k.store(ctx), pendingTotalKey)) || !pending.Equal(issued) || issued.GT(DailyGCeiling()) {
		return ErrGInvariant
	}
	return nil
}
