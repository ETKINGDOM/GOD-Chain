// Package godapp assembles a local native God SDK application prototype.
// It exposes no server, validator selection, EVM execution or bridge, and cannot
// claim backing. The underlying BaseApp is private to prevent lifecycle bypass.
package godapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"sync"
	"time"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtx"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	abci "github.com/cometbft/cometbft/abci/types"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/runtime"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensuskeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
)

var (
	ErrConfiguration = errors.New("invalid or mismatched native application configuration")
	ErrGenesis       = errors.New("invalid local-prototype genesis")
	ErrLifecycle     = errors.New("native application lifecycle rejected")
	ErrBlock         = errors.New("invalid native prototype block")
)

const metadataStore = "god_app"
const authorityModule = "god_protocol_authority"
const ImplementationStage = "native-application-prototype"

var metadataKey = []byte{1}
var chainIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,50}$`)

// All execution limits are explicit. This configuration selects neither a
// production gas price nor G issuance. Its bytes are bound to committed state.
type Config struct {
	ChainID       string
	Policy        godtx.Policy
	MaxBlockGas   int64
	MaxBlockBytes int64
	MaxBlockTxs   int
}

func (c Config) validate() error {
	if !chainIDPattern.MatchString(c.ChainID) || c.Policy.Validate() != nil || c.MaxBlockGas <= 0 ||
		uint64(c.MaxBlockGas) < c.Policy.MaxGas || c.MaxBlockBytes < int64(c.Policy.MaxTxBytes) || c.MaxBlockBytes > 64<<20 ||
		c.MaxBlockTxs <= 0 || c.MaxBlockTxs > 10_000 {
		return ErrConfiguration
	}
	return nil
}

type metadata struct {
	Binding []byte    `json:"binding"`
	Height  int64     `json:"height"`
	Time    time.Time `json:"time"`
}

type App struct {
	mu          sync.Mutex
	base        *baseapp.BaseApp
	encoding    godtx.Encoding
	accounts    authkeeper.AccountKeeper
	bank        bankkeeper.BaseKeeper
	rewards     godrewards.Keeper
	key         *storetypes.KVStoreKey
	config      Config
	binding     []byte
	clock       time.Time
	pendingTime time.Time
	initialized bool
	finalized   bool
	failed      bool
	closed      bool
}

// New requires an already configured GOD address prefix and a caller-owned
// database. On success Close owns closing that database. Logging is disabled;
// the prototype never emits private text, addresses, keys or signed payloads.
func New(db dbm.DB, config Config) (*App, error) {
	if db == nil || config.validate() != nil || sdk.GetConfig().GetBech32AccountAddrPrefix() != godaddress.AccountPrefix ||
		sdk.GetConfig().GetBech32ValidatorAddrPrefix() != godaddress.ValidatorOperatorPrefix ||
		sdk.GetConfig().GetBech32ConsensusAddrPrefix() != godaddress.ConsensusPrefix {
		return nil, ErrConfiguration
	}
	config.Policy.MinFeePerGas = sdkmath.NewIntFromBigInt(config.Policy.MinFeePerGas.BigInt())
	binding, err := json.Marshal(struct {
		Version uint32
		Config  Config
	}{Version: 1, Config: config})
	if err != nil {
		return nil, ErrConfiguration
	}
	encoding, err := godtx.NewEncoding()
	if err != nil {
		return nil, err
	}
	logger := log.NewNopLogger()
	base := baseapp.NewBaseApp("GOD Chain native prototype", logger, db, encoding.Decoder(config.Policy.MaxTxBytes), baseapp.SetChainID(config.ChainID))
	base.SetInterfaceRegistry(encoding.Registry)
	base.SetTxEncoder(encoding.TxConfig.TxEncoder())
	authKey := storetypes.NewKVStoreKey(authtypes.StoreKey)
	bankKey := storetypes.NewKVStoreKey(banktypes.StoreKey)
	rewardKey := storetypes.NewKVStoreKey(godrewards.StoreKey)
	consensusKey := storetypes.NewKVStoreKey(consensuskeeper.StoreKey)
	key := storetypes.NewKVStoreKey(metadataStore)
	base.MountKVStores(map[string]*storetypes.KVStoreKey{
		authKey.Name(): authKey, bankKey.Name(): bankKey, rewardKey.Name(): rewardKey, consensusKey.Name(): consensusKey, key.Name(): key,
	})
	permissions := map[string][]string{}
	for _, name := range moduleNames() {
		permissions[name] = nil // No module may mint or burn GOD.
	}
	authority, err := (godaddress.Codec{}).BytesToString(authtypes.NewModuleAddress(authorityModule))
	if err != nil {
		return nil, err
	}
	accounts := authkeeper.NewAccountKeeper(encoding.Codec, runtime.NewKVStoreService(authKey), authtypes.ProtoBaseAccount,
		permissions, godaddress.Codec{}, godaddress.AccountPrefix, authority)
	blocked := map[string]bool{}
	for _, name := range moduleNames() {
		blocked[accounts.GetModuleAddress(name).String()] = true
	}
	bank := bankkeeper.NewBaseKeeper(encoding.Codec, runtime.NewKVStoreService(bankKey), accounts, blocked, authority, logger)
	rewards, err := godrewards.NewKeeper(rewardKey, bank, accounts)
	if err != nil {
		return nil, err
	}
	consensus := consensuskeeper.NewKeeper(encoding.Codec, runtime.NewKVStoreService(consensusKey), authority, runtime.EventService{})
	base.SetParamStore(consensus.ParamsStore)
	ante, err := godtx.NewAnte(accounts, bank, encoding.TxConfig, rewards, config.Policy)
	if err != nil {
		return nil, err
	}
	base.SetAnteHandler(ante)
	msg.RegisterMsgServer(base.MsgServiceRouter(), msg.NewServer(rewards))
	a := &App{base: base, encoding: encoding, accounts: accounts, bank: bank, rewards: rewards, key: key, config: config, binding: binding}
	base.SetInitChainer(func(ctx sdk.Context, req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
		balances, err := decodeGenesis(req.AppStateBytes)
		if err != nil {
			return nil, err
		}
		if err := a.initialize(ctx, balances); err != nil {
			return nil, err
		}
		return &abci.ResponseInitChain{}, nil
	})
	base.SetPreBlocker(a.preBlock)
	base.SetEndBlocker(a.endBlock)
	if err := base.LoadLatestVersion(); err != nil {
		return nil, err
	}
	if base.LastBlockHeight() > 0 {
		ctx := a.readContext()
		stored, err := a.getMetadata(ctx)
		if err != nil || stored.Height != base.LastBlockHeight() {
			return nil, ErrConfiguration
		}
		a.clock, a.initialized = stored.Time, true
		if _, err := rewards.Snapshot(a.readContext()); err != nil {
			return nil, err
		}
	}
	return a, nil
}

func (a *App) getMetadata(ctx sdk.Context) (metadata, error) {
	var stored metadata
	raw := ctx.KVStore(a.key).Get(metadataKey)
	if raw == nil || json.Unmarshal(raw, &stored) != nil || !bytes.Equal(stored.Binding, a.binding) ||
		stored.Height < 0 || !validTime(stored.Time) {
		return metadata{}, ErrConfiguration
	}
	return stored, nil
}

func (a *App) putMetadata(ctx sdk.Context, stored metadata) error {
	raw, err := json.Marshal(stored)
	if err != nil {
		return ErrConfiguration
	}
	ctx.KVStore(a.key).Set(metadataKey, raw)
	return nil
}

func (a *App) readContext() sdk.Context {
	return a.base.NewUncachedContext(false, cmtproto.Header{ChainID: a.config.ChainID, Height: a.base.LastBlockHeight(), Time: a.clock})
}

func validTime(value time.Time) bool {
	year := value.UTC().Year()
	return value.Unix() > 0 && year >= 1970 && year <= 9999
}

func (a *App) preBlock(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*sdk.ResponsePreBlock, error) {
	stored, err := a.getMetadata(ctx)
	if err != nil {
		return nil, err
	}
	if stored.Height == math.MaxInt64 || req.Height != stored.Height+1 || req.Time.Before(stored.Time) || !validTime(req.Time) {
		return nil, ErrBlock
	}
	cache, write := ctx.CacheContext()
	if err := a.rewards.BeginDay(cache); err != nil {
		return nil, err
	}
	if err := a.putMetadata(cache, metadata{Binding: a.binding, Height: req.Height, Time: req.Time.UTC()}); err != nil {
		return nil, err
	}
	write()
	return &sdk.ResponsePreBlock{}, nil
}

func (a *App) endBlock(ctx sdk.Context) (sdk.EndBlock, error) {
	// Native ante already routes accepted fees. This collects any residual fees
	// without staking distribution, fee burning, inflation or G issuance.
	return sdk.EndBlock{}, a.rewards.CollectGasFees(ctx)
}

func (a *App) usable() bool { return !a.closed && !a.failed }

// InitChain accepts only bounded local-prototype state. Invalid requests are
// rejected before SDK initialization; an internal failure poisons this instance.
func (a *App) InitChain(req *abci.RequestInitChain) (*abci.ResponseInitChain, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable() || a.initialized {
		return nil, ErrLifecycle
	}
	if req == nil || req.ChainId != a.config.ChainID || !validTime(req.Time) ||
		(req.InitialHeight != 0 && req.InitialHeight != 1) || len(req.Validators) != 0 || req.ConsensusParams == nil {
		return nil, ErrGenesis
	}
	params := cmttypes.ConsensusParamsFromProto(*req.ConsensusParams)
	if params.ValidateBasic() != nil || params.Block.MaxGas != a.config.MaxBlockGas || params.Block.MaxBytes != a.config.MaxBlockBytes {
		return nil, ErrGenesis
	}
	if _, err := decodeGenesis(req.AppStateBytes); err != nil {
		return nil, err
	}
	// Poison first: a caller recovering an internal SDK panic must not be able
	// to commit or continue a partially initialized/executed instance.
	a.failed = true
	response, err := a.base.InitChain(req)
	if err != nil {
		return nil, err
	}
	a.failed = false
	a.initialized, a.clock = true, req.Time.UTC()
	return response, nil
}

func (a *App) FinalizeBlock(req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable() || !a.initialized || a.finalized {
		return nil, ErrLifecycle
	}
	lastHeight := a.base.LastBlockHeight()
	if req == nil || lastHeight == math.MaxInt64 || req.Height != lastHeight+1 || !validTime(req.Time) || req.Time.Before(a.clock) || len(req.Txs) > a.config.MaxBlockTxs {
		return nil, ErrBlock
	}
	// Bound work before the SDK decodes any wire. Envelope errors still produce
	// normal transaction rejection; an oversized block is rejected as a whole.
	total := int64(0)
	for _, wire := range req.Txs {
		if len(wire) == 0 || len(wire) > a.config.Policy.MaxTxBytes || int64(len(wire)) > a.config.MaxBlockBytes-total {
			return nil, ErrBlock
		}
		total += int64(len(wire))
	}
	a.failed = true
	response, err := a.base.FinalizeBlock(req)
	if err != nil {
		return nil, err
	}
	a.failed = false
	a.finalized, a.pendingTime = true, req.Time.UTC()
	return response, nil
}

func (a *App) Commit() (*abci.ResponseCommit, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable() || !a.initialized || !a.finalized {
		return nil, ErrLifecycle
	}
	a.failed = true
	response, err := a.base.Commit()
	if err != nil {
		return nil, err
	}
	a.failed = false
	a.finalized, a.clock = false, a.pendingTime
	return response, nil
}

// Snapshot reads committed reward state, not an uncommitted finalized block.
func (a *App) Snapshot() (godrewards.Snapshot, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.usable() || a.base.LastBlockHeight() == 0 {
		return godrewards.Snapshot{}, ErrLifecycle
	}
	// FinalizeBlock calculates a working hash by writing its cache into the root
	// working store before Commit. A raw context would leak uncommitted state.
	ctx, err := a.base.CreateQueryContext(a.base.LastBlockHeight(), false)
	if err != nil {
		return godrewards.Snapshot{}, err
	}
	return a.rewards.Snapshot(ctx.WithBlockTime(a.clock))
}

func (a *App) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return ErrLifecycle
	}
	a.closed = true
	return a.base.Close()
}
