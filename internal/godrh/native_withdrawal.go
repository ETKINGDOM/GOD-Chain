package godrh

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
)

var ErrNativeWithdrawalRead = errors.New("native committed withdrawal consistency read rejected; no proof or financial approval")

// NativeBridgeReadSource has only two bounded reads. A commit is a provider
// claim, not an authenticated consensus/state proof. Implementations must honor
// cancellation and must not sign, mutate, broadcast or reenter a relay journal.
type NativeBridgeReadSource interface {
	Bridge(context.Context, int64) (godnode.BridgeView, error)
	BridgeWithdrawal(context.Context, uint64, int64) (godnode.BridgeWithdrawalView, error)
}

// NativeWithdrawalReader is an explicit simulation-only discovery adapter.
// Its source is separately supplied; it never chooses a default node/endpoint.
type NativeWithdrawalReader struct {
	config Config
	source NativeBridgeReadSource
}

func (*NativeWithdrawalReader) String() string {
	return "native simulation withdrawal reader (redacted)"
}
func (r *NativeWithdrawalReader) GoString() string { return r.String() }
func (*NativeWithdrawalReader) MarshalJSON() ([]byte, error) {
	return []byte(`{"readOnly":true,"simulationOnly":true,"redacted":true}`), nil
}

func NewNativeWithdrawalReader(c Config, source NativeBridgeReadSource) (*NativeWithdrawalReader, error) {
	if _, err := relayBinding(c); err != nil || source == nil {
		return nil, ErrNativeWithdrawalRead
	}
	return &NativeWithdrawalReader{c, source}, nil
}

func (r *NativeWithdrawalReader) Withdrawal(ctx context.Context, sequence uint64, id [32]byte) (godbridge.Withdrawal, error) {
	if r == nil {
		return godbridge.Withdrawal{}, ErrNativeWithdrawalRead
	}
	v, err := ReadNativeWithdrawal(ctx, r.config, r.source, sequence, id)
	if err != nil {
		return godbridge.Withdrawal{}, ErrNativeWithdrawalRead
	}
	w, _, err := v.Snapshot()
	return w, err
}

// Owned ephemeral data only. No provider, journal, store or signing pointer.
// Explicit Snapshot access returns private detached values, never a certificate.
type NativeWithdrawalObservation struct {
	bridge     godnode.BridgeView
	view       godnode.BridgeWithdrawalView
	withdrawal godbridge.Withdrawal
	ready      bool
}

type NativeWithdrawalReport struct {
	SimulationOnly                    bool `json:"simulationOnly"`
	CommittedViewMatched              bool `json:"committedViewMatched"`
	SameCommitRechecked               bool `json:"sameCommitRechecked"`
	BindingMatched                    bool `json:"bindingMatched"`
	WithdrawalIdentityMatched         bool `json:"withdrawalIdentityMatched"`
	NativeStateProven                 bool `json:"nativeStateProven"`
	IndependentSourceFinalityVerified bool `json:"independentSourceFinalityVerified"`
	ApprovalReady                     bool `json:"approvalReady"`
	SigningEnabled                    bool `json:"signingEnabled"`
	BroadcastEnabled                  bool `json:"broadcastEnabled"`
	RealAssetsReady                   bool `json:"realAssetsReady"`
}

func (NativeWithdrawalObservation) String() string {
	return "native simulation withdrawal observation (redacted)"
}
func (o NativeWithdrawalObservation) GoString() string             { return o.String() }
func (o NativeWithdrawalObservation) MarshalJSON() ([]byte, error) { return json.Marshal(o.Report()) }
func (o NativeWithdrawalObservation) Report() NativeWithdrawalReport {
	if !o.ready {
		return NativeWithdrawalReport{}
	}
	return NativeWithdrawalReport{SimulationOnly: true, CommittedViewMatched: true, SameCommitRechecked: true,
		BindingMatched: true, WithdrawalIdentityMatched: true}
}
func (o NativeWithdrawalObservation) Snapshot() (godbridge.Withdrawal, godnode.CommittedView, error) {
	if !o.ready {
		return godbridge.Withdrawal{}, godnode.CommittedView{}, ErrNativeWithdrawalRead
	}
	w := o.withdrawal
	w.Amount = sdkmath.NewIntFromBigInt(w.Amount.BigInt())
	return w, o.bridge.Commit, nil
}

// Four reads, one commit, no scan/retry/wall-clock eligibility calculation.
// Tip advance, absence, terminal native state or any inconsistency fails closed.
// Agreement is relative to a supplied operator source, not native finality.
func ReadNativeWithdrawal(ctx context.Context, c Config, source NativeBridgeReadSource, sequence uint64, id [32]byte) (out NativeWithdrawalObservation, err error) {
	defer func() {
		if recover() != nil {
			out, err = NativeWithdrawalObservation{}, ErrNativeWithdrawalRead
		}
	}()
	if ctx == nil || ctx.Err() != nil || source == nil || sequence == 0 || id == [32]byte{} {
		return NativeWithdrawalObservation{}, ErrNativeWithdrawalRead
	}
	if _, err := relayBinding(c); err != nil {
		return NativeWithdrawalObservation{}, ErrNativeWithdrawalRead
	}
	readCtx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	a, err := readNativeWithdrawalAt(readCtx, c, source, sequence, id, 0)
	if err != nil {
		return NativeWithdrawalObservation{}, ErrNativeWithdrawalRead
	}
	b, err := readNativeWithdrawalAt(readCtx, c, source, sequence, id, a.bridge.Commit.Height)
	if err != nil || !sameNativeWithdrawal(a, b) || readCtx.Err() != nil {
		return NativeWithdrawalObservation{}, ErrNativeWithdrawalRead
	}
	a.ready = true
	return a, nil
}

func nativeAmount(text string, maximum sdkmath.Int) (sdkmath.Int, bool) {
	if len(text) == 0 || len(text) > 78 {
		return sdkmath.Int{}, false
	}
	n, ok := sdkmath.NewIntFromString(text)
	return n, ok && !n.IsNegative() && n.String() == text && (maximum.IsNil() || n.LTE(maximum))
}

func validNativeCommit(c godnode.CommittedView, height int64) bool {
	_, err := SourceMaterialPin(c.AppHash)
	return err == nil && c.Height > 0 && (height == 0 || c.Height == height) &&
		relayTime(c.Time) && c.Time.Location() == time.UTC && c.Synthetic && !c.RealAssets
}

func nativeBridgeShape(c Config, b godnode.BridgeView, height int64) bool {
	binding, err := c.Binding()
	if err != nil || !validNativeCommit(b.Commit, height) || b.SourceChain != binding.SourceChain ||
		b.NativeChain != binding.NativeChain || b.AssetID != hex.EncodeToString(binding.AssetID[:]) ||
		!b.NativeAccountingMatched || b.IndependentSourceFinalityVerified || b.RealAssetsReady ||
		b.WithdrawalDelaySeconds != uint64(godbridge.WithdrawalDelay/time.Second) || b.HeadSequence == 0 ||
		b.NextSequence < b.HeadSequence || b.NextSequence-b.HeadSequence > godbridge.MaxOpenWithdrawals ||
		b.OpenWithdrawals > godbridge.MaxOpenWithdrawals || uint64(b.OpenWithdrawals) > b.NextSequence-b.HeadSequence {
		return false
	}
	r, rOK := nativeAmount(b.ReleasedGod, godbridge.ExposureLimit())
	p, pOK := nativeAmount(b.PendingGod, godrewards.FixedGodSupply())
	v, vOK := nativeAmount(b.ReserveGod, godrewards.FixedGodSupply())
	d, dOK := nativeAmount(b.TotalDeposited, sdkmath.Int{})
	paid, paidOK := nativeAmount(b.TotalPaid, sdkmath.Int{})
	_, rollingOK := nativeAmount(b.RollingOutflow, godbridge.OutflowLimit())
	return rOK && pOK && vOK && dOK && paidOK && rollingOK && r.Add(v).Equal(godrewards.FixedGodSupply()) &&
		p.LTE(r) && paid.LTE(d) && d.Sub(paid).Equal(r) &&
		(b.OpenWithdrawals != 0 && p.IsPositive() || b.OpenWithdrawals == 0 && p.IsZero() && b.HeadSequence == b.NextSequence)
}

func readNativeWithdrawalAt(ctx context.Context, c Config, source NativeBridgeReadSource, sequence uint64, id [32]byte, height int64) (NativeWithdrawalObservation, error) {
	bad := func() (NativeWithdrawalObservation, error) {
		return NativeWithdrawalObservation{}, ErrNativeWithdrawalRead
	}
	if ctx.Err() != nil {
		return bad()
	}
	b, err := source.Bridge(ctx, height)
	if err != nil || ctx.Err() != nil || !nativeBridgeShape(c, b, height) || sequence >= b.NextSequence {
		return bad()
	}
	v, err := source.BridgeWithdrawal(ctx, sequence, b.Commit.Height)
	if err != nil || ctx.Err() != nil || v.Commit != b.Commit || !v.Exists || v.Sequence != sequence ||
		v.ID != hex.EncodeToString(id[:]) || !v.PendingNativeGod || v.AttestedSourcePayment || v.AttestedSourceCancellation ||
		v.SourceResolution != nil || v.ResolvedAt != "" || v.IndependentSourceFinalityVerified || v.RealAssetsReady ||
		(v.Status != string(godbridge.Queued) && v.Status != string(godbridge.Authorized)) ||
		(v.Status == string(godbridge.Authorized) && sequence != b.HeadSequence) ||
		(v.Status == string(godbridge.Queued) && v.AuthorizedAt != "") ||
		v.HeadOfQueue != (sequence == b.HeadSequence) || sequence < b.HeadSequence {
		return bad()
	}
	sender, err := godaddress.FromNative(v.NativeSender)
	if err != nil {
		return bad()
	}
	senderText, err := godaddress.ToNative(sender)
	if err != nil || senderText != v.NativeSender {
		return bad()
	}
	recipient, err := godaddress.FromEVM(v.SourceRecipient)
	if err != nil {
		return bad()
	}
	recipientText, err := godaddress.ToEVM(recipient)
	if err != nil || recipientText != v.SourceRecipient {
		return bad()
	}
	amount, ok := nativeAmount(v.Amount, godbridge.TransferLimit())
	if !ok || !amount.IsPositive() {
		return bad()
	}
	queued, err := time.Parse(time.RFC3339Nano, v.QueuedAt)
	if err != nil || !relayTime(queued) || queued.UTC().Format(time.RFC3339Nano) != v.QueuedAt || queued.After(b.Commit.Time) {
		return bad()
	}
	end := queued.Add(godbridge.WithdrawalDelay).UTC()
	if end.Year() > 9999 || v.DelayEndsAt != end.Format(time.RFC3339Nano) {
		return bad()
	}
	var remaining uint64
	if b.Commit.Time.Before(end) {
		delay := end.Sub(b.Commit.Time)
		remaining = uint64(delay / time.Second)
		if delay%time.Second != 0 {
			remaining++
		}
	}
	if v.DelayRemainingSeconds != remaining || v.DelayElapsed != (remaining == 0) {
		return bad()
	}
	w := godbridge.Withdrawal{ID: id, Sequence: sequence, Sender: [20]byte(sender), Recipient: [20]byte(recipient),
		Amount: amount, QueuedAt: queued.UTC(), Status: godbridge.Status(v.Status)}
	if v.AuthorizedAt != "" {
		authorized, err := time.Parse(time.RFC3339Nano, v.AuthorizedAt)
		if err != nil || authorized.UTC().Format(time.RFC3339Nano) != v.AuthorizedAt || authorized.After(b.Commit.Time) {
			return bad()
		}
		w.AuthorizedAt = authorized.UTC()
	}
	// Recompute the full bound identity; also check status and authorization time.
	_, w, err = resolutionRequest(c, ResolutionRequest{TransactionHash: id, Withdrawal: w, Outcome: godbridge.Cancelled})
	if err != nil {
		return bad()
	}
	pending, _ := sdkmath.NewIntFromString(b.PendingGod)
	if b.OpenWithdrawals == 0 || amount.GT(pending) {
		return bad()
	}
	return NativeWithdrawalObservation{bridge: b, view: v, withdrawal: w}, nil
}

func sameNativeWithdrawal(a, b NativeWithdrawalObservation) bool {
	return a.bridge == b.bridge && a.view == b.view && sameWithdrawal(a.withdrawal, b.withdrawal)
}
func sameWithdrawal(a, b godbridge.Withdrawal) bool {
	return a.ID == b.ID && a.Sequence == b.Sequence && a.Sender == b.Sender && a.Recipient == b.Recipient &&
		!a.Amount.IsNil() && !b.Amount.IsNil() && a.Amount.Equal(b.Amount) && a.QueuedAt.Equal(b.QueuedAt) &&
		a.Status == b.Status && a.AuthorizedAt.Equal(b.AuthorizedAt) && a.ResolvedAt.Equal(b.ResolvedAt) && a.ResolutionEvidence == b.ResolutionEvidence
}

// Keep decimal sequence formatting here explicit for the HTTP adapter.
func nativeSequence(sequence uint64) string { return strconv.FormatUint(sequence, 10) }

var _ NativeWithdrawalSource = (*NativeWithdrawalReader)(nil)
