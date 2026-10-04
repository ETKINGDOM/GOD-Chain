// Package stack reports local implementation scope, not a live network.
package stack

import (
	"runtime/debug"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
	cmtversion "github.com/cometbft/cometbft/version"
	sdkversion "github.com/cosmos/cosmos-sdk/version"
	evmversion "github.com/cosmos/evm/version"
)

type Status struct {
	Project              string            `json:"project"`
	Stage                string            `json:"stage"`
	SDK                  string            `json:"sdk"`
	Consensus            string            `json:"consensus"`
	Execution            string            `json:"execution"`
	NativeDenom          string            `json:"nativeDenom"`
	AccountPrefix        string            `json:"accountAddressPrefix"`
	NativeAddressFormat  string            `json:"nativeAddressFormat"`
	EVMAddressFormat     string            `json:"evmAddressFormat"`
	RealAssets           bool              `json:"realAssets"`
	NodeReady            bool              `json:"nodeReady"`
	LocalPrototype       bool              `json:"localNodePrototype"`
	RHPrivateConfig      bool              `json:"rhPrivateConfigurationImplemented"`
	RHReadOnlyProbe      bool              `json:"rhReadOnlyProbeImplemented"`
	RHDepositObserver    bool              `json:"rhDepositObservationImplemented"`
	RHResolutionObserver bool              `json:"rhResolutionObservationImplemented"`
	RHReadOnlyJournal    bool              `json:"rhReadOnlyJournalImplemented"`
	RHEventDiscovery     bool              `json:"rhEventDiscoveryImplemented"`
	RHReceiptSets        bool              `json:"rhSimulationReceiptSetsImplemented"`
	RHSourceFinality     bool              `json:"rhSourceFinalityImplemented"`
	Dependencies         map[string]string `json:"dependencies"`
}

func BuildStatus() Status {
	_ = sdkversion.Name
	_ = evmversion.Version
	status := Status{
		Project: "GOD Chain", Stage: godnode.ImplementationStage,
		SDK: "God SDK", Consensus: "GodCometBFT", Execution: "God EVM",
		NativeDenom: "agod", RealAssets: false, NodeReady: false, LocalPrototype: true,
		RHPrivateConfig: true, RHReadOnlyProbe: true, RHDepositObserver: true, RHResolutionObserver: true, RHReadOnlyJournal: true, RHEventDiscovery: true, RHReceiptSets: true, RHSourceFinality: false,
		AccountPrefix: godaddress.AccountPrefix, NativeAddressFormat: "Bech32 lowercase", EVMAddressFormat: "0x EIP-55 hex",
		Dependencies: map[string]string{},
	}
	_ = cmtversion.TMCoreSemVer
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			switch dep.Path {
			case "github.com/cosmos/cosmos-sdk", "github.com/cosmos/evm", "github.com/cometbft/cometbft":
				status.Dependencies[dep.Path] = dep.Version
			}
		}
	}
	return status
}
