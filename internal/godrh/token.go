package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"math/big"

	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// TokenReport is a separate pre-custody diagnostic. A provider observation or
// a retained runtime pin does not establish permissions, backing or finality.
// No operational values or provider messages enter this report.
type TokenReport struct {
	Mode                        string   `json:"mode"`
	ConnectionReady             bool     `json:"connectionReady"`
	RuntimePinConfigured        bool     `json:"runtimePinConfigured"`
	SourceChecked               bool     `json:"sourceChecked"`
	TokenObservationPassed      bool     `json:"tokenObservationPassed"`
	PinnedTokenProbePassed      bool     `json:"pinnedTokenProbePassed"`
	IndependentFinalityVerified bool     `json:"independentFinalityVerified"`
	TokenPermissionsReviewed    bool     `json:"tokenPermissionsReviewed"`
	RealAssetsReady             bool     `json:"realAssetsReady"`
	Missing                     []string `json:"missing"`
	Checks                      []Check  `json:"checks"`
	ReleaseGates                []string `json:"releaseGates"`
}

// TokenReport requires only source identity, endpoint and token. Other supplied
// fields still undergo the full syntax validation; absent custody, native-chain
// and signer fields remain absent and cannot produce a protocol Binding.
func (c Config) TokenReport() TokenReport {
	d := c.private
	r := TokenReport{Missing: []string{}, Checks: []Check{}, ReleaseGates: []string{
		"token_runtime_code_review", "token_permissions_review", "complete_custody_binding",
		"independent_source_finality", "independent_signer_operations", "recoverable_relayer",
		"source_backed_genesis_and_fee_funding", "delayed_network_payment",
		"production_node_wallet_and_rpc", "governance_and_operational_verification", "explicit_activation_authorization",
	}}
	if d.Mode == "simulation" || d.Mode == "production" {
		r.Mode = d.Mode
	}
	for _, field := range []struct{ name, value string }{
		{"source_chain", d.SourceChain}, {"source_chain_id", d.SourceChainID},
		{"rpc_endpoint", d.RPCEndpoint}, {"token_contract", d.TokenContract},
	} {
		if field.value == "" {
			r.Missing = append(r.Missing, field.name)
		}
	}
	r.ConnectionReady = d.Version == 1 && r.Mode != "" && len(r.Missing) == 0 && c.validate() == nil
	r.RuntimePinConfigured = d.ExpectedTokenCodeHash != "" && c.validate() == nil
	return r
}

// TokenInspection owns private candidate bytes, not an approved pin. Normal
// formatting and JSON are redacted. CandidateRuntime is explicit, detached and
// unavailable on any failed check. It never edits configuration or grants use.
type TokenInspection struct {
	report  TokenReport
	runtime []byte
	digest  [32]byte
	binding [32]byte
}

func (TokenInspection) String() string                 { return "GOD Chain RH token inspection (redacted)" }
func (i TokenInspection) GoString() string             { return i.String() }
func (i TokenInspection) MarshalJSON() ([]byte, error) { return json.Marshal(i.Report()) }

func (i TokenInspection) Report() TokenReport {
	r := i.report
	r.Missing = append([]string{}, r.Missing...)
	r.Checks = append([]Check{}, r.Checks...)
	r.ReleaseGates = append([]string{}, r.ReleaseGates...)
	return r
}

func (i TokenInspection) CandidateRuntime() ([]byte, [32]byte, bool) {
	if !i.report.TokenObservationPassed || len(i.runtime) == 0 || i.digest == [32]byte{} {
		return nil, [32]byte{}, false
	}
	return append([]byte(nil), i.runtime...), i.digest, true
}

// InspectToken performs at most eight source reads at one provider-reported
// finalized checkpoint. All contract reads are hash-pinned. A final code read
// detects an inconsistent provider view; it is not a state proof. No custody
// queries, keys, transaction submission, retries or activation are supplied.
// Replaceable Source implementations must honor the bounded context.
func InspectToken(ctx context.Context, c Config, source Source) (inspection TokenInspection) {
	inspection.report = c.TokenReport()
	r := &inspection.report
	defer func() {
		if recover() != nil {
			r.Checks = append(r.Checks, Check{"source_failure", false})
			r.TokenObservationPassed, r.PinnedTokenProbePassed = false, false
			inspection.runtime, inspection.digest = nil, [32]byte{}
			inspection.binding = [32]byte{}
		}
	}()
	if !r.ConnectionReady || source == nil || ctx == nil || ctx.Err() != nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, RelayReadTimeout)
	defer cancel()
	r.SourceChecked = true
	check := func(name string, ok bool) bool {
		ok = ok && ctx.Err() == nil
		r.Checks = append(r.Checks, Check{name, ok})
		return ok
	}
	expectedChain, _ := chainID(c.private.SourceChainID)
	observedChain, err := source.ChainID(ctx)
	if !check("source_chain_id", err == nil && observedChain != nil && observedChain.Cmp(expectedChain) == 0) {
		return
	}
	checkpoint, err := source.FinalizedBlock(ctx)
	if !check("provider_finalized_checkpoint", err == nil && checkpoint.Height > 0 && checkpoint.Hash != [32]byte{}) {
		return
	}
	token, _ := address(c.private.TokenContract)
	runtime, err := source.Code(ctx, token, checkpoint.Hash)
	if !check("token_runtime_read", err == nil) {
		return
	}
	if !check("token_runtime_present", len(runtime) > 0 && len(runtime) <= maxCodeBytes) {
		return
	}
	runtime = append([]byte(nil), runtime...)
	digest := [32]byte(ethcrypto.Keccak256Hash(runtime))
	if r.RuntimePinConfigured {
		expected, _ := hash(c.private.ExpectedTokenCodeHash)
		if !check("token_runtime_code_pin", digest == expected) {
			return
		}
	}
	for _, view := range []struct {
		name, signature string
		expected        *big.Int
	}{
		{"token_decimals", "decimals()", big.NewInt(godrewards.Decimals)},
		{"token_fixed_supply", "totalSupply()", godrewards.FixedGodSupply().BigInt()},
	} {
		value, err := source.Call(ctx, token, selector(view.signature), checkpoint.Hash)
		if !check(view.name, err == nil && len(value) == 32 && new(big.Int).SetBytes(value).Cmp(view.expected) == 0) {
			return
		}
	}
	observedChain, err = source.ChainID(ctx)
	if !check("source_chain_id_stable", err == nil && observedChain != nil && observedChain.Cmp(expectedChain) == 0) {
		return
	}
	retained, err := source.Block(ctx, checkpoint.Height)
	if !check("provider_checkpoint_stable", err == nil && retained == checkpoint) {
		return
	}
	retainedCode, err := source.Code(ctx, token, checkpoint.Hash)
	if !check("token_runtime_stable", err == nil && len(retainedCode) <= maxCodeBytes && bytes.Equal(runtime, retainedCode)) {
		return
	}
	inspection.runtime, inspection.digest = runtime, digest
	inspection.binding = tokenInspectionBinding(c)
	r.TokenObservationPassed, r.PinnedTokenProbePassed = true, r.RuntimePinConfigured
	return
}

func tokenRuntimeDigest(runtime []byte) [32]byte { return [32]byte(ethcrypto.Keccak256Hash(runtime)) }

// Bind private observations to the exact minimal connection, including its
// endpoint and retained code pin. Unrelated future custody fields are omitted.
// The digest is never a credential, a signature or independent source proof.
func tokenInspectionBinding(c Config) [32]byte {
	d := c.private
	raw, _ := json.Marshal(struct {
		Purpose                                                       string
		Version                                                       uint32
		Mode, SourceChain, SourceChainID, Endpoint, Token, RuntimePin string
	}{"GOD Chain private token observation v1", d.Version, d.Mode, d.SourceChain, d.SourceChainID, d.RPCEndpoint, d.TokenContract, d.ExpectedTokenCodeHash})
	return sha256.Sum256(raw)
}
