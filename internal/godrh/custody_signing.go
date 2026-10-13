package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

var (
	ErrCustodySigning        = errors.New("RH private local signing preparation rejected; production signing remains disabled")
	ErrCustodySigningUnknown = errors.New("RH local signing attempt consumed; retain any private envelope and independently review, never automatically sign again")
)

type CustodySigningInputs struct {
	CallInputs CustodyFileInputs
	PlanPath   string
	PlanSHA256 [32]byte
}

func (CustodySigningInputs) String() string               { return "RH local signing inputs (redacted)" }
func (i CustodySigningInputs) GoString() string           { return i.String() }
func (CustodySigningInputs) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

// The explicit LOCAL adapter owns the key; no private key is accepted by this
// interface, stored by the attempt or returned by a report. It receives one
// detached canonical unsigned dynamic-fee envelope and returns signed bytes.
// It must honor context, avoid hidden retries/network submission, and not
// reenter signing on this attempt. Returned bytes must not be concurrently
// mutated while the caller detaches them. A malicious adapter is not sandboxed.
type LocalCustodySimulationSigner interface {
	SignLocalCustody(context.Context, []byte) ([]byte, error)
}

type CustodySigningReport struct {
	SimulationOnly                bool `json:"simulationOnly"`
	ExactUnsignedEnvelopePrepared bool `json:"exactUnsignedEnvelopePrepared"`
	LocalSigningAttemptConsumed   bool `json:"localSigningAttemptConsumed"`
	SignerResultMatched           bool `json:"signerResultMatched"`
	PrivateEnvelopeRetained       bool `json:"privateEnvelopeRetained"`
	SigningOutcomeUnknown         bool `json:"signingOutcomeUnknown"`
	DurableSigningAttempt         bool `json:"durableSigningAttempt"`
	SharedNonceReserved           bool `json:"sharedNonceReserved"`
	SourceStateVerified           bool `json:"sourceStateVerified"`
	ProductionSigningEnabled      bool `json:"productionSigningEnabled"`
	BroadcastEnabled              bool `json:"broadcastEnabled"`
	RealAssetsReady               bool `json:"realAssetsReady"`
}

// Process-local single attempt, NOT durable key/nonce coordination. All
// cooperating handles must point to this same object; do not copy it. A restart
// cannot authenticate that a recreated object has never signed. Production
// operation still requires enforced shared signing ownership and recovery.
type CustodySigningAttempt struct {
	mu       sync.Mutex
	inputs   CustodySigningInputs
	unsigned []byte
	report   CustodySigningReport
}

func (*CustodySigningAttempt) String() string                 { return "RH local signing attempt (redacted)" }
func (a *CustodySigningAttempt) GoString() string             { return a.String() }
func (a *CustodySigningAttempt) MarshalJSON() ([]byte, error) { return json.Marshal(a.Report()) }
func (a *CustodySigningAttempt) Report() CustodySigningReport {
	if a == nil {
		return CustodySigningReport{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.report
}

func loadCustodySigning(i CustodySigningInputs) (CustodyCall, custodyTransactionPlan, []byte, error) {
	bad := func() (CustodyCall, custodyTransactionPlan, []byte, error) {
		return CustodyCall{}, custodyTransactionPlan{}, nil, ErrCustodySigning
	}
	call, err := loadCustodyInputs(i.CallInputs)
	if err != nil {
		return bad()
	}
	configRaw, err := readCustodyPrivateSnapshot(i.CallInputs.ConfigurationPath, maxConfigBytes, i.CallInputs.ConfigurationSHA256)
	if err != nil {
		return bad()
	}
	c, err := Parse(configRaw)
	if err != nil || c.private.Mode != "simulation" {
		return bad()
	}
	planRaw, err := readCustodyPrivateSnapshot(i.PlanPath, 4096, i.PlanSHA256)
	if err != nil {
		return bad()
	}
	p, err := parseCustodyTransactionPlan(planRaw)
	if err != nil {
		return bad()
	}
	chain, target, data, err := call.Payload()
	token, tokenErr := address(c.private.TokenContract)
	if err != nil || tokenErr != nil || p.sender == token || p.sender == target {
		return bad()
	}
	intrinsic, err := custodyLondonIntrinsicGas(data)
	if err != nil || p.gas < intrinsic {
		return bad()
	}
	to := common.Address(target)
	tx := types.NewTx(&types.DynamicFeeTx{ChainID: chain, Nonce: p.nonce, Gas: p.gas, GasFeeCap: p.fee, GasTipCap: p.tip,
		To: &to, Value: new(big.Int), Data: data})
	raw, err := tx.MarshalBinary()
	if err != nil || len(raw) == 0 || len(raw) > MaxCustodyTransactionBytes {
		return bad()
	}
	return call, p, raw, nil
}

// Preparation has no signer, network, disk output or nonce reservation. All
// source, request and fee fields are independently pinned private inputs, with
// no automatically chosen fee, sender, amount, chain or contract.
func PrepareCustodySigningForSimulation(ctx context.Context, i CustodySigningInputs) (attempt *CustodySigningAttempt, err error) {
	defer func() {
		if recover() != nil {
			attempt, err = nil, ErrCustodySigning
		}
	}()
	if ctx == nil || ctx.Err() != nil {
		return nil, ErrCustodySigning
	}
	_, _, raw, err := loadCustodySigning(i)
	if err != nil || ctx.Err() != nil {
		return nil, ErrCustodySigning
	}
	return &CustodySigningAttempt{inputs: i, unsigned: raw, report: CustodySigningReport{SimulationOnly: true, ExactUnsignedEnvelopePrepared: true}}, nil
}

// Consume this process-local attempt BEFORE one bounded explicit local signer
// call. The result must pass the SAME exact signed-envelope reviewer used by
// the sender book. Save only to a fresh exclusive owner-private file, sync it,
// read back and recheck all originals before returning existing transaction
// inputs for an explicit Reserve/Submit flow. No automatic submission occurs.
// Any error after consumption forbids re-signing this object. A saved or partial
// file is preserved on uncertainty; do not overwrite/delete it or recreate an
// attempt to repair it. The cooperative ten-second context is not a hard fence.
func (a *CustodySigningAttempt) SignForSimulation(ctx context.Context, output string, signer LocalCustodySimulationSigner) (inputs CustodyTransactionInputs, report CustodySigningReport, err error) {
	if a == nil || ctx == nil || ctx.Err() != nil || signer == nil {
		return inputs, report, ErrCustodySigning
	}
	if err := a.consume(ctx, output); err != nil {
		return inputs, a.Report(), err
	}
	report = a.Report()
	defer func() {
		if recover() != nil {
			inputs, err = CustodyTransactionInputs{}, ErrCustodySigningUnknown
		}
		if err != nil {
			report.SigningOutcomeUnknown = true
		}
		a.mu.Lock()
		a.report = report
		a.mu.Unlock()
	}()
	signCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if signCtx.Err() != nil {
		return inputs, report, ErrCustodySigningUnknown
	}
	raw, err := signer.SignLocalCustody(signCtx, bytes.Clone(a.unsigned))
	if err != nil || len(raw) == 0 || len(raw) > MaxCustodyTransactionBytes || signCtx.Err() != nil {
		return inputs, report, ErrCustodySigningUnknown
	}
	// Own the result before any subsequent callback or private-file check.
	raw = bytes.Clone(raw)
	call, p, unsigned, err := loadCustodySigning(a.inputs)
	if err != nil || !bytes.Equal(unsigned, a.unsigned) || signCtx.Err() != nil {
		return inputs, report, ErrCustodySigningUnknown
	}
	checked, err := validateCustodyTransaction(call, p, raw)
	if err != nil || signCtx.Err() != nil {
		return inputs, report, ErrCustodySigningUnknown
	}
	report.SignerResultMatched = true
	if writeCustodyPrivateFile(output, checked.raw) != nil {
		return inputs, report, ErrCustodySigningUnknown
	}
	wanted := CustodyTransactionInputs{CallInputs: a.inputs.CallInputs, PlanPath: a.inputs.PlanPath, PlanSHA256: a.inputs.PlanSHA256,
		TransactionPath: output, TransactionSHA256: sha256.Sum256(checked.raw)}
	retained, err := loadCustodyTransaction(wanted)
	if err != nil || !retained.report.SimulationConfiguration || !bytes.Equal(retained.raw, checked.raw) || signCtx.Err() != nil {
		return inputs, report, ErrCustodySigningUnknown
	}
	report.PrivateEnvelopeRetained, report.SigningOutcomeUnknown = true, false
	return wanted, report, nil
}

func (a *CustodySigningAttempt) consume(ctx context.Context, output string) (err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	defer func() {
		if recover() != nil {
			err = ErrCustodySigning
		}
	}()
	if a.report.LocalSigningAttemptConsumed {
		return ErrCustodySigningUnknown
	}
	if !a.report.ExactUnsignedEnvelopePrepared || custodySigningOutputAvailable(output) != nil {
		return ErrCustodySigning
	}
	_, _, unsigned, err := loadCustodySigning(a.inputs)
	if err != nil || !bytes.Equal(unsigned, a.unsigned) || ctx.Err() != nil {
		return ErrCustodySigning
	}
	a.report.LocalSigningAttemptConsumed, a.report.SigningOutcomeUnknown = true, true
	return nil
}
