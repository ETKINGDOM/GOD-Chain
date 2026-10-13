package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sync"
)

var ErrCustodyCreationJournal = errors.New("RH private creation journal refused; preserve original files and retained latest pin")

const maxCustodyCreationJournalBytes = 4096

// One cooperatively owned append-only simulation history, not a shared account
// nonce allocator, key controller or protection from whole-history rollback.
// Append must durably retain bytes before success; uncertain writes may remain.
type CustodyCreationJournalStorage interface {
	Load() ([][]byte, error)
	Append([]byte) error
	Close() error
}

type custodyCreationSnapshot struct {
	Version           uint32 `json:"version"`
	Sequence          uint32 `json:"sequence"`
	Purpose           string `json:"purpose"`
	PreviousSHA256    string `json:"previousSHA256"`
	UnsignedSHA256    string `json:"unsignedSHA256"`
	SignedSHA256      string `json:"signedSHA256"`
	SourceSHA256      string `json:"sourceSHA256"`
	RequestSHA256     string `json:"requestSHA256"`
	NodeSHA256        string `json:"nodeSHA256"`
	CompilerSHA256    string `json:"compilerSHA256"`
	Stage             string `json:"stage"`
	ObservationSHA256 string `json:"observationSHA256"`
	ReceiptSucceeded  bool   `json:"receiptSucceeded"`
}

type CustodyCreationJournalReport struct {
	SimulationOnly                 bool `json:"simulationOnly"`
	ExactSignedCreationRetained    bool `json:"exactSignedCreationRetained"`
	DispatchConsumed               bool `json:"dispatchConsumed"`
	DispatchOutcomeUnknown         bool `json:"dispatchOutcomeUnknown"`
	ConditionalObservationRetained bool `json:"conditionalObservationRetained"`
	ReceiptSucceeded               bool `json:"receiptSucceeded"`
	StorageUsable                  bool `json:"storageUsable"`
	WriteOutcomeUncertain          bool `json:"writeOutcomeUncertain"`
	SharedAccountNonceCoordinated  bool `json:"sharedAccountNonceCoordinated"`
	NonceReleased                  bool `json:"nonceReleased"`
	FeeReservationReleased         bool `json:"feeReservationReleased"`
	SigningEnabled                 bool `json:"signingEnabled"`
	BroadcastEnabled               bool `json:"broadcastEnabled"`
	RealAssetsReady                bool `json:"realAssetsReady"`
}

type CustodyCreationSimulationJournal struct {
	mu                        sync.Mutex
	files                     CustodyDeploymentFiles
	compiler                  TokenCompiler
	signedPath                string
	signedPin                 [32]byte
	checked                   checkedCustodyDeployment
	store                     CustodyCreationJournalStorage
	latest                    []byte
	state                     custodyCreationSnapshot
	usable, closed, uncertain bool
}

func (*CustodyCreationSimulationJournal) String() string {
	return "RH private creation simulation journal (redacted)"
}
func (j *CustodyCreationSimulationJournal) GoString() string { return j.String() }
func (j *CustodyCreationSimulationJournal) MarshalJSON() ([]byte, error) {
	return json.Marshal(j.Report())
}
func (j *CustodyCreationSimulationJournal) Report() CustodyCreationJournalReport {
	if j == nil {
		return CustodyCreationJournalReport{}
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.report()
}
func (j *CustodyCreationSimulationJournal) report() CustodyCreationJournalReport {
	return CustodyCreationJournalReport{SimulationOnly: true, ExactSignedCreationRetained: len(j.latest) > 0, DispatchConsumed: j.state.Sequence >= 2,
		DispatchOutcomeUnknown: j.state.Sequence >= 2, ConditionalObservationRetained: j.state.Sequence == 3, ReceiptSucceeded: j.state.ReceiptSucceeded, StorageUsable: j.usable && !j.closed, WriteOutcomeUncertain: j.uncertain}
}

func creationSnapshot(raw []byte) (custodyCreationSnapshot, error) {
	var s custodyCreationSnapshot
	if len(raw) == 0 || len(raw) > maxCustodyCreationJournalBytes {
		return s, ErrCustodyCreationJournal
	}
	if _, err := custodyObject(raw, []string{"version", "sequence", "purpose", "previousSHA256", "unsignedSHA256", "signedSHA256", "sourceSHA256", "requestSHA256", "nodeSHA256", "compilerSHA256", "stage", "observationSHA256", "receiptSucceeded"}, nil); err != nil {
		return s, ErrCustodyCreationJournal
	}
	if json.Unmarshal(raw, &s) != nil || s.Version != 1 || s.Sequence < 1 || s.Sequence > 3 || s.Purpose != "GOD Chain private custody creation simulation v1" {
		return custodyCreationSnapshot{}, ErrCustodyCreationJournal
	}
	for _, pin := range []string{s.UnsignedSHA256, s.SignedSHA256, s.SourceSHA256, s.RequestSHA256, s.NodeSHA256, s.CompilerSHA256} {
		if _, err := SourceMaterialPin(pin); err != nil {
			return custodyCreationSnapshot{}, ErrCustodyCreationJournal
		}
	}
	if s.Sequence == 1 && (s.Stage != "prepared" || s.PreviousSHA256 != "" || s.ObservationSHA256 != "" || s.ReceiptSucceeded) {
		return custodyCreationSnapshot{}, ErrCustodyCreationJournal
	}
	if s.Sequence >= 2 {
		if _, err := SourceMaterialPin(s.PreviousSHA256); err != nil {
			return custodyCreationSnapshot{}, ErrCustodyCreationJournal
		}
	}
	if s.Sequence == 2 && (s.Stage != "dispatch-unknown" || s.ObservationSHA256 != "" || s.ReceiptSucceeded) {
		return custodyCreationSnapshot{}, ErrCustodyCreationJournal
	}
	if s.Sequence == 3 {
		if _, err := SourceMaterialPin(s.ObservationSHA256); err != nil || s.Stage != "conditional-observation" {
			return custodyCreationSnapshot{}, ErrCustodyCreationJournal
		}
	}
	canonical, err := json.Marshal(s)
	if err != nil || !bytes.Equal(raw, canonical) {
		return custodyCreationSnapshot{}, ErrCustodyCreationJournal
	}
	return s, nil
}

func creationSnapshotSameIntent(a, b custodyCreationSnapshot) bool {
	return a.UnsignedSHA256 == b.UnsignedSHA256 && a.SignedSHA256 == b.SignedSHA256 && a.SourceSHA256 == b.SourceSHA256 && a.RequestSHA256 == b.RequestSHA256 && a.NodeSHA256 == b.NodeSHA256 && a.CompilerSHA256 == b.CompilerSHA256
}

func creationJournalRows(rows [][]byte, want custodyCreationSnapshot) (custodyCreationSnapshot, []byte, error) {
	if len(rows) < 1 || len(rows) > 3 {
		return custodyCreationSnapshot{}, nil, ErrCustodyCreationJournal
	}
	var previous []byte
	var latest custodyCreationSnapshot
	for i, raw := range rows {
		s, err := creationSnapshot(raw)
		if err != nil || s.Sequence != uint32(i+1) || !creationSnapshotSameIntent(want, s) || i > 0 && s.PreviousSHA256 != creationHexPin(sha256.Sum256(previous)) {
			return custodyCreationSnapshot{}, nil, ErrCustodyCreationJournal
		}
		previous = bytes.Clone(raw)
		latest = s
	}
	return latest, previous, nil
}

func OpenCustodyCreationJournalForSimulation(ctx context.Context, f CustodyDeploymentFiles, c TokenCompiler, signedPath string, signedPin [32]byte, store CustodyCreationJournalStorage, create bool, resumePin [32]byte) (*CustodyCreationSimulationJournal, error) {
	if store == nil || ctx == nil || ctx.Err() != nil || create && resumePin != ([32]byte{}) || !create && resumePin == ([32]byte{}) {
		return nil, ErrCustodyCreationJournal
	}
	checked, err := loadCustodyDeploymentTransaction(ctx, f, c, signedPath, signedPin)
	if err != nil {
		return nil, ErrCustodyCreationJournal
	}
	want := custodyCreationSnapshot{Version: 1, Sequence: 1, Purpose: "GOD Chain private custody creation simulation v1", Stage: "prepared",
		UnsignedSHA256: creationHexPin(sha256.Sum256(checked.deployment.raw)), SignedSHA256: creationHexPin(signedPin), SourceSHA256: creationHexPin(f.SourcePin), RequestSHA256: creationHexPin(f.RequestPin), NodeSHA256: creationHexPin(c.nodePin), CompilerSHA256: creationHexPin(c.compilerPin)}
	j := &CustodyCreationSimulationJournal{files: f, compiler: c, signedPath: signedPath, signedPin: signedPin, checked: checked, store: store, state: want}
	rows, err := store.Load()
	if err != nil || len(rows) > 3 || create && len(rows) != 0 || !create && len(rows) == 0 {
		return nil, ErrCustodyCreationJournal
	}
	if create {
		if ctx.Err() != nil {
			return nil, ErrCustodyCreationJournal
		}
		raw, _ := json.Marshal(want)
		if err := store.Append(bytes.Clone(raw)); err != nil {
			return nil, ErrCustodyCreationJournal
		}
		rows, err = store.Load()
		if err != nil || len(rows) != 1 || !bytes.Equal(rows[0], raw) {
			return nil, ErrCustodyCreationJournal
		}
	}
	state, previous, err := creationJournalRows(rows, want)
	if err != nil {
		return nil, ErrCustodyCreationJournal
	}
	j.state = state
	if !create && sha256.Sum256(previous) != resumePin || ctx.Err() != nil {
		return nil, ErrCustodyCreationJournal
	}
	j.latest, j.usable = previous, true
	if !j.recheck(ctx) {
		return nil, ErrCustodyCreationJournal
	}
	return j, nil
}

func (j *CustodyCreationSimulationJournal) recheck(ctx context.Context) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	if !j.usable || j.closed || ctx == nil || ctx.Err() != nil {
		return false
	}
	for _, file := range []struct {
		path       string
		pin        [32]byte
		limit      int64
		executable bool
	}{{j.files.SourcePath, j.files.SourcePin, 64 << 10, false}, {j.compiler.nodePath, j.compiler.nodePin, maxTokenRuntimeBytes, true}, {j.compiler.compilerPath, j.compiler.compilerPin, maxTokenCompilerBytes, false}} {
		if _, err := tokenCompilerFile(ctx, file.path, file.pin, file.limit, file.executable); err != nil {
			return false
		}
	}
	if _, err := readRHPrivateBytes(j.files.RequestPath, MaxCustodyRequestBytes, j.files.RequestPin); err != nil {
		return false
	}
	wire, err := readRHPrivateBytes(j.signedPath, MaxTokenCreationInputBytes+1024, j.signedPin)
	if err != nil || !bytes.Equal(wire, j.checked.raw) {
		return false
	}
	rows, err := j.store.Load()
	if err != nil {
		return false
	}
	state, latest, err := creationJournalRows(rows, j.state)
	return err == nil && state == j.state && bytes.Equal(latest, j.latest) && ctx.Err() == nil
}

func (j *CustodyCreationSimulationJournal) append(s custodyCreationSnapshot) (err error) {
	defer func() {
		if recover() != nil {
			j.usable, j.uncertain = false, true
			err = ErrCustodyCreationJournal
		}
	}()
	s.PreviousSHA256 = creationHexPin(sha256.Sum256(j.latest))
	raw, err := json.Marshal(s)
	if err != nil {
		return ErrCustodyCreationJournal
	}
	if _, err := creationSnapshot(raw); err != nil {
		return ErrCustodyCreationJournal
	}
	j.usable = false // any ambiguous append freezes this handle
	if j.store.Append(bytes.Clone(raw)) != nil {
		j.uncertain = true
		return ErrCustodyCreationJournal
	}
	rows, err := j.store.Load()
	if err != nil || len(rows) != int(s.Sequence) || !bytes.Equal(rows[len(rows)-1], raw) {
		j.uncertain = true
		return ErrCustodyCreationJournal
	}
	j.state, j.latest, j.usable = s, bytes.Clone(raw), true
	return nil
}

// Durable UNKNOWN precedes one private-byte export. There is no broadcaster or
// signer here. Reopen at UNKNOWN/observed can never export another submission.
func (j *CustodyCreationSimulationJournal) TakeForSimulation(ctx context.Context, expectedUnknownPin [32]byte) ([]byte, error) {
	if j == nil {
		return nil, ErrCustodyCreationJournal
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.recheck(ctx) || j.state.Sequence != 1 {
		return nil, ErrCustodyCreationJournal
	}
	s := j.state
	s.Sequence, s.Stage = 2, "dispatch-unknown"
	s.PreviousSHA256 = creationHexPin(sha256.Sum256(j.latest))
	expected, _ := json.Marshal(s)
	if expectedUnknownPin == ([32]byte{}) || sha256.Sum256(expected) != expectedUnknownPin {
		return nil, ErrCustodyCreationJournal
	}
	if j.append(s) != nil || !j.recheck(ctx) {
		return nil, ErrCustodyCreationJournal
	}
	return bytes.Clone(j.checked.raw), nil
}

// Independently retain this deterministic next-state pin BEFORE requesting a
// handoff. It permits exact UNKNOWN recovery if termination loses the return.
// No output bytes or journal transition are produced by this method.
func (j *CustodyCreationSimulationJournal) ExpectedHandoffPin(ctx context.Context) ([32]byte, error) {
	if j == nil {
		return [32]byte{}, ErrCustodyCreationJournal
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.recheck(ctx) || j.state.Sequence != 1 {
		return [32]byte{}, ErrCustodyCreationJournal
	}
	s := j.state
	s.Sequence, s.Stage, s.PreviousSHA256 = 2, "dispatch-unknown", creationHexPin(sha256.Sum256(j.latest))
	raw, err := json.Marshal(s)
	if err != nil {
		return [32]byte{}, ErrCustodyCreationJournal
	}
	return sha256.Sum256(raw), nil
}

func (j *CustodyCreationSimulationJournal) Observe(ctx context.Context, block Block, m BlockMaterial, p ContractStateProof) (CustodyCreationExecutionReport, error) {
	if j == nil {
		return CustodyCreationExecutionReport{}, ErrCustodyCreationJournal
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.recheck(ctx) || j.state.Sequence < 2 {
		return CustodyCreationExecutionReport{}, ErrCustodyCreationJournal
	}
	r, pin, err := checkCustodyCreationExecution(ctx, j.checked, block, m, p)
	if err != nil {
		return CustodyCreationExecutionReport{}, ErrCustodyCreationJournal
	}
	if !j.recheck(ctx) {
		return CustodyCreationExecutionReport{}, ErrCustodyCreationJournal
	}
	if j.state.Sequence == 3 {
		if j.state.ObservationSHA256 != creationHexPin(pin) || j.state.ReceiptSucceeded != r.ReceiptSucceeded {
			return CustodyCreationExecutionReport{}, ErrCustodyCreationJournal
		}
		return r, nil
	}
	s := j.state
	s.Sequence, s.Stage, s.ObservationSHA256, s.ReceiptSucceeded = 3, "conditional-observation", creationHexPin(pin), r.ReceiptSucceeded
	if j.append(s) != nil || !j.recheck(ctx) {
		return CustodyCreationExecutionReport{}, ErrCustodyCreationJournal
	}
	return r, nil
}

func (j *CustodyCreationSimulationJournal) RetainedStatePin(ctx context.Context) ([32]byte, error) {
	if j == nil {
		return [32]byte{}, ErrCustodyCreationJournal
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.recheck(ctx) {
		return [32]byte{}, ErrCustodyCreationJournal
	}
	return sha256.Sum256(j.latest), nil
}
func (j *CustodyCreationSimulationJournal) Close() error {
	if j == nil {
		return ErrCustodyCreationJournal
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	j.closed, j.usable = true, false
	if j.store.Close() != nil {
		return ErrCustodyCreationJournal
	}
	return nil
}
