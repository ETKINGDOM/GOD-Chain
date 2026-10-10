package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	"github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	ErrCustodyReconcile         = errors.New("RH local custody receipt review rejected; no finality or financial resolution")
	ErrCustodyReconcileConflict = errors.New("RH local custody receipt observation conflict; independently review the original journal")
)

// Optional v1 field. Exact old snapshots without reviews retain their encoding.
// Only a complete-set digest is retained here. Caller must separately retain
// that private material to recheck it; a digest is not a transferable proof.
type custodyNonceReview struct {
	TransactionHash string `json:"transactionHash"`
	MaterialSHA256  string `json:"materialSHA256"`
	Succeeded       bool   `json:"succeeded"`
}

func sameCustodyNonceReviews(a, b []custodyNonceReview) bool {
	if len(a) != len(b) {
		return false
	}
	for n := range a {
		if a[n] != b[n] {
			return false
		}
	}
	return true
}

type CustodyReconciliationReport struct {
	SimulationOnly                bool `json:"simulationOnly"`
	CompleteReceiptSetMatched     bool `json:"completeReceiptSetMatched"`
	ExactEnvelopeMatched          bool `json:"exactEnvelopeMatched"`
	ExecutionGasAccountingMatched bool `json:"executionGasAccountingMatched"`
	ReceiptSucceeded              bool `json:"receiptSucceeded"`
	CustodyEventMatched           bool `json:"custodyEventMatched"`
	ObservationRetained           bool `json:"observationRetained"`
	DispatchOutcomeUnknown        bool `json:"dispatchOutcomeUnknown"`
	IndependentFinalityVerified   bool `json:"independentFinalityVerified"`
	PaymentVerified               bool `json:"paymentVerified"`
	NonceReleased                 bool `json:"nonceReleased"`
	FeeReservationReleased        bool `json:"feeReservationReleased"`
	BroadcastEnabled              bool `json:"broadcastEnabled"`
	RealAssetsReady               bool `json:"realAssetsReady"`
}

func (CustodyReconciliationReport) String() string {
	return "RH local simulation receipt review (redacted)"
}
func (r CustodyReconciliationReport) GoString() string { return r.String() }

func custodyMaterialPin(m BlockMaterial) [32]byte {
	h := sha256.New()
	_, _ = h.Write([]byte("GOD Chain local custody complete receipt set v1"))
	// Every element and list count is explicitly length-delimited. This is
	// corruption/identity continuity, not authenticated source consensus.
	put := func(raw []byte) {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(raw)))
		_, _ = h.Write(n[:])
		_, _ = h.Write(raw)
	}
	put(m.Header)
	for _, list := range [][][]byte{m.Transactions, m.Receipts} {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(len(list)))
		_, _ = h.Write(n[:])
		for _, raw := range list {
			put(raw)
		}
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// Complete-set root agreement is NOT authenticated header origin, source state,
// permission, L1 settlement, full block execution, RH fees or financial finality.
func checkCustodyExecution(ctx context.Context, i CustodyTransactionInputs, checked checkedCustodyTransaction, block Block, material BlockMaterial) (custodyNonceReview, CustodyReconciliationReport, error) {
	bad := func() (custodyNonceReview, CustodyReconciliationReport, error) {
		return custodyNonceReview{}, CustodyReconciliationReport{}, ErrCustodyReconcile
	}
	if ctx == nil || ctx.Err() != nil || !validDiscoveryBlock(block) {
		return bad()
	}
	m, err := detachReceiptSetMaterial(material)
	if err != nil {
		return bad()
	}
	var tx types.Transaction
	if tx.UnmarshalBinary(checked.raw) != nil || tx.To() == nil {
		return bad()
	}
	custody := [20]byte(*tx.To())
	if _, err := receiptSetEvents(ctx, m, custody, block); err != nil {
		return bad()
	}
	selected := -1
	for n, raw := range m.Transactions {
		if bytes.Equal(raw, checked.raw) {
			if selected != -1 {
				return bad()
			}
			selected = n
		}
	}
	if selected == -1 {
		return bad()
	}
	var header types.Header
	var receipt types.Receipt
	if rlp.DecodeBytes(m.Header, &header) != nil || header.BaseFee == nil || header.BaseFee.Sign() < 0 || header.BaseFee.BitLen() > 256 || header.BaseFee.Cmp(tx.GasFeeCap()) > 0 || receipt.UnmarshalBinary(m.Receipts[selected]) != nil {
		return bad()
	}
	var priorGas uint64
	if selected > 0 {
		var prior types.Receipt
		if prior.UnmarshalBinary(m.Receipts[selected-1]) != nil {
			return bad()
		}
		priorGas = prior.CumulativeGasUsed
	}
	if receipt.CumulativeGasUsed <= priorGas {
		return bad()
	}
	used := receipt.CumulativeGasUsed - priorGas
	if used < 21000 || used > tx.Gas() {
		return bad()
	}
	price := new(big.Int).Add(header.BaseFee, tx.GasTipCap())
	if price.Cmp(tx.GasFeeCap()) > 0 {
		price.Set(tx.GasFeeCap())
	}
	cost := new(big.Int).Mul(new(big.Int).SetUint64(used), price)
	if checked.maximumGasCost == nil || cost.Cmp(checked.maximumGasCost) > 0 {
		return bad()
	}
	raw, err := readCustodyPrivateSnapshot(i.CallInputs.RequestPath, MaxCustodyRequestBytes, i.CallInputs.RequestSHA256)
	if err != nil {
		return bad()
	}
	r, err := ParseCustodyRequest(raw)
	if err != nil {
		return bad()
	}
	succeeded := receipt.Status == types.ReceiptStatusSuccessful
	if succeeded && !custodyExecutionEvent(receipt.Logs, custody, checked.sender, r) {
		return bad()
	}
	if ctx.Err() != nil {
		return bad()
	}
	pin := custodyMaterialPin(m)
	review := custodyNonceReview{TransactionHash: hex.EncodeToString(checked.hash[:]), MaterialSHA256: hex.EncodeToString(pin[:]), Succeeded: succeeded}
	return review, CustodyReconciliationReport{SimulationOnly: true, CompleteReceiptSetMatched: true, ExactEnvelopeMatched: true, ExecutionGasAccountingMatched: true, ReceiptSucceeded: succeeded, CustodyEventMatched: succeeded, DispatchOutcomeUnknown: true}, nil
}

// Exactly one recognized custody effect, with exact request fields. Unrelated
// token logs are allowed, but wrong/duplicate recognized custody outcomes are not.
func custodyExecutionEvent(logs []*types.Log, custody, sender [20]byte, r CustodyCallRequest) bool {
	names := []string{"Deposited(uint64,address,bytes20,uint256)", "Paid(uint64,bytes32,bytes20,uint256)", "Cancelled(uint64,bytes32)", "Paused(uint64,bool,bool)"}
	count := 0
	for _, log := range logs {
		if log == nil || [20]byte(log.Address) != custody || len(log.Topics) == 0 {
			continue
		}
		recognized := false
		for _, name := range names {
			if log.Topics[0] == ethcrypto.Keccak256Hash([]byte(name)) {
				recognized = true
			}
		}
		if !recognized {
			continue
		}
		count++
		l := ReceiptLog{Emitter: custody, Data: bytes.Clone(log.Data)}
		for _, topic := range log.Topics {
			l.Topics = append(l.Topics, [32]byte(topic))
		}
		switch r.Action {
		case CustodyDeposit:
			if log.Topics[0] != ethcrypto.Keccak256Hash([]byte(names[0])) {
				return false
			}
			deposit, err := decodeDeposit(l, custody, Receipt{})
			recipient, addressErr := godaddress.FromNative(r.NativeRecipient)
			if err != nil || addressErr != nil || !bytes.Equal(deposit.Recipient[:], recipient) || r.Amount.IsNil() || !deposit.Amount.Equal(r.Amount) || !bytes.Equal(l.Topics[2][12:], sender[:]) {
				return false
			}
		case CustodyPay, CustodyCancel:
			outcome := godbridge.Paid
			if r.Action == CustodyCancel {
				outcome = godbridge.Cancelled
			}
			if decodeResolutionEvent(l, custody, r.Withdrawal, outcome) != nil {
				return false
			}
		case CustodyPause:
			if len(l.Topics) != 2 || l.Topics[0] != [32]byte(ethcrypto.Keccak256Hash([]byte(names[3]))) || !bytes.Equal(l.Topics[1][:24], make([]byte, 24)) || binary.BigEndian.Uint64(l.Topics[1][24:]) != r.ControlNonce || len(l.Data) != 64 {
				return false
			}
			expected := make([]byte, 64)
			if r.Intake {
				expected[31] = 1
			}
			if r.Outflow {
				expected[63] = 1
			}
			if !bytes.Equal(l.Data, expected) {
				return false
			}
		default:
			return false
		}
	}
	return count == 1
}

// Only unknown entries can acquire ONE immutable local receipt observation.
// Full supplied material is rechecked on exact repeats. A different material
// digest (including a consistent fork) stops the instance without overwriting.
// This never resolves payment, releases fees/nonces, or installs a provider.
func (b *CustodyNonceBook) ReconcileForSimulation(ctx context.Context, i CustodyTransactionInputs, block Block, material BlockMaterial) (report CustodyReconciliationReport, err error) {
	if b == nil {
		return CustodyReconciliationReport{}, ErrCustodyReconcile
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	defer func() {
		if recover() != nil {
			b.usable = false
			report, err = CustodyReconciliationReport{}, ErrCustodyReconcile
		}
	}()
	if !b.ready(ctx) {
		return CustodyReconciliationReport{}, ErrCustodyReconcile
	}
	if err := b.recheck(ctx); err != nil {
		return CustodyReconciliationReport{}, err
	}
	wanted, checked, err := b.candidate(i)
	if err != nil {
		return CustodyReconciliationReport{}, ErrCustodyReconcile
	}
	matched := false
	for _, e := range b.state.Entries {
		if e.Binding.Stage == CustodyAttemptUnknown && sameCustodyAttempt(e.Binding, wanted.Binding) && e.Nonce == wanted.Nonce && e.MaximumGasCost == wanted.MaximumGasCost {
			matched = true
		}
	}
	if !matched {
		return CustodyReconciliationReport{}, ErrCustodyReconcile
	}
	review, report, err := checkCustodyExecution(ctx, i, checked, block, material)
	if err != nil {
		return CustodyReconciliationReport{}, ErrCustodyReconcile
	}
	for _, old := range b.state.Reviews {
		if old.TransactionHash != review.TransactionHash {
			continue
		}
		if old != review {
			b.usable = false
			return CustodyReconciliationReport{}, ErrCustodyReconcileConflict
		}
		report.ObservationRetained = true
		return report, nil
	}
	if ctx.Err() != nil {
		return CustodyReconciliationReport{}, ErrCustodyReconcile
	}
	next := b.nextState()
	next.Reviews = append(next.Reviews, review)
	if err := b.write(next); err != nil {
		return CustodyReconciliationReport{}, err
	}
	if ctx.Err() != nil {
		return CustodyReconciliationReport{}, ErrCustodyReconcile
	}
	report.ObservationRetained = true
	return report, nil
}
