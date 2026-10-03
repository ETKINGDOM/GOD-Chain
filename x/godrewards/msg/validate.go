// Package msg adapts authenticated God SDK messages to the internal G keeper.
// Register its service only on a transaction router protected by godtx ante.
package msg

import (
	"errors"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	ErrMessage = errors.New("unsupported or invalid G message")
	ErrAddress = errors.New("transaction address must be canonical GOD account format")
	ErrAmount  = errors.New("amount must be a canonical positive bounded integer")
	ErrLock    = errors.New("invalid lock identifier")
)

// Account deliberately requires canonical native transaction text, although
// the presentation codec also accepts uppercase Bech32 outside transactions.
func Account(value string) (sdk.AccAddress, error) {
	raw, err := godaddress.FromNative(value)
	if err != nil {
		return nil, ErrAddress
	}
	canonical, err := godaddress.ToNative(raw)
	if err != nil || canonical != value {
		return nil, ErrAddress
	}
	return sdk.AccAddress(raw), nil
}

func Amount(value string) (sdkmath.Int, error) {
	// 2^256-1 has 78 decimal digits. Bound before parsing untrusted text.
	if len(value) == 0 || len(value) > 78 || value[0] == '0' {
		return sdkmath.Int{}, ErrAmount
	}
	for _, c := range value {
		if c < '0' || c > '9' {
			return sdkmath.Int{}, ErrAmount
		}
	}
	n, ok := sdkmath.NewIntFromString(value)
	if !ok || !n.IsPositive() {
		return sdkmath.Int{}, ErrAmount
	}
	return n, nil
}

func lockID(id string) error {
	if len(id) == 0 || len(id) > 64 {
		return ErrLock
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return ErrLock
		}
	}
	return nil
}

// Validate rejects unknown messages. This is also the native ante allowlist:
// no public mint, emission, bridge, bank-send or authority message is enabled.
func Validate(message sdk.Msg) error {
	var sender string
	switch m := message.(type) {
	case *MsgClaimG:
		if m == nil {
			return ErrMessage
		}
		sender = m.Sender
	case *MsgTransferG:
		if m == nil {
			return ErrMessage
		}
		sender = m.Sender
		if _, err := Account(m.Recipient); err != nil {
			return err
		}
		if _, err := Amount(m.Amount); err != nil {
			return err
		}
	case *MsgLockG:
		if m == nil {
			return ErrMessage
		}
		sender = m.Sender
		if err := lockID(m.LockId); err != nil {
			return err
		}
		if _, err := Amount(m.Amount); err != nil {
			return err
		}
	case *MsgUnlockG:
		if m == nil {
			return ErrMessage
		}
		sender = m.Sender
		if err := lockID(m.LockId); err != nil {
			return err
		}
	case *MsgRedeemG:
		if m == nil {
			return ErrMessage
		}
		sender = m.Sender
		if _, err := Account(m.Beneficiary); err != nil {
			return err
		}
		if _, err := Amount(m.Amount); err != nil {
			return err
		}
		if _, err := Amount(m.MinGodOut); err != nil {
			return err
		}
		if m.DeadlineUnixNanos <= 0 {
			return ErrMessage
		}
	case *MsgDonateGod:
		if m == nil {
			return ErrMessage
		}
		sender = m.Sender
		if _, err := Amount(m.Amount); err != nil {
			return err
		}
	default:
		return ErrMessage
	}
	_, err := Account(sender)
	return err
}

func (m *MsgClaimG) ValidateBasic() error    { return Validate(m) }
func (m *MsgTransferG) ValidateBasic() error { return Validate(m) }
func (m *MsgLockG) ValidateBasic() error     { return Validate(m) }
func (m *MsgUnlockG) ValidateBasic() error   { return Validate(m) }
func (m *MsgRedeemG) ValidateBasic() error   { return Validate(m) }
func (m *MsgDonateGod) ValidateBasic() error { return Validate(m) }
