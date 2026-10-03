package godnode

import (
	"context"
	"math/big"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

const quarantineModule = "god_quarantine"

// stakingBank replaces ALL upstream staking burn paths (including historical
// unbonding/redelegation exposure) with custody in a non-spendable quarantine.
// Neither the base bank nor any module receives GOD mint/burn permission.
type stakingBank struct{ bankkeeper.BaseKeeper }

func (b stakingBank) BurnCoins(ctx context.Context, module string, coins sdk.Coins) error {
	if (module != stakingtypes.BondedPoolName && module != stakingtypes.NotBondedPoolName) || !godCoins(coins) {
		return ErrSupply
	}
	return b.SendCoinsFromModuleToModule(ctx, module, quarantineModule, coins)
}

// evmBank stages the upstream balance-setter's deltas rather than minting or
// burning. With native precompiles and hooks disabled, there is one final flush
// per Ethereum message. Debits execute before credits in the same message cache,
// regardless of address sorting, with no reserve loan and no supply mutation.
type evmBank struct {
	bankkeeper.BaseKeeper
	transient *storetypes.TransientStoreKey
}

var creditPrefix = byte(1)
var debitPrefix = byte(2)
var mintTicket = []byte{3}
var burnTicket = []byte{4}

func godCoins(coins sdk.Coins) bool {
	return coins.IsValid() && len(coins) == 1 && coins[0].Denom == godrewards.GodDenom && coins[0].IsPositive()
}

func (b evmBank) ticket(ctx context.Context, key []byte, coins sdk.Coins, put bool) error {
	if !godCoins(coins) {
		return ErrSupply
	}
	store := sdk.UnwrapSDKContext(ctx).TransientStore(b.transient)
	if put {
		if store.Has(key) {
			return ErrSupply
		}
		store.Set(key, []byte(coins[0].Amount.String()))
	} else {
		if string(store.Get(key)) != coins[0].Amount.String() {
			return ErrSupply
		}
		store.Delete(key)
	}
	return nil
}

func (b evmBank) MintCoins(ctx context.Context, module string, coins sdk.Coins) error {
	if module != evmtypes.ModuleName {
		return ErrSupply
	}
	return b.ticket(ctx, mintTicket, coins, true)
}

func (b evmBank) BurnCoins(ctx context.Context, module string, coins sdk.Coins) error {
	if module != evmtypes.ModuleName {
		return ErrSupply
	}
	return b.ticket(ctx, burnTicket, coins, false)
}

func (b evmBank) stage(ctx context.Context, prefix byte, address sdk.AccAddress, coins sdk.Coins) error {
	if len(address) != 20 || !godCoins(coins) {
		return ErrSupply
	}
	store := sdk.UnwrapSDKContext(ctx).TransientStore(b.transient)
	key := append([]byte{prefix}, address...)
	if store.Has(key) {
		return ErrSupply
	}
	store.Set(key, []byte(coins[0].Amount.String()))
	return nil
}

func (b evmBank) SendCoinsFromAccountToModule(ctx context.Context, sender sdk.AccAddress, module string, coins sdk.Coins) error {
	if module != evmtypes.ModuleName {
		return b.BaseKeeper.SendCoinsFromAccountToModule(ctx, sender, module, coins)
	}
	if err := b.ticket(ctx, burnTicket, coins, true); err != nil {
		return err
	}
	return b.stage(ctx, debitPrefix, sender, coins)
}

func (b evmBank) SendCoinsFromModuleToAccount(ctx context.Context, module string, recipient sdk.AccAddress, coins sdk.Coins) error {
	if module != evmtypes.ModuleName {
		return b.BaseKeeper.SendCoinsFromModuleToAccount(ctx, module, recipient, coins)
	}
	if err := b.ticket(ctx, mintTicket, coins, false); err != nil {
		return err
	}
	return b.stage(ctx, creditPrefix, recipient, coins)
}

func (b evmBank) flush(ctx sdk.Context) error {
	store := ctx.TransientStore(b.transient)
	if store.Has(mintTicket) || store.Has(burnTicket) {
		return ErrSupply
	}
	if !b.GetBalance(ctx, sdk.AccAddress(evmModuleAddress()), godrewards.GodDenom).IsZero() {
		return ErrSupply
	}
	type movement struct {
		address sdk.AccAddress
		coins   sdk.Coins
		key     []byte
	}
	groups := [2][]movement{}
	totals := [2]*big.Int{new(big.Int), new(big.Int)}
	for group, prefix := range []byte{debitPrefix, creditPrefix} {
		iterator := storetypes.KVStorePrefixIterator(store, []byte{prefix})
		for ; iterator.Valid(); iterator.Next() {
			key := append([]byte(nil), iterator.Key()...)
			n, ok := sdkmath.NewIntFromString(string(iterator.Value()))
			if !ok || !n.IsPositive() || len(key) != 21 {
				iterator.Close()
				return ErrSupply
			}
			totals[group].Add(totals[group], n.BigInt())
			groups[group] = append(groups[group], movement{key[1:], sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, n)), key})
		}
		iterator.Close()
	}
	// EVM self-destruction-to-self may discard value. It is quarantined, never
	// destroyed, redistributed or made available as bridge/reward liquidity.
	if totals[1].Cmp(totals[0]) > 0 {
		return ErrSupply
	}
	for _, move := range groups[0] {
		if err := b.BaseKeeper.SendCoinsFromAccountToModule(ctx, move.address, evmtypes.ModuleName, move.coins); err != nil {
			return err
		}
		store.Delete(move.key)
	}
	for _, move := range groups[1] {
		if err := b.BaseKeeper.SendCoinsFromModuleToAccount(ctx, evmtypes.ModuleName, move.address, move.coins); err != nil {
			return err
		}
		store.Delete(move.key)
	}
	remaining := b.GetBalance(ctx, sdk.AccAddress(evmModuleAddress()), godrewards.GodDenom)
	if remaining.IsPositive() {
		if err := b.SendCoinsFromModuleToModule(ctx, evmtypes.ModuleName, quarantineModule, sdk.NewCoins(remaining)); err != nil {
			return err
		}
	}
	if !b.GetSupply(ctx, godrewards.GodDenom).Amount.Equal(godrewards.FixedGodSupply()) {
		return ErrSupply
	}
	return nil
}
