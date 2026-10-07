//go:build go1.25

package godtestnet

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
)

var ErrHealth = errors.New("synthetic testnet health check did not pass")

// HealthReport deliberately excludes endpoints, identities, hashes, response
// material, paths and keys. Healthy is an availability sample, not a quorum,
// authenticated state, decentralization or public-launch certification.
type HealthReport struct {
	Synthetic        bool   `json:"synthetic"`
	RealAssets       bool   `json:"realAssets"`
	PublicAcceptance bool   `json:"publicAcceptance"`
	Healthy          bool   `json:"healthy"`
	Reason           string `json:"reason"`
	Height           string `json:"height,omitempty"`
	ConnectedPeers   int    `json:"connectedPeers"`
	BlockAgeSeconds  int64  `json:"blockAgeSeconds"`
	CommitPending    bool   `json:"commitPending"`
}

type healthLive struct {
	Running           bool   `json:"running"`
	ApplicationUsable bool   `json:"applicationUsable"`
	ApplicationHeight string `json:"applicationHeight"`
	BlockStoreHeight  string `json:"blockStoreHeight"`
	CommitPending     bool   `json:"commitPending"`
	ConnectedPeers    int    `json:"connectedPeers"`
	CatchingUp        bool   `json:"catchingUp"`
	ConsensusProgress string `json:"consensusProgress"`
	Synthetic         bool   `json:"synthetic"`
	RealAssets        bool   `json:"realAssets"`
}

func healthEndpoint(text string) bool {
	u, err := url.Parse(text)
	if err != nil || u.Opaque != "" || u.User != nil || u.Host == "" || u.Hostname() == "" ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/") {
		return false
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 || strconv.Itoa(n) != p {
			return false
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	if ip != nil && (ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
		return false
	}
	if u.Scheme == "http" {
		return ip != nil && ip.IsLoopback()
	}
	if u.Scheme != "https" {
		return false
	}
	if ip != nil {
		return true
	}
	if len(u.Hostname()) > 253 {
		return false
	}
	for _, label := range strings.Split(u.Hostname(), ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if c != '-' && !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') {
				return false
			}
		}
	}
	return true
}

func exactKeys(raw []byte, keys ...string) bool {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || len(object) != len(keys) {
		return false
	}
	for _, key := range keys {
		if _, ok := object[key]; !ok {
			return false
		}
	}
	return true
}

func healthCall(ctx context.Context, client *http.Client, endpoint, method string, id int64) ([]byte, error) {
	return healthOriginCall(ctx, client, endpoint, method, id, "")
}

func healthOriginCall(ctx context.Context, client *http.Client, endpoint, method string, id int64, origin string) ([]byte, error) {
	// Fixed read methods and empty params only. Health supplies no origin; the
	// service smoke check supplies only its validated companion origin.
	if method != "god_liveness" && method != "god_network" {
		return nil, ErrHealth
	}
	raw, _ := json.Marshal(struct {
		Version string   `json:"jsonrpc"`
		ID      int64    `json:"id"`
		Method  string   `json:"method"`
		Params  []string `json:"params"`
	}{"2.0", id, method, []string{}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, ErrHealth
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, ErrHealth
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, ErrHealth
	}
	if origin != "" && !smokeCORS(response.Header, origin) {
		return nil, ErrHealth
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (16<<10)+1))
	if err != nil || len(body) > 16<<10 {
		return nil, ErrHealth
	}
	var reply struct {
		Version string          `json:"jsonrpc"`
		ID      int64           `json:"id"`
		Result  json.RawMessage `json:"result"`
	}
	if decode(body, &reply) != nil || !exactKeys(body, "jsonrpc", "id", "result") || reply.Version != "2.0" || reply.ID != id || len(reply.Result) == 0 || bytes.Equal(reply.Result, []byte("null")) {
		return nil, ErrHealth
	}
	return reply.Result, nil
}

func healthAmount(s string) bool {
	if len(s) == 0 || len(s) > 78 || len(s) > 1 && s[0] == '0' {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	n, ok := new(big.Int).SetString(s, 10)
	return ok && n.Sign() >= 0 && n.BitLen() <= 256
}

func evaluateHealth(b *Bundle, liveRaw, networkRaw []byte, now time.Time, maxAge time.Duration, minPeers int) (HealthReport, error) {
	r := HealthReport{Synthetic: true, Reason: "invalid-response"}
	var live healthLive
	var network godnode.NetworkView
	if decode(liveRaw, &live) != nil || !exactKeys(liveRaw, "running", "applicationUsable", "applicationHeight", "blockStoreHeight", "commitPending", "connectedPeers", "catchingUp", "consensusProgress", "synthetic", "realAssets") ||
		decode(networkRaw, &network) != nil || !exactKeys(networkRaw, "commit", "chainId", "evmChainId", "nativeDenom", "decimals", "totalGod", "rewardPoolGod", "pendingRewardGod", "outstandingG", "pendingG") {
		return r, ErrHealth
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(networkRaw, &fields)
	var commitFields map[string]json.RawMessage
	_ = json.Unmarshal(fields["commit"], &commitFields)
	var evmText, heightText string
	if json.Unmarshal(fields["evmChainId"], &evmText) != nil || evmText != strconv.FormatUint(network.EVMChainID, 10) ||
		json.Unmarshal(commitFields["height"], &heightText) != nil || heightText != strconv.FormatInt(network.Commit.Height, 10) {
		return r, ErrHealth
	}
	if !exactKeys(fields["commit"], "height", "time", "appHash", "synthetic", "realAssets") ||
		!live.Synthetic || live.RealAssets || !network.Commit.Synthetic || network.Commit.RealAssets || network.Commit.Height < 1 ||
		live.ConnectedPeers < 0 || live.ConnectedPeers > 32 || len(live.ConsensusProgress) == 0 || len(live.ConsensusProgress) > 64 || network.Commit.Time.IsZero() {
		return r, ErrHealth
	}
	hash, err := hex.DecodeString(network.Commit.AppHash)
	if err != nil || len(hash) != 32 || hex.EncodeToString(hash) != network.Commit.AppHash {
		return r, ErrHealth
	}
	for _, amount := range []string{network.TotalGod, network.RewardPoolGod, network.PendingRewardGod, network.OutstandingG, network.PendingG} {
		if !healthAmount(amount) {
			return r, ErrHealth
		}
	}
	if network.ChainID != b.Runtime.ChainID || network.EVMChainID != b.Runtime.EVMChainID || network.NativeDenom != "agod" || network.Decimals != 18 || network.TotalGod != "1000000000000000000000000000" {
		r.Reason = "network-binding"
		return r, ErrHealth
	}
	app, e1 := decimalUint(live.ApplicationHeight)
	store, e2 := decimalUint(live.BlockStoreHeight)
	height := uint64(network.Commit.Height)
	if e1 != nil || e2 != nil || app < 1 || store < app || store-app > 1 || (app > height && app-height > 1) || (height > app && height-app > 2) {
		r.Reason = "sampling-overlap"
		return r, ErrHealth
	}
	r.Height, r.ConnectedPeers, r.CommitPending = strconv.FormatInt(network.Commit.Height, 10), live.ConnectedPeers, live.CommitPending
	age := now.Sub(network.Commit.Time)
	if age < -5*time.Second {
		r.Reason = "clock-skew"
		return r, ErrHealth
	}
	if age < 0 {
		age = 0
	}
	r.BlockAgeSeconds = int64(age / time.Second)
	if age > maxAge {
		r.Reason = "stale-block"
		return r, ErrHealth
	}
	if !live.Running || !live.ApplicationUsable {
		r.Reason = "not-running"
		return r, ErrHealth
	}
	if live.CatchingUp {
		r.Reason = "catching-up"
		return r, ErrHealth
	}
	if live.ConnectedPeers < minPeers {
		r.Reason = "insufficient-peers"
		return r, ErrHealth
	}
	r.Healthy, r.Reason = true, "sample-passed"
	return r, nil
}

// Health performs exactly two bounded read-only RPC calls after validating the
// entire reviewed private bundle. It never submits, signs, follows redirects,
// uses an environment proxy, changes files or starts/stops a node.
func healthTransport() *http.Transport {
	return &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: 2 * time.Second, ResponseHeaderTimeout: 3 * time.Second, MaxResponseHeaderBytes: 8192, DisableKeepAlives: true, DisableCompression: true}
}

func Health(ctx context.Context, bundlePath, expectedBundle, endpoint string, maxAge time.Duration, minPeers int) (HealthReport, error) {
	r := HealthReport{Synthetic: true, Reason: "configuration"}
	if ctx == nil || !healthEndpoint(endpoint) || maxAge < 10*time.Second || maxAge > 10*time.Minute || minPeers < 0 || minPeers > 32 {
		return r, ErrHealth
	}
	b, err := loadBundle(bundlePath, expectedBundle)
	if err != nil {
		return r, ErrHealth
	}
	transport := healthTransport()
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrHealth }}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	live, err := healthCall(ctx, client, endpoint, "god_liveness", 1)
	if err != nil {
		r.Reason = "unavailable"
		return r, ErrHealth
	}
	network, err := healthCall(ctx, client, endpoint, "god_network", 2)
	if err != nil {
		r.Reason = "unavailable"
		return r, ErrHealth
	}
	return evaluateHealth(&b, live, network, time.Now().UTC(), maxAge, minPeers)
}
