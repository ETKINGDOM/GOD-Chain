package godnode

import (
	"context"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	cmtdb "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtconfig "github.com/cometbft/cometbft/config"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/mempool"
	cmtNode "github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/proxy"
	cmttypes "github.com/cometbft/cometbft/types"
)

// LocalOptions accepts only explicit loopback peers and a caller-owned private
// fixture directory. There is no public peer discovery, remote ABCI/RPC,
// telemetry, transaction indexer, built-in signer or mainnet start command.
type LocalOptions struct {
	Directory     string
	Listen        string
	Peers         []string
	BlockInterval time.Duration
	Genesis       *cmttypes.GenesisDoc
	Signer        cmttypes.PrivValidator
	NodeKey       *p2p.NodeKey
	// Persistence is opt-in and still synthetic, private and loopback-only.
	Persistent bool
}

func loopbackEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "tcp" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil || port == "" || port == "0" {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type LocalNode struct {
	app  *App
	node *cmtNode.Node
}

func StartLocal(ctx context.Context, a *App, o LocalOptions) (*LocalNode, error) {
	if a == nil || !a.config.Prototype || !filepath.IsAbs(o.Directory) || filepath.Clean(o.Directory) == string(filepath.Separator) ||
		!loopbackEndpoint(o.Listen) || o.BlockInterval < 100*time.Millisecond || o.BlockInterval > 6*time.Second || o.Genesis == nil ||
		o.Signer == nil || o.NodeKey == nil || o.NodeKey.PrivKey == nil || len(o.Peers) > 32 {
		return nil, ErrConfig
	}
	for _, peer := range o.Peers {
		at := strings.IndexByte(peer, '@')
		if at != 40 || !loopbackEndpoint("tcp://"+peer[at+1:]) {
			return nil, ErrConfig
		}
	}
	if o.Genesis.ChainID != a.config.ChainID || o.Genesis.ValidateAndComplete() != nil {
		return nil, ErrGenesis
	}
	a.mu.Lock()
	if !a.usable() || a.finalized || a.engineAssigned {
		a.mu.Unlock()
		return nil, ErrLifecycle
	}
	if o.Persistent {
		if err := bindLocalStorage(a, o); err != nil {
			a.mu.Unlock()
			return nil, err
		}
	}
	// Never attach two consensus engines to the same execution state, even
	// after Stop. A restart must use a fresh process and committed database.
	a.engineAssigned, a.engineRunning = true, true
	a.mu.Unlock()
	success := false
	defer func() {
		if !success {
			a.mu.Lock()
			a.engineRunning, a.failed = false, true
			a.mu.Unlock()
		}
	}()
	config := cmtconfig.DefaultConfig().SetRoot(o.Directory)
	config.Moniker = "GodCometBFT local prototype"
	config.RPC.ListenAddress = ""
	config.RPC.GRPCListenAddress = ""
	config.RPC.PprofListenAddress = ""
	config.P2P.ListenAddress = o.Listen
	config.P2P.ExternalAddress = ""
	config.P2P.PersistentPeers = strings.Join(o.Peers, ",")
	config.P2P.Seeds = ""
	config.P2P.PexReactor = false
	config.P2P.AddrBookStrict = false
	config.P2P.AllowDuplicateIP = true
	config.P2P.MaxNumInboundPeers = 32
	config.P2P.MaxNumOutboundPeers = 32
	config.Consensus.TimeoutCommit = o.BlockInterval
	config.Consensus.TimeoutPropose = 2 * time.Second
	config.Consensus.TimeoutPrevote = time.Second
	config.Consensus.TimeoutPrecommit = time.Second
	config.Instrumentation.Prometheus = false
	config.TxIndex.Indexer = "null"
	config.Mempool.MaxTxBytes = a.config.Policy.MaxTxBytes
	config.Mempool.MaxTxsBytes = a.config.MaxBlockBytes * 4
	config.Mempool.Size = a.config.MaxBlockTxs * 4
	if config.ValidateBasic() != nil {
		return nil, ErrConfig
	}
	provider := func(*cmtconfig.DBContext) (cmtdb.DB, error) { return cmtdb.NewMemDB(), nil }
	if o.Persistent {
		config.DBBackend = string(cmtdb.GoLevelDBBackend)
		provider = cmtconfig.DefaultDBProvider
	}
	n, err := cmtNode.NewNodeWithContext(ctx, config, o.Signer, o.NodeKey, proxy.NewLocalClientCreator(application{a}),
		func() (*cmttypes.GenesisDoc, error) { return o.Genesis, nil },
		provider, cmtNode.DefaultMetricsProvider(config.Instrumentation), cmtlog.NewNopLogger())
	if err != nil {
		return nil, err
	}
	if err := n.Start(); err != nil {
		n.Stop()
		return nil, err
	}
	success = true
	return &LocalNode{a, n}, nil
}

func (n *LocalNode) Submit(ctx context.Context, wire []byte) error {
	if n == nil || n.node == nil || !n.node.IsRunning() || len(wire) == 0 || len(wire) > n.app.config.Policy.MaxTxBytes {
		return ErrLifecycle
	}
	result := make(chan uint32, 1)
	err := n.node.Mempool().CheckTx(cmttypes.Tx(wire), func(res *abci.ResponseCheckTx) { result <- res.Code }, mempool.TxInfo{})
	if err != nil {
		return err
	}
	select {
	case code := <-result:
		if code != 0 {
			return ErrBlock
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (n *LocalNode) Stop() error {
	if n == nil || n.node == nil {
		return ErrLifecycle
	}
	err := n.node.Stop()
	n.node.Wait()
	if err == nil {
		n.app.mu.Lock()
		n.app.engineRunning = false
		n.app.mu.Unlock()
	}
	return err
}
