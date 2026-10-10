package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
)

const MaxCustodyRecordBytes = 8 << 10

const custodyRecordPurpose = "GOD Chain offline unsigned custody record v1"

var (
	ErrCustodyRecordState   = errors.New("RH offline custody record or immutable binding rejected")
	ErrCustodyRecordStorage = errors.New("RH offline custody record storage outcome uncertain; close, reopen and review")
	ErrCustodyRecordFile    = errors.New("RH unsigned custody file requires independent recovery review; no overwrite or submission")
	ErrCustodyRecordReport  = errors.New("RH offline custody report delivery failed; reopen the original record and review")
)

type CustodyRecordStage string

const (
	CustodyReserved CustodyRecordStage = "reserved_unsigned"
	CustodyRetained CustodyRecordStage = "retained_unsigned"
)

// Private canonical storage only. This records an unsigned file's lifecycle,
// not a financial attempt, source nonce, transaction, approval or payment.
type custodyRecordState struct {
	Purpose             string             `json:"purpose"`
	Version             uint32             `json:"version"`
	ConfigurationSHA256 string             `json:"configurationSHA256"`
	RequestSHA256       string             `json:"requestSHA256"`
	PacketSHA256        string             `json:"packetSHA256"`
	OutputPath          string             `json:"outputPath"`
	Action              CustodyAction      `json:"action"`
	Stage               CustodyRecordStage `json:"stage"`
}

type CustodyRecordReport struct {
	Call                  CustodyCallReport  `json:"call"`
	Stage                 CustodyRecordStage `json:"stage"`
	StorageUsable         bool               `json:"storageUsable"`
	ReservationRetained   bool               `json:"reservationRetained"`
	UnsignedFileRecorded  bool               `json:"unsignedFileRecorded"`
	RetainedFileMatched   bool               `json:"retainedFileMatched"`
	NeedsFileReview       bool               `json:"needsFileReview"`
	WriteOutcomeUncertain bool               `json:"writeOutcomeUncertain"`
}

// Successful Save must synchronously retain one complete detached snapshot in
// exclusively owned private storage. An error can leave old, new or partial
// state; Load must not silently repair/reset it. A constructor owns Close only
// after success. This is separate from the read-only journal's storage contract.
type CustodyRecordStorage interface {
	Load() ([]byte, error)
	Save([]byte) error
	Close() error
}

// One immutable slot, never shared with a relay journal or another record. Store callbacks
// must not reenter. The constructor owns Close only after success. Repeated
// export is idempotent in THIS slot; separate stores or legacy file commands
// are not fenced. No global duplicate prevention or source execution is claimed.
type CustodyRecord struct {
	mu                        sync.Mutex
	inputs                    CustodyFileInputs
	store                     CustodyRecordStorage
	state                     custodyRecordState
	call                      CustodyCallReport
	usable, closed, uncertain bool
	fileBlocked, matched      bool
}

func (*CustodyRecord) String() string                 { return "RH offline unsigned custody record (redacted)" }
func (r *CustodyRecord) GoString() string             { return r.String() }
func (r *CustodyRecord) MarshalJSON() ([]byte, error) { return json.Marshal(r.Report()) }
func (r *CustodyRecord) Report() CustodyRecordReport {
	if r == nil {
		return CustodyRecordReport{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.report()
}
func (r *CustodyRecord) report() CustodyRecordReport {
	usable := r.usable && !r.closed
	return CustodyRecordReport{Call: r.call, Stage: r.state.Stage, StorageUsable: usable,
		ReservationRetained: usable, UnsignedFileRecorded: usable && r.state.Stage == CustodyRetained,
		RetainedFileMatched: usable && r.matched, NeedsFileReview: usable && (r.state.Stage == CustodyReserved || r.fileBlocked), WriteOutcomeUncertain: r.uncertain}
}

func custodyOutputPath(path string) bool {
	return len(path) > 0 && len(path) <= 4096 && !strings.ContainsRune(path, 0) && filepath.IsAbs(path) && filepath.Clean(path) == path && filepath.Dir(path) != string(filepath.Separator)
}

func encodeCustodyRecord(s custodyRecordState) ([]byte, error) {
	if s.Purpose != custodyRecordPurpose || s.Version != 1 || !custodyOutputPath(s.OutputPath) ||
		(s.Stage != CustodyReserved && s.Stage != CustodyRetained) ||
		(s.Action != CustodyDeposit && s.Action != CustodyPay && s.Action != CustodyCancel && s.Action != CustodyPause) {
		return nil, ErrCustodyRecordState
	}
	for _, pin := range []string{s.ConfigurationSHA256, s.RequestSHA256, s.PacketSHA256} {
		if _, err := SourceMaterialPin(pin); err != nil {
			return nil, ErrCustodyRecordState
		}
	}
	raw, err := json.Marshal(s)
	if err != nil || len(raw) > MaxCustodyRecordBytes {
		return nil, ErrCustodyRecordState
	}
	return raw, nil
}

func decodeCustodyRecord(raw []byte) (custodyRecordState, error) {
	var s custodyRecordState
	if len(raw) == 0 || len(raw) > MaxCustodyRecordBytes || json.Unmarshal(raw, &s) != nil {
		return custodyRecordState{}, ErrCustodyRecordState
	}
	canonical, err := encodeCustodyRecord(s)
	if err != nil || !bytes.Equal(raw, canonical) {
		return custodyRecordState{}, ErrCustodyRecordState
	}
	return s, nil
}

func sameCustodyRecord(a, b custodyRecordState) bool {
	a.Stage, b.Stage = CustodyReserved, CustodyReserved
	return a == b
}

func custodyRecordCandidate(inputs CustodyFileInputs, outputPath string) (custodyRecordState, CustodyCallReport, error) {
	if !custodyOutputPath(outputPath) {
		return custodyRecordState{}, CustodyCallReport{}, ErrCustodyRecordState
	}
	call, err := loadCustodyInputs(inputs)
	if err != nil {
		return custodyRecordState{}, CustodyCallReport{}, ErrCustodyRecordState
	}
	raw, err := custodyPacketBytes(inputs, call)
	if err != nil {
		return custodyRecordState{}, CustodyCallReport{}, ErrCustodyRecordState
	}
	packetPin := sha256.Sum256(raw)
	s := custodyRecordState{Purpose: custodyRecordPurpose, Version: 1,
		ConfigurationSHA256: hex.EncodeToString(inputs.ConfigurationSHA256[:]), RequestSHA256: hex.EncodeToString(inputs.RequestSHA256[:]),
		PacketSHA256: hex.EncodeToString(packetPin[:]), OutputPath: outputPath, Action: call.Report().Action, Stage: CustodyReserved}
	return s, call.Report(), nil
}

// Initialization durably reserves the original request/output BEFORE export.
// Reopen never rewrites state, changes binding or repairs missing/partial files.
// A retained state must match its actual output file. A reserved state requires
// explicit Export or Recover; no output is written automatically on startup.
func NewCustodyRecord(ctx context.Context, inputs CustodyFileInputs, outputPath string, store CustodyRecordStorage, create bool) (record *CustodyRecord, err error) {
	defer func() {
		if recover() != nil {
			record, err = nil, ErrCustodyRecordStorage
		}
	}()
	if ctx == nil || ctx.Err() != nil || store == nil {
		return nil, ErrCustodyRecordState
	}
	wanted, call, err := custodyRecordCandidate(inputs, outputPath)
	if err != nil {
		return nil, ErrCustodyRecordState
	}
	raw, err := store.Load()
	if err != nil {
		return nil, ErrCustodyRecordStorage
	}
	if len(raw) > MaxCustodyRecordBytes {
		return nil, ErrCustodyRecordState
	}
	r := &CustodyRecord{inputs: inputs, store: store, state: wanted, call: call, usable: true}
	if create {
		if len(raw) != 0 || checkCustodyNewOutput(outputPath) != nil || ctx.Err() != nil {
			return nil, ErrCustodyRecordState
		}
		if r.write(wanted) != nil {
			return nil, ErrCustodyRecordStorage
		}
		return r, nil
	}
	s, err := decodeCustodyRecord(bytes.Clone(raw))
	if err != nil || !sameCustodyRecord(s, wanted) {
		return nil, ErrCustodyRecordState
	}
	r.state = s
	if s.Stage == CustodyRetained {
		if r.matchFile(ctx) != nil {
			return nil, ErrCustodyRecordFile
		}
		r.matched = true
	}
	if ctx.Err() != nil {
		return nil, ErrCustodyRecordState
	}
	return r, nil
}

// A Save error/panic has an UNKNOWN outcome, not a rollback. The current
// instance is unusable until closed and reopened against the retained state.
func (r *CustodyRecord) write(next custodyRecordState) (err error) {
	raw, err := encodeCustodyRecord(next)
	if err != nil {
		return ErrCustodyRecordState
	}
	defer func() {
		if recover() != nil {
			err = ErrCustodyRecordStorage
		}
		if err != nil {
			r.usable, r.uncertain, r.matched = false, true, false
		}
	}()
	if r.store.Save(bytes.Clone(raw)) != nil {
		return ErrCustodyRecordStorage
	}
	r.state = next
	return nil
}

func (r *CustodyRecord) matchFile(ctx context.Context) error {
	if ctx.Err() != nil {
		return ErrCustodyRecordState
	}
	pin, err := SourceMaterialPin(r.state.PacketSHA256)
	if err != nil {
		return ErrCustodyRecordState
	}
	if _, err = CheckCustodyCallFile(r.inputs, r.state.OutputPath, pin); err != nil {
		return ErrCustodyRecordFile
	}
	if ctx.Err() != nil {
		return ErrCustodyRecordState
	}
	return nil
}

// Reread the exact reservation/stage before file operations. Missing, changed,
// unreadable or panicking storage disables this instance WITHOUT calling Save
// or claiming a failed read was an uncertain write. This is not hostile-owner
// or rollback protection; a separately reopened older valid snapshot remains
// unauthenticated historical state.
func (r *CustodyRecord) checkStored() (err error) {
	defer func() {
		if recover() != nil {
			err = ErrCustodyRecordStorage
		}
		if err != nil {
			r.usable, r.matched = false, false
		}
	}()
	raw, err := r.store.Load()
	if err != nil || len(raw) > MaxCustodyRecordBytes {
		return ErrCustodyRecordStorage
	}
	s, err := decodeCustodyRecord(raw)
	if err != nil || s != r.state {
		return ErrCustodyRecordStorage
	}
	return nil
}

// Export creates only the original fixed path. Existing output is NEVER
// overwritten or implicitly adopted from a reserved state. After any failed
// file operation this instance permits only recovery review, not another export.
func (r *CustodyRecord) Export(ctx context.Context) (CustodyRecordReport, error) {
	if r == nil || ctx == nil {
		return CustodyRecordReport{}, ErrCustodyRecordState
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.usable || r.closed || ctx.Err() != nil {
		return CustodyRecordReport{}, ErrCustodyRecordState
	}
	if err := r.checkStored(); err != nil {
		return CustodyRecordReport{}, err
	}
	if r.state.Stage == CustodyRetained {
		return r.recover(ctx)
	}
	if r.fileBlocked {
		return CustodyRecordReport{}, ErrCustodyRecordFile
	}
	wanted, _, err := custodyRecordCandidate(r.inputs, r.state.OutputPath)
	if err != nil || !sameCustodyRecord(wanted, r.state) {
		return CustodyRecordReport{}, ErrCustodyRecordState
	}
	if ctx.Err() != nil {
		return CustodyRecordReport{}, ErrCustodyRecordState
	}
	r.matched, r.fileBlocked = false, true
	if _, err = PrepareCustodyCallFile(r.inputs, r.state.OutputPath); err != nil {
		return CustodyRecordReport{}, ErrCustodyRecordFile
	}
	return r.recover(ctx)
}

// Explicit recovery compares exact original pins, reconstructed bytes and the
// fixed retained path. It creates no output. Only a verified complete file can
// durably advance reserved_unsigned -> retained_unsigned. Repeats write nothing.
func (r *CustodyRecord) Recover(ctx context.Context) (CustodyRecordReport, error) {
	if r == nil || ctx == nil {
		return CustodyRecordReport{}, ErrCustodyRecordState
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.usable || r.closed || ctx.Err() != nil {
		return CustodyRecordReport{}, ErrCustodyRecordState
	}
	return r.recover(ctx)
}

func (r *CustodyRecord) recover(ctx context.Context) (CustodyRecordReport, error) {
	r.matched = false
	if err := r.checkStored(); err != nil {
		return CustodyRecordReport{}, err
	}
	if err := r.matchFile(ctx); err != nil {
		r.fileBlocked = true
		return CustodyRecordReport{}, err
	}
	if r.state.Stage == CustodyReserved {
		next := r.state
		next.Stage = CustodyRetained
		if ctx.Err() != nil {
			return CustodyRecordReport{}, ErrCustodyRecordState
		}
		if r.write(next) != nil {
			return CustodyRecordReport{}, ErrCustodyRecordStorage
		}
	}
	// Once Save succeeds, acknowledge it even if context cancellation occurs
	// inside the store's atomic write. A new operation must recheck its context.
	r.matched, r.fileBlocked = true, false
	return r.report(), nil
}

func (r *CustodyRecord) Close() (err error) {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed, r.usable, r.matched = true, false, false
	defer func() {
		if recover() != nil {
			err = ErrCustodyRecordStorage
		}
		if err != nil {
			r.uncertain = true
		}
	}()
	if r.store.Close() != nil {
		return ErrCustodyRecordStorage
	}
	return nil
}
