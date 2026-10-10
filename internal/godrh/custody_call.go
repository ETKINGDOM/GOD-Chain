package godrh

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/big"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

var ErrCustodyCall = errors.New("RH offline custody call rejected; no signing, submission or asset activation")

type CustodyAction string

const (
	CustodyDeposit CustodyAction = "deposit"
	CustodyPay     CustodyAction = "pay"
	CustodyCancel  CustodyAction = "cancel"
	CustodyPause   CustodyAction = "pause"
)

// Explicit tagged request, not arbitrary calldata. Unrelated populated fields
// are rejected. Supplied withdrawal state/times are claims, not chain evidence.
type CustodyCallRequest struct {
	Action          CustodyAction
	NativeRecipient string
	Amount          sdkmath.Int
	Withdrawal      godbridge.Withdrawal
	Approvals       [][]byte
	ControlNonce    uint64
	Intake, Outflow bool
}

func (CustodyCallRequest) String() string               { return "RH offline custody call request (redacted)" }
func (r CustodyCallRequest) GoString() string           { return r.String() }
func (CustodyCallRequest) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type CustodyCallReport struct {
	Action                        CustodyAction `json:"action"`
	UnsignedCallPrepared          bool          `json:"unsignedCallPrepared"`
	CalldataBytes                 int           `json:"calldataBytes"`
	ApprovalSignaturesChecked     int           `json:"approvalSignaturesChecked"`
	ApprovalClaimsVerified        bool          `json:"approvalClaimsVerified"`
	SourceStateChecked            bool          `json:"sourceStateChecked"`
	NativeWithdrawalStateVerified bool          `json:"nativeWithdrawalStateVerified"`
	DelayEligibilityVerified      bool          `json:"delayEligibilityVerified"`
	SigningEnabled                bool          `json:"signingEnabled"`
	BroadcastEnabled              bool          `json:"broadcastEnabled"`
	RealAssetsReady               bool          `json:"realAssetsReady"`
}

// Owns private detached chain/target/calldata only. No transaction nonce, gas
// budget, wallet key, source connection, ledger or broadcasting capability.
type CustodyCall struct {
	action CustodyAction
	chain  *big.Int
	target [20]byte
	data   []byte
	quorum int
}

func (CustodyCall) String() string                 { return "RH offline unsigned custody call (redacted)" }
func (c CustodyCall) GoString() string             { return c.String() }
func (c CustodyCall) MarshalJSON() ([]byte, error) { return json.Marshal(c.Report()) }
func (c CustodyCall) Report() CustodyCallReport {
	if c.chain == nil || len(c.data) == 0 {
		return CustodyCallReport{}
	}
	return CustodyCallReport{Action: c.action, UnsignedCallPrepared: true, CalldataBytes: len(c.data), ApprovalSignaturesChecked: c.quorum}
}

// Explicit access for a separately reviewed transaction workflow. Data must
// remain private; preparing these bytes does not authorize their submission.
func (c CustodyCall) Payload() (*big.Int, [20]byte, []byte, error) {
	if !c.Report().UnsignedCallPrepared {
		return nil, [20]byte{}, nil, ErrCustodyCall
	}
	return new(big.Int).Set(c.chain), c.target, bytes.Clone(c.data), nil
}

func emptyWithdrawal(w godbridge.Withdrawal) bool {
	return w.ID == [32]byte{} && w.Sequence == 0 && w.Sender == [20]byte{} && w.Recipient == [20]byte{} &&
		w.Amount.IsNil() && w.QueuedAt.IsZero() && w.Status == "" && w.AuthorizedAt.IsZero() &&
		w.ResolvedAt.IsZero() && w.ResolutionEvidence == (godbridge.Evidence{})
}

func unfinishedWithdrawal(w godbridge.Withdrawal) bool {
	if !w.ResolvedAt.IsZero() || w.ResolutionEvidence != (godbridge.Evidence{}) {
		return false
	}
	switch w.Status {
	case godbridge.Queued:
		return w.AuthorizedAt.IsZero()
	case godbridge.Authorized:
		return w.AuthorizedAt.Unix() > 0 && w.AuthorizedAt.UTC().Year() <= 9999 &&
			!w.AuthorizedAt.Before(w.QueuedAt.Add(godbridge.WithdrawalDelay))
	default:
		return false
	}
}

type custodyWireWithdrawal struct {
	Sequence          uint64
	Sender, Recipient [20]byte
	Amount            *big.Int
	QueuedSeconds     uint64
	QueuedNanos       uint32
	Id                [32]byte
}

func custodyMethod(action CustodyAction) (abi.Method, error) {
	typeOf := func(name string, components []abi.ArgumentMarshaling) (abi.Type, error) {
		return abi.NewType(name, "", components)
	}
	var arguments abi.Arguments
	add := func(name, kind string, components []abi.ArgumentMarshaling) error {
		t, err := typeOf(kind, components)
		if err != nil {
			return ErrCustodyCall
		}
		arguments = append(arguments, abi.Argument{Name: name, Type: t})
		return nil
	}
	switch action {
	case CustodyDeposit:
		if add("amount", "uint256", nil) != nil || add("recipient", "bytes20", nil) != nil {
			return abi.Method{}, ErrCustodyCall
		}
	case CustodyPay, CustodyCancel:
		fields := []abi.ArgumentMarshaling{{Name: "sequence", Type: "uint64"}, {Name: "sender", Type: "bytes20"},
			{Name: "recipient", Type: "bytes20"}, {Name: "amount", Type: "uint256"}, {Name: "queuedSeconds", Type: "uint64"},
			{Name: "queuedNanos", Type: "uint32"}, {Name: "id", Type: "bytes32"}}
		if add("w", "tuple", fields) != nil || add("approvals", "bytes[]", nil) != nil {
			return abi.Method{}, ErrCustodyCall
		}
	case CustodyPause:
		if add("nonce", "uint64", nil) != nil || add("intake", "bool", nil) != nil || add("outflow", "bool", nil) != nil || add("approvals", "bytes[]", nil) != nil {
			return abi.Method{}, ErrCustodyCall
		}
	default:
		return abi.Method{}, ErrCustodyCall
	}
	return abi.NewMethod(string(action), string(action), abi.Function, "nonpayable", false, false, arguments, nil), nil
}

// PrepareCustodyCall constructs only four exact prototype contract methods.
// It validates full bindings, request shape and supplied quorum signatures,
// never the truth/finality of signed claims, queue order, balance or source time.
// Even complete production configuration leaves every activation gate false.
func PrepareCustodyCall(config Config, request CustodyCallRequest) (call CustodyCall, err error) {
	defer func() {
		if recover() != nil {
			call, err = CustodyCall{}, ErrCustodyCall
		}
	}()
	fail := func() (CustodyCall, error) { return CustodyCall{}, ErrCustodyCall }
	if !config.Report().ConfigurationReady {
		return fail()
	}
	binding, err := config.Binding()
	if err != nil {
		return fail()
	}
	chain, err := chainID(config.private.SourceChainID)
	if err != nil {
		return fail()
	}
	target, err := address(config.private.CustodyContract)
	if err != nil {
		return fail()
	}
	token, err := address(config.private.TokenContract)
	if err != nil {
		return fail()
	}
	method, err := custodyMethod(request.Action)
	if err != nil {
		return fail()
	}
	var args []any
	var digest [32]byte
	var approvals [][]byte
	if request.Action != CustodyDeposit {
		if len(request.Approvals) < godbridge.Threshold || len(request.Approvals) > godbridge.SignerCount {
			return fail()
		}
		for _, signature := range request.Approvals {
			if len(signature) != 65 {
				return fail()
			}
			approvals = append(approvals, bytes.Clone(signature))
		}
	}
	switch request.Action {
	case CustodyDeposit:
		if !emptyWithdrawal(request.Withdrawal) || len(request.Approvals) != 0 || request.ControlNonce != 0 || request.Intake || request.Outflow ||
			request.Amount.IsNil() || !request.Amount.IsPositive() || request.Amount.GT(godbridge.TransferLimit()) {
			return fail()
		}
		raw, err := godaddress.FromNative(request.NativeRecipient)
		if err != nil {
			return fail()
		}
		canonicalRecipient, err := godaddress.ToNative(raw)
		if err != nil || canonicalRecipient != request.NativeRecipient {
			return fail()
		}
		var recipient [20]byte
		copy(recipient[:], raw)
		if recipient == [20]byte{} {
			return fail()
		}
		args = []any{new(big.Int).Set(request.Amount.BigInt()), recipient}
	case CustodyPay, CustodyCancel:
		if request.NativeRecipient != "" || !request.Amount.IsNil() || request.ControlNonce != 0 || request.Intake || request.Outflow {
			return fail()
		}
		w := request.Withdrawal
		if !unfinishedWithdrawal(w) || request.Action == CustodyPay && (w.Recipient == target || w.Recipient == token || w.Status != godbridge.Authorized) {
			return fail()
		}
		if request.Action == CustodyPay {
			digest, err = godbridge.AuthorizationAttestationDigest(binding, w)
		} else {
			digest, err = godbridge.CancellationRequestAttestationDigest(binding, w)
		}
		if err != nil {
			return fail()
		}
		wire := custodyWireWithdrawal{w.Sequence, w.Sender, w.Recipient, new(big.Int).Set(w.Amount.BigInt()),
			uint64(w.QueuedAt.Unix()), uint32(w.QueuedAt.Nanosecond()), w.ID}
		args = []any{wire, approvals}
	case CustodyPause:
		if request.NativeRecipient != "" || !request.Amount.IsNil() || !emptyWithdrawal(request.Withdrawal) {
			return fail()
		}
		digest, err = godbridge.PauseAttestationDigest(binding, request.ControlNonce, request.Intake, request.Outflow)
		if err != nil {
			return fail()
		}
		args = []any{request.ControlNonce, request.Intake, request.Outflow, approvals}
	}
	if request.Action != CustodyDeposit && godbridge.VerifyAttestation(binding, digest, approvals) != nil {
		return fail()
	}
	packed, err := method.Inputs.Pack(args...)
	if err != nil || len(packed)+len(method.ID) > 4096 {
		return fail()
	}
	data := append(bytes.Clone(method.ID), packed...)
	return CustodyCall{action: request.Action, chain: new(big.Int).Set(chain), target: target, data: data, quorum: len(approvals)}, nil
}

// Keep the source second-precision ceiling explicit for operator review; this
// does not read a source timestamp or prove elapsed delay. Nanoseconds round UP.
func (r CustodyCallRequest) EarliestPaymentSecond() (uint64, error) {
	if r.Action != CustodyPay || !unfinishedWithdrawal(r.Withdrawal) || r.Withdrawal.Status != godbridge.Authorized ||
		r.Withdrawal.QueuedAt.Unix() <= 0 || r.Withdrawal.QueuedAt.UTC().Year() > 9999 {
		return 0, ErrCustodyCall
	}
	n := uint64(r.Withdrawal.QueuedAt.Unix()) + uint64(godbridge.WithdrawalDelay/time.Second)
	if r.Withdrawal.QueuedAt.Nanosecond() != 0 {
		n++
	}
	return n, nil
}
