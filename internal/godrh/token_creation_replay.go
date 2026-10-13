package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"math"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	evmstate "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	evmruntime "github.com/ethereum/go-ethereum/core/vm/runtime"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

const (
	MaxTokenCreationGas   = 10_000_000
	maxTokenCreationSteps = 200_000
	TokenReplayTimeout    = 3 * time.Second
)

// Explicit private replay inputs, not a source transaction. Nonce is only for
// CREATE; salt is only for CREATE2. No fee, key, signature, source state, caller
// supplied state database or live-node connection is accepted.
type TokenCreationReplay struct {
	Scheme      string
	Creator     [20]byte
	Nonce       uint64
	Salt        [32]byte
	BlockNumber uint64
	Time        uint64
	Gas         uint64
	Arguments   []byte
}

func (TokenCreationReplay) String() string {
	return "GOD Chain private creation replay inputs (redacted)"
}
func (r TokenCreationReplay) GoString() string { return r.String() }
func (TokenCreationReplay) MarshalJSON() ([]byte, error) {
	return []byte(`{"signingEnabled":false,"broadcastEnabled":false,"realAssetsReady":false}`), nil
}

type TokenCreationReplayReport struct {
	CreationArtifactMatched    bool `json:"creationArtifactMatched"`
	InitCodePinMatched         bool `json:"initCodePinMatched"`
	CreationAddressMatched     bool `json:"creationAddressMatched"`
	ConstructorExecuted        bool `json:"constructorExecuted"`
	ExactRuntimeReproduced     bool `json:"exactRuntimeReproduced"`
	CompilerExecutionVerified  bool `json:"compilerExecutionVerified"`
	CreationProvenanceVerified bool `json:"creationProvenanceVerified"`
	RHExecutionVerified        bool `json:"rhExecutionVerified"`
	TokenPermissionsReviewed   bool `json:"tokenPermissionsReviewed"`
	SourceFinalityVerified     bool `json:"sourceFinalityVerified"`
	SigningEnabled             bool `json:"signingEnabled"`
	BroadcastEnabled           bool `json:"broadcastEnabled"`
	RealAssetsReady            bool `json:"realAssetsReady"`
}

// ReproduceTokenCreation executes one exact compiled init code in a disposable
// in-memory Cancun VM. Full init bytes and CREATE/CREATE2 destination are bound
// before execution. No source or caller state can be read or changed. It checks
// conditional reproduction only, never authentic RH history or permissions.
// All errors, including context/tracer panics, return a static error and zero
// report. The VM/state are not returned or committed to any persistent store.
func ReproduceTokenCreation(ctx context.Context, a CompiledTokenCreation, request TokenCreationReplay, initPin [32]byte) (report TokenCreationReplayReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = TokenCreationReplayReport{}, ErrTokenCreation
		}
	}()
	// Copy caller-owned bytes before any user context method can run.
	if len(request.Arguments) > MaxTokenConstructorBytes {
		return TokenCreationReplayReport{}, ErrTokenCreation
	}
	request.Arguments = bytes.Clone(request.Arguments)
	if !a.valid || ctx == nil || ctx.Err() != nil || initPin == [32]byte{} || request.Creator == [20]byte{} ||
		request.Creator == a.token || request.BlockNumber == 0 || request.Time == 0 || request.Gas == 0 || request.Gas > MaxTokenCreationGas {
		return TokenCreationReplayReport{}, ErrTokenCreation
	}
	input, err := a.Input(request.Arguments)
	if err != nil || sha256.Sum256(input) != initPin {
		return TokenCreationReplayReport{}, ErrTokenCreation
	}
	creator := common.Address(request.Creator)
	var target common.Address
	switch request.Scheme {
	case "create":
		if request.Salt != [32]byte{} || request.Nonce == math.MaxUint64 {
			return TokenCreationReplayReport{}, ErrTokenCreation
		}
		target = ethcrypto.CreateAddress(creator, request.Nonce)
	case "create2":
		if request.Nonce != 0 {
			return TokenCreationReplayReport{}, ErrTokenCreation
		}
		target = ethcrypto.CreateAddress2(creator, request.Salt, ethcrypto.Keccak256(input))
	default:
		return TokenCreationReplayReport{}, ErrTokenCreation
	}
	if target != common.Address(a.token) {
		return TokenCreationReplayReport{}, ErrTokenCreation
	}
	id, err := chainID(a.chain)
	if err != nil {
		return TokenCreationReplayReport{}, ErrTokenCreation
	}
	ctx, cancel := context.WithTimeout(ctx, TokenReplayTimeout)
	defer cancel()
	database := evmstate.NewDatabaseForTesting()
	defer database.TrieDB().Close()
	state, err := evmstate.New(gethtypes.EmptyRootHash, database)
	if err != nil {
		return TokenCreationReplayReport{}, ErrTokenCreation
	}
	zero := uint64(0)
	chain := &params.ChainConfig{
		ChainID: id, HomesteadBlock: new(big.Int), EIP150Block: new(big.Int), EIP155Block: new(big.Int), EIP158Block: new(big.Int),
		ByzantiumBlock: new(big.Int), ConstantinopleBlock: new(big.Int), PetersburgBlock: new(big.Int), IstanbulBlock: new(big.Int),
		BerlinBlock: new(big.Int), LondonBlock: new(big.Int), TerminalTotalDifficulty: new(big.Int), ShanghaiTime: &zero, CancunTime: &zero,
	}
	steps := 0
	tracer := &tracing.Hooks{OnOpcode: func(_ uint64, op byte, _, _ uint64, _ tracing.OpContext, _ []byte, _ int, _ error) {
		steps++
		if steps > maxTokenCreationSteps || ctx.Err() != nil || !tokenCreationOpcodeAllowed(vm.OpCode(op)) {
			// Deliberate local abort BEFORE a forbidden opcode. Cancellation alone
			// need not prevent the next opcode from running in all VM paths.
			panic(ErrTokenCreation)
		}
	}}
	cfg := &evmruntime.Config{
		ChainConfig: chain, Difficulty: new(big.Int), Origin: creator, BlockNumber: new(big.Int).SetUint64(request.BlockNumber),
		Time: request.Time, GasLimit: request.Gas, GasPrice: new(big.Int), Value: new(big.Int), BaseFee: new(big.Int), BlobBaseFee: new(big.Int),
		Random: new(common.Hash), State: state, EVMConfig: vm.Config{Tracer: tracer}, GetHashFn: func(uint64) common.Hash { panic(ErrTokenCreation) },
	}
	env := evmruntime.NewEnv(cfg)
	rules := chain.Rules(cfg.BlockNumber, true, request.Time)
	state.Prepare(rules, creator, cfg.Coinbase, nil, vm.ActivePrecompiles(rules), nil)
	state.SetNonce(creator, request.Nonce, tracing.NonceChangeGenesis)
	var runtime []byte
	var created common.Address
	if request.Scheme == "create" {
		runtime, created, _, err = env.Create(creator, input, request.Gas, new(uint256.Int))
	} else {
		runtime, created, _, err = env.Create2(creator, input, request.Gas, new(uint256.Int), new(uint256.Int).SetBytes(request.Salt[:]))
	}
	if err != nil || ctx.Err() != nil || created != target || !tokenCreationRuntimeMatches(a, runtime) || !bytes.Equal(runtime, state.GetCode(target)) {
		return TokenCreationReplayReport{}, ErrTokenCreation
	}
	return TokenCreationReplayReport{CreationArtifactMatched: true, InitCodePinMatched: true, CreationAddressMatched: true,
		ConstructorExecuted: true, ExactRuntimeReproduced: true, CompilerExecutionVerified: true}, nil
}

// Cancun local constructors may use their own fresh storage and explicit
// caller/address/height/time/chain. External calls/state, nested creation,
// destruction, unprovided block/origin/fee data and gas-dependent behavior are
// unsupported. Deny executed instructions, not matching bytes in PUSH data.
func tokenCreationOpcodeAllowed(op vm.OpCode) bool {
	switch op {
	case vm.CALL, vm.CALLCODE, vm.DELEGATECALL, vm.STATICCALL, vm.CREATE, vm.CREATE2, vm.SELFDESTRUCT,
		vm.BALANCE, vm.EXTCODESIZE, vm.EXTCODECOPY, vm.EXTCODEHASH, vm.BLOCKHASH, vm.ORIGIN, vm.GASPRICE,
		vm.COINBASE, vm.PREVRANDAO, vm.GASLIMIT, vm.BASEFEE, vm.BLOBHASH, vm.BLOBBASEFEE, vm.SELFBALANCE, vm.GAS:
		return false
	}
	return true
}
