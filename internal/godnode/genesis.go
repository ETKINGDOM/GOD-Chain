package godnode

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"sort"
	"sync"
	"time"
	"unicode/utf8"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
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

func stakingGenesisParams() stakingtypes.Params {
	params := stakingtypes.DefaultParams()
	params.BondDenom, params.MaxValidators, params.UnbondingTime = godrewards.GodDenom, 32, 21*24*time.Hour
	params.MinCommissionRate = commission()
	return params
}

func executionGenesis(c Config) (*feetypes.GenesisState, *evmtypes.GenesisState) {
	fg := feetypes.DefaultGenesisState()
	fg.Params.BaseFee = sdkmath.LegacyNewDecFromInt(c.Policy.MinFeePerGas)
	fg.Params.MinGasPrice = sdkmath.LegacyNewDecFromInt(c.Policy.MinFeePerGas)
	vg := evmtypes.DefaultGenesisState()
	vg.Params.EvmDenom = godrewards.GodDenom
	vg.Params.ExtendedDenomOptions = &evmtypes.ExtendedDenomOptions{ExtendedDenom: godrewards.GodDenom}
	vg.Params.ActiveStaticPrecompiles, vg.Params.EVMChannels, vg.Preinstalls = nil, nil, nil
	return fg, vg
}

// Runtime-created synthetic fixtures only. No populated genesis, addresses,
// validator/private keys or endpoint configuration belongs in public source.
type Genesis struct {
	Version    uint32                    `json:"version"`
	Prototype  bool                      `json:"prototype"`
	Balances   []GenesisBalance          `json:"balances"`
	Validators []GenesisValidator        `json:"validators"`
	Bridge     *BridgeGenesisCertificate `json:"bridge,omitempty"`
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

// ValidateTestnetGenesis performs offline validation without consuming the
// process-global execution runtime or touching any database. Only unbacked
// synthetic genesis is admitted by the standalone testnet tools.
func ValidateTestnetGenesis(c Config, g *cmttypes.GenesisDoc) error {
	if c.validate() != nil || c.BridgeGenesis != nil || c.BridgeApprovalGas != 0 || g == nil ||
		g.ChainID != c.ChainID || g.ConsensusParams == nil || g.ValidateAndComplete() != nil {
		return ErrGenesis
	}
	cp := g.ConsensusParams.ToProto()
	r := &abci.RequestInitChain{ChainId: g.ChainID, Time: g.GenesisTime, InitialHeight: g.InitialHeight,
		AppStateBytes: g.AppState, ConsensusParams: &cp}
	for _, v := range g.Validators {
		if v.PubKey == nil || v.PubKey.Type() != "ed25519" || len(v.PubKey.Bytes()) != 32 {
			return ErrGenesis
		}
		r.Validators = append(r.Validators, abci.Ed25519ValidatorUpdate(v.PubKey.Bytes(), v.Power))
	}
	return validateGenesisRequest(r, c)
}

func strictJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var visit func(reflect.Type, int) error
	visit = func(kind reflect.Type, depth int) error {
		if depth > 16 {
			return ErrGenesis
		}
		for kind.Kind() == reflect.Pointer {
			kind = kind.Elem()
		}
		t, err := d.Token()
		if err != nil {
			return ErrGenesis
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				if kind.Kind() != reflect.Struct || reflect.PointerTo(kind).Implements(reflect.TypeFor[json.Unmarshaler]()) {
					return ErrGenesis
				}
				fields := map[string]reflect.Type{}
				for i := 0; i < kind.NumField(); i++ {
					field := kind.Field(i)
					if !field.IsExported() {
						continue
					}
					name := field.Name
					if tag := field.Tag.Get("json"); tag != "" {
						part := bytes.Split([]byte(tag), []byte(","))[0]
						if string(part) == "-" {
							continue
						}
						if len(part) != 0 {
							name = string(part)
						}
					}
					fields[name] = field.Type
				}
				seen := map[string]bool{}
				for d.More() {
					k, err := d.Token()
					text, ok := k.(string)
					field, allowed := fields[text]
					if err != nil || !ok || seen[text] || !allowed {
						return ErrGenesis
					}
					seen[text] = true
					if err := visit(field, depth+1); err != nil {
						return err
					}
				}
			case '[':
				if kind.Kind() != reflect.Array && kind.Kind() != reflect.Slice || kind.Kind() == reflect.Slice && kind.Elem().Kind() == reflect.Uint8 {
					return ErrGenesis
				}
				count := 0
				for d.More() {
					count++
					if kind.Kind() == reflect.Array && count > kind.Len() {
						return ErrGenesis
					}
					if err := visit(kind.Elem(), depth+1); err != nil {
						return err
					}
				}
				if kind.Kind() == reflect.Array && count != kind.Len() {
					return ErrGenesis
				}
			default:
				return ErrGenesis
			}
			close, err := d.Token()
			if err != nil || delim == '{' && close != json.Delim('}') || delim == '[' && close != json.Delim(']') {
				return ErrGenesis
			}
		}
		return nil
	}
	if err := visit(reflect.TypeFor[Genesis](), 0); err != nil {
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
		if a.Equals(authtypes.NewModuleAddress(godbridge.PendingModule)) {
			return g, ErrGenesis
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
	if err := validateGenesisRequest(req, a.config); err != nil {
		return nil, err
	}
	g, err := decodeGenesis(req.AppStateBytes)
	if err != nil {
		return nil, err
	}
	var certificate *[32]byte
	if a.config.BridgeGenesis != nil {
		digest, err := BridgeGenesisDigest(a.config, req)
		if err != nil {
			return nil, err
		}
		certificate = &digest
	}
	if ctx.KVStore(a.key).Has([]byte{1}) {
		return nil, ErrLifecycle
	}
	// Bank allocation, bridge releases/replay records, staking, execution and
	// metadata share one cache. Never leave a partially initialized genesis.
	cache, write := ctx.CacheContext()
	ctx = cache
	a.accounts.InitGenesis(ctx, *authtypes.DefaultGenesisState())
	names := nodeModuleNames(a.config)
	sort.Strings(names)
	for _, name := range names {
		a.accounts.GetModuleAccount(ctx, name)
	}
	bg := banktypes.DefaultGenesisState()
	bg.Supply = sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, godrewards.FixedGodSupply()))
	total := sdkmath.ZeroInt()
	if a.config.BridgeGenesis == nil {
		for _, b := range g.Balances {
			address, _ := msg.Account(b.Address)
			n, _ := godAmount(b.Amount)
			a.accounts.SetAccount(ctx, a.accounts.NewAccountWithAddress(ctx, address))
			bg.Balances = append(bg.Balances, banktypes.Balance{Address: b.Address, Coins: sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, n))})
			total = total.Add(n)
		}
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
	if a.config.BridgeGenesis != nil {
		if a.bridge == nil {
			return nil, ErrBridgeGenesis
		}
		// Balances above are reconciliation declarations, NOT a second bank
		// allocation. InitBootstrap releases from the full restricted reserve
		// once and creates fresh accounts before self-staking is applied.
		if err := a.bridge.InitBootstrap(ctx, g.Bridge.Plan, g.Bridge.PlanApprovals); err != nil {
			return nil, err
		}
	}
	sg := stakingtypes.DefaultGenesisState()
	sg.Params = stakingGenesisParams()
	a.staking.InitGenesis(ctx, sg)
	pg := slashingtypes.DefaultGenesisState()
	pg.Params = penaltyParams()
	if err := pg.Params.Validate(); err != nil {
		return nil, err
	}
	a.slashing.InitGenesis(ctx, a.staking, pg)
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
	fg, vg := executionGenesis(a.config)
	feemarket.InitGenesis(ctx, a.fees, *fg)
	evm.InitGenesis(ctx, a.evm, a.accounts, a.evmBank, *vg, new(sync.Once))
	if err := a.putMetadata(ctx, metadata{Binding: a.binding, Height: 0, Time: req.Time.UTC(), BridgeGenesis: certificate}); err != nil {
		return nil, err
	}
	if err := a.initializeSigningSets(ctx, updates); err != nil {
		return nil, err
	}
	if _, err := a.bridgeSnapshot(ctx); err != nil {
		return nil, err
	}
	write()
	return &abci.ResponseInitChain{Validators: updates}, nil
}

func validateBaseGenesisRequest(req *abci.RequestInitChain, c Config) error {
	if req == nil || req.ChainId != c.ChainID || !validTime(req.Time) || (req.InitialHeight != 0 && req.InitialHeight != 1) || req.ConsensusParams == nil {
		return ErrGenesis
	}
	// The upstream conversion dereferences these fields before ValidateBasic.
	if req.ConsensusParams.Block == nil || req.ConsensusParams.Evidence == nil || req.ConsensusParams.Validator == nil || req.ConsensusParams.Version == nil {
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
