package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math/big"
	"unicode/utf8"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
)

const MaxLaunchEconomicsBytes = 16 << 10
const launchEconomicsPurpose = "god-chain-launch-economics-candidate-v1"

var ErrLaunchEconomics = errors.New("launch economics candidate rejected; no owner approval, funding evidence or activation")

type launchEconomicsDocument struct {
	Purpose                        string `json:"purpose"`
	Version                        uint32 `json:"version"`
	PhysicalHosts                  string `json:"physicalHosts"`
	OperatorCustodians             string `json:"operatorCustodians"`
	InitialValidators              string `json:"initialValidators"`
	GPerSignedBlockSmallestUnits   string `json:"gPerSignedBlockSmallestUnits"`
	AssumedBlockSeconds            string `json:"assumedBlockSeconds"`
	MinFeePerGasSmallestUnits      string `json:"minFeePerGasSmallestUnits"`
	StartupGODSmallestUnits        string `json:"startupGODSmallestUnits"`
	GasWorkingGODSmallestUnits     string `json:"gasWorkingGODSmallestUnits"`
	RedemptionSeedGODSmallestUnits string `json:"redemptionSeedGODSmallestUnits"`
	SourceFeeBudgetWei             string `json:"sourceFeeBudgetWei"`
}

// Candidate only: user-approved one-host founder operation is declared, NOT
// verified. Monetary choices remain blank rather than copying test values.
func LaunchEconomicsTemplate() []byte {
	raw, _ := json.MarshalIndent(launchEconomicsDocument{Purpose: launchEconomicsPurpose, Version: 1,
		PhysicalHosts: "1", OperatorCustodians: "1"}, "", "  ")
	return append(raw, '\n')
}

type LaunchEconomicsReport struct {
	Baseline                             godnode.EconomicBaseline `json:"currentPrototypeBaseline"`
	CandidatePinned                      bool                     `json:"candidatePinned"`
	DeclaredSingleHostFounderControl     bool                     `json:"declaredSingleHostFounderControl"`
	InitialValidators                    uint64                   `json:"initialValidators"`
	RequiredStakeSmallestUnits           string                   `json:"requiredStakeSmallestUnits"`
	RequiredStartupGODSmallestUnits      string                   `json:"requiredStartupGODSmallestUnits"`
	NominalBlocksPerDay                  uint64                   `json:"nominalBlocksPerDay"`
	NominalDailyGSmallestUnits           string                   `json:"nominalDailyGSmallestUnits"`
	CappedNominalDailyGSmallestUnits     string                   `json:"cappedNominalDailyGSmallestUnits"`
	DailyCapWouldApplyAtAssumedCadence   bool                     `json:"dailyCapWouldApplyAtAssumedCadence"`
	PlainTransferMinimumFeeSmallestUnits string                   `json:"plainTransferMinimumFeeSmallestUnits"`
	StartupFitsDeclaredBudget            bool                     `json:"startupFitsDeclaredBudget"`
	BudgetFitsCurrentBridgeExposure      bool                     `json:"budgetFitsCurrentBridgeExposure"`
	WorkingGasBudgetDeclared             bool                     `json:"workingGasBudgetDeclared"`
	WorkingGasCoversOnePlainTransfer     bool                     `json:"workingGasCoversOnePlainTransfer"`
	SourceFeeBudgetDeclared              bool                     `json:"sourceFeeBudgetDeclared"`
	ArithmeticConsistent                 bool                     `json:"arithmeticConsistent"`
	EmptyPoolPreservesG                  bool                     `json:"emptyPoolPreservesG"`
	BurnOnlyAfterSuccessfulRedemption    bool                     `json:"burnOnlyAfterSuccessfulRedemption"`
	GHasLifetimeCap                      bool                     `json:"gHasLifetimeCap"`
	TopologyIndependentlyVerified        bool                     `json:"topologyIndependentlyVerified"`
	SourceBackingVerified                bool                     `json:"sourceBackingVerified"`
	SourceFeesMeasured                   bool                     `json:"sourceFeesMeasured"`
	CadenceMeasured                      bool                     `json:"cadenceMeasured"`
	OwnerApproved                        bool                     `json:"ownerApproved"`
	ProductionConfigurationApplied       bool                     `json:"productionConfigurationApplied"`
	RealAssetsReady                      bool                     `json:"realAssetsReady"`
}

// Exact decimal text, including zero only when explicitly allowed. No floats,
// whitespace, signs, exponents, leading zeros or values above uint256.
func launchEconomicsAmount(s string, zero bool) (*big.Int, error) {
	if len(s) == 0 || len(s) > 78 || len(s) > 1 && s[0] == '0' {
		return nil, ErrLaunchEconomics
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return nil, ErrLaunchEconomics
		}
	}
	n, ok := new(big.Int).SetString(s, 10)
	if !ok || n.BitLen() > 256 || !zero && n.Sign() == 0 {
		return nil, ErrLaunchEconomics
	}
	return n, nil
}

// Read-only arithmetic review against CURRENT prototype constants. This is a
// plan, not an account balance, verified backing, fee quote, decentralization
// proof, owner approval or production-genesis initializer. Public reports do
// not expose the funding budget, addresses, keys, source paths or file pins.
func ReviewLaunchEconomics(ctx context.Context, raw []byte, pin [32]byte) (report LaunchEconomicsReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = LaunchEconomicsReport{}, ErrLaunchEconomics
		}
	}()
	fail := func() (LaunchEconomicsReport, error) { return LaunchEconomicsReport{}, ErrLaunchEconomics }
	if len(raw) == 0 || len(raw) > MaxLaunchEconomicsBytes {
		return fail()
	}
	raw = bytes.Clone(raw)
	if ctx == nil || ctx.Err() != nil || pin == [32]byte{} || sha256.Sum256(raw) != pin || !utf8.Valid(raw) || !uniqueTokenSourceJSON(raw) {
		return fail()
	}
	fields := []string{"purpose", "version", "physicalHosts", "operatorCustodians", "initialValidators", "gPerSignedBlockSmallestUnits", "assumedBlockSeconds", "minFeePerGasSmallestUnits", "startupGODSmallestUnits", "gasWorkingGODSmallestUnits", "redemptionSeedGODSmallestUnits", "sourceFeeBudgetWei"}
	if _, err := custodyObject(raw, fields, nil); err != nil {
		return fail()
	}
	var d launchEconomicsDocument
	if json.Unmarshal(raw, &d) != nil || d.Version != 1 || d.Purpose != launchEconomicsPurpose || d.PhysicalHosts != "1" || d.OperatorCustodians != "1" {
		return fail()
	}
	baseline := godnode.CurrentEconomicBaseline()
	validators, err := ParentMaterialSlot(d.InitialValidators)
	if err != nil || validators == 0 || validators > uint64(baseline.MaximumValidators) {
		return fail()
	}
	seconds, err := ParentMaterialSlot(d.AssumedBlockSeconds)
	if err != nil || seconds == 0 || seconds > 86400 {
		return fail()
	}
	amounts := make([]*big.Int, 6)
	for i, text := range []string{d.GPerSignedBlockSmallestUnits, d.MinFeePerGasSmallestUnits, d.StartupGODSmallestUnits, d.GasWorkingGODSmallestUnits, d.RedemptionSeedGODSmallestUnits, d.SourceFeeBudgetWei} {
		amounts[i], err = launchEconomicsAmount(text, i >= 2)
		if err != nil {
			return fail()
		}
	}
	g, fee, budget, working, seed, sourceFee := amounts[0], amounts[1], amounts[2], amounts[3], amounts[4], amounts[5]
	ceiling, _ := new(big.Int).SetString(baseline.DailyGCeilingSmallestUnits, 10)
	supply, _ := new(big.Int).SetString(baseline.FixedGODSmallestUnits, 10)
	exposure, _ := new(big.Int).SetString(baseline.BridgeExposureSmallestUnits, 10)
	stake, _ := new(big.Int).SetString(baseline.MinimumSelfStakeSmallestUnits, 10)
	if g.Cmp(ceiling) > 0 || budget.Cmp(supply) > 0 || working.Cmp(supply) > 0 || seed.Cmp(supply) > 0 {
		return fail()
	}
	stake.Mul(stake, new(big.Int).SetUint64(validators))
	required := new(big.Int).Add(new(big.Int).Add(stake, working), seed)
	blocks := (86400 + seconds - 1) / seconds
	daily := new(big.Int).Mul(g, new(big.Int).SetUint64(blocks))
	capped := new(big.Int).Set(daily)
	if capped.Cmp(ceiling) > 0 {
		capped.Set(ceiling)
	}
	transferFee := new(big.Int).Mul(fee, big.NewInt(21000))
	r := LaunchEconomicsReport{Baseline: baseline, CandidatePinned: true, DeclaredSingleHostFounderControl: true, InitialValidators: validators,
		RequiredStakeSmallestUnits: stake.String(), RequiredStartupGODSmallestUnits: required.String(), NominalBlocksPerDay: blocks,
		NominalDailyGSmallestUnits: daily.String(), CappedNominalDailyGSmallestUnits: capped.String(), DailyCapWouldApplyAtAssumedCadence: daily.Cmp(ceiling) > 0,
		PlainTransferMinimumFeeSmallestUnits: transferFee.String(), StartupFitsDeclaredBudget: required.Cmp(budget) <= 0,
		BudgetFitsCurrentBridgeExposure: budget.Cmp(exposure) <= 0, WorkingGasBudgetDeclared: working.Sign() > 0,
		WorkingGasCoversOnePlainTransfer: working.Cmp(transferFee) >= 0, SourceFeeBudgetDeclared: sourceFee.Sign() > 0,
		EmptyPoolPreservesG: true, BurnOnlyAfterSuccessfulRedemption: true}
	r.ArithmeticConsistent = r.StartupFitsDeclaredBudget && r.BudgetFitsCurrentBridgeExposure && r.WorkingGasBudgetDeclared && r.WorkingGasCoversOnePlainTransfer && r.SourceFeeBudgetDeclared
	if ctx.Err() != nil {
		return fail()
	}
	return r, nil
}

// Owner-only private POSIX snapshot; repeat exact original pin after review.
// No writes, network, approval file, signing, service restart or activation.
func ReviewPrivateLaunchEconomics(ctx context.Context, path string, pin [32]byte) (report LaunchEconomicsReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = LaunchEconomicsReport{}, ErrLaunchEconomics
		}
	}()
	raw, err := readRHPrivateSnapshot(path, MaxLaunchEconomicsBytes, pin)
	if err != nil {
		return LaunchEconomicsReport{}, ErrLaunchEconomics
	}
	r, err := ReviewLaunchEconomics(ctx, raw, pin)
	if err != nil {
		return LaunchEconomicsReport{}, ErrLaunchEconomics
	}
	again, err := readRHPrivateSnapshot(path, MaxLaunchEconomicsBytes, pin)
	if err != nil || !bytes.Equal(raw, again) || ctx.Err() != nil {
		return LaunchEconomicsReport{}, ErrLaunchEconomics
	}
	return r, nil
}
