//go:build go1.25

package godtestnet

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
	"github.com/ETKINGDOM/GOD-Chain/internal/godrh"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	dbm "github.com/cosmos/cosmos-db"
)

const launchObserverFile = "launch-observer.json"
const launchObserverMode = "private-synthetic-launch-observer"

// All listeners, peers and browser origins are explicit loopback inputs. This
// adapter has no public listener, transaction submission or source-chain client.
type LaunchObserverOptions struct {
	P2PListen  string   `json:"p2pListen"`
	Peers      []string `json:"peers"`
	RPCListen  string   `json:"rpcListen"`
	RPCHosts   []string `json:"rpcHosts"`
	RPCOrigins []string `json:"rpcOrigins"`
}

type launchObserverDocument struct {
	Version            uint32                `json:"version"`
	Mode               string                `json:"mode"`
	InputPin           [32]byte              `json:"inputPin"`
	CandidatePin       [32]byte              `json:"candidatePin"`
	GenesisPin         [32]byte              `json:"genesisPin"`
	ConsensusPublicKey []byte                `json:"consensusPublicKey"`
	PeerPublicKey      []byte                `json:"peerPublicKey"`
	Options            LaunchObserverOptions `json:"options"`
}

// Observations do not expose keys, addresses, paths, endpoints or candidate pins.
type LaunchObserverReport struct {
	Mode                   string `json:"mode"`
	Event                  string `json:"event"`
	Synthetic              bool   `json:"synthetic"`
	RealAssets             bool   `json:"realAssets"`
	ProductionStartEnabled bool   `json:"productionStartEnabled"`
	PublicPeeringEnabled   bool   `json:"publicPeeringEnabled"`
	Observer               bool   `json:"observer"`
	ReadOnlyRPC            bool   `json:"readOnlyRpc"`
	CandidateMatched       bool   `json:"candidateMatched"`
	ValidatorCount         int    `json:"validatorCount"`
	CommittedHeight        int64  `json:"committedHeight,string"`
	FixedSupplyMatches     bool   `json:"fixedSupplyMatches"`
}

func launchObserverReport(event string, l *Loaded) LaunchObserverReport {
	return LaunchObserverReport{Mode: launchObserverMode, Event: event, Synthetic: true,
		Observer: true, ReadOnlyRPC: true, CandidateMatched: true, ValidatorCount: len(l.Genesis.Validators)}
}

func loadLaunchObserverCandidate(ctx context.Context, r *os.Root, home string, inputs LaunchRehearsalInputs, options LaunchObserverOptions) (*Loaded, launchObserverDocument, error) {
	c, g, interval, _, err := godrh.ReadPrivateLaunchCandidate(ctx, inputs.InputPath, inputs.InputPin, inputs.CandidatePath, inputs.CandidatePin)
	if err != nil || ctx.Err() != nil || !launchRootMatches(r, home) || !c.Prototype || !c.RequireValidatorProof || c.BridgeGenesis == nil || c.BridgeApprovalGas == 0 {
		return nil, launchObserverDocument{}, ErrLaunchRehearsal
	}
	pv, nk, err := loadLaunchKeys(r, home)
	if err != nil || !launchPeerOptions(LaunchRehearsalOptions{options.P2PListen, options.Peers}, nk) || !loopbackRPC(options.RPCListen) ||
		godnode.ValidateLaunchCandidateRPCOptions(godnode.RPCOptions{Hosts: options.RPCHosts, Origins: options.RPCOrigins}) != nil ||
		pv.LastSignState.Height != 0 || pv.LastSignState.Round != 0 || pv.LastSignState.Step != 0 || len(pv.LastSignState.SignBytes) != 0 || len(pv.LastSignState.Signature) != 0 || pv.Key.PubKey.Equals(nk.PrivKey.PubKey()) {
		return nil, launchObserverDocument{}, ErrLaunchRehearsal
	}
	for _, v := range g.Validators {
		if v.PubKey.Equals(pv.Key.PubKey) || v.PubKey.Equals(nk.PrivKey.PubKey()) {
			return nil, launchObserverDocument{}, ErrLaunchRehearsal
		}
	}
	raw, err := cmtjson.Marshal(g)
	if err != nil {
		return nil, launchObserverDocument{}, ErrLaunchRehearsal
	}
	options.Peers = append([]string{}, options.Peers...)
	options.RPCHosts = append([]string{}, options.RPCHosts...)
	options.RPCOrigins = append([]string{}, options.RPCOrigins...)
	d := launchObserverDocument{Version: 1, Mode: launchObserverMode, InputPin: inputs.InputPin, CandidatePin: inputs.CandidatePin,
		GenesisPin: sha256.Sum256(raw), ConsensusPublicKey: bytes.Clone(pv.Key.PubKey.Bytes()), PeerPublicKey: bytes.Clone(nk.PrivKey.PubKey().Bytes()), Options: options}
	l := &Loaded{Document: Document{Runtime: c, Role: "observer", ConsensusPublicKey: bytes.Clone(d.ConsensusPublicKey), PeerPublicKey: bytes.Clone(d.PeerPublicKey), P2PListen: options.P2PListen, Peers: options.Peers, RPCListen: options.RPCListen,
		RPCHosts: options.RPCHosts, RPCOrigins: options.RPCOrigins, BlockIntervalMillis: uint32(interval / time.Millisecond)}, Genesis: g, Signer: pv, NodeKey: nk}
	return l, d, nil
}

func loadLaunchObserverHome(ctx context.Context, r *os.Root, home string, inputs LaunchRehearsalInputs) (*Loaded, error) {
	raw, err := readLaunchFile(r, launchObserverFile, 16<<10)
	var d launchObserverDocument
	if err != nil || decode(raw, &d) != nil || d.Version != 1 || d.Mode != launchObserverMode || d.InputPin != inputs.InputPin || d.CandidatePin != inputs.CandidatePin {
		return nil, ErrLaunchRehearsal
	}
	l, want, err := loadLaunchObserverCandidate(ctx, r, home, inputs, d.Options)
	canonical, e := json.Marshal(want)
	if err != nil || e != nil || !bytes.Equal(raw, canonical) {
		return nil, ErrLaunchRehearsal
	}
	for _, name := range []string{configFile, genesisFile, launchRehearsalFile, signingGuardFile} {
		if _, err := r.Lstat(name); !os.IsNotExist(err) {
			return nil, ErrLaunchRehearsal
		}
	}
	return l, nil
}

func freshLaunchObserverHome(r *os.Root) bool {
	for _, name := range []string{configFile, genesisFile, launchRehearsalFile, launchObserverFile, signingGuardFile, "data"} {
		if _, err := r.Lstat(name); !os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// InitializeLaunchObserver binds fresh pre-existing non-validator identities to
// the exact original assembly and candidate. It never generates/copies a key,
// initializes a signing witness, starts a node or repairs a partial home.
func InitializeLaunchObserver(ctx context.Context, home string, inputs LaunchRehearsalInputs, options LaunchObserverOptions) (report LaunchObserverReport, result error) {
	defer func() {
		if recover() != nil {
			report, result = LaunchObserverReport{}, ErrLaunchRehearsal
		}
	}()
	if ctx == nil || ctx.Err() != nil {
		return report, ErrLaunchRehearsal
	}
	r, err := openLaunchRoot(home)
	if err != nil {
		return report, ErrLaunchRehearsal
	}
	defer r.Close()
	// Invalid inputs are read-only; do not create a lease before preflight.
	if !freshLaunchObserverHome(r) {
		return report, ErrLaunchRehearsal
	}
	if _, _, err := loadLaunchObserverCandidate(ctx, r, home, inputs, options); err != nil {
		return report, ErrLaunchRehearsal
	}
	lease, err := acquireNodeLease(r)
	if err != nil {
		return report, ErrLaunchRehearsal
	}
	defer func() {
		if lease.Close() != nil {
			report, result = LaunchObserverReport{}, ErrLaunchRehearsal
		}
	}()
	if !freshLaunchObserverHome(r) {
		return report, ErrLaunchRehearsal
	}
	l, d, err := loadLaunchObserverCandidate(ctx, r, home, inputs, options)
	if err != nil {
		return report, ErrLaunchRehearsal
	}
	raw, err := json.Marshal(d)
	if err != nil || ctx.Err() != nil || !launchRootMatches(r, home) || writePrivate(r, launchObserverFile, raw) != nil {
		return report, ErrLaunchRehearsal
	}
	if _, err := loadLaunchObserverHome(ctx, r, home, inputs); err != nil {
		return report, ErrLaunchRehearsal
	}
	return launchObserverReport("initialized", l), nil
}

// ServeLaunchObserver replicates the same synthetic bridge-funded candidate as
// the guarded validators, serving committed reads only until caller shutdown.
// The private observer has no consensus signing capability or write RPC route.
// Shutdown joins HTTP requests and consensus before releasing stores/home lease.
func ServeLaunchObserver(ctx context.Context, home string, inputs LaunchRehearsalInputs, options LaunchServiceOptions, out io.Writer) (result error) {
	defer func() {
		if recover() != nil {
			result = ErrLaunchRehearsal
		}
	}()
	if ctx == nil || ctx.Err() != nil || ctx.Done() == nil || out == nil || options.ReportInterval < time.Second || options.ReportInterval > 300*time.Second {
		return ErrLaunchRehearsal
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	r, err := openLaunchRoot(home)
	if err != nil {
		return ErrLaunchRehearsal
	}
	defer r.Close()
	if _, err := loadLaunchObserverHome(ctx, r, home, inputs); err != nil {
		return ErrLaunchRehearsal
	}
	lease, err := acquireExistingNodeLease(r)
	if err != nil {
		return ErrLaunchRehearsal
	}
	defer func() {
		if lease.Close() != nil {
			result = ErrLaunchRehearsal
		}
	}()
	l, err := loadLaunchObserverHome(ctx, r, home, inputs)
	if err != nil {
		return ErrLaunchRehearsal
	}
	listener, err := net.Listen("tcp", l.Document.RPCListen)
	if err != nil {
		return ErrLaunchRehearsal
	}
	defer listener.Close()
	for _, name := range []string{"data", "data/sdk"} {
		if launchDataDirectory(r, name) != nil {
			return ErrLaunchRehearsal
		}
	}
	if ctx.Err() != nil || !launchRootMatches(r, home) {
		return ErrLaunchRehearsal
	}
	db, err := dbm.NewDB("god_app", dbm.GoLevelDBBackend, filepath.Join(home, "data", "sdk"))
	if err != nil {
		return ErrLaunchRehearsal
	}
	a, err := godnode.New(db, l.Document.Runtime)
	if err != nil {
		_ = db.Close()
		return ErrLaunchRehearsal
	}
	defer func() {
		if a.Close() != nil {
			result = ErrLaunchRehearsal
		}
	}()
	n, err := godnode.StartLocal(ctx, a, godnode.LocalOptions{Directory: home, Listen: l.Document.P2PListen, Peers: l.Document.Peers,
		BlockInterval: time.Duration(l.Document.BlockIntervalMillis) * time.Millisecond, Genesis: l.Genesis, Signer: l.Signer, NodeKey: l.NodeKey, Persistent: true, Observer: true})
	if err != nil {
		return ErrLaunchRehearsal
	}
	defer func() {
		if n.Stop() != nil {
			result = ErrLaunchRehearsal
		}
	}()
	done := make(chan error, 1)
	go func() {
		done <- godnode.ServeLaunchCandidateRPC(ctx, n, listener, godnode.RPCOptions{Hosts: l.Document.RPCHosts, Origins: l.Document.RPCOrigins})
	}()
	joined := false
	defer func() {
		cancel()
		if !joined {
			if <-done != nil {
				result = ErrLaunchRehearsal
			}
		}
	}()
	report := launchObserverReport("started", l)
	encoder := json.NewEncoder(out)
	if encoder.Encode(report) != nil {
		return ErrLaunchRehearsal
	}
	ticker := time.NewTicker(options.ReportInterval)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			joined = true
			if err != nil || ctx.Err() == nil {
				return ErrLaunchRehearsal
			}
			return nil
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if ctx.Err() != nil {
				return nil
			}
			current, err := loadLaunchObserverHome(ctx, r, home, inputs)
			old, e := json.Marshal(l.Document)
			var now []byte
			if current != nil {
				now, _ = json.Marshal(current.Document)
			}
			if err != nil || e != nil || !bytes.Equal(old, now) {
				return ErrLaunchRehearsal
			}
			view, err := a.QueryNetwork(0)
			if err != nil {
				if report.CommittedHeight > 0 {
					return ErrLaunchRehearsal
				}
				report.Event = "heartbeat"
			} else {
				if !view.Commit.Synthetic || view.Commit.RealAssets || view.Commit.Height < report.CommittedHeight || view.TotalGod != godrewards.FixedGodSupply().String() {
					return ErrLaunchRehearsal
				}
				report.Event = "heartbeat"
				if view.Commit.Height > report.CommittedHeight {
					report.Event = "committed"
				}
				report.CommittedHeight, report.FixedSupplyMatches = view.Commit.Height, true
			}
			if encoder.Encode(report) != nil {
				return ErrLaunchRehearsal
			}
		}
	}
}
