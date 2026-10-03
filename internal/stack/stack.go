// Package stack records the imported infrastructure, not a running node.
package stack

import (
	"runtime/debug"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	cmtversion "github.com/cometbft/cometbft/version"
	sdkversion "github.com/cosmos/cosmos-sdk/version"
	evmversion "github.com/cosmos/evm/version"
)

type Status struct {
	Project             string            `json:"project"`
	Stage               string            `json:"stage"`
	SDK                 string            `json:"sdk"`
	Consensus           string            `json:"consensus"`
	Execution           string            `json:"execution"`
	NativeDenom         string            `json:"nativeDenom"`
	AccountPrefix       string            `json:"accountAddressPrefix"`
	NativeAddressFormat string            `json:"nativeAddressFormat"`
	EVMAddressFormat    string            `json:"evmAddressFormat"`
	RealAssets          bool              `json:"realAssets"`
	NodeReady           bool              `json:"nodeReady"`
	Dependencies        map[string]string `json:"dependencies"`
}

func BuildStatus() Status {
	_ = sdkversion.Name
	_ = evmversion.Version
	status := Status{
		Project: "GOD Chain", Stage: "native-transactions-prototype",
		SDK: "God SDK", Consensus: "GodCometBFT", Execution: "God EVM",
		NativeDenom: "agod", RealAssets: false, NodeReady: false,
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
