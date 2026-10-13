package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/big"
	"unicode/utf8"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

const (
	ReviewedCustodySourceSHA256     = "11212d146b3f0a1b18dd7f534f1c80837b4bc732b99581cbeefe95273f6806a0"
	custodyDeploymentCompiler       = "d73fead3cbc860438f56f4d1723677499cdef212d0f8890241748d2d5afc1c5f"
	MaxCustodyDeploymentPacketBytes = 128 << 10
)

var ErrCustodyDeployment = errors.New("RH offline custody deployment preparation refused; no signing, broadcast or activation")

// Private opaque output. Only execution of the pinned compiler against the
// exact reviewed source/settings can produce an admitted creation artifact.
type CompiledCustodyDeployment struct {
	creation, template     []byte
	tokenSites, assetSites []tokenImmutableSite
	valid                  bool
}

func (CompiledCustodyDeployment) String() string {
	return "RH private custody creation artifact (redacted)"
}
func (c CompiledCustodyDeployment) GoString() string { return c.String() }
func (c CompiledCustodyDeployment) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		CreationArtifactReady bool `json:"creationArtifactReady"`
		RealAssetsReady       bool `json:"realAssetsReady"`
	}{CreationArtifactReady: c.valid})
}

type CustodyDeploymentReport struct {
	ReviewedSourceMatched            bool `json:"reviewedSourceMatched"`
	CompilerWorkerCompleted          bool `json:"compilerWorkerCompleted"`
	CreationArtifactReady            bool `json:"creationArtifactReady"`
	ConstructorMatched               bool `json:"constructorMatched"`
	SevenOrderedMembersChecked       bool `json:"sevenOrderedMembersChecked"`
	CreateAddressCalculated          bool `json:"createAddressCalculated"`
	UnsignedTransactionPrepared      bool `json:"unsignedTransactionPrepared"`
	CancunIntrinsicGasChecked        bool `json:"cancunIntrinsicGasChecked"`
	WithinSuppliedExecutionFeeBudget bool `json:"withinSuppliedExecutionFeeBudget"`
	PrivateInputsChecked             bool `json:"privateInputsChecked"`
	UnsignedFileWritten              bool `json:"unsignedFileWritten"`
	RetainedFileMatched              bool `json:"retainedFileMatched"`
	InitCodeBytes                    int  `json:"initCodeBytes"`
	GasEstimateVerified              bool `json:"gasEstimateVerified"`
	SourceGasScheduleVerified        bool `json:"sourceGasScheduleVerified"`
	SourceAdditionalFeesVerified     bool `json:"sourceAdditionalFeesVerified"`
	NonceAvailabilityVerified        bool `json:"nonceAvailabilityVerified"`
	MemberControlVerified            bool `json:"memberControlVerified"`
	TokenStateVerified               bool `json:"tokenStateVerified"`
	CustodyDeployed                  bool `json:"custodyDeployed"`
	SigningEnabled                   bool `json:"signingEnabled"`
	BroadcastEnabled                 bool `json:"broadcastEnabled"`
	RealAssetsReady                  bool `json:"realAssetsReady"`
}

// Compile the standalone reviewed prototype, never a caller-selected contract
// or caller-provided ABI/bytecode. Metadata and source label are reproducible.
// Runtime immutable placeholders are checked, not misreported as deployed code.
func CompileCustodyDeployment(ctx context.Context, source []byte, sourcePin [32]byte, compiler TokenCompiler) (CompiledCustodyDeployment, CustodyDeploymentReport, error) {
	r := CustodyDeploymentReport{}
	if ctx == nil || ctx.Err() != nil || len(source) == 0 || len(source) > 64<<10 || !utf8.Valid(source) || sha256.Sum256(source) != sourcePin ||
		hex.EncodeToString(sourcePin[:]) != ReviewedCustodySourceSHA256 || hex.EncodeToString(compiler.compilerPin[:]) != custodyDeploymentCompiler {
		return CompiledCustodyDeployment{}, r, ErrCustodyDeployment
	}
	r.ReviewedSourceMatched = true
	label := "contracts/GodBridgeEscrow.sol"
	input, err := json.Marshal(map[string]any{"language": "Solidity", "sources": map[string]any{label: map[string]string{"content": string(source)}},
		"settings": map[string]any{"optimizer": map[string]any{"enabled": true, "runs": 200}, "viaIR": false, "evmVersion": "cancun",
			"metadata": map[string]string{"bytecodeHash": "none"}, "outputSelection": map[string]any{label: map[string]any{"GodBridgeEscrow": []string{
				"evm.bytecode.object", "evm.bytecode.linkReferences", "evm.deployedBytecode.object", "evm.deployedBytecode.linkReferences", "evm.deployedBytecode.immutableReferences"}, "": []string{"ast"}}}}})
	if err != nil {
		return CompiledCustodyDeployment{}, r, ErrCustodyDeployment
	}
	base, raw := runPinnedCompilerMode(ctx, compiler, input, label, "GodBridgeEscrow", true, true, TokenCompilationReport{})
	r.CompilerWorkerCompleted = base.CompilerWorkerCompleted
	fields, err := custodyObject(raw, []string{"version", "hasErrors", "runtime", "linkReferences", "immutableReferences", "creation", "creationLinkReferences", "immutableNames"}, nil)
	if err != nil || len(raw) > maxTokenCompilerOutputBytes {
		return CompiledCustodyDeployment{}, r, ErrCustodyDeployment
	}
	var names map[string]string
	if json.Unmarshal(fields["immutableNames"], &names) != nil || len(names) != 2 || names["token"] == "" || names["assetID"] == "" || names["token"] == names["assetID"] {
		return CompiledCustodyDeployment{}, r, ErrCustodyDeployment
	}
	delete(fields, "immutableNames")
	ordinary, err := json.Marshal(fields)
	if err != nil {
		return CompiledCustodyDeployment{}, r, ErrCustodyDeployment
	}
	out, err := parseTokenCreationCompileOutput(ordinary)
	if err != nil || ctx.Err() != nil || out.HasErrors || out.Version != tokenCompilerReleases[custodyDeploymentCompiler] || len(out.LinkReferences) != 0 || len(out.CreationLinkReferences) != 0 {
		return CompiledCustodyDeployment{}, r, ErrCustodyDeployment
	}
	creation, err := tokenCreationHex(out.Creation, MaxTokenCreationInputBytes)
	if err != nil {
		return CompiledCustodyDeployment{}, r, ErrCustodyDeployment
	}
	template, err := tokenCreationHex(out.Runtime, maxTokenCreatedCodeBytes)
	if err != nil {
		return CompiledCustodyDeployment{}, r, ErrCustodyDeployment
	}
	groups, err := tokenImmutableGroups(template, out.ImmutableReferences)
	if err != nil || len(groups) != 2 {
		return CompiledCustodyDeployment{}, r, ErrCustodyDeployment
	}
	var tokenSites, assetSites []tokenImmutableSite
	if json.Unmarshal(out.ImmutableReferences[names["token"]], &tokenSites) != nil || json.Unmarshal(out.ImmutableReferences[names["assetID"]], &assetSites) != nil || len(tokenSites) == 0 || len(assetSites) == 0 {
		return CompiledCustodyDeployment{}, r, ErrCustodyDeployment
	}
	r.CreationArtifactReady = true
	return CompiledCustodyDeployment{creation: bytes.Clone(creation), template: bytes.Clone(template), tokenSites: tokenSites, assetSites: assetSites, valid: true}, r, nil
}

type custodyDeploymentRequest struct {
	asset          common.Address
	chain          *big.Int
	source, native string
	members        [7]common.Address
	plan           custodyTransactionPlan
}

func CustodyDeploymentTemplate() []byte {
	return []byte("{\n  \"version\": 1,\n  \"tokenContract\": \"\",\n  \"sourceChainId\": \"\",\n  \"sourceChain\": \"\",\n  \"nativeChain\": \"\",\n  \"members\": [\"\", \"\", \"\", \"\", \"\", \"\", \"\"],\n  \"transactionPlan\": {\n    \"version\": 1,\n    \"sender\": \"\",\n    \"nonce\": \"\",\n    \"gasLimit\": \"\",\n    \"maxFeePerGas\": \"\",\n    \"maxPriorityFeePerGas\": \"\",\n    \"maximumFeeCost\": \"\"\n  }\n}\n")
}

func custodyDeploymentChainName(s string) bool {
	if len(s) < 1 || len(s) > 50 {
		return false
	}
	for _, c := range []byte(s) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func parseCustodyDeploymentRequest(raw []byte) (custodyDeploymentRequest, error) {
	var r custodyDeploymentRequest
	if len(raw) == 0 || len(raw) > MaxCustodyRequestBytes || !utf8.Valid(raw) {
		return r, ErrCustodyDeployment
	}
	f, err := custodyObject(raw, []string{"version", "tokenContract", "sourceChainId", "sourceChain", "nativeChain", "members", "transactionPlan"}, nil)
	if err != nil || !bytes.Equal(bytes.TrimSpace(f["version"]), []byte("1")) {
		return r, ErrCustodyDeployment
	}
	a, err := custodyHex(f["tokenContract"], 20)
	if err != nil {
		return r, ErrCustodyDeployment
	}
	copy(r.asset[:], a)
	n, err := custodyPositiveDecimal(f["sourceChainId"])
	if err != nil {
		return r, ErrCustodyDeployment
	}
	r.chain, _ = new(big.Int).SetString(n, 10)
	if r.chain == nil || r.chain.BitLen() > 256 || r.asset == (common.Address{}) {
		return custodyDeploymentRequest{}, ErrCustodyDeployment
	}
	r.source, err = custodyText(f["sourceChain"])
	if err != nil {
		return custodyDeploymentRequest{}, ErrCustodyDeployment
	}
	r.native, err = custodyText(f["nativeChain"])
	if err != nil || !custodyDeploymentChainName(r.source) || !custodyDeploymentChainName(r.native) || r.source == r.native {
		return custodyDeploymentRequest{}, ErrCustodyDeployment
	}
	var members []json.RawMessage
	if json.Unmarshal(f["members"], &members) != nil || len(members) != 7 {
		return custodyDeploymentRequest{}, ErrCustodyDeployment
	}
	for i, m := range members {
		a, err := custodyHex(m, 20)
		if err != nil {
			return custodyDeploymentRequest{}, ErrCustodyDeployment
		}
		copy(r.members[i][:], a)
		if r.members[i] == (common.Address{}) || i > 0 && bytes.Compare(r.members[i-1][:], r.members[i][:]) >= 0 {
			return custodyDeploymentRequest{}, ErrCustodyDeployment
		}
	}
	r.plan, err = parseCustodyTransactionPlan(f["transactionPlan"])
	if err != nil {
		return custodyDeploymentRequest{}, ErrCustodyDeployment
	}
	return r, nil
}

type CustodyDeployment struct {
	raw, runtime       []byte
	creator, predicted common.Address
	report             CustodyDeploymentReport
}

func (CustodyDeployment) String() string                    { return "RH private unsigned custody deployment (redacted)" }
func (d CustodyDeployment) GoString() string                { return d.String() }
func (d CustodyDeployment) MarshalJSON() ([]byte, error)    { return json.Marshal(d.Report()) }
func (d CustodyDeployment) Report() CustodyDeploymentReport { return d.report }

// Explicit private export for separate review, not a source-provider or signing
// route. Zero signature fields mean the encoded type-2 object is UNSIGNED.
func (d CustodyDeployment) Payload() (common.Address, common.Address, []byte, error) {
	if !d.report.UnsignedTransactionPrepared || len(d.raw) == 0 {
		return common.Address{}, common.Address{}, nil, ErrCustodyDeployment
	}
	return d.creator, d.predicted, bytes.Clone(d.raw), nil
}

func PrepareCustodyDeployment(artifact CompiledCustodyDeployment, request []byte) (CustodyDeployment, error) {
	r, err := parseCustodyDeploymentRequest(request)
	if err != nil || !artifact.valid || len(artifact.creation) == 0 {
		return CustodyDeployment{}, ErrCustodyDeployment
	}
	var args abi.Arguments
	for _, name := range []string{"address", "uint256", "string", "string", "address[7]"} {
		t, err := abi.NewType(name, "", nil)
		if err != nil {
			return CustodyDeployment{}, ErrCustodyDeployment
		}
		args = append(args, abi.Argument{Type: t})
	}
	constructor, err := args.Pack(r.asset, r.chain, r.source, r.native, r.members)
	if err != nil || len(constructor) > MaxTokenConstructorBytes || len(artifact.creation)+len(constructor) > MaxTokenCreationInputBytes {
		return CustodyDeployment{}, ErrCustodyDeployment
	}
	init := append(bytes.Clone(artifact.creation), constructor...)
	intrinsic, err := core.IntrinsicGas(init, nil, nil, true, true, true, true)
	if err != nil || r.plan.gas < intrinsic {
		return CustodyDeployment{}, ErrCustodyDeployment
	}
	creator := common.Address(r.plan.sender)
	predicted := ethcrypto.CreateAddress(creator, r.plan.nonce)
	if predicted == (common.Address{}) || predicted == r.asset || predicted == creator {
		return CustodyDeployment{}, ErrCustodyDeployment
	}
	for _, m := range r.members {
		if predicted == m {
			return CustodyDeployment{}, ErrCustodyDeployment
		}
	}
	// Exact known immutable values, derived from reviewed constructor semantics
	// and compiler AST IDs. No supplied replacement map or arbitrary wildcard.
	runtime := bytes.Clone(artifact.template)
	if len(runtime) > 0 {
		id, err := godbridge.AssetBinding(sdkmath.NewIntFromBigInt(r.chain), [20]byte(r.asset), [20]byte(predicted), 18)
		if err != nil {
			return CustodyDeployment{}, ErrCustodyDeployment
		}
		var tokenWord [32]byte
		copy(tokenWord[12:], r.asset[:])
		for _, binding := range []struct {
			sites []tokenImmutableSite
			word  [32]byte
		}{{artifact.tokenSites, tokenWord}, {artifact.assetSites, id}} {
			if len(binding.sites) == 0 {
				return CustodyDeployment{}, ErrCustodyDeployment
			}
			for _, site := range binding.sites {
				if site.Length != 32 || uint64(site.Start)+32 > uint64(len(runtime)) {
					return CustodyDeployment{}, ErrCustodyDeployment
				}
				copy(runtime[site.Start:site.Start+32], binding.word[:])
			}
		}
	}
	tx := types.NewTx(&types.DynamicFeeTx{ChainID: new(big.Int).Set(r.chain), Nonce: r.plan.nonce, Gas: r.plan.gas,
		GasFeeCap: new(big.Int).Set(r.plan.fee), GasTipCap: new(big.Int).Set(r.plan.tip), Value: new(big.Int), Data: init})
	raw, err := tx.MarshalBinary()
	if err != nil {
		return CustodyDeployment{}, ErrCustodyDeployment
	}
	report := CustodyDeploymentReport{ReviewedSourceMatched: true, CompilerWorkerCompleted: true, CreationArtifactReady: true, ConstructorMatched: true,
		SevenOrderedMembersChecked: true, CreateAddressCalculated: true, UnsignedTransactionPrepared: true, CancunIntrinsicGasChecked: true,
		WithinSuppliedExecutionFeeBudget: true, InitCodeBytes: len(init)}
	return CustodyDeployment{raw: raw, runtime: runtime, creator: creator, predicted: predicted, report: report}, nil
}
