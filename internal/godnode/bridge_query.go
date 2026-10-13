package godnode

import (
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
)

var ErrBridgeQuery = errors.New("committed bridge query unavailable or unconfigured")
var ErrWithdrawalQuery = errors.New("bridge withdrawal sequence rejected")

// These detached committed views report the native ledger's attestations,
// not independently verified RH custody/finality or authority to move assets.
type BridgeView struct {
	Commit                            CommittedView `json:"commit"`
	SourceChain                       string        `json:"sourceChain"`
	NativeChain                       string        `json:"nativeChain"`
	AssetID                           string        `json:"assetId"`
	ReleasedGod                       string        `json:"nativeGodOutsideReserveSmallestUnits"`
	PendingGod                        string        `json:"pendingWithdrawalsSmallestUnits"`
	ReserveGod                        string        `json:"restrictedReserveSmallestUnits"`
	TotalDeposited                    string        `json:"attestedDepositsSmallestUnits"`
	TotalPaid                         string        `json:"attestedPaymentsSmallestUnits"`
	RollingOutflow                    string        `json:"reservedRollingOutflowSmallestUnits"`
	OpenWithdrawals                   uint32        `json:"openWithdrawals"`
	HeadSequence                      uint64        `json:"headSequence,string"`
	NextSequence                      uint64        `json:"nextSequence,string"`
	WithdrawalDelaySeconds            uint64        `json:"withdrawalDelaySeconds,string"`
	IntakePaused                      bool          `json:"intakePaused"`
	OutflowPaused                     bool          `json:"outflowPaused"`
	NativeAccountingMatched           bool          `json:"nativeAccountingMatched"`
	IndependentSourceFinalityVerified bool          `json:"independentSourceFinalityVerified"`
	RealAssetsReady                   bool          `json:"realAssetsReady"`
}

type BridgeResolutionView struct {
	Height          uint64 `json:"height,string"`
	BlockHash       string `json:"blockHash"`
	TransactionHash string `json:"transactionHash"`
	LogIndex        uint32 `json:"logIndex"`
}

type BridgeWithdrawalView struct {
	Commit                            CommittedView         `json:"commit"`
	Sequence                          uint64                `json:"sequence,string"`
	Exists                            bool                  `json:"exists"`
	ID                                string                `json:"id,omitempty"`
	NativeSender                      string                `json:"nativeSender,omitempty"`
	SourceRecipient                   string                `json:"sourceRecipient,omitempty"`
	Amount                            string                `json:"amountSmallestUnits,omitempty"`
	Status                            string                `json:"status"`
	QueuedAt                          string                `json:"queuedAt,omitempty"`
	DelayEndsAt                       string                `json:"delayEndsAt,omitempty"`
	DelayRemainingSeconds             uint64                `json:"delayRemainingSeconds,string"`
	DelayElapsed                      bool                  `json:"delayElapsed"`
	HeadOfQueue                       bool                  `json:"headOfQueue"`
	PendingNativeGod                  bool                  `json:"pendingNativeGod"`
	AuthorizedAt                      string                `json:"authorizedAt,omitempty"`
	ResolvedAt                        string                `json:"resolvedAt,omitempty"`
	AttestedSourcePayment             bool                  `json:"attestedSourcePayment"`
	AttestedSourceCancellation        bool                  `json:"attestedSourceCancellation"`
	SourceResolution                  *BridgeResolutionView `json:"sourceResolution,omitempty"`
	IndependentSourceFinalityVerified bool                  `json:"independentSourceFinalityVerified"`
	RealAssetsReady                   bool                  `json:"realAssetsReady"`
}

func bridgeSequence(text string) (uint64, error) {
	if len(text) == 0 || len(text) > 20 {
		return 0, ErrWithdrawalQuery
	}
	n, err := strconv.ParseUint(text, 10, 64)
	if err != nil || n == 0 || strconv.FormatUint(n, 10) != text {
		return 0, ErrWithdrawalQuery
	}
	return n, nil
}

// Only latest committed versions, bounded state/window reads and one optional
// numbered withdrawal record. No collection scan, signature or submission.
func (a *App) QueryBridge(height int64) (BridgeView, error) {
	if a == nil {
		return BridgeView{}, ErrLifecycle
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.queryBridge(height)
}
func (a *App) queryBridge(height int64) (view BridgeView, err error) {
	defer func() {
		if recover() != nil {
			view, err = BridgeView{}, ErrBridgeQuery
		}
	}()
	ctx, commit, e := a.committedContext(height)
	if e != nil {
		return BridgeView{}, e
	}
	s, e := a.bridgeSnapshot(ctx)
	if e != nil || s == nil || a.config.BridgeGenesis == nil {
		return BridgeView{}, ErrBridgeQuery
	}
	c := a.config.BridgeGenesis
	return BridgeView{Commit: commit, SourceChain: c.SourceChain, NativeChain: c.NativeChain, AssetID: hex.EncodeToString(c.AssetID[:]),
		ReleasedGod: s.ReleasedGod.String(), PendingGod: s.PendingGod.String(), ReserveGod: s.ReserveGod.String(),
		TotalDeposited: s.TotalDeposited.String(), TotalPaid: s.TotalPaid.String(), RollingOutflow: s.RollingOutflow.String(),
		OpenWithdrawals: s.OpenWithdrawals, HeadSequence: s.HeadSequence, NextSequence: s.NextSequence,
		WithdrawalDelaySeconds: uint64(godbridge.WithdrawalDelay / time.Second), IntakePaused: s.IntakePaused, OutflowPaused: s.OutflowPaused, NativeAccountingMatched: true}, nil
}

func (a *App) QueryBridgeWithdrawal(sequence string, height int64) (BridgeWithdrawalView, error) {
	if a == nil {
		return BridgeWithdrawalView{}, ErrLifecycle
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.queryBridgeWithdrawal(sequence, height)
}
func (a *App) queryBridgeWithdrawal(sequence string, height int64) (view BridgeWithdrawalView, err error) {
	defer func() {
		if recover() != nil {
			view, err = BridgeWithdrawalView{}, ErrBridgeQuery
		}
	}()
	n, e := bridgeSequence(sequence)
	if e != nil {
		return BridgeWithdrawalView{}, e
	}
	ctx, commit, e := a.committedContext(height)
	if e != nil {
		return BridgeWithdrawalView{}, e
	}
	s, e := a.bridgeSnapshot(ctx)
	if e != nil || s == nil {
		return BridgeWithdrawalView{}, ErrBridgeQuery
	}
	if n >= s.NextSequence {
		return BridgeWithdrawalView{Commit: commit, Sequence: n, Status: "not-found"}, nil
	}
	w, e := a.bridge.Withdrawal(ctx, n)
	if e != nil || w.Sequence != n || w.QueuedAt.After(commit.Time) || w.AuthorizedAt.After(commit.Time) || w.ResolvedAt.After(commit.Time) {
		return BridgeWithdrawalView{}, ErrBridgeQuery
	}
	sender, e := godaddress.ToNative(w.Sender[:])
	if e != nil {
		return BridgeWithdrawalView{}, ErrBridgeQuery
	}
	recipient, e := godaddress.ToEVM(w.Recipient[:])
	if e != nil {
		return BridgeWithdrawalView{}, ErrBridgeQuery
	}
	end := w.QueuedAt.Add(godbridge.WithdrawalDelay).UTC()
	if end.Year() > 9999 {
		return BridgeWithdrawalView{}, ErrBridgeQuery
	}
	var remaining uint64
	if commit.Time.Before(end) {
		d := end.Sub(commit.Time)
		remaining = uint64(d / time.Second)
		if d%time.Second != 0 {
			remaining++
		}
	}
	v := BridgeWithdrawalView{Commit: commit, Sequence: n, Exists: true, ID: hex.EncodeToString(w.ID[:]), NativeSender: sender, SourceRecipient: recipient,
		Amount: w.Amount.String(), Status: string(w.Status), QueuedAt: w.QueuedAt.UTC().Format(time.RFC3339Nano), DelayEndsAt: end.Format(time.RFC3339Nano),
		DelayRemainingSeconds: remaining, DelayElapsed: remaining == 0, HeadOfQueue: n == s.HeadSequence && (w.Status == godbridge.Queued || w.Status == godbridge.Authorized),
		PendingNativeGod:      w.Status == godbridge.Queued || w.Status == godbridge.Authorized,
		AttestedSourcePayment: w.Status == godbridge.Paid, AttestedSourceCancellation: w.Status == godbridge.Cancelled}
	if !w.AuthorizedAt.IsZero() {
		v.AuthorizedAt = w.AuthorizedAt.UTC().Format(time.RFC3339Nano)
	}
	if !w.ResolvedAt.IsZero() {
		v.ResolvedAt = w.ResolvedAt.UTC().Format(time.RFC3339Nano)
		r := w.ResolutionEvidence
		v.SourceResolution = &BridgeResolutionView{Height: r.Height, BlockHash: hex.EncodeToString(r.BlockHash[:]), TransactionHash: hex.EncodeToString(r.TransactionHash[:]), LogIndex: r.LogIndex}
	}
	return v, nil
}
