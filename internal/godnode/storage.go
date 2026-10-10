package godnode

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	cmtdb "github.com/cometbft/cometbft-db"
	cmted "github.com/cometbft/cometbft/crypto/ed25519"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/libs/protoio"
	"github.com/cometbft/cometbft/privval"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
)

// Persistent storage is an isolated local recovery prototype, not an operator
// release. The caller must provision and load its FilePV with BOTH existing
// key and last-sign-state files. Never call LoadFilePVEmptyState, reset signing
// progress or regenerate missing keys as a way to recover this node.
// A stale signing state fails closed; restoring/rolling back backups is not
// made safe by this check. FilePV and the consensus WAL remain responsible for
// same-height signature reuse. No keys or populated paths are supplied here.
func validLocalSigner(a *App, o LocalOptions) bool {
	pv, ok := o.Signer.(*privval.FilePV)
	if !ok || pv == nil {
		return false
	}
	key, ok := pv.Key.PrivKey.(cmted.PrivKey)
	if !ok || len(key) != cmted.PrivateKeySize || pv.Key.PubKey == nil ||
		!bytes.Equal(key, ed25519.NewKeyFromSeed(key[:ed25519.SeedSize])) ||
		!pv.Key.PubKey.Equals(key.PubKey()) || !bytes.Equal(pv.Key.Address, key.PubKey().Address()) {
		return false
	}
	s := pv.LastSignState
	h := a.base.LastBlockHeight()
	floor := signingProgress{}
	if h > 0 {
		ctx, err := a.base.CreateQueryContext(h, false)
		if err != nil {
			return false
		}
		floor, err = a.observedSigning(ctx, key.PubKey().Address())
		if err != nil {
			return false
		}
	}
	if s.Height < floor.Height || s.Height > h+1 || s.Round < 0 || s.Step < 0 || s.Step > 3 ||
		(s.Height == floor.Height && (s.Round < floor.Round || s.Round == floor.Round && s.Step < floor.Step)) {
		return false
	}
	if s.Height == 0 {
		return floor.Height == 0 && s.Round == 0 && s.Step == 0 && len(s.Signature) == 0 && len(s.SignBytes) == 0
	}
	if s.Step == 0 || len(s.Signature) != cmted.SignatureSize || len(s.SignBytes) == 0 || len(s.SignBytes) > 4096 ||
		!key.PubKey().VerifySignature(s.SignBytes, s.Signature) {
		return false
	}
	if s.Step == 1 {
		var p cmtproto.CanonicalProposal
		return protoio.UnmarshalDelimited(s.SignBytes, &p) == nil && p.ChainID == a.config.ChainID &&
			p.Height == s.Height && p.Round == int64(s.Round) && p.Type == cmtproto.ProposalType
	}
	var v cmtproto.CanonicalVote
	typeWant := cmtproto.PrevoteType
	if s.Step == 3 {
		typeWant = cmtproto.PrecommitType
	}
	return protoio.UnmarshalDelimited(s.SignBytes, &v) == nil && v.ChainID == a.config.ChainID &&
		v.Height == s.Height && v.Round == int64(s.Round) && v.Type == typeWant
}

func bindLocalStorage(a *App, o LocalOptions) error {
	info, err := os.Lstat(o.Directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 ||
		(runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) || !validLocalSigner(a, o) {
		return ErrConfig
	}
	// The caller-provided genesis must remain bound even when consensus loads
	// its own stored genesis instead of calling the provider on restart.
	genesis, err := cmtjson.Marshal(o.Genesis)
	if err != nil {
		return ErrGenesis
	}
	genesisHash := sha256.Sum256(genesis)
	pub, err := o.Signer.GetPubKey()
	if err != nil {
		return ErrConfig
	}
	binding, err := json.Marshal(struct {
		Version        uint32
		Config         []byte
		Genesis        [32]byte
		Signer, Peer   []byte
		CandidateOwner string `json:",omitempty"`
	}{1, a.binding, genesisHash, pub.Bytes(), o.NodeKey.PubKey().Bytes(), o.CandidateOwner})
	if err != nil {
		return ErrConfig
	}
	directory := filepath.Join(o.Directory, "data")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return ErrConfig
	}
	if info, err := os.Lstat(directory); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return ErrConfig
	}
	db, err := cmtdb.NewDB("god_local_binding", cmtdb.GoLevelDBBackend, directory)
	if err != nil {
		return ErrConfig
	}
	defer db.Close()
	stored, err := db.Get([]byte{1})
	if err != nil {
		return ErrConfig
	}
	if stored == nil {
		// Never reconstruct a lost identity binding over existing app state.
		if a.base.LastBlockHeight() != 0 || db.SetSync([]byte{1}, binding) != nil {
			return ErrConfig
		}
	} else if !bytes.Equal(stored, binding) {
		return ErrConfig
	}
	return nil
}
