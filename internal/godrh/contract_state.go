package godrh

import (
	"bytes"
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

const (
	MaxContractStateSlots      = 8
	MaxContractStateProofBytes = 256 << 10
	MaxParentStateContracts    = 4
)

var ErrContractStateProof = errors.New("conditional contract state proof rejected; no permission, RH finality or asset approval")

// Slot keys and exact words are retained review inputs. This verifier does NOT
// infer their Solidity layout, role, proxy semantics or permission meaning.
type ContractStorageBinding struct{ Key, Value [32]byte }
type ContractStateBinding struct {
	Address     [20]byte
	RuntimeHash [32]byte
	Slots       []ContractStorageBinding
}

func (ContractStorageBinding) String() string               { return "contract storage binding (redacted)" }
func (b ContractStorageBinding) GoString() string           { return b.String() }
func (ContractStorageBinding) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }
func (ContractStateBinding) String() string                 { return "contract state binding (redacted)" }
func (b ContractStateBinding) GoString() string             { return b.String() }
func (ContractStateBinding) MarshalJSON() ([]byte, error)   { return []byte(`{"redacted":true}`), nil }

// Strict root-to-leaf account/storage MPT nodes, canonical execution header
// and exact runtime bytes. StorageNodes correspond one-for-one to bound Slots.
// An absent slot is zero only after a complete non-inclusion proof (or the
// authenticated empty storage root); an empty list at a nonempty root fails.
type ContractStateProof struct {
	Header       []byte
	AccountNodes [][]byte
	Runtime      []byte
	StorageNodes [][][]byte
}

func (ContractStateProof) String() string               { return "contract state proof (redacted)" }
func (p ContractStateProof) GoString() string           { return p.String() }
func (ContractStateProof) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type ContractStateReport struct {
	HeaderMatched               bool `json:"headerMatched"`
	ContractAccountIncluded     bool `json:"contractAccountIncluded"`
	CanonicalAccountEncoding    bool `json:"canonicalAccountEncoding"`
	RuntimeMatchesAccountAndPin bool `json:"runtimeMatchesAccountAndPin"`
	ExactStorageBindingsChecked int  `json:"exactStorageBindingsChecked"`
	StorageLayoutReviewed       bool `json:"storageLayoutReviewed"`
	ContractPermissionsReviewed bool `json:"contractPermissionsReviewed"`
	IndependentHeaderVerified   bool `json:"independentHeaderVerified"`
	SourceChainIdentityVerified bool `json:"sourceChainIdentityVerified"`
	RHSourceFinalityVerified    bool `json:"rhSourceFinalityVerified"`
	SigningEnabled              bool `json:"signingEnabled"`
	BroadcastEnabled            bool `json:"broadcastEnabled"`
	RealAssetsReady             bool `json:"realAssetsReady"`
}

func detachContractState(b ContractStateBinding, p ContractStateProof) (ContractStateBinding, ContractStateProof, error) {
	fail := func() (ContractStateBinding, ContractStateProof, error) {
		return ContractStateBinding{}, ContractStateProof{}, ErrContractStateProof
	}
	if b.Address == [20]byte{} || b.RuntimeHash == [32]byte{} || b.RuntimeHash == [32]byte(types.EmptyCodeHash) ||
		len(b.Slots) > MaxContractStateSlots || len(p.StorageNodes) != len(b.Slots) || len(p.Runtime) == 0 || len(p.Runtime) > maxCodeBytes {
		return fail()
	}
	seen := map[[32]byte]bool{}
	for _, slot := range b.Slots {
		if seen[slot.Key] {
			return fail()
		}
		seen[slot.Key] = true
	}
	a, err := detachCustodyAccountProof(CustodyAccountProof{Header: p.Header, AccountNodes: p.AccountNodes})
	if err != nil {
		return fail()
	}
	total := len(p.Header) + len(p.Runtime)
	for _, node := range p.AccountNodes {
		total += len(node)
	}
	for _, nodes := range p.StorageNodes {
		if len(nodes) > MaxCustodyAccountProofNodes {
			return fail()
		}
		for _, node := range nodes {
			if len(node) == 0 || len(node) > maxCustodyAccountNodeBytes || len(node) > MaxContractStateProofBytes-total {
				return fail()
			}
			total += len(node)
		}
	}
	if total > MaxContractStateProofBytes {
		return fail()
	}
	b.Slots = append([]ContractStorageBinding(nil), b.Slots...)
	owned := ContractStateProof{Header: a.Header, AccountNodes: a.AccountNodes, Runtime: bytes.Clone(p.Runtime), StorageNodes: make([][][]byte, len(p.StorageNodes))}
	for i, nodes := range p.StorageNodes {
		owned.StorageNodes[i] = make([][]byte, len(nodes))
		for j, node := range nodes {
			owned.StorageNodes[i][j] = bytes.Clone(node)
		}
	}
	return b, owned, nil
}

func contractStorageWord(ctx context.Context, root common.Hash, key [32]byte, nodes [][]byte) ([32]byte, error) {
	var word [32]byte
	fail := func() ([32]byte, error) { return [32]byte{}, ErrContractStateProof }
	if ctx.Err() != nil || root == (common.Hash{}) {
		return fail()
	}
	if root == types.EmptyRootHash {
		if len(nodes) != 0 {
			return fail()
		}
		return word, nil
	}
	if len(nodes) == 0 {
		return fail()
	}
	reader := inclusionProofReader{ctx: ctx, nodes: nodes}
	seen := map[common.Hash]bool{}
	for i, node := range nodes {
		if ctx.Err() != nil || i > 0 && len(node) < 32 || !validInclusionNode(node, 0) {
			return fail()
		}
		h := ethcrypto.Keccak256Hash(node)
		if seen[h] {
			return fail()
		}
		seen[h] = true
		reader.hashes = append(reader.hashes, h)
	}
	value, err := trie.VerifyProof(root, ethcrypto.Keccak256(key[:]), &reader)
	if err != nil || reader.next != len(nodes) || ctx.Err() != nil {
		return fail()
	}
	if value == nil {
		return word, nil
	}
	var scalar []byte
	if rlp.DecodeBytes(value, &scalar) != nil || len(scalar) == 0 || len(scalar) > 32 || scalar[0] == 0 {
		return fail()
	}
	canonical, err := rlp.EncodeToBytes(scalar)
	if err != nil || !bytes.Equal(canonical, value) {
		return fail()
	}
	copy(word[32-len(scalar):], scalar)
	return word, nil
}

func checkContractState(ctx context.Context, block Block, b ContractStateBinding, p ContractStateProof) (ContractStateReport, error) {
	if ctx == nil || ctx.Err() != nil || !validDiscoveryBlock(block) {
		return ContractStateReport{}, ErrContractStateProof
	}
	a, err := custodyAccountValue(ctx, block, CustodyAccountProof{Header: p.Header, AccountNodes: p.AccountNodes}, b.Address)
	if err != nil || !bytes.Equal(a.CodeHash, b.RuntimeHash[:]) || tokenRuntimeDigest(p.Runtime) != b.RuntimeHash {
		return ContractStateReport{}, ErrContractStateProof
	}
	for i, slot := range b.Slots {
		word, err := contractStorageWord(ctx, a.Root, slot.Key, p.StorageNodes[i])
		if err != nil || word != slot.Value {
			return ContractStateReport{}, ErrContractStateProof
		}
	}
	if ctx.Err() != nil {
		return ContractStateReport{}, ErrContractStateProof
	}
	return ContractStateReport{HeaderMatched: true, ContractAccountIncluded: true, CanonicalAccountEncoding: true,
		RuntimeMatchesAccountAndPin: true, ExactStorageBindingsChecked: len(b.Slots)}, nil
}

// Conditional on a separately retained block/address/code/slot pin. Header
// consistency is NOT independent source identity, freshness or finality. No
// provider, store, signer, journal or financial capability is exposed.
func VerifyContractStateProof(ctx context.Context, block Block, b ContractStateBinding, p ContractStateProof) (report ContractStateReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = ContractStateReport{}, ErrContractStateProof
		}
	}()
	b, p, err = detachContractState(b, p) // before ANY caller context callback
	if err != nil {
		return ContractStateReport{}, ErrContractStateProof
	}
	return checkContractState(ctx, block, b, p)
}

type ParentDeploymentStateReport struct {
	ParentFinalityReport
	ContractsChecked                   int  `json:"contractsChecked"`
	ExactStorageBindingsChecked        int  `json:"exactStorageBindingsChecked"`
	AccountsBoundToParentExecutionRoot bool `json:"accountsBoundToParentExecutionRoot"`
	DeploymentIdentityReviewed         bool `json:"deploymentIdentityReviewed"`
	StorageLayoutReviewed              bool `json:"storageLayoutReviewed"`
	ContractPermissionsReviewed        bool `json:"contractPermissionsReviewed"`
}

// Verifies account/code/slot proofs against the EXACT execution header inside
// the conditional parent finality proof, also matching a separate block pin.
// next may only prove the original anchor's immediately adjacent committee.
// This is not Nitro assertion/batch provenance, RH execution or RH finality.
// Callers must not concurrently mutate inputs during entry/copy.
func VerifyParentDeploymentState(ctx context.Context, t ParentFinalityTrust, parent ParentFinalityProof, next *ParentNextCommitteeProof, block Block, bindings []ContractStateBinding, proofs []ContractStateProof) (report ParentDeploymentStateReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = ParentDeploymentStateReport{}, ErrContractStateProof
		}
	}()
	fail := func() (ParentDeploymentStateReport, error) {
		return ParentDeploymentStateReport{}, ErrContractStateProof
	}
	if len(bindings) == 0 || len(bindings) > MaxParentStateContracts || len(proofs) != len(bindings) ||
		len(parent.FinalizedExecutionHeader) == 0 || len(parent.FinalizedExecutionHeader) > maxSetHeaderBytes ||
		len(parent.CommitteeBranch) > 6 || len(parent.FinalityBranch) > 7 || len(parent.ExecutionHashBranch) != 9 || next != nil && len(next.Branch) > 6 {
		return fail()
	}
	parent.FinalizedExecutionHeader = bytes.Clone(parent.FinalizedExecutionHeader)
	parent.CommitteeBranch = append([][32]byte(nil), parent.CommitteeBranch...)
	parent.FinalityBranch = append([][32]byte(nil), parent.FinalityBranch...)
	parent.ExecutionHashBranch = append([][32]byte(nil), parent.ExecutionHashBranch...)
	if next != nil {
		owned := *next
		owned.Branch = append([][32]byte(nil), next.Branch...)
		next = &owned
	}
	ownedBindings, ownedProofs := make([]ContractStateBinding, len(bindings)), make([]ContractStateProof, len(proofs))
	seen := map[[20]byte]bool{}
	for i := range bindings {
		if seen[bindings[i].Address] {
			return fail()
		}
		seen[bindings[i].Address] = true
		ownedBindings[i], ownedProofs[i], err = detachContractState(bindings[i], proofs[i])
		if err != nil || !bytes.Equal(ownedProofs[i].Header, parent.FinalizedExecutionHeader) {
			return fail()
		}
	}
	r, err := verifyParentFinalityProof(ctx, t, parent, next)
	if err != nil {
		return fail()
	}
	count := 0
	for i := range ownedBindings {
		checked, err := checkContractState(ctx, block, ownedBindings[i], ownedProofs[i])
		if err != nil {
			return fail()
		}
		count += checked.ExactStorageBindingsChecked
	}
	if ctx.Err() != nil {
		return fail()
	}
	return ParentDeploymentStateReport{ParentFinalityReport: r, ContractsChecked: len(bindings),
		ExactStorageBindingsChecked: count, AccountsBoundToParentExecutionRoot: true}, nil
}

type TokenStateReport struct {
	ContractStateReport
	SourceMaterialsMatched    bool `json:"sourceMaterialsMatched"`
	CompilerExecutionVerified bool `json:"compilerExecutionVerified"`
	TokenPermissionsReviewed  bool `json:"tokenPermissionsReviewed"`
}

// Exact source/config/runtime binding at a retained RH header. A capsule's
// compiledRuntime is still a declaration until independently recompiled.
// Slots require source-reviewed layout; no ERC-20/proxy/role slots are guessed.
func VerifyTokenStateProof(ctx context.Context, c Config, bundle TokenSourceBundle, compiler [32]byte, block Block, slots []ContractStorageBinding, proof ContractStateProof) (report TokenStateReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = TokenStateReport{}, ErrContractStateProof
		}
	}()
	if !CheckTokenSourceMaterials(c, bundle, compiler).OfflineMaterialsMatched {
		return TokenStateReport{}, ErrContractStateProof
	}
	address, err := address(c.private.TokenContract)
	if err != nil || !bytes.Equal(bundle.runtime, proof.Runtime) {
		return TokenStateReport{}, ErrContractStateProof
	}
	runtime, err := hash(c.private.ExpectedTokenCodeHash)
	if err != nil {
		return TokenStateReport{}, ErrContractStateProof
	}
	r, err := VerifyContractStateProof(ctx, block, ContractStateBinding{Address: address, RuntimeHash: runtime, Slots: slots}, proof)
	if err != nil {
		return TokenStateReport{}, ErrContractStateProof
	}
	return TokenStateReport{ContractStateReport: r, SourceMaterialsMatched: true}, nil
}
