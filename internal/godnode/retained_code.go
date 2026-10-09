package godnode

import (
	"github.com/ethereum/go-ethereum/common"
)

// retainedCode must run under a.mu. It reads only code from one explicit
// retained committed application version. It is not historical account/call
// support, a consensus/header proof, or a fallback to the latest code.
func (a *App) retainedCode(address common.Address, height int64) ([]byte, error) {
	if !a.usable() || !a.initialized || height < 1 || height > a.base.LastBlockHeight() {
		return nil, ErrQueryHeight
	}
	id := a.base.LastCommitID()
	if id.Version != a.base.LastBlockHeight() || len(id.Hash) != 32 {
		return nil, ErrQueryState
	}
	q, err := a.base.CreateQueryContext(height, false)
	if err != nil {
		return nil, ErrQueryState
	}
	m, err := a.metadata(q)
	if err != nil || m.Height != height {
		return nil, ErrQueryState
	}
	q = q.WithChainID(a.config.ChainID).WithBlockHeight(height).WithBlockTime(m.Time.UTC())
	code := a.evm.GetCode(q, a.evm.GetCodeHash(q, address))
	if len(code) > 24576 {
		return nil, ErrQueryState
	}
	return append([]byte(nil), code...), nil
}
