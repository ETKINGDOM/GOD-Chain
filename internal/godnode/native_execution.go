package godnode

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"

	abci "github.com/cometbft/cometbft/abci/types"
	cmted "github.com/cometbft/cometbft/crypto/ed25519"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmtlocal "github.com/cometbft/cometbft/rpc/client/local"
	cmttypes "github.com/cometbft/cometbft/types"
)

const MaxNativeExecutionTransactions = 128
const MaxNativeExecutionMaterialBytes = 3 << 20

var ErrNativeExecution = errors.New("conditional native execution review unavailable or rejected")

type NativeExecutionValidator struct {
	PublicKey [32]byte
	Power     int64
}

// A separately reviewed exact confirmation header and its validator set.
// Origin/freshness and validator-set transitions are NOT discovered here.
// Never derive this trust from the same unreviewed receipt provider.
type NativeExecutionTrust struct {
	ChainID            string
	ConfirmationHeight int64
	ConfirmationHash   [32]byte
	Validators         []NativeExecutionValidator
}

func (NativeExecutionTrust) String() string               { return "conditional native execution trust (redacted)" }
func (t NativeExecutionTrust) GoString() string           { return t.String() }
func (NativeExecutionTrust) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

// Canonical protocol bytes only. Results contain the four consensus-committed
// fields, never Log, Info, Events or Codespace. Ordinary rendering is redacted.
type NativeExecutionMaterial struct {
	TransactionBlock   []byte
	ConfirmationHeader []byte
	ConfirmationCommit []byte
	Results            [][]byte
}

func (NativeExecutionMaterial) String() string               { return "private native execution material (redacted)" }
func (m NativeExecutionMaterial) GoString() string           { return m.String() }
func (NativeExecutionMaterial) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type NativeExecutionReport struct {
	SimulationOnly              bool `json:"simulationOnly"`
	ConditionalInclusionMatched bool `json:"conditionalInclusionMatched"`
	ExactEnvelopeMatched        bool `json:"exactEnvelopeMatched"`
	ResultCommitmentMatched     bool `json:"resultCommitmentMatched"`
	QuorumSignaturesMatched     bool `json:"quorumSignaturesMatched"`
	// This is outer SDK code zero, not a general VM-success verdict. The RH
	// consumer separately admits only its exact native bridge message.
	ExecutionSucceeded            bool   `json:"executionSucceeded"`
	ExecutionCode                 uint32 `json:"executionCode"`
	Height                        int64  `json:"height,string"`
	ConfirmationHeight            int64  `json:"confirmationHeight,string"`
	Index                         uint32 `json:"index"`
	TrustOriginAuthenticated      bool   `json:"trustOriginAuthenticated"`
	NativeStateProven             bool   `json:"nativeStateProven"`
	IndependentSourceFinality     bool   `json:"independentSourceFinalityVerified"`
	FinancialResolutionAuthorized bool   `json:"financialResolutionAuthorized"`
	RealAssetsReady               bool   `json:"realAssetsReady"`
}

type NativeExecutionReview struct {
	report          NativeExecutionReport
	material, trust [32]byte
}

func (NativeExecutionReview) String() string                  { return "conditional native execution review (redacted)" }
func (r NativeExecutionReview) GoString() string              { return r.String() }
func (r NativeExecutionReview) MarshalJSON() ([]byte, error)  { return json.Marshal(r.Report()) }
func (r NativeExecutionReview) Report() NativeExecutionReport { return r.report }

// Explicit private retention identities, not new trust anchors or proofs.
func (r NativeExecutionReview) RetainedIdentities() ([32]byte, [32]byte, error) {
	if !r.report.ConditionalInclusionMatched {
		return [32]byte{}, [32]byte{}, ErrNativeExecution
	}
	return r.material, r.trust, nil
}

func nativeExecutionMaterial(m NativeExecutionMaterial) (NativeExecutionMaterial, error) {
	if len(m.TransactionBlock) == 0 || len(m.TransactionBlock) > 2<<20 || len(m.ConfirmationHeader) == 0 || len(m.ConfirmationHeader) > 16<<10 ||
		len(m.ConfirmationCommit) == 0 || len(m.ConfirmationCommit) > 16<<10 || len(m.Results) == 0 || len(m.Results) > MaxNativeExecutionTransactions {
		return NativeExecutionMaterial{}, ErrNativeExecution
	}
	total := len(m.TransactionBlock) + len(m.ConfirmationHeader) + len(m.ConfirmationCommit)
	for _, r := range m.Results {
		if len(r) == 0 || len(r) > 64<<10 {
			return NativeExecutionMaterial{}, ErrNativeExecution
		}
		total += len(r)
	}
	if total > MaxNativeExecutionMaterialBytes {
		return NativeExecutionMaterial{}, ErrNativeExecution
	}
	out := NativeExecutionMaterial{bytes.Clone(m.TransactionBlock), bytes.Clone(m.ConfirmationHeader), bytes.Clone(m.ConfirmationCommit), make([][]byte, len(m.Results))}
	for i, r := range m.Results {
		out.Results[i] = bytes.Clone(r)
	}
	return out, nil
}

func nativeExecutionTrust(t NativeExecutionTrust) (*cmttypes.ValidatorSet, [32]byte, error) {
	if len(t.ChainID) == 0 || len(t.ChainID) > 50 || t.ConfirmationHeight < 2 || t.ConfirmationHash == [32]byte{} || len(t.Validators) == 0 || len(t.Validators) > 32 {
		return nil, [32]byte{}, ErrNativeExecution
	}
	for _, c := range t.ChainID {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return nil, [32]byte{}, ErrNativeExecution
		}
	}
	seen, total := map[[32]byte]bool{}, int64(0)
	vs := make([]*cmttypes.Validator, 0, len(t.Validators))
	for _, v := range t.Validators {
		if v.PublicKey == [32]byte{} || seen[v.PublicKey] || v.Power <= 0 || v.Power > cmttypes.MaxTotalVotingPower-total {
			return nil, [32]byte{}, ErrNativeExecution
		}
		seen[v.PublicKey], total = true, total+v.Power
		vs = append(vs, cmttypes.NewValidator(cmted.PubKey(bytes.Clone(v.PublicKey[:])), v.Power))
	}
	set := cmttypes.NewValidatorSet(vs)
	// Canonical order follows the consensus validator set, not caller order.
	canonical := make([]NativeExecutionValidator, len(set.Validators))
	for i, v := range set.Validators {
		copy(canonical[i].PublicKey[:], v.PubKey.Bytes())
		canonical[i].Power = v.VotingPower
	}
	raw, err := json.Marshal(struct {
		Domain, Chain string
		Height        int64
		Hash          [32]byte
		Validators    []NativeExecutionValidator
	}{"GOD Chain conditional native execution trust v1", t.ChainID, t.ConfirmationHeight, t.ConfirmationHash, canonical})
	if err != nil {
		return nil, [32]byte{}, ErrNativeExecution
	}
	return set, sha256.Sum256(raw), nil
}

func nativeExecutionMaterialPin(m NativeExecutionMaterial) [32]byte {
	h := sha256.New()
	_, _ = h.Write([]byte("GOD Chain conditional native execution material v1"))
	put := func(b []byte) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(b)))
		_, _ = h.Write(size[:])
		_, _ = h.Write(b)
	}
	put(m.TransactionBlock)
	put(m.ConfirmationHeader)
	put(m.ConfirmationCommit)
	var count [8]byte
	binary.BigEndian.PutUint64(count[:], uint64(len(m.Results)))
	_, _ = h.Write(count[:])
	for _, b := range m.Results {
		put(b)
	}
	var result [32]byte
	copy(result[:], h.Sum(nil))
	return result
}

// Binds the exact outer wire and indexed deterministic result to a subsequent
// signed header. Code zero is not inferred from an event, current withdrawal
// status, mempool acknowledgment or another transaction. This is conditional
// on the supplied native trust; it authenticates neither its origin nor RH.
func ReviewNativeExecution(ctx context.Context, wire []byte, expectedGas uint64, trust NativeExecutionTrust, material NativeExecutionMaterial, index uint32) (review NativeExecutionReview, err error) {
	defer func() {
		if recover() != nil {
			review, err = NativeExecutionReview{}, ErrNativeExecution
		}
	}()
	bad := func() (NativeExecutionReview, error) { return NativeExecutionReview{}, ErrNativeExecution }
	if ctx == nil || ctx.Err() != nil || len(wire) == 0 || len(wire) > 16<<10 || expectedGas == 0 || expectedGas > 2_000_000 {
		return bad()
	}
	wire = bytes.Clone(wire)
	trust.Validators = append([]NativeExecutionValidator{}, trust.Validators...)
	validators, trustPin, err := nativeExecutionTrust(trust)
	if err != nil {
		return bad()
	}
	m, err := nativeExecutionMaterial(material)
	if err != nil {
		return bad()
	}
	var blockProto cmtproto.Block
	if blockProto.Unmarshal(m.TransactionBlock) != nil {
		return bad()
	}
	canonical, err := blockProto.Marshal()
	if err != nil || !bytes.Equal(canonical, m.TransactionBlock) {
		return bad()
	}
	block, err := cmttypes.BlockFromProto(&blockProto)
	if err != nil || block.ValidateBasic() != nil || block.ChainID != trust.ChainID || block.Height != trust.ConfirmationHeight-1 ||
		len(block.Txs) != len(m.Results) || uint64(index) >= uint64(len(block.Txs)) || !bytes.Equal(block.Txs[index], wire) || !bytes.Equal(block.DataHash, block.Txs.Hash()) {
		return bad()
	}
	matches := 0
	for _, tx := range block.Txs {
		if bytes.Equal(tx, wire) {
			matches++
		}
	}
	if matches != 1 {
		return bad()
	}
	var headerProto cmtproto.Header
	var commitProto cmtproto.Commit
	if headerProto.Unmarshal(m.ConfirmationHeader) != nil || commitProto.Unmarshal(m.ConfirmationCommit) != nil {
		return bad()
	}
	hraw, e := headerProto.Marshal()
	craw, f := commitProto.Marshal()
	if e != nil || f != nil || !bytes.Equal(hraw, m.ConfirmationHeader) || !bytes.Equal(craw, m.ConfirmationCommit) {
		return bad()
	}
	header, err := cmttypes.HeaderFromProto(&headerProto)
	if err != nil {
		return bad()
	}
	commit, err := cmttypes.CommitFromProto(&commitProto)
	if err != nil || header.ValidateBasic() != nil || (cmttypes.SignedHeader{Header: &header, Commit: commit}).ValidateBasic(trust.ChainID) != nil ||
		header.Height != trust.ConfirmationHeight || !bytes.Equal(header.Hash(), trust.ConfirmationHash[:]) || !bytes.Equal(header.LastBlockID.Hash, block.Header.Hash()) ||
		!header.Time.After(block.Time) || len(header.AppHash) != sha256.Size || !bytes.Equal(header.ValidatorsHash, validators.Hash()) || len(commit.Signatures) != len(validators.Validators) {
		return bad()
	}
	if validators.VerifyCommitLightAllSignatures(trust.ChainID, commit.BlockID, header.Height, commit) != nil {
		return bad()
	}
	results := make([]*abci.ExecTxResult, len(m.Results))
	for i, raw := range m.Results {
		if ctx.Err() != nil {
			return bad()
		}
		r := new(abci.ExecTxResult)
		if r.Unmarshal(raw) != nil {
			return bad()
		}
		deterministic := cmttypes.NewResults([]*abci.ExecTxResult{r})[0]
		canonical, e := deterministic.Marshal()
		if e != nil || !bytes.Equal(raw, canonical) {
			return bad()
		}
		results[i] = deterministic
	}
	if !bytes.Equal(header.LastResultsHash, cmttypes.NewResults(results).Hash()) {
		return bad()
	}
	r := results[index]
	if r.GasWanted < 0 || uint64(r.GasWanted) > expectedGas || r.GasUsed < 0 || r.Code == 0 && (uint64(r.GasWanted) != expectedGas || r.GasUsed > r.GasWanted) || ctx.Err() != nil {
		return bad()
	}
	return NativeExecutionReview{NativeExecutionReport{SimulationOnly: true, ConditionalInclusionMatched: true, ExactEnvelopeMatched: true,
		ResultCommitmentMatched: true, QuorumSignaturesMatched: true, ExecutionSucceeded: r.Code == 0, ExecutionCode: r.Code,
		Height: block.Height, ConfirmationHeight: header.Height, Index: index}, nativeExecutionMaterialPin(m), trustPin}, nil
}

// Private in-process export from an already running synthetic node. No listener,
// tx-index search, provider, latest fallback, signer or submission is created.
// The caller supplies independent trust to review it, never trusts an export's
// hash simply because the same node supplied it. Two later blocks must exist.
func (n *LocalNode) ExportNativeExecution(ctx context.Context, height int64) (material NativeExecutionMaterial, err error) {
	defer func() {
		if recover() != nil {
			material, err = NativeExecutionMaterial{}, ErrNativeExecution
		}
	}()
	bad := func() (NativeExecutionMaterial, error) { return NativeExecutionMaterial{}, ErrNativeExecution }
	if ctx == nil || ctx.Err() != nil || n == nil || n.node == nil || n.app == nil || !n.app.config.Prototype || height <= 0 || height > math.MaxInt64-2 {
		return bad()
	}
	view, err := n.app.QueryNetwork(0)
	// The following block carries a commit for the confirmation header. Its
	// own SDK commit may still be in progress; only height+1 must already be
	// committed locally. Requiring height+2 would race block-store publication.
	if err != nil || view.Commit.Height < height+1 {
		return bad()
	}
	block := n.node.BlockStore().LoadBlock(height)
	confirmation := n.node.BlockStore().LoadBlock(height + 1)
	following := n.node.BlockStore().LoadBlock(height + 2)
	if block == nil || confirmation == nil || following == nil || following.LastCommit == nil || len(block.Txs) == 0 || len(block.Txs) > MaxNativeExecutionTransactions {
		return bad()
	}
	results, err := cmtlocal.New(n.node).BlockResults(ctx, &height)
	if err != nil || results == nil || len(results.TxsResults) != len(block.Txs) || ctx.Err() != nil {
		return bad()
	}
	bp, err := block.ToProto()
	if err != nil {
		return bad()
	}
	material.TransactionBlock, err = bp.Marshal()
	if err != nil {
		return bad()
	}
	hp := confirmation.Header.ToProto()
	material.ConfirmationHeader, err = hp.Marshal()
	if err != nil {
		return bad()
	}
	cp := following.LastCommit.ToProto()
	material.ConfirmationCommit, err = cp.Marshal()
	if err != nil {
		return bad()
	}
	for _, r := range results.TxsResults {
		if r == nil || ctx.Err() != nil {
			return bad()
		}
		raw, err := cmttypes.NewResults([]*abci.ExecTxResult{r})[0].Marshal()
		if err != nil {
			return bad()
		}
		material.Results = append(material.Results, raw)
	}
	if ctx.Err() != nil {
		return bad()
	}
	return nativeExecutionMaterial(material)
}
