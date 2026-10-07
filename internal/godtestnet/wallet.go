//go:build go1.25

package godtestnet

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	rewardmsg "github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/scrypt"
)

var ErrWallet = errors.New("synthetic test wallet operation rejected")

// Version 1 fixes all cryptographic parameters; untrusted files cannot select
// weaker or excessive work factors. Standard library AES-256-GCM authenticates
// the encrypted scalar AND the full reviewed bundle digest, with a fresh salt
// and nonce. Pinned x/crypto scrypt uses N=262144, r=8, p=1, dkLen=32.
type walletVault struct {
	Version    uint32 `json:"version"`
	Mode       string `json:"mode"`
	KDF        string `json:"kdf"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type WalletAddress struct {
	Version       uint32 `json:"version"`
	Mode          string `json:"mode"`
	ChainID       string `json:"chainId"`
	EVMChainID    string `json:"evmChainId"`
	BundleSHA256  string `json:"bundleSha256"`
	NativeAddress string `json:"nativeAddress"`
	EVMAddress    string `json:"evmAddress"`
}

type WalletTransferRequest struct {
	Version      uint32 `json:"version"`
	Mode         string `json:"mode"`
	Kind         string `json:"kind"`
	ChainID      string `json:"chainId"`
	EVMChainID   string `json:"evmChainId"`
	BundleSHA256 string `json:"bundleSha256"`
	Sender       string `json:"sender"`
	Sequence     string `json:"sequence"`
	Recipient    string `json:"recipient"`
	Amount       string `json:"amountSmallestUnits"`
	GasPrice     string `json:"gasPriceSmallestUnits"`
}

// Review is delivered only to the caller's private terminal, not server logs.
// The explicit callback must approve the derived sender and exact signed fields.
type WalletReview struct {
	ChainID       string `json:"chainId"`
	EVMChainID    string `json:"evmChainId"`
	BundleSHA256  string `json:"bundleSha256"`
	Sender        string `json:"sender"`
	Operation     string `json:"operation"`
	Sequence      string `json:"sequence"`
	AccountNumber string `json:"accountNumber,omitempty"`
	Recipient     string `json:"recipient,omitempty"`
	Validator     string `json:"validator,omitempty"`
	Amount        string `json:"amountSmallestUnits,omitempty"`
	Gas           string `json:"gas"`
	Fee           string `json:"maximumFeeSmallestUnits"`
	MinGodOut     string `json:"minGodOutSmallestUnits,omitempty"`
	Deadline      string `json:"deadlineUnixNanos,omitempty"`
}

func walletPassword(p []byte) bool {
	return len(p) >= 12 && len(p) <= 256 && utf8.Valid(p) && len(bytes.TrimSpace(p)) == len(p) && !bytes.ContainsAny(p, "\r\n\x00")
}

func walletAEAD(password, salt []byte) (cipher.AEAD, error) {
	derived, err := scrypt.Key(password, salt, 1<<18, 8, 1, 32)
	if err != nil {
		return nil, ErrWallet
	}
	defer clear(derived)
	block, err := aes.NewCipher(derived)
	if err != nil {
		return nil, ErrWallet
	}
	return cipher.NewGCM(block)
}

func walletHex(text string, size int) ([]byte, error) {
	raw, err := hex.DecodeString(text)
	if err != nil || len(raw) != size || hex.EncodeToString(raw) != text {
		return nil, ErrWallet
	}
	return raw, nil
}

func walletWrite(path string, value any) error {
	if !filepath.IsAbs(path) {
		return ErrWallet
	}
	r, err := openPrivate(filepath.Dir(path))
	if err != nil {
		return ErrWallet
	}
	defer r.Close()
	raw, err := json.Marshal(value)
	if err != nil || writePrivate(r, filepath.Base(path), raw) != nil {
		return ErrWallet
	}
	return nil
}

// WalletCreate has no key-import, recovery phrase, raw export or RPC facility.
// Encrypted file plus password is the only backup; lost credentials are lost.
func WalletCreate(path, bundlePath, pin string, password []byte) error {
	if !walletPassword(password) {
		return ErrWallet
	}
	if _, err := loadBundle(bundlePath, pin); err != nil {
		return ErrWallet
	}
	if !filepath.IsAbs(path) {
		return ErrWallet
	}
	root, err := openPrivate(filepath.Dir(path))
	if err != nil {
		return ErrWallet
	}
	_, existing := root.Lstat(filepath.Base(path))
	_ = root.Close()
	if !os.IsNotExist(existing) {
		return ErrWallet
	}
	bound, err := walletHex(pin, 32)
	if err != nil {
		return ErrWallet
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		return ErrWallet
	}
	defer key.D.SetInt64(0)
	scalar := crypto.FromECDSA(key)
	defer clear(scalar)
	payload := make([]byte, 64)
	copy(payload, scalar)
	copy(payload[32:], bound)
	defer clear(payload)
	salt, nonce := make([]byte, 32), make([]byte, 12)
	if _, err := rand.Read(salt); err != nil {
		return ErrWallet
	}
	if _, err := rand.Read(nonce); err != nil {
		return ErrWallet
	}
	aead, err := walletAEAD(password, salt)
	if err != nil {
		return ErrWallet
	}
	sealed := aead.Seal(nil, nonce, payload, []byte("GOD Chain synthetic wallet v1"))
	return walletWrite(path, walletVault{1, "synthetic", "scrypt-262144-8-1-aes256gcm", hex.EncodeToString(salt), hex.EncodeToString(nonce), hex.EncodeToString(sealed)})
}

func walletUnlock(path, bundlePath, pin string, password []byte) (Bundle, []byte, error) {
	if !walletPassword(password) {
		return Bundle{}, nil, ErrWallet
	}
	b, err := loadBundle(bundlePath, pin)
	if err != nil {
		return Bundle{}, nil, ErrWallet
	}
	var v walletVault
	if readDocument(path, 4096, &v) != nil || v.Version != 1 || v.Mode != "synthetic" || v.KDF != "scrypt-262144-8-1-aes256gcm" {
		return Bundle{}, nil, ErrWallet
	}
	salt, e1 := walletHex(v.Salt, 32)
	nonce, e2 := walletHex(v.Nonce, 12)
	sealed, e3 := walletHex(v.Ciphertext, 80)
	bound, e4 := walletHex(pin, 32)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		return Bundle{}, nil, ErrWallet
	}
	aead, err := walletAEAD(password, salt)
	if err != nil {
		return Bundle{}, nil, ErrWallet
	}
	payload, err := aead.Open(nil, nonce, sealed, []byte("GOD Chain synthetic wallet v1"))
	if err != nil || len(payload) != 64 || !bytes.Equal(payload[32:], bound) {
		clear(payload)
		return Bundle{}, nil, ErrWallet
	}
	return b, payload, nil
}

func walletIdentity(b Bundle, pin string, scalar []byte) (WalletAddress, error) {
	key, err := crypto.ToECDSA(scalar)
	if err != nil {
		return WalletAddress{}, ErrWallet
	}
	defer key.D.SetInt64(0)
	address := crypto.PubkeyToAddress(key.PublicKey)
	native, err := godaddress.ToNative(address.Bytes())
	if err != nil {
		return WalletAddress{}, ErrWallet
	}
	return WalletAddress{1, "synthetic", b.Runtime.ChainID, strconv.FormatUint(b.Runtime.EVMChainID, 10), pin, native, address.Hex()}, nil
}

func WalletExportAddress(path, output, bundlePath, pin string, password []byte) error {
	b, payload, err := walletUnlock(path, bundlePath, pin, password)
	if err != nil {
		return ErrWallet
	}
	defer clear(payload)
	identity, err := walletIdentity(b, pin, payload[:32])
	if err != nil {
		return ErrWallet
	}
	return walletWrite(output, identity)
}

// WalletSign signs only six native operations or a calldata-free GOD transfer.
// Fees are capped at 0.01 GOD, no arbitrary EVM contract signing is supported.
// It never contacts a network or writes an unencrypted private key to disk.
func WalletSign(path, request, output, bundlePath, pin string, password []byte, approve func(WalletReview) bool) (SignReport, error) {
	if approve == nil {
		return SignReport{}, ErrWallet
	}
	b, payload, err := walletUnlock(path, bundlePath, pin, password)
	if err != nil {
		return SignReport{}, ErrWallet
	}
	defer clear(payload)
	identity, err := walletIdentity(b, pin, payload[:32])
	if err != nil {
		return SignReport{}, ErrWallet
	}
	// Read one private file once before review. No time-of-check reread of intent.
	var raw json.RawMessage
	if readDocument(request, 16<<10, &raw) != nil {
		return SignReport{}, ErrWallet
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return SignReport{}, ErrWallet
	}
	if _, transfer := fields["kind"]; transfer {
		var q WalletTransferRequest
		if decode(raw, &q) != nil {
			return SignReport{}, ErrWallet
		}
		return walletSignTransfer(b, identity, payload[:32], q, output, approve)
	}
	var q SignRequest
	if decode(raw, &q) != nil {
		return SignReport{}, ErrWallet
	}
	fee, err := rewardmsg.Amount(q.Fee)
	if err != nil || fee.GT(godrewards.Unit().QuoRaw(100)) {
		return SignReport{}, ErrWallet
	}
	if q.Action == "redeem-g" {
		deadline, err := decimalUint(q.DeadlineUnixNanos)
		if err != nil || deadline > uint64(^uint64(0)>>1) || int64(deadline) <= time.Now().UnixNano() {
			return SignReport{}, ErrWallet
		}
	}
	// Validate the exact intent before approval; produce no signature until the
	// private terminal explicitly accepts. Signing repeats these checks.
	_, numberErr := decimalUint(q.AccountNumber)
	sequence, sequenceErr := decimalUint(q.Sequence)
	gas, gasErr := decimalUint(q.Gas)
	_, actionErr := nativeAction(q, identity.NativeAddress)
	if q.Version != 1 || q.Mode != "synthetic" || q.ChainID != b.Runtime.ChainID || numberErr != nil || sequenceErr != nil || sequence == ^uint64(0) || gasErr != nil || gas == 0 || gas > b.Runtime.Policy.MaxGas || fee.LT(b.Runtime.Policy.MinFeePerGas.MulRaw(int64(gas))) || actionErr != nil {
		return SignReport{}, ErrWallet
	}
	view := WalletReview{ChainID: identity.ChainID, EVMChainID: identity.EVMChainID, BundleSHA256: pin, Sender: identity.NativeAddress, Operation: q.Action, Sequence: q.Sequence, AccountNumber: q.AccountNumber, Recipient: q.Recipient, Validator: q.Validator, Amount: q.Amount, Gas: q.Gas, Fee: q.Fee, MinGodOut: q.MinGodOut, Deadline: q.DeadlineUnixNanos}
	if !approve(view) {
		return SignReport{}, ErrWallet
	}
	result, err := signNativeBytes(b, q, payload[:32])
	if err != nil {
		return SignReport{}, ErrWallet
	}
	return writeSigned(output, result)
}

func walletSignTransfer(b Bundle, identity WalletAddress, scalar []byte, q WalletTransferRequest, output string, approve func(WalletReview) bool) (SignReport, error) {
	if q.Version != 1 || q.Mode != "synthetic" || q.Kind != "god-transfer" || q.ChainID != identity.ChainID || q.EVMChainID != identity.EVMChainID || q.BundleSHA256 != identity.BundleSHA256 || q.Sender != identity.NativeAddress {
		return SignReport{}, ErrWallet
	}
	nonce, err := decimalUint(q.Sequence)
	if err != nil || nonce == ^uint64(0) {
		return SignReport{}, ErrWallet
	}
	amount, e1 := rewardmsg.Amount(q.Amount)
	price, e2 := rewardmsg.Amount(q.GasPrice)
	if e1 != nil || e2 != nil || price.LT(b.Runtime.Policy.MinFeePerGas) || price.MulRaw(21000).GT(godrewards.Unit().QuoRaw(100)) || b.Runtime.Policy.MaxGas < 21000 {
		return SignReport{}, ErrWallet
	}
	var recipient []byte
	if strings.HasPrefix(q.Recipient, "0x") {
		recipient, err = godaddress.FromEVM(q.Recipient)
	} else {
		recipient, err = godaddress.FromNative(q.Recipient)
	}
	if err != nil {
		return SignReport{}, ErrWallet
	}
	to := common.BytesToAddress(recipient)
	key, err := crypto.ToECDSA(scalar)
	if err != nil {
		return SignReport{}, ErrWallet
	}
	defer key.D.SetInt64(0)
	if crypto.PubkeyToAddress(key.PublicKey) == to {
		return SignReport{}, ErrWallet
	}
	view := WalletReview{ChainID: identity.ChainID, EVMChainID: identity.EVMChainID, BundleSHA256: identity.BundleSHA256, Sender: identity.NativeAddress, Operation: "transfer-god", Sequence: q.Sequence, Recipient: to.Hex(), Amount: q.Amount, Gas: "21000", Fee: price.MulRaw(21000).String()}
	if !approve(view) {
		return SignReport{}, ErrWallet
	}
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: nonce, GasPrice: price.BigInt(), Gas: 21000, To: &to, Value: amount.BigInt()})
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(new(big.Int).SetUint64(b.Runtime.EVMChainID)), key)
	if err != nil {
		return SignReport{}, ErrWallet
	}
	wire, err := signed.MarshalBinary()
	if err != nil || len(wire) > b.Runtime.Policy.MaxTxBytes {
		return SignReport{}, ErrWallet
	}
	return writeSigned(output, SignedFunding{1, "synthetic", q.ChainID, q.EVMChainID, signed.Hash().Hex(), "0x" + hex.EncodeToString(wire)})
}
