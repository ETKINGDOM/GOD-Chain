// Package godnativegateway is a separate keyless synthetic native-operation
// boundary. It never loads wallet/node files, starts consensus or retries sends.
package godnativegateway

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"math/big"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godnative"
	"github.com/ETKINGDOM/GOD-Chain/internal/godorigin"
)

var ErrGateway = errors.New("native gateway unavailable")

// Only the exact node commit-boundary response guarantees that this call
// stopped before ante/admission. A generic rejection, duplicate, timeout or
// transport failure cannot establish that the hash was never admitted.
var errCommitPending = errors.New("native admission deferred before ante")

type upstreamFailure string

func (e upstreamFailure) Error() string { return string(e) }

var stateErrors = [...]error{
	errors.New("network snapshot unavailable"),
	errors.New("account snapshot unavailable"),
	errors.New("delegation snapshot unavailable"),
	errors.New("final snapshot unavailable"),
}
var networkErrors = [...]error{
	errors.New("network transport unavailable"),
	errors.New("network schema unavailable"),
	errors.New("network policy unavailable"),
}

type Config struct {
	Listen            string
	Upstream          string
	Origin            string
	ExtensionOrigin   string
	ChainID           string
	CompatibleChainID string
	Gas               uint64
	MinFeePerGas      string
	ValidatorOnly     bool
	AttemptDirectory  string
	AttemptCapacity   uint64
}
type Gateway struct {
	config      Config
	codec       *godnative.Codec
	client      *http.Client
	now         func() time.Time
	slots       chan struct{}
	mu          sync.Mutex
	attempts    map[string]time.Time
	durable     map[string]bool
	inflight    map[string]bool
	store       *attemptStore
	storeFailed bool
	ips         map[string]time.Time
	statusIPs   map[string]time.Time
	last        time.Time
	stateIPs    map[string]time.Time
	stateLast   time.Time
}

func loopback(s string) bool {
	h, p, e := net.SplitHostPort(s)
	n, err := strconv.Atoi(p)
	return e == nil && err == nil && n >= 1024 && n <= 65535 && strconv.Itoa(n) == p && net.ParseIP(h).IsLoopback()
}
func validConfig(c Config) bool {
	if !validAttemptCapacity(c.AttemptCapacity) || c.ValidatorOnly && c.AttemptCapacity != 0 {
		return false
	}
	u, err := url.Parse(c.Upstream)
	if err != nil || u.Scheme != "http" || !loopback(u.Host) || u.User != nil || u.Path != "/" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || !loopback(c.Listen) || u.Host == c.Listen || !godorigin.ValidExtension(c.ExtensionOrigin) {
		return false
	}
	o, err := url.Parse(c.Origin)
	if err != nil || o.Scheme != "https" || o.User != nil || o.Port() != "" || o.Path != "" || o.RawPath != "" || o.RawQuery != "" || o.ForceQuery || o.Fragment != "" || o.Host != strings.ToLower(o.Host) || len(o.Host) > 253 || len(strings.Split(o.Host, ".")) < 2 {
		return false
	}
	for _, label := range strings.Split(o.Host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, x := range label {
			if !(x >= 'a' && x <= 'z' || x >= '0' && x <= '9' || x == '-') {
				return false
			}
		}
	}
	n, err := uint64Text(c.CompatibleChainID)
	return err == nil && n > 0
}

func New(c Config) (*Gateway, error) {
	if !validConfig(c) {
		return nil, ErrGateway
	}
	codec, err := godnative.New(godnative.Policy{ChainID: c.ChainID, Gas: c.Gas, MinFeePerGas: c.MinFeePerGas})
	if err != nil {
		return nil, ErrGateway
	}
	var store *attemptStore
	durable := map[string]bool{}
	if c.ValidatorOnly {
		if c.AttemptDirectory != "" {
			return nil, ErrGateway
		}
	} else {
		store, err = openAttempts(c, false, nil, nil)
		if err != nil {
			return nil, ErrGateway
		}
	}
	// A fresh connection per RPC prevents stale-connection replay. Requests also
	// have no GetBody, proxy, redirects or retry loop (including submission).
	t := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, DisableCompression: true, DisableKeepAlives: true, MaxConnsPerHost: 3, ResponseHeaderTimeout: 3 * time.Second, MaxResponseHeaderBytes: 8192}
	cclient := &http.Client{Transport: t, Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrGateway }}
	attempts := map[string]time.Time{}
	return &Gateway{config: c, codec: codec, client: cclient, now: time.Now, slots: make(chan struct{}, 3), attempts: attempts, durable: durable, inflight: map[string]bool{}, store: store, ips: map[string]time.Time{}, statusIPs: map[string]time.Time{}, stateIPs: map[string]time.Time{}}, nil
}

// Close requires the caller to drain handlers first. Serve does so before
// closing the exclusive attempt store; external embedding must do the same.
func (g *Gateway) Close() error {
	if g == nil {
		return ErrGateway
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.storeFailed = true
	g.client.CloseIdleConnections()
	return g.store.close()
}

// Snapshot is read-only and consolidates loopback reads, not arbitrary RPC.
// Every component and the final height-pinned tip must match exactly. A commit
// during these reads fails closed; there is no retry, signature or admission.
type snapshot struct {
	Network    network     `json:"network"`
	Account    account     `json:"account"`
	Delegation *delegation `json:"delegation"`
}

func (g *Gateway) snapshot(ctx context.Context, owner, operator string) (snapshot, error) {
	var out snapshot
	n, err := g.network(ctx, "")
	if err != nil {
		return out, err
	}
	a, err := g.ordinary(ctx, owner, n, false)
	if err != nil {
		return out, stateErrors[1]
	}
	if operator != "" {
		raw, err := g.call(ctx, "god_delegation", []any{owner, operator, n.Commit.Height})
		var d delegation
		if err != nil || strict(raw, &d) != nil || d.Commit != n.Commit || d.Owner != owner || d.Validator != operator {
			return out, stateErrors[2]
		}
		value, err := integer(d.God, fixedGod, false)
		if err != nil {
			return out, stateErrors[2]
		}
		parts := strings.Split(d.Shares, ".")
		if len(parts) > 2 || len(parts) == 2 && (len(parts[1]) != 18 || strings.IndexFunc(parts[1], func(r rune) bool { return r < '0' || r > '9' }) >= 0) {
			return out, stateErrors[2]
		}
		whole, err := integer(parts[0], maxG, false)
		if err != nil {
			return out, stateErrors[2]
		}
		nonzero := whole.Sign() > 0 || len(parts) == 2 && strings.Trim(parts[1], "0") != ""
		if d.Exists != nonzero || !d.Exists && value.Sign() != 0 {
			return out, stateErrors[2]
		}
		out.Delegation = &d
	}
	end, err := g.network(ctx, n.Commit.Height)
	if err != nil || end != n {
		return out, stateErrors[3]
	}
	out.Network, out.Account = n, a
	return out, nil
}

// Read-only preparation has a separate bounded budget: it cannot consume or
// release the signed-transaction attempt guard or the existing status budget.
func (g *Gateway) reserveState(ip string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	for k, t := range g.stateIPs {
		if now.Sub(t) >= time.Minute {
			delete(g.stateIPs, k)
		}
	}
	if now.Sub(g.stateLast) < 100*time.Millisecond || now.Sub(g.stateIPs[ip]) < 500*time.Millisecond || len(g.stateIPs) >= 1024 {
		return false
	}
	g.stateIPs[ip], g.stateLast = now, now
	return true
}

func (g *Gateway) call(ctx context.Context, method string, params []any) (json.RawMessage, error) {
	b, err := json.Marshal(struct {
		Version string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Method  string `json:"method"`
		Params  []any  `json:"params"`
	}{"2.0", 1, method, params})
	if err != nil {
		return nil, ErrGateway
	}
	r, err := http.NewRequestWithContext(ctx, "POST", g.config.Upstream, bytes.NewReader(b))
	if err != nil {
		return nil, ErrGateway
	}
	r.GetBody = nil
	r.Header.Set("Content-Type", "application/json")
	response, err := g.client.Do(r)
	if err != nil {
		return nil, upstreamFailure("transport-unavailable")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(raw) > 65536 {
		return nil, upstreamFailure("response-unavailable")
	}
	if response.StatusCode != 200 {
		if response.StatusCode == 429 {
			return nil, upstreamFailure("upstream-budget")
		}
		return nil, upstreamFailure("upstream-http")
	}
	if unique(raw) != nil {
		return nil, upstreamFailure("response-invalid")
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(raw, &envelope) != nil || len(envelope) != 3 || string(envelope["jsonrpc"]) != `"2.0"` || string(envelope["id"]) != "1" {
		return nil, upstreamFailure("response-invalid")
	}
	if envelope["error"] != nil {
		var failure struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		var fields map[string]json.RawMessage
		if strict(envelope["error"], &failure) != nil || json.Unmarshal(envelope["error"], &fields) != nil || len(fields) != 2 || fields["code"] == nil || fields["message"] == nil {
			return nil, upstreamFailure("response-invalid")
		}
		if method == "god_submitTransaction" {
			switch {
			case failure.Code == -32001 && failure.Message == "retry after application commit; transaction not admitted":
				return nil, errCommitPending
			case failure.Code == -32002 && failure.Message == "signed transaction not admitted":
				return nil, upstreamFailure("node-declined")
			case failure.Code == -32005 && failure.Message == "submission outcome unknown; check transaction hash before retrying":
				return nil, upstreamFailure("node-outcome-unknown")
			}
		}
		return nil, upstreamFailure("node-error")
	}
	if envelope["result"] == nil {
		return nil, upstreamFailure("response-invalid")
	}
	return envelope["result"], nil
}

func (g *Gateway) network(ctx context.Context, height string) (network, error) {
	params := []any{}
	if height != "" {
		params = append(params, height)
	}
	raw, err := g.call(ctx, "god_network", params)
	var n network
	if err != nil {
		return n, networkErrors[0]
	}
	if strict(raw, &n) != nil {
		return n, networkErrors[1]
	}
	if !g.validNetwork(n) {
		return n, networkErrors[2]
	}
	return n, nil
}

func (g *Gateway) ordinary(ctx context.Context, s string, n network, mustExist bool) (account, error) {
	var a account
	owner, err := godnative.Account(s)
	if err != nil {
		return a, ErrGateway
	}
	evm, err := godaddress.ToEVM(owner)
	if err != nil {
		return a, ErrGateway
	}
	// Both independent reads must finish before the final exact-tip check.
	// They carry no signing material, write no state and are never retried.
	var raw, codeRaw json.RawMessage
	var accountErr, codeErr error
	var reads sync.WaitGroup
	reads.Add(2)
	go func() { defer reads.Done(); raw, accountErr = g.call(ctx, "god_account", []any{s, n.Commit.Height}) }()
	go func() { defer reads.Done(); codeRaw, codeErr = g.call(ctx, "eth_getCode", []any{evm, "latest"}) }()
	reads.Wait()
	if accountErr != nil || strict(raw, &a) != nil || a.Commit != n.Commit || a.Native != s || a.Compatible != evm || a.Module || mustExist && !a.Exists {
		return a, ErrGateway
	}
	if _, err := uint64Text(a.Number); err != nil {
		return a, ErrGateway
	}
	if _, err := uint64Text(a.Sequence); err != nil {
		return a, ErrGateway
	}
	if _, err := integer(a.Balance, fixedGod, false); err != nil {
		return a, ErrGateway
	}
	for _, x := range []string{a.Spendable, a.Unclaimed, a.Pending} {
		if _, err := integer(x, maxG, false); err != nil {
			return a, ErrGateway
		}
	}
	if !validBalances(n, a) {
		return a, ErrGateway
	}
	// G ledger transfers need not create an auth/bank account. Such a recipient
	// can already hold G while still lacking GOD for fees; do not erase that view.
	if !a.Exists && (a.Number != "0" || a.Sequence != "0" || a.Balance != "0") {
		return a, ErrGateway
	}
	// Account lookups are height-pinned. Code lookups are latest-only: the final
	// pinned network check rejects any intervening commit.
	var code string
	if codeErr != nil || json.Unmarshal(codeRaw, &code) != nil || code != "0x" {
		return a, ErrGateway
	}
	return a, nil
}

func (g *Gateway) preflight(ctx context.Context, d *godnative.Decoded) error {
	n, err := g.network(ctx, "")
	if err != nil {
		return ErrGateway
	}
	i := d.Intent
	a, err := g.ordinary(ctx, i.Sender, n, true)
	if err != nil || a.Sequence != i.Sequence {
		return ErrGateway
	}
	number, err := uint64Text(a.Number)
	if err != nil || g.codec.Verify(d, number) != nil {
		return ErrGateway
	}
	fee, _ := new(big.Int).SetString(i.Fee, 10)
	balance, _ := new(big.Int).SetString(a.Balance, 10)
	amount := big.NewInt(0)
	if i.Amount != "" {
		amount, _ = new(big.Int).SetString(i.Amount, 10)
	}
	need := new(big.Int).Set(fee)
	if i.Operation == "delegate" || i.Operation == "donate-god" {
		need.Add(need, amount)
	}
	if balance.Cmp(need) < 0 {
		return ErrGateway
	}
	switch i.Operation {
	case "undelegate":
		raw, err := g.call(ctx, "god_delegation", []any{i.Sender, i.Validator, n.Commit.Height})
		var v delegation
		if err != nil || strict(raw, &v) != nil || v.Commit != n.Commit || v.Owner != i.Sender || v.Validator != i.Validator || !v.Exists {
			return ErrGateway
		}
		god, err := integer(v.God, fixedGod, true)
		if err != nil || god.Cmp(amount) < 0 {
			return ErrGateway
		}
		remaining := new(big.Int).Sub(god, amount)
		if remaining.Sign() > 0 && remaining.Cmp(big.NewInt(1_000_000_000_000_000_000)) < 0 {
			return ErrGateway
		}
		parts := strings.Split(v.Shares, ".")
		if len(parts) != 2 || len(parts[1]) != 18 {
			return ErrGateway
		}
		whole, e := integer(parts[0], maxG, false)
		frac, e2 := integer(strings.TrimLeft(parts[1], "0"), maxG, false)
		if strings.Trim(parts[1], "0") == "" {
			frac = big.NewInt(0)
			e2 = nil
		}
		if e != nil || e2 != nil || whole.Sign() == 0 && frac.Sign() == 0 {
			return ErrGateway
		}
	case "claim-g":
		if a.Unclaimed == "0" {
			return ErrGateway
		}
	case "transfer-g", "redeem-g":
		spendable, _ := new(big.Int).SetString(a.Spendable, 10)
		if spendable.Cmp(amount) < 0 {
			return ErrGateway
		}
		if i.Recipient != i.Sender {
			recipient, err := g.ordinary(ctx, i.Recipient, n, false)
			if err != nil || !validBalances(n, a, recipient) {
				return ErrGateway
			}
		}
		if i.Operation == "redeem-g" {
			pool, _ := new(big.Int).SetString(n.Pool, 10)
			total, _ := new(big.Int).SetString(n.Outstanding, 10)
			min, _ := new(big.Int).SetString(i.MinGodOut, 10)
			if pool.Sign() == 0 || total.Sign() == 0 || amount.Cmp(total) > 0 {
				return ErrGateway
			}
			quote := new(big.Int).Quo(new(big.Int).Mul(pool, amount), total)
			if quote.Cmp(min) < 0 {
				return ErrGateway
			}
			deadline, _ := strconv.ParseInt(i.Deadline, 10, 64)
			t, _ := time.Parse(time.RFC3339Nano, n.Commit.Time)
			end := time.Unix(0, deadline)
			if !end.After(t) || end.After(t.Add(10*time.Minute)) {
				return ErrGateway
			}
		}
	}
	end, err := g.network(ctx, n.Commit.Height)
	if err != nil || end != n {
		return ErrGateway
	}
	return nil
}

type reply struct {
	Hash       string            `json:"hash"`
	Status     string            `json:"status"`
	Attempted  bool              `json:"submissionAttempted"`
	Included   bool              `json:"included"`
	Synthetic  bool              `json:"synthetic"`
	RealAssets bool              `json:"realAssets"`
	Height     string            `json:"height,omitempty"`
	Index      string            `json:"index,omitempty"`
	Code       *uint32           `json:"code,omitempty"`
	GasWanted  string            `json:"gasWanted,omitempty"`
	GasUsed    string            `json:"gasUsed,omitempty"`
	OfferedFee string            `json:"offeredFeeSmallestUnits,omitempty"`
	BlockHash  string            `json:"blockHash,omitempty"`
	Operation  *godnative.Intent `json:"operation,omitempty"`
}

func result(w http.ResponseWriter, status int, r reply) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(r)
}
func initial(hash, status string, attempted bool) reply {
	return reply{Hash: hash, Status: status, Attempted: attempted, Synthetic: true}
}

// Reserve before preflight; release on explicit no-attempt failure. Attempted
// hashes are durably blocked before the one upstream attempt, without TTL or
// read-side release. This is a conservative guard, not an exactly-once queue.
type reservation uint8

const (
	reservationAllowed reservation = iota
	reservationLimited
	reservationDuplicate
)

func (g *Gateway) reserve(ip, hash string, submit bool) reservation {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	for k, t := range g.attempts {
		if !g.inflight[k] && now.Sub(t) >= time.Hour {
			delete(g.attempts, k)
			delete(g.durable, k) // cache only; never delete a disk hash here
		}
	}
	ips := g.ips
	if !submit {
		ips = g.statusIPs
	}
	for k, t := range ips {
		if now.Sub(t) >= time.Minute {
			delete(ips, k)
		}
	}
	// Check known attempts first even when a quota also rejects the request.
	// A read can never turn an uncertain attempted hash into a fresh request.
	if submit {
		if _, ok := g.attempts[hash]; ok {
			return reservationDuplicate
		}
		if g.storeFailed || !g.store.fileOK() {
			g.storeFailed = true
			return reservationLimited
		}
		found, err := g.store.contains(hash)
		if err != nil {
			g.storeFailed = true
			return reservationLimited
		}
		if found {
			return reservationDuplicate
		}
		if g.store.count >= g.store.capacity {
			return reservationLimited
		}
	}
	// Keep the combined global budget. Only the per-IP status and submission
	// windows are separated, so confirming an earlier TX does not exhaust the
	// next independently reviewed submission's five-second window.
	if now.Sub(g.last) < time.Second || now.Sub(ips[ip]) < 5*time.Second || len(ips) >= 1024 {
		return reservationLimited
	}
	if submit {
		if len(g.attempts) >= attemptLimit {
			oldest := ""
			var when time.Time
			for k, t := range g.attempts {
				if g.durable[k] && !g.inflight[k] && (oldest == "" || t.Before(when)) {
					oldest, when = k, t
				}
			}
			if oldest == "" {
				return reservationLimited
			}
			delete(g.attempts, oldest)
			delete(g.durable, oldest)
		}
		g.attempts[hash] = now
		g.inflight[hash] = true
	}
	ips[ip] = now
	g.last = now
	return reservationAllowed
}

func (g *Gateway) finish(hash string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.inflight, hash)
}

func (g *Gateway) persist(hash string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, reserved := g.attempts[hash]; !reserved || g.storeFailed || g.store.change(hash, false) != nil {
		g.storeFailed = true
		// Retain the in-process guard even for an ambiguous disk failure. No
		// upstream request is made; never delete or retry the storage write.
		g.durable[hash] = true
		return false
	}
	g.durable[hash] = true
	return true
}
func (g *Gateway) release(hash string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.durable[hash] {
		if g.storeFailed || g.store.change(hash, true) != nil {
			g.storeFailed = true
			return false
		}
		delete(g.durable, hash)
	}
	delete(g.attempts, hash)
	return true
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// A separately deployed read service cannot enter any transaction route,
	// including preflight, status reservations or submission decoding.
	if g.config.ValidatorOnly && r.URL.Path != "/validator" && r.URL.Path != "/validators" {
		http.Error(w, "request rejected", 403)
		return
	}
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || !net.ParseIP(peer).IsLoopback() || r.Host != g.config.Listen || r.URL.RawPath != "" || r.URL.RawQuery != "" || r.URL.ForceQuery || r.URL.Fragment != "" || r.URL.Path != "/submit" && r.URL.Path != "/status" && r.URL.Path != "/state" && r.URL.Path != "/unbonding" && r.URL.Path != "/validator" && r.URL.Path != "/validators" || len(r.Header.Values("Origin")) != 1 || !godorigin.Allowed(g.config.Origin, g.config.ExtensionOrigin, r.Header.Get("Origin")) {
		http.Error(w, "request rejected", 403)
		return
	}
	if godorigin.CORS(w, r) {
		return
	}
	if r.Method != "POST" || len(r.Header.Values("Content-Type")) != 1 || r.Header.Get("Content-Encoding") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
		http.Error(w, "request rejected", 400)
		return
	}
	media, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || len(params) != 0 {
		http.Error(w, "request rejected", 400)
		return
	}
	if len(r.Header.Values("X-God-Client-IP")) != 1 {
		http.Error(w, "proxy required", 403)
		return
	}
	ip := net.ParseIP(r.Header.Get("X-God-Client-IP"))
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
		http.Error(w, "proxy required", 403)
		return
	}
	if ip.To4() == nil {
		ip = ip.Mask(net.CIDRMask(64, 128))
	}
	clientIP := ip.String()
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 5000))
	if err != nil {
		http.Error(w, "request rejected", 400)
		return
	}
	select {
	case g.slots <- struct{}{}:
		defer func() { <-g.slots }()
	default:
		http.Error(w, "busy", 429)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if r.URL.Path == "/validators" {
		var q struct {
			After  string `json:"after"`
			Height string `json:"height"`
		}
		validHeight := func() bool { _, err := integer(q.Height, new(big.Int).SetInt64(1<<63-1), true); return err == nil }
		if strict(raw, &q) != nil || q.After == "" && q.Height != "" || q.After != "" && (!validOperator(q.After) || !validHeight()) {
			http.Error(w, "request rejected", 400)
			return
		}
		if !g.reserveState(clientIP) {
			http.Error(w, "read budget exhausted", 429)
			return
		}
		view, err := g.validatorsSnapshot(ctx, q.After, q.Height)
		if err != nil {
			http.Error(w, "validator page unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(view)
		return
	}
	if r.URL.Path == "/validator" {
		var q struct {
			Validator string `json:"validator"`
		}
		if strict(raw, &q) != nil || !validOperator(q.Validator) {
			http.Error(w, "request rejected", 400)
			return
		}
		if !g.reserveState(clientIP) {
			http.Error(w, "read budget exhausted", 429)
			return
		}
		view, err := g.validatorSnapshot(ctx, q.Validator)
		if err != nil {
			http.Error(w, "validator snapshot unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(view)
		return
	}
	if r.URL.Path == "/state" || r.URL.Path == "/unbonding" {
		var q struct {
			Address   string `json:"address"`
			Validator string `json:"validator"`
		}
		if strict(raw, &q) != nil {
			http.Error(w, "request rejected", 400)
			return
		}
		if _, err := godnative.Account(q.Address); err != nil {
			http.Error(w, "request rejected", 400)
			return
		}
		if r.URL.Path == "/unbonding" && q.Validator == "" || q.Validator != "" && godnative.Validate(godnative.Intent{Operation: "undelegate", Sender: q.Address, Validator: q.Validator, Amount: "1000000000000000000"}) != nil {
			http.Error(w, "request rejected", 400)
			return
		}
		if !g.reserveState(clientIP) {
			http.Error(w, "read budget exhausted", 429)
			return
		}
		if r.URL.Path == "/unbonding" {
			view, err := g.unbondingSnapshot(ctx, q.Address, q.Validator)
			if err != nil {
				http.Error(w, "unbonding snapshot unavailable", 503)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(view)
			return
		}
		state, err := g.snapshot(ctx, q.Address, q.Validator)
		if err != nil {
			// Only these fixed public messages, never an upstream error, address,
			// storage path or request body, are exposed for read diagnostics.
			http.Error(w, err.Error(), 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state)
		return
	}
	if r.URL.Path == "/status" {
		var q struct {
			Hash string `json:"hash"`
		}
		if strict(raw, &q) != nil || !lowerHash(q.Hash, true) {
			http.Error(w, "request rejected", 400)
			return
		}
		if g.reserve(clientIP, q.Hash, false) != reservationAllowed {
			result(w, 429, initial(q.Hash, "rate-limited", false))
			return
		}
		answer := g.status(ctx, q.Hash)
		result(w, 200, answer)
		return
	}
	var q struct {
		Hash string `json:"hash"`
		Wire string `json:"wire"`
	}
	if strict(raw, &q) != nil || !lowerHash(q.Hash, true) || !strings.HasPrefix(q.Wire, "0x") || q.Wire != strings.ToLower(q.Wire) || len(q.Wire) > 2+godnative.MaxWireBytes*2 {
		http.Error(w, "request rejected", 400)
		return
	}
	wire, err := hex.DecodeString(q.Wire[2:])
	if err != nil {
		result(w, 400, initial(q.Hash, "rejected", false))
		return
	}
	d, err := g.codec.Decode(wire)
	if err != nil || d.Hash != q.Hash {
		result(w, 400, initial(q.Hash, "rejected", false))
		return
	}
	if reserved := g.reserve(clientIP, q.Hash, true); reserved != reservationAllowed {
		if reserved == reservationDuplicate {
			result(w, 429, initial(q.Hash, "blocked-check-status", false))
		} else {
			result(w, 409, initial(q.Hash, "not-submitted", false))
		}
		return
	}
	defer g.finish(q.Hash)
	if g.preflight(ctx, d) != nil {
		g.release(q.Hash)
		result(w, 409, initial(q.Hash, "not-submitted", false))
		return
	}
	if !g.persist(q.Hash) {
		result(w, 409, initial(q.Hash, "not-submitted", false))
		return
	}
	response, err := g.call(ctx, "god_submitTransaction", []any{q.Wire})
	if errors.Is(err, errCommitPending) {
		// No automatic retry. A future explicit review must read fresh state.
		// This does not resolve any earlier unknown attempt for another hash.
		if g.release(q.Hash) {
			result(w, 409, initial(q.Hash, "not-admitted", true))
		} else {
			result(w, 202, initial(q.Hash, "unknown", true))
		}
		return
	}
	if err != nil {
		category := "unavailable"
		var failure upstreamFailure
		if errors.As(err, &failure) {
			category = string(failure)
		}
		// Bounded fixed categories only: no upstream logs, identities, wire,
		// request bodies, credentials or private application text.
		log.Printf("native admission unresolved: %s", category)
	}
	var admitted struct {
		Hash       string `json:"hash"`
		Admitted   bool   `json:"admitted"`
		Included   bool   `json:"included"`
		Synthetic  bool   `json:"synthetic"`
		RealAssets bool   `json:"realAssets"`
	}
	if err != nil || strict(response, &admitted) != nil || admitted.Hash != q.Hash || !admitted.Admitted || admitted.Included || !admitted.Synthetic || admitted.RealAssets {
		result(w, 202, initial(q.Hash, "unknown", true))
		return
	}
	result(w, 202, initial(q.Hash, "submitted", true))
}

func (g *Gateway) status(ctx context.Context, hash string) reply {
	unknown := initial(hash, "unknown", false)
	n, err := g.network(ctx, "")
	if err != nil {
		return unknown
	}
	raw, err := g.call(ctx, "god_transaction", []any{hash})
	if err != nil || string(raw) == "null" {
		return unknown
	}
	var tx transaction
	if strict(raw, &tx) != nil || tx.Hash != hash || !tx.Included || tx.Successful != (tx.Code == 0) || !tx.Synthetic || tx.RealAssets {
		return unknown
	}
	h, e := integer(tx.Height, maxInt, true)
	tip, _ := integer(n.Commit.Height, maxInt, true)
	if e != nil || h.Cmp(tip) > 0 {
		return unknown
	}
	if _, e := integer(tx.Index, maxUint, false); e != nil {
		return unknown
	}
	gas, e := uint64Text(tx.GasWanted)
	if e != nil || gas != g.config.Gas {
		return unknown
	}
	if _, e := integer(tx.GasUsed, maxInt, false); e != nil {
		return unknown
	}
	raw, err = g.call(ctx, "god_transactionDetails", []any{"native", hash})
	var d details
	if err != nil || strict(raw, &d) != nil || d.Hash != hash || d.Height != tx.Height || d.Index != tx.Index || d.Code != tx.Code || d.Successful != tx.Successful || d.GasWanted != tx.GasWanted || d.GasUsed != tx.GasUsed || !d.Synthetic || d.RealAssets || d.Kind != "native" || !lowerHash(d.BlockHash, true) || len(d.Operations) != 1 || !strings.HasSuffix(d.Fee, "agod") {
		return unknown
	}
	fee, e := integer(strings.TrimSuffix(d.Fee, "agod"), big.NewInt(10_000_000_000_000_000), true)
	min, _ := new(big.Int).SetString(g.config.MinFeePerGas, 10)
	if e != nil || fee.Cmp(new(big.Int).Mul(min, new(big.Int).SetUint64(g.config.Gas))) < 0 {
		return unknown
	}
	var intent godnative.Intent
	if strict(d.Operations[0], &intent) != nil || godnative.Validate(intent) != nil || intent.Sequence != "" || intent.Gas != "" || intent.Fee != "" {
		return unknown
	}
	var actual, expected map[string]json.RawMessage
	canonical, _ := json.Marshal(intent)
	if json.Unmarshal(d.Operations[0], &actual) != nil || json.Unmarshal(canonical, &expected) != nil || len(actual) != len(expected) {
		return unknown
	}
	for k := range actual {
		if expected[k] == nil {
			return unknown
		}
	}
	end, err := g.network(ctx, n.Commit.Height)
	if err != nil || end != n {
		return unknown
	}
	answer := initial(hash, "confirmed", false)
	if tx.Code != 0 {
		answer.Status = "failed"
	}
	answer.Included = true
	answer.Height = tx.Height
	answer.Index = tx.Index
	answer.Code = &tx.Code
	answer.GasWanted = tx.GasWanted
	answer.GasUsed = tx.GasUsed
	answer.OfferedFee = fee.String()
	answer.BlockHash = d.BlockHash
	answer.Operation = &intent
	return answer
}

func Serve(ctx context.Context, c Config) error {
	g, err := New(c)
	if err != nil {
		return ErrGateway
	}
	defer g.Close()
	s := &http.Server{Addr: c.Listen, Handler: g, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 10 * time.Second, MaxHeaderBytes: 8192, ErrorLog: log.New(io.Discard, "", 0)}
	listener, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return ErrGateway
	}
	done := make(chan error, 1)
	go func() { done <- s.Serve(listener) }()
	select {
	case <-ctx.Done():
		stop, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if s.Shutdown(stop) != nil {
			_ = s.Close()
		}
		<-done
		return nil
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return ErrGateway
	}
}
