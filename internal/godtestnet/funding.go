//go:build go1.25

package godtestnet

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	rewardmsg "github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// FundingRequest is a manually reviewed participant request, not an approval,
// automated faucet claim, transaction or proof of unique human identity.
type FundingRequest struct {
	Version      uint32 `json:"version"`
	Mode         string `json:"mode"`
	Kind         string `json:"kind"`
	ChainID      string `json:"chainId"`
	EVMChainID   string `json:"evmChainId"`
	BundleSHA256 string `json:"bundleSha256"`
	Recipient    string `json:"recipient"`
	Amount       string `json:"amountSmallestUnits"`
}

type SignedFunding struct {
	Version        uint32 `json:"version"`
	Mode           string `json:"mode"`
	ChainID        string `json:"chainId"`
	EVMChainID     string `json:"evmChainId"`
	Hash           string `json:"hash"`
	RawTransaction string `json:"rawTransaction"`
}

// SignFunding signs a bounded plain transfer from a genesis validator's
// separately held disposable account. It never mints, uses reserve, submits,
// queries a network, changes validator keys or stores an account key on RPC.
// The five-GOD amount and 0.001-GOD fee caps are synthetic-tool limits only.
func SignFunding(requestPath, keyPath, outputPath, bundlePath, expectedBundle, nonceText, priceText string) (SignReport, error) {
	b, err := loadBundle(bundlePath, expectedBundle)
	if err != nil {
		return SignReport{}, err
	}
	var q FundingRequest
	if readDocument(requestPath, 16<<10, &q) != nil || q.Version != 1 || q.Mode != "synthetic" || q.Kind != "test-funding" ||
		q.ChainID != b.Runtime.ChainID || q.EVMChainID != strconv.FormatUint(b.Runtime.EVMChainID, 10) || q.BundleSHA256 != expectedBundle {
		return SignReport{}, ErrConfig
	}
	nonce, err := decimalUint(nonceText)
	if err != nil || nonce == ^uint64(0) {
		return SignReport{}, ErrConfig
	}
	amount, err := rewardmsg.Amount(q.Amount)
	if err != nil || amount.GT(godrewards.Unit().MulRaw(5)) {
		return SignReport{}, ErrConfig
	}
	price, err := rewardmsg.Amount(priceText)
	if err != nil || price.LT(b.Runtime.Policy.MinFeePerGas) || price.MulRaw(21000).GT(godrewards.Unit().QuoRaw(1000)) || b.Runtime.Policy.MaxGas < 21000 {
		return SignReport{}, ErrConfig
	}
	var recipient []byte
	if strings.HasPrefix(q.Recipient, "0x") {
		recipient, err = godaddress.FromEVM(q.Recipient)
	} else {
		recipient, err = godaddress.FromNative(q.Recipient)
	}
	if err != nil {
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
	if err != nil || len(keyBytes) != 32 {
		return SignReport{}, ErrPrivate
	}
	key, err := crypto.ToECDSA(keyBytes)
	if err != nil {
		return SignReport{}, ErrPrivate
	}
	defer key.D.SetInt64(0)
	sender := crypto.PubkeyToAddress(key.PublicKey)
	owner, err := godaddress.ToNative(sender.Bytes())
	if err != nil {
		return SignReport{}, ErrConfig
	}
	authorized := false
	for _, profile := range b.Profiles {
		if profile.Role == "validator" && profile.Owner == owner {
			authorized = true
		}
	}
	to := common.BytesToAddress(recipient)
	if !authorized || sender == to {
		return SignReport{}, ErrConfig
	}
	id := new(big.Int).SetUint64(b.Runtime.EVMChainID)
	tx := ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: nonce, GasPrice: price.BigInt(), Gas: 21000, To: &to, Value: amount.BigInt()})
	signed, err := ethtypes.SignTx(tx, ethtypes.LatestSignerForChainID(id), key)
	if err != nil {
		return SignReport{}, ErrConfig
	}
	wire, err := signed.MarshalBinary()
	if err != nil || len(wire) > b.Runtime.Policy.MaxTxBytes {
		return SignReport{}, ErrConfig
	}
	result := SignedFunding{1, "synthetic", q.ChainID, q.EVMChainID, signed.Hash().Hex(), "0x" + hex.EncodeToString(wire)}
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
