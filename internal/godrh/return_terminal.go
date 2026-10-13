package godrh

import (
	"context"
	"encoding/hex"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
)

// Private operator-view data, never an authenticated native/source proof.
type returnTerminalSnapshot struct {
	Bridge     godnode.BridgeView
	Withdrawal godnode.BridgeWithdrawalView
}

// A replay of retained private views for local validation, never live reads.
type storedReturnNativeSource struct {
	bridge     godnode.BridgeView
	withdrawal godnode.BridgeWithdrawalView
}

func (s storedReturnNativeSource) Bridge(context.Context, int64) (godnode.BridgeView, error) {
	return s.bridge, nil
}
func (s storedReturnNativeSource) BridgeWithdrawal(context.Context, uint64, int64) (godnode.BridgeWithdrawalView, error) {
	return s.withdrawal, nil
}

func returnCommitCovers(current, previous godnode.CommittedView) bool {
	return validNativeCommit(current, 0) && validNativeCommit(previous, 0) && current.Height >= previous.Height &&
		!current.Time.Before(previous.Time) && (current.Height != previous.Height || current == previous)
}

// Terminal native attestation must match the ORIGINAL immutable parties,
// authorization time, source outcome and full evidence tuple. Waiting progress
// still uses committed block time. This performs no financial transition.
func readReturnTerminalAt(ctx context.Context, c Config, native NativeBridgeReadSource, w godbridge.Withdrawal,
	outcome godbridge.Status, evidence godbridge.Evidence, handoff godnode.CommittedView, height int64) (returnTerminalSnapshot, error) {
	bad := func() (returnTerminalSnapshot, error) { return returnTerminalSnapshot{}, ErrReturnOutboxState }
	if ctx == nil || ctx.Err() != nil || native == nil {
		return bad()
	}
	b, err := native.Bridge(ctx, height)
	if err != nil || ctx.Err() != nil || !nativeBridgeShape(c, b, height) || !returnCommitCovers(b.Commit, handoff) || b.Commit.Height <= handoff.Height || w.Sequence >= b.NextSequence {
		return bad()
	}
	v, err := native.BridgeWithdrawal(ctx, w.Sequence, b.Commit.Height)
	// Detach the only provider-owned pointer BEFORE another callback.
	if v.SourceResolution != nil {
		copy := *v.SourceResolution
		v.SourceResolution = &copy
	}
	if err != nil || ctx.Err() != nil || !returnTerminalShape(c, b, v, w, outcome, evidence, handoff) {
		return bad()
	}
	return returnTerminalSnapshot{b, v}, nil
}

func returnTerminalShape(c Config, b godnode.BridgeView, v godnode.BridgeWithdrawalView, w godbridge.Withdrawal,
	outcome godbridge.Status, evidence godbridge.Evidence, handoff godnode.CommittedView) bool {
	if !nativeBridgeShape(c, b, b.Commit.Height) || !returnCommitCovers(b.Commit, handoff) || b.Commit.Height <= handoff.Height ||
		v.Commit != b.Commit || !v.Exists || v.Sequence != w.Sequence || v.Sequence >= b.NextSequence ||
		v.ID != hex.EncodeToString(w.ID[:]) || v.Status != string(outcome) || v.PendingNativeGod || v.HeadOfQueue ||
		v.AttestedSourcePayment != (outcome == godbridge.Paid) || v.AttestedSourceCancellation != (outcome == godbridge.Cancelled) ||
		v.SourceResolution == nil || v.RealAssetsReady || v.IndependentSourceFinalityVerified ||
		(outcome != godbridge.Paid && outcome != godbridge.Cancelled) {
		return false
	}
	if outcome == godbridge.Paid && v.Sequence >= b.HeadSequence {
		return false
	}
	sender, err := godaddress.ToNative(w.Sender[:])
	if err != nil || v.NativeSender != sender {
		return false
	}
	recipient, err := godaddress.ToEVM(w.Recipient[:])
	if err != nil || v.SourceRecipient != recipient || w.Amount.IsNil() || v.Amount != w.Amount.String() ||
		v.QueuedAt != w.QueuedAt.UTC().Format(time.RFC3339Nano) {
		return false
	}
	authorized := ""
	if !w.AuthorizedAt.IsZero() {
		authorized = w.AuthorizedAt.UTC().Format(time.RFC3339Nano)
	}
	if v.AuthorizedAt != authorized || outcome == godbridge.Paid && authorized == "" {
		return false
	}
	resolved, err := time.Parse(time.RFC3339Nano, v.ResolvedAt)
	if err != nil || !relayTime(resolved) || resolved.UTC().Format(time.RFC3339Nano) != v.ResolvedAt ||
		resolved.Before(w.QueuedAt) || !w.AuthorizedAt.IsZero() && resolved.Before(w.AuthorizedAt) ||
		resolved.Before(handoff.Time) || resolved.After(b.Commit.Time) {
		return false
	}
	end := w.QueuedAt.Add(godbridge.WithdrawalDelay).UTC()
	if end.Year() > 9999 || v.DelayEndsAt != end.Format(time.RFC3339Nano) {
		return false
	}
	remaining := uint64(0)
	if b.Commit.Time.Before(end) {
		d := end.Sub(b.Commit.Time)
		remaining = uint64(d / time.Second)
		if d%time.Second != 0 {
			remaining++
		}
	}
	r := v.SourceResolution
	return v.DelayRemainingSeconds == remaining && v.DelayElapsed == (remaining == 0) &&
		r.Height == evidence.Height && r.Height > 0 && r.BlockHash == hex.EncodeToString(evidence.BlockHash[:]) &&
		r.TransactionHash == hex.EncodeToString(evidence.TransactionHash[:]) && r.LogIndex == evidence.LogIndex
}

func sameReturnTerminal(a, b returnTerminalSnapshot) bool {
	if a.Bridge != b.Bridge || a.Withdrawal.SourceResolution == nil || b.Withdrawal.SourceResolution == nil ||
		*a.Withdrawal.SourceResolution != *b.Withdrawal.SourceResolution {
		return false
	}
	x, y := a.Withdrawal, b.Withdrawal
	x.SourceResolution, y.SourceResolution = nil, nil
	return x == y
}

// Later operator views may advance the commit, but never alter the retained
// terminal record. Same-height hashes and clocks must be exactly identical.
func returnTerminalCovers(current, previous returnTerminalSnapshot) bool {
	if !returnCommitCovers(current.Bridge.Commit, previous.Bridge.Commit) {
		return false
	}
	if current.Bridge.Commit.Height == previous.Bridge.Commit.Height && current.Bridge != previous.Bridge {
		return false
	}
	x, y := current.Withdrawal, previous.Withdrawal
	x.Commit, y.Commit = godnode.CommittedView{}, godnode.CommittedView{}
	x.DelayRemainingSeconds, y.DelayRemainingSeconds, x.DelayElapsed, y.DelayElapsed = 0, 0, false, false
	a, b := returnTerminalSnapshot{Withdrawal: x}, returnTerminalSnapshot{Withdrawal: y}
	return sameReturnTerminal(a, b)
}
