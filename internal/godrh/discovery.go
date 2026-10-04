package godrh

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// Local scanning bounds, not approved mainnet or provider throughput settings.
const (
	MaxDiscoveryBlocks      = 8
	MaxDiscoveryLogs        = 64
	MaxDiscoveryBatchEvents = 128
)

var ErrDiscovery = errors.New("RH simulation event discovery rejected; no cursor advance or financial approval")

// SourceLog contains PRIVATE candidate data, not receipt/finality evidence.
type SourceLog struct {
	Block            Block
	TransactionHash  [32]byte
	TransactionIndex uint32
	Log              ReceiptLog
}

func (SourceLog) String() string               { return "RH discovered candidate log (redacted)" }
func (l SourceLog) GoString() string           { return l.String() }
func (SourceLog) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type DiscoverySource interface {
	Source
	Logs(context.Context, [20]byte, Block) ([]SourceLog, error)
}

// NativeWithdrawalSource is a replaceable READ interface. The result's full ID
// is checked, not its committed native-state authenticity or finality. Adapters
// must honor context cancellation and must not sign, mutate or broadcast.
type NativeWithdrawalSource interface {
	Withdrawal(context.Context, uint64, [32]byte) (godbridge.Withdrawal, error)
}

type DiscoveryReport struct {
	SimulationOnly              bool   `json:"simulationOnly"`
	ScannedBlocks               int    `json:"scannedBlocks"`
	ReceiptSetsMatched          int    `json:"receiptSetsMatched"`
	CandidateEvents             int    `json:"candidateEvents"`
	AddedTasks                  int    `json:"addedTasks"`
	RetainedTasks               int    `json:"retainedTasks"`
	ScannedThroughHeight        uint64 `json:"scannedThroughHeight"`
	IndependentFinalityVerified bool   `json:"independentFinalityVerified"`
	CompletenessVerified        bool   `json:"completenessVerified"`
	ApprovalReady               bool   `json:"approvalReady"`
	RealAssetsReady             bool   `json:"realAssetsReady"`
}

// Optional schema extension: nil encodes exactly the earlier journal schema.
// Earlier readers reject cursor-bearing state; no reset/migration is supplied.
type relayCursor struct {
	Origin     Block
	Through    Block
	Checkpoint Block
}

func validDiscoveryBlock(b Block) bool { return b.Height > 0 && b.Hash != [32]byte{} }
func validRelayCursor(c *relayCursor) bool {
	if c == nil {
		return true
	}
	if !validDiscoveryBlock(c.Origin) {
		return false
	}
	if c.Through == (Block{}) {
		return c.Checkpoint == (Block{})
	}
	return validDiscoveryBlock(c.Through) && validDiscoveryBlock(c.Checkpoint) &&
		c.Through.Height >= c.Origin.Height && c.Checkpoint.Height >= c.Through.Height &&
		(c.Through.Height != c.Origin.Height || c.Through.Hash == c.Origin.Hash) &&
		(c.Checkpoint.Height != c.Through.Height || c.Checkpoint.Hash == c.Through.Hash)
}

// EnableDiscovery binds a caller-reviewed first block; it neither discovers a
// default start height nor trusts a provider's current tip as complete history.
// Repeating the same origin is idempotent. The origin cannot be moved/reset.
func (j *RelayJournal) EnableDiscovery(origin Block, at time.Time) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	at = at.Round(0).UTC()
	if !j.usable || j.closed || !relayTime(at) || at.Before(j.state.Clock) || !validDiscoveryBlock(origin) {
		return ErrRelayState
	}
	if j.state.Discovery != nil {
		if j.state.Discovery.Origin != origin {
			return ErrRelayConflict
		}
		return nil
	}
	next := j.copyState()
	next.Clock, next.Discovery = at, &relayCursor{Origin: origin}
	return j.write(next)
}

// ScanNext advances at most eight hash-pinned blocks and atomically saves the
// candidate tasks plus progress. Log hints DO NOT establish receipt success,
// code at the event block, independent finality, completeness or approval.
// ObserveNext must separately verify each receipt. There is no automatic loop.
func (j *RelayJournal) ScanNext(ctx context.Context, source DiscoverySource, native NativeWithdrawalSource, at time.Time) (DiscoveryReport, error) {
	return j.scanNext(ctx, source, native, at, false)
}

// ScanReceiptSets extracts candidates from full, root-matched block material
// instead of getLogs hints. Assurance applies ONLY to this successful batch,
// relative to the supplied header hashes. It neither upgrades retained cursor
// history nor authenticates headers/finality. ObserveNext remains mandatory.
func (j *RelayJournal) ScanReceiptSets(ctx context.Context, source ReceiptSetSource, native NativeWithdrawalSource, at time.Time) (DiscoveryReport, error) {
	if source == nil {
		return DiscoveryReport{SimulationOnly: true}, ErrRelayState
	}
	return j.scanNext(ctx, receiptSetDiscovery{source}, native, at, true)
}

func (j *RelayJournal) scanNext(ctx context.Context, source DiscoverySource, native NativeWithdrawalSource, at time.Time, receiptSets bool) (DiscoveryReport, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	fail := DiscoveryReport{SimulationOnly: true}
	at = at.Round(0).UTC()
	if !j.usable || j.closed || ctx == nil || ctx.Err() != nil || source == nil || j.state.Discovery == nil ||
		!relayTime(at) || at.Before(j.state.Clock) {
		return fail, ErrRelayState
	}
	if j.state.Discovery.Through.Height == math.MaxUint64 {
		return fail, ErrRelayNoWork
	}
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	requests, cursor, blocks, err := j.discover(readCtx, source, native, *j.state.Discovery)
	if err != nil || readCtx.Err() != nil {
		if err == ErrRelayNoWork && readCtx.Err() == nil {
			return fail, err
		}
		return fail, ErrDiscovery
	}
	next := j.copyState()
	added := 0
	for _, request := range requests {
		_, newTask, err := appendRelayRequest(&next, request, at)
		if err != nil {
			return fail, err
		}
		if newTask {
			added++
		}
	}
	next.Clock, next.Discovery = at, &cursor
	if err := j.write(next); err != nil {
		return fail, err
	}
	matched := 0
	if receiptSets {
		matched = blocks
	}
	return DiscoveryReport{SimulationOnly: true, ScannedBlocks: blocks, ReceiptSetsMatched: matched, CandidateEvents: len(requests),
		AddedTasks: added, RetainedTasks: len(requests) - added, ScannedThroughHeight: cursor.Through.Height}, nil
}

func (j *RelayJournal) discover(ctx context.Context, source DiscoverySource, native NativeWithdrawalSource, cursor relayCursor) (requests []relayRequest, result relayCursor, scanned int, err error) {
	// All provider/lookup panics remain redacted; no candidate state is saved.
	defer func() {
		if recover() != nil {
			requests, result, scanned, err = nil, relayCursor{}, 0, ErrDiscovery
		}
	}()
	bad := func() ([]relayRequest, relayCursor, int, error) { return nil, relayCursor{}, 0, ErrDiscovery }
	if !Probe(ctx, j.config, source).ReadOnlyProbePassed {
		return bad()
	}
	checkpoint, err := source.FinalizedBlock(ctx)
	if err != nil || !validDiscoveryBlock(checkpoint) || checkpoint.Height < cursor.Origin.Height || checkpoint.Height < cursor.Checkpoint.Height {
		return bad()
	}
	from := cursor.Origin.Height
	if cursor.Through.Height != 0 {
		from = cursor.Through.Height + 1
	}
	// Recheck retained anchors even when there are no newer blocks.
	anchors := []Block{cursor.Origin, checkpoint}
	if cursor.Through.Height != 0 {
		anchors = append(anchors, cursor.Through, cursor.Checkpoint)
	}
	for _, anchor := range anchors {
		canonical, err := source.Block(ctx, anchor.Height)
		if err != nil || canonical != anchor {
			return bad()
		}
	}
	if from > checkpoint.Height {
		return nil, relayCursor{}, 0, ErrRelayNoWork
	}
	to := checkpoint.Height
	if to-from >= MaxDiscoveryBlocks {
		to = from + MaxDiscoveryBlocks - 1
	}
	custody, _ := address(j.config.private.CustodyContract)
	type eventKey struct {
		Tx    [32]byte
		Index uint32
	}
	seen := map[eventKey]bool{}
	for height := from; ; height++ {
		block, err := source.Block(ctx, height)
		if err != nil || block.Height != height || !validDiscoveryBlock(block) {
			return bad()
		}
		logs, err := source.Logs(ctx, custody, block)
		if err != nil || validateSourceLogs(logs, custody, block) != nil || len(requests)+len(logs) > MaxDiscoveryBatchEvents {
			return bad()
		}
		// Detach the entire bounded block batch before invoking another adapter.
		// A native lookup must not be able to change provider-owned log slices.
		detached := append([]SourceLog(nil), logs...)
		for i := range detached {
			detached[i].Log.Topics = append([][32]byte(nil), logs[i].Log.Topics...)
			detached[i].Log.Data = append([]byte(nil), logs[i].Log.Data...)
		}
		logs = detached
		for _, event := range logs {
			key := eventKey{event.TransactionHash, event.Log.Index}
			if seen[key] {
				return bad()
			}
			seen[key] = true
			request, err := j.discoveredRequest(ctx, native, event, custody)
			if err != nil || j.validRequest(request) != nil {
				return bad()
			}
			requests = append(requests, request)
		}
		anchors = append(anchors, block)
		cursor.Through = block
		scanned++
		if height == to {
			break
		}
	}
	for _, anchor := range anchors {
		canonical, err := source.Block(ctx, anchor.Height)
		if err != nil || canonical != anchor {
			return bad()
		}
	}
	expected, _ := chainID(j.config.private.SourceChainID)
	chain, err := source.ChainID(ctx)
	if err != nil || chain == nil || chain.Cmp(expected) != 0 || ctx.Err() != nil {
		return bad()
	}
	cursor.Checkpoint = checkpoint
	return requests, cursor, scanned, nil
}

func bridgeEventTopics() [3][32]byte {
	return [3][32]byte{
		[32]byte(ethcrypto.Keccak256Hash([]byte("Deposited(uint64,address,bytes20,uint256)"))),
		[32]byte(ethcrypto.Keccak256Hash([]byte("Paid(uint64,bytes32,bytes20,uint256)"))),
		[32]byte(ethcrypto.Keccak256Hash([]byte("Cancelled(uint64,bytes32)"))),
	}
}

func validateSourceLogs(logs []SourceLog, custody [20]byte, block Block) error {
	if len(logs) > MaxDiscoveryLogs || !validDiscoveryBlock(block) || custody == [20]byte{} {
		return ErrSource
	}
	topics := bridgeEventTopics()
	for i, event := range logs {
		log := event.Log
		if event.Block != block || event.TransactionHash == [32]byte{} || log.Emitter != custody || log.Removed || len(log.Topics) != 3 ||
			len(log.Data) > MaxLogDataBytes || log.Topics[0] != topics[0] && log.Topics[0] != topics[1] && log.Topics[0] != topics[2] {
			return ErrSource
		}
		if i > 0 {
			previous := logs[i-1]
			if log.Index <= previous.Log.Index || event.TransactionIndex < previous.TransactionIndex ||
				(event.TransactionIndex == previous.TransactionIndex) != (event.TransactionHash == previous.TransactionHash) {
				return ErrSource
			}
		}
		for _, previous := range logs[:i] {
			if event.TransactionHash == previous.TransactionHash && event.TransactionIndex != previous.TransactionIndex {
				return ErrSource
			}
		}
	}
	return nil
}

func (j *RelayJournal) discoveredRequest(ctx context.Context, native NativeWithdrawalSource, event SourceLog, custody [20]byte) (relayRequest, error) {
	log := event.Log
	topics := bridgeEventTopics()
	if log.Topics[0] == topics[0] {
		d, err := decodeDeposit(log, custody, Receipt{Block: event.Block, TransactionHash: event.TransactionHash})
		if err != nil {
			return relayRequest{}, ErrDiscovery
		}
		return relayRequest{Kind: "deposit", Transaction: event.TransactionHash, LogIndex: log.Index, Recipient: d.Recipient, Amount: d.Amount.String()}, nil
	}
	if native == nil || !bytes.Equal(log.Topics[1][:24], make([]byte, 24)) || log.Topics[2] == [32]byte{} {
		return relayRequest{}, ErrDiscovery
	}
	sequence := binary.BigEndian.Uint64(log.Topics[1][24:])
	if sequence == 0 {
		return relayRequest{}, ErrDiscovery
	}
	w, err := native.Withdrawal(ctx, sequence, log.Topics[2])
	if err != nil || w.Sequence != sequence || w.ID != log.Topics[2] {
		return relayRequest{}, ErrDiscovery
	}
	outcome := godbridge.Cancelled
	if log.Topics[0] == topics[1] {
		outcome = godbridge.Paid
	}
	_, w, err = resolutionRequest(j.config, ResolutionRequest{TransactionHash: event.TransactionHash, LogIndex: log.Index, Withdrawal: w, Outcome: outcome})
	if err != nil {
		return relayRequest{}, ErrDiscovery
	}
	if outcome == godbridge.Paid {
		if len(log.Data) != 64 || !bytes.Equal(log.Data[:20], w.Recipient[:]) || !bytes.Equal(log.Data[20:32], make([]byte, 12)) ||
			new(big.Int).SetBytes(log.Data[32:]).Cmp(w.Amount.BigInt()) != 0 {
			return relayRequest{}, ErrDiscovery
		}
	} else if len(log.Data) != 0 {
		return relayRequest{}, ErrDiscovery
	}
	return relayRequest{Kind: string(outcome), Transaction: event.TransactionHash, LogIndex: log.Index, Withdrawal: &w}, nil
}
