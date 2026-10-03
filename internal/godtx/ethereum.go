package godtx

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	txproto "github.com/cosmos/cosmos-sdk/types/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

const EthereumExtension = "/cosmos.evm.vm.v1.ExtensionOptionsEthereumTx"

func protoEnvelope(tx sdk.Tx) *txproto.Tx {
	provider, ok := tx.(interface{ GetProtoTx() *txproto.Tx })
	if !ok {
		return nil
	}
	return provider.GetProtoTx()
}

func IsEthereum(tx sdk.Tx) bool {
	p := protoEnvelope(tx)
	return p != nil && p.Body != nil && len(p.Body.ExtensionOptions) == 1 &&
		p.Body.ExtensionOptions[0] != nil && p.Body.ExtensionOptions[0].TypeUrl == EthereumExtension
}

// ValidateEthereumStructure runs before upstream getters and ante logic. An
// Ethereum signature lives inside the one raw transaction, not SignerInfos.
func ValidateEthereumStructure(tx sdk.Tx) error {
	p := protoEnvelope(tx)
	if !IsEthereum(tx) || p.AuthInfo == nil || p.AuthInfo.Fee == nil || len(p.Body.Messages) != 1 ||
		p.Body.Messages[0] == nil || p.Body.Messages[0].TypeUrl != sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}) ||
		len(p.Body.ExtensionOptions[0].Value) != 0 || len(p.Body.NonCriticalExtensionOptions) != 0 ||
		p.Body.Memo != "" || p.Body.TimeoutHeight != 0 || p.Body.Unordered || p.Body.TimeoutTimestamp != nil ||
		len(p.AuthInfo.SignerInfos) != 0 || len(p.Signatures) != 0 || p.AuthInfo.Fee.Payer != "" || p.AuthInfo.Fee.Granter != "" {
		return ErrPolicy
	}
	fees := p.AuthInfo.Fee.Amount
	if !fees.IsValid() || len(fees) != 1 {
		return ErrFee
	}
	messages := tx.GetMsgs()
	if len(messages) != 1 {
		return ErrPolicy
	}
	m, ok := messages[0].(*evmtypes.MsgEthereumTx)
	if !ok || m == nil || m.Raw.Transaction == nil || len(m.From) != 20 || m.ValidateBasic() != nil {
		return ErrPolicy
	}
	return nil
}
