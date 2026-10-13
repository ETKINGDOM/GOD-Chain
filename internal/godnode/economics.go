package godnode

import (
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
)

// EconomicBaseline is a detached read-only description of CURRENT prototype
// policy, not an approved mainnet configuration or a runtime mutation API.
type EconomicBaseline struct {
	PrototypeOnly                   bool   `json:"prototypeOnly"`
	FixedGODSmallestUnits           string `json:"fixedGODSmallestUnits"`
	DailyGCeilingSmallestUnits      string `json:"dailyGCeilingSmallestUnits"`
	MinimumSelfStakeSmallestUnits   string `json:"minimumSelfStakeSmallestUnits"`
	MaximumValidators               uint32 `json:"maximumValidators"`
	UnbondingSeconds                int64  `json:"unbondingSeconds"`
	MinimumCommission               string `json:"minimumCommission"`
	SigningWindow                   int64  `json:"signingWindow"`
	MaximumMissedOpportunities      int64  `json:"maximumMissedOpportunities"`
	MinimumSignedFraction           string `json:"minimumSignedFraction"`
	SuspensionSeconds               int64  `json:"suspensionSeconds"`
	DoubleSignSlashFraction         string `json:"doubleSignSlashFraction"`
	DowntimeSlashFraction           string `json:"downtimeSlashFraction"`
	BridgeThreshold                 int    `json:"bridgeThreshold"`
	BridgeKeyCount                  int    `json:"bridgeKeyCount"`
	BridgeExposureSmallestUnits     string `json:"bridgeExposureSmallestUnits"`
	BridgeTransferSmallestUnits     string `json:"bridgeTransferSmallestUnits"`
	BridgeDailyOutflowSmallestUnits string `json:"bridgeDailyOutflowSmallestUnits"`
	WithdrawalDelaySeconds          int64  `json:"withdrawalDelaySeconds"`
}

func CurrentEconomicBaseline() EconomicBaseline {
	staking, penalties := stakingGenesisParams(), penaltyParams()
	return EconomicBaseline{PrototypeOnly: true, FixedGODSmallestUnits: godrewards.FixedGodSupply().String(),
		DailyGCeilingSmallestUnits: godrewards.DailyGCeiling().String(), MinimumSelfStakeSmallestUnits: minSelfStake().String(),
		MaximumValidators: staking.MaxValidators, UnbondingSeconds: int64(staking.UnbondingTime.Seconds()), MinimumCommission: staking.MinCommissionRate.String(),
		SigningWindow: penalties.SignedBlocksWindow, MaximumMissedOpportunities: maxMissed, MinimumSignedFraction: penalties.MinSignedPerWindow.String(),
		SuspensionSeconds: int64(penalties.DowntimeJailDuration.Seconds()), DoubleSignSlashFraction: penalties.SlashFractionDoubleSign.String(),
		DowntimeSlashFraction: penalties.SlashFractionDowntime.String(), BridgeThreshold: godbridge.Threshold, BridgeKeyCount: godbridge.SignerCount,
		BridgeExposureSmallestUnits: godbridge.ExposureLimit().String(), BridgeTransferSmallestUnits: godbridge.TransferLimit().String(),
		BridgeDailyOutflowSmallestUnits: godbridge.OutflowLimit().String(), WithdrawalDelaySeconds: int64(godbridge.WithdrawalDelay.Seconds())}
}
