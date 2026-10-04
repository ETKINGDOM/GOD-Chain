package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sync"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
)

// Local prototype bounds, not approved production relay parameters.
const (
	MaxRelayTasks      = 256
	MaxRelayAttempts   = 8
	MaxRelayStateBytes = 1 << 20
	RelayReadTimeout   = 30 * time.Second
)

var (
	ErrRelayState       = errors.New("RH read-only journal state or simulation binding rejected")
	ErrRelayStorage     = errors.New("RH read-only journal storage failed; reopen and review before continuing")
	ErrRelayRequest     = errors.New("RH read-only journal request rejected")
	ErrRelayConflict    = errors.New("RH read-only journal request conflicts with retained task")
	ErrRelayLimit       = errors.New("RH read-only journal workload or retry limit reached")
	ErrRelayNoWork      = errors.New("RH read-only journal has no eligible observation task")
	ErrRelayObservation = errors.New("RH read-only journal observation failed; no approval or asset movement occurred")
)

// RelayStore must exclusively own its storage and atomically commit a detached
// payload durably before Save returns. Any Save error has an UNKNOWN outcome.
// Private payloads must not be logged/published. A successful journal constructor
// takes ownership of Close; caller owns the store after constructor failure.
type RelayStore interface {
	Load() ([]byte, error)
	Save([]byte) error
	Close() error
}

type RelayStage string

const (
	RelayPending   RelayStage = "pending"
	RelayObserving RelayStage = "observing"
	RelayCached    RelayStage = "cached_unsigned"
	RelayExhausted RelayStage = "exhausted"
)

type RelayTaskReport struct {
	Ticket                      uint64     `json:"ticket"`
	Kind                        string     `json:"kind"`
	Stage                       RelayStage `json:"stage"`
	Attempts                    uint32     `json:"attempts"`
	NextAttemptAt               time.Time  `json:"nextAttemptAt"`
	CachedUnsignedObservation   bool       `json:"cachedUnsignedObservation"`
	IndependentFinalityVerified bool       `json:"independentFinalityVerified"`
	ApprovalReady               bool       `json:"approvalReady"`
	RealAssetsReady             bool       `json:"realAssetsReady"`
}

type RelayReport struct {
	Usable                      bool   `json:"usable"`
	SimulationOnly              bool   `json:"simulationOnly"`
	Tasks                       int    `json:"tasks"`
	Pending                     int    `json:"pending"`
	Observing                   int    `json:"observing"`
	CachedUnsignedObservations  int    `json:"cachedUnsignedObservations"`
	Exhausted                   int    `json:"exhausted"`
	DiscoveryEnabled            bool   `json:"discoveryEnabled"`
	ScannedThroughHeight        uint64 `json:"scannedThroughHeight"`
	IndependentFinalityVerified bool   `json:"independentFinalityVerified"`
	ApprovalReady               bool   `json:"approvalReady"`
	SigningEnabled              bool   `json:"signingEnabled"`
	BroadcastEnabled            bool   `json:"broadcastEnabled"`
	RealAssetsReady             bool   `json:"realAssetsReady"`
}

// RelayJournal schedules READS only. Its private cache is neither a receipt
// proof nor an approved/outgoing transaction. No signer or ledger is installed.
type RelayJournal struct {
	mu     sync.Mutex
	config Config
	store  RelayStore
	state  relayState
	usable bool
	closed bool
}

func (*RelayJournal) String() string                 { return "RH read-only journal (redacted)" }
func (j *RelayJournal) GoString() string             { return j.String() }
func (j *RelayJournal) MarshalJSON() ([]byte, error) { return json.Marshal(j.Report()) }

type relayRequest struct {
	Kind        string
	Transaction [32]byte
	LogIndex    uint32
	Recipient   [20]byte
	Amount      string
	Withdrawal  *godbridge.Withdrawal
}

type relayCache struct {
	Sequence   uint64
	Evidence   godbridge.Evidence
	Digest     [32]byte
	Checkpoint Block
}

type relayTask struct {
	Ticket    uint64
	Request   relayRequest
	CreatedAt time.Time
	NextAt    time.Time
	Attempts  uint32
	Stage     RelayStage
	Cache     *relayCache
}

type relayState struct {
	Version   uint32
	Binding   [32]byte
	Clock     time.Time
	Tasks     []relayTask
	Discovery *relayCursor `json:",omitempty"`
}

func relayBinding(c Config) ([32]byte, error) {
	if c.private.Mode != "simulation" {
		return [32]byte{}, ErrRelayState
	}
	binding, err := c.Binding()
	if err != nil {
		return [32]byte{}, ErrRelayState
	}
	token, _ := hash(c.private.ExpectedTokenCodeHash)
	custody, _ := hash(c.private.ExpectedCustodyCodeHash)
	raw, err := json.Marshal(struct {
		Purpose                string
		Version                uint32
		Config                 godbridge.Config
		TokenCode, CustodyCode [32]byte
	}{"GOD Chain read-only simulation journal", 1, binding, token, custody})
	if err != nil {
		return [32]byte{}, ErrRelayState
	}
	return sha256.Sum256(raw), nil
}

// NewRelayJournal cannot initialize over an existing payload or resume missing
// state. Recovery retains attempt budgets and retry times, never creates a new
// binding, and only converts interrupted read attempts back to eligible work.
func NewRelayJournal(c Config, store RelayStore, create bool) (*RelayJournal, error) {
	binding, err := relayBinding(c)
	if err != nil || store == nil {
		return nil, ErrRelayState
	}
	raw, err := store.Load()
	if err != nil {
		return nil, ErrRelayStorage
	}
	j := &RelayJournal{config: c, store: store, usable: true}
	if create {
		if len(raw) != 0 {
			return nil, ErrRelayState
		}
		j.state = relayState{Version: 1, Binding: binding, Tasks: []relayTask{}}
		if j.write(j.state) != nil {
			return nil, ErrRelayStorage
		}
		return j, nil
	}
	s, err := decodeRelayState(raw)
	if err != nil || s.Binding != binding || j.validate(s) != nil {
		return nil, ErrRelayState
	}
	j.state = s
	recovered := false
	for i := range s.Tasks {
		if s.Tasks[i].Stage != RelayObserving {
			continue
		}
		s.Tasks[i].Stage = RelayPending
		if s.Tasks[i].Attempts == MaxRelayAttempts {
			s.Tasks[i].Stage, s.Tasks[i].NextAt = RelayExhausted, time.Time{}
		}
		recovered = true
	}
	if recovered && j.write(s) != nil {
		return nil, ErrRelayStorage
	}
	return j, nil
}

func (j *RelayJournal) Report() RelayReport {
	r := RelayReport{SimulationOnly: true}
	if j == nil {
		return r
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	r.Usable = j.usable && !j.closed
	r.Tasks = len(j.state.Tasks)
	if j.state.Discovery != nil {
		r.DiscoveryEnabled = true
		r.ScannedThroughHeight = j.state.Discovery.Through.Height
	}
	for _, task := range j.state.Tasks {
		switch task.Stage {
		case RelayPending:
			r.Pending++
		case RelayObserving:
			r.Observing++
		case RelayCached:
			r.CachedUnsignedObservations++
		case RelayExhausted:
			r.Exhausted++
		}
	}
	return r
}

func taskReport(t relayTask) RelayTaskReport {
	return RelayTaskReport{Ticket: t.Ticket, Kind: t.Request.Kind, Stage: t.Stage,
		Attempts: t.Attempts, NextAttemptAt: t.NextAt, CachedUnsignedObservation: t.Cache != nil}
}

func (j *RelayJournal) Task(ticket uint64) (RelayTaskReport, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if !j.usable || j.closed || ticket == 0 || ticket > uint64(len(j.state.Tasks)) {
		return RelayTaskReport{}, ErrRelayState
	}
	return taskReport(j.state.Tasks[ticket-1]), nil
}

func (j *RelayJournal) QueueDeposit(r DepositRequest, at time.Time) (uint64, error) {
	if r.TransactionHash == [32]byte{} || r.Recipient == [20]byte{} || r.Amount.IsNil() ||
		!r.Amount.IsPositive() || r.Amount.GT(godbridge.TransferLimit()) {
		return 0, ErrRelayRequest
	}
	return j.queue(relayRequest{Kind: "deposit", Transaction: r.TransactionHash, LogIndex: r.LogIndex,
		Recipient: r.Recipient, Amount: r.Amount.String()}, at)
}

func (j *RelayJournal) QueueResolution(r ResolutionRequest, at time.Time) (uint64, error) {
	_, w, err := resolutionRequest(j.config, r)
	if err != nil {
		return 0, ErrRelayRequest
	}
	return j.queue(relayRequest{Kind: string(r.Outcome), Transaction: r.TransactionHash, LogIndex: r.LogIndex, Withdrawal: &w}, at)
}

func (j *RelayJournal) queue(r relayRequest, at time.Time) (uint64, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	at = at.Round(0).UTC()
	if !j.usable || j.closed || !relayTime(at) || at.Before(j.state.Clock) {
		return 0, ErrRelayState
	}
	next := j.copyState()
	ticket, added, err := appendRelayRequest(&next, r, at)
	if err != nil || !added {
		return ticket, err
	}
	next.Clock = at
	if err := j.write(next); err != nil {
		return 0, err
	}
	return ticket, nil
}

// appendRelayRequest changes only a detached candidate state. Discovery uses
// this same admission rule before atomically saving all tasks and its cursor.
func appendRelayRequest(next *relayState, r relayRequest, at time.Time) (uint64, bool, error) {
	raw, _ := json.Marshal(r)
	for _, t := range next.Tasks {
		old := t.Request
		if old.Transaction == r.Transaction && old.LogIndex == r.LogIndex {
			previous, _ := json.Marshal(old)
			if bytes.Equal(previous, raw) {
				return t.Ticket, false, nil
			}
			return 0, false, ErrRelayConflict
		}
		if old.Withdrawal != nil && r.Withdrawal != nil && old.Withdrawal.ID == r.Withdrawal.ID {
			return 0, false, ErrRelayConflict
		}
	}
	if len(next.Tasks) == MaxRelayTasks {
		return 0, false, ErrRelayLimit
	}
	ticket := uint64(len(next.Tasks) + 1)
	next.Tasks = append(next.Tasks, relayTask{Ticket: ticket, Request: r, CreatedAt: at, NextAt: at, Stage: RelayPending})
	return ticket, true, nil
}

// Refresh discards only the cached unsigned observation, not its request,
// ticket or lifetime retry budget. It does not authorize replay/asset release.
func (j *RelayJournal) Refresh(ticket uint64, at time.Time) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	at = at.Round(0).UTC()
	if !j.usable || j.closed || !relayTime(at) || at.Before(j.state.Clock) || ticket == 0 || ticket > uint64(len(j.state.Tasks)) {
		return ErrRelayState
	}
	t := j.state.Tasks[ticket-1]
	if t.Stage != RelayCached {
		return ErrRelayState
	}
	if t.Attempts == MaxRelayAttempts {
		return ErrRelayLimit
	}
	next := j.copyState()
	next.Clock = at
	next.Tasks[ticket-1].Stage, next.Tasks[ticket-1].NextAt, next.Tasks[ticket-1].Cache = RelayPending, at, nil
	return j.write(next)
}

// ObserveNext serializes one bounded read while retaining the journal lock.
// Adapters must honor context cancellation. The supplied monotonic local clock
// is not consensus time. Retry is conservatively scheduled after the entire
// read-time budget plus backoff, even when an error returns immediately.
func (j *RelayJournal) ObserveNext(ctx context.Context, source ReceiptSource, at time.Time) (RelayTaskReport, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	at = at.Round(0).UTC()
	if !j.usable || j.closed || ctx == nil || ctx.Err() != nil || source == nil ||
		!relayTime(at) || at.Before(j.state.Clock) {
		return RelayTaskReport{}, ErrRelayState
	}
	index := -1
	for i, task := range j.state.Tasks {
		if task.Stage == RelayPending && !at.Before(task.NextAt) {
			index = i
			break
		}
	}
	if index < 0 {
		return RelayTaskReport{}, ErrRelayNoWork
	}
	next := j.copyState()
	next.Clock = at
	task := &next.Tasks[index]
	task.Attempts++
	task.Stage = RelayObserving
	task.NextAt = at.Add(RelayReadTimeout + relayBackoff(task.Attempts))
	if err := j.write(next); err != nil {
		return RelayTaskReport{}, err
	}
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	cache, err := j.observe(readCtx, source, task.Request)
	if readCtx.Err() != nil {
		cache, err = nil, ErrRelayObservation
	}
	next = j.copyState()
	task = &next.Tasks[index]
	if err == nil {
		task.Stage, task.NextAt, task.Cache = RelayCached, time.Time{}, cache
	} else if task.Attempts == MaxRelayAttempts {
		task.Stage, task.NextAt = RelayExhausted, time.Time{}
	} else {
		task.Stage = RelayPending
	}
	if writeErr := j.write(next); writeErr != nil {
		return RelayTaskReport{}, writeErr
	}
	if err != nil {
		return taskReport(*task), ErrRelayObservation
	}
	return taskReport(*task), nil
}

func (j *RelayJournal) observe(ctx context.Context, source ReceiptSource, r relayRequest) (result *relayCache, err error) {
	// Provider panics are contained without printing their private payload.
	defer func() {
		if recover() != nil {
			result, err = nil, ErrRelayObservation
		}
	}()
	if r.Kind == "deposit" {
		amount, _ := sdkmath.NewIntFromString(r.Amount)
		o, err := ObserveDeposit(ctx, j.config, source, DepositRequest{r.Transaction, r.LogIndex, r.Recipient, amount})
		if err != nil {
			return nil, ErrRelayObservation
		}
		d, digest, checkpoint, err := o.Proposal()
		if err != nil {
			return nil, ErrRelayObservation
		}
		return &relayCache{d.Sequence, d.Evidence, digest, checkpoint}, nil
	}
	o, err := ObserveResolution(ctx, j.config, source, ResolutionRequest{r.Transaction, r.LogIndex, *r.Withdrawal, godbridge.Status(r.Kind)})
	if err != nil {
		return nil, ErrRelayObservation
	}
	w, _, evidence, digest, checkpoint, err := o.Proposal()
	if err != nil {
		return nil, ErrRelayObservation
	}
	return &relayCache{w.Sequence, evidence, digest, checkpoint}, nil
}

func (j *RelayJournal) copyState() relayState {
	next := j.state
	next.Tasks = append([]relayTask{}, j.state.Tasks...)
	if j.state.Discovery != nil {
		cursor := *j.state.Discovery
		next.Discovery = &cursor
	}
	return next
}

func (j *RelayJournal) write(s relayState) error {
	if j.validate(s) != nil {
		j.usable = false
		return ErrRelayState
	}
	raw, err := encodeRelayState(s)
	if err != nil {
		j.usable = false
		return ErrRelayState
	}
	if j.store.Save(raw) != nil {
		j.usable = false
		return ErrRelayStorage
	}
	j.state = s
	return nil
}

func (j *RelayJournal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return nil
	}
	j.closed, j.usable = true, false
	if j.store.Close() != nil {
		return ErrRelayStorage
	}
	return nil
}

func relayTime(at time.Time) bool { return at.Unix() > 0 && at.UTC().Year() <= 9998 }
func relayBackoff(attempt uint32) time.Duration {
	delay := time.Second * time.Duration(1<<(attempt-1))
	if delay > time.Minute {
		return time.Minute
	}
	return delay
}

func encodeRelayState(s relayState) ([]byte, error) {
	raw, err := json.Marshal(s)
	if err != nil || len(raw)+sha256.Size > MaxRelayStateBytes {
		return nil, ErrRelayState
	}
	sum := sha256.Sum256(raw)
	return append(raw, sum[:]...), nil
}

func decodeRelayState(raw []byte) (relayState, error) {
	if len(raw) <= sha256.Size || len(raw) > MaxRelayStateBytes {
		return relayState{}, ErrRelayState
	}
	payload := raw[:len(raw)-sha256.Size]
	sum := sha256.Sum256(payload)
	if !bytes.Equal(sum[:], raw[len(payload):]) {
		return relayState{}, ErrRelayState
	}
	var s relayState
	if json.Unmarshal(payload, &s) != nil {
		return relayState{}, ErrRelayState
	}
	canonical, err := json.Marshal(s)
	if err != nil || !bytes.Equal(canonical, payload) {
		return relayState{}, ErrRelayState
	}
	return s, nil
}

func (j *RelayJournal) validate(s relayState) error {
	binding, err := relayBinding(j.config)
	if err != nil || s.Version != 1 || s.Binding != binding || len(s.Tasks) > MaxRelayTasks || s.Tasks == nil ||
		(len(s.Tasks) == 0 && s.Discovery == nil && !s.Clock.IsZero()) ||
		((len(s.Tasks) > 0 || s.Discovery != nil) && !relayTime(s.Clock)) || !validRelayCursor(s.Discovery) {
		return ErrRelayState
	}
	protocol, _ := j.config.Binding()
	for i, task := range s.Tasks {
		if task.Ticket != uint64(i+1) || task.Attempts > MaxRelayAttempts || !relayTime(task.CreatedAt) ||
			task.CreatedAt.After(s.Clock) || j.validRequest(task.Request) != nil {
			return ErrRelayState
		}
		for _, earlier := range s.Tasks[:i] {
			if earlier.Request.Transaction == task.Request.Transaction && earlier.Request.LogIndex == task.Request.LogIndex ||
				earlier.Request.Withdrawal != nil && task.Request.Withdrawal != nil && earlier.Request.Withdrawal.ID == task.Request.Withdrawal.ID {
				return ErrRelayState
			}
		}
		switch task.Stage {
		case RelayPending, RelayObserving:
			if !relayTime(task.NextAt) || task.NextAt.Before(task.CreatedAt) || task.Cache != nil ||
				task.Stage == RelayPending && task.Attempts == MaxRelayAttempts || task.Stage == RelayObserving && task.Attempts == 0 {
				return ErrRelayState
			}
		case RelayExhausted:
			if task.Attempts != MaxRelayAttempts || task.Cache != nil || !task.NextAt.IsZero() {
				return ErrRelayState
			}
		case RelayCached:
			cache := task.Cache
			if task.Attempts == 0 || cache == nil || !task.NextAt.IsZero() || cache.Sequence == 0 || cache.Digest == [32]byte{} ||
				cache.Evidence.TransactionHash != task.Request.Transaction || cache.Evidence.LogIndex != task.Request.LogIndex ||
				cache.Checkpoint.Height < cache.Evidence.Height || cache.Checkpoint.Hash == [32]byte{} ||
				cache.Checkpoint.Height == cache.Evidence.Height && cache.Checkpoint.Hash != cache.Evidence.BlockHash {
				return ErrRelayState
			}
			var digest [32]byte
			if task.Request.Kind == "deposit" {
				amount, _ := sdkmath.NewIntFromString(task.Request.Amount)
				digest, err = godbridge.DepositAttestationDigest(protocol, godbridge.Deposit{
					Sequence: cache.Sequence, Recipient: task.Request.Recipient, Amount: amount, Evidence: cache.Evidence})
			} else {
				if cache.Sequence != task.Request.Withdrawal.Sequence {
					return ErrRelayState
				}
				digest, err = godbridge.ResolutionAttestationDigest(protocol, *task.Request.Withdrawal, godbridge.Status(task.Request.Kind), cache.Evidence)
			}
			if err != nil || digest != cache.Digest {
				return ErrRelayState
			}
		default:
			return ErrRelayState
		}
	}
	return nil
}

func (j *RelayJournal) validRequest(r relayRequest) error {
	if r.Transaction == [32]byte{} {
		return ErrRelayRequest
	}
	if r.Kind == "deposit" {
		amount, ok := sdkmath.NewIntFromString(r.Amount)
		if !ok || amount.IsNil() || !amount.IsPositive() || amount.GT(godbridge.TransferLimit()) || amount.String() != r.Amount ||
			r.Recipient == [20]byte{} || r.Withdrawal != nil {
			return ErrRelayRequest
		}
		return nil
	}
	if r.Withdrawal == nil || r.Recipient != [20]byte{} || r.Amount != "" {
		return ErrRelayRequest
	}
	_, _, err := resolutionRequest(j.config, ResolutionRequest{r.Transaction, r.LogIndex, *r.Withdrawal, godbridge.Status(r.Kind)})
	if err != nil {
		return ErrRelayRequest
	}
	return nil
}
