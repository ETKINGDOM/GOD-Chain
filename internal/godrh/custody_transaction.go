package godrh

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math"
	"math/big"
	"strconv"
	"unicode/utf8"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
)

// Local rehearsal bounds, not production gas policy or source gas estimation.
const MaxCustodyTransactionBytes = 5 << 10
const MaxCustodySimulationGas uint64 = 10_000_000

var ErrCustodyTransaction = errors.New("RH offline custody transaction binding rejected; no signing or broadcast")

type CustodyTransactionInputs struct {
	CallInputs        CustodyFileInputs
	PlanPath          string
	PlanSHA256        [32]byte
	TransactionPath   string
	TransactionSHA256 [32]byte
}

func (CustodyTransactionInputs) String() string     { return "RH custody transaction inputs (redacted)" }
func (i CustodyTransactionInputs) GoString() string { return i.String() }
func (CustodyTransactionInputs) MarshalJSON() ([]byte, error) {
	return []byte(`{"redacted":true}`), nil
}

type custodyTransactionPlan struct {
	sender           [20]byte
	nonce, gas       uint64
	fee, tip, budget *big.Int
}

func (p custodyTransactionPlan) valid() bool {
	return p.sender != [20]byte{} && p.nonce != math.MaxUint64 && p.gas >= 21000 && p.gas <= MaxCustodySimulationGas &&
		p.fee != nil && p.tip != nil && p.budget != nil && p.fee.Sign() > 0 && p.tip.Sign() > 0 && p.budget.Sign() > 0 &&
		p.fee.BitLen() <= 256 && p.tip.BitLen() <= 256 && p.budget.BitLen() <= 256 && p.tip.Cmp(p.fee) <= 0 &&
		new(big.Int).Mul(new(big.Int).SetUint64(p.gas), p.fee).Cmp(p.budget) <= 0
}

type CustodyTransactionReport struct {
	Call                      CustodyCallReport `json:"call"`
	PrivateInputsChecked      bool              `json:"privateInputsChecked"`
	SignedEnvelopeMatched     bool              `json:"signedEnvelopeMatched"`
	SenderSignatureChecked    bool              `json:"senderSignatureChecked"`
	ExactNonceAndFeesMatched  bool              `json:"exactNonceAndFeesMatched"`
	WithinSuppliedFeeBudget   bool              `json:"withinSuppliedFeeBudget"`
	LondonIntrinsicGasMatched bool              `json:"londonIntrinsicGasMatched"`
	SimulationConfiguration   bool              `json:"simulationConfiguration"`
	NonceAvailabilityVerified bool              `json:"nonceAvailabilityVerified"`
	SourceGasScheduleVerified bool              `json:"sourceGasScheduleVerified"`
	FeeMarketVerified         bool              `json:"feeMarketVerified"`
	SourceFinalityVerified    bool              `json:"sourceFinalityVerified"`
	SigningEnabled            bool              `json:"signingEnabled"`
	BroadcastEnabled          bool              `json:"broadcastEnabled"`
	RealAssetsReady           bool              `json:"realAssetsReady"`
}

type checkedCustodyTransaction struct {
	raw                   []byte
	hash                  [32]byte
	report                CustodyTransactionReport
	sender                [20]byte
	nonce                 uint64
	chain, maximumGasCost *big.Int
}

// Blank diagnostic input; never supplies a nonce, fee, sender or authority.
func CustodyTransactionTemplate() []byte {
	return []byte("{\n  \"version\": 1,\n  \"sender\": \"\",\n  \"nonce\": \"\",\n  \"gasLimit\": \"\",\n  \"maxFeePerGas\": \"\",\n  \"maxPriorityFeePerGas\": \"\",\n  \"maximumFeeCost\": \"\"\n}\n")
}

func parseCustodyTransactionPlan(raw []byte) (custodyTransactionPlan, error) {
	var p custodyTransactionPlan
	if len(raw) == 0 || len(raw) > 4096 || !utf8.Valid(raw) {
		return p, ErrCustodyTransaction
	}
	f, err := custodyObject(raw, []string{"version", "sender", "nonce", "gasLimit", "maxFeePerGas", "maxPriorityFeePerGas", "maximumFeeCost"}, nil)
	if err != nil || !bytes.Equal(bytes.TrimSpace(f["version"]), []byte("1")) {
		return p, ErrCustodyTransaction
	}
	sender, err := custodyHex(f["sender"], 20)
	if err != nil {
		return p, ErrCustodyTransaction
	}
	copy(p.sender[:], sender)
	nonce, err := custodyText(f["nonce"])
	if err != nil || len(nonce) > 20 {
		return custodyTransactionPlan{}, ErrCustodyTransaction
	}
	p.nonce, err = strconv.ParseUint(nonce, 10, 64)
	if err != nil || strconv.FormatUint(p.nonce, 10) != nonce {
		return custodyTransactionPlan{}, ErrCustodyTransaction
	}
	p.gas, err = custodyUint64(f["gasLimit"])
	if err != nil {
		return custodyTransactionPlan{}, ErrCustodyTransaction
	}
	for name, target := range map[string]**big.Int{"maxFeePerGas": &p.fee, "maxPriorityFeePerGas": &p.tip, "maximumFeeCost": &p.budget} {
		text, err := custodyPositiveDecimal(f[name])
		if err != nil {
			return custodyTransactionPlan{}, ErrCustodyTransaction
		}
		n, ok := new(big.Int).SetString(text, 10)
		if !ok || n.BitLen() > 256 {
			return custodyTransactionPlan{}, ErrCustodyTransaction
		}
		*target = n
	}
	if !p.valid() {
		return custodyTransactionPlan{}, ErrCustodyTransaction
	}
	return p, nil
}

// Minimum admission cost for the already restricted London envelope model:
// non-creation, empty access list, no authorizations, EIP-2028 calldata pricing.
// This is NOT a contract execution estimate or an RH fork/fee certification.
// In particular it does not apply Prague's separately activated EIP-7623 floor.
func custodyLondonIntrinsicGas(data []byte) (uint64, error) {
	if len(data) > MaxCustodyTransactionBytes {
		return 0, ErrCustodyTransaction
	}
	gas, err := core.IntrinsicGas(data, nil, nil, false, true, true, false)
	if err != nil {
		return 0, ErrCustodyTransaction
	}
	return gas, nil
}

// Exact dynamic-fee envelope review with the pinned execution library. No
// signing, nonce allocation, estimate, fee selection, approval or network call.
// The supplied plan is a reviewed policy claim, NOT authenticated source state.
func validateCustodyTransaction(call CustodyCall, p custodyTransactionPlan, raw []byte) (checkedCustodyTransaction, error) {
	if !p.valid() || len(raw) == 0 || len(raw) > MaxCustodyTransactionBytes {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	chain, target, data, err := call.Payload()
	if err != nil || p.sender == target {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	var tx types.Transaction
	if tx.UnmarshalBinary(bytes.Clone(raw)) != nil || tx.Type() != types.DynamicFeeTxType || tx.ChainId().Cmp(chain) != 0 ||
		tx.To() == nil || [20]byte(*tx.To()) != target || tx.Value().Sign() != 0 || !bytes.Equal(tx.Data(), data) ||
		tx.Nonce() != p.nonce || tx.Gas() != p.gas || tx.GasFeeCap().Cmp(p.fee) != 0 || tx.GasTipCap().Cmp(p.tip) != 0 || len(tx.AccessList()) != 0 {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	canonical, err := tx.MarshalBinary()
	if err != nil || !bytes.Equal(raw, canonical) {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	intrinsic, err := custodyLondonIntrinsicGas(tx.Data())
	if err != nil || tx.Gas() < intrinsic {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	sender, err := types.Sender(types.NewLondonSigner(chain), &tx)
	if err != nil || [20]byte(sender) != p.sender {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	return checkedCustodyTransaction{raw: bytes.Clone(raw), hash: [32]byte(tx.Hash()),
		sender: p.sender, nonce: p.nonce, chain: new(big.Int).Set(chain),
		maximumGasCost: new(big.Int).Mul(new(big.Int).SetUint64(p.gas), p.fee),
		report: CustodyTransactionReport{Call: call.Report(), SignedEnvelopeMatched: true, SenderSignatureChecked: true,
			ExactNonceAndFeesMatched: true, WithinSuppliedFeeBudget: true, LondonIntrinsicGasMatched: true}}, nil
}

func loadCustodyTransaction(inputs CustodyTransactionInputs) (checked checkedCustodyTransaction, err error) {
	defer func() {
		if recover() != nil {
			checked, err = checkedCustodyTransaction{}, ErrCustodyTransaction
		}
	}()
	call, err := loadCustodyInputs(inputs.CallInputs)
	if err != nil {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	configRaw, err := readCustodyPrivateSnapshot(inputs.CallInputs.ConfigurationPath, maxConfigBytes, inputs.CallInputs.ConfigurationSHA256)
	if err != nil {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	c, err := Parse(configRaw)
	if err != nil {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	planRaw, err := readCustodyPrivateSnapshot(inputs.PlanPath, 4096, inputs.PlanSHA256)
	if err != nil {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	p, err := parseCustodyTransactionPlan(planRaw)
	if err != nil {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	token, err := address(c.private.TokenContract)
	if err != nil || p.sender == token {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	raw, err := readCustodyPrivateSnapshot(inputs.TransactionPath, MaxCustodyTransactionBytes, inputs.TransactionSHA256)
	if err != nil || sha256.Sum256(raw) != inputs.TransactionSHA256 {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	checked, err = validateCustodyTransaction(call, p, raw)
	if err != nil {
		return checkedCustodyTransaction{}, ErrCustodyTransaction
	}
	checked.report.PrivateInputsChecked = true
	checked.report.SimulationConfiguration = c.private.Mode == "simulation"
	return checked, nil
}

// Production-mode offline review is allowed, but never enables transaction
// submission. Signed bytes are bearer execution authority: retain them privately.
func CheckCustodyTransactionFiles(inputs CustodyTransactionInputs) (CustodyTransactionReport, error) {
	tx, err := loadCustodyTransaction(inputs)
	if err != nil {
		return CustodyTransactionReport{}, ErrCustodyTransaction
	}
	return tx.report, nil
}
