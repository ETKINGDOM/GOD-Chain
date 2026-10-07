//go:build go1.25

package godtestnet

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtx"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	cmted "github.com/cometbft/cometbft/crypto/ed25519"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
)

const configFile = "testnet-node.json"
const genesisFile = "genesis.json"
const keyFile = "priv_validator_key.json"
const stateFile = "priv_validator_state.json"
const peerFile = "node_key.json"

type Document struct {
	Version             uint32         `json:"version"`
	Mode                string         `json:"mode"`
	Role                string         `json:"role"`
	Runtime             godnode.Config `json:"runtime"`
	GenesisSHA256       string         `json:"genesisSha256"`
	ConsensusPublicKey  []byte         `json:"consensusPublicKey"`
	PeerPublicKey       []byte         `json:"peerPublicKey"`
	P2PListen           string         `json:"p2pListen"`
	Peers               []string       `json:"peers"`
	RPCListen           string         `json:"rpcListen"`
	RPCHosts            []string       `json:"rpcHosts"`
	RPCOrigins          []string       `json:"rpcOrigins"`
	BlockIntervalMillis uint32         `json:"blockIntervalMillis"`
}

// The report contains no addresses, keys, endpoints, signer IDs or local paths.
type Report struct {
	Mode            string `json:"mode"`
	Role            string `json:"role,omitempty"`
	Synthetic       bool   `json:"synthetic"`
	RealAssets      bool   `json:"realAssets"`
	BridgeEnabled   bool   `json:"bridgeEnabled"`
	ValidatorCount  int    `json:"validatorCount"`
	NetworkExposure bool   `json:"networkExposure"`
	Persistent      bool   `json:"persistent"`
}

type Loaded struct {
	Document Document
	Genesis  *cmttypes.GenesisDoc
	Signer   *privval.FilePV
	NodeKey  *p2p.NodeKey
	Report   Report
}

func endpoint(text string, p2p, listen bool) (bool, bool) {
	if p2p {
		if !strings.HasPrefix(text, "tcp://") {
			return false, false
		}
		text = strings.TrimPrefix(text, "tcp://")
	}
	host, port, err := net.SplitHostPort(text)
	n, e := strconv.Atoi(port)
	ip := net.ParseIP(host)
	valid := err == nil && e == nil && ip != nil && n > 0 && n <= 65535 && port == strconv.Itoa(n) &&
		!ip.IsMulticast() && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && (listen || !ip.IsUnspecified())
	return valid, valid && !ip.IsLoopback()
}

func Load(home string) (*Loaded, error) {
	r, err := openPrivate(home)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	raw, err := readPrivate(r, configFile, 64<<10)
	if err != nil {
		return nil, err
	}
	var d Document
	if decode(raw, &d) != nil || d.Version != 1 || d.Mode != "synthetic" || (d.Role != "validator" && d.Role != "observer") || !strings.HasPrefix(d.Runtime.ChainID, "god-test-") ||
		!d.Runtime.Prototype || d.Runtime.BridgeGenesis != nil || d.Runtime.BridgeApprovalGas != 0 ||
		d.BlockIntervalMillis < 100 || d.BlockIntervalMillis > 6000 || len(d.Peers) > 32 || len(d.GenesisSHA256) != 64 {
		return nil, ErrConfig
	}
	if godnode.ValidateRPCOptions(godnode.RPCOptions{Hosts: d.RPCHosts, Origins: d.RPCOrigins}) != nil {
		return nil, ErrConfig
	}
	valid, exposed := endpoint(d.P2PListen, true, true)
	if !valid {
		return nil, ErrConfig
	}
	valid, rpcExposed := endpoint(d.RPCListen, false, true)
	if !valid || d.Role == "validator" && rpcExposed {
		return nil, ErrConfig
	}
	exposed = exposed || rpcExposed
	seen := map[string]bool{}
	for _, peer := range d.Peers {
		at := strings.IndexByte(peer, '@')
		if at != 40 || seen[peer[:at]] {
			return nil, ErrConfig
		}
		id, e := hex.DecodeString(peer[:at])
		ok, remote := endpoint(peer[at+1:], false, false)
		if e != nil || len(id) != 20 || !ok {
			return nil, ErrConfig
		}
		seen[peer[:at]] = true
		exposed = exposed || remote
	}
	genesisRaw, err := readPrivate(r, genesisFile, 2<<20)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(genesisRaw)
	if hex.EncodeToString(hash[:]) != d.GenesisSHA256 {
		return nil, ErrConfig
	}
	var g cmttypes.GenesisDoc
	if decodeConsensus(genesisRaw, &g) != nil || godnode.ValidateTestnetGenesis(d.Runtime, &g) != nil {
		return nil, ErrConfig
	}
	keyRaw, err := readPrivate(r, keyFile, 4096)
	if err != nil {
		return nil, err
	}
	var k privval.FilePVKey
	if decodeConsensus(keyRaw, &k) != nil {
		return nil, ErrPrivate
	}
	kp, ok := k.PrivKey.(cmted.PrivKey)
	if !ok || len(kp) != 64 || !bytes.Equal(kp, ed25519.NewKeyFromSeed(kp[:32])) ||
		k.PubKey == nil || !k.PubKey.Equals(kp.PubKey()) || !bytes.Equal(k.Address, kp.PubKey().Address()) || !bytes.Equal(d.ConsensusPublicKey, kp.PubKey().Bytes()) {
		return nil, ErrPrivate
	}
	member := false
	for _, v := range g.Validators {
		if v.PubKey.Equals(kp.PubKey()) {
			member = true
		}
	}
	if member != (d.Role == "validator") {
		return nil, ErrConfig
	}
	pv := privval.NewFilePV(kp, filepath.Join(home, keyFile), filepath.Join(home, stateFile))
	stateRaw, err := readPrivate(r, stateFile, 16<<10)
	if err != nil {
		return nil, err
	}
	if decodeConsensus(stateRaw, &pv.LastSignState) != nil {
		return nil, ErrPrivate
	}
	s := pv.LastSignState
	if s.Height < 0 || s.Round < 0 || s.Step < 0 || s.Step > 3 || s.Height == 0 && (s.Round != 0 || s.Step != 0 || len(s.Signature) != 0 || len(s.SignBytes) != 0) || s.Height > 0 && (s.Step == 0 || len(s.Signature) != 64 || len(s.SignBytes) == 0 || len(s.SignBytes) > 4096 || !kp.PubKey().VerifySignature(s.SignBytes, s.Signature)) {
		return nil, ErrPrivate
	}
	peerRaw, err := readPrivate(r, peerFile, 4096)
	if err != nil {
		return nil, err
	}
	var nk p2p.NodeKey
	if decodeConsensus(peerRaw, &nk) != nil {
		return nil, ErrPrivate
	}
	pk, ok := nk.PrivKey.(cmted.PrivKey)
	if !ok || len(pk) != 64 || !bytes.Equal(pk, ed25519.NewKeyFromSeed(pk[:32])) || seen[string(nk.ID())] || !bytes.Equal(d.PeerPublicKey, pk.PubKey().Bytes()) {
		return nil, ErrPrivate
	}
	return &Loaded{d, &g, pv, &nk, Report{Mode: "synthetic", Role: d.Role, Synthetic: true, ValidatorCount: len(g.Validators), NetworkExposure: exposed, Persistent: true}}, nil
}

// CreateLocal provisions a private multi-process development cluster only.
// All operators here share one owner. Distributing its signing keys is NOT a
// decentralized launch; independent operators need their own reviewed setup.
// Genesis funding and G policy are synthetic test settings, not mainnet rules.
func CreateLocal(home string, count, firstPort int) (Report, error) {
	if count < 1 || count > 16 || firstPort < 1024 || firstPort+count*2 > 65535 || !filepath.IsAbs(home) {
		return Report{}, ErrConfig
	}
	parent, err := openPrivate(filepath.Dir(home))
	if err != nil {
		return Report{}, err
	}
	defer parent.Close()
	name := filepath.Base(home)
	if parent.Mkdir(name, 0700) != nil {
		return Report{}, ErrPrivate
	}
	r, err := openPrivate(home)
	if err != nil {
		return Report{}, err
	}
	defer r.Close()
	cfg, err := syntheticConfig()
	if err != nil {
		return Report{}, err
	}
	u := godrewards.Unit()
	state := godnode.Genesis{Version: 1, Prototype: true}
	keys, nks := make([]cmted.PrivKey, count), make([]*p2p.NodeKey, count)
	for i := 0; i < count; i++ {
		owner, err := ethsecp256k1.GenerateKey()
		if err != nil {
			return Report{}, ErrPrivate
		}
		address, err := godaddress.ToNative(owner.PubKey().Address())
		if err != nil {
			return Report{}, ErrPrivate
		}
		keys[i] = cmted.GenPrivKey()
		nks[i] = &p2p.NodeKey{PrivKey: cmted.GenPrivKey()}
		state.Balances = append(state.Balances, godnode.GenesisBalance{Address: address, Amount: u.MulRaw(1100).String()})
		state.Validators = append(state.Validators, godnode.GenesisValidator{Owner: address, PublicKey: keys[i].PubKey().Bytes(), Stake: u.MulRaw(1000).String()})
		// Private development wallets stay at the cluster root, outside every
		// node workspace and RPC process. Never distribute this cluster's keys
		// as a substitute for independently owned validators.
		if writePrivate(r, fmt.Sprintf("operator-%d.key", i+1), owner.Key) != nil {
			return Report{}, ErrPrivate
		}
		clear(owner.Key)
	}
	appState, err := json.Marshal(state)
	if err != nil {
		return Report{}, ErrPrivate
	}
	cp := cmttypes.DefaultConsensusParams()
	cp.Block.MaxGas = cfg.MaxBlockGas
	cp.Block.MaxBytes = cfg.MaxBlockBytes
	cp.Evidence.MaxBytes = 1024
	g := &cmttypes.GenesisDoc{ChainID: cfg.ChainID, GenesisTime: time.Now().UTC(), InitialHeight: 1, ConsensusParams: cp, AppState: appState}
	for _, v := range state.Validators {
		g.Validators = append(g.Validators, cmttypes.GenesisValidator{PubKey: cmted.PubKey(v.PublicKey), Power: 1000})
	}
	if godnode.ValidateTestnetGenesis(cfg, g) != nil {
		return Report{}, ErrConfig
	}
	genesisRaw, err := cmtjson.Marshal(g)
	if err != nil {
		return Report{}, ErrPrivate
	}
	digest := sha256.Sum256(genesisRaw)
	for i := 0; i < count; i++ {
		nodeName := fmt.Sprintf("node-%d", i+1)
		if r.Mkdir(nodeName, 0700) != nil {
			return Report{}, ErrPrivate
		}
		dir := filepath.Join(home, nodeName)
		nr, err := openPrivate(dir)
		if err != nil {
			return Report{}, err
		}
		pv := privval.NewFilePV(keys[i], filepath.Join(dir, keyFile), filepath.Join(dir, stateFile))
		d := Document{Version: 1, Mode: "synthetic", Role: "validator", Runtime: cfg, GenesisSHA256: hex.EncodeToString(digest[:]), ConsensusPublicKey: keys[i].PubKey().Bytes(), PeerPublicKey: nks[i].PubKey().Bytes(),
			P2PListen: fmt.Sprintf("tcp://127.0.0.1:%d", firstPort+i*2), RPCListen: fmt.Sprintf("127.0.0.1:%d", firstPort+i*2+1),
			RPCHosts: []string{"127.0.0.1", "localhost"}, RPCOrigins: []string{}, Peers: []string{}, BlockIntervalMillis: 1000}
		for j := 0; j < count; j++ {
			if i != j {
				d.Peers = append(d.Peers, fmt.Sprintf("%s@127.0.0.1:%d", nks[j].ID(), firstPort+j*2))
			}
		}
		configRaw, e := json.MarshalIndent(d, "", "  ")
		if e != nil {
			nr.Close()
			return Report{}, ErrPrivate
		}
		material := map[string]any{keyFile: pv.Key, stateFile: pv.LastSignState, peerFile: nks[i]}
		for name, value := range material {
			raw, e := cmtjson.Marshal(value)
			if e != nil || writePrivate(nr, name, raw) != nil {
				nr.Close()
				return Report{}, ErrPrivate
			}
		}
		if writePrivate(nr, genesisFile, genesisRaw) != nil || writePrivate(nr, configFile, configRaw) != nil {
			nr.Close()
			return Report{}, ErrPrivate
		}
		nr.Close()
	}
	return Report{Mode: "synthetic", Synthetic: true, ValidatorCount: count, Persistent: true}, nil
}

func syntheticConfig() (godnode.Config, error) {
	var salt [16]byte
	if _, err := rand.Read(salt[:]); err != nil {
		return godnode.Config{}, ErrPrivate
	}
	id := uint64(salt[0])<<24 | uint64(salt[1])<<16 | uint64(salt[2])<<8 | uint64(salt[3])
	return godnode.Config{Prototype: true, ChainID: "god-test-" + hex.EncodeToString(salt[:]), EVMChainID: id + 1<<32,
		Policy:      godtx.Policy{MaxTxBytes: 16384, MaxMessages: 8, MaxGas: 2_000_000, MinFeePerGas: sdkmath.OneInt(), SignatureGas: 21000, MessageGas: 10000},
		MaxBlockGas: 20_000_000, MaxBlockBytes: 1 << 20, MaxBlockTxs: 128, GPerSignedBlock: godrewards.Unit()}, nil
}

func PrivateParent(home string) error {
	// Helper for command callers that explicitly choose a new private parent.
	// It does not create or repair broad/shared directories automatically.
	r, err := openPrivate(home)
	if err != nil {
		return err
	}
	return r.Close()
}
