package godnode

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"sync"
	"time"
	"unicode/utf8"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	feemarket "github.com/cosmos/evm/x/feemarket"
	feetypes "github.com/cosmos/evm/x/feemarket/types"
	evm "github.com/cosmos/evm/x/vm"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func stakingkeeperServer(a *App) stakingtypes.MsgServer {
	return stakingkeeper.NewMsgServerImpl(a.staking)
}

// Runtime-created synthetic fixtures only. No populated genesis, addresses,
// validator/private keys or endpoint configuration belongs in public source.
type Genesis struct {
	Version    uint32             `json:"version"`
	Prototype  bool               `json:"prototype"`
	Balances   []GenesisBalance   `json:"balances"`
	Validators []GenesisValidator `json:"validators"`
}
type GenesisBalance struct {
	Address string `json:"address"`
	Amount  string `json:"godSmallestUnits"`
}
type GenesisValidator struct {
	Owner     string `json:"owner"`
	PublicKey []byte `json:"consensusPublicKey"`
	Stake     string `json:"stakeSmallestUnits"`
}

func strictJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var visit func(int) error
	visit = func(depth int) error {
		if depth > 8 {
			return ErrGenesis
		}
		t, err := d.Token()
		if err != nil {
			return ErrGenesis
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					k, err := d.Token()
					text, ok := k.(string)
					allowed := depth == 0 && (text == "version" || text == "prototype" || text == "balances" || text == "validators") || depth == 2 &&
						(text == "address" || text == "godSmallestUnits" || text == "owner" || text == "consensusPublicKey" || text == "stakeSmallestUnits")
					if err != nil || !ok || seen[text] || !allowed {
						return ErrGenesis
					}
					seen[text] = true
					if err := visit(depth + 1); err != nil {
						return err
					}
				}
			case '[':
				for d.More() {
					if err := visit(depth + 1); err != nil {
						return err
					}
				}
			default:
				return ErrGenesis
			}
			if _, err := d.Token(); err != nil {
				return ErrGenesis
			}
		}
		return nil
	}
	if err := visit(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrGenesis
	}
	return nil
}

func godAmount(text string) (sdkmath.Int, error) {
	if len(text) == 0 || len(text) > 28 || text[0] == '0' {
		return sdkmath.Int{}, ErrGenesis
	}
	for _, ch := range text {
		if ch < '0' || ch > '9' {
			return sdkmath.Int{}, ErrGenesis
		}
	}
	n, ok := sdkmath.NewIntFromString(text)
	if !ok || !n.IsPositive() || n.GT(godrewards.FixedGodSupply()) {
		return sdkmath.Int{}, ErrGenesis
	}
	return n, nil
}

func decodeGenesis(raw []byte) (Genesis, error) {
	var g Genesis
	if len(raw) == 0 || len(raw) > 1<<20 || !utf8.Valid(raw) || strictJSON(raw) != nil {
		return g, ErrGenesis
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&g) != nil || g.Version != 1 || !g.Prototype || len(g.Balances) > 4096 || len(g.Validators) == 0 || len(g.Validators) > 32 {
		return g, ErrGenesis
	}
	balances := map[string]sdkmath.Int{}
	total := sdkmath.ZeroInt()
	for _, b := range g.Balances {
		a, err := msg.Account(b.Address)
		if err != nil {
			return g, ErrGenesis
		}
		if _, ok := balances[b.Address]; ok {
			return g, ErrGenesis
		}
		for _, name := range moduleNames() {
			if a.Equals(authtypes.NewModuleAddress(name)) {
				return g, ErrGenesis
			}
		}
		n, err := godAmount(b.Amount)
		if err != nil {
			return g, err
		}
		total = total.Add(n)
		if total.GT(godrewards.FixedGodSupply()) {
			return g, ErrGenesis
		}
		balances[b.Address] = n
	}
	seenKeys, seenOwners := map[string]bool{}, map[string]bool{}
	for _, v := range g.Validators {
		if _, err := msg.Account(v.Owner); err != nil {
			return g, ErrGenesis
		}
		n, err := godAmount(v.Stake)
		balance, found := balances[v.Owner]
		if err != nil || n.LT(minSelfStake()) || !found || n.GT(balance) || len(v.PublicKey) != 32 || seenKeys[string(v.PublicKey)] || seenOwners[v.Owner] {
			return g, ErrGenesis
		}
		seenKeys[string(v.PublicKey)], seenOwners[v.Owner] = true, true
	}
	sort.Slice(g.Balances, func(i, j int) bool { return g.Balances[i].Address < g.Balances[j].Address })
	sort.Slice(g.Validators, func(i, j int) bool { return g.Validators[i].Owner < g.Validators[j].Owner })
	return g, nil
}

func (a *App) initChain(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	g, err := decodeGenesis(req.AppStateBytes)
	if err != nil {
		return nil, err
	}
	a.accounts.InitGenesis(ctx, *authtypes.DefaultGenesisState())
	names := moduleNames()
	sort.Strings(names)
	for _, name := range names {
		a.accounts.GetModuleAccount(ctx, name)
	}
	bg := banktypes.DefaultGenesisState()
	bg.Supply = sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, godrewards.FixedGodSupply()))
	total := sdkmath.ZeroInt()
	for _, b := range g.Balances {
		address, _ := msg.Account(b.Address)
		n, _ := godAmount(b.Amount)
		a.accounts.SetAccount(ctx, a.accounts.NewAccountWithAddress(ctx, address))
		bg.Balances = append(bg.Balances, banktypes.Balance{Address: b.Address, Coins: sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, n))})
		total = total.Add(n)
	}
	if remainder := godrewards.FixedGodSupply().Sub(total); remainder.IsPositive() {
		bg.Balances = append(bg.Balances, banktypes.Balance{Address: a.accounts.GetModuleAddress(godrewards.ReserveModule).String(), Coins: sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, remainder))})
	}
	a.bank.InitGenesis(ctx, bg)
	a.bank.SetDenomMetaData(ctx, banktypes.Metadata{Base: godrewards.GodDenom, Display: "god", Name: "GOD", Symbol: "GOD",
		DenomUnits: []*banktypes.DenomUnit{{Denom: godrewards.GodDenom, Exponent: 0}, {Denom: "god", Exponent: 18}}})
	if err := a.rewards.Init(ctx); err != nil {
		return nil, err
	}
	sg := stakingtypes.DefaultGenesisState()
	sg.Params.BondDenom, sg.Params.MaxValidators, sg.Params.UnbondingTime = godrewards.GodDenom, 32, 21*24*time.Hour
	sg.Params.MinCommissionRate = commission()
	a.staking.InitGenesis(ctx, sg)
	server := stakingkeeperServer(a)
	for _, v := range g.Validators {
		owner, _ := msg.Account(v.Owner)
		n, _ := godAmount(v.Stake)
		pub, err := codectypes.NewAnyWithValue(&ed25519.PubKey{Key: v.PublicKey})
		if err != nil {
			return nil, err
		}
		m := &stakingtypes.MsgCreateValidator{ValidatorAddress: sdk.ValAddress(owner).String(), Pubkey: pub,
			Description: stakingtypes.Description{Moniker: "God validator"},
			Value:       sdk.NewCoin(godrewards.GodDenom, n), MinSelfDelegation: minSelfStake(),
			Commission: stakingtypes.NewCommissionRates(commission(), commission(), sdkmath.LegacyZeroDec())}
		if err := validateNativeMessage(m); err != nil {
			return nil, err
		}
		if _, err := server.CreateValidator(ctx, m); err != nil {
			return nil, err
		}
	}
	updates, err := a.staking.ApplyAndReturnValidatorSetUpdates(ctx)
	if err != nil {
		return nil, err
	}
	fg := feetypes.DefaultGenesisState()
	fg.Params.BaseFee = sdkmath.LegacyNewDecFromInt(a.config.Policy.MinFeePerGas)
	fg.Params.MinGasPrice = sdkmath.LegacyNewDecFromInt(a.config.Policy.MinFeePerGas)
	feemarket.InitGenesis(ctx, a.fees, *fg)
	vg := evmtypes.DefaultGenesisState()
	vg.Params.EvmDenom = godrewards.GodDenom
	vg.Params.ExtendedDenomOptions = &evmtypes.ExtendedDenomOptions{ExtendedDenom: godrewards.GodDenom}
	vg.Params.ActiveStaticPrecompiles, vg.Params.EVMChannels, vg.Preinstalls = nil, nil, nil
	evm.InitGenesis(ctx, a.evm, a.accounts, a.evmBank, *vg, new(sync.Once))
	if err := a.putMetadata(ctx, metadata{Binding: a.binding, Height: 0, Time: req.Time.UTC()}); err != nil {
		return nil, err
	}
	if err := a.initializeSigningSets(ctx, updates); err != nil {
		return nil, err
	}
	return &abci.ResponseInitChain{Validators: updates}, nil
}

func validateGenesisRequest(req *abci.RequestInitChain, c Config) error {
	if req == nil || req.ChainId != c.ChainID || !validTime(req.Time) || (req.InitialHeight != 0 && req.InitialHeight != 1) || req.ConsensusParams == nil {
		return ErrGenesis
	}
	cp := cmttypes.ConsensusParamsFromProto(*req.ConsensusParams)
	if cp.ValidateBasic() != nil || cp.Block.MaxGas != c.MaxBlockGas || cp.Block.MaxBytes != c.MaxBlockBytes ||
		cp.ABCI.VoteExtensionsEnableHeight != 0 || len(cp.Validator.PubKeyTypes) != 1 || cp.Validator.PubKeyTypes[0] != "ed25519" {
		return ErrGenesis
	}
	g, err := decodeGenesis(req.AppStateBytes)
	if err != nil {
		return err
	}
	if len(req.Validators) != 0 {
		if len(req.Validators) != len(g.Validators) {
			return ErrGenesis
		}
		wanted := map[string]int64{}
		for _, v := range g.Validators {
			n, _ := godAmount(v.Stake)
			wanted[string(v.PublicKey)] = n.Quo(godrewards.Unit()).Int64()
		}
		for _, v := range req.Validators {
			pk := v.PubKey.GetEd25519()
			power, ok := wanted[string(pk)]
			if !ok || v.Power != power {
				return ErrGenesis
			}
			delete(wanted, string(pk))
		}
	}
	return nil
}
