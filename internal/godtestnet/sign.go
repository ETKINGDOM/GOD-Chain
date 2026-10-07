//go:build go1.25

package godtestnet

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strconv"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtx"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	rewardmsg "github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	cmttypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/ethereum/go-ethereum/crypto"
)

// SignRequest is private offline input. Review chain identity, operation,
// amounts, fees and freshly queried account metadata before using a disposable
// test wallet. This tool never queries RPC, sends funds or signs bridge actions.
type SignRequest struct {
	Version           uint32 `json:"version"`
	Mode              string `json:"mode"`
	ChainID           string `json:"chainId"`
	AccountNumber     string `json:"accountNumber"`
	Sequence          string `json:"sequence"`
	Gas               string `json:"gas"`
	Fee               string `json:"feeSmallestUnits"`
	Action            string `json:"action"`
	Validator         string `json:"validator"`
	Recipient         string `json:"recipient"`
	Amount            string `json:"amountSmallestUnits"`
	MinGodOut         string `json:"minGodOutSmallestUnits"`
	DeadlineUnixNanos string `json:"deadlineUnixNanos"`
}
type SignedTransaction struct {
	Version uint32 `json:"version"`
	Mode    string `json:"mode"`
	ChainID string `json:"chainId"`
	Hash    string `json:"hash"`
	Wire    string `json:"wire"`
}
type SignReport struct {
	Synthetic  bool `json:"synthetic"`
	RealAssets bool `json:"realAssets"`
	Signed     bool `json:"signed"`
	Submitted  bool `json:"submitted"`
}

func decimalUint(text string) (uint64, error) {
	n, err := strconv.ParseUint(text, 10, 64)
	if err != nil || strconv.FormatUint(n, 10) != text {
		return 0, ErrConfig
	}
	return n, nil
}

func nativeAction(q SignRequest, owner string) (sdk.Msg, error) {
	if q.Action != "redeem-g" && (q.MinGodOut != "" || q.DeadlineUnixNanos != "") {
		return nil, ErrConfig
	}
	if q.Action != "delegate" && q.Action != "undelegate" && q.Validator != "" {
		return nil, ErrConfig
	}
	if q.Action != "transfer-g" && q.Action != "redeem-g" && q.Recipient != "" {
		return nil, ErrConfig
	}
	if q.Action == "claim-g" {
		if q.Amount != "" {
			return nil, ErrConfig
		}
		return &rewardmsg.MsgClaimG{Sender: owner}, nil
	}
	amount, err := rewardmsg.Amount(q.Amount)
	if err != nil {
		return nil, ErrConfig
	}
	var m sdk.Msg
	switch q.Action {
	case "delegate", "undelegate":
		if q.Action == "delegate" && amount.LT(godrewards.Unit()) {
			return nil, ErrConfig
		}
		address, err := sdk.ValAddressFromBech32(q.Validator)
		if err != nil || address.String() != q.Validator {
			return nil, ErrConfig
		}
		coin := sdk.NewCoin(godrewards.GodDenom, amount)
		if q.Action == "delegate" {
			return &stakingtypes.MsgDelegate{DelegatorAddress: owner, ValidatorAddress: q.Validator, Amount: coin}, nil
		}
		return &stakingtypes.MsgUndelegate{DelegatorAddress: owner, ValidatorAddress: q.Validator, Amount: coin}, nil
	case "transfer-g":
		m = &rewardmsg.MsgTransferG{Sender: owner, Recipient: q.Recipient, Amount: q.Amount}
	case "donate-god":
		m = &rewardmsg.MsgDonateGod{Sender: owner, Amount: q.Amount}
	case "redeem-g":
		if amount.LT(godrewards.Unit()) {
			return nil, ErrConfig
		}
		deadline, err := decimalUint(q.DeadlineUnixNanos)
		if err != nil || deadline > uint64(^uint64(0)>>1) {
			return nil, ErrConfig
		}
		m = &rewardmsg.MsgRedeemG{Sender: owner, Beneficiary: q.Recipient, Amount: q.Amount, MinGodOut: q.MinGodOut, DeadlineUnixNanos: int64(deadline)}
	default:
		return nil, ErrConfig
	}
	if rewardmsg.Validate(m) != nil {
		return nil, ErrConfig
	}
	return m, nil
}

// SignNative uses a separately held binary 32-byte disposable test wallet key.
// It creates a new owner-only output, never overwrites one, and never emits key,
// address, wire or path values in its report. Signing and submission are separate.
func SignNative(requestPath, keyPath, outputPath, bundlePath, expectedBundle string) (SignReport, error) {
	b, err := loadBundle(bundlePath, expectedBundle)
	if err != nil {
		return SignReport{}, err
	}
	var q SignRequest
	if readDocument(requestPath, 16<<10, &q) != nil || q.Version != 1 || q.Mode != "synthetic" || q.ChainID != b.Runtime.ChainID {
		return SignReport{}, ErrConfig
	}
	if !filepath.IsAbs(keyPath) || !filepath.IsAbs(outputPath) {
		return SignReport{}, ErrPrivate
	}
	r, err := openPrivate(filepath.Dir(keyPath))
	if err != nil {
		return SignReport{}, err
	}
	keyBytes, err := readPrivate(r, filepath.Base(keyPath), 32)
	_ = r.Close()
	defer clear(keyBytes)
	if err != nil {
		return SignReport{}, ErrPrivate
	}
	result, err := signNativeBytes(b, q, keyBytes)
	if err != nil {
		return SignReport{}, err
	}
	return writeSigned(outputPath, result)
}

// Shared offline signing keeps encrypted-wallet keys in memory, never in a
// temporary plaintext file. Both callers retain the same native allowlist.
func signNativeBytes(b Bundle, q SignRequest, keyBytes []byte) (SignedTransaction, error) {
	if q.Version != 1 || q.Mode != "synthetic" || q.ChainID != b.Runtime.ChainID {
		return SignedTransaction{}, ErrConfig
	}
	number, err := decimalUint(q.AccountNumber)
	if err != nil {
		return SignedTransaction{}, err
	}
	sequence, err := decimalUint(q.Sequence)
	if err != nil {
		return SignedTransaction{}, err
	}
	gas, err := decimalUint(q.Gas)
	if err != nil || gas == 0 || gas > b.Runtime.Policy.MaxGas {
		return SignedTransaction{}, ErrConfig
	}
	fee, err := rewardmsg.Amount(q.Fee)
	if err != nil || fee.LT(b.Runtime.Policy.MinFeePerGas.MulRaw(int64(gas))) {
		return SignedTransaction{}, ErrConfig
	}
	keyCheck, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		return SignedTransaction{}, ErrPrivate
	}
	keyCheck.D.SetInt64(0)
	key := &ethsecp256k1.PrivKey{Key: keyBytes}
	owner, err := godaddress.ToNative(key.PubKey().Address())
	if err != nil {
		return SignedTransaction{}, ErrPrivate
	}
	message, err := nativeAction(q, owner)
	if err != nil {
		return SignedTransaction{}, err
	}
	encoding, err := godtx.NewExecutionEncoding()
	if err != nil {
		return SignedTransaction{}, ErrConfig
	}
	builder := encoding.TxConfig.NewTxBuilder()
	if builder.SetMsgs(message) != nil {
		return SignedTransaction{}, ErrConfig
	}
	builder.SetGasLimit(gas)
	builder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, fee)))
	signature := signing.SignatureV2{PubKey: key.PubKey(), Sequence: sequence, Data: &signing.SingleSignatureData{SignMode: signing.SignMode_SIGN_MODE_DIRECT}}
	if builder.SetSignatures(signature) != nil {
		return SignedTransaction{}, ErrConfig
	}
	data, err := authsigning.GetSignBytesAdapter(context.Background(), encoding.TxConfig.SignModeHandler(), signing.SignMode_SIGN_MODE_DIRECT, authsigning.SignerData{Address: owner, ChainID: q.ChainID, AccountNumber: number, Sequence: sequence, PubKey: key.PubKey()}, builder.GetTx())
	if err != nil {
		return SignedTransaction{}, ErrConfig
	}
	signed, err := key.Sign(data)
	if err != nil {
		return SignedTransaction{}, ErrPrivate
	}
	signature.Data = &signing.SingleSignatureData{SignMode: signing.SignMode_SIGN_MODE_DIRECT, Signature: signed}
	if builder.SetSignatures(signature) != nil {
		return SignedTransaction{}, ErrConfig
	}
	wire, err := encoding.TxConfig.TxEncoder()(builder.GetTx())
	if err != nil || len(wire) > b.Runtime.Policy.MaxTxBytes {
		return SignedTransaction{}, ErrConfig
	}
	return SignedTransaction{Version: 1, Mode: "synthetic", ChainID: q.ChainID, Hash: "0x" + hex.EncodeToString(cmttypes.Tx(wire).Hash()), Wire: "0x" + hex.EncodeToString(wire)}, nil
}

func writeSigned(outputPath string, result any) (SignReport, error) {
	if !filepath.IsAbs(outputPath) {
		return SignReport{}, ErrPrivate
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return SignReport{}, ErrConfig
	}
	out, err := openPrivate(filepath.Dir(outputPath))
	if err != nil {
		return SignReport{}, err
	}
	defer out.Close()
	if writePrivate(out, filepath.Base(outputPath), raw) != nil {
		return SignReport{}, ErrPrivate
	}
	return SignReport{Synthetic: true, Signed: true}, nil
}
