package godaddress

import sdk "github.com/cosmos/cosmos-sdk/types"

// ConfigureSDK sets process-wide serialization prefixes before Config.Seal and
// before account keepers or wallets are created. It is startup-only, not a
// runtime migration. Key algorithms, HD paths and coin types are unchanged.
func ConfigureSDK(config *sdk.Config) {
	if config == nil {
		panic("GOD SDK configuration is required")
	}
	config.SetBech32PrefixForAccount(AccountPrefix, AccountPrefix+sdk.PrefixPublic)
	config.SetBech32PrefixForValidator(ValidatorOperatorPrefix, ValidatorOperatorPrefix+sdk.PrefixPublic)
	config.SetBech32PrefixForConsensusNode(ConsensusPrefix, ConsensusPrefix+sdk.PrefixPublic)
}
