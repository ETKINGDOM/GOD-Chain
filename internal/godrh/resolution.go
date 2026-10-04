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

var ErrResolutionObservation = errors.New("RH withdrawal outcome evidence rejected; source finality and approval remain required")

// ResolutionRequest binds one provider receipt to a caller-supplied native
// withdrawal. Its structure is checked, not its native state finality.
type ResolutionRequest struct {
	TransactionHash [32]byte
	LogIndex        uint32
	Withdrawal      godbridge.Withdrawal
	Outcome         godbridge.Status
}

func (ResolutionRequest) String() string               { return "RH resolution request (redacted)" }
func (r ResolutionRequest) GoString() string           { return r.String() }
func (ResolutionRequest) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type ResolutionReport struct {
	UnsignedProposalReady       bool `json:"unsignedProposalReady"`
	IndependentFinalityVerified bool `json:"independentFinalityVerified"`
	ApprovalReady               bool `json:"approvalReady"`
	RealAssetsReady             bool `json:"realAssetsReady"`
}

// ResolutionObservation never acknowledges payment or refunds native funds.
// The expected withdrawal and returned proposal are private operational data.
type ResolutionObservation struct {
	withdrawal godbridge.Withdrawal
	outcome    godbridge.Status
	evidence   godbridge.Evidence
	digest     [32]byte
	checkpoint Block
	ready      bool
}

func (ResolutionObservation) String() string                 { return "RH resolution observation (redacted)" }
func (o ResolutionObservation) GoString() string             { return o.String() }
func (o ResolutionObservation) MarshalJSON() ([]byte, error) { return json.Marshal(o.Report()) }
func (o ResolutionObservation) Report() ResolutionReport {
	return ResolutionReport{UnsignedProposalReady: o.ready}
}

// Proposal explicitly returns detached private data, not approvals. It must
// never be logged, published or accepted as trusted source evidence by itself.
func (o ResolutionObservation) Proposal() (godbridge.Withdrawal, godbridge.Status, godbridge.Evidence, [32]byte, Block, error) {
	if !o.ready {
		return godbridge.Withdrawal{}, "", godbridge.Evidence{}, [32]byte{}, Block{}, ErrResolutionObservation
	}
	w := o.withdrawal
	w.Amount = sdkmath.NewIntFromBigInt(w.Amount.BigInt())
	return w, o.outcome, o.evidence, o.digest, o.checkpoint, nil
}

// ObserveResolution checks exact Paid/Cancelled event bytes and the matching
// terminal view at the receipt block. Provider agreement is not a receipt proof,
// independent finality or proof of native withdrawal state.
func ObserveResolution(ctx context.Context, c Config, source ReceiptSource, request ResolutionRequest) (ResolutionObservation, error) {
	return observeResolution(ctx, c, source, request, nil)
}

func observeResolution(ctx context.Context, c Config, source ReceiptSource, request ResolutionRequest, receiptSet *relayReceiptBinding) (ResolutionObservation, error) {
	fail := func() (ResolutionObservation, error) { return ResolutionObservation{}, ErrResolutionObservation }
	if ctx == nil || ctx.Err() != nil || source == nil {
		return fail()
	}
	binding, w, err := resolutionRequest(c, request)
	if err != nil {
		return fail()
	}
	observed, err := observeReceipt(ctx, c, source, request.TransactionHash, request.LogIndex, receiptSet)
	if err != nil {
		return fail()
	}
	custody, _ := address(c.private.CustodyContract)
	log := observed.log
	terminal := byte(2)
	if request.Outcome == godbridge.Paid {
		terminal = 1
	}
	if decodeResolutionEvent(log, custody, w, request.Outcome) != nil {
		return fail()
	}
	input := make([]byte, 36)
	copy(input, selector("terminal(uint64)"))
	binary.BigEndian.PutUint64(input[28:], w.Sequence)
	value, err := source.Call(ctx, custody, input, observed.receipt.Block.Hash)
	if err != nil || len(value) != 64 || !bytes.Equal(value[:32], w.ID[:]) ||
		!bytes.Equal(value[32:63], make([]byte, 31)) || value[63] != terminal {
		return fail()
	}
	evidence := godbridge.Evidence{Height: observed.receipt.Block.Height, BlockHash: observed.receipt.Block.Hash,
		TransactionHash: observed.receipt.TransactionHash, LogIndex: log.Index}
	digest, err := godbridge.ResolutionAttestationDigest(binding, w, request.Outcome, evidence)
	if err != nil || observed.recheck(ctx, c, source) != nil {
		return fail()
	}
	return ResolutionObservation{w, request.Outcome, evidence, digest, observed.checkpoint, true}, nil
}

// Shared event ABI check only. This does not query terminal/native state or
// establish execution, code identity, canonical headers, finality or approval.
// Callers must separately validate the complete withdrawal request.
func decodeResolutionEvent(log ReceiptLog, custody [20]byte, w godbridge.Withdrawal, outcome godbridge.Status) error {
	event := "Cancelled(uint64,bytes32)"
	if outcome == godbridge.Paid {
		event = "Paid(uint64,bytes32,bytes20,uint256)"
	} else if outcome != godbridge.Cancelled {
		return ErrResolutionObservation
	}
	if custody == [20]byte{} || w.Sequence == 0 || w.ID == [32]byte{} || log.Removed || log.Emitter != custody ||
		len(log.Topics) != 3 || log.Topics[0] != [32]byte(ethcrypto.Keccak256Hash([]byte(event))) ||
		!bytes.Equal(log.Topics[1][:24], make([]byte, 24)) || binary.BigEndian.Uint64(log.Topics[1][24:]) != w.Sequence || log.Topics[2] != w.ID {
		return ErrResolutionObservation
	}
	if outcome == godbridge.Paid {
		if w.Amount.IsNil() || len(log.Data) != 64 || !bytes.Equal(log.Data[:20], w.Recipient[:]) ||
			!bytes.Equal(log.Data[20:32], make([]byte, 12)) || new(big.Int).SetBytes(log.Data[32:]).Cmp(w.Amount.BigInt()) != 0 {
			return ErrResolutionObservation
		}
	} else if len(log.Data) != 0 {
		return ErrResolutionObservation
	}
	return nil
}

// resolutionRequest is shared by live observation and the simulation journal.
// It validates/detaches request shape without querying either chain.
func resolutionRequest(c Config, request ResolutionRequest) (godbridge.Config, godbridge.Withdrawal, error) {
	fail := func() (godbridge.Config, godbridge.Withdrawal, error) {
		return godbridge.Config{}, godbridge.Withdrawal{}, ErrResolutionObservation
	}
	if request.TransactionHash == [32]byte{} ||
		!c.Report().ConfigurationReady || request.Outcome != godbridge.Paid && request.Outcome != godbridge.Cancelled {
		return fail()
	}
	w := request.Withdrawal
	if !w.ResolvedAt.IsZero() || w.ResolutionEvidence != (godbridge.Evidence{}) ||
		w.Status != godbridge.Queued && w.Status != godbridge.Authorized ||
		request.Outcome == godbridge.Paid && w.Status != godbridge.Authorized ||
		w.Status == godbridge.Queued && !w.AuthorizedAt.IsZero() ||
		w.Status == godbridge.Authorized && (w.AuthorizedAt.Unix() <= 0 || w.AuthorizedAt.UTC().Year() > 9999 ||
			w.AuthorizedAt.Before(w.QueuedAt.Add(godbridge.WithdrawalDelay))) {
		return fail()
	}
	binding, err := c.Binding()
	if err != nil {
		return fail()
	}
	// Validate every immutable field and recompute the full withdrawal identity.
	if _, err := godbridge.AuthorizationAttestationDigest(binding, w); err != nil {
		return fail()
	}
	w.Amount = sdkmath.NewIntFromBigInt(w.Amount.BigInt())
	return binding, w, nil
}
