package godrh

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const (
	maxCodeBytes = 128 << 10
	maxRPCBytes  = 512 << 10
)

var ErrSource = errors.New("RH read-only source request rejected")

// HTTPSource has only bounded reads. It has no signer, transaction submission,
// provider default, redirect following, ambient proxy or logging facility.
type HTTPSource struct {
	endpoint string
	client   *http.Client
	nextID   atomic.Uint64
}

func (*HTTPSource) String() string     { return "GOD Chain RH read-only source (redacted)" }
func (s *HTTPSource) GoString() string { return s.String() }
func (*HTTPSource) MarshalJSON() ([]byte, error) {
	return []byte(`{"readOnly":true,"redacted":true}`), nil
}

func NewHTTPSource(c Config) (*HTTPSource, error) {
	if !c.Report().ConfigurationReady {
		return nil, ErrConfig
	}
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok || base == nil {
		return nil, ErrSource
	}
	transport := base.Clone()
	transport.Proxy = nil
	transport.MaxResponseHeaderBytes = 16 << 10
	transport.ResponseHeaderTimeout = 10 * time.Second
	return &HTTPSource{endpoint: c.private.RPCEndpoint, client: &http.Client{
		Transport: transport, Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return ErrSource },
	}}, nil
}

func (s *HTTPSource) Close() {
	if s != nil && s.client != nil {
		s.client.CloseIdleConnections()
	}
}

func (s *HTTPSource) rpc(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if s == nil || s.client == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrSource
	}
	switch method {
	case "eth_chainId", "eth_getBlockByNumber", "eth_getCode", "eth_call", "eth_getTransactionReceipt", "eth_getLogs":
	default:
		return nil, ErrSource
	}
	id := s.nextID.Add(1)
	if id == 0 {
		return nil, ErrSource
	}
	body, err := json.Marshal(struct {
		Version string `json:"jsonrpc"`
		ID      uint64 `json:"id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{"2.0", id, method, params})
	if err != nil {
		return nil, ErrSource
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, ErrSource
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, ErrSource
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.ContentLength > maxRPCBytes {
		return nil, ErrSource
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxRPCBytes+1))
	if err != nil || len(raw) > maxRPCBytes {
		return nil, ErrSource
	}
	// Reject ambiguous JSON-RPC envelopes rather than last-key-wins results.
	decoder := json.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return nil, ErrSource
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok || fields[name] != nil || (name != "jsonrpc" && name != "id" && name != "result" && name != "error") {
			return nil, ErrSource
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil, ErrSource
		}
		fields[name] = value
	}
	last, err := decoder.Token()
	if err != nil || last != json.Delim('}') {
		return nil, ErrSource
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, ErrSource
	}
	var version string
	var returnedID uint64
	if json.Unmarshal(fields["jsonrpc"], &version) != nil || version != "2.0" || json.Unmarshal(fields["id"], &returnedID) != nil || returnedID != id ||
		fields["error"] != nil || fields["result"] == nil || bytes.Equal(bytes.TrimSpace(fields["result"]), []byte("null")) {
		return nil, ErrSource
	}
	return fields["result"], nil
}

func quantity(text string) (*big.Int, error) {
	if len(text) < 3 || len(text) > 66 || !strings.HasPrefix(text, "0x") || len(text) > 3 && text[2] == '0' {
		return nil, ErrSource
	}
	for _, ch := range text[2:] {
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f') {
			return nil, ErrSource
		}
	}
	n, ok := new(big.Int).SetString(text[2:], 16)
	if !ok {
		return nil, ErrSource
	}
	return n, nil
}

func (s *HTTPSource) ChainID(ctx context.Context) (*big.Int, error) {
	raw, err := s.rpc(ctx, "eth_chainId", []any{})
	var text string
	if err != nil || json.Unmarshal(raw, &text) != nil {
		return nil, ErrSource
	}
	n, err := quantity(text)
	if err != nil || n.Sign() <= 0 {
		return nil, ErrSource
	}
	return n, nil
}

func (s *HTTPSource) FinalizedBlock(ctx context.Context) (Block, error) {
	return s.block(ctx, "finalized")
}
func (s *HTTPSource) Block(ctx context.Context, height uint64) (Block, error) {
	block, err := s.block(ctx, "0x"+strconv.FormatUint(height, 16))
	if err != nil || height == 0 || block.Height != height {
		return Block{}, ErrSource
	}
	return block, nil
}

func (s *HTTPSource) block(ctx context.Context, tag string) (Block, error) {
	raw, err := s.rpc(ctx, "eth_getBlockByNumber", []any{tag, false})
	var wire struct {
		Number string `json:"number"`
		Hash   string `json:"hash"`
	}
	// Block objects may contain additional standard fields, but selected fields
	// must be unique and exactly cased. Do not accept ambiguous checkpoint data.
	decoder := json.NewDecoder(bytes.NewReader(raw))
	first, tokenErr := decoder.Token()
	if err != nil || tokenErr != nil || first != json.Delim('{') {
		return Block{}, ErrSource
	}
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok || seen[name] || (strings.EqualFold(name, "number") && name != "number") || (strings.EqualFold(name, "hash") && name != "hash") {
			return Block{}, ErrSource
		}
		seen[name] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return Block{}, ErrSource
		}
		switch name {
		case "number":
			err = json.Unmarshal(value, &wire.Number)
		case "hash":
			err = json.Unmarshal(value, &wire.Hash)
		}
		if err != nil {
			return Block{}, ErrSource
		}
	}
	last, err := decoder.Token()
	if err != nil || last != json.Delim('}') {
		return Block{}, ErrSource
	}
	if _, err := decoder.Token(); err != io.EOF {
		return Block{}, ErrSource
	}
	number, numberErr := quantity(wire.Number)
	value, hashErr := hash(wire.Hash)
	if numberErr != nil || hashErr != nil || number.Sign() <= 0 || !number.IsUint64() {
		return Block{}, ErrSource
	}
	return Block{number.Uint64(), value}, nil
}

func blockReference(value [32]byte) map[string]any {
	return map[string]any{"blockHash": "0x" + hex.EncodeToString(value[:]), "requireCanonical": true}
}

func (s *HTTPSource) hexRead(ctx context.Context, method string, params any, maxBytes int) ([]byte, error) {
	raw, err := s.rpc(ctx, method, params)
	var text string
	if err != nil || json.Unmarshal(raw, &text) != nil || len(text) < 2 || len(text) > 2+maxBytes*2 || !strings.HasPrefix(text, "0x") {
		return nil, ErrSource
	}
	value, err := hex.DecodeString(text[2:])
	if err != nil {
		return nil, ErrSource
	}
	return value, nil
}

func (s *HTTPSource) Code(ctx context.Context, target [20]byte, block [32]byte) ([]byte, error) {
	if target == [20]byte{} || block == [32]byte{} {
		return nil, ErrSource
	}
	return s.hexRead(ctx, "eth_getCode", []any{"0x" + hex.EncodeToString(target[:]), blockReference(block)}, maxCodeBytes)
}

func (s *HTTPSource) Call(ctx context.Context, target [20]byte, input []byte, block [32]byte) ([]byte, error) {
	if target == [20]byte{} || block == [32]byte{} || (len(input) != 4 && len(input) != 36) {
		return nil, ErrSource
	}
	return s.hexRead(ctx, "eth_call", []any{map[string]string{
		"to": "0x" + hex.EncodeToString(target[:]), "data": "0x" + hex.EncodeToString(input),
	}, blockReference(block)}, 128)
}

var _ Source = (*HTTPSource)(nil)
