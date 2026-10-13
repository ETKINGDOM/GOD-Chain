//go:build go1.25

package godtestnet

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
	"github.com/ETKINGDOM/GOD-Chain/internal/godrh"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	cmted "github.com/cometbft/cometbft/crypto/ed25519"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	dbm "github.com/cosmos/cosmos-db"
)

const launchRehearsalFile = "launch-rehearsal.json"
const launchRehearsalMode = "private-synthetic-launch-rehearsal"
const launchServiceMode = "private-synthetic-launch-service"

var ErrLaunchRehearsal = errors.New("private launch rehearsal rejected; production and real assets remain disabled")

func launchGuardSeparated(home, guard string) bool {
	if !separateGuard(home, guard) {
		return false
	}
	actualHome, err := filepath.EvalSymlinks(home)
	actualGuard, e := filepath.EvalSymlinks(guard)
	// A different spelling or intermediate symlink is not a second backup
	// scope. The original core guard still requires private, unchanged roots.
	return err == nil && e == nil && separateGuard(actualHome, actualGuard)
}

// Both pins are independently reviewed inputs at every invocation, not read
// from the candidate's own hash or a restored home. Paths never reach reports.
type LaunchRehearsalInputs struct {
	InputPath, CandidatePath string
	InputPin, CandidatePin   [32]byte
}

type LaunchRehearsalOptions struct {
	P2PListen string
	Peers     []string
}

// Explicit operational reporting cadence, not a consensus/economic parameter.
// An external supervisor must bound resources and shutdown of blocked I/O.
type LaunchServiceOptions struct {
	ReportInterval time.Duration
}

type launchRehearsalDocument struct {
	Version            uint32   `json:"version"`
	Mode               string   `json:"mode"`
	InputPin           [32]byte `json:"inputPin"`
	CandidatePin       [32]byte `json:"candidatePin"`
	GenesisPin         [32]byte `json:"genesisPin"`
	ConsensusPublicKey []byte   `json:"consensusPublicKey"`
	PeerPublicKey      []byte   `json:"peerPublicKey"`
	P2PListen          string   `json:"p2pListen"`
	Peers              []string `json:"peers"`
}

// These reports are observations of a temporary synthetic ledger, never a
// funding/finality certificate. No address, key, path, hash or endpoint escapes.
type LaunchRehearsalReport struct {
	Mode                   string `json:"mode"`
	Event                  string `json:"event"`
	Synthetic              bool   `json:"synthetic"`
	RealAssets             bool   `json:"realAssets"`
	ProductionStartEnabled bool   `json:"productionStartEnabled"`
	PublicPeeringEnabled   bool   `json:"publicPeeringEnabled"`
	SigningGuard           bool   `json:"signingGuard"`
	CandidateMatched       bool   `json:"candidateMatched"`
	ValidatorCount         int    `json:"validatorCount"`
	CommittedHeight        int64  `json:"committedHeight,string"`
	FixedSupplyMatches     bool   `json:"fixedSupplyMatches"`
	SignedGObserved        bool   `json:"signedGObserved"`
	Service                bool   `json:"service,omitempty"`
}

func launchReport(event string, loaded *Loaded) LaunchRehearsalReport {
	return LaunchRehearsalReport{Mode: launchRehearsalMode, Event: event, Synthetic: true, SigningGuard: true, CandidateMatched: true, ValidatorCount: len(loaded.Genesis.Validators)}
}

func launchPeerOptions(o LaunchRehearsalOptions, nk *p2p.NodeKey) bool {
	valid, exposed := endpoint(o.P2PListen, true, true)
	if !valid || exposed || len(o.Peers) > 32 || o.Peers == nil || nk == nil || nk.PrivKey == nil {
		return false
	}
	seen := map[string]bool{}
	for _, peer := range o.Peers {
		at := strings.IndexByte(peer, '@')
		if at != 40 || seen[peer[:at]] || peer[:at] == string(nk.ID()) {
			return false
		}
		id, err := hex.DecodeString(peer[:at])
		valid, exposed := endpoint(peer[at+1:], false, false)
		if err != nil || len(id) != 20 || hex.EncodeToString(id) != peer[:at] || !valid || exposed {
			return false
		}
		seen[peer[:at]] = true
	}
	return true
}

func launchConsensusDecode(raw []byte, value any) error {
	if decodeConsensus(raw, value) != nil {
		return ErrLaunchRehearsal
	}
	want, err := cmtjson.Marshal(value)
	var compact bytes.Buffer
	if err != nil || json.Compact(&compact, raw) != nil || !bytes.Equal(want, compact.Bytes()) {
		return ErrLaunchRehearsal
	}
	return nil
}

// Read the original existing identities, never generate, copy, adopt a used
// signer or reset last-sign state. The guard additionally checks chain/step.
func loadLaunchKeys(r *os.Root, home string) (*privval.FilePV, *p2p.NodeKey, error) {
	key, err := readLaunchFile(r, keyFile, 4096)
	defer clear(key)
	var k privval.FilePVKey
	if err != nil || launchConsensusDecode(key, &k) != nil {
		return nil, nil, ErrLaunchRehearsal
	}
	kp, ok := k.PrivKey.(cmted.PrivKey)
	if !ok || len(kp) != ed25519.PrivateKeySize || !bytes.Equal(kp, ed25519.NewKeyFromSeed(kp[:32])) || k.PubKey == nil ||
		!k.PubKey.Equals(kp.PubKey()) || !bytes.Equal(k.Address, kp.PubKey().Address()) {
		return nil, nil, ErrLaunchRehearsal
	}
	pv := privval.NewFilePV(kp, filepath.Join(home, keyFile), filepath.Join(home, stateFile))
	state, err := readLaunchFile(r, stateFile, 16<<10)
	defer clear(state)
	if err != nil || launchConsensusDecode(state, &pv.LastSignState) != nil {
		return nil, nil, ErrLaunchRehearsal
	}
	peer, err := readLaunchFile(r, peerFile, 4096)
	defer clear(peer)
	var nk p2p.NodeKey
	if err != nil || launchConsensusDecode(peer, &nk) != nil {
		return nil, nil, ErrLaunchRehearsal
	}
	pk, ok := nk.PrivKey.(cmted.PrivKey)
	if !ok || len(pk) != ed25519.PrivateKeySize || !bytes.Equal(pk, ed25519.NewKeyFromSeed(pk[:32])) {
		return nil, nil, ErrLaunchRehearsal
	}
	return pv, &nk, nil
}

func loadLaunchCandidate(ctx context.Context, r *os.Root, home string, inputs LaunchRehearsalInputs, options LaunchRehearsalOptions) (*Loaded, launchRehearsalDocument, error) {
	c, g, interval, _, err := godrh.ReadPrivateLaunchCandidate(ctx, inputs.InputPath, inputs.InputPin, inputs.CandidatePath, inputs.CandidatePin)
	if err != nil {
		return nil, launchRehearsalDocument{}, ErrLaunchRehearsal
	}
	pv, nk, err := loadLaunchKeys(r, home)
	if err != nil || !launchPeerOptions(options, nk) || !launchRootMatches(r, home) || !c.Prototype || !c.RequireValidatorProof || c.BridgeGenesis == nil || c.BridgeApprovalGas == 0 {
		return nil, launchRehearsalDocument{}, ErrLaunchRehearsal
	}
	member := false
	for _, v := range g.Validators {
		member = member || v.PubKey.Equals(pv.Key.PubKey)
	}
	if !member {
		return nil, launchRehearsalDocument{}, ErrLaunchRehearsal
	}
	raw, err := cmtjson.Marshal(g)
	if err != nil || ctx.Err() != nil {
		return nil, launchRehearsalDocument{}, ErrLaunchRehearsal
	}
	d := launchRehearsalDocument{1, launchRehearsalMode, inputs.InputPin, inputs.CandidatePin, sha256.Sum256(raw), bytes.Clone(pv.Key.PubKey.Bytes()), bytes.Clone(nk.PrivKey.PubKey().Bytes()), options.P2PListen, append([]string{}, options.Peers...)}
	l := &Loaded{Document: Document{Runtime: c, Role: "validator", P2PListen: d.P2PListen, Peers: d.Peers, BlockIntervalMillis: uint32(interval / time.Millisecond)}, Genesis: g, Signer: pv, NodeKey: nk}
	return l, d, nil
}

func readLaunchProfile(r *os.Root) (launchRehearsalDocument, []byte, error) {
	raw, err := readLaunchFile(r, launchRehearsalFile, 16<<10)
	var d launchRehearsalDocument
	if err != nil || decode(raw, &d) != nil || d.Version != 1 || d.Mode != launchRehearsalMode {
		return d, nil, ErrLaunchRehearsal
	}
	want, err := json.Marshal(d)
	if err != nil || !bytes.Equal(raw, want) {
		return d, nil, ErrLaunchRehearsal
	}
	return d, raw, nil
}

func loadLaunchHome(ctx context.Context, r *os.Root, home string, inputs LaunchRehearsalInputs) (*Loaded, error) {
	d, raw, err := readLaunchProfile(r)
	if err != nil || d.InputPin != inputs.InputPin || d.CandidatePin != inputs.CandidatePin {
		return nil, ErrLaunchRehearsal
	}
	l, want, err := loadLaunchCandidate(ctx, r, home, inputs, LaunchRehearsalOptions{d.P2PListen, d.Peers})
	canonical, e := json.Marshal(want)
	if err != nil || e != nil || !bytes.Equal(canonical, raw) {
		return nil, ErrLaunchRehearsal
	}
	marker, err := readLaunchFile(r, signingGuardFile, 2048)
	if err != nil || len(marker) == 0 {
		return nil, ErrLaunchRehearsal
	}
	return l, nil
}

// InitializeLaunchRehearsal binds fresh, pre-existing keys to an exact private
// candidate and mandatory separate witness. Existing/used/partial homes fail
// closed. An interruption leaves files for review; no deletion or repair.
func InitializeLaunchRehearsal(ctx context.Context, home, guard string, inputs LaunchRehearsalInputs, options LaunchRehearsalOptions) (report LaunchRehearsalReport, result error) {
	defer func() {
		if recover() != nil {
			report, result = LaunchRehearsalReport{}, ErrLaunchRehearsal
		}
	}()
	if ctx == nil || ctx.Err() != nil || !launchGuardSeparated(home, guard) {
		return report, ErrLaunchRehearsal
	}
	r, err := openLaunchRoot(home)
	if err != nil {
		return report, ErrLaunchRehearsal
	}
	defer r.Close()
	lease, err := acquireNodeLease(r)
	if err != nil {
		return report, ErrLaunchRehearsal
	}
	defer lease.Close()
	for _, name := range []string{configFile, genesisFile, "data", launchRehearsalFile, signingGuardFile} {
		if _, e := r.Lstat(name); !os.IsNotExist(e) {
			return report, ErrLaunchRehearsal
		}
	}
	l, d, err := loadLaunchCandidate(ctx, r, home, inputs, options)
	if err != nil || l.Signer.LastSignState.Height != 0 || l.Signer.LastSignState.Round != 0 || l.Signer.LastSignState.Step != 0 ||
		len(l.Signer.LastSignState.SignBytes) != 0 || len(l.Signer.LastSignState.Signature) != 0 {
		return report, ErrLaunchRehearsal
	}
	raw, err := json.Marshal(d)
	marker, e := json.Marshal(guardRequirement{1, sha256.Sum256([]byte(guard))})
	if err != nil || e != nil || ctx.Err() != nil {
		return report, ErrLaunchRehearsal
	}
	if godnode.InitializeFileSigningGuard(guard, l.Document.Runtime.ChainID, d.GenesisPin, l.Signer,
		godnode.SigningGuardFiles{Home: home, KeyFile: keyFile, StateFile: stateFile}, func() error {
			if ctx.Err() != nil || !launchRootMatches(r, home) || writePrivate(r, launchRehearsalFile, raw) != nil || writePrivate(r, signingGuardFile, marker) != nil {
				return ErrLaunchRehearsal
			}
			return nil
		}) != nil {
		return report, ErrLaunchRehearsal
	}
	return launchReport("initialized", l), nil
}

// RunLaunchRehearsal runs an actual persistent God EVM + God SDK + GodCometBFT
// validator, only with synthetic funding, explicit loopback peers and a retained
// signing witness. No RPC, source client, wallet signer or production flag. The
// bounded run joins consensus workers before closing stores/leases. A deadline
// is cooperative, not a hard limit for blocked filesystem calls or shutdown.
func RunLaunchRehearsal(ctx context.Context, home, guard string, inputs LaunchRehearsalInputs, duration time.Duration, out io.Writer) (result error) {
	defer func() {
		if recover() != nil {
			result = ErrLaunchRehearsal
		}
	}()
	if ctx == nil || ctx.Err() != nil || duration < 2*time.Second || duration > 180*time.Second {
		return ErrLaunchRehearsal
	}
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	return runLaunchCandidate(ctx, home, guard, inputs, false, 500*time.Millisecond, out)
}

// ServeLaunchCandidate uses the same exact staged synthetic candidate and
// retained identities/witness as the bounded rehearsal, until caller shutdown.
// No duration, production mode, public peer, RPC or asset override is accepted.
// Callers must supply a cancellable context; the CLI supplies SIGINT/SIGTERM.
// A healthy heartbeat is not committed progress or source funding/finality.
func ServeLaunchCandidate(ctx context.Context, home, guard string, inputs LaunchRehearsalInputs, options LaunchServiceOptions, out io.Writer) (result error) {
	defer func() {
		if recover() != nil {
			result = ErrLaunchRehearsal
		}
	}()
	if ctx == nil || ctx.Err() != nil || ctx.Done() == nil || options.ReportInterval < time.Second || options.ReportInterval > 300*time.Second {
		return ErrLaunchRehearsal
	}
	return runLaunchCandidate(ctx, home, guard, inputs, true, options.ReportInterval, out)
}

// Recheck immutable service inputs without rereading a changing signing state
// or opening a second signer. The active guard checks its key/progress itself.
// This detects changes between checks, not a hostile host owner or TOCTOU fence.
func checkLaunchServiceInputs(ctx context.Context, r *os.Root, home, guard string, inputs LaunchRehearsalInputs, l *Loaded) error {
	if ctx == nil || ctx.Err() != nil || !launchRootMatches(r, home) || !launchGuardSeparated(home, guard) {
		return ErrLaunchRehearsal
	}
	_, raw, err := readLaunchProfile(r)
	genesisPin, e := guardGenesis(l)
	pub, p := l.Signer.GetPubKey()
	if err != nil || e != nil || p != nil || pub == nil {
		return ErrLaunchRehearsal
	}
	want, err := json.Marshal(launchRehearsalDocument{1, launchRehearsalMode, inputs.InputPin, inputs.CandidatePin, genesisPin,
		bytes.Clone(pub.Bytes()), bytes.Clone(l.NodeKey.PrivKey.PubKey().Bytes()), l.Document.P2PListen, append([]string{}, l.Document.Peers...)})
	if err != nil || !bytes.Equal(raw, want) {
		return ErrLaunchRehearsal
	}
	marker, err := readLaunchFile(r, signingGuardFile, 2048)
	want, e = json.Marshal(guardRequirement{1, sha256.Sum256([]byte(guard))})
	if err != nil || e != nil || !bytes.Equal(marker, want) {
		return ErrLaunchRehearsal
	}
	_, _, _, _, err = godrh.ReadPrivateLaunchCandidate(ctx, inputs.InputPath, inputs.InputPin, inputs.CandidatePath, inputs.CandidatePin)
	if err != nil || ctx.Err() != nil {
		return ErrLaunchRehearsal
	}
	return nil
}

func runLaunchCandidate(ctx context.Context, home, guard string, inputs LaunchRehearsalInputs, service bool, reportInterval time.Duration, out io.Writer) (result error) {
	defer func() {
		if recover() != nil {
			result = ErrLaunchRehearsal
		}
	}()
	if ctx == nil || ctx.Err() != nil || out == nil || !launchGuardSeparated(home, guard) {
		return ErrLaunchRehearsal
	}
	r, err := openLaunchRoot(home)
	if err != nil {
		return ErrLaunchRehearsal
	}
	defer r.Close()
	lease, err := acquireExistingNodeLease(r)
	if err != nil {
		return ErrLaunchRehearsal
	}
	defer func() {
		if lease.Close() != nil {
			result = ErrLaunchRehearsal
		}
	}()
	l, err := loadLaunchHome(ctx, r, home, inputs)
	if err != nil {
		return ErrLaunchRehearsal
	}
	g, err := openNodeSigningGuard(r, home, guard, l)
	if err != nil || g == nil {
		return ErrLaunchRehearsal
	}
	defer func() {
		if g.Close() != nil {
			result = ErrLaunchRehearsal
		}
	}()
	if ctx.Err() != nil || g.Check() != nil || !launchRootMatches(r, home) {
		return ErrLaunchRehearsal
	}
	if service && checkLaunchServiceInputs(ctx, r, home, guard, inputs, l) != nil {
		return ErrLaunchRehearsal
	}
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
		BlockInterval: time.Duration(l.Document.BlockIntervalMillis) * time.Millisecond, Genesis: l.Genesis, Signer: g, NodeKey: l.NodeKey, Persistent: true})
	if err != nil {
		return ErrLaunchRehearsal
	}
	defer func() {
		if n.Stop() != nil || g.Check() != nil {
			result = ErrLaunchRehearsal
		}
	}()
	report := launchReport("started", l)
	if service {
		report.Mode, report.Service = launchServiceMode, true
	}
	encoder := json.NewEncoder(out)
	if encoder.Encode(report) != nil {
		return ErrLaunchRehearsal
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	lastReport := time.Now()
	var emittedHeight int64
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if ctx.Err() != nil {
				return nil
			}
			if g.Check() != nil || !launchRootMatches(r, home) {
				return ErrLaunchRehearsal
			}
			due := !service || time.Since(lastReport) >= reportInterval
			if service && due && checkLaunchServiceInputs(ctx, r, home, guard, inputs, l) != nil {
				if ctx.Err() != nil {
					return nil
				}
				return ErrLaunchRehearsal
			}
			view, e := a.QueryNetwork(0)
			if e != nil {
				// Fresh application has no committed version before block one.
				// No report fabricates a height, asset balance or progress.
				if report.CommittedHeight > 0 {
					return ErrLaunchRehearsal
				}
				if service && due {
					report.Event = "heartbeat"
					if encoder.Encode(report) != nil {
						return ErrLaunchRehearsal
					}
					lastReport = time.Now()
				}
				continue
			}
			if !view.Commit.Synthetic || view.Commit.RealAssets || view.TotalGod != godrewards.FixedGodSupply().String() || view.Commit.Height < report.CommittedHeight {
				return ErrLaunchRehearsal
			}
			report.CommittedHeight, report.FixedSupplyMatches = view.Commit.Height, true
			report.SignedGObserved = view.PendingG != "0" || view.OutstandingG != "0"
			if !due || !service && view.Commit.Height == emittedHeight {
				continue
			}
			report.Event = "committed"
			if service && view.Commit.Height == emittedHeight {
				report.Event = "heartbeat"
			}
			if encoder.Encode(report) != nil {
				return ErrLaunchRehearsal
			}
			lastReport, emittedHeight = time.Now(), view.Commit.Height
		}
	}
}
