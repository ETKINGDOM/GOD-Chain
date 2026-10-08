package godhistory

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Source must provide exactly these two existing read-only methods. It cannot
// start a node, sign, submit, access key files or authorize a real-asset mode.
type Source interface {
	Read(context.Context, string, []string) (json.RawMessage, error)
}
type RPC struct {
	endpoint string
	client   *http.Client
	mu       sync.Mutex
	last     time.Time
}

func NewRPC(endpoint string) (*RPC, error) {
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "http" || u.User != nil || u.Path != "/" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" {
		return nil, ErrHistory
	}
	h, p, e := net.SplitHostPort(u.Host)
	port, f := strconv.Atoi(p)
	if e != nil || f != nil || !net.ParseIP(h).IsLoopback() || port < 1024 || port > 65535 || strconv.Itoa(port) != p {
		return nil, ErrHistory
	}
	t := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: time.Second}).DialContext, DisableCompression: true, MaxConnsPerHost: 1, MaxIdleConnsPerHost: 1, ResponseHeaderTimeout: 3 * time.Second, IdleConnTimeout: 15 * time.Second, MaxResponseHeaderBytes: 8192}
	return &RPC{endpoint: endpoint, client: &http.Client{Transport: t, Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrHistory }}}, nil
}
func (r *RPC) Close() { r.client.CloseIdleConnections() }
func (r *RPC) Read(ctx context.Context, method string, params []string) (json.RawMessage, error) {
	if method != "god_network" && method != "god_block" || method == "god_network" && len(params) != 0 || method == "god_block" && len(params) != 3 {
		return nil, ErrHistory
	}
	if method == "god_block" {
		if _, e := integer(params[0], 1, 9223372036854775807); e != nil {
			return nil, ErrHistory
		}
		if _, e := integer(params[1], 0, BlockLimit); e != nil || params[2] != "20" {
			return nil, ErrHistory
		}
	}
	// Background consumers must not burst through the shared observer budget.
	// This is pacing, not a retry; a denied/unknown read returns unavailable.
	r.mu.Lock()
	defer r.mu.Unlock()
	if delay := time.Until(r.last.Add(250 * time.Millisecond)); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return nil, ErrHistory
		}
	}
	if ctx.Err() != nil {
		return nil, ErrHistory
	}
	r.last = time.Now()
	if params == nil {
		params = []string{}
	}
	data, e := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if e != nil {
		return nil, ErrHistory
	}
	q, e := http.NewRequestWithContext(ctx, "POST", r.endpoint, bytes.NewReader(data))
	if e != nil {
		return nil, ErrHistory
	}
	q.Header.Set("Content-Type", "application/json")
	response, e := r.client.Do(q)
	if e != nil {
		return nil, ErrHistory
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, ErrHistory
	}
	raw, e := io.ReadAll(io.LimitReader(response.Body, maxReply+1))
	if e != nil || !validJSON(raw) {
		return nil, ErrHistory
	}
	var envelope struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      int             `json:"id"`
		Result  json.RawMessage `json:"result"`
	}
	if exact(raw, &envelope) != nil || envelope.JSONRPC != "2.0" || envelope.ID != 1 || len(envelope.Result) == 0 || string(envelope.Result) == "null" {
		return nil, ErrHistory
	}
	return envelope.Result, nil
}
