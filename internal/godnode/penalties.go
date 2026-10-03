package godnode

import (
	"context"
	"encoding/json"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtx"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const signingWindow int64 = 10000
const maxMissed int64 = 1000
const suspension = time.Hour

func penaltyParams() slashingtypes.Params {
	return slashingtypes.NewParams(signingWindow, sdkmath.LegacyNewDecWithPrec(9, 1), suspension,
		sdkmath.LegacyNewDecWithPrec(5, 2), sdkmath.LegacyZeroDec())
}

func penaltyKey(prefix byte, address []byte) []byte {
	return append([]byte{prefix}, address...)
}

// Count only expected signing opportunities, not heights while unbonded.
// Count saturates at one full window; the SDK bitmap/index stays bounded.
type opportunityProgress struct {
	Count, LastHeight int64
}

// This is a lower bound from commits already cryptographically checked by the
// local consensus engine, NOT a substitute for the original FilePV/WAL. It
// permits an offline validator's signer to lag the app without allowing its
// previously observed signatures to be silently forgotten on restart.
type signingProgress struct {
	Height int64
	Round  int32
	Step   int8
}

func (a *App) observedSigning(ctx sdk.Context, address []byte) (signingProgress, error) {
	var p signingProgress
	if raw := ctx.KVStore(a.key).Get(penaltyKey(5, address)); raw != nil {
		if json.Unmarshal(raw, &p) != nil || p.Height <= 0 || p.Height >= ctx.BlockHeight() || p.Round < 0 || p.Step != 3 {
			return p, ErrConfig
		}
	}
	return p, nil
}

// ABCI misconduct summaries are trusted ONLY at the private in-process client
// boundary. GodCometBFT validates both signatures and historical membership
// before delivering them. No transaction accepts evidence or a slash amount.
// Light-client attacks need a separate policy and remain unsupported.
func validMisbehavior(height int64, now time.Time, evidence []abci.Misbehavior) bool {
	if len(evidence) > 32 {
		return false
	}
	for _, e := range evidence {
		if e.Type != abci.MisbehaviorType_DUPLICATE_VOTE || len(e.Validator.Address) != 20 || e.Validator.Power <= 0 ||
			e.Validator.Power > 1_000_000_000 || e.TotalVotingPower < e.Validator.Power || e.TotalVotingPower > 1_000_000_000 ||
			e.Height <= 0 || e.Height >= height || !validTime(e.Time) || e.Time.After(now) {
			return false
		}
	}
	return true
}

func (a *App) applyPenalties(ctx sdk.Context, req *abci.RequestFinalizeBlock) error {
	if !validMisbehavior(req.Height, req.Time, req.Misbehavior) {
		return ErrBlock
	}
	for _, e := range req.Misbehavior {
		if err := a.doubleSign(ctx, e); err != nil {
			return err
		}
	}
	if req.Height == 1 {
		return nil
	}
	set, err := a.getSet(ctx, req.Height-1)
	if err != nil {
		return err
	}
	if _, err := validateCommit(set, req.DecidedLastCommit); err != nil {
		return err
	}
	for _, vote := range req.DecidedLastCommit.Votes {
		addr := sdk.ConsAddress(vote.Validator.Address)
		if vote.BlockIdFlag != cmtproto.BlockIDFlagAbsent {
			p := signingProgress{req.Height - 1, req.DecidedLastCommit.Round, 3}
			raw, err := json.Marshal(p)
			if err != nil {
				return err
			}
			ctx.KVStore(a.key).Set(penaltyKey(5, addr), raw)
		}
		// A verified nil precommit is participation, not offline behavior.
		// It earns no block G, but must not be counted as a missing signature.
		if err := a.recordOpportunity(ctx, addr, req.Height-1, vote.BlockIdFlag == cmtproto.BlockIDFlagAbsent); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) recordOpportunity(ctx sdk.Context, addr sdk.ConsAddress, height int64, missed bool) error {
	info, err := a.slashing.GetValidatorSigningInfo(ctx, addr)
	if err != nil {
		return err
	}
	v, err := a.staking.GetValidatorByConsAddr(ctx, addr)
	if err != nil {
		return err
	}
	// The consensus set has a two-block removal delay. A jailed validator must
	// not accumulate a second suspension during that delay.
	if v.Jailed || info.Tombstoned {
		return nil
	}
	var p opportunityProgress
	key := penaltyKey(4, addr)
	if raw := ctx.KVStore(a.key).Get(key); raw != nil && json.Unmarshal(raw, &p) != nil {
		return ErrBlock
	}
	if height <= p.LastHeight || p.Count < 0 || p.Count > signingWindow || info.IndexOffset < 0 || info.IndexOffset >= signingWindow ||
		info.MissedBlocksCounter < 0 || info.MissedBlocksCounter > signingWindow {
		return ErrBlock
	}
	old, err := a.slashing.GetMissedBlockBitmapValue(ctx, addr, info.IndexOffset)
	if err != nil {
		return err
	}
	if old != missed {
		if err := a.slashing.SetMissedBlockBitmapValue(ctx, addr, info.IndexOffset, missed); err != nil {
			return err
		}
		if missed {
			info.MissedBlocksCounter++
		} else {
			info.MissedBlocksCounter--
		}
	}
	info.IndexOffset = (info.IndexOffset + 1) % signingWindow
	if p.Count < signingWindow {
		p.Count++
	}
	p.LastHeight = height
	// Evaluate after a complete window, including exactly the 10,000th
	// opportunity. Exactly 1,000 misses is permitted; 1,001 is not.
	if p.Count == signingWindow && info.MissedBlocksCounter > maxMissed {
		if err := a.staking.Jail(ctx, addr); err != nil {
			return err
		}
		info.JailedUntil = ctx.BlockTime().Add(suspension)
		ctx.EventManager().EmitEvent(sdk.NewEvent("god_validator_suspended"))
		// Deliberately NO Slash call: ordinary downtime never removes principal.
	}
	if err := a.slashing.SetValidatorSigningInfo(ctx, addr, info); err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	ctx.KVStore(a.key).Set(key, raw)
	return nil
}

func (a *App) doubleSign(ctx sdk.Context, e abci.Misbehavior) error {
	addr := sdk.ConsAddress(e.Validator.Address)
	info, err := a.slashing.GetValidatorSigningInfo(ctx, addr)
	if err != nil {
		return err
	}
	if info.Tombstoned {
		return nil // Repeated or multiple historical proofs never charge twice.
	}
	v, err := a.staking.GetValidatorByConsAddr(ctx, addr)
	if err != nil {
		return err
	}
	before := a.bank.GetBalance(ctx, a.accounts.GetModuleAddress(quarantineModule), godrewards.GodDenom).Amount
	if !v.IsUnbonded() {
		// Keep the SDK's infraction-height handling of unbonding/redelegation
		// exposure. stakingBank redirects every pool BurnCoins to quarantine.
		if _, err := a.staking.SlashWithInfractionReason(ctx, addr, e.Height-sdk.ValidatorUpdateDelay, e.Validator.Power,
			penaltyParams().SlashFractionDoubleSign, stakingtypes.Infraction_INFRACTION_DOUBLE_SIGN); err != nil {
			return err
		}
	}
	if !v.Jailed {
		if err := a.staking.Jail(ctx, addr); err != nil {
			return err
		}
	}
	info.Tombstoned = true
	// Tombstone, not a timestamp, is the permanent exclusion authority.
	if err := a.slashing.SetValidatorSigningInfo(ctx, addr, info); err != nil {
		return err
	}
	operator, err := validatorAddress(v.OperatorAddress)
	if err != nil {
		return err
	}
	ctx.KVStore(a.key).Set(penaltyKey(6, operator), []byte{1})
	after := a.bank.GetBalance(ctx, a.accounts.GetModuleAddress(quarantineModule), godrewards.GodDenom).Amount
	if after.LT(before) || !a.bank.GetSupply(ctx, godrewards.GodDenom).Amount.Equal(godrewards.FixedGodSupply()) {
		return ErrSupply
	}
	ctx.EventManager().EmitEvent(sdk.NewEvent("god_double_sign_quarantine", sdk.NewAttribute("amount", after.Sub(before).String())))
	return nil
}

type penaltyServer struct {
	slashingtypes.UnimplementedMsgServer
	app *App
}

func (s penaltyServer) Unjail(ctx context.Context, m *slashingtypes.MsgUnjail) (*slashingtypes.MsgUnjailResponse, error) {
	if err := validateNativeMessage(m); err != nil {
		return nil, err
	}
	c := sdk.UnwrapSDKContext(ctx)
	operator, _ := validatorAddress(m.ValidatorAddr)
	v, err := s.app.staking.GetValidator(c, operator)
	if err != nil {
		return nil, err
	}
	addr, err := v.GetConsAddr()
	if err != nil {
		return nil, err
	}
	info, err := s.app.slashing.GetValidatorSigningInfo(c, addr)
	if err != nil || info.Tombstoned || c.KVStore(s.app.key).Has(penaltyKey(6, operator)) || !v.Jailed || c.BlockTime().Before(info.JailedUntil) {
		return nil, godtx.ErrPolicy
	}
	self, err := s.app.staking.GetDelegation(c, sdk.AccAddress(operator), operator)
	if err != nil || v.TokensFromShares(self.Shares).TruncateInt().LT(minSelfStake()) {
		return nil, godtx.ErrPolicy
	}
	if err := s.app.staking.Unjail(c, addr); err != nil {
		return nil, err
	}
	info.IndexOffset, info.MissedBlocksCounter = 0, 0
	if err := s.app.slashing.DeleteMissedBlockBitmap(c, addr); err != nil {
		return nil, err
	}
	if err := s.app.slashing.SetValidatorSigningInfo(c, addr, info); err != nil {
		return nil, err
	}
	c.KVStore(s.app.key).Delete(penaltyKey(4, addr))
	return &slashingtypes.MsgUnjailResponse{}, nil
}
