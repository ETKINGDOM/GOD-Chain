package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

var ErrCustodyCreationExecution = errors.New("RH conditional custody creation review refused; no source finality or activation")

type CustodyCreationExecutionReport struct {
	PrivateInputsChecked          bool `json:"privateInputsChecked"`
	SingleCustodianDeclared       bool `json:"singleCustodianDeclared"`
	MemberControlVerified         bool `json:"memberControlVerified"`
	SignedEnvelopeMatched         bool `json:"signedEnvelopeMatched"`
	SenderSignatureChecked        bool `json:"senderSignatureChecked"`
	CompleteReceiptSetMatched     bool `json:"completeReceiptSetMatched"`
	ExactCreationIncluded         bool `json:"exactCreationIncluded"`
	ExecutionGasAccountingMatched bool `json:"executionGasAccountingMatched"`
	ReceiptSucceeded              bool `json:"receiptSucceeded"`
	RuntimeAccountProofMatched    bool `json:"runtimeAccountProofMatched"`
	CompiledRuntimeMatched        bool `json:"compiledRuntimeMatched"`
	ConditionalCreationObserved   bool `json:"conditionalCreationObserved"`
	ConstructorStorageVerified    bool `json:"constructorStorageVerified"`
	SourceChainIdentityVerified   bool `json:"sourceChainIdentityVerified"`
	IndependentFinalityVerified   bool `json:"independentFinalityVerified"`
	NonceReleased                 bool `json:"nonceReleased"`
	FeeReservationReleased        bool `json:"feeReservationReleased"`
	SigningEnabled                bool `json:"signingEnabled"`
	BroadcastEnabled              bool `json:"broadcastEnabled"`
	RealAssetsReady               bool `json:"realAssetsReady"`
}

type checkedCustodyDeployment struct {
	deployment CustodyDeployment
	raw        []byte
	hash       [32]byte
	report     CustodyCreationExecutionReport
}

func checkCustodyDeploymentTransaction(d CustodyDeployment, raw []byte) (checkedCustodyDeployment, error) {
	bad := func() (checkedCustodyDeployment, error) {
		return checkedCustodyDeployment{}, ErrCustodyCreationExecution
	}
	if !d.report.UnsignedTransactionPrepared || len(d.raw) == 0 || len(d.runtime) == 0 || len(d.runtime) > maxTokenCreatedCodeBytes || len(raw) == 0 || len(raw) > MaxTokenCreationInputBytes+1024 {
		return bad()
	}
	var planned, tx types.Transaction
	if planned.UnmarshalBinary(d.raw) != nil || tx.UnmarshalBinary(bytes.Clone(raw)) != nil || planned.Type() != types.DynamicFeeTxType || tx.Type() != types.DynamicFeeTxType ||
		tx.To() != nil || tx.Value().Sign() != 0 || tx.ChainId().Cmp(planned.ChainId()) != 0 || tx.Nonce() != planned.Nonce() || tx.Gas() != planned.Gas() ||
		tx.GasFeeCap().Cmp(planned.GasFeeCap()) != 0 || tx.GasTipCap().Cmp(planned.GasTipCap()) != 0 || !bytes.Equal(tx.Data(), planned.Data()) || len(tx.AccessList()) != 0 || len(tx.SetCodeAuthorizations()) != 0 {
		return bad()
	}
	canonical, err := tx.MarshalBinary()
	if err != nil || !bytes.Equal(raw, canonical) {
		return bad()
	}
	sender, err := types.Sender(types.NewLondonSigner(tx.ChainId()), &tx)
	if err != nil || sender != d.creator || ethcrypto.CreateAddress(sender, tx.Nonce()) != d.predicted {
		return bad()
	}
	d.raw, d.runtime = bytes.Clone(d.raw), bytes.Clone(d.runtime)
	return checkedCustodyDeployment{deployment: d, raw: bytes.Clone(raw), hash: [32]byte(tx.Hash()), report: CustodyCreationExecutionReport{
		SignedEnvelopeMatched: true, SenderSignatureChecked: true, SingleCustodianDeclared: d.report.SingleCustodianDeclared}}, nil
}

func CheckCustodyDeploymentTransaction(d CustodyDeployment, raw []byte) (CustodyCreationExecutionReport, error) {
	c, err := checkCustodyDeploymentTransaction(d, raw)
	if err != nil {
		return CustodyCreationExecutionReport{}, ErrCustodyCreationExecution
	}
	return c.report, nil
}

func loadCustodyDeploymentTransaction(ctx context.Context, f CustodyDeploymentFiles, c TokenCompiler, signedPath string, signedPin [32]byte) (checkedCustodyDeployment, error) {
	wire, err := readRHPrivateBytes(signedPath, MaxTokenCreationInputBytes+1024, signedPin)
	if err != nil {
		return checkedCustodyDeployment{}, ErrCustodyCreationExecution
	}
	d, err := loadCustodyDeployment(ctx, f, c)
	if err != nil {
		return checkedCustodyDeployment{}, ErrCustodyCreationExecution
	}
	checked, err := checkCustodyDeploymentTransaction(d, wire)
	if err != nil {
		return checkedCustodyDeployment{}, ErrCustodyCreationExecution
	}
	retained, err := readRHPrivateBytes(signedPath, MaxTokenCreationInputBytes+1024, signedPin)
	if err != nil || !bytes.Equal(retained, wire) || ctx.Err() != nil {
		return checkedCustodyDeployment{}, ErrCustodyCreationExecution
	}
	checked.report.PrivateInputsChecked = true
	return checked, nil
}

// Offline signed-file review. No signer, provider, nonce reservation or dispatch.
func CheckCustodyDeploymentTransactionFiles(ctx context.Context, f CustodyDeploymentFiles, c TokenCompiler, signedPath string, signedPin [32]byte) (CustodyCreationExecutionReport, error) {
	r, err := loadCustodyDeploymentTransaction(ctx, f, c, signedPath, signedPin)
	if err != nil {
		return CustodyCreationExecutionReport{}, ErrCustodyCreationExecution
	}
	return r.report, nil
}

// Exact complete-set roots plus code inclusion at the SAME supplied header.
// This does not authenticate header origin, RH consensus/L1 settlement or the
// mutable constructor storage. JSON contractAddress metadata is not evidence.
func checkCustodyCreationExecution(ctx context.Context, c checkedCustodyDeployment, block Block, material BlockMaterial, proof ContractStateProof) (report CustodyCreationExecutionReport, digest [32]byte, err error) {
	defer func() {
		if recover() != nil {
			report, digest, err = CustodyCreationExecutionReport{}, [32]byte{}, ErrCustodyCreationExecution
		}
	}()
	bad := func() (CustodyCreationExecutionReport, [32]byte, error) {
		return CustodyCreationExecutionReport{}, [32]byte{}, ErrCustodyCreationExecution
	}
	if ctx == nil {
		return bad()
	}
	m, err := detachReceiptSetMaterial(material)
	if err != nil {
		return bad()
	}
	// Detach all supplied proof fields before calling caller-controlled context.
	var owned ContractStateProof
	if len(proof.Header) > 0 || len(proof.Runtime) > 0 || len(proof.AccountNodes) > 0 || len(proof.StorageNodes) > 0 {
		if len(proof.StorageNodes) != 0 {
			return bad()
		}
		_, owned, err = detachContractState(ContractStateBinding{Address: [20]byte(c.deployment.predicted), RuntimeHash: tokenRuntimeDigest(c.deployment.runtime)}, proof)
		if err != nil {
			return bad()
		}
	}
	if ctx.Err() != nil || !validDiscoveryBlock(block) {
		return bad()
	}
	if _, err := receiptSetEvents(ctx, m, [20]byte(c.deployment.predicted), block); err != nil {
		return bad()
	}
	selected := -1
	for i, wire := range m.Transactions {
		if bytes.Equal(wire, c.raw) {
			if selected != -1 {
				return bad()
			}
			selected = i
		}
	}
	if selected < 0 {
		return bad()
	}
	var header types.Header
	var tx types.Transaction
	var receipt types.Receipt
	if rlp.DecodeBytes(m.Header, &header) != nil || tx.UnmarshalBinary(c.raw) != nil || receipt.UnmarshalBinary(m.Receipts[selected]) != nil ||
		header.BaseFee == nil || header.BaseFee.Sign() < 0 || header.BaseFee.BitLen() > 256 || header.BaseFee.Cmp(tx.GasFeeCap()) > 0 {
		return bad()
	}
	var previous uint64
	if selected > 0 {
		var prior types.Receipt
		if prior.UnmarshalBinary(m.Receipts[selected-1]) != nil {
			return bad()
		}
		previous = prior.CumulativeGasUsed
	}
	intrinsic, err := core.IntrinsicGas(tx.Data(), nil, nil, true, true, true, true)
	if err != nil || receipt.CumulativeGasUsed <= previous {
		return bad()
	}
	used := receipt.CumulativeGasUsed - previous
	if used < intrinsic || used > tx.Gas() {
		return bad()
	}
	price := new(big.Int).Add(header.BaseFee, tx.GasTipCap())
	if price.Cmp(tx.GasFeeCap()) > 0 {
		price.Set(tx.GasFeeCap())
	}
	cost := new(big.Int).Mul(new(big.Int).SetUint64(used), price)
	maximum := new(big.Int).Mul(new(big.Int).SetUint64(tx.Gas()), tx.GasFeeCap())
	if cost.Cmp(maximum) > 0 {
		return bad()
	}
	r := c.report
	r.CompleteReceiptSetMatched, r.ExactCreationIncluded, r.ExecutionGasAccountingMatched = true, true, true
	r.ReceiptSucceeded = receipt.Status == types.ReceiptStatusSuccessful
	if r.ReceiptSucceeded {
		if !bytes.Equal(owned.Header, m.Header) || !bytes.Equal(owned.Runtime, c.deployment.runtime) {
			return bad()
		}
		b := ContractStateBinding{Address: [20]byte(c.deployment.predicted), RuntimeHash: tokenRuntimeDigest(c.deployment.runtime)}
		state, err := checkContractState(ctx, block, b, owned)
		if err != nil || !state.RuntimeMatchesAccountAndPin {
			return bad()
		}
		r.RuntimeAccountProofMatched, r.CompiledRuntimeMatched, r.ConditionalCreationObserved = true, true, true
	} else if len(owned.Header) != 0 {
		return bad()
	}
	if ctx.Err() != nil {
		return bad()
	}
	// Retained observation identity binds complete roots, exact account proof and
	// compiled runtime; a digest is NOT transferable source finality evidence.
	h := sha256.New()
	h.Write([]byte("GOD Chain conditional custody creation observation v1"))
	pin := custodyMaterialPin(m)
	h.Write(pin[:])
	codePin := sha256.Sum256(owned.Runtime)
	h.Write(codePin[:])
	for _, node := range owned.AccountNodes {
		n := sha256.Sum256(node)
		h.Write(n[:])
	}
	digest = [32]byte{}
	copy(digest[:], h.Sum(nil))
	return r, digest, nil
}

// Pure observation composition; no financial or mutable ledger transition.
func CheckCustodyDeploymentExecution(ctx context.Context, d CustodyDeployment, wire []byte, block Block, m BlockMaterial, p ContractStateProof) (CustodyCreationExecutionReport, error) {
	c, err := checkCustodyDeploymentTransaction(d, wire)
	if err != nil {
		return CustodyCreationExecutionReport{}, ErrCustodyCreationExecution
	}
	r, _, err := checkCustodyCreationExecution(ctx, c, block, m, p)
	return r, err
}

func creationHexPin(pin [32]byte) string { return hex.EncodeToString(pin[:]) }
