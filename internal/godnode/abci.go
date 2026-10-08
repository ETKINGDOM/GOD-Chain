package godnode

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// application is only connected by the in-process GodCometBFT client. There
// is no public ABCI socket where a caller can forge DecidedLastCommit votes.
// ABCI vote flags are not themselves cryptographic proofs: GodCometBFT verifies
// actual signed commits before delivering them through this trusted boundary.
type application struct{ app *App }

var _ abci.Application = application{}

func (x application) Info(_ context.Context, req *abci.RequestInfo) (*abci.ResponseInfo, error) {
	a := x.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable() {
		return nil, ErrLifecycle
	}
	return a.base.Info(req)
}
func (x application) Query(_ context.Context, req *abci.RequestQuery) (*abci.ResponseQuery, error) {
	a := x.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable() {
		return nil, ErrLifecycle
	}
	if req == nil || req.Prove || req.Height < 0 || (req.Height != 0 && req.Height != a.base.LastBlockHeight()) {
		return &abci.ResponseQuery{Code: 1}, nil
	}
	var state any
	var height int64
	var err error
	switch req.Path {
	case "/god/status":
		if len(req.Data) != 0 {
			return &abci.ResponseQuery{Code: 1}, nil
		}
		var result Snapshot
		result, err = a.snapshot()
		state, height = result, result.Height
	case "/god/network":
		if len(req.Data) != 0 {
			return &abci.ResponseQuery{Code: 1}, nil
		}
		var result NetworkView
		result, err = a.queryNetwork(req.Height)
		state, height = result, result.Commit.Height
	case "/god/account":
		if len(req.Data) != godaddress.NativeLength {
			return &abci.ResponseQuery{Code: 1}, nil
		}
		var result AccountView
		// The payload is the address text, not a JSON envelope or store key.
		// Address parsers bound its length before decoding or store access.
		result, err = a.queryAccount(string(req.Data), req.Height)
		state, height = result, result.Commit.Height
	case "/god/validator":
		const length = len(godaddress.ValidatorOperatorPrefix) + 1 + godaddress.AccountBytes*8/5 + 6
		if len(req.Data) != length {
			return &abci.ResponseQuery{Code: 1}, nil
		}
		var result ValidatorView
		result, err = a.queryValidator(string(req.Data), req.Height)
		state, height = result, result.Commit.Height
	default:
		return &abci.ResponseQuery{Code: 1}, nil
	}
	if err != nil {
		return &abci.ResponseQuery{Code: 1}, nil
	}
	raw, err := json.Marshal(state)
	if err != nil {
		return nil, err
	}
	return &abci.ResponseQuery{Value: raw, Height: height}, nil
}
func (x application) InitChain(_ context.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	a := x.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable() || a.initialized || a.base.LastBlockHeight() != 0 {
		return nil, ErrLifecycle
	}
	if err := validateGenesisRequest(req, a.config); err != nil {
		return nil, err
	}
	a.failed = true
	response, err := a.base.InitChain(req)
	if err != nil {
		return nil, err
	}
	a.failed = false
	a.initialized = true
	a.clock = req.Time.UTC()
	return response, nil
}
func (x application) CheckTx(_ context.Context, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
	a := x.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable() || !a.initialized {
		return nil, ErrLifecycle
	}
	if req == nil || len(req.Tx) == 0 || len(req.Tx) > a.config.Policy.MaxTxBytes || (req.Type != abci.CheckTxType_New && req.Type != abci.CheckTxType_Recheck) {
		return &abci.ResponseCheckTx{Code: 1}, nil
	}
	// FinalizeBlock writes the new working stores before Commit refreshes
	// BaseApp's CheckTx context. Do not authenticate against that mixed view.
	// Never wait here: the caller may hold the mempool lock Commit needs.
	if a.finalized {
		return &abci.ResponseCheckTx{Code: 1, Codespace: "godnode_commit_pending"}, nil
	}
	return a.base.CheckTx(req)
}

func commitPendingCheck(result *abci.ResponseCheckTx) bool {
	return result != nil && result.Code == 1 && result.Codespace == "godnode_commit_pending"
}
func (a *App) nextBlock(height int64, t time.Time) bool {
	return a.initialized && a.base.LastBlockHeight() < math.MaxInt64-2 && height == a.base.LastBlockHeight()+1 && validTime(t) && !t.Before(a.clock)
}
func (a *App) boundedBlock(txs [][]byte) bool {
	if len(txs) > a.config.MaxBlockTxs {
		return false
	}
	bytes, gas := int64(0), uint64(0)
	for _, wire := range txs {
		if len(wire) == 0 || len(wire) > a.config.Policy.MaxTxBytes || int64(len(wire)) > a.config.MaxBlockBytes-bytes {
			return false
		}
		bytes += int64(len(wire))
		tx, err := a.encoding.Decoder(a.config.Policy.MaxTxBytes)(wire)
		if err != nil {
			return false
		}
		fee, ok := tx.(sdk.FeeTx)
		if !ok || fee.GetGas() == 0 || fee.GetGas() > a.config.Policy.MaxGas || fee.GetGas() > uint64(a.config.MaxBlockGas)-gas {
			return false
		}
		gas += fee.GetGas()
	}
	return true
}
func (x application) PrepareProposal(_ context.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	a := x.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable() || a.finalized {
		return nil, ErrLifecycle
	}
	if req == nil || !a.nextBlock(req.Height, req.Time) || req.MaxTxBytes <= 0 {
		return nil, ErrBlock
	}
	copyReq := *req
	copyReq.Txs = nil
	bytes, gas := int64(0), uint64(0)
	for _, wire := range req.Txs {
		if len(copyReq.Txs) >= a.config.MaxBlockTxs {
			break
		}
		if len(wire) == 0 || len(wire) > a.config.Policy.MaxTxBytes || int64(len(wire)) > req.MaxTxBytes-bytes || int64(len(wire)) > a.config.MaxBlockBytes-bytes {
			continue
		}
		tx, err := a.encoding.Decoder(a.config.Policy.MaxTxBytes)(wire)
		if err != nil {
			continue
		}
		fee, ok := tx.(sdk.FeeTx)
		if !ok || fee.GetGas() == 0 || fee.GetGas() > a.config.Policy.MaxGas || fee.GetGas() > uint64(a.config.MaxBlockGas)-gas {
			continue
		}
		copyReq.Txs = append(copyReq.Txs, wire)
		bytes += int64(len(wire))
		gas += fee.GetGas()
	}
	return a.base.PrepareProposal(&copyReq)
}
func (x application) ProcessProposal(_ context.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
	a := x.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable() || a.finalized {
		return nil, ErrLifecycle
	}
	if req == nil || !a.nextBlock(req.Height, req.Time) || !a.boundedBlock(req.Txs) || !validMisbehavior(req.Height, req.Time, req.Misbehavior) {
		return &abci.ResponseProcessProposal{Status: abci.ResponseProcessProposal_REJECT}, nil
	}
	return a.base.ProcessProposal(req)
}
func (x application) FinalizeBlock(_ context.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	a := x.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable() || a.finalized {
		return nil, ErrLifecycle
	}
	if req == nil || !a.nextBlock(req.Height, req.Time) || !a.boundedBlock(req.Txs) || !validMisbehavior(req.Height, req.Time, req.Misbehavior) {
		return nil, ErrBlock
	}
	a.failed = true
	response, err := a.base.FinalizeBlock(req)
	if err != nil {
		return nil, err
	}
	a.failed = false
	a.finalized = true
	a.pendingTime = req.Time.UTC()
	return response, nil
}
func (x application) Commit(context.Context, *abci.RequestCommit) (*abci.ResponseCommit, error) {
	a := x.app
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable() || !a.initialized || !a.finalized {
		return nil, ErrLifecycle
	}
	a.failed = true
	response, err := a.base.Commit()
	if err != nil {
		return nil, err
	}
	a.failed = false
	a.finalized = false
	a.clock = a.pendingTime
	return response, nil
}

// Vote extensions and snapshot restore are unsupported, not implicitly trusted.
func (application) ExtendVote(context.Context, *abci.RequestExtendVote) (*abci.ResponseExtendVote, error) {
	return &abci.ResponseExtendVote{}, nil
}
func (application) VerifyVoteExtension(context.Context, *abci.RequestVerifyVoteExtension) (*abci.ResponseVerifyVoteExtension, error) {
	return &abci.ResponseVerifyVoteExtension{Status: abci.ResponseVerifyVoteExtension_REJECT}, nil
}
func (application) ListSnapshots(context.Context, *abci.RequestListSnapshots) (*abci.ResponseListSnapshots, error) {
	return &abci.ResponseListSnapshots{}, nil
}
func (application) OfferSnapshot(context.Context, *abci.RequestOfferSnapshot) (*abci.ResponseOfferSnapshot, error) {
	return &abci.ResponseOfferSnapshot{Result: abci.ResponseOfferSnapshot_REJECT}, nil
}
func (application) LoadSnapshotChunk(context.Context, *abci.RequestLoadSnapshotChunk) (*abci.ResponseLoadSnapshotChunk, error) {
	return nil, ErrLifecycle
}
func (application) ApplySnapshotChunk(context.Context, *abci.RequestApplySnapshotChunk) (*abci.ResponseApplySnapshotChunk, error) {
	return &abci.ResponseApplySnapshotChunk{Result: abci.ResponseApplySnapshotChunk_ABORT}, nil
}

type Snapshot struct {
	Height    int64
	Rewards   godrewards.Snapshot
	Synthetic bool
	Bridge    *godbridge.Snapshot `json:"bridge,omitempty"`
}

func (a *App) snapshot() (Snapshot, error) {
	if !a.usable() || a.base.LastBlockHeight() <= 0 {
		return Snapshot{}, ErrLifecycle
	}
	ctx, err := a.base.CreateQueryContext(a.base.LastBlockHeight(), false)
	if err != nil {
		return Snapshot{}, err
	}
	rewards, err := a.rewards.Snapshot(ctx.WithBlockTime(a.clock))
	if err != nil {
		return Snapshot{}, err
	}
	bridge, err := a.bridgeSnapshot(ctx.WithBlockTime(a.clock))
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Height: a.base.LastBlockHeight(), Rewards: rewards, Synthetic: true, Bridge: bridge}, nil
}
func (a *App) Snapshot() (Snapshot, error) { a.mu.Lock(); defer a.mu.Unlock(); return a.snapshot() }
