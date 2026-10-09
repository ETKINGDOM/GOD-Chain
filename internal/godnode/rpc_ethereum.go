package godnode

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"math"
	"math/big"
	"strconv"
	"strings"

	storetypes "cosmossdk.io/store/types"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtx"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

func ethHeight(raw json.RawMessage) (int64, bool) {
	s, ok := rpcString(raw)
	if !ok {
		return 0, false
	}
	if s == "latest" {
		return 0, true
	}
	h, err := hexutil.DecodeUint64(s)
	return int64(h), err == nil && h > 0 && h <= uint64(^uint64(0)>>1)
}

func (r *TestnetRPC) ethereum(ctx context.Context, call rpcRequest) (any, *rpcError) {
	bad := &rpcError{-32602, "invalid parameters"}
	unavailable := &rpcError{-32000, "synthetic node result unavailable"}
	a := r.n.app
	switch call.Method {
	case "web3_clientVersion", "eth_chainId", "net_version", "eth_blockNumber", "eth_syncing", "eth_accounts":
		if len(call.Params) != 0 {
			return nil, bad
		}
		switch call.Method {
		case "web3_clientVersion":
			return "GOD Chain/synthetic-testnet", nil
		case "eth_chainId":
			return hexutil.EncodeUint64(a.config.EVMChainID), nil
		case "net_version":
			return strconv.FormatUint(a.config.EVMChainID, 10), nil
		case "eth_accounts":
			return []string{}, nil // No server-held account signer.
		case "eth_syncing":
			status, err := r.client.Status(ctx)
			if err != nil {
				return nil, unavailable
			}
			if status.SyncInfo.CatchingUp {
				// The engine's status does not expose a verified remote tip.
				// Do not invent a highestBlock or pretend to be synchronized.
				return nil, &rpcError{-32004, "node synchronizing; remote tip unavailable"}
			}
			return false, nil
		default:
			view, err := a.QueryNetwork(0)
			if err != nil {
				return nil, unavailable
			}
			return hexutil.EncodeUint64(uint64(view.Commit.Height)), nil
		}
	case "eth_sendRawTransaction":
		if len(call.Params) != 1 {
			return nil, bad
		}
		text, ok := rpcString(call.Params[0])
		if !ok || len(text) > 2+a.config.Policy.MaxTxBytes*2 {
			return nil, bad
		}
		raw, err := hexutil.Decode(text)
		if err != nil || len(raw) == 0 {
			return nil, bad
		}
		var tx ethtypes.Transaction
		if tx.UnmarshalBinary(raw) != nil || tx.Type() > 2 || tx.ChainId().Uint64() != a.config.EVMChainID || tx.ChainId().BitLen() > 64 || tx.Gas() > a.config.Policy.MaxGas || tx.Gas() == 0 {
			return nil, bad
		}
		m := &evmtypes.MsgEthereumTx{}
		if m.FromSignedEthereumTx(&tx, ethtypes.LatestSignerForChainID(new(big.Int).SetUint64(a.config.EVMChainID))) != nil {
			return nil, bad
		}
		builder := a.encoding.TxConfig.NewTxBuilder()
		signed, err := m.BuildTx(builder, godrewards.GodDenom)
		if err != nil {
			return nil, bad
		}
		wire, err := a.encoding.TxConfig.TxEncoder()(signed)
		if err != nil || len(wire) > a.config.Policy.MaxTxBytes {
			return nil, bad
		}
		err = r.n.Submit(ctx, wire)
		if err != nil {
			return nil, submissionFailure(err)
		}
		return tx.Hash().Hex(), nil
	case "eth_getTransactionReceipt":
		if len(call.Params) != 1 {
			return nil, bad
		}
		hash, ok := rpcString(call.Params[0])
		if !ok || len(hash) != 66 || !strings.HasPrefix(hash, "0x") {
			return nil, bad
		}
		if _, err := hex.DecodeString(hash[2:]); err != nil {
			return nil, bad
		}
		return r.ethereumReceipt(ctx, strings.ToLower(hash))
	case "eth_getBlockByNumber":
		if len(call.Params) != 2 {
			return nil, bad
		}
		height, ok := ethHeight(call.Params[0])
		var full bool
		if !ok || json.Unmarshal(call.Params[1], &full) != nil || full {
			return nil, bad
		}
		view, err := a.QueryNetwork(0)
		if err != nil {
			return nil, unavailable
		}
		if height == 0 {
			height = view.Commit.Height
		}
		if height > view.Commit.Height {
			return nil, nil
		}
		block, err := r.client.Block(ctx, &height)
		if err != nil || block == nil || block.Block == nil {
			return nil, nil
		}
		results, err := r.client.BlockResults(ctx, &height)
		if err != nil || len(results.TxsResults) != len(block.Block.Txs) {
			return nil, unavailable
		}
		hashes, used, _, _, e := ethereumLane(a, block.Block, results.TxsResults, -1)
		if e != nil {
			return nil, unavailable
		}
		var sdkUsed uint64
		for i := range block.Block.Txs {
			gas := results.TxsResults[i].GasUsed
			if gas < 0 || uint64(gas) > math.MaxUint64-sdkUsed {
				return nil, unavailable
			}
			sdkUsed += uint64(gas)
		}
		return map[string]any{"number": hexutil.EncodeUint64(uint64(height)), "hash": "0x" + hex.EncodeToString(block.Block.Hash()), "parentHash": common.BytesToHash(block.Block.LastBlockID.Hash).Hex(), "timestamp": hexutil.EncodeUint64(uint64(block.Block.Time.Unix())), "gasLimit": hexutil.EncodeUint64(uint64(a.config.MaxBlockGas)), "gasUsed": hexutil.EncodeUint64(used), "godSdkGasUsed": hexutil.EncodeUint64(sdkUsed), "transactions": hashes, "synthetic": true, "realAssets": false}, nil
	}

	// All state/call methods bind one committed application version and its
	// actual consensus header. Simulation writes remain in a discarded cache.
	switch call.Method {
	case "eth_gasPrice", "eth_maxPriorityFeePerGas", "eth_getBalance", "eth_getTransactionCount", "eth_getCode", "eth_getStorageAt", "eth_call", "eth_estimateGas":
	default:
		return nil, &rpcError{-32601, "method not supported"}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	q, commit, err := a.committedContext(0)
	if err != nil {
		return nil, unavailable
	}
	block := r.n.node.BlockStore().LoadBlock(commit.Height)
	if block == nil {
		return nil, unavailable
	}
	q = q.WithBlockHeader(*block.Header.ToProto()).WithHeaderHash(block.Hash()).WithConsensusParams(a.base.GetConsensusParams(q))
	baseFee := a.config.Policy.MinFeePerGas.BigInt()
	fee := a.fees.GetBaseFee(q)
	if !fee.IsNil() && fee.IsPositive() && fee.Ceil().TruncateInt().BigInt().Cmp(baseFee) > 0 {
		baseFee = fee.Ceil().TruncateInt().BigInt()
	}
	switch call.Method {
	case "eth_gasPrice", "eth_maxPriorityFeePerGas":
		if len(call.Params) != 0 {
			return nil, bad
		}
		if call.Method == "eth_maxPriorityFeePerGas" {
			return "0x0", nil
		}
		return hexutil.EncodeBig(baseFee), nil
	case "eth_getBalance", "eth_getTransactionCount", "eth_getCode", "eth_getStorageAt":
		count := 2
		if call.Method == "eth_getStorageAt" {
			count = 3
		}
		if len(call.Params) != count {
			return nil, bad
		}
		text, ok := rpcString(call.Params[0])
		if !ok {
			return nil, bad
		}
		raw, e := godaddress.FromEVM(text)
		if e != nil {
			return nil, bad
		}
		address := common.BytesToAddress(raw)
		tag, ok := rpcString(call.Params[count-1])
		if !ok {
			return nil, bad
		}
		if call.Method == "eth_getTransactionCount" && tag == "pending" {
			if a.finalized {
				return nil, &rpcError{-32001, "retry after application commit"}
			}
			account := a.accounts.GetAccount(a.base.GetContextForCheckTx(nil), sdk.AccAddress(raw))
			if account == nil {
				return "0x0", nil
			}
			return hexutil.EncodeUint64(account.GetSequence()), nil
		}
		height, ok := ethHeight(call.Params[count-1])
		if !ok || height != 0 && height != commit.Height && call.Method != "eth_getCode" {
			return nil, bad
		}
		if call.Method == "eth_getCode" && height != 0 {
			code, err := a.retainedCode(address, height)
			if err != nil {
				return nil, unavailable
			}
			return hexutil.Encode(code), nil
		}
		switch call.Method {
		case "eth_getBalance":
			return hexutil.EncodeBig(a.bank.GetBalance(q, sdk.AccAddress(raw), godrewards.GodDenom).Amount.BigInt()), nil
		case "eth_getTransactionCount":
			account := a.accounts.GetAccount(q, sdk.AccAddress(raw))
			if account == nil {
				return "0x0", nil
			}
			return hexutil.EncodeUint64(account.GetSequence()), nil
		case "eth_getCode":
			return hexutil.Encode(a.evm.GetCode(q, a.evm.GetCodeHash(q, address))), nil
		default:
			text, ok := rpcString(call.Params[1])
			if !ok {
				return nil, bad
			}
			index, e := hexutil.DecodeBig(text)
			if e != nil || index.BitLen() > 256 {
				return nil, bad
			}
			return a.evm.GetState(q, address, common.BigToHash(index)).Hex(), nil
		}
	case "eth_call", "eth_estimateGas":
		if len(call.Params) < 1 || len(call.Params) > 2 {
			return nil, bad
		}
		if len(call.Params) == 2 {
			height, ok := ethHeight(call.Params[1])
			if !ok || height != 0 && height != commit.Height {
				return nil, bad
			}
		}
		if _, err := boundedCallArgs(call.Params[0], a.config); err != nil {
			return nil, bad
		}
		cached, _ := q.CacheContext()
		cached = cached.WithContext(ctx).WithGasMeter(storetypes.NewInfiniteGasMeter())
		request := &evmtypes.EthCallRequest{Args: call.Params[0], GasCap: a.config.Policy.MaxGas}
		if call.Method == "eth_estimateGas" {
			response, e := a.evm.EstimateGas(sdk.WrapSDKContext(cached), request)
			if e != nil || response == nil {
				return nil, &rpcError{-32003, "bounded execution rejected"}
			}
			if response.VmError != "" {
				return nil, &rpcError{3, "execution reverted"}
			}
			return hexutil.EncodeUint64(response.Gas), nil
		}
		response, e := a.evm.EthCall(sdk.WrapSDKContext(cached), request)
		if e != nil || response == nil {
			return nil, &rpcError{-32003, "bounded execution rejected"}
		}
		if response.VmError != "" {
			return nil, &rpcError{3, "execution reverted"}
		}
		return hexutil.Encode(response.Ret), nil
	default:
		return nil, &rpcError{-32601, "method not supported"}
	}
}

// Ethereum-compatible methods expose the ordered Ethereum transaction lane.
// Native SDK transactions retain their separate consensus index and never
// silently shift receipt/log indexes relative to eth_getBlockByNumber.
func ethereumLane(a *App, block *cmttypes.Block, results []*abci.ExecTxResult, target int) ([]string, uint64, uint64, uint, error) {
	if block == nil || len(results) != len(block.Txs) || target < -1 || target >= len(block.Txs) {
		return nil, 0, 0, 0, ErrBlock
	}
	hashes := []string{}
	var cumulative, index uint64
	var logs, offset uint
	found := target == -1
	for i, wire := range block.Txs {
		decoded, err := a.encoding.TxConfig.TxDecoder()(wire)
		if err != nil || godtx.ValidateEthereumStructure(decoded) != nil {
			continue
		}
		hash := decoded.GetMsgs()[0].(*evmtypes.MsgEthereumTx).AsTransaction().Hash().Hex()
		hashes = append(hashes, hash)
		if results[i] == nil || results[i].GasUsed < 0 {
			return nil, 0, 0, 0, ErrBlock
		}
		gas := uint64(results[i].GasUsed)
		logCount := 0
		if results[i].Code == 0 {
			response, err := evmtypes.DecodeTxResponse(results[i].Data)
			if err != nil || response == nil || response.Hash != hash {
				return nil, 0, 0, 0, ErrBlock
			}
			gas, logCount = response.GasUsed, len(response.Logs)
		}
		if target == -1 || i <= target {
			if gas > math.MaxUint64-cumulative {
				return nil, 0, 0, 0, ErrBlock
			}
			cumulative += gas
		}
		if i == target {
			index, offset, found = uint64(len(hashes)-1), logs, true
		}
		logs += uint(logCount)
	}
	if !found {
		return nil, 0, 0, 0, ErrBlock
	}
	return hashes, cumulative, index, offset, nil
}

func boundedCallArgs(raw json.RawMessage, c Config) (evmtypes.TransactionArgs, error) {
	var args evmtypes.TransactionArgs
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return args, ErrConfig
	}
	for name := range fields {
		switch name {
		case "from", "to", "gas", "gasPrice", "maxFeePerGas", "maxPriorityFeePerGas", "value", "nonce", "data", "input", "accessList", "chainId":
		default:
			return args, ErrConfig
		}
	}
	if json.Unmarshal(raw, &args) != nil || args.From == nil || len(args.GetData()) > c.Policy.MaxTxBytes ||
		args.Gas != nil && (uint64(*args.Gas) == 0 || uint64(*args.Gas) > c.Policy.MaxGas) ||
		args.Value != nil && args.Value.ToInt().BitLen() > 256 ||
		args.ChainID != nil && args.ChainID.ToInt().Cmp(new(big.Int).SetUint64(c.EVMChainID)) != 0 ||
		args.GasPrice != nil && (args.MaxFeePerGas != nil || args.MaxPriorityFeePerGas != nil) ||
		args.Data != nil && args.Input != nil && !strings.EqualFold(hexutil.Encode(*args.Data), hexutil.Encode(*args.Input)) {
		return args, ErrConfig
	}
	for _, price := range []*hexutil.Big{args.GasPrice, args.MaxFeePerGas, args.MaxPriorityFeePerGas} {
		if price != nil && (price.ToInt().Sign() < 0 || price.ToInt().BitLen() > 256) {
			return args, ErrConfig
		}
	}
	if list, ok := fields["accessList"]; ok && string(list) != "null" {
		var entries []map[string]json.RawMessage
		if json.Unmarshal(list, &entries) != nil || len(entries) > 128 {
			return args, ErrConfig
		}
		total := 0
		for _, entry := range entries {
			if len(entry) != 2 || entry["address"] == nil || entry["storageKeys"] == nil {
				return args, ErrConfig
			}
			var keys []common.Hash
			if json.Unmarshal(entry["storageKeys"], &keys) != nil {
				return args, ErrConfig
			}
			total += len(keys)
			if total > 512 {
				return args, ErrConfig
			}
		}
	}
	return args, nil
}

func (r *TestnetRPC) ethereumReceipt(ctx context.Context, hash string) (any, *rpcError) {
	failure := &rpcError{-32000, "synthetic transaction result unavailable"}
	page, limit := 1, 2
	search, err := r.client.TxSearch(ctx, evmtypes.EventTypeEthereumTx+"."+evmtypes.AttributeKeyEthereumTxHash+"='"+hash+"'", false, &page, &limit, "asc")
	if err != nil {
		return nil, failure
	}
	if search.TotalCount == 0 {
		return nil, nil
	}
	if search.TotalCount != 1 || len(search.Txs) != 1 {
		return nil, failure
	}
	x := search.Txs[0]
	view, err := r.n.app.QueryNetwork(0)
	if err != nil || x.Height > view.Commit.Height {
		return nil, nil
	}
	decoded, err := r.n.app.encoding.TxConfig.TxDecoder()(x.Tx)
	if err != nil || godtx.ValidateEthereumStructure(decoded) != nil {
		return nil, failure
	}
	m := decoded.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
	tx := m.AsTransaction()
	if tx.Hash().Hex() != hash {
		return nil, failure
	}
	block, err := r.client.Block(ctx, &x.Height)
	if err != nil || block == nil || int(x.Index) >= len(block.Block.Txs) || !strings.EqualFold(hex.EncodeToString(cmttypes.Tx(x.Tx).Hash()), hex.EncodeToString(cmttypes.Tx(block.Block.Txs[x.Index]).Hash())) {
		return nil, failure
	}
	response, err := evmtypes.DecodeTxResponse(x.TxResult.Data)
	if err != nil || response == nil || response.Hash != hash {
		return nil, failure
	}
	status := uint64(0)
	if x.TxResult.Code == 0 && response.VmError == "" {
		status = 1
	}
	sender, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(new(big.Int).SetUint64(r.n.app.config.EVMChainID)), tx)
	if err != nil {
		return nil, failure
	}
	var contract any
	if tx.To() == nil && status == 1 {
		contract = crypto.CreateAddress(sender, tx.Nonce()).Hex()
	}
	results, err := r.client.BlockResults(ctx, &x.Height)
	if err != nil || len(results.TxsResults) != len(block.Block.Txs) {
		return nil, failure
	}
	_, cumulative, index, logOffset, err := ethereumLane(r.n.app, block.Block, results.TxsResults, int(x.Index))
	if err != nil {
		return nil, failure
	}
	logs := evmtypes.LogsToEthereum(response.Logs)
	for i, log := range logs {
		log.BlockNumber = uint64(x.Height)
		log.BlockHash = common.BytesToHash(block.Block.Hash())
		log.TxHash = tx.Hash()
		log.TxIndex = uint(index)
		log.Index = logOffset + uint(i)
	}
	// Comet block identifiers are the compatible chain's block identifiers,
	// not Ethereum RLP header hashes. Never fabricate Ethereum state roots.
	return map[string]any{"transactionHash": hash, "transactionIndex": hexutil.EncodeUint64(index), "godConsensusIndex": hexutil.EncodeUint64(uint64(x.Index)), "blockHash": common.BytesToHash(block.Block.Hash()).Hex(), "blockNumber": hexutil.EncodeUint64(uint64(x.Height)),
		"from": sender.Hex(), "to": tx.To(), "contractAddress": contract, "status": hexutil.EncodeUint64(status), "gasUsed": hexutil.EncodeUint64(response.GasUsed), "cumulativeGasUsed": hexutil.EncodeUint64(cumulative), "type": hexutil.EncodeUint64(uint64(tx.Type())), "logs": logs, "logsBloom": ethtypes.CreateBloom(&ethtypes.Receipt{Logs: logs}), "synthetic": true, "realAssets": false}, nil
}
