// Package godnode assembles the isolated, synthetic-asset node runtime. It is
// not mainnet activation or a claim of RH backing. The God SDK BaseApp and
// keepers remain private; consensus is accessed only through a local adapter.
package godnode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"sync"
	"time"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtx"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensuskeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmante "github.com/cosmos/evm/ante/evm"
	feekeeper "github.com/cosmos/evm/x/feemarket/keeper"
	feetypes "github.com/cosmos/evm/x/feemarket/types"
	evmmodule "github.com/cosmos/evm/x/vm"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

var (
	ErrConfig    = errors.New("node configuration rejected")
	ErrLifecycle = errors.New("node lifecycle rejected")
	ErrGenesis   = errors.New("synthetic node genesis rejected")
	ErrBlock     = errors.New("node block or verified commit rejected")
	ErrSupply    = errors.New("noninflating GOD settlement rejected")
)

const nodeStore = "god_node"
const authorityModule = "god_protocol_authority"
const settlementStore = "god_evm_settlement"
const ImplementationStage = "local-consensus-execution-prototype"

// No default production chain ID, gas price or per-block G emission is chosen.
// Prototype MUST be true until backed genesis, governance and penalties pass
// their independent launch gates. Per-block G is an explicit local policy.
type Config struct {
	Prototype       bool
	ChainID         string
	EVMChainID      uint64
	Policy          godtx.Policy
	MaxBlockGas     int64
	MaxBlockBytes   int64
	MaxBlockTxs     int
	GPerSignedBlock sdkmath.Int
}

func (c Config) validate() error {
	if !c.Prototype || len(c.ChainID) == 0 || len(c.ChainID) > 50 || c.EVMChainID == 0 || c.EVMChainID > math.MaxInt64 ||
		c.Policy.Validate() != nil || c.MaxBlockGas < int64(c.Policy.MaxGas) || c.MaxBlockBytes < int64(c.Policy.MaxTxBytes) ||
		c.MaxBlockBytes > 64<<20 || c.MaxBlockTxs <= 0 || c.MaxBlockTxs > 10000 || c.GPerSignedBlock.IsNil() ||
		!c.GPerSignedBlock.IsPositive() || c.GPerSignedBlock.GT(godrewards.DailyGCeiling()) {
		return ErrConfig
	}
	for _, ch := range c.ChainID {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' || ch == '.') {
			return ErrConfig
		}
	}
	return nil
}

type metadata struct {
	Binding []byte
	Height  int64
	Time    time.Time
}

type App struct {
	mu                                     sync.Mutex
	base                                   *baseapp.BaseApp
	encoding                               godtx.Encoding
	accounts                               authkeeper.AccountKeeper
	bank                                   bankkeeper.BaseKeeper
	staking                                *stakingkeeper.Keeper
	slashing                               slashingkeeper.Keeper
	fees                                   feekeeper.Keeper
	evm                                    *evmkeeper.Keeper
	evmBank                                evmBank
	rewards                                godrewards.Keeper
	key                                    *storetypes.KVStoreKey
	config                                 Config
	binding                                []byte
	clock, pendingTime                     time.Time
	initialized, finalized, failed, closed bool
	engineAssigned, engineRunning          bool
}

var processRuntime struct {
	sync.Mutex
	consumed bool
}

// New allows only one God EVM instance per process because its pinned upstream
// opcode/coin configuration is process-global and sealed. Separate validators
// run in separate processes; restart opens state in a new process, not by
// mutating globals. Reject a second instance before any upstream panic/race.
func New(db dbm.DB, c Config) (*App, error) {
	if db == nil || c.validate() != nil || sdk.GetConfig().GetBech32AccountAddrPrefix() != godaddress.AccountPrefix ||
		sdk.GetConfig().GetBech32ValidatorAddrPrefix() != godaddress.ValidatorOperatorPrefix || sdk.GetConfig().GetBech32ConsensusAddrPrefix() != godaddress.ConsensusPrefix {
		return nil, ErrConfig
	}
	processRuntime.Lock()
	defer processRuntime.Unlock()
	if processRuntime.consumed {
		return nil, ErrConfig
	}
	processRuntime.consumed = true
	c.Policy.MinFeePerGas = sdkmath.NewIntFromBigInt(c.Policy.MinFeePerGas.BigInt())
	c.GPerSignedBlock = sdkmath.NewIntFromBigInt(c.GPerSignedBlock.BigInt())
	binding, err := json.Marshal(struct {
		Version uint32
		Config  Config
	}{2, c})
	if err != nil {
		return nil, ErrConfig
	}
	enc, err := godtx.NewExecutionEncoding()
	if err != nil {
		return nil, err
	}
	logger := log.NewNopLogger()
	base := baseapp.NewBaseApp("GOD Chain local node", logger, db, enc.Decoder(c.Policy.MaxTxBytes), baseapp.SetChainID(c.ChainID))
	base.SetInterfaceRegistry(enc.Registry)
	base.SetTxEncoder(enc.TxConfig.TxEncoder())
	keys := storetypes.NewKVStoreKeys(authtypes.StoreKey, banktypes.StoreKey, consensuskeeper.StoreKey,
		stakingtypes.StoreKey, slashingtypes.StoreKey, godrewards.StoreKey, feetypes.StoreKey, evmtypes.StoreKey, nodeStore)
	transients := storetypes.NewTransientStoreKeys(feetypes.TransientKey, evmtypes.TransientKey, settlementStore)
	base.MountKVStores(keys)
	base.MountTransientStores(transients)
	permissions := map[string][]string{}
	for _, name := range moduleNames() {
		permissions[name] = nil
	}
	permissions[stakingtypes.BondedPoolName] = []string{authtypes.Staking}
	permissions[stakingtypes.NotBondedPoolName] = []string{authtypes.Staking}
	authority := authtypes.NewModuleAddress(authorityModule)
	ak := authkeeper.NewAccountKeeper(enc.Codec, runtime.NewKVStoreService(keys[authtypes.StoreKey]), authtypes.ProtoBaseAccount,
		permissions, godaddress.Codec{}, godaddress.AccountPrefix, authority.String())
	blocked := map[string]bool{}
	for _, name := range moduleNames() {
		blocked[ak.GetModuleAddress(name).String()] = true
	}
	bk := bankkeeper.NewBaseKeeper(enc.Codec, runtime.NewKVStoreService(keys[banktypes.StoreKey]), ak, blocked, authority.String(), logger)
	sk := stakingkeeper.NewKeeper(enc.Codec, runtime.NewKVStoreService(keys[stakingtypes.StoreKey]), ak, stakingBank{bk}, authority.String(),
		addresscodec.NewBech32Codec(godaddress.ValidatorOperatorPrefix), addresscodec.NewBech32Codec(godaddress.ConsensusPrefix))
	slk := slashingkeeper.NewKeeper(enc.Codec, codec.NewLegacyAmino(), runtime.NewKVStoreService(keys[slashingtypes.StoreKey]), sk, authority.String())
	sk.SetHooks(slk.Hooks())
	// One unit of consensus voting power is one GOD, not one six-decimal coin.
	sdk.DefaultPowerReduction = godrewards.Unit()
	rewards, err := godrewards.NewKeeper(keys[godrewards.StoreKey], bk, ak)
	if err != nil {
		return nil, err
	}
	cp := consensuskeeper.NewKeeper(enc.Codec, runtime.NewKVStoreService(keys[consensuskeeper.StoreKey]), authority.String(), runtime.EventService{})
	base.SetParamStore(cp.ParamsStore)
	fk := feekeeper.NewKeeper(enc.Codec, authority, keys[feetypes.StoreKey], transients[feetypes.TransientKey])
	eb := evmBank{bk, transients[settlementStore]}
	ek := evmkeeper.NewKeeper(enc.Codec, keys[evmtypes.StoreKey], transients[evmtypes.TransientKey], keys, authority, ak, eb, sk, fk, &cp,
		disabledERC20{}, c.EVMChainID, "").WithDefaultEvmCoinInfo(evmtypes.EvmCoinInfo{Denom: godrewards.GodDenom, ExtendedDenom: godrewards.GodDenom, DisplayDenom: "god", Decimals: 18})
	a := &App{base: base, encoding: enc, accounts: ak, bank: bk, staking: sk, slashing: slk, fees: fk, evm: ek, evmBank: eb, rewards: rewards,
		key: keys[nodeStore], config: c, binding: binding}
	native, err := godtx.NewAnteWithMessages(ak, bk, enc.TxConfig, rewards, c.Policy, validateNativeMessage)
	if err != nil {
		return nil, err
	}
	base.SetAnteHandler(func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		if !godtx.IsEthereum(tx) {
			return native(ctx, tx, simulate)
		}
		if simulate || ctx.BlockHeight() <= 0 || godtx.ValidateEthereumStructure(tx) != nil {
			return ctx, godtx.ErrPolicy
		}
		m := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
		if m.Raw.Gas() == 0 || m.Raw.Gas() > c.Policy.MaxGas || m.Raw.Nonce() == math.MaxUint64 ||
			m.Raw.Type() > 2 || len(ctx.TxBytes()) > c.Policy.MaxTxBytes {
			return ctx, godtx.ErrPolicy
		}
		params, feeParams := ek.GetParams(ctx), fk.GetParams(ctx)
		return sdk.ChainAnteDecorators(evmante.NewEVMMonoDecorator(ak, fk, ek, c.Policy.MaxGas, &params, &feeParams))(ctx, tx, false)
	})
	msg.RegisterMsgServer(base.MsgServiceRouter(), msg.NewServer(rewards))
	stakingtypes.RegisterMsgServer(base.MsgServiceRouter(), stakingServer{stakingkeeper.NewMsgServerImpl(sk), a})
	slashingtypes.RegisterMsgServer(base.MsgServiceRouter(), &penaltyServer{app: a})
	evmtypes.RegisterMsgServer(base.MsgServiceRouter(), ethereumServer{Keeper: ek, bank: eb})
	base.SetInitChainer(a.initChain)
	base.SetPreBlocker(a.preBlock)
	base.SetBeginBlocker(func(ctx sdk.Context) (sdk.BeginBlock, error) {
		if err := sk.BeginBlocker(ctx); err != nil {
			return sdk.BeginBlock{}, err
		}
		if err := fk.BeginBlock(ctx); err != nil {
			return sdk.BeginBlock{}, err
		}
		return sdk.BeginBlock{}, ek.BeginBlock(ctx)
	})
	base.SetEndBlocker(a.endBlock)
	if err := base.LoadLatestVersion(); err != nil {
		return nil, err
	}
	if base.LastBlockHeight() > 0 {
		ctx := a.base.NewUncachedContext(false, cmtproto.Header{ChainID: c.ChainID})
		stored, err := a.metadata(ctx)
		if err != nil || stored.Height != base.LastBlockHeight() {
			return nil, ErrConfig
		}
		// Genesis does not run again on process restart. Restore the sealed
		// execution globals from verified committed state, without rewriting
		// genesis, bank balances, staking or reward progress.
		coin := ek.GetEvmCoinInfo(ctx)
		if coin.Denom != godrewards.GodDenom || coin.ExtendedDenom != godrewards.GodDenom || coin.DisplayDenom != "god" || coin.Decimals != 18 ||
			!bk.GetSupply(ctx, godrewards.GodDenom).Amount.Equal(godrewards.FixedGodSupply()) {
			return nil, ErrConfig
		}
		evmmodule.SetGlobalConfigVariables(coin)
		a.clock, a.initialized = stored.Time, true
	}
	return a, nil
}

type disabledERC20 struct{}

func (disabledERC20) GetERC20PrecompileInstance(sdk.Context, common.Address) (vm.PrecompiledContract, bool, error) {
	return nil, false, nil
}

type ethereumServer struct {
	*evmkeeper.Keeper
	bank evmBank
}

func (s ethereumServer) EthereumTx(ctx context.Context, msg *evmtypes.MsgEthereumTx) (*evmtypes.MsgEthereumTxResponse, error) {
	res, err := s.Keeper.EthereumTx(ctx, msg)
	if err != nil {
		return nil, err
	}
	// The EVM has already charged opcode/state-access gas. Like the upstream
	// state commit, bank settlement must not add a second SDK storage gas bill.
	if err := s.bank.flush(sdk.UnwrapSDKContext(ctx).WithGasMeter(storetypes.NewInfiniteGasMeter())); err != nil {
		return nil, err
	}
	return res, nil
}

func moduleNames() []string {
	return []string{authtypes.FeeCollectorName, godrewards.PendingModule, godrewards.PoolModule, godrewards.ReserveModule,
		authorityModule, stakingtypes.BondedPoolName, stakingtypes.NotBondedPoolName, quarantineModule, evmtypes.ModuleName}
}
func evmModuleAddress() []byte { return authtypes.NewModuleAddress(evmtypes.ModuleName) }

func (a *App) metadata(ctx sdk.Context) (metadata, error) {
	var m metadata
	raw := ctx.KVStore(a.key).Get([]byte{1})
	if raw == nil || json.Unmarshal(raw, &m) != nil || !bytes.Equal(m.Binding, a.binding) || m.Height < 0 || !validTime(m.Time) {
		return m, ErrConfig
	}
	return m, nil
}
func (a *App) putMetadata(ctx sdk.Context, m metadata) error {
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	ctx.KVStore(a.key).Set([]byte{1}, raw)
	return nil
}
func validTime(t time.Time) bool { return t.Unix() > 0 && t.UTC().Year() <= 9999 }
func (a *App) usable() bool      { return !a.closed && !a.failed }
func (a *App) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.engineRunning {
		return ErrLifecycle
	}
	a.closed = true
	return a.base.Close()
}
