//go:build go1.25

package godtestnet

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strconv"

	sdkmath "cosmossdk.io/math"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtx"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	rewardmsg "github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	cmted "github.com/cometbft/cometbft/crypto/ed25519"
	cmttypes "github.com/cometbft/cometbft/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdked "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
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
	keyCheck, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		return SignedTransaction{}, ErrPrivate
	}
	keyCheck.D.SetInt64(0)
	owner, err := godaddress.ToNative((&ethsecp256k1.PrivKey{Key: keyBytes}).PubKey().Address())
	if err != nil {
		return SignedTransaction{}, ErrPrivate
	}
	message, err := nativeAction(q, owner)
	if err != nil {
		return SignedTransaction{}, err
	}
	return signDirectMessage(b, q, keyBytes, message)
}

// Registration has its own explicit offline command. It never extends the six
// browser/gateway actions or lets arbitrary SDK messages reach those signers.
func signDirectMessage(b Bundle, q SignRequest, keyBytes []byte, message sdk.Msg) (SignedTransaction, error) {
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

// RegistrationRequest is private, portable offline input. A local node-key
// proof binds the complete request to its pinned chain, owner and amounts; the
// separately held account wallet then authenticates the standard staking tx.
// A separate compact consensus proof travels on-chain in standard staking
// description details. The immutable runtime must enforce that proof too.
type RegistrationRequest struct {
	Version            uint32 `json:"version"`
	Mode               string `json:"mode"`
	ChainID            string `json:"chainId"`
	GenesisSHA256      string `json:"genesisSha256"`
	Owner              string `json:"owner"`
	ConsensusPublicKey []byte `json:"consensusPublicKey"`
	AccountNumber      string `json:"accountNumber"`
	Sequence           string `json:"sequence"`
	Gas                string `json:"gas"`
	Fee                string `json:"feeSmallestUnits"`
	Stake              string `json:"stakeSmallestUnits"`
	ConsensusProof     []byte `json:"consensusProof"`
	NodeProof          []byte `json:"nodeProof"`
}

func registrationBytes(q RegistrationRequest) ([]byte, error) {
	q.NodeProof = nil
	raw, err := json.Marshal(q)
	return append([]byte("GOD Chain synthetic candidate registration v1\n"), raw...), err
}

func registrationMessage(b Bundle, q RegistrationRequest) (sdk.Msg, error) {
	owner, err := godaddress.FromNative(q.Owner)
	if err != nil || !b.Runtime.RequireValidatorProof || q.Version != 1 || q.Mode != "synthetic" || q.ChainID != b.Runtime.ChainID || q.GenesisSHA256 != b.GenesisSHA256 || len(q.ConsensusPublicKey) != 32 {
		return nil, ErrConfig
	}
	for _, p := range b.Profiles {
		if bytes.Equal(p.ConsensusPublicKey, q.ConsensusPublicKey) || bytes.Equal(p.PeerPublicKey, q.ConsensusPublicKey) || p.Owner == q.Owner {
			return nil, ErrConfig
		}
	}
	for _, text := range []string{q.AccountNumber, q.Sequence} {
		if _, err := decimalUint(text); err != nil {
			return nil, ErrConfig
		}
	}
	if q.Sequence == "18446744073709551615" {
		return nil, ErrConfig
	}
	gas, err := decimalUint(q.Gas)
	fee, e := rewardmsg.Amount(q.Fee)
	stake, f := rewardmsg.Amount(q.Stake)
	if err != nil || gas == 0 || gas > b.Runtime.Policy.MaxGas || e != nil || f != nil || fee.LT(b.Runtime.Policy.MinFeePerGas.MulRaw(int64(gas))) || fee.GT(godrewards.Unit().QuoRaw(1000)) || stake.LT(godrewards.Unit().MulRaw(1000)) || stake.Add(fee).GT(godrewards.FixedGodSupply()) {
		return nil, ErrConfig
	}
	pub, err := codectypes.NewAnyWithValue(&sdked.PubKey{Key: q.ConsensusPublicKey})
	if err != nil {
		return nil, ErrConfig
	}
	rate := sdkmath.LegacyNewDecWithPrec(1, 1)
	operator, err := addresscodec.NewBech32Codec(godaddress.ValidatorOperatorPrefix).BytesToString(owner)
	if err != nil {
		return nil, ErrConfig
	}
	m := &stakingtypes.MsgCreateValidator{ValidatorAddress: operator, Pubkey: pub,
		Value: sdk.NewCoin(godrewards.GodDenom, stake), MinSelfDelegation: godrewards.Unit().MulRaw(1000),
		Description: stakingtypes.Description{Moniker: "GOD Chain candidate"}, Commission: stakingtypes.NewCommissionRates(rate, rate, sdkmath.LegacyZeroDec())}
	if m.Validate(addresscodec.NewBech32Codec(godaddress.ValidatorOperatorPrefix)) != nil {
		return nil, ErrConfig
	}
	if len(q.ConsensusProof) != 0 && godnode.AttachValidatorProof(b.Runtime, m, q.ConsensusProof) != nil {
		return nil, ErrConfig
	}
	return m, nil
}

// PrepareRegistration writes a new owner-only request, using the candidate's
// own consensus key only for a domain-separated proof, never a vote/proposal.
// Metadata must be freshly reviewed by the operator; no RPC request is made.
func PrepareRegistration(home, bundlePath, expectedBundle, output string, q RegistrationRequest) (SignReport, error) {
	b, err := loadBundle(bundlePath, expectedBundle)
	if err != nil {
		return SignReport{}, err
	}
	loaded, err := Load(home)
	if err != nil || loaded.Document.Role != "candidate" {
		return SignReport{}, ErrConfig
	}
	left, _ := json.Marshal(b.Runtime)
	right, _ := json.Marshal(loaded.Document.Runtime)
	if !bytes.Equal(left, right) || b.GenesisSHA256 != loaded.Document.GenesisSHA256 || q.Version != 0 || q.Mode != "" || q.ChainID != "" || q.GenesisSHA256 != "" || q.Owner != "" || len(q.ConsensusPublicKey) != 0 || len(q.NodeProof) != 0 || len(q.ConsensusProof) != 0 {
		return SignReport{}, ErrConfig
	}
	q.Version, q.Mode, q.ChainID, q.GenesisSHA256 = 1, "synthetic", b.Runtime.ChainID, b.GenesisSHA256
	q.Owner, q.ConsensusPublicKey = loaded.Document.CandidateOwner, loaded.Document.ConsensusPublicKey
	m, err := registrationMessage(b, q)
	if err != nil {
		return SignReport{}, err
	}
	intent, err := godnode.ValidatorProofBytes(b.Runtime, m.(*stakingtypes.MsgCreateValidator))
	if err != nil {
		return SignReport{}, ErrConfig
	}
	q.ConsensusProof, err = loaded.Signer.Key.PrivKey.Sign(intent)
	if err != nil {
		return SignReport{}, ErrPrivate
	}
	raw, err := registrationBytes(q)
	if err != nil {
		return SignReport{}, ErrConfig
	}
	q.NodeProof, err = loaded.Signer.Key.PrivKey.Sign(raw)
	if err != nil {
		return SignReport{}, ErrPrivate
	}
	// Reuse the no-overwrite private writer, but distinguish a node proof from a
	// wallet-signed transaction. Nothing is submitted or activated here.
	if _, err := writeSigned(output, q); err != nil {
		return SignReport{}, err
	}
	return SignReport{Synthetic: true}, nil
}

// SignRegistration requires BOTH the node-key proof and the exact owner wallet.
// The wallet key is separately held and is never stored in the node workspace.
func SignRegistration(requestPath, keyPath, outputPath, bundlePath, expectedBundle string) (SignReport, error) {
	b, err := loadBundle(bundlePath, expectedBundle)
	if err != nil {
		return SignReport{}, err
	}
	var q RegistrationRequest
	if readDocument(requestPath, 16<<10, &q) != nil {
		return SignReport{}, ErrConfig
	}
	if _, err := registrationIntent(b, q); err != nil {
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
	result, err := signRegistrationBytes(b, q, keyBytes)
	if err != nil {
		return SignReport{}, err
	}
	return writeSigned(outputPath, result)
}

func registrationIntent(b Bundle, q RegistrationRequest) (sdk.Msg, error) {
	m, err := registrationMessage(b, q)
	raw, e := registrationBytes(q)
	if err != nil || e != nil || len(q.ConsensusProof) != 64 || godnode.ValidateValidatorProof(b.Runtime, m.(*stakingtypes.MsgCreateValidator)) != nil || len(q.NodeProof) != 64 || !cmted.PubKey(q.ConsensusPublicKey).VerifySignature(raw, q.NodeProof) {
		return nil, ErrConfig
	}
	return m, nil
}

func signRegistrationBytes(b Bundle, q RegistrationRequest, keyBytes []byte) (SignedTransaction, error) {
	m, err := registrationIntent(b, q)
	if err != nil {
		return SignedTransaction{}, err
	}
	key, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		return SignedTransaction{}, ErrPrivate
	}
	owner, err := godaddress.ToNative(crypto.PubkeyToAddress(key.PublicKey).Bytes())
	key.D.SetInt64(0)
	if err != nil || owner != q.Owner {
		return SignedTransaction{}, ErrConfig
	}
	return signDirectMessage(b, SignRequest{Version: q.Version, Mode: q.Mode, ChainID: q.ChainID, AccountNumber: q.AccountNumber, Sequence: q.Sequence, Gas: q.Gas, Fee: q.Fee}, keyBytes, m)
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
