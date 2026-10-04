package godrh

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/big"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

var ErrDepositObservation = errors.New("RH deposit evidence rejected; source finality and approval remain required")

// DepositSource extends the replaceable read-only connection with bounded
// receipts. No signer, native keeper or transaction-submission API is supplied.
type DepositSource interface {
	Source
	Receipt(context.Context, [32]byte) (Receipt, error)
}

// DepositRequest selects an exact source log and the expected native recipient
// and amount. LogIndex is the block-wide RPC log index, not a slice offset.
type DepositRequest struct {
	TransactionHash [32]byte
	LogIndex        uint32
	Recipient       [20]byte
	Amount          sdkmath.Int
}

func (DepositRequest) String() string               { return "RH deposit request (redacted)" }
func (r DepositRequest) GoString() string           { return r.String() }
func (DepositRequest) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

// DepositObservation owns a detached unsigned proposal. A provider's receipt
// and finalized tag can be fabricated together; matching them is not approval,
// independent finality, receipt inclusion proof or authorization to release GOD.
type DepositObservation struct {
	deposit    godbridge.Deposit
	digest     [32]byte
	checkpoint Block
	ready      bool
}

type DepositReport struct {
	UnsignedProposalReady       bool `json:"unsignedProposalReady"`
	IndependentFinalityVerified bool `json:"independentFinalityVerified"`
	ApprovalReady               bool `json:"approvalReady"`
	RealAssetsReady             bool `json:"realAssetsReady"`
}

func (DepositObservation) String() string                 { return "RH deposit observation (redacted)" }
func (o DepositObservation) GoString() string             { return o.String() }
func (o DepositObservation) MarshalJSON() ([]byte, error) { return json.Marshal(o.Report()) }
func (o DepositObservation) Report() DepositReport {
	return DepositReport{UnsignedProposalReady: o.ready}
}

// Proposal explicitly returns private operational content for later review.
// It never signs or grants approval; callers must not log or publish the result.
func (o DepositObservation) Proposal() (godbridge.Deposit, [32]byte, Block, error) {
	if !o.ready {
		return godbridge.Deposit{}, [32]byte{}, Block{}, ErrDepositObservation
	}
	d := o.deposit
	d.Amount = sdkmath.NewIntFromBigInt(d.Amount.BigInt())
	return d, o.digest, o.checkpoint, nil
}

// ObserveDeposit checks a source receipt, its canonical block, reviewed runtime
// code pins and the exact custody event ABI. It checks the original checkpoint
// again rather than rejecting ordinary forward progress. All evidence remains
// provider-attested. It creates no approval, replay record, balance or signature.
func ObserveDeposit(ctx context.Context, c Config, source DepositSource, request DepositRequest) (DepositObservation, error) {
	fail := func() (DepositObservation, error) { return DepositObservation{}, ErrDepositObservation }
	if ctx == nil || ctx.Err() != nil || source == nil || !c.Report().ConfigurationReady ||
		request.TransactionHash == [32]byte{} || request.Recipient == [20]byte{} || request.Amount.IsNil() ||
		!request.Amount.IsPositive() || request.Amount.GT(godbridge.TransferLimit()) {
		return fail()
	}
	request.Amount = sdkmath.NewIntFromBigInt(request.Amount.BigInt())
	if !Probe(ctx, c, source).ReadOnlyProbePassed {
		return fail()
	}
	expectedChain, _ := chainID(c.private.SourceChainID)
	chain, err := source.ChainID(ctx)
	if err != nil || chain == nil || chain.Cmp(expectedChain) != 0 {
		return fail()
	}
	checkpoint, err := source.FinalizedBlock(ctx)
	if err != nil || checkpoint.Height == 0 || checkpoint.Hash == [32]byte{} {
		return fail()
	}
	receipt, err := source.Receipt(ctx, request.TransactionHash)
	if err != nil || !receipt.Success || receipt.TransactionHash != request.TransactionHash ||
		receipt.Block.Height == 0 || receipt.Block.Height > checkpoint.Height || receipt.Block.Hash == [32]byte{} ||
		len(receipt.Logs) == 0 || len(receipt.Logs) > MaxReceiptLogs {
		return fail()
	}
	canonical, err := source.Block(ctx, receipt.Block.Height)
	if err != nil || canonical != receipt.Block {
		return fail()
	}
	token, _ := address(c.private.TokenContract)
	custody, _ := address(c.private.CustodyContract)
	for _, target := range []struct {
		address [20]byte
		pin     string
	}{
		{token, c.private.ExpectedTokenCodeHash}, {custody, c.private.ExpectedCustodyCodeHash},
	} {
		code, err := source.Code(ctx, target.address, receipt.Block.Hash)
		pin, _ := hash(target.pin)
		if err != nil || len(code) == 0 || len(code) > maxCodeBytes || ethcrypto.Keccak256Hash(code) != pin {
			return fail()
		}
	}
	var deposit godbridge.Deposit
	found := false
	for i, log := range receipt.Logs {
		if log.Removed || log.Emitter == [20]byte{} || len(log.Topics) > 4 || len(log.Data) > MaxLogDataBytes ||
			i > 0 && log.Index <= receipt.Logs[i-1].Index {
			return fail()
		}
		if log.Index != request.LogIndex {
			continue
		}
		deposit, err = decodeDeposit(log, custody, receipt)
		if err != nil || deposit.Recipient != request.Recipient || !deposit.Amount.Equal(request.Amount) {
			return fail()
		}
		found = true
	}
	if !found {
		return fail()
	}
	binding, err := c.Binding()
	if err != nil {
		return fail()
	}
	digest, err := godbridge.DepositAttestationDigest(binding, deposit)
	if err != nil {
		return fail()
	}
	canonical, err = source.Block(ctx, receipt.Block.Height)
	if err != nil || canonical != receipt.Block {
		return fail()
	}
	canonical, err = source.Block(ctx, checkpoint.Height)
	if err != nil || canonical != checkpoint {
		return fail()
	}
	chain, err = source.ChainID(ctx)
	if err != nil || chain == nil || chain.Cmp(expectedChain) != 0 || ctx.Err() != nil {
		return fail()
	}
	return DepositObservation{deposit, digest, checkpoint, true}, nil
}

func decodeDeposit(log ReceiptLog, custody [20]byte, receipt Receipt) (godbridge.Deposit, error) {
	var deposit godbridge.Deposit
	signature := ethcrypto.Keccak256Hash([]byte("Deposited(uint64,address,bytes20,uint256)"))
	if log.Emitter != custody || len(log.Topics) != 3 || log.Topics[0] != [32]byte(signature) || len(log.Data) != 64 ||
		!bytes.Equal(log.Topics[1][:24], make([]byte, 24)) || !bytes.Equal(log.Topics[2][:12], make([]byte, 12)) ||
		!bytes.Equal(log.Data[20:32], make([]byte, 12)) {
		return deposit, ErrDepositObservation
	}
	deposit.Sequence = binary.BigEndian.Uint64(log.Topics[1][24:])
	var sender [20]byte
	copy(sender[:], log.Topics[2][12:])
	copy(deposit.Recipient[:], log.Data[:20])
	deposit.Amount = sdkmath.NewIntFromBigInt(new(big.Int).SetBytes(log.Data[32:]))
	if deposit.Sequence == 0 || sender == [20]byte{} || deposit.Recipient == [20]byte{} || !deposit.Amount.IsPositive() || deposit.Amount.GT(godbridge.TransferLimit()) {
		return godbridge.Deposit{}, ErrDepositObservation
	}
	deposit.Evidence = godbridge.Evidence{Height: receipt.Block.Height, BlockHash: receipt.Block.Hash,
		TransactionHash: receipt.TransactionHash, LogIndex: log.Index}
	return deposit, nil
}

var _ DepositSource = (*HTTPSource)(nil)
