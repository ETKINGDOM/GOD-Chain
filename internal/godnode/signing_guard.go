package godnode

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"sync"

	cmtcrypto "github.com/cometbft/cometbft/crypto"
	cmted "github.com/cometbft/cometbft/crypto/ed25519"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	"github.com/cometbft/cometbft/libs/protoio"
	"github.com/cometbft/cometbft/privval"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
)

var ErrSigningGuard = errors.New("retained signing guard rejected")

// GuardedSigner retains a second progress witness outside the node home and
// holds a public-key-wide cooperating-process lease in that directory. It is
// not a remote signer, host-owner defence or permission to restore both copies.
// The supplied FilePV must be exclusively owned by this adapter until Close.
type GuardedSigner struct {
	mu             sync.Mutex
	pv             *privval.FilePV
	record         signingGuardRecord
	store          signingGuardStore
	failed, closed bool
}

type signingGuardRecord struct {
	Version uint32   `json:"version"`
	Key     [32]byte `json:"key"`
	Chain   string   `json:"chain"`
	Genesis [32]byte `json:"genesis"`
	State   [32]byte `json:"state"`
}

// SigningGuardFiles names the original private FilePV files. The guarded node
// workflow binds these explicitly; a bare OpenSigningGuard checks only memory
// and the separate witness, not the original on-disk key/progress files.
type SigningGuardFiles struct {
	Home      string
	KeyFile   string
	StateFile string
}

type signingGuardStore interface {
	check(signingGuardRecord) error
	advance(signingGuardRecord, signingGuardRecord) error
	close() error
}

func guardRecord(chain string, genesis [32]byte, pv *privval.FilePV) (signingGuardRecord, error) {
	if len(chain) == 0 || len(chain) > 50 || genesis == ([32]byte{}) || pv == nil {
		return signingGuardRecord{}, ErrSigningGuard
	}
	for _, b := range []byte(chain) {
		if b < 33 || b > 126 {
			return signingGuardRecord{}, ErrSigningGuard
		}
	}
	k, ok := pv.Key.PrivKey.(cmted.PrivKey)
	if !ok || len(k) != ed25519.PrivateKeySize || !bytes.Equal(k, ed25519.NewKeyFromSeed(k[:32])) ||
		pv.Key.PubKey == nil || !pv.Key.PubKey.Equals(k.PubKey()) || !bytes.Equal(pv.Key.Address, k.PubKey().Address()) || !validGuardState(chain, pv) {
		return signingGuardRecord{}, ErrSigningGuard
	}
	raw, err := cmtjson.Marshal(pv.LastSignState)
	if err != nil {
		return signingGuardRecord{}, ErrSigningGuard
	}
	return signingGuardRecord{1, sha256.Sum256(pv.Key.PubKey.Bytes()), chain, genesis, sha256.Sum256(raw)}, nil
}

func validGuardState(chain string, pv *privval.FilePV) bool {
	s := pv.LastSignState
	if s.Height == 0 {
		return s.Round == 0 && s.Step == 0 && len(s.Signature) == 0 && len(s.SignBytes) == 0
	}
	if s.Height < 0 || s.Round < 0 || s.Step < 1 || s.Step > 3 || len(s.SignBytes) == 0 || len(s.SignBytes) > 4096 || len(s.Signature) != 64 || !pv.Key.PubKey.VerifySignature(s.SignBytes, s.Signature) {
		return false
	}
	if s.Step == 1 {
		var p cmtproto.CanonicalProposal
		return protoio.UnmarshalDelimited(s.SignBytes, &p) == nil && p.ChainID == chain && p.Height == s.Height && p.Round == int64(s.Round) && p.Type == cmtproto.ProposalType
	}
	var v cmtproto.CanonicalVote
	want := cmtproto.PrevoteType
	if s.Step == 3 {
		want = cmtproto.PrecommitType
	}
	return protoio.UnmarshalDelimited(s.SignBytes, &v) == nil && v.ChainID == chain && v.Height == s.Height && v.Round == int64(s.Round) && v.Type == want
}

func (g *GuardedSigner) GetPubKey() (cmtcrypto.PubKey, error) {
	if g == nil {
		return nil, ErrSigningGuard
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed || g.failed {
		return nil, ErrSigningGuard
	}
	return g.pv.GetPubKey()
}

func (g *GuardedSigner) matches(chain string, genesis [32]byte) bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return !g.closed && !g.failed && g.record.Chain == chain && g.record.Genesis == genesis && g.current() == nil
}

func (g *GuardedSigner) current() error {
	r, err := guardRecord(g.record.Chain, g.record.Genesis, g.pv)
	if err != nil || r != g.record || g.store.check(g.record) != nil {
		return ErrSigningGuard
	}
	return nil
}

// Check revalidates retained progress without signing, writing or releasing the
// lease. File-bound guards also recheck the original private key/state files.
// This is point-in-time evidence, not permission to restart or restore a backup.
func (g *GuardedSigner) Check() (err error) {
	if g == nil {
		return ErrSigningGuard
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	defer func() {
		if recover() != nil {
			g.failed, err = true, ErrSigningGuard
		}
	}()
	if g.closed || g.failed {
		return ErrSigningGuard
	}
	if g.current() != nil {
		g.failed = true
		return ErrSigningGuard
	}
	return nil
}

// Persist before publishing any signature into the caller's message. A panic,
// I/O failure or uncertain write poisons this instance; no repair or retry is
// inferred. A crash between the FilePV and witness writes refuses later reopen.
func (g *GuardedSigner) sign(chain string, action func() error) (err error) {
	if g.closed || g.failed || chain != g.record.Chain {
		return ErrSigningGuard
	}
	defer func() {
		if recover() != nil {
			g.failed = true
			err = ErrSigningGuard
		}
	}()
	if g.current() != nil {
		g.failed = true
		return ErrSigningGuard
	}
	if action() != nil {
		if g.current() != nil {
			g.failed = true
		}
		return ErrSigningGuard
	}
	next, err := guardRecord(chain, g.record.Genesis, g.pv)
	if err != nil {
		g.failed = true
		return ErrSigningGuard
	}
	if next != g.record {
		if g.store.advance(g.record, next) != nil {
			g.failed = true
			return ErrSigningGuard
		}
		g.record = next
	}
	if g.current() != nil {
		g.failed = true
		return ErrSigningGuard
	}
	return nil
}

func (g *GuardedSigner) SignVote(chain string, vote *cmtproto.Vote) error {
	if g == nil || vote == nil {
		return ErrSigningGuard
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	vote.Signature, vote.ExtensionSignature = nil, nil
	if vote.Height <= 0 || vote.Round < 0 || (vote.Type != cmtproto.PrevoteType && vote.Type != cmtproto.PrecommitType) || len(vote.Extension) != 0 || !guardBlock(vote.BlockID, true) || len(vote.ValidatorAddress) > 20 || vote.ValidatorIndex < 0 {
		return ErrSigningGuard
	}
	work := *vote
	if err := g.sign(chain, func() error { return g.pv.SignVote(chain, &work) }); err != nil {
		return err
	}
	// Vote extensions are disabled in this runtime. Do not expose even the
	// upstream signature over an empty extension as an extra signing capability.
	vote.Timestamp, vote.Signature = work.Timestamp, append([]byte(nil), work.Signature...)
	return nil
}

func (g *GuardedSigner) SignProposal(chain string, proposal *cmtproto.Proposal) error {
	if g == nil || proposal == nil {
		return ErrSigningGuard
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	proposal.Signature = nil
	if proposal.Height <= 0 || proposal.Round < 0 || proposal.Type != cmtproto.ProposalType || proposal.PolRound < -1 || proposal.PolRound >= proposal.Round || !guardBlock(proposal.BlockID, false) {
		return ErrSigningGuard
	}
	work := *proposal
	if err := g.sign(chain, func() error { return g.pv.SignProposal(chain, &work) }); err != nil {
		return err
	}
	proposal.Timestamp, proposal.Signature = work.Timestamp, append([]byte(nil), work.Signature...)
	return nil
}

func guardBlock(id cmtproto.BlockID, allowNil bool) bool {
	if len(id.Hash) > 32 || len(id.PartSetHeader.Hash) > 32 {
		return false
	}
	b, err := cmttypes.BlockIDFromProto(&id)
	return err == nil && (b.IsComplete() || allowNil && b.IsZero())
}

// Close must follow engine shutdown and worker joins, never race a live node.
func (g *GuardedSigner) Close() error {
	if g == nil {
		return ErrSigningGuard
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return nil
	}
	g.closed = true
	if g.store.close() != nil {
		return ErrSigningGuard
	}
	return nil
}

func fileSigner(signer interface{}) *privval.FilePV {
	switch v := signer.(type) {
	case *privval.FilePV:
		return v
	case *GuardedSigner:
		if v != nil {
			return v.pv
		}
	}
	return nil
}
