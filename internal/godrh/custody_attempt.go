package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
)

const MaxCustodyAttemptBytes = 2048
const custodyAttemptPurpose = "GOD Chain simulation transaction attempt v1"

var (
	ErrCustodyAttemptState   = errors.New("RH simulation custody attempt or immutable binding rejected")
	ErrCustodyAttemptStorage = errors.New("RH simulation custody attempt storage uncertain; close, reopen and review")
	ErrCustodyAttemptUnknown = errors.New("RH simulation transaction dispatch outcome unknown; no retry, replacement or nonce release")
)

type CustodyAttemptStage string

const (
	CustodyAttemptReserved CustodyAttemptStage = "reserved_simulation"
	CustodyAttemptUnknown  CustodyAttemptStage = "dispatch_unknown_simulation"
)

type custodyAttemptState struct {
	Purpose             string              `json:"purpose"`
	Version             uint32              `json:"version"`
	ConfigurationSHA256 string              `json:"configurationSHA256"`
	RequestSHA256       string              `json:"requestSHA256"`
	PlanSHA256          string              `json:"planSHA256"`
	EnvelopeSHA256      string              `json:"envelopeSHA256"`
	TransactionHash     string              `json:"transactionHash"`
	Action              CustodyAction       `json:"action"`
	Stage               CustodyAttemptStage `json:"stage"`
}

// Separate private single-slot storage contract. A Save error can retain old,
// new or incomplete state, so the current instance must stop. Successful Save
// synchronously retains complete detached bytes. No callback may reenter.
type CustodyAttemptStorage interface {
	Load() ([]byte, error)
	Save([]byte) error
	Close() error
}

type CustodyAttemptReport struct {
	Transaction            CustodyTransactionReport `json:"transaction"`
	Stage                  CustodyAttemptStage      `json:"stage"`
	StorageUsable          bool                     `json:"storageUsable"`
	DispatchOutcomeUnknown bool                     `json:"dispatchOutcomeUnknown"`
	WriteOutcomeUncertain  bool                     `json:"writeOutcomeUncertain"`
	RetryAllowed           bool                     `json:"retryAllowed"`
	NonceReleased          bool                     `json:"nonceReleased"`
	PaymentVerified        bool                     `json:"paymentVerified"`
}

// Simulation ONLY. No provider, signer, HTTP transport, retry or finalization
// capability. Owns one exact signed envelope after private offline validation.
// Different directories, legacy tools, rollback and a hostile owner are NOT
// fenced. This is not the production sender-wide nonce journal.
type CustodyAttempt struct {
	mu                        sync.Mutex
	inputs                    CustodyTransactionInputs
	store                     CustodyAttemptStorage
	state                     custodyAttemptState
	reportData                CustodyTransactionReport
	usable, closed, uncertain bool
}

func (*CustodyAttempt) String() string                 { return "RH simulation custody attempt (redacted)" }
func (a *CustodyAttempt) GoString() string             { return a.String() }
func (a *CustodyAttempt) MarshalJSON() ([]byte, error) { return json.Marshal(a.Report()) }
func (a *CustodyAttempt) Report() CustodyAttemptReport {
	if a == nil {
		return CustodyAttemptReport{}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return CustodyAttemptReport{Transaction: a.reportData, Stage: a.state.Stage, StorageUsable: a.usable && !a.closed,
		DispatchOutcomeUnknown: a.state.Stage == CustodyAttemptUnknown || a.uncertain, WriteOutcomeUncertain: a.uncertain}
}

func encodeCustodyAttempt(s custodyAttemptState) ([]byte, error) {
	if s.Purpose != custodyAttemptPurpose || s.Version != 1 ||
		(s.Stage != CustodyAttemptReserved && s.Stage != CustodyAttemptUnknown) ||
		(s.Action != CustodyDeposit && s.Action != CustodyPay && s.Action != CustodyCancel && s.Action != CustodyPause) {
		return nil, ErrCustodyAttemptState
	}
	for _, pin := range []string{s.ConfigurationSHA256, s.RequestSHA256, s.PlanSHA256, s.EnvelopeSHA256, s.TransactionHash} {
		if _, err := SourceMaterialPin(pin); err != nil {
			return nil, ErrCustodyAttemptState
		}
	}
	raw, err := json.Marshal(s)
	if err != nil || len(raw) > MaxCustodyAttemptBytes {
		return nil, ErrCustodyAttemptState
	}
	return raw, nil
}
func decodeCustodyAttempt(raw []byte) (custodyAttemptState, error) {
	var s custodyAttemptState
	if len(raw) == 0 || len(raw) > MaxCustodyAttemptBytes || json.Unmarshal(raw, &s) != nil {
		return custodyAttemptState{}, ErrCustodyAttemptState
	}
	exact, err := encodeCustodyAttempt(s)
	if err != nil || !bytes.Equal(raw, exact) {
		return custodyAttemptState{}, ErrCustodyAttemptState
	}
	return s, nil
}
func sameCustodyAttempt(a, b custodyAttemptState) bool {
	a.Stage, b.Stage = CustodyAttemptReserved, CustodyAttemptReserved
	return a == b
}
func custodyAttemptCandidate(i CustodyTransactionInputs) (custodyAttemptState, checkedCustodyTransaction, error) {
	tx, err := loadCustodyTransaction(i)
	if err != nil || !tx.report.SimulationConfiguration {
		return custodyAttemptState{}, checkedCustodyTransaction{}, ErrCustodyAttemptState
	}
	encode := func(v [32]byte) string { return hex.EncodeToString(v[:]) }
	return custodyAttemptState{Purpose: custodyAttemptPurpose, Version: 1, ConfigurationSHA256: encode(i.CallInputs.ConfigurationSHA256),
		RequestSHA256: encode(i.CallInputs.RequestSHA256), PlanSHA256: encode(i.PlanSHA256), EnvelopeSHA256: encode(i.TransactionSHA256),
		TransactionHash: encode(tx.hash), Action: tx.report.Call.Action, Stage: CustodyAttemptReserved}, tx, nil
}

// Reopen requires ONE independently retained exact latest-state checksum. Do
// not derive it from the same unreviewed storage being reopened. Create accepts
// no resume pin. Constructor owns Close only after success. No observation or
// replay of an older state with the latest pin can release an unknown slot.
func NewCustodyAttempt(ctx context.Context, i CustodyTransactionInputs, store CustodyAttemptStorage, create bool, resumePin ...[32]byte) (attempt *CustodyAttempt, err error) {
	defer func() {
		if recover() != nil {
			attempt, err = nil, ErrCustodyAttemptStorage
		}
	}()
	if ctx == nil || ctx.Err() != nil || store == nil || !custodyAttemptResumeValid(create, resumePin) {
		return nil, ErrCustodyAttemptState
	}
	wanted, tx, err := custodyAttemptCandidate(i)
	if err != nil {
		return nil, ErrCustodyAttemptState
	}
	raw, err := store.Load()
	if err != nil {
		return nil, ErrCustodyAttemptStorage
	}
	if ctx.Err() != nil {
		return nil, ErrCustodyAttemptState
	}
	a := &CustodyAttempt{inputs: i, store: store, state: wanted, reportData: tx.report, usable: true}
	if create {
		if len(raw) != 0 {
			return nil, ErrCustodyAttemptState
		}
		if a.write(wanted) != nil {
			return nil, ErrCustodyAttemptStorage
		}
	} else {
		if sha256.Sum256(raw) != resumePin[0] {
			return nil, ErrCustodyAttemptState
		}
		s, err := decodeCustodyAttempt(bytes.Clone(raw))
		if err != nil || !sameCustodyAttempt(s, wanted) {
			return nil, ErrCustodyAttemptState
		}
		a.state = s
	}
	return a, nil
}

func custodyAttemptResumeValid(create bool, pins [][32]byte) bool {
	return create && len(pins) == 0 || !create && len(pins) == 1 && pins[0] != [32]byte{}
}

// Explicit PRIVATE access for independently retained recovery material, never
// ordinary reports. A checksum is exact-byte continuity, not authenticated
// origin or protection if both storage and the external pin are rolled back.
func (a *CustodyAttempt) RetainedStatePin(ctx context.Context) ([32]byte, error) {
	if a == nil || ctx == nil {
		return [32]byte{}, ErrCustodyAttemptState
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable || a.closed || ctx.Err() != nil {
		return [32]byte{}, ErrCustodyAttemptState
	}
	if a.checkStored() != nil {
		return [32]byte{}, ErrCustodyAttemptStorage
	}
	raw, err := encodeCustodyAttempt(a.state)
	if err != nil || ctx.Err() != nil {
		return [32]byte{}, ErrCustodyAttemptState
	}
	return sha256.Sum256(raw), nil
}
func (a *CustodyAttempt) write(s custodyAttemptState) (err error) {
	raw, err := encodeCustodyAttempt(s)
	if err != nil {
		return err
	}
	defer func() {
		if recover() != nil {
			err = ErrCustodyAttemptStorage
		}
		if err != nil {
			a.usable, a.uncertain = false, true
		}
	}()
	if a.store.Save(bytes.Clone(raw)) != nil {
		return ErrCustodyAttemptStorage
	}
	a.state = s
	return nil
}
func (a *CustodyAttempt) checkStored() (err error) {
	defer func() {
		if recover() != nil {
			err = ErrCustodyAttemptStorage
		}
		if err != nil {
			a.usable = false
		}
	}()
	raw, err := a.store.Load()
	if err != nil {
		return ErrCustodyAttemptStorage
	}
	s, err := decodeCustodyAttempt(bytes.Clone(raw))
	if err != nil || s != a.state {
		return ErrCustodyAttemptState
	}
	return nil
}

// Returns exact signed bytes ONCE for an explicitly supplied LOCAL simulator.
// Persist unknown BEFORE exposing bytes. Interruption after that Save (even
// before this function returns) permanently refuses re-dispatch in this slot.
// No transport is installed. Do not call with a production signer/provider.
func (a *CustodyAttempt) TakeForSimulation(ctx context.Context) ([]byte, error) {
	if a == nil || ctx == nil {
		return nil, ErrCustodyAttemptState
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable || a.closed || ctx.Err() != nil {
		return nil, ErrCustodyAttemptState
	}
	if a.checkStored() != nil {
		return nil, ErrCustodyAttemptStorage
	}
	if a.state.Stage == CustodyAttemptUnknown {
		return nil, ErrCustodyAttemptUnknown
	}
	wanted, tx, err := custodyAttemptCandidate(a.inputs)
	if err != nil || !sameCustodyAttempt(wanted, a.state) {
		a.usable = false
		return nil, ErrCustodyAttemptState
	}
	if ctx.Err() != nil {
		return nil, ErrCustodyAttemptState
	}
	next := a.state
	next.Stage = CustodyAttemptUnknown
	if a.write(next) != nil {
		return nil, ErrCustodyAttemptStorage
	}
	if ctx.Err() != nil {
		return nil, ErrCustodyAttemptUnknown
	}
	return bytes.Clone(tx.raw), nil
}

// Recheck only, never repair/resubmit. Supplied provider "not found", a nonce
// observation or an unverified receipt must not turn unknown into retryable.
func (a *CustodyAttempt) Recheck(ctx context.Context) error {
	if a == nil || ctx == nil {
		return ErrCustodyAttemptState
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable || a.closed || ctx.Err() != nil {
		return ErrCustodyAttemptState
	}
	if a.checkStored() != nil {
		return ErrCustodyAttemptStorage
	}
	wanted, _, err := custodyAttemptCandidate(a.inputs)
	if err != nil || !sameCustodyAttempt(wanted, a.state) {
		a.usable = false
		return ErrCustodyAttemptState
	}
	if ctx.Err() != nil {
		return ErrCustodyAttemptState
	}
	return nil
}

func (a *CustodyAttempt) Close() (err error) {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed, a.usable = true, false
	defer func() {
		if recover() != nil {
			err = ErrCustodyAttemptStorage
		}
		if err != nil {
			a.uncertain = true
		}
	}()
	if a.store.Close() != nil {
		return ErrCustodyAttemptStorage
	}
	return nil
}
