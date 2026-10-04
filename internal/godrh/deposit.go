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
type DepositSource = ReceiptSource

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
	observed, err := observeReceipt(ctx, c, source, request.TransactionHash, request.LogIndex)
	if err != nil {
		return fail()
	}
	custody, _ := address(c.private.CustodyContract)
	deposit, err := decodeDeposit(observed.log, custody, observed.receipt)
	if err != nil || deposit.Recipient != request.Recipient || !deposit.Amount.Equal(request.Amount) {
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
	if observed.recheck(ctx, c, source) != nil {
		return fail()
	}
	return DepositObservation{deposit, digest, observed.checkpoint, true}, nil
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
