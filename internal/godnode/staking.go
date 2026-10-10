package godnode

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtx"
	bridgemsg "github.com/ETKINGDOM/GOD-Chain/x/godbridge/msg"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	sdked "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func minSelfStake() sdkmath.Int     { return godrewards.Unit().MulRaw(1000) }
func commission() sdkmath.LegacyDec { return sdkmath.LegacyNewDecWithPrec(1, 1) }

const validatorProofPrefix = "god-chain-consensus-possession-v1:"

// ValidatorProofBytes is the deterministic, domain-separated node-key intent.
// It binds the immutable runtime and every registration field except the proof
// carrier itself. Account signatures separately bind transaction fees/sequence.
// This is not a consensus vote, proposal, ownership grant or signing-state reset.
func ValidatorProofBytes(c Config, m *stakingtypes.MsgCreateValidator) ([]byte, error) {
	if c.validate() != nil || !c.RequireValidatorProof || validateNativeMessage(m) != nil {
		return nil, godtx.ErrPolicy
	}
	if _, err := m.Description.EnsureLength(); err != nil {
		return nil, godtx.ErrPolicy
	}
	config, err := json.Marshal(c)
	if err != nil {
		return nil, godtx.ErrPolicy
	}
	digest := sha256.Sum256(config)
	description := m.Description
	description.Details = ""
	pub := m.Pubkey.GetCachedValue().(*sdked.PubKey)
	raw, err := json.Marshal(struct {
		RuntimeSHA256 [32]byte
		Operator      string
		PublicKey     []byte
		Value         sdk.Coin
		Minimum       string
		Commission    stakingtypes.CommissionRates
		Description   stakingtypes.Description
	}{digest, m.ValidatorAddress, pub.Key, m.Value, m.MinSelfDelegation.String(), m.Commission, description})
	if err != nil {
		return nil, godtx.ErrPolicy
	}
	return append([]byte("GOD Chain synthetic validator possession v1\n"), raw...), nil
}

// AttachValidatorProof verifies before filling the bounded standard staking
// description carrier. No new protobuf route or public gateway action is added.
func AttachValidatorProof(c Config, m *stakingtypes.MsgCreateValidator, signature []byte) error {
	raw, err := ValidatorProofBytes(c, m)
	if err != nil || len(signature) != ed25519.SignatureSize || !ed25519.Verify(m.Pubkey.GetCachedValue().(*sdked.PubKey).Key, raw, signature) {
		return godtx.ErrPolicy
	}
	m.Description.Details = validatorProofPrefix + base64.RawURLEncoding.EncodeToString(signature)
	return nil
}

// ValidateValidatorProof fails closed for every authenticated registration on
// a proof-enabled network. Genesis remains a separately reviewed launch input.
func ValidateValidatorProof(c Config, m *stakingtypes.MsgCreateValidator) error {
	raw, err := ValidatorProofBytes(c, m)
	if err != nil || len(m.Description.Details) != len(validatorProofPrefix)+86 || !strings.HasPrefix(m.Description.Details, validatorProofPrefix) {
		return godtx.ErrPolicy
	}
	text := strings.TrimPrefix(m.Description.Details, validatorProofPrefix)
	proof, err := base64.RawURLEncoding.Strict().DecodeString(text)
	if err != nil || len(proof) != ed25519.SignatureSize || base64.RawURLEncoding.EncodeToString(proof) != text || !ed25519.Verify(m.Pubkey.GetCachedValue().(*sdked.PubKey).Key, raw, proof) {
		return godtx.ErrPolicy
	}
	return nil
}

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

func (a *App) validateNativeMessage(message sdk.Msg) error {
	if m, ok := message.(*stakingtypes.MsgCreateValidator); ok && a.config.RequireValidatorProof {
		return ValidateValidatorProof(a.config, m)
	}
	switch message.(type) {
	case *bridgemsg.MsgAcceptDeposit, *bridgemsg.MsgRequestWithdrawal, *bridgemsg.MsgAuthorizeWithdrawal,
		*bridgemsg.MsgResolvePayment, *bridgemsg.MsgCancelWithdrawal, *bridgemsg.MsgPauseBridge:
		if a.config.BridgeApprovalGas == 0 || a.bridge == nil || !a.bridge.IsConfigured() {
			return godtx.ErrPolicy
		}
		return bridgemsg.Validate(message)
	default:
		return validateNativeMessage(message)
	}
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
		pk, ok := m.Pubkey.GetCachedValue().(*sdked.PubKey)
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
	case *slashingtypes.MsgUnjail:
		if m == nil {
			return godtx.ErrPolicy
		}
		_, err := validatorAddress(m.ValidatorAddr)
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

func (s stakingServer) CreateValidator(ctx context.Context, m *stakingtypes.MsgCreateValidator) (*stakingtypes.MsgCreateValidatorResponse, error) {
	if err := s.app.validateNativeMessage(m); err != nil {
		return nil, err
	}
	val, _ := validatorAddress(m.ValidatorAddress)
	pub := m.Pubkey.GetCachedValue().(*sdked.PubKey)
	c := sdk.UnwrapSDKContext(ctx)
	if c.KVStore(s.app.key).Has(penaltyKey(6, val)) || s.app.slashing.IsTombstoned(c, sdk.ConsAddress(pub.Address())) {
		return nil, godtx.ErrPolicy
	}
	return s.MsgServer.CreateValidator(ctx, m)
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
