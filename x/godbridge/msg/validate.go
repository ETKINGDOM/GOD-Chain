// Package msg adapts signed God SDK transactions to the bridge ledger. These
// services are trusted adapters, never standalone authentication endpoints.
package msg

import (
	"bytes"
	"errors"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	rewardmsg "github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var ErrMessage = errors.New("unsupported or invalid bridge message")

func nonzero(raw []byte, size int) bool {
	return len(raw) == size && !bytes.Equal(raw, make([]byte, size))
}

func amount(text string) (sdkmath.Int, error) {
	n, err := rewardmsg.Amount(text)
	if err != nil || n.GT(godbridge.TransferLimit()) {
		return sdkmath.Int{}, ErrMessage
	}
	return n, nil
}

func evidence(e *SourceEvidence) (godbridge.Evidence, error) {
	if e == nil || e.Height == 0 || !nonzero(e.BlockHash, 32) || !nonzero(e.TransactionHash, 32) {
		return godbridge.Evidence{}, ErrMessage
	}
	return godbridge.Evidence{Height: e.Height, BlockHash: [32]byte(e.BlockHash), TransactionHash: [32]byte(e.TransactionHash), LogIndex: e.LogIndex}, nil
}

func approvals(signatures [][]byte) error {
	if len(signatures) < godbridge.Threshold || len(signatures) > godbridge.SignerCount {
		return ErrMessage
	}
	for _, signature := range signatures {
		if len(signature) != 65 || signature[64] > 1 {
			return ErrMessage
		}
	}
	return nil
}

func withdrawal(sequence uint64, id []byte) error {
	if sequence == 0 || !nonzero(id, 32) {
		return ErrMessage
	}
	return nil
}

// Validate bounds and canonicalizes structure, not trust. The native ante
// verifies sender ownership; the keeper separately verifies quorum membership,
// the action-specific digest, limits, delay and exact-once state transition.
func Validate(message sdk.Msg) error {
	var sender string
	var signatures [][]byte
	switch m := message.(type) {
	case *MsgAcceptDeposit:
		if m == nil || m.DepositSequence == 0 {
			return ErrMessage
		}
		sender, signatures = m.Sender, m.Approvals
		recipient, err := rewardmsg.Account(m.Recipient)
		if err != nil || !nonzero(recipient, 20) {
			return ErrMessage
		}
		if _, err := amount(m.Amount); err != nil {
			return err
		}
		if _, err := evidence(m.Evidence); err != nil {
			return err
		}
	case *MsgRequestWithdrawal:
		if m == nil || !nonzero(m.SourceRecipient, 20) {
			return ErrMessage
		}
		sender = m.Sender
		if _, err := amount(m.Amount); err != nil {
			return err
		}
	case *MsgAuthorizeWithdrawal:
		if m == nil || withdrawal(m.WithdrawalSequence, m.WithdrawalId) != nil {
			return ErrMessage
		}
		sender, signatures = m.Sender, m.Approvals
	case *MsgResolvePayment:
		if m == nil || withdrawal(m.WithdrawalSequence, m.WithdrawalId) != nil {
			return ErrMessage
		}
		sender, signatures = m.Sender, m.Approvals
		if _, err := evidence(m.Evidence); err != nil {
			return err
		}
	case *MsgCancelWithdrawal:
		if m == nil || withdrawal(m.WithdrawalSequence, m.WithdrawalId) != nil {
			return ErrMessage
		}
		sender, signatures = m.Sender, m.Approvals
		if _, err := evidence(m.Evidence); err != nil {
			return err
		}
	case *MsgPauseBridge:
		if m == nil || m.ControlNonce == 0 || !m.Intake && !m.Outflow {
			return ErrMessage
		}
		sender, signatures = m.Sender, m.Approvals
	default:
		return ErrMessage
	}
	address, err := rewardmsg.Account(sender)
	if err != nil || !nonzero(address, 20) {
		return ErrMessage
	}
	if _, ok := message.(*MsgRequestWithdrawal); !ok {
		return approvals(signatures)
	}
	return nil
}

func (m *MsgAcceptDeposit) ValidateBasic() error       { return Validate(m) }
func (m *MsgRequestWithdrawal) ValidateBasic() error   { return Validate(m) }
func (m *MsgAuthorizeWithdrawal) ValidateBasic() error { return Validate(m) }
func (m *MsgResolvePayment) ValidateBasic() error      { return Validate(m) }
func (m *MsgCancelWithdrawal) ValidateBasic() error    { return Validate(m) }
func (m *MsgPauseBridge) ValidateBasic() error         { return Validate(m) }
