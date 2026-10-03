package godnode

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math/big"
	"sort"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	abci "github.com/cometbft/cometbft/abci/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

var ErrBridgeGenesis = errors.New("bridge node genesis authorization rejected")

// BridgeGenesisCertificate is a synthetic local genesis verification input,
// not a source proof. The opt-in node mounts and initializes a synthetic bridge
// ledger atomically; independent RH observation remains a real-asset gate.
// No populated certificate, signature, key or address belongs in public source.
type BridgeGenesisCertificate struct {
	Version       uint32                   `json:"version"`
	Plan          godbridge.BootstrapPlan  `json:"plan"`
	PlanApprovals [][]byte                 `json:"planApprovals"`
	NodeApprovals [][]byte                 `json:"nodeApprovals"`
	Consents      []BridgeValidatorConsent `json:"consents"`
}

type BridgeValidatorConsent struct {
	Owner              [20]byte `json:"owner"`
	OwnerSignature     []byte   `json:"ownerSignature"`
	ConsensusSignature []byte   `json:"consensusSignature"`
}

// BridgeGenesisDigest binds reconciled funding to the actual node policy,
// canonical participant/validator genesis, exact UTC time, normalized initial
// height and full consensus parameters. Signature witnesses are excluded to
// avoid circular signing. This is an unsigned proposal, not authorization.
func BridgeGenesisDigest(c Config, req *abci.RequestInitChain) ([32]byte, error) {
	if c.validate() != nil || c.BridgeGenesis == nil || validateBaseGenesisRequest(req, c) != nil {
		return [32]byte{}, ErrBridgeGenesis
	}
	g, err := decodeGenesis(req.AppStateBytes)
	if err != nil || g.Bridge == nil || g.Bridge.Version != 1 {
		return [32]byte{}, ErrBridgeGenesis
	}
	bridgeConfig, err := godbridge.CanonicalConfig(*c.BridgeGenesis)
	if err != nil {
		return [32]byte{}, ErrBridgeGenesis
	}
	c.BridgeGenesis = &bridgeConfig
	p, planDigest, err := godbridge.CanonicalBootstrap(bridgeConfig, g.Bridge.Plan)
	if err != nil || !p.GenesisTime.Equal(req.Time) || !matchingBridgeFunding(g, p) {
		return [32]byte{}, ErrBridgeGenesis
	}
	g.Bridge = nil
	consensus, err := req.ConsensusParams.Marshal()
	if err != nil {
		return [32]byte{}, ErrBridgeGenesis
	}
	fees, execution := executionGenesis(c)
	modules := nodeModuleNames(c)
	sort.Strings(modules)
	raw, err := json.Marshal(struct {
		Domain                string
		RuntimeBindingVersion uint32
		Config                Config
		Genesis               Genesis
		GenesisTime           string
		InitialHeight         int64
		Consensus             []byte
		Plan                  [32]byte
		Protocol              any
	}{
		Domain: "GOD Chain node genesis authorization v2", RuntimeBindingVersion: runtimeBindingVersion(c),
		Config: c, Genesis: g, GenesisTime: req.Time.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		InitialHeight: 1, Consensus: consensus, Plan: planDigest,
		Protocol: struct {
			Staking                                                    any
			Penalties                                                  any
			FeeGenesis                                                 any
			ExecutionGenesis                                           any
			MinimumSelfStake, PowerUnit, FixedGodSupply, DailyGCeiling string
			BridgeExposure, BridgeTransfer, BridgeOutflow              string
			BridgeWithdrawalDelay, BridgeOutflowWindow                 int64
			BridgeMaxOpenWithdrawals, BridgeMaxWindowEntries           int
			Modules                                                    []string
		}{stakingGenesisParams(), penaltyParams(), fees, execution,
			minSelfStake().String(), godrewards.Unit().String(), godrewards.FixedGodSupply().String(), godrewards.DailyGCeiling().String(),
			godbridge.ExposureLimit().String(), godbridge.TransferLimit().String(), godbridge.OutflowLimit().String(),
			int64(godbridge.WithdrawalDelay), int64(godbridge.OutflowWindow), godbridge.MaxOpenWithdrawals, godbridge.MaxWindowEntries, modules},
	})
	if err != nil {
		return [32]byte{}, ErrBridgeGenesis
	}
	return sha256.Sum256(raw), nil
}

func matchingBridgeFunding(g Genesis, p godbridge.BootstrapPlan) bool {
	balances := map[[20]byte]sdkmath.Int{}
	for _, d := range p.Deposits {
		// Protocol custody can never be a deposit-funded participant.
		if bytes.Equal(d.Deposit.Recipient[:], authtypes.NewModuleAddress(godbridge.PendingModule)) {
			return false
		}
		value, found := balances[d.Deposit.Recipient]
		if !found {
			value = sdkmath.ZeroInt()
		}
		balances[d.Deposit.Recipient] = value.Add(d.Deposit.Amount)
	}
	if len(g.Balances) != len(balances) || len(g.Validators) != len(p.Validators) {
		return false
	}
	for _, b := range g.Balances {
		address, err := msg.Account(b.Address)
		if err != nil || len(address) != 20 {
			return false
		}
		value, amountErr := godAmount(b.Amount)
		expected, found := balances[[20]byte(address)]
		if err != nil || amountErr != nil || !found || !value.Equal(expected) {
			return false
		}
	}
	validators := map[[20]byte]godbridge.BootstrapValidator{}
	for _, v := range p.Validators {
		validators[v.Owner] = v
	}
	for _, v := range g.Validators {
		owner, err := msg.Account(v.Owner)
		if err != nil || len(owner) != 20 {
			return false
		}
		expected, found := validators[[20]byte(owner)]
		stake, amountErr := godAmount(v.Stake)
		if err != nil || amountErr != nil || !found || !stake.Equal(expected.SelfStake) || !bytes.Equal(v.PublicKey, expected.ConsensusKey[:]) {
			return false
		}
	}
	return true
}

// BridgeValidatorConsentDigests keeps owner consent and consensus-key proof
// in separate roles and domains. The common node digest binds all stake/fee
// amounts, other validators, policy and genesis; it is not a consensus vote.
func BridgeValidatorConsentDigests(node [32]byte, owner [20]byte, consensusKey [32]byte) ([32]byte, [32]byte, error) {
	if node == [32]byte{} || owner == [20]byte{} || consensusKey == [32]byte{} {
		return [32]byte{}, [32]byte{}, ErrBridgeGenesis
	}
	digest := func(role string) [32]byte {
		p := append([]byte(role), node[:]...)
		p = append(p, owner[:]...)
		p = append(p, consensusKey[:]...)
		return sha256.Sum256(p)
	}
	return digest("GOD Chain validator owner consent v1"), digest("GOD Chain validator consensus possession v1"), nil
}

func validOwnerConsent(owner [20]byte, digest [32]byte, signature []byte) bool {
	if len(signature) != 65 || !ethcrypto.ValidateSignatureValues(signature[64], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:64]), true) {
		return false
	}
	pub, err := ethcrypto.SigToPub(digest[:], append([]byte(nil), signature...))
	return err == nil && [20]byte(ethcrypto.PubkeyToAddress(*pub)) == owner
}

func validateGenesisRequest(req *abci.RequestInitChain, c Config) error {
	if c.validate() != nil || validateBaseGenesisRequest(req, c) != nil {
		return ErrGenesis
	}
	g, err := decodeGenesis(req.AppStateBytes)
	if err != nil {
		return err
	}
	if c.BridgeGenesis == nil {
		if g.Bridge != nil {
			return ErrBridgeGenesis
		}
		return nil
	}
	if g.Bridge == nil || g.Bridge.Version != 1 {
		return ErrBridgeGenesis
	}
	p, _, err := godbridge.ReviewBootstrap(*c.BridgeGenesis, g.Bridge.Plan, g.Bridge.PlanApprovals)
	if err != nil {
		return ErrBridgeGenesis
	}
	digest, err := BridgeGenesisDigest(c, req)
	if err != nil || godbridge.VerifyAttestation(*c.BridgeGenesis, digest, g.Bridge.NodeApprovals) != nil || len(g.Bridge.Consents) != len(p.Validators) {
		return ErrBridgeGenesis
	}
	wanted := map[[20]byte][32]byte{}
	for _, v := range p.Validators {
		wanted[v.Owner] = v.ConsensusKey
	}
	for _, consent := range g.Bridge.Consents {
		key, found := wanted[consent.Owner]
		if !found {
			return ErrBridgeGenesis
		}
		ownerDigest, consensusDigest, err := BridgeValidatorConsentDigests(digest, consent.Owner, key)
		if err != nil || !validOwnerConsent(consent.Owner, ownerDigest, consent.OwnerSignature) ||
			len(consent.ConsensusSignature) != ed25519.SignatureSize || !ed25519.Verify(ed25519.PublicKey(key[:]), consensusDigest[:], consent.ConsensusSignature) {
			return ErrBridgeGenesis
		}
		delete(wanted, consent.Owner)
	}
	return nil
}
