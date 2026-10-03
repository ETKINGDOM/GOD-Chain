package godnode

import (
	"context"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtx"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func minSelfStake() sdkmath.Int     { return godrewards.Unit().MulRaw(1000) }
func commission() sdkmath.LegacyDec { return sdkmath.LegacyNewDecWithPrec(1, 1) }

func validatorAddress(text string) ([]byte, error) {
	codec := addresscodec.NewBech32Codec(godaddress.ValidatorOperatorPrefix)
	address, err := codec.StringToBytes(text)
	if err != nil || len(address) != 20 {
		return nil, godtx.ErrPolicy
	}
	canonical, err := codec.BytesToString(address)
	if err != nil || canonical != text {
		return nil, godtx.ErrPolicy
	}
	return address, nil
}

func stakeCoin(coin sdk.Coin) bool {
	return coin.IsValid() && coin.Denom == godrewards.GodDenom && coin.Amount.GTE(godrewards.Unit()) && coin.Amount.LTE(godrewards.FixedGodSupply())
}

func validateNativeMessage(message sdk.Msg) error {
	switch m := message.(type) {
	case *stakingtypes.MsgCreateValidator:
		if m == nil || m.Pubkey == nil || m.Commission.Rate.IsNil() || m.Commission.MaxRate.IsNil() || m.Commission.MaxChangeRate.IsNil() || !stakeCoin(m.Value) || m.Value.Amount.LT(minSelfStake()) || m.MinSelfDelegation.IsNil() ||
			!m.MinSelfDelegation.Equal(minSelfStake()) || !m.Commission.Rate.Equal(commission()) || !m.Commission.MaxRate.Equal(commission()) ||
			!m.Commission.MaxChangeRate.IsZero() {
			return godtx.ErrPolicy
		}
		if _, err := validatorAddress(m.ValidatorAddress); err != nil {
			return err
		}
		pk, ok := m.Pubkey.GetCachedValue().(*ed25519.PubKey)
		if !ok || pk == nil || len(pk.Key) != 32 {
			return godtx.ErrKey
		}
		return m.Validate(addresscodec.NewBech32Codec(godaddress.ValidatorOperatorPrefix))
	case *stakingtypes.MsgDelegate:
		if m == nil || !stakeCoin(m.Amount) {
			return godtx.ErrPolicy
		}
		if _, err := msg.Account(m.DelegatorAddress); err != nil {
			return err
		}
		_, err := validatorAddress(m.ValidatorAddress)
		return err
	case *stakingtypes.MsgUndelegate:
		if m == nil || !stakeCoin(m.Amount) {
			return godtx.ErrPolicy
		}
		if _, err := msg.Account(m.DelegatorAddress); err != nil {
			return err
		}
		_, err := validatorAddress(m.ValidatorAddress)
		return err
	default:
		// Redelegation, commission edits, pool spending, governance and parameter
		// updates are intentionally not authenticated routes in this milestone.
		return msg.Validate(message)
	}
}

type stakingServer struct {
	stakingtypes.MsgServer
	app *App
}

func (s stakingServer) Delegate(ctx context.Context, m *stakingtypes.MsgDelegate) (*stakingtypes.MsgDelegateResponse, error) {
	if err := validateNativeMessage(m); err != nil {
		return nil, err
	}
	val, err := validatorAddress(m.ValidatorAddress)
	if err != nil {
		return nil, err
	}
	delegator, err := msg.Account(m.DelegatorAddress)
	if err != nil {
		return nil, err
	}
	if _, err := s.app.staking.GetDelegation(ctx, delegator, val); err != nil {
		delegations, err := s.app.staking.GetValidatorDelegations(ctx, val)
		if err != nil || len(delegations) >= 256 {
			return nil, godtx.ErrPolicy
		}
	}
	return s.MsgServer.Delegate(ctx, m)
}

func (s stakingServer) Undelegate(ctx context.Context, m *stakingtypes.MsgUndelegate) (*stakingtypes.MsgUndelegateResponse, error) {
	if err := validateNativeMessage(m); err != nil {
		return nil, err
	}
	val, err := validatorAddress(m.ValidatorAddress)
	if err != nil {
		return nil, err
	}
	delegator, err := msg.Account(m.DelegatorAddress)
	if err != nil {
		return nil, err
	}
	d, err := s.app.staking.GetDelegation(ctx, delegator, val)
	if err != nil {
		return nil, err
	}
	v, err := s.app.staking.GetValidator(ctx, val)
	if err != nil {
		return nil, err
	}
	remaining := v.TokensFromShares(d.Shares).TruncateInt().Sub(m.Amount.Amount)
	if remaining.IsPositive() && remaining.LT(godrewards.Unit()) {
		return nil, godtx.ErrPolicy
	}
	return s.MsgServer.Undelegate(ctx, m)
}
