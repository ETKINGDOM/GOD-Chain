package godrh

import (
	"bytes"
	"context"
	"math/big"
)

// HTTPTokenSource exposes only the configured token's code and two metadata
// views, plus chain/checkpoint reads. Receipt, log, storage, debug and write
// methods are unavailable. Missing custody and signers remain missing.
type HTTPTokenSource struct {
	source *HTTPSource
	token  [20]byte
}

func (*HTTPTokenSource) String() string     { return "GOD Chain RH read-only token source (redacted)" }
func (s *HTTPTokenSource) GoString() string { return s.String() }
func (*HTTPTokenSource) MarshalJSON() ([]byte, error) {
	return []byte(`{"readOnly":true,"redacted":true}`), nil
}

func NewHTTPTokenSource(c Config) (*HTTPTokenSource, error) {
	if !c.TokenReport().ConnectionReady {
		return nil, ErrConfig
	}
	source, err := newHTTPSource(c.private.RPCEndpoint)
	if err != nil {
		return nil, err
	}
	source.tokenReads = true
	token, _ := address(c.private.TokenContract)
	return &HTTPTokenSource{source: source, token: token}, nil
}

func (s *HTTPTokenSource) base() *HTTPSource {
	if s == nil {
		return nil
	}
	return s.source
}
func (s *HTTPTokenSource) Close() { s.base().Close() }
func (s *HTTPTokenSource) ChainID(ctx context.Context) (*big.Int, error) {
	return s.base().ChainID(ctx)
}
func (s *HTTPTokenSource) FinalizedBlock(ctx context.Context) (Block, error) {
	return s.base().FinalizedBlock(ctx)
}
func (s *HTTPTokenSource) Block(ctx context.Context, height uint64) (Block, error) {
	return s.base().Block(ctx, height)
}
func (s *HTTPTokenSource) Code(ctx context.Context, target [20]byte, block [32]byte) ([]byte, error) {
	if s == nil || target != s.token {
		return nil, ErrSource
	}
	return s.base().Code(ctx, target, block)
}
func (s *HTTPTokenSource) Call(ctx context.Context, target [20]byte, input []byte, block [32]byte) ([]byte, error) {
	if s == nil || target != s.token || (!bytes.Equal(input, selector("decimals()")) && !bytes.Equal(input, selector("totalSupply()"))) {
		return nil, ErrSource
	}
	return s.base().Call(ctx, target, input, block)
}

var _ Source = (*HTTPTokenSource)(nil)
