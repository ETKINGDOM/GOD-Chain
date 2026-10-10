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
	Project                  string            `json:"project"`
	Stage                    string            `json:"stage"`
	SDK                      string            `json:"sdk"`
	Consensus                string            `json:"consensus"`
	Execution                string            `json:"execution"`
	NativeDenom              string            `json:"nativeDenom"`
	AccountPrefix            string            `json:"accountAddressPrefix"`
	NativeAddressFormat      string            `json:"nativeAddressFormat"`
	EVMAddressFormat         string            `json:"evmAddressFormat"`
	RealAssets               bool              `json:"realAssets"`
	NodeReady                bool              `json:"nodeReady"`
	LocalPrototype           bool              `json:"localNodePrototype"`
	CommittedQueries         bool              `json:"localCommittedQueriesImplemented"`
	SyntheticNodeCommands    bool              `json:"syntheticNodeCommandsImplemented"`
	RestrictedTestnetRPC     bool              `json:"restrictedTestnetRPCImplemented"`
	IndependentOperatorSetup bool              `json:"syntheticIndependentOperatorSetupImplemented"`
	RHPrivateConfig          bool              `json:"rhPrivateConfigurationImplemented"`
	RHReadOnlyProbe          bool              `json:"rhReadOnlyProbeImplemented"`
	RHTokenInspection        bool              `json:"rhReadOnlyTokenInspectionImplemented"`
	RHTokenSourceMaterials   bool              `json:"rhOfflineTokenSourceMaterialsImplemented"`
	RHTokenRecompilation     bool              `json:"rhOfflineTokenRecompilationImplemented"`
	RHDepositObserver        bool              `json:"rhDepositObservationImplemented"`
	RHResolutionObserver     bool              `json:"rhResolutionObservationImplemented"`
	RHReadOnlyJournal        bool              `json:"rhReadOnlyJournalImplemented"`
	RHEventDiscovery         bool              `json:"rhEventDiscoveryImplemented"`
	RHReceiptSets            bool              `json:"rhSimulationReceiptSetsImplemented"`
	RHReceiptInclusion       bool              `json:"rhSimulationReceiptInclusionImplemented"`
	RHProofPreparation       bool              `json:"rhSimulationReceiptProofPreparationImplemented"`
	RHRelayProofPreparation  bool              `json:"rhSimulationTaskReceiptProofPreparationImplemented"`
	RHProofPersistence       bool              `json:"rhPrivateTaskReceiptProofStorageImplemented"`
	RHMaterialHTTP           bool              `json:"rhReadOnlyMaterialHTTPAdapterImplemented"`
	RHTaskProofFetch         bool              `json:"rhReadOnlyTaskProofFetchImplemented"`
	RHRetainedProofSource    bool              `json:"rhReadOnlyRetainedProofSourceCheckImplemented"`
	RHTaskEvidenceReview     bool              `json:"rhReadOnlyTaskEvidenceReviewImplemented"`
	RHSourceFinality         bool              `json:"rhSourceFinalityImplemented"`
	Dependencies             map[string]string `json:"dependencies"`
}

func BuildStatus() Status {
	_ = sdkversion.Name
	_ = evmversion.Version
	status := Status{
		Project: "GOD Chain", Stage: godnode.ImplementationStage,
		SDK: "God SDK", Consensus: "GodCometBFT", Execution: "God EVM",
		NativeDenom: "agod", RealAssets: false, NodeReady: false, LocalPrototype: true, CommittedQueries: true,
		SyntheticNodeCommands: true, RestrictedTestnetRPC: true, IndependentOperatorSetup: true,
		RHPrivateConfig: true, RHReadOnlyProbe: true, RHTokenInspection: true, RHTokenSourceMaterials: true, RHTokenRecompilation: true, RHDepositObserver: true, RHResolutionObserver: true, RHReadOnlyJournal: true, RHEventDiscovery: true, RHReceiptSets: true, RHReceiptInclusion: true, RHProofPreparation: true, RHRelayProofPreparation: true, RHProofPersistence: true, RHMaterialHTTP: true, RHTaskProofFetch: true, RHRetainedProofSource: true, RHTaskEvidenceReview: true, RHSourceFinality: false,
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
