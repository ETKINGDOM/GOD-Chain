package godnode

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	cmtlocal "github.com/cometbft/cometbft/rpc/client/local"
	cmttypes "github.com/cometbft/cometbft/types"
)

// RPCOptions allows exact hosts/origins only. Network exposure needs a separate
// explicit opt-in; use a hardened TLS proxy for an eventual public endpoint.
// No authentication key, signing service or unsafe engine API is supplied.
type RPCOptions struct {
	Hosts        []string
	Origins      []string
	AllowNetwork bool
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type rpcRequest struct {
	JSONRPC string            `json:"jsonrpc"`
	ID      json.RawMessage   `json:"id"`
	Method  string            `json:"method"`
	Params  []json.RawMessage `json:"params"`
}
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type TestnetRPC struct {
	n              *LocalNode
	client         *cmtlocal.Local
	hosts, origins map[string]bool
	slots          chan struct{}
	mu             sync.Mutex
	tokens         float64
	last           time.Time
}

func NewTestnetRPC(n *LocalNode, options RPCOptions) (*TestnetRPC, error) {
	if n == nil || n.node == nil || n.app == nil || !n.app.config.Prototype || n.app.config.BridgeGenesis != nil || len(options.Hosts) == 0 || len(options.Hosts) > 16 || len(options.Origins) > 16 {
		return nil, ErrConfig
	}
	hosts, origins, err := rpcAccess(options)
	if err != nil {
		return nil, err
	}
	r := &TestnetRPC{n: n, client: cmtlocal.New(n.node), hosts: hosts, origins: origins, slots: make(chan struct{}, 4), tokens: 20, last: time.Now()}
	return r, nil
}

func ValidateRPCOptions(options RPCOptions) error { _, _, err := rpcAccess(options); return err }

func rpcAccess(options RPCOptions) (map[string]bool, map[string]bool, error) {
	if len(options.Hosts) == 0 || len(options.Hosts) > 16 || len(options.Origins) > 16 {
		return nil, nil, ErrConfig
	}
	hosts, origins := map[string]bool{}, map[string]bool{}
	for _, host := range options.Hosts {
		if !rpcHostname(host) || hosts[host] {
			return nil, nil, ErrConfig
		}
		hosts[host] = true
	}
	for _, origin := range options.Origins {
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Path != "" || u.RawPath != "" || !rpcHostname(u.Hostname()) || origins[origin] ||
			(u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || net.ParseIP(u.Hostname()).IsLoopback())) && u.Scheme != "chrome-extension" && u.Scheme != "moz-extension") {
			return nil, nil, ErrConfig
		}
		if u.Port() != "" {
			port, e := strconv.Atoi(u.Port())
			if e != nil || port < 1 || port > 65535 || strconv.Itoa(port) != u.Port() || u.Scheme == "chrome-extension" || u.Scheme == "moz-extension" {
				return nil, nil, ErrConfig
			}
		}
		origins[origin] = true
	}
	return hosts, origins, nil
}

func rpcHostname(host string) bool {
	if len(host) == 0 || len(host) > 253 || host != strings.ToLower(host) {
		return false
	}
	if net.ParseIP(host) != nil {
		return true
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

func (r *TestnetRPC) allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	r.tokens += now.Sub(r.last).Seconds() * 10
	r.last = now
	if r.tokens > 20 {
		r.tokens = 20
	}
	if r.tokens < 1 {
		return false
	}
	r.tokens--
	return true
}

func (r *TestnetRPC) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	host := req.Host
	if h, _, e := net.SplitHostPort(host); e == nil {
		host = h
	}
	if !r.hosts[strings.ToLower(host)] {
		http.Error(w, "host rejected", http.StatusForbidden)
		return
	}
	origin := req.Header.Get("Origin")
	if origin != "" {
		if !r.origins[origin] {
			http.Error(w, "origin rejected", http.StatusForbidden)
			return
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	}
	if req.URL.Path != "/" || req.URL.RawQuery != "" {
		http.NotFound(w, req)
		return
	}
	if req.Method == "OPTIONS" && origin != "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if req.Method != "POST" {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if strings.Split(req.Header.Get("Content-Type"), ";")[0] != "application/json" {
		http.Error(w, "JSON required", http.StatusUnsupportedMediaType)
		return
	}
	if !r.allow() {
		http.Error(w, "request budget exhausted", http.StatusTooManyRequests)
		return
	}
	select {
	case r.slots <- struct{}{}:
		defer func() { <-r.slots }()
	default:
		http.Error(w, "request capacity exhausted", http.StatusTooManyRequests)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, req.Body, 64<<10))
	var call rpcRequest
	if err != nil || !strictRPCJSON(raw) || json.Unmarshal(raw, &call) != nil || call.JSONRPC != "2.0" || len(call.ID) == 0 || len(call.ID) > 66 || string(call.ID) == "null" || len(call.Params) > 8 || len(call.Method) > 64 {
		r.respond(w, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32600, "invalid bounded request"}})
		return
	}
	var id any
	ids := json.NewDecoder(bytes.NewReader(call.ID))
	ids.UseNumber()
	if ids.Decode(&id) != nil {
		return
	}
	switch id.(type) {
	case string, json.Number:
	default:
		r.respond(w, rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32600, "invalid request id"}})
		return
	}
	ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
	defer cancel()
	result, failure := r.dispatch(ctx, call)
	r.respond(w, rpcResponse{JSONRPC: "2.0", ID: call.ID, Result: result, Error: failure})
}

func (r *TestnetRPC) respond(w http.ResponseWriter, result rpcResponse) {
	// Encode to a bounded detached buffer before writing any successful result.
	envelope := map[string]any{"jsonrpc": "2.0", "id": result.ID}
	if result.Error != nil {
		envelope["error"] = result.Error
	} else {
		envelope["result"] = result.Result
	}
	raw, err := json.Marshal(envelope)
	if err != nil || len(raw) > 2<<20 {
		http.Error(w, "response unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

func strictRPCJSON(raw []byte) bool {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var visit func(int) bool
	visit = func(depth int) bool {
		if depth > 16 {
			return false
		}
		token, err := d.Token()
		if err != nil {
			return false
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					k, e := d.Token()
					s, ok := k.(string)
					if e != nil || !ok || seen[s] {
						return false
					}
					seen[s] = true
					if !visit(depth + 1) {
						return false
					}
				}
			case '[':
				for d.More() {
					if !visit(depth + 1) {
						return false
					}
				}
			default:
				return false
			}
			close, e := d.Token()
			if e != nil || delim == '{' && close != json.Delim('}') || delim == '[' && close != json.Delim(']') {
				return false
			}
		}
		return true
	}
	if !visit(0) {
		return false
	}
	_, err := d.Token()
	if err != io.EOF {
		return false
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) != nil || obj == nil {
		return false
	}
	for key := range obj {
		if key != "jsonrpc" && key != "id" && key != "method" && key != "params" {
			return false
		}
	}
	return true
}

func rpcString(raw json.RawMessage) (string, bool) {
	var s string
	err := json.Unmarshal(raw, &s)
	return s, err == nil
}
func rpcHeight(raw json.RawMessage) (int64, bool) {
	s, ok := rpcString(raw)
	if !ok {
		return 0, false
	}
	if s == "latest" {
		return 0, true
	}
	h, err := strconv.ParseInt(s, 10, 64)
	return h, err == nil && h > 0 && strconv.FormatInt(h, 10) == s
}

func (r *TestnetRPC) dispatch(ctx context.Context, call rpcRequest) (result any, failure *rpcError) {
	defer func() {
		if recover() != nil {
			result = nil
			failure = &rpcError{-32000, "synthetic node result unavailable"}
		}
	}()
	bad := &rpcError{-32602, "invalid parameters"}
	unavailable := &rpcError{-32000, "synthetic node result unavailable"}
	switch call.Method {
	case "god_blocks", "god_block", "god_transactionDetails":
		return r.explorer(ctx, call)
	case "god_liveness":
		if len(call.Params) != 0 {
			return nil, bad
		}
		a := r.n.app
		a.mu.Lock()
		height, usable, pending := a.base.LastBlockHeight(), a.usable() && a.initialized, a.finalized
		a.mu.Unlock()
		status, err := r.client.Status(ctx)
		if err != nil {
			return nil, unavailable
		}
		state, err := r.client.ConsensusState(ctx)
		if err != nil || state == nil {
			return nil, unavailable
		}
		var round struct {
			Progress string `json:"height/round/step"`
		}
		if json.Unmarshal(state.RoundState, &round) != nil || len(round.Progress) > 64 {
			return nil, unavailable
		}
		return map[string]any{"running": r.n.node.IsRunning(), "applicationUsable": usable, "applicationHeight": strconv.FormatInt(height, 10), "blockStoreHeight": strconv.FormatInt(r.n.node.BlockStore().Height(), 10), "commitPending": pending, "connectedPeers": r.n.node.Switch().Peers().Size(), "catchingUp": status.SyncInfo.CatchingUp, "consensusProgress": round.Progress, "synthetic": true, "realAssets": false}, nil
	case "god_delegation", "god_unbonding":
		if len(call.Params) < 2 || len(call.Params) > 3 {
			return nil, bad
		}
		owner, ok := rpcString(call.Params[0])
		validator, valid := rpcString(call.Params[1])
		if !ok || !valid {
			return nil, bad
		}
		height := int64(0)
		if len(call.Params) == 3 {
			height, ok = rpcHeight(call.Params[2])
			if !ok {
				return nil, bad
			}
		}
		if call.Method == "god_unbonding" {
			v, err := r.n.app.QueryUnbonding(owner, validator, height)
			if err != nil {
				return nil, bad
			}
			return v, nil
		}
		v, err := r.n.app.QueryDelegation(owner, validator, height)
		if err != nil {
			return nil, bad
		}
		return v, nil
	case "god_network":
		if len(call.Params) > 1 {
			return nil, bad
		}
		h := int64(0)
		if len(call.Params) == 1 {
			var ok bool
			h, ok = rpcHeight(call.Params[0])
			if !ok {
				return nil, bad
			}
		}
		v, err := r.n.app.QueryNetwork(h)
		if err != nil {
			return nil, unavailable
		}
		return v, nil
	case "god_account":
		if len(call.Params) < 1 || len(call.Params) > 2 {
			return nil, bad
		}
		address, ok := rpcString(call.Params[0])
		if !ok {
			return nil, bad
		}
		h := int64(0)
		if len(call.Params) == 2 {
			h, ok = rpcHeight(call.Params[1])
			if !ok {
				return nil, bad
			}
		}
		v, err := r.n.app.QueryAccount(address, h)
		if err != nil {
			return nil, bad
		}
		return v, nil
	case "god_submitTransaction":
		if len(call.Params) != 1 {
			return nil, bad
		}
		text, ok := rpcString(call.Params[0])
		if !ok || !strings.HasPrefix(text, "0x") || len(text) > 2+r.n.app.config.Policy.MaxTxBytes*2 {
			return nil, bad
		}
		wire, err := hex.DecodeString(text[2:])
		if err != nil || len(wire) == 0 {
			return nil, bad
		}
		err = r.n.Submit(ctx, wire)
		if err != nil {
			return nil, submissionFailure(err)
		}
		return map[string]any{"hash": "0x" + hex.EncodeToString(cmttypes.Tx(wire).Hash()), "admitted": true, "included": false, "synthetic": true, "realAssets": false}, nil
	case "god_transaction":
		if len(call.Params) != 1 {
			return nil, bad
		}
		text, ok := rpcString(call.Params[0])
		if !ok || len(text) != 66 || !strings.HasPrefix(text, "0x") {
			return nil, bad
		}
		hash, err := hex.DecodeString(text[2:])
		if err != nil {
			return nil, bad
		}
		x, err := r.client.Tx(ctx, hash, false)
		if err != nil || x == nil {
			return nil, nil
		}
		v, err := r.n.app.QueryNetwork(0)
		if err != nil || x.Height > v.Commit.Height {
			return nil, nil
		}
		return map[string]any{"hash": text, "height": strconv.FormatInt(x.Height, 10), "index": strconv.FormatUint(uint64(x.Index), 10), "code": x.TxResult.Code,
			"gasWanted": strconv.FormatInt(x.TxResult.GasWanted, 10), "gasUsed": strconv.FormatInt(x.TxResult.GasUsed, 10), "included": true, "sdkSuccessful": x.TxResult.Code == 0, "synthetic": true, "realAssets": false}, nil
	default:
		return r.ethereum(ctx, call)
	}
}

func submissionFailure(err error) *rpcError {
	if errors.Is(err, ErrCommitPending) {
		return &rpcError{-32001, "retry after application commit; transaction not admitted"}
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// Cancellation does not retract an admission attempt already in flight.
		return &rpcError{-32005, "submission outcome unknown; check transaction hash before retrying"}
	}
	return &rpcError{-32002, "signed transaction not admitted"}
}

// ServeTestnetRPC does not expose the engine RPC, gRPC, metrics or debug routes.
func ServeTestnetRPC(ctx context.Context, n *LocalNode, listener net.Listener, options RPCOptions) error {
	if ctx == nil || listener == nil {
		return ErrConfig
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || (!address.IP.IsLoopback() && !options.AllowNetwork) {
		return ErrConfig
	}
	handler, err := NewTestnetRPC(n, options)
	if err != nil {
		return err
	}
	listener = &rpcListener{Listener: listener, slots: make(chan struct{}, 64), closed: make(chan struct{})}
	server := &http.Server{Handler: handler, ErrorLog: log.New(io.Discard, "", 0), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 8 << 10,
		BaseContext: func(net.Listener) context.Context { return ctx }}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return ErrLifecycle
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if server.Shutdown(shutdown) != nil {
			_ = server.Close()
			<-done
			return ErrLifecycle
		}
		<-done
		return nil
	}
}

// Bound open HTTP connections as well as parsed requests. Close unblocks both
// an underlying Accept and a capacity wait; each connection releases once.
type rpcListener struct {
	net.Listener
	slots  chan struct{}
	closed chan struct{}
	once   sync.Once
}

func (l *rpcListener) Accept() (net.Conn, error) {
	select {
	case l.slots <- struct{}{}:
	case <-l.closed:
		return nil, net.ErrClosed
	}
	c, err := l.Listener.Accept()
	if err != nil {
		<-l.slots
		return nil, err
	}
	return &rpcConn{Conn: c, release: func() { <-l.slots }}, nil
}
func (l *rpcListener) Close() error {
	l.once.Do(func() { close(l.closed) })
	return l.Listener.Close()
}

type rpcConn struct {
	net.Conn
	once    sync.Once
	release func()
}

func (c *rpcConn) Close() error { err := c.Conn.Close(); c.once.Do(c.release); return err }
