// Package godnative restricts signed native operations before admission. It
// holds no private keys, performs no RPC, and does not change consensus rules.
package godnative

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"regexp"
	"strconv"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtx"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	rewardmsg "github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txproto "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/ethereum/go-ethereum/crypto"
)

const MaxWireBytes = 2048

var ErrIntent = errors.New("native operation rejected")
var chainPattern = regexp.MustCompile(`^god-test-[a-f0-9]{32}$`)
var uintText = regexp.MustCompile(`^(0|[1-9][0-9]*)$`)
var maxG = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
var fixedGod = godrewards.FixedGodSupply().BigInt()
var maxFee = big.NewInt(10_000_000_000_000_000)

type Policy struct {
	ChainID      string
	Gas          uint64
	MinFeePerGas string
}

// Intent contains offered values, not charged fees or final staking/pool state.
type Intent struct {
	Operation string `json:"operation"`
	Sender    string `json:"sender"`
	Validator string `json:"validator,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Amount    string `json:"amountSmallestUnits,omitempty"`
	MinGodOut string `json:"minGodOutSmallestUnits,omitempty"`
	Deadline  string `json:"deadlineUnixNanos,omitempty"`
	Sequence  string `json:"sequence,omitempty"`
	Gas       string `json:"gas,omitempty"`
	Fee       string `json:"feeSmallestUnits,omitempty"`
}

type Codec struct {
	encoding godtx.Encoding
	policy   Policy
}
type Decoded struct {
	Intent Intent
	Hash   string
	tx     authsigning.Tx
	sig    signing.SignatureV2
}

func Integer(s string, max *big.Int, positive bool) (*big.Int, error) {
	if len(s) > 78 || !uintText.MatchString(s) {
		return nil, ErrIntent
	}
	n, ok := new(big.Int).SetString(s, 10)
	if !ok || positive && n.Sign() <= 0 || n.Cmp(max) > 0 {
		return nil, ErrIntent
	}
	return n, nil
}

func New(p Policy) (*Codec, error) {
	n, err := Integer(p.MinFeePerGas, maxFee, true)
	if err != nil || !chainPattern.MatchString(p.ChainID) || p.Gas < 50_000 || p.Gas > 2_000_000 || new(big.Int).Mul(n, new(big.Int).SetUint64(p.Gas)).Cmp(maxFee) > 0 {
		return nil, ErrIntent
	}
	e, err := godtx.NewExecutionEncoding()
	if err != nil {
		return nil, ErrIntent
	}
	return &Codec{e, p}, nil
}

func Account(s string) ([]byte, error) {
	b, err := godaddress.FromNative(s)
	if err != nil || len(b) != 20 || bytes.Equal(b, make([]byte, 20)) {
		return nil, ErrIntent
	}
	c, err := godaddress.ToNative(b)
	if err != nil || c != s {
		return nil, ErrIntent
	}
	return b, nil
}

func validator(s string) error {
	b, err := sdk.GetFromBech32(s, godaddress.ValidatorOperatorPrefix)
	if err != nil || len(b) != 20 || bytes.Equal(b, make([]byte, 20)) {
		return ErrIntent
	}
	c, err := sdk.Bech32ifyAddressBytes(godaddress.ValidatorOperatorPrefix, b)
	if err != nil || c != s {
		return ErrIntent
	}
	return nil
}

// Scope extracts only reviewed fields and creates a fresh message. Re-encoding
// this message rejects unknown protobuf fields, aliases and extra envelopes.
func Scope(m sdk.Msg) (Intent, sdk.Msg, error) {
	var i Intent
	var clean sdk.Msg
	switch x := m.(type) {
	case *stakingtypes.MsgDelegate:
		if x == nil || x.Amount.Amount.IsNil() {
			return i, nil, ErrIntent
		}
		i = Intent{Operation: "delegate", Sender: x.DelegatorAddress, Validator: x.ValidatorAddress, Amount: x.Amount.Amount.String()}
		if x.Amount.Denom != "agod" {
			return i, nil, ErrIntent
		}
		clean = &stakingtypes.MsgDelegate{DelegatorAddress: i.Sender, ValidatorAddress: i.Validator, Amount: x.Amount}
	case *stakingtypes.MsgUndelegate:
		if x == nil || x.Amount.Amount.IsNil() {
			return i, nil, ErrIntent
		}
		i = Intent{Operation: "undelegate", Sender: x.DelegatorAddress, Validator: x.ValidatorAddress, Amount: x.Amount.Amount.String()}
		if x.Amount.Denom != "agod" {
			return i, nil, ErrIntent
		}
		clean = &stakingtypes.MsgUndelegate{DelegatorAddress: i.Sender, ValidatorAddress: i.Validator, Amount: x.Amount}
	case *rewardmsg.MsgClaimG:
		if x == nil {
			return i, nil, ErrIntent
		}
		i = Intent{Operation: "claim-g", Sender: x.Sender}
		clean = &rewardmsg.MsgClaimG{Sender: i.Sender}
	case *rewardmsg.MsgTransferG:
		if x == nil {
			return i, nil, ErrIntent
		}
		i = Intent{Operation: "transfer-g", Sender: x.Sender, Recipient: x.Recipient, Amount: x.Amount}
		clean = &rewardmsg.MsgTransferG{Sender: i.Sender, Recipient: i.Recipient, Amount: i.Amount}
	case *rewardmsg.MsgDonateGod:
		if x == nil {
			return i, nil, ErrIntent
		}
		i = Intent{Operation: "donate-god", Sender: x.Sender, Amount: x.Amount}
		clean = &rewardmsg.MsgDonateGod{Sender: i.Sender, Amount: i.Amount}
	case *rewardmsg.MsgRedeemG:
		if x == nil {
			return i, nil, ErrIntent
		}
		i = Intent{Operation: "redeem-g", Sender: x.Sender, Recipient: x.Beneficiary, Amount: x.Amount, MinGodOut: x.MinGodOut, Deadline: strconv.FormatInt(x.DeadlineUnixNanos, 10)}
		clean = &rewardmsg.MsgRedeemG{Sender: i.Sender, Beneficiary: i.Recipient, Amount: i.Amount, MinGodOut: i.MinGodOut, DeadlineUnixNanos: x.DeadlineUnixNanos}
	default:
		return i, nil, ErrIntent
	}
	if Validate(i) != nil {
		return i, nil, ErrIntent
	}
	return i, clean, nil
}

// Validate also validates a committed operation summary without signing metadata.
func Validate(i Intent) error {
	if _, err := Account(i.Sender); err != nil {
		return ErrIntent
	}
	switch i.Operation {
	case "delegate", "undelegate":
		if validator(i.Validator) != nil || i.Recipient != "" || i.MinGodOut != "" || i.Deadline != "" {
			return ErrIntent
		}
		n, err := Integer(i.Amount, fixedGod, true)
		if err != nil || n.Cmp(godrewards.Unit().BigInt()) < 0 {
			return ErrIntent
		}
	case "claim-g":
		if i.Validator != "" || i.Recipient != "" || i.Amount != "" || i.MinGodOut != "" || i.Deadline != "" {
			return ErrIntent
		}
	case "transfer-g", "redeem-g":
		if _, err := Account(i.Recipient); err != nil || i.Validator != "" {
			return ErrIntent
		}
		n, err := Integer(i.Amount, maxG, true)
		if err != nil {
			return ErrIntent
		}
		if i.Operation == "transfer-g" {
			if i.Sender == i.Recipient || i.MinGodOut != "" || i.Deadline != "" {
				return ErrIntent
			}
		} else {
			if n.Cmp(godrewards.Unit().BigInt()) < 0 {
				return ErrIntent
			}
			if _, err := Integer(i.MinGodOut, fixedGod, true); err != nil {
				return ErrIntent
			}
			if _, err := Integer(i.Deadline, big.NewInt(1<<63-1), true); err != nil {
				return ErrIntent
			}
		}
	case "donate-god":
		if i.Validator != "" || i.Recipient != "" || i.MinGodOut != "" || i.Deadline != "" {
			return ErrIntent
		}
		if _, err := Integer(i.Amount, fixedGod, true); err != nil {
			return ErrIntent
		}
	default:
		return ErrIntent
	}
	return nil
}

func (c *Codec) Decode(wire []byte) (*Decoded, error) {
	t, err := c.encoding.Decoder(MaxWireBytes)(wire)
	if err != nil {
		return nil, ErrIntent
	}
	tx, ok := t.(authsigning.Tx)
	if !ok {
		return nil, ErrIntent
	}
	p, ok := t.(interface{ GetProtoTx() *txproto.Tx })
	if !ok {
		return nil, ErrIntent
	}
	proto := p.GetProtoTx()
	if len(tx.GetMsgs()) != 1 || proto.Body.Memo != "" || proto.Body.TimeoutHeight != 0 || proto.Body.Unordered || proto.Body.TimeoutTimestamp != nil || len(proto.Body.ExtensionOptions) != 0 || len(proto.Body.NonCriticalExtensionOptions) != 0 || proto.AuthInfo.Tip != nil || proto.AuthInfo.Fee.Payer != "" || proto.AuthInfo.Fee.Granter != "" {
		return nil, ErrIntent
	}
	i, m, err := Scope(tx.GetMsgs()[0])
	if err != nil {
		return nil, ErrIntent
	}
	fee := tx.GetFee()
	if tx.GetGas() != c.policy.Gas || len(fee) != 1 || fee[0].Denom != "agod" {
		return nil, ErrIntent
	}
	f, err := Integer(fee[0].Amount.String(), maxFee, true)
	min, _ := new(big.Int).SetString(c.policy.MinFeePerGas, 10)
	if err != nil || f.Cmp(new(big.Int).Mul(min, new(big.Int).SetUint64(c.policy.Gas))) < 0 {
		return nil, ErrIntent
	}
	sigs, err := tx.GetSignaturesV2()
	if err != nil || len(sigs) != 1 {
		return nil, ErrIntent
	}
	sig := sigs[0]
	pub, ok := sig.PubKey.(*ethsecp256k1.PubKey)
	data, single := sig.Data.(*signing.SingleSignatureData)
	owner, _ := Account(i.Sender)
	if !ok || len(pub.Key) != 33 || !single || data.SignMode != signing.SignMode_SIGN_MODE_DIRECT || len(data.Signature) != 65 || sig.Sequence == ^uint64(0) {
		return nil, ErrIntent
	}
	key, err := crypto.DecompressPubkey(pub.Key)
	if err != nil || !bytes.Equal(crypto.PubkeyToAddress(*key).Bytes(), owner) {
		return nil, ErrIntent
	}
	s := data.Signature
	if !crypto.ValidateSignatureValues(s[64], new(big.Int).SetBytes(s[:32]), new(big.Int).SetBytes(s[32:64]), true) {
		return nil, ErrIntent
	}
	b := c.encoding.TxConfig.NewTxBuilder()
	if b.SetMsgs(m) != nil {
		return nil, ErrIntent
	}
	b.SetGasLimit(c.policy.Gas)
	b.SetFeeAmount(sdk.NewCoins(sdk.NewCoin("agod", sdkmath.NewIntFromBigInt(f))))
	if b.SetSignatures(sig) != nil {
		return nil, ErrIntent
	}
	exact, err := c.encoding.TxConfig.TxEncoder()(b.GetTx())
	if err != nil || !bytes.Equal(exact, wire) {
		return nil, ErrIntent
	}
	h := sha256.Sum256(wire)
	i.Sequence = strconv.FormatUint(sig.Sequence, 10)
	i.Gas = strconv.FormatUint(c.policy.Gas, 10)
	i.Fee = f.String()
	return &Decoded{i, "0x" + hex.EncodeToString(h[:]), b.GetTx(), sig}, nil
}

// Verify binds the configured chain and freshly read account number, neither
// of which is included in TxRaw. A decode alone is not authentication.
func (c *Codec) Verify(d *Decoded, number uint64) error {
	if d == nil {
		return ErrIntent
	}
	pub := d.sig.PubKey.(*ethsecp256k1.PubKey)
	data := d.sig.Data.(*signing.SingleSignatureData)
	signer := authsigning.SignerData{Address: d.Intent.Sender, ChainID: c.policy.ChainID, AccountNumber: number, Sequence: d.sig.Sequence, PubKey: pub}
	b, err := authsigning.GetSignBytesAdapter(context.Background(), c.encoding.TxConfig.SignModeHandler(), signing.SignMode_SIGN_MODE_DIRECT, signer, d.tx)
	if err != nil || !pub.VerifySignature(b, data.Signature) {
		return ErrIntent
	}
	r, err := crypto.SigToPub(crypto.Keccak256(b), data.Signature)
	if err != nil || !bytes.Equal(crypto.CompressPubkey(r), pub.Key) {
		return ErrIntent
	}
	return nil
}
