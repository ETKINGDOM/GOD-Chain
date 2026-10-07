package godnode

import (
	"context"
	"encoding/hex"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cmtdb "github.com/cometbft/cometbft-db"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtconfig "github.com/cometbft/cometbft/config"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/mempool"
	cmtNode "github.com/cometbft/cometbft/node"
	"github.com/cometbft/cometbft/p2p"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
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
	// Observer nodes may replicate and serve RPC but can never sign consensus.
	Observer bool
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
	return startNode(ctx, a, o, false)
}

// StartTestnet enables explicitly configured synthetic network peers, not
// mainnet or RH assets. It requires persistent storage and forbids bridge
// genesis/routes. The caller must secure the host and retain signing progress.
// No RPC or discovery listener is enabled by this adapter.
func StartTestnet(ctx context.Context, a *App, o LocalOptions) (*LocalNode, error) {
	if a == nil || !o.Persistent || a.config.BridgeGenesis != nil || a.config.BridgeApprovalGas != 0 {
		return nil, ErrConfig
	}
	return startNode(ctx, a, o, true)
}

func testnetEndpoint(endpoint string, listen bool) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "tcp" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	host, port, err := net.SplitHostPort(u.Host)
	n, parseErr := strconv.Atoi(port)
	ip := net.ParseIP(host)
	return err == nil && parseErr == nil && n > 0 && n <= 65535 && strconv.Itoa(n) == port && ip != nil &&
		!ip.IsMulticast() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && (listen || !ip.IsUnspecified())
}

func startNode(ctx context.Context, a *App, o LocalOptions, network bool) (*LocalNode, error) {
	endpointOK := loopbackEndpoint(o.Listen)
	if network {
		endpointOK = testnetEndpoint(o.Listen, true)
	}
	if ctx == nil || a == nil || !a.config.Prototype || !filepath.IsAbs(o.Directory) || filepath.Clean(o.Directory) == string(filepath.Separator) ||
		!endpointOK || o.BlockInterval < 100*time.Millisecond || o.BlockInterval > 6*time.Second || o.Genesis == nil ||
		o.Signer == nil || o.NodeKey == nil || o.NodeKey.PrivKey == nil || len(o.Peers) > 32 {
		return nil, ErrConfig
	}
	seen := map[string]bool{}
	for _, peer := range o.Peers {
		at := strings.IndexByte(peer, '@')
		if at != 40 || seen[peer[:at]] {
			return nil, ErrConfig
		}
		id, err := hex.DecodeString(peer[:at])
		valid := loopbackEndpoint("tcp://" + peer[at+1:])
		if network {
			valid = testnetEndpoint("tcp://"+peer[at+1:], false)
		}
		if err != nil || len(id) != 20 || !valid || peer[:at] == string(o.NodeKey.ID()) {
			return nil, ErrConfig
		}
		seen[peer[:at]] = true
	}
	if o.Genesis.ChainID != a.config.ChainID || o.Genesis.ValidateAndComplete() != nil {
		return nil, ErrGenesis
	}
	// Review the same request that consensus will deliver before assigning an
	// engine or writing a persistent storage binding. InitChain rechecks it.
	cp := o.Genesis.ConsensusParams.ToProto()
	request := &abci.RequestInitChain{ChainId: o.Genesis.ChainID, Time: o.Genesis.GenesisTime,
		InitialHeight: o.Genesis.InitialHeight, AppStateBytes: o.Genesis.AppState, ConsensusParams: &cp}
	for _, v := range o.Genesis.Validators {
		if v.PubKey == nil || v.PubKey.Type() != "ed25519" || len(v.PubKey.Bytes()) != 32 {
			return nil, ErrGenesis
		}
		request.Validators = append(request.Validators, abci.Ed25519ValidatorUpdate(v.PubKey.Bytes(), v.Power))
	}
	if err := validateGenesisRequest(request, a.config); err != nil {
		return nil, err
	}
	a.mu.Lock()
	if !a.usable() || a.finalized || a.engineAssigned {
		a.mu.Unlock()
		return nil, ErrLifecycle
	}
	if a.config.BridgeGenesis != nil && a.base.LastBlockHeight() > 0 {
		ctx, err := a.base.CreateQueryContext(a.base.LastBlockHeight(), false)
		if err != nil {
			a.mu.Unlock()
			return nil, ErrConfig
		}
		stored, err := a.metadata(ctx)
		digest, digestErr := BridgeGenesisDigest(a.config, request)
		if err != nil || digestErr != nil || stored.BridgeGenesis == nil || *stored.BridgeGenesis != digest {
			a.mu.Unlock()
			return nil, ErrBridgeGenesis
		}
	}
	if o.Persistent {
		if err := bindLocalStorage(a, o); err != nil {
			a.mu.Unlock()
			return nil, err
		}
	}
	signer := o.Signer
	if o.Observer {
		pub, err := signer.GetPubKey()
		if err != nil {
			a.mu.Unlock()
			return nil, ErrConfig
		}
		for _, v := range o.Genesis.Validators {
			if v.PubKey.Equals(pub) {
				a.mu.Unlock()
				return nil, ErrConfig
			}
		}
		signer = observerSigner{pub}
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
	if network {
		config.Moniker = "GodCometBFT synthetic testnet"
		config.TxIndex.Indexer = "kv"
	}
	config.RPC.ListenAddress = ""
	config.RPC.GRPCListenAddress = ""
	config.RPC.PprofListenAddress = ""
	config.P2P.ListenAddress = o.Listen
	config.P2P.ExternalAddress = ""
	config.P2P.PersistentPeers = strings.Join(o.Peers, ",")
	config.P2P.Seeds = ""
	config.P2P.PexReactor = false
	config.P2P.AddrBookStrict = false
	listenURL, _ := url.Parse(o.Listen)
	listenHost, _, _ := net.SplitHostPort(listenURL.Host)
	config.P2P.AllowDuplicateIP = net.ParseIP(listenHost).IsLoopback()
	config.P2P.MaxNumInboundPeers = 32
	config.P2P.MaxNumOutboundPeers = 32
	config.Consensus.TimeoutCommit = o.BlockInterval
	config.Consensus.TimeoutPropose = 2 * time.Second
	config.Consensus.TimeoutPrevote = time.Second
	config.Consensus.TimeoutPrecommit = time.Second
	config.Instrumentation.Prometheus = false
	if !network {
		config.TxIndex.Indexer = "null"
	}
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
	n, err := cmtNode.NewNodeWithContext(ctx, config, signer, o.NodeKey, proxy.NewLocalClientCreator(application{a}),
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

type observerSigner struct{ pub cmtcrypto.PubKey }

func (s observerSigner) GetPubKey() (cmtcrypto.PubKey, error)        { return s.pub, nil }
func (observerSigner) SignVote(string, *cmtproto.Vote) error         { return ErrConfig }
func (observerSigner) SignProposal(string, *cmtproto.Proposal) error { return ErrConfig }

// Submit performs one admission attempt. ErrCommitPending means no ante state
// was touched and the caller may retry after commitment; it is not acceptance.
// Other errors must not be indiscriminately retried or reported as success.
func (n *LocalNode) Submit(ctx context.Context, wire []byte) error {
	if ctx == nil || n == nil || n.node == nil || !n.node.IsRunning() || len(wire) == 0 || len(wire) > n.app.config.Policy.MaxTxBytes {
		return ErrLifecycle
	}
	result := make(chan error, 1)
	err := n.node.Mempool().CheckTx(cmttypes.Tx(wire), func(res *abci.ResponseCheckTx) {
		switch {
		case commitPendingCheck(res):
			result <- ErrCommitPending
		case res == nil || res.Code != 0:
			result <- ErrBlock
		default:
			result <- nil
		}
	}, mempool.TxInfo{})
	if err != nil {
		return err
	}
	select {
	case err := <-result:
		return err
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
	// This method exists only with the reviewed checksum-bound lifecycle
	// overlay. A plain unpatched build fails compilation rather than silently
	// retaining the peer-read/store-close race. OnStop joins before DB closure.
	n.node.ConsensusReactor().WaitPeerRoutines()
	if err == nil {
		n.app.mu.Lock()
		n.app.engineRunning = false
		n.app.mu.Unlock()
	}
	return err
}
