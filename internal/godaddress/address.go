// Package godaddress defines GOD Chain account address encodings. It does not
// generate keys, derive accounts or prove ownership, balances or network access.
package godaddress

import (
	"encoding/hex"
	"errors"
	"strings"

	sdkbech32 "github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/ethereum/go-ethereum/common"
)

const (
	AccountPrefix           = "god"
	ValidatorOperatorPrefix = "godvaloper"
	ConsensusPrefix         = "godvalcons"
	AccountBytes            = 20
	NativeLength            = len(AccountPrefix) + 1 + AccountBytes*8/5 + 6
)

var (
	ErrAccountLength = errors.New("account address must contain exactly 20 bytes")
	ErrNativeAddress = errors.New("invalid GOD native account address")
	ErrEVMAddress    = errors.New("invalid EVM account address format")
	ErrEVMChecksum   = errors.New("invalid mixed-case EVM address checksum")
)

// Codec implements the God SDK account address codec interface. Every account
// has one raw 20-byte identifier; the encodings are not separate balance ledgers.
type Codec struct{}

func (Codec) StringToBytes(text string) ([]byte, error) { return FromNative(text) }
func (Codec) BytesToString(raw []byte) (string, error)  { return ToNative(raw) }

func ToNative(raw []byte) (string, error) {
	if len(raw) != AccountBytes {
		return "", ErrAccountLength
	}
	return sdkbech32.ConvertAndEncode(AccountPrefix, raw)
}

// FromNative accepts the canonical lowercase format and Bech32's all-uppercase
// presentation used by some QR encoders. Mixed case is never accepted. It
// rejects whitespace, other roles/prefixes and bad checksums.
// Errors deliberately contain no user-supplied address value.
func FromNative(text string) ([]byte, error) {
	if len(text) != NativeLength {
		return nil, ErrNativeAddress
	}
	lower := strings.ToLower(text)
	if (text != lower && text != strings.ToUpper(text)) || !strings.HasPrefix(lower, AccountPrefix+"1") {
		return nil, ErrNativeAddress
	}
	prefix, raw, err := sdkbech32.DecodeAndConvert(lower)
	if err != nil || prefix != AccountPrefix || len(raw) != AccountBytes {
		return nil, ErrNativeAddress
	}
	canonical, err := ToNative(raw)
	if err != nil || canonical != lower {
		return nil, ErrNativeAddress
	}
	return append([]byte(nil), raw...), nil
}

// ToEVM returns the existing reference library's EIP-55 checksum encoding.
func ToEVM(raw []byte) (string, error) {
	if len(raw) != AccountBytes {
		return "", ErrAccountLength
	}
	return common.BytesToAddress(raw).Hex(), nil
}

// FromEVM requires a full 0x-prefixed 20-byte address. Standard unchecksummed
// lowercase/uppercase forms are accepted; mixed-case input must pass EIP-55.
// It never silently truncates or left-pads malformed input.
func FromEVM(text string) ([]byte, error) {
	if len(text) != 2+AccountBytes*2 || !strings.HasPrefix(text, "0x") {
		return nil, ErrEVMAddress
	}
	payload := text[2:]
	raw, err := hex.DecodeString(payload)
	if err != nil || len(raw) != AccountBytes {
		return nil, ErrEVMAddress
	}
	if payload != strings.ToLower(payload) && payload != strings.ToUpper(payload) {
		canonical, err := ToEVM(raw)
		if err != nil || canonical != text {
			return nil, ErrEVMChecksum
		}
	}
	return raw, nil
}

func EVMToNative(text string) (string, error) {
	raw, err := FromEVM(text)
	if err != nil {
		return "", err
	}
	return ToNative(raw)
}

func NativeToEVM(text string) (string, error) {
	raw, err := FromNative(text)
	if err != nil {
		return "", err
	}
	return ToEVM(raw)
}
