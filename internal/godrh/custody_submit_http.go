package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

const maxCustodyAdmissionBytes = 4096

var ErrCustodySubmitHTTP = errors.New("RH isolated custody HTTP submission rejected; production broadcast remains disabled")

// A separate, one-use write transport, never a capability of HTTPSource. Its
// only constructor requires pinned private simulation inputs and a literal
// loopback endpoint. Locality is NOT proof that a node or its assets are safe;
// acceptance must use an explicitly isolated synthetic source. No public route,
// production constructor, signer, provider default or polling loop is installed.
type httpCustodySimulationSubmitter struct {
	endpoint string
	client   *http.Client
	wirePin  [32]byte
	txHash   [32]byte
	used     atomic.Bool
	closed   atomic.Bool
}

func (*httpCustodySimulationSubmitter) String() string {
	return "RH isolated custody HTTP submission (redacted)"
}
func (s *httpCustodySimulationSubmitter) GoString() string { return s.String() }
func (*httpCustodySimulationSubmitter) MarshalJSON() ([]byte, error) {
	return []byte(`{"simulationOnly":true,"redacted":true}`), nil
}

func custodySimulationHTTPEndpoint(text string) bool {
	if !endpoint(text, "simulation") {
		return false
	}
	u, err := url.Parse(text)
	if err != nil {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	return ip != nil && ip.IsLoopback()
}

func newHTTPCustodySimulationSubmitter(ctx context.Context, i CustodyTransactionInputs) (*httpCustodySimulationSubmitter, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, ErrCustodySubmitHTTP
	}
	checked, err := loadCustodyTransaction(i)
	if err != nil || !checked.report.SimulationConfiguration {
		return nil, ErrCustodySubmitHTTP
	}
	raw, err := readCustodyPrivateSnapshot(i.CallInputs.ConfigurationPath, maxConfigBytes, i.CallInputs.ConfigurationSHA256)
	if err != nil {
		return nil, ErrCustodySubmitHTTP
	}
	c, err := Parse(raw)
	if err != nil || c.private.Mode != "simulation" || !c.Report().ConfigurationReady ||
		!custodySimulationHTTPEndpoint(c.private.RPCEndpoint) || ctx.Err() != nil {
		return nil, ErrCustodySubmitHTTP
	}
	// Do not clone ambient DefaultTransport: it may install a proxy, custom
	// dialer, TLS bypass or alternative protocol. HTTP/1, a fresh connection,
	// a non-replayable nonempty POST and no idempotency header disable the pinned
	// Go transport's transparent request retries. Redirects are also refused.
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: -1}).DialContext,
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12},
		TLSNextProto:           map[string]func(string, *tls.Conn) http.RoundTripper{},
		ForceAttemptHTTP2:      false,
		DisableKeepAlives:      true,
		DisableCompression:     true,
		MaxConnsPerHost:        1,
		MaxResponseHeaderBytes: 16 << 10,
		ResponseHeaderTimeout:  10 * time.Second,
		TLSHandshakeTimeout:    5 * time.Second,
	}
	return &httpCustodySimulationSubmitter{
		endpoint: c.private.RPCEndpoint, wirePin: i.TransactionSHA256, txHash: checked.hash,
		client: &http.Client{Transport: transport, Timeout: 10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return ErrCustodySubmitHTTP }},
	}, nil
}

func (s *httpCustodySimulationSubmitter) Close() {
	if s != nil {
		s.closed.Store(true)
		if s.client != nil {
			s.client.CloseIdleConnections()
		}
	}
}

// The book must retain dispatch UNKNOWN before invoking this method. Consuming
// the object itself is only additional process-local protection, never durable
// transaction identity or permission to recreate an attempt after restart.
func (s *httpCustodySimulationSubmitter) SubmitLocalCustody(ctx context.Context, wire []byte) error {
	if s == nil || s.client == nil || ctx == nil || ctx.Err() != nil || s.closed.Load() || !s.used.CompareAndSwap(false, true) {
		return ErrCustodySubmitHTTP
	}
	if len(wire) == 0 || len(wire) > MaxCustodyTransactionBytes || sha256.Sum256(wire) != s.wirePin {
		return ErrCustodySubmitHTTP
	}
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	body, err := json.Marshal(struct {
		Version string   `json:"jsonrpc"`
		ID      uint64   `json:"id"`
		Method  string   `json:"method"`
		Params  []string `json:"params"`
	}{"2.0", 1, "eth_sendRawTransaction", []string{"0x" + hex.EncodeToString(wire)}})
	if err != nil {
		return ErrCustodySubmitHTTP
	}
	// NopCloser intentionally hides bytes.Reader from NewRequest: GetBody is
	// nil. Set a nonzero content length; never add an idempotency/replay header.
	request, err := http.NewRequestWithContext(sendCtx, http.MethodPost, s.endpoint, io.NopCloser(bytes.NewReader(body)))
	if err != nil {
		return ErrCustodySubmitHTTP
	}
	request.ContentLength = int64(len(body))
	request.GetBody = nil
	request.Close = true
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return ErrCustodySubmitHTTP
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > maxCustodyAdmissionBytes ||
		(response.Header.Get("Content-Encoding") != "" && response.Header.Get("Content-Encoding") != "identity") {
		return ErrCustodySubmitHTTP
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxCustodyAdmissionBytes+1))
	if err != nil || sendCtx.Err() != nil || checkCustodyAdmission(raw, s.txHash) != nil {
		return ErrCustodySubmitHTTP
	}
	return nil // Exact hash acknowledgment only; not execution, finality or payment.
}

func checkCustodyAdmission(raw []byte, expected [32]byte) error {
	if len(raw) == 0 || len(raw) > maxCustodyAdmissionBytes || !utf8.Valid(raw) || expected == [32]byte{} {
		return ErrCustodySubmitHTTP
	}
	fields, err := custodyObject(raw, []string{"jsonrpc", "id", "result"}, nil)
	if err != nil {
		return ErrCustodySubmitHTTP
	}
	var version string
	var id uint64
	if json.Unmarshal(fields["jsonrpc"], &version) != nil || version != "2.0" ||
		json.Unmarshal(fields["id"], &id) != nil || id != 1 {
		return ErrCustodySubmitHTTP
	}
	value, err := custodyText(fields["result"])
	if err != nil || value != "0x"+hex.EncodeToString(expected[:]) {
		return ErrCustodySubmitHTTP
	}
	return nil
}

// SubmitHTTPForSimulation composes one pinned loopback HTTP handoff with the
// SAME durable nonce/Gas book. Preparation makes no network call. The book
// independently rechecks original inputs and saves/readbacks UNKNOWN before
// HTTP. A matching returned TX hash is admission at most: even success leaves
// dispatch unknown and never releases nonce/Gas or permits redispatch. Recovery
// remains the existing read-only RecoverForSimulation, with no write fallback.
func (b *CustodyNonceBook) SubmitHTTPForSimulation(ctx context.Context, i CustodyTransactionInputs) (CustodySubmissionReport, error) {
	s, err := newHTTPCustodySimulationSubmitter(ctx, i)
	if err != nil {
		return CustodySubmissionReport{SimulationOnly: true}, ErrCustodySubmitHTTP
	}
	defer s.Close()
	return b.SubmitForSimulation(ctx, i, s)
}
