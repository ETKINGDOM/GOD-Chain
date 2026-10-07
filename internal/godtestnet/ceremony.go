//go:build go1.25

package godtestnet

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	cmted "github.com/cometbft/cometbft/crypto/ed25519"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	cmttypes "github.com/cometbft/cometbft/types"
)

const identityFile = "identity.json"
const profileFile = "profile.json"
const bundleFile = "testnet-bundle.json"

// Profiles contain public operator data but still belong in private operational
// storage, not the sanitized source repository. Both node keys prove possession
// over the same domain-separated profile. No private key leaves its operator.
type Profile struct {
	Version            uint32 `json:"version"`
	Role               string `json:"role"`
	Owner              string `json:"owner"`
	ConsensusPublicKey []byte `json:"consensusPublicKey"`
	PeerPublicKey      []byte `json:"peerPublicKey"`
	Endpoint           string `json:"endpoint"`
	ConsensusProof     []byte `json:"consensusProof"`
	PeerProof          []byte `json:"peerProof"`
}
type IdentityOptions struct {
	Role       string   `json:"role"`
	Owner      string   `json:"owner"`
	Endpoint   string   `json:"endpoint"`
	P2PListen  string   `json:"p2pListen"`
	RPCListen  string   `json:"rpcListen"`
	RPCHosts   []string `json:"rpcHosts"`
	RPCOrigins []string `json:"rpcOrigins"`
}
type identity struct {
	Options IdentityOptions `json:"options"`
	Profile Profile         `json:"profile"`
}
type Bundle struct {
	Version       uint32          `json:"version"`
	Mode          string          `json:"mode"`
	Runtime       godnode.Config  `json:"runtime"`
	Genesis       json.RawMessage `json:"genesis"`
	GenesisSHA256 string          `json:"genesisSha256"`
	Profiles      []Profile       `json:"profiles"`
}

func newWorkspace(home string) (*os.Root, error) {
	if !filepath.IsAbs(home) {
		return nil, ErrPrivate
	}
	parent, err := openPrivate(filepath.Dir(home))
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	if parent.Mkdir(filepath.Base(home), 0700) != nil {
		return nil, ErrPrivate
	}
	return openPrivate(home)
}
func profileBytes(p Profile) ([]byte, error) {
	p.ConsensusProof, p.PeerProof = nil, nil
	raw, err := json.Marshal(p)
	return append([]byte("GOD Chain synthetic operator profile v1\n"), raw...), err
}
func validProfile(p Profile) bool {
	ok, _ := endpoint(p.Endpoint, false, false)
	if p.Version != 1 || !ok || (p.Role != "validator" && p.Role != "observer") || len(p.ConsensusPublicKey) != 32 || len(p.PeerPublicKey) != 32 || len(p.ConsensusProof) != 64 || len(p.PeerProof) != 64 || bytes.Equal(p.ConsensusPublicKey, p.PeerPublicKey) {
		return false
	}
	if p.Role == "validator" {
		if _, err := godaddress.FromNative(p.Owner); err != nil {
			return false
		}
	} else if p.Owner != "" {
		return false
	}
	raw, err := profileBytes(p)
	return err == nil && cmted.PubKey(p.ConsensusPublicKey).VerifySignature(raw, p.ConsensusProof) && cmted.PubKey(p.PeerPublicKey).VerifySignature(raw, p.PeerProof)
}

// CreateIdentity is run on the operator's own machine. The account owner is an
// externally held wallet, never a server-held account signing key. The returned
// profile proves node-key possession, not account ownership or decentralization.
func CreateIdentity(home string, options IdentityOptions) (Report, error) {
	if options.Role == "validator" {
		var raw []byte
		var err error
		if strings.HasPrefix(options.Owner, "0x") {
			raw, err = godaddress.FromEVM(options.Owner)
		} else {
			raw, err = godaddress.FromNative(options.Owner)
		}
		if err != nil {
			return Report{}, ErrConfig
		}
		options.Owner, err = godaddress.ToNative(raw)
		if err != nil {
			return Report{}, ErrConfig
		}
	} else if options.Role != "observer" || options.Owner != "" {
		return Report{}, ErrConfig
	}
	ok, exposed := endpoint(options.P2PListen, true, true)
	valid, remote := endpoint(options.RPCListen, false, true)
	advertised, public := endpoint(options.Endpoint, false, false)
	if !ok || !valid || !advertised || options.Role == "validator" && remote || godnode.ValidateRPCOptions(godnode.RPCOptions{Hosts: options.RPCHosts, Origins: options.RPCOrigins}) != nil {
		return Report{}, ErrConfig
	}
	r, err := newWorkspace(home)
	if err != nil {
		return Report{}, err
	}
	defer r.Close()
	key, peer := cmted.GenPrivKey(), cmted.GenPrivKey()
	p := Profile{Version: 1, Role: options.Role, Owner: options.Owner, ConsensusPublicKey: key.PubKey().Bytes(), PeerPublicKey: peer.PubKey().Bytes(), Endpoint: options.Endpoint}
	raw, err := profileBytes(p)
	if err != nil {
		return Report{}, ErrPrivate
	}
	p.ConsensusProof, err = key.Sign(raw)
	if err != nil {
		return Report{}, ErrPrivate
	}
	p.PeerProof, err = peer.Sign(raw)
	if err != nil || !validProfile(p) {
		return Report{}, ErrPrivate
	}
	pv := privval.NewFilePV(key, filepath.Join(home, keyFile), filepath.Join(home, stateFile))
	for name, value := range map[string]any{keyFile: pv.Key, stateFile: pv.LastSignState, peerFile: &p2p.NodeKey{PrivKey: peer}} {
		raw, err := cmtjson.Marshal(value)
		if err != nil || writePrivate(r, name, raw) != nil {
			return Report{}, ErrPrivate
		}
	}
	for name, value := range map[string]any{profileFile: p, identityFile: identity{options, p}} {
		raw, err := json.Marshal(value)
		if err != nil || writePrivate(r, name, raw) != nil {
			return Report{}, ErrPrivate
		}
	}
	return Report{Mode: "synthetic", Role: options.Role, Synthetic: true, NetworkExposure: exposed || remote || public, Persistent: true}, nil
}

func readDocument(path string, limit int64, out any) error {
	if !filepath.IsAbs(path) {
		return ErrPrivate
	}
	r, err := openPrivate(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer r.Close()
	raw, err := readPrivate(r, filepath.Base(path), limit)
	if err != nil {
		return err
	}
	return decode(raw, out)
}

func validateBundle(b *Bundle) (*cmttypes.GenesisDoc, int, error) {
	if b == nil || b.Version != 1 || b.Mode != "synthetic" || !strings.HasPrefix(b.Runtime.ChainID, "god-test-") || len(b.Profiles) < 4 || len(b.Profiles) > 32 {
		return nil, 0, ErrConfig
	}
	hash := sha256.Sum256(b.Genesis)
	if hex.EncodeToString(hash[:]) != b.GenesisSHA256 {
		return nil, 0, ErrConfig
	}
	var g cmttypes.GenesisDoc
	if decodeConsensus(b.Genesis, &g) != nil || godnode.ValidateTestnetGenesis(b.Runtime, &g) != nil {
		return nil, 0, ErrConfig
	}
	var state godnode.Genesis
	if json.Unmarshal(g.AppState, &state) != nil {
		return nil, 0, ErrConfig
	}
	owners, keys, endpoints := map[string]bool{}, map[string]bool{}, map[string]bool{}
	validators := 0
	for _, p := range b.Profiles {
		if !validProfile(p) || keys[string(p.ConsensusPublicKey)] || keys[string(p.PeerPublicKey)] || endpoints[p.Endpoint] {
			return nil, 0, ErrConfig
		}
		keys[string(p.ConsensusPublicKey)], keys[string(p.PeerPublicKey)], endpoints[p.Endpoint] = true, true, true
		if p.Role == "observer" {
			continue
		}
		if owners[p.Owner] {
			return nil, 0, ErrConfig
		}
		owners[p.Owner] = true
		validators++
		found := false
		for _, v := range state.Validators {
			if v.Owner == p.Owner && bytes.Equal(v.PublicKey, p.ConsensusPublicKey) && v.Stake == godrewards.Unit().MulRaw(1000).String() {
				found = true
			}
		}
		funded := false
		for _, balance := range state.Balances {
			if balance.Address == p.Owner && balance.Amount == godrewards.Unit().MulRaw(1100).String() {
				funded = true
			}
		}
		if !found || !funded {
			return nil, 0, ErrConfig
		}
	}
	if validators < 4 || validators > 16 || validators != len(g.Validators) || validators != len(state.Validators) || validators != len(state.Balances) {
		return nil, 0, ErrConfig
	}
	return &g, validators, nil
}

// Assemble produces an immutable synthetic bundle from independently exported
// profiles. Operators must compare its genesis digest over an independent
// trusted channel before joining; a hash alone does not authenticate a launch.
func Assemble(home string, profiles []string) (Report, error) {
	if len(profiles) < 4 || len(profiles) > 32 {
		return Report{}, ErrConfig
	}
	b := Bundle{Version: 1, Mode: "synthetic"}
	for _, path := range profiles {
		var p Profile
		if readDocument(path, 16<<10, &p) != nil || !validProfile(p) {
			return Report{}, ErrConfig
		}
		b.Profiles = append(b.Profiles, p)
	}
	cfg, err := syntheticConfig()
	if err != nil {
		return Report{}, err
	}
	b.Runtime = cfg
	state := godnode.Genesis{Version: 1, Prototype: true}
	cp := cmttypes.DefaultConsensusParams()
	cp.Block.MaxGas, cp.Block.MaxBytes, cp.Evidence.MaxBytes = cfg.MaxBlockGas, cfg.MaxBlockBytes, 1024
	g := cmttypes.GenesisDoc{ChainID: cfg.ChainID, GenesisTime: time.Now().UTC(), InitialHeight: 1, ConsensusParams: cp}
	for _, p := range b.Profiles {
		if p.Role == "observer" {
			continue
		}
		state.Balances = append(state.Balances, godnode.GenesisBalance{Address: p.Owner, Amount: godrewards.Unit().MulRaw(1100).String()})
		state.Validators = append(state.Validators, godnode.GenesisValidator{Owner: p.Owner, PublicKey: p.ConsensusPublicKey, Stake: godrewards.Unit().MulRaw(1000).String()})
		g.Validators = append(g.Validators, cmttypes.GenesisValidator{PubKey: cmted.PubKey(p.ConsensusPublicKey), Power: 1000})
	}
	g.AppState, err = json.Marshal(state)
	if err != nil {
		return Report{}, ErrConfig
	}
	b.Genesis, err = cmtjson.Marshal(g)
	if err != nil {
		return Report{}, ErrConfig
	}
	hash := sha256.Sum256(b.Genesis)
	b.GenesisSHA256 = hex.EncodeToString(hash[:])
	_, count, err := validateBundle(&b)
	if err != nil {
		return Report{}, err
	}
	r, err := newWorkspace(home)
	if err != nil {
		return Report{}, err
	}
	defer r.Close()
	raw, err := json.Marshal(b)
	if err != nil {
		return Report{}, ErrPrivate
	}
	bundleHash := sha256.Sum256(raw)
	if writePrivate(r, bundleFile, raw) != nil || writePrivate(r, "genesis.sha256", []byte(b.GenesisSHA256+"\n")) != nil || writePrivate(r, "bundle.sha256", []byte(hex.EncodeToString(bundleHash[:])+"\n")) != nil {
		return Report{}, ErrPrivate
	}
	return Report{Mode: "synthetic", Synthetic: true, ValidatorCount: count, Persistent: true}, nil
}

func Join(home, bundlePath, expectedBundle string) (Report, error) {
	b, err := loadBundle(bundlePath, expectedBundle)
	if err != nil {
		return Report{}, err
	}
	r, err := openPrivate(home)
	if err != nil {
		return Report{}, err
	}
	defer r.Close()
	for _, name := range []string{configFile, genesisFile, "data"} {
		if _, err := r.Lstat(name); !os.IsNotExist(err) {
			return Report{}, ErrPrivate
		}
	}
	identityRaw, err := readPrivate(r, identityFile, 32<<10)
	var id identity
	if err != nil || decode(identityRaw, &id) != nil || !validProfile(id.Profile) {
		return Report{}, ErrConfig
	}
	matched := false
	for _, p := range b.Profiles {
		left, _ := json.Marshal(p)
		right, _ := json.Marshal(id.Profile)
		if bytes.Equal(left, right) {
			matched = true
		}
	}
	if !matched || id.Options.Role != id.Profile.Role || id.Options.Owner != id.Profile.Owner || id.Options.Endpoint != id.Profile.Endpoint {
		return Report{}, ErrConfig
	}
	p2pOK, _ := endpoint(id.Options.P2PListen, true, true)
	rpcOK, _ := endpoint(id.Options.RPCListen, false, true)
	if !p2pOK || !rpcOK || godnode.ValidateRPCOptions(godnode.RPCOptions{Hosts: id.Options.RPCHosts, Origins: id.Options.RPCOrigins}) != nil || !pendingKeysMatch(r, id.Profile) {
		return Report{}, ErrPrivate
	}
	stateRaw, err := readPrivate(r, stateFile, 16<<10)
	var state privval.FilePVLastSignState
	if err != nil || decodeConsensus(stateRaw, &state) != nil || state.Height != 0 || state.Round != 0 || state.Step != 0 || len(state.SignBytes) != 0 || len(state.Signature) != 0 {
		return Report{}, ErrPrivate
	}
	d := Document{Version: 1, Mode: "synthetic", Role: id.Profile.Role, Runtime: b.Runtime, GenesisSHA256: b.GenesisSHA256, ConsensusPublicKey: id.Profile.ConsensusPublicKey, PeerPublicKey: id.Profile.PeerPublicKey,
		P2PListen: id.Options.P2PListen, RPCListen: id.Options.RPCListen, RPCHosts: id.Options.RPCHosts, RPCOrigins: id.Options.RPCOrigins, Peers: []string{}, BlockIntervalMillis: 1000}
	for _, p := range b.Profiles {
		if !bytes.Equal(p.PeerPublicKey, id.Profile.PeerPublicKey) {
			d.Peers = append(d.Peers, string(p2p.PubKeyToID(cmted.PubKey(p.PeerPublicKey)))+"@"+p.Endpoint)
		}
	}
	raw, err := json.Marshal(d)
	if err != nil || writePrivate(r, genesisFile, b.Genesis) != nil || writePrivate(r, configFile, raw) != nil {
		return Report{}, ErrPrivate
	}
	loaded, err := Load(home)
	if err != nil {
		return Report{}, err
	}
	return loaded.Report, nil
}

func loadBundle(bundlePath, expectedBundle string) (Bundle, error) {
	var b Bundle
	if !filepath.IsAbs(bundlePath) || len(expectedBundle) != 64 {
		return b, ErrConfig
	}
	br, err := openPrivate(filepath.Dir(bundlePath))
	if err != nil {
		return b, err
	}
	bundleRaw, readErr := readPrivate(br, filepath.Base(bundlePath), 2<<20)
	_ = br.Close()
	bundleHash := sha256.Sum256(bundleRaw)
	if readErr != nil || decode(bundleRaw, &b) != nil || hex.EncodeToString(bundleHash[:]) != expectedBundle {
		return b, ErrConfig
	}
	_, _, err = validateBundle(&b)
	if err != nil {
		return b, err
	}
	return b, nil
}

func pendingKeysMatch(r *os.Root, p Profile) bool {
	keyRaw, err := readPrivate(r, keyFile, 4096)
	var key privval.FilePVKey
	if err != nil || decodeConsensus(keyRaw, &key) != nil {
		return false
	}
	k, ok := key.PrivKey.(cmted.PrivKey)
	if !ok || len(k) != 64 || !bytes.Equal(k, ed25519.NewKeyFromSeed(k[:32])) || key.PubKey == nil || !key.PubKey.Equals(k.PubKey()) || !bytes.Equal(key.Address, k.PubKey().Address()) || !bytes.Equal(k.PubKey().Bytes(), p.ConsensusPublicKey) {
		return false
	}
	peerRaw, err := readPrivate(r, peerFile, 4096)
	var nk p2p.NodeKey
	if err != nil || decodeConsensus(peerRaw, &nk) != nil {
		return false
	}
	k, ok = nk.PrivKey.(cmted.PrivKey)
	return ok && len(k) == 64 && bytes.Equal(k, ed25519.NewKeyFromSeed(k[:32])) && bytes.Equal(k.PubKey().Bytes(), p.PeerPublicKey)
}
