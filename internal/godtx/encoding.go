// Package godtx provides a restricted native signed G transaction prototype.
// It does not assemble an EVM node or expose a network/signing endpoint.
package godtx

import (
	txsigning "cosmossdk.io/x/tx/signing"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	bridgemsg "github.com/ETKINGDOM/GOD-Chain/x/godbridge/msg"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txproto "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/gogoproto/proto"
)

type Encoding struct {
	Registry  codectypes.InterfaceRegistry
	Codec     *codec.ProtoCodec
	TxConfig  client.TxConfig
	execution bool
}

func NewEncoding() (Encoding, error) {
	return newEncoding(false, false)
}

// NewExecutionEncoding adds the signed Ethereum envelope and staking messages.
// Registration is not authorization: the node ante allowlist must still reject
// parameter updates, generic bank transfers and unrelated module messages.
func NewExecutionEncoding() (Encoding, error) { return newEncoding(true, false) }

// NewBridgeEncoding is an opt-in native transaction codec for isolated bridge
// integration. Default native/node codecs do not authorize or mount a bridge.
// Registration alone does not supply backed genesis or verify source finality.
func NewBridgeEncoding() (Encoding, error) { return newEncoding(false, true) }

// NewBridgeExecutionEncoding is explicitly opt-in for synthetic node bridge
// transactions. A configured ledger, authenticated ante and approval-gas policy
// are still required; registration does not verify source backing or finality.
func NewBridgeExecutionEncoding() (Encoding, error) { return newEncoding(true, true) }

func newEncoding(execution, bridge bool) (Encoding, error) {
	options := txsigning.Options{
		AddressCodec:          godaddress.Codec{},
		ValidatorAddressCodec: addresscodec.NewBech32Codec(godaddress.ValidatorOperatorPrefix),
	}
	if execution {
		options.DefineCustomGetSigners(evmtypes.MsgEthereumTxCustomGetSigner.MsgType, evmtypes.MsgEthereumTxCustomGetSigner.Fn)
	}
	registry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles:     proto.HybridResolver,
		SigningOptions: options,
	})
	if err != nil {
		return Encoding{}, err
	}
	authtypes.RegisterInterfaces(registry)
	cryptocodec.RegisterInterfaces(registry)
	// Never register an Ethereum private-key type in the transaction codec.
	registry.RegisterImplementations((*cryptotypes.PubKey)(nil), &ethsecp256k1.PubKey{})
	msg.RegisterInterfaces(registry)
	if bridge {
		bridgemsg.RegisterInterfaces(registry)
	}
	if execution {
		stakingtypes.RegisterInterfaces(registry)
		slashingtypes.RegisterInterfaces(registry)
		evmtypes.RegisterInterfaces(registry)
	}
	cdc := codec.NewProtoCodec(registry)
	config, err := authtx.NewTxConfigWithOptions(cdc, authtx.ConfigOptions{
		EnabledSignModes: []signing.SignMode{signing.SignMode_SIGN_MODE_DIRECT},
		SigningContext:   registry.SigningContext(),
	})
	if err != nil {
		return Encoding{}, err
	}
	return Encoding{Registry: registry, Codec: cdc, TxConfig: config, execution: execution}, nil
}

// Decoder bounds wire input before protobuf/Any decoding. BaseApp must use
// this decoder, not the unbounded TxConfig decoder, together with NewAnte.
func (e Encoding) Decoder(maxBytes int) sdk.TxDecoder {
	return func(wire []byte) (sdk.Tx, error) {
		if maxBytes <= 0 || maxBytes > 1<<20 || len(wire) == 0 || len(wire) > maxBytes {
			return nil, ErrPolicy
		}
		decoded, err := e.TxConfig.TxDecoder()(wire)
		if err != nil {
			return nil, err
		}
		if e.execution && IsEthereum(decoded) {
			if err := ValidateEthereumStructure(decoded); err != nil {
				return nil, err
			}
			return decoded, nil
		}
		if err := validateStructure(decoded); err != nil {
			return nil, err
		}
		return decoded, nil
	}
}

// The reference decoder accepts incomplete protobuf envelopes. Reject those
// before SDK getters read Fee or signature mode fields. No recovery blanket is
// used: malformed input is handled by explicit structural validation.
func validateStructure(decoded sdk.Tx) error {
	provider, ok := decoded.(interface{ GetProtoTx() *txproto.Tx })
	if !ok {
		return ErrPolicy
	}
	tx := provider.GetProtoTx()
	if tx == nil || tx.Body == nil || tx.AuthInfo == nil || tx.AuthInfo.Fee == nil ||
		len(tx.Body.Messages) == 0 || len(tx.Body.Messages) > 64 || len(tx.Signatures) != 1 || len(tx.AuthInfo.SignerInfos) != 1 {
		return ErrPolicy
	}
	fee := tx.AuthInfo.Fee
	// Do not call SDK FeeGranter() on untrusted text; it panics on bad encoding.
	if fee.Granter != "" {
		return ErrPolicy
	}
	if fee.Payer != "" {
		if _, err := msg.Account(fee.Payer); err != nil {
			return err
		}
	}
	info := tx.AuthInfo.SignerInfos[0]
	if info == nil || info.PublicKey == nil || info.ModeInfo == nil {
		return ErrKey
	}
	single, ok := info.ModeInfo.Sum.(*txproto.ModeInfo_Single_)
	if !ok || single == nil || single.Single == nil || single.Single.Mode != signing.SignMode_SIGN_MODE_DIRECT {
		return ErrKey
	}
	for _, message := range tx.Body.Messages {
		if message == nil {
			return ErrPolicy
		}
	}
	// This standard check validates fee integers and signer/signature counts.
	basic, ok := decoded.(sdk.HasValidateBasic)
	if !ok {
		return ErrPolicy
	}
	if err := basic.ValidateBasic(); err != nil {
		return ErrPolicy
	}
	return nil
}
