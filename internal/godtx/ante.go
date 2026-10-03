package godtx

import (
	"bytes"
	"errors"
	"math"
	"math/big"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	ErrPolicy = errors.New("native G transaction policy rejected")
	ErrKey    = errors.New("native G transactions require one valid EVM-compatible signer")
	ErrFee    = errors.New("insufficient or invalid GOD fee")
)

// Policy must be explicitly selected by the application. There is deliberately
// no default mainnet gas price or claim that these limits are economically final.
type Policy struct {
	MaxTxBytes   int
	MaxMessages  int
	MaxGas       uint64
	MinFeePerGas sdkmath.Int // smallest GOD units per requested gas unit
	SignatureGas uint64
	MessageGas   uint64
}

func (p Policy) valid() bool {
	return p.MaxTxBytes > 0 && p.MaxTxBytes <= 1<<20 && p.MaxMessages > 0 && p.MaxMessages <= 64 &&
		p.MaxGas > 0 && p.MaxGas <= math.MaxInt64 && !p.MinFeePerGas.IsNil() && p.MinFeePerGas.IsPositive() &&
		p.SignatureGas > 0 && p.SignatureGas <= p.MaxGas && p.MessageGas > 0 && p.MessageGas <= p.MaxGas
}

// NewAnte reuses the standard God SDK account number, chain ID, sequence,
// SIGN_MODE_DIRECT verification, timeout, size gas and fee deduction. BaseApp
// supplies the ante cache: callers must not run this against uncached state.
// Its separate message cache preserves fees/sequence on business failure.
func NewAnte(ak ante.AccountKeeper, bk authtypes.BankKeeper, config client.TxConfig, keeper godrewards.Keeper, policy Policy) (sdk.AnteHandler, error) {
	if !policy.valid() || config == nil {
		return nil, ErrPolicy
	}
	if ak == nil || bk == nil || config.SignModeHandler() == nil {
		return nil, ErrPolicy
	}
	// Detach the consensus policy from the caller's mutable integer backing.
	policy.MinFeePerGas = sdkmath.NewIntFromBigInt(policy.MinFeePerGas.BigInt())
	gasConsumer := func(meter storetypes.GasMeter, signature signing.SignatureV2, _ authtypes.Params) error {
		if _, ok := signature.PubKey.(*ethsecp256k1.PubKey); !ok {
			return ErrKey
		}
		meter.ConsumeGas(policy.SignatureGas, "native G signature verification")
		return nil
	}
	feeChecker := func(_ sdk.Context, tx sdk.Tx) (sdk.Coins, int64, error) {
		feeTx, ok := tx.(sdk.FeeTx)
		if !ok {
			return nil, 0, ErrFee
		}
		fees := feeTx.GetFee()
		minimum := new(big.Int).Mul(policy.MinFeePerGas.BigInt(), new(big.Int).SetUint64(feeTx.GetGas()))
		if !fees.IsValid() || len(fees) != 1 || fees[0].Denom != godrewards.GodDenom ||
			fees[0].Amount.BigInt().Cmp(minimum) < 0 {
			return nil, 0, ErrFee
		}
		return fees, 0, nil
	}
	// Keep the reference SDK decorator order. The final fee-routing decorator
	// stays inside SetUpContext's gas recovery and the BaseApp ante cache.
	chain := sdk.ChainAnteDecorators(
		ante.NewSetUpContextDecorator(),
		policyDecorator{policy},
		ante.NewExtensionOptionsDecorator(nil),
		ante.NewValidateBasicDecorator(),
		ante.NewTxTimeoutHeightDecorator(),
		ante.NewValidateMemoDecorator(ak),
		ante.NewConsumeGasForTxSizeDecorator(ak),
		ante.NewDeductFeeDecorator(ak, bk, nil, feeChecker),
		ante.NewSetPubKeyDecorator(ak),
		ante.NewValidateSigCountDecorator(ak),
		ante.NewSigGasConsumeDecorator(ak, gasConsumer),
		ante.NewSigVerificationDecorator(ak, config.SignModeHandler()),
		ante.NewIncrementSequenceDecorator(ak),
		feeRoutingDecorator{keeper, policy.MessageGas},
	)
	return func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		// Guard SetUpContext.GetGas even if an internal caller bypasses Decoder.
		if err := validateStructure(tx); err != nil {
			return ctx, err
		}
		return chain(ctx, tx, simulate)
	}, nil
}

type policyDecorator struct{ policy Policy }

func (d policyDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	// Genesis bypasses standard signature/gas checks; never allow this path.
	if simulate || ctx.BlockHeight() <= 0 || ctx.ChainID() == "" || len(ctx.TxBytes()) == 0 || len(ctx.TxBytes()) > d.policy.MaxTxBytes {
		return ctx, ErrPolicy
	}
	if err := validate(tx, d.policy); err != nil {
		return ctx, err
	}
	return next(ctx, tx, false)
}

type feeRoutingDecorator struct {
	keeper     godrewards.Keeper
	messageGas uint64
}

func (d feeRoutingDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (sdk.Context, error) {
	// Charge a prototype operation cost in addition to SDK storage gas.
	// No allocation message is exposed, so participants cannot issue G.
	for range tx.GetMsgs() {
		ctx.GasMeter().ConsumeGas(d.messageGas, "native G operation")
	}
	// Fees settle to pending only after authentication. CollectGasFees and
	// its daily boundary run inside the same BaseApp ante cache.
	if err := d.keeper.CollectGasFees(ctx); err != nil {
		return ctx, err
	}
	return next(ctx, tx, simulate)
}

func validate(tx sdk.Tx, p Policy) error {
	if err := validateStructure(tx); err != nil {
		return err
	}
	native, ok := tx.(authsigning.Tx)
	if !ok || native.GetUnordered() || !native.GetTimeoutTimeStamp().IsZero() || native.GetMemo() != "" ||
		native.GetGas() == 0 || native.GetGas() > p.MaxGas {
		return ErrPolicy
	}
	if ext, ok := tx.(ante.HasExtensionOptionsTx); ok && (len(ext.GetExtensionOptions()) != 0 || len(ext.GetNonCriticalExtensionOptions()) != 0) {
		return ErrPolicy
	}
	messages := native.GetMsgs()
	if len(messages) == 0 || len(messages) > p.MaxMessages {
		return ErrPolicy
	}
	for _, message := range messages {
		if err := msg.Validate(message); err != nil {
			return err
		}
	}
	signers, err := native.GetSigners()
	if err != nil || len(signers) != 1 || len(signers[0]) != 20 || !bytes.Equal(signers[0], native.FeePayer()) {
		return ErrKey
	}
	signatures, err := native.GetSignaturesV2()
	if err != nil || len(signatures) != 1 || signatures[0].Sequence == math.MaxUint64 {
		return ErrKey
	}
	public, ok := signatures[0].PubKey.(*ethsecp256k1.PubKey)
	if !ok || public == nil || len(public.Key) != ethsecp256k1.PubKeySize {
		return ErrKey
	}
	if _, err := crypto.DecompressPubkey(public.Key); err != nil {
		return ErrKey
	}
	if !bytes.Equal(public.Address(), signers[0]) {
		return ErrKey
	}
	data, ok := signatures[0].Data.(*signing.SingleSignatureData)
	if !ok || data.SignMode != signing.SignMode_SIGN_MODE_DIRECT || len(data.Signature) != crypto.SignatureLength || data.Signature[crypto.SignatureLength-1] > 1 {
		return ErrKey
	}
	return nil
}
