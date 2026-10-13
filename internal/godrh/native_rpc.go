package godrh

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
)

const maxNativeBridgeRPCBytes = 16 << 10

// Explicit simulation-only native endpoint, separate from RH configuration.
// No default, ambient proxy, redirect, write method, retry or background loop.
// A provider's commit/app hash is a consistency pin, NOT a trust anchor.
type HTTPNativeBridgeSource struct{ source *HTTPSource }

func (*HTTPNativeBridgeSource) String() string     { return "native simulation bridge source (redacted)" }
func (s *HTTPNativeBridgeSource) GoString() string { return s.String() }
func (*HTTPNativeBridgeSource) MarshalJSON() ([]byte, error) {
	return []byte(`{"readOnly":true,"simulationOnly":true,"redacted":true}`), nil
}
func NewHTTPNativeBridgeSource(c Config, nativeEndpoint string) (*HTTPNativeBridgeSource, error) {
	if _, err := relayBinding(c); err != nil || !endpoint(nativeEndpoint, "simulation") {
		return nil, ErrNativeWithdrawalRead
	}
	s, err := newHTTPSource(nativeEndpoint)
	if err != nil {
		return nil, ErrNativeWithdrawalRead
	}
	s.nativeReads = true
	return &HTTPNativeBridgeSource{s}, nil
}
func (s *HTTPNativeBridgeSource) Close() {
	if s != nil {
		s.source.Close()
	}
}

func nativeHeight(height int64) string {
	if height == 0 {
		return "latest"
	}
	return strconv.FormatInt(height, 10)
}
func (s *HTTPNativeBridgeSource) Bridge(ctx context.Context, height int64) (godnode.BridgeView, error) {
	if s == nil || height < 0 {
		return godnode.BridgeView{}, ErrNativeWithdrawalRead
	}
	raw, err := s.source.rpc(ctx, "god_bridge", []string{nativeHeight(height)})
	if err != nil {
		return godnode.BridgeView{}, ErrNativeWithdrawalRead
	}
	v := godnode.BridgeView{}
	if decodeNativeBridgeView(raw, &v, false) != nil || !validNativeCommit(v.Commit, height) {
		return godnode.BridgeView{}, ErrNativeWithdrawalRead
	}
	return v, nil
}
func (s *HTTPNativeBridgeSource) BridgeWithdrawal(ctx context.Context, sequence uint64, height int64) (godnode.BridgeWithdrawalView, error) {
	if s == nil || height < 0 || sequence == 0 {
		return godnode.BridgeWithdrawalView{}, ErrNativeWithdrawalRead
	}
	raw, err := s.source.rpc(ctx, "god_bridgeWithdrawal", []string{nativeSequence(sequence), nativeHeight(height)})
	if err != nil {
		return godnode.BridgeWithdrawalView{}, ErrNativeWithdrawalRead
	}
	v := godnode.BridgeWithdrawalView{}
	if decodeNativeBridgeView(raw, &v, true) != nil || v.Sequence != sequence || !validNativeCommit(v.Commit, height) {
		return godnode.BridgeWithdrawalView{}, ErrNativeWithdrawalRead
	}
	return v, nil
}

// Match the node's exact current query schema, including nested fields. Reject
// duplicate/escaped/case aliases, null, unknown/missing keys, noncanonical
// decimal strings and numeric substitutions BEFORE typed decoding.
func decodeNativeBridgeView(raw []byte, value any, withdrawal bool) error {
	if len(raw) == 0 || len(raw) > maxNativeBridgeRPCBytes || !utf8.Valid(raw) || !uniqueTokenSourceJSON(raw) {
		return ErrNativeWithdrawalRead
	}
	required := []string{"commit", "sourceChain", "nativeChain", "assetId", "nativeGodOutsideReserveSmallestUnits", "pendingWithdrawalsSmallestUnits",
		"restrictedReserveSmallestUnits", "attestedDepositsSmallestUnits", "attestedPaymentsSmallestUnits", "reservedRollingOutflowSmallestUnits",
		"openWithdrawals", "headSequence", "nextSequence", "withdrawalDelaySeconds", "intakePaused", "outflowPaused",
		"nativeAccountingMatched", "independentSourceFinalityVerified", "realAssetsReady"}
	decimals := []string{"headSequence", "nextSequence", "withdrawalDelaySeconds"}
	var optional []string
	if withdrawal {
		required = []string{"commit", "sequence", "exists", "status", "delayRemainingSeconds", "delayElapsed", "headOfQueue", "pendingNativeGod",
			"attestedSourcePayment", "attestedSourceCancellation", "independentSourceFinalityVerified", "realAssetsReady"}
		optional = []string{"id", "nativeSender", "sourceRecipient", "amountSmallestUnits", "queuedAt", "delayEndsAt", "authorizedAt", "resolvedAt", "sourceResolution"}
		decimals = []string{"sequence", "delayRemainingSeconds"}
	}
	f, err := custodyObject(raw, required, optional)
	if err != nil || !nativeDecimalFields(f, decimals, 64) {
		return ErrNativeWithdrawalRead
	}
	commit, err := custodyObject(f["commit"], []string{"height", "time", "appHash", "synthetic", "realAssets"}, nil)
	if err != nil || !nativeDecimalFields(commit, []string{"height"}, 63) {
		return ErrNativeWithdrawalRead
	}
	if resolution := f["sourceResolution"]; resolution != nil {
		r, err := custodyObject(resolution, []string{"height", "blockHash", "transactionHash", "logIndex"}, nil)
		if err != nil || !nativeDecimalFields(r, []string{"height"}, 64) {
			return ErrNativeWithdrawalRead
		}
	}
	var text string
	if json.Unmarshal(commit["time"], &text) != nil {
		return ErrNativeWithdrawalRead
	}
	clock, err := time.Parse(time.RFC3339Nano, text)
	if err != nil || clock.UTC().Format(time.RFC3339Nano) != text {
		return ErrNativeWithdrawalRead
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(value) != nil {
		return ErrNativeWithdrawalRead
	}
	return nil
}
func nativeDecimalFields(f map[string]json.RawMessage, names []string, bits int) bool {
	for _, name := range names {
		text, err := custodyText(f[name])
		if err != nil || len(text) > 20 {
			return false
		}
		n, err := strconv.ParseUint(text, 10, bits)
		if err != nil || strconv.FormatUint(n, 10) != text {
			return false
		}
	}
	return true
}

var _ NativeBridgeReadSource = (*HTTPNativeBridgeSource)(nil)
