package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"strconv"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtx"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txproto "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/ethereum/go-ethereum/crypto"
)

const MaxNativeResolutionTransactionBytes = 4096

// Prototype review ceiling only; not an approved mainnet fee or gas schedule.
const MaxNativeResolutionFeeSmallestUnits int64 = 10_000_000_000_000_000

var ErrNativeResolutionTransaction = errors.New("offline native return transaction rejected; no signing, submission or state authentication")

// Every value is explicit operator input, NOT fresh authenticated account or
// fee-market state. No nonce, gas estimate, budget or network default is chosen.
type NativeResolutionTransactionPlan struct {
	AccountNumber, Sequence, Gas      uint64
	Fee, MinFeePerGas, MaximumFeeCost sdkmath.Int
}

func (NativeResolutionTransactionPlan) String() string {
	return "private native return transaction plan (redacted)"
}
func (p NativeResolutionTransactionPlan) GoString() string { return p.String() }
func (NativeResolutionTransactionPlan) MarshalJSON() ([]byte, error) {
	return []byte(`{"redacted":true}`), nil
}

type NativeResolutionTransactionReport struct {
	Message                            NativeResolutionMessageReport `json:"message"`
	SignedEnvelopeMatched              bool                          `json:"signedEnvelopeMatched"`
	SenderSignatureChecked             bool                          `json:"senderSignatureChecked"`
	ExactAccountSequenceAndFeesMatched bool                          `json:"exactAccountSequenceAndFeesMatched"`
	WithinSuppliedFeeBudget            bool                          `json:"withinSuppliedFeeBudget"`
	NativeAccountStateAuthenticated    bool                          `json:"nativeAccountStateAuthenticated"`
	FeeMarketVerified                  bool                          `json:"feeMarketVerified"`
	IndependentSourceFinalityVerified  bool                          `json:"independentSourceFinalityVerified"`
	SigningEnabled                     bool                          `json:"signingEnabled"`
	BroadcastEnabled                   bool                          `json:"broadcastEnabled"`
	RealAssetsReady                    bool                          `json:"realAssetsReady"`
}

// Detached, already-signed bearer bytes. They are intentionally not exposed by
// this review object; a separate simulation attempt must retain before handoff.
// This object owns no key, provider, sender, sequence allocator or financial API.
type NativeResolutionTransaction struct {
	binding                               godbridge.Config
	request                               NativeResolutionRequest
	plan                                  NativeResolutionTransactionPlan
	raw                                   []byte
	hash, bindingPin, requestPin, planPin [32]byte
	report                                NativeResolutionTransactionReport
}

func (NativeResolutionTransaction) String() string {
	return "private reviewed native return transaction (redacted)"
}
func (t NativeResolutionTransaction) GoString() string                          { return t.String() }
func (t NativeResolutionTransaction) MarshalJSON() ([]byte, error)              { return json.Marshal(t.Report()) }
func (t NativeResolutionTransaction) Report() NativeResolutionTransactionReport { return t.report }

// Explicit private status-query identity, not a receipt, inclusion proof or
// permission to resubmit. Ordinary reports never expose this hash.
func (t NativeResolutionTransaction) TransactionID() ([32]byte, error) {
	if !t.report.SignedEnvelopeMatched {
		return [32]byte{}, ErrNativeResolutionTransaction
	}
	return t.hash, nil
}

func validNativeResolutionPlan(p NativeResolutionTransactionPlan) bool {
	limit := sdkmath.NewInt(MaxNativeResolutionFeeSmallestUnits)
	return p.Sequence != math.MaxUint64 && p.Gas >= 50_000 && p.Gas <= 2_000_000 &&
		!p.Fee.IsNil() && !p.MinFeePerGas.IsNil() && !p.MaximumFeeCost.IsNil() &&
		p.Fee.IsPositive() && p.MinFeePerGas.IsPositive() && p.MaximumFeeCost.IsPositive() &&
		p.MaximumFeeCost.LTE(limit) && p.Fee.LTE(p.MaximumFeeCost) &&
		new(big.Int).Mul(p.MinFeePerGas.BigInt(), new(big.Int).SetUint64(p.Gas)).Cmp(p.Fee.BigInt()) <= 0
}

func nativeResolutionPins(c godbridge.Config, r NativeResolutionRequest, p NativeResolutionTransactionPlan) (binding, request, plan [32]byte, err error) {
	c, err = godbridge.CanonicalConfig(c)
	if err != nil || !validNativeResolutionPlan(p) {
		return binding, request, plan, ErrNativeResolutionTransaction
	}
	b, err := json.Marshal(c)
	if err != nil {
		return binding, request, plan, ErrNativeResolutionTransaction
	}
	binding = sha256.Sum256(b)
	// Explicit PRIVATE serialization bypasses the ordinary redacted request.
	b, err = json.Marshal(struct {
		Sender     string
		Withdrawal godbridge.Withdrawal
		Outcome    godbridge.Status
		Evidence   godbridge.Evidence
		Digest     [32]byte
		Approvals  [][]byte
	}{r.Sender, r.Withdrawal, r.Outcome, r.Evidence, r.AttestationDigest, r.Approvals})
	if err != nil {
		return binding, request, plan, ErrNativeResolutionTransaction
	}
	request = sha256.Sum256(b)
	b, err = json.Marshal(struct{ AccountNumber, Sequence, Gas, Fee, MinFeePerGas, MaximumFeeCost string }{
		strconv.FormatUint(p.AccountNumber, 10), strconv.FormatUint(p.Sequence, 10), strconv.FormatUint(p.Gas, 10),
		p.Fee.String(), p.MinFeePerGas.String(), p.MaximumFeeCost.String()})
	if err != nil {
		return binding, request, plan, ErrNativeResolutionTransaction
	}
	plan = sha256.Sum256(b)
	return binding, request, plan, nil
}

// Offline verification ONLY. Accepts exactly one previously prepared bridge
// resolution message in one canonical SIGN_MODE_DIRECT envelope. It binds the
// supplied chain/account number/sequence/fee budget and verifies sender key,
// low-S signature AND recovery bit. No signer, account read or RPC is installed.
func ReviewNativeResolutionTransaction(config godbridge.Config, request NativeResolutionRequest,
	plan NativeResolutionTransactionPlan, wire []byte) (out NativeResolutionTransaction, err error) {
	defer func() {
		if recover() != nil {
			out, err = NativeResolutionTransaction{}, ErrNativeResolutionTransaction
		}
	}()
	fail := func() (NativeResolutionTransaction, error) {
		return NativeResolutionTransaction{}, ErrNativeResolutionTransaction
	}
	if len(wire) == 0 || len(wire) > MaxNativeResolutionTransactionBytes || !validNativeResolutionPlan(plan) {
		return fail()
	}
	prepared, err := PrepareNativeResolutionMessage(config, request)
	if err != nil {
		return fail()
	}
	clean, err := prepared.Payload()
	if err != nil {
		return fail()
	}
	config, err = godbridge.CanonicalConfig(config)
	if err != nil {
		return fail()
	}
	e, err := godtx.NewBridgeEncoding()
	if err != nil {
		return fail()
	}
	wire = bytes.Clone(wire)
	decoded, err := e.Decoder(MaxNativeResolutionTransactionBytes)(wire)
	if err != nil {
		return fail()
	}
	tx, ok := decoded.(authsigning.Tx)
	provider, has := decoded.(interface{ GetProtoTx() *txproto.Tx })
	if !ok || !has {
		return fail()
	}
	proto := provider.GetProtoTx()
	if len(tx.GetMsgs()) != 1 || proto.Body.Memo != "" || proto.Body.TimeoutHeight != 0 || proto.Body.Unordered ||
		proto.Body.TimeoutTimestamp != nil || len(proto.Body.ExtensionOptions) != 0 || len(proto.Body.NonCriticalExtensionOptions) != 0 ||
		proto.AuthInfo.Tip != nil || proto.AuthInfo.Fee.Payer != "" || proto.AuthInfo.Fee.Granter != "" || tx.GetGas() != plan.Gas {
		return fail()
	}
	fee := tx.GetFee()
	if len(fee) != 1 || fee[0].Denom != godrewards.GodDenom || !fee[0].Amount.Equal(plan.Fee) {
		return fail()
	}
	sigs, err := tx.GetSignaturesV2()
	if err != nil || len(sigs) != 1 || sigs[0].Sequence != plan.Sequence {
		return fail()
	}
	sig := sigs[0]
	pub, public := sig.PubKey.(*ethsecp256k1.PubKey)
	data, single := sig.Data.(*signing.SingleSignatureData)
	if !public || pub == nil || len(pub.Key) != 33 || !single || data == nil ||
		data.SignMode != signing.SignMode_SIGN_MODE_DIRECT || len(data.Signature) != 65 {
		return fail()
	}
	key, err := crypto.DecompressPubkey(pub.Key)
	owner, addressErr := godaddress.FromNative(request.Sender)
	if err != nil || addressErr != nil || !bytes.Equal(crypto.PubkeyToAddress(*key).Bytes(), owner) {
		return fail()
	}
	s := data.Signature
	if !crypto.ValidateSignatureValues(s[64], new(big.Int).SetBytes(s[:32]), new(big.Int).SetBytes(s[32:64]), true) {
		return fail()
	}
	b := e.TxConfig.NewTxBuilder()
	if b.SetMsgs(clean) != nil {
		return fail()
	}
	b.SetGasLimit(plan.Gas)
	b.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, sdkmath.NewIntFromBigInt(plan.Fee.BigInt()))))
	if b.SetSignatures(sig) != nil {
		return fail()
	}
	exact, err := e.TxConfig.TxEncoder()(b.GetTx())
	if err != nil || !bytes.Equal(exact, wire) {
		return fail()
	}
	signBytes, err := authsigning.GetSignBytesAdapter(context.Background(), e.TxConfig.SignModeHandler(), signing.SignMode_SIGN_MODE_DIRECT,
		authsigning.SignerData{Address: request.Sender, ChainID: config.NativeChain, AccountNumber: plan.AccountNumber, Sequence: plan.Sequence, PubKey: pub}, b.GetTx())
	if err != nil || !pub.VerifySignature(signBytes, s) {
		return fail()
	}
	recovered, err := crypto.SigToPub(crypto.Keccak256(signBytes), s)
	if err != nil || !bytes.Equal(crypto.CompressPubkey(recovered), pub.Key) {
		return fail()
	}
	request.Withdrawal.Amount = sdkmath.NewIntFromBigInt(request.Withdrawal.Amount.BigInt())
	request.Approvals = cloneResolutionApprovals(request.Approvals)
	plan.Fee, plan.MinFeePerGas, plan.MaximumFeeCost = sdkmath.NewIntFromBigInt(plan.Fee.BigInt()),
		sdkmath.NewIntFromBigInt(plan.MinFeePerGas.BigInt()), sdkmath.NewIntFromBigInt(plan.MaximumFeeCost.BigInt())
	bindingPin, requestPin, planPin, err := nativeResolutionPins(config, request, plan)
	if err != nil {
		return fail()
	}
	return NativeResolutionTransaction{binding: config, request: request, plan: plan, raw: wire, hash: sha256.Sum256(wire),
		bindingPin: bindingPin, requestPin: requestPin, planPin: planPin,
		report: NativeResolutionTransactionReport{Message: prepared.Report(), SignedEnvelopeMatched: true,
			SenderSignatureChecked: true, ExactAccountSequenceAndFeesMatched: true, WithinSuppliedFeeBudget: true}}, nil
}
