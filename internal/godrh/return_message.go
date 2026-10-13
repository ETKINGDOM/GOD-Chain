package godrh

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	bridgemsg "github.com/ETKINGDOM/GOD-Chain/x/godbridge/msg"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var ErrNativeResolutionMessage = errors.New("offline native return message rejected; no signing, submission or financial approval")

// An offline claim, not authenticated native state or independently final RH
// evidence. The original digest is mandatory to detect changed handoff fields.
type NativeResolutionRequest struct {
	Sender            string
	Withdrawal        godbridge.Withdrawal
	Outcome           godbridge.Status
	Evidence          godbridge.Evidence
	AttestationDigest [32]byte
	Approvals         [][]byte
}

func (NativeResolutionRequest) String() string {
	return "private native return message request (redacted)"
}
func (r NativeResolutionRequest) GoString() string           { return r.String() }
func (NativeResolutionRequest) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type NativeResolutionMessageReport struct {
	Outcome                           godbridge.Status `json:"outcome"`
	UnsignedMessagePrepared           bool             `json:"unsignedMessagePrepared"`
	ApprovalSignaturesChecked         int              `json:"approvalSignaturesChecked"`
	ApprovalClaimsVerified            bool             `json:"approvalClaimsVerified"`
	NativeStateProven                 bool             `json:"nativeStateProven"`
	IndependentSourceFinalityVerified bool             `json:"independentSourceFinalityVerified"`
	SenderAuthenticated               bool             `json:"senderAuthenticated"`
	SigningEnabled                    bool             `json:"signingEnabled"`
	BroadcastEnabled                  bool             `json:"broadcastEnabled"`
	RealAssetsReady                   bool             `json:"realAssetsReady"`
}

// Contains one bounded, detached SDK message, NOT a signed transaction or a
// durable attempt. Repeated Payload calls are copies, not another outbox take.
// No nonce, fee, signer, network adapter, retry or financial release is supplied.
type NativeResolutionMessage struct {
	outcome   godbridge.Status
	sender    string
	sequence  uint64
	id        [32]byte
	evidence  godbridge.Evidence
	approvals [][]byte
}

func (NativeResolutionMessage) String() string {
	return "private unsigned native return message (redacted)"
}
func (m NativeResolutionMessage) GoString() string             { return m.String() }
func (m NativeResolutionMessage) MarshalJSON() ([]byte, error) { return json.Marshal(m.Report()) }
func (m NativeResolutionMessage) Report() NativeResolutionMessageReport {
	if m.sequence == 0 {
		return NativeResolutionMessageReport{}
	}
	return NativeResolutionMessageReport{Outcome: m.outcome, UnsignedMessagePrepared: true, ApprovalSignaturesChecked: len(m.approvals)}
}

func cloneResolutionApprovals(input [][]byte) [][]byte {
	out := make([][]byte, len(input))
	for i := range input {
		out[i] = bytes.Clone(input[i])
	}
	return out
}

// Explicit PRIVATE access for a separately reviewed outer transaction workflow.
// Its caller must authenticate/retain a signed attempt before any submission.
func (m NativeResolutionMessage) Payload() (sdk.Msg, error) {
	if !m.Report().UnsignedMessagePrepared {
		return nil, ErrNativeResolutionMessage
	}
	e := &bridgemsg.SourceEvidence{Height: m.evidence.Height, BlockHash: bytes.Clone(m.evidence.BlockHash[:]),
		TransactionHash: bytes.Clone(m.evidence.TransactionHash[:]), LogIndex: m.evidence.LogIndex}
	if m.outcome == godbridge.Paid {
		return &bridgemsg.MsgResolvePayment{Sender: m.sender, WithdrawalSequence: m.sequence, WithdrawalId: bytes.Clone(m.id[:]),
			Evidence: e, Approvals: cloneResolutionApprovals(m.approvals)}, nil
	}
	if m.outcome == godbridge.Cancelled {
		return &bridgemsg.MsgCancelWithdrawal{Sender: m.sender, WithdrawalSequence: m.sequence, WithdrawalId: bytes.Clone(m.id[:]),
			Evidence: e, Approvals: cloneResolutionApprovals(m.approvals)}, nil
	}
	return nil, ErrNativeResolutionMessage
}

// Verify only immutable request binding and five-through-seven distinct
// configured signatures. Signatures attest claims; they do not prove those
// claims. No provider reads, role/state checks, outer signing or writes occur.
func PrepareNativeResolutionMessage(config godbridge.Config, request NativeResolutionRequest) (out NativeResolutionMessage, err error) {
	defer func() {
		if recover() != nil {
			out, err = NativeResolutionMessage{}, ErrNativeResolutionMessage
		}
	}()
	fail := func() (NativeResolutionMessage, error) { return NativeResolutionMessage{}, ErrNativeResolutionMessage }
	raw, err := godaddress.FromNative(request.Sender)
	if err != nil || bytes.Equal(raw, make([]byte, godaddress.AccountBytes)) {
		return fail()
	}
	canonical, err := godaddress.ToNative(raw)
	if err != nil || canonical != request.Sender || !unfinishedWithdrawal(request.Withdrawal) ||
		request.Outcome == godbridge.Paid && request.Withdrawal.Status != godbridge.Authorized ||
		len(request.Approvals) < godbridge.Threshold || len(request.Approvals) > godbridge.SignerCount {
		return fail()
	}
	for _, signature := range request.Approvals {
		if len(signature) != 65 {
			return fail()
		}
	}
	digest, err := godbridge.ResolutionAttestationDigest(config, request.Withdrawal, request.Outcome, request.Evidence)
	if err != nil || digest != request.AttestationDigest {
		return fail()
	}
	approvals := cloneResolutionApprovals(request.Approvals)
	if godbridge.VerifyAttestation(config, digest, approvals) != nil {
		return fail()
	}
	out = NativeResolutionMessage{request.Outcome, request.Sender, request.Withdrawal.Sequence, request.Withdrawal.ID, request.Evidence, approvals}
	message, err := out.Payload()
	if err != nil || bridgemsg.Validate(message) != nil {
		return fail()
	}
	return out, nil
}

// Connects an existing observation/outbox proposal to the exact SDK message
// constructor. It never consumes/reopens/releases an outbox, fetches approvals,
// signs or submits anything; failed approval review leaves an unknown slot so.
func (o ResolutionObservation) PrepareNativeMessage(config Config, sender string, approvals [][]byte) (NativeResolutionMessage, error) {
	w, outcome, evidence, digest, _, err := o.Proposal()
	if err != nil {
		return NativeResolutionMessage{}, ErrNativeResolutionMessage
	}
	binding, err := config.Binding()
	if err != nil {
		return NativeResolutionMessage{}, ErrNativeResolutionMessage
	}
	return PrepareNativeResolutionMessage(binding, NativeResolutionRequest{sender, w, outcome, evidence, digest, approvals})
}
