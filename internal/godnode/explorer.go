package godnode

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strconv"
	"strings"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtx"
	rewardmsg "github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	abci "github.com/cometbft/cometbft/abci/types"
	cmttypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// This is a bounded view of stored committed data, not an archive index or
// state proof. Never expose memos, arbitrary calldata, events, logs or VM errors
// through these explorer methods: faith text must not become decoded UI text.
func explorerInteger(raw json.RawMessage, min, max int64) (int64, bool) {
	s, ok := rpcString(raw)
	n, err := strconv.ParseInt(s, 10, 64)
	return n, ok && err == nil && n >= min && n <= max && strconv.FormatInt(n, 10) == s
}

func (r *TestnetRPC) explorer(ctx context.Context, call rpcRequest) (any, *rpcError) {
	bad := &rpcError{-32602, "invalid parameters"}
	failure := &rpcError{-32000, "synthetic explorer result unavailable"}
	if call.Method == "god_transactionDetails" {
		if len(call.Params) != 2 {
			return nil, bad
		}
		kind, ok := rpcString(call.Params[0])
		hash, valid := rpcString(call.Params[1])
		if !ok || !valid || kind != "native" && kind != "ethereum" || len(hash) != 66 || !strings.HasPrefix(hash, "0x") {
			return nil, bad
		}
		raw, err := hex.DecodeString(hash[2:])
		if err != nil {
			return nil, bad
		}
		hash = strings.ToLower(hash)
		var height int64
		var index uint32
		var wire []byte
		if kind == "native" {
			x, err := r.client.Tx(ctx, raw, false)
			if err != nil || x == nil {
				return nil, nil
			}
			height, index, wire = x.Height, x.Index, x.Tx
		} else {
			page, limit := 1, 2
			x, err := r.client.TxSearch(ctx, evmtypes.EventTypeEthereumTx+"."+evmtypes.AttributeKeyEthereumTxHash+"='"+hash+"'", false, &page, &limit, "asc")
			if err != nil || x == nil {
				return nil, failure
			}
			if x.TotalCount == 0 {
				return nil, nil
			}
			if x.TotalCount != 1 || len(x.Txs) != 1 || x.Txs[0] == nil {
				return nil, failure
			}
			height, index, wire = x.Txs[0].Height, x.Txs[0].Index, x.Txs[0].Tx
		}
		view, err := r.n.app.QueryNetwork(0)
		if err != nil {
			return nil, failure
		}
		if height > view.Commit.Height {
			return nil, nil
		}
		block, err := r.client.Block(ctx, &height)
		if err != nil || block == nil || block.Block == nil || uint64(index) >= uint64(len(block.Block.Txs)) || !bytes.Equal(wire, block.Block.Txs[index]) {
			return nil, failure
		}
		results, err := r.client.BlockResults(ctx, &height)
		if err != nil || results == nil || len(results.TxsResults) != len(block.Block.Txs) {
			return nil, failure
		}
		v, err := explorerSummary(r.n.app, wire, results.TxsResults[index], height, index)
		if err != nil || kind == "ethereum" && v["ethereumHash"] != hash || kind == "native" && v["consensusHash"] != hash {
			return nil, failure
		}
		v["blockHash"] = "0x" + hex.EncodeToString(block.Block.Hash())
		return v, nil
	}
	if len(call.Params) != 2 && call.Method == "god_blocks" || len(call.Params) != 3 && call.Method == "god_block" {
		return nil, bad
	}
	if len(call.Params) < 1 {
		return nil, bad
	}
	height, ok := rpcHeight(call.Params[0])
	if !ok {
		return nil, bad
	}
	view, err := r.n.app.QueryNetwork(0)
	if err != nil {
		return nil, failure
	}
	if height == 0 {
		height = view.Commit.Height
	}
	if height > view.Commit.Height {
		return nil, nil
	}
	if call.Method == "god_blocks" {
		count, ok := explorerInteger(call.Params[1], 1, 10)
		if !ok {
			return nil, bad
		}
		items := []any{}
		for i := int64(0); i < count && height-i > 0; i++ {
			h := height - i
			block, err := r.client.Block(ctx, &h)
			if err != nil || block == nil || block.Block == nil {
				return nil, failure
			}
			items = append(items, map[string]any{"height": strconv.FormatInt(h, 10), "time": block.Block.Time, "hash": "0x" + hex.EncodeToString(block.Block.Hash()), "transactions": len(block.Block.Txs)})
		}
		var next any
		if n := height - int64(len(items)); n > 0 {
			next = strconv.FormatInt(n, 10)
		}
		return map[string]any{"blocks": items, "nextHeight": next, "synthetic": true, "realAssets": false}, nil
	}
	offset, valid := explorerInteger(call.Params[1], 0, 1000000)
	limit, bounded := explorerInteger(call.Params[2], 1, 20)
	if !valid || !bounded {
		return nil, bad
	}
	block, err := r.client.Block(ctx, &height)
	if err != nil || block == nil || block.Block == nil {
		return nil, nil
	}
	results, err := r.client.BlockResults(ctx, &height)
	if err != nil || results == nil || len(results.TxsResults) != len(block.Block.Txs) {
		return nil, failure
	}
	if offset > int64(len(block.Block.Txs)) {
		return nil, bad
	}
	items := []any{}
	end := offset + limit
	if end > int64(len(block.Block.Txs)) {
		end = int64(len(block.Block.Txs))
	}
	for i := offset; i < end; i++ {
		v, err := explorerSummary(r.n.app, block.Block.Txs[i], results.TxsResults[i], height, uint32(i))
		if err != nil {
			return nil, failure
		}
		items = append(items, v)
	}
	var next any
	if end < int64(len(block.Block.Txs)) {
		next = strconv.FormatInt(end, 10)
	}
	return map[string]any{"height": strconv.FormatInt(height, 10), "hash": "0x" + hex.EncodeToString(block.Block.Hash()), "parentHash": "0x" + hex.EncodeToString(block.Block.LastBlockID.Hash), "time": block.Block.Time, "transactionCount": len(block.Block.Txs), "offset": strconv.FormatInt(offset, 10), "nextOffset": next, "transactions": items, "synthetic": true, "realAssets": false}, nil
}

func explorerSummary(a *App, wire []byte, result *abci.ExecTxResult, height int64, index uint32) (map[string]any, error) {
	if result == nil || height < 1 || result.GasUsed < 0 || result.GasWanted < 0 || len(wire) > a.config.Policy.MaxTxBytes {
		return nil, ErrBlock
	}
	tx, err := a.encoding.Decoder(a.config.Policy.MaxTxBytes)(wire)
	if err != nil {
		return nil, ErrBlock
	}
	v := map[string]any{"consensusHash": "0x" + hex.EncodeToString(cmttypes.Tx(wire).Hash()), "height": strconv.FormatInt(height, 10), "consensusIndex": strconv.FormatUint(uint64(index), 10), "code": result.Code, "sdkSuccessful": result.Code == 0, "gasWanted": strconv.FormatInt(result.GasWanted, 10), "gasUsed": strconv.FormatInt(result.GasUsed, 10), "synthetic": true, "realAssets": false}
	if godtx.IsEthereum(tx) {
		m := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
		eth := m.AsTransaction()
		sender, err := ethtypes.Sender(ethtypes.LatestSignerForChainID(new(big.Int).SetUint64(a.config.EVMChainID)), eth)
		if err != nil {
			return nil, ErrBlock
		}
		native, err := godaddress.ToNative(sender.Bytes())
		if err != nil {
			return nil, ErrBlock
		}
		v["kind"], v["ethereumHash"], v["sender"], v["senderEVM"] = "ethereum", eth.Hash().Hex(), native, sender.Hex()
		v["valueSmallestUnits"], v["inputBytes"], v["sequence"] = eth.Value().String(), len(eth.Data()), strconv.FormatUint(eth.Nonce(), 10)
		v["recipient"] = eth.To()
		v["evmExecution"] = "sdk-failed"
		if result.Code == 0 {
			response, err := evmtypes.DecodeTxResponse(result.Data)
			if err != nil || response == nil || response.Hash != eth.Hash().Hex() {
				return nil, ErrBlock
			}
			v["evmExecution"] = "succeeded"
			if response.VmError != "" {
				v["evmExecution"] = "failed"
			}
		}
		return v, nil
	}
	v["kind"] = "native"
	operations := []any{}
	for _, message := range tx.GetMsgs() {
		x := map[string]any{}
		switch m := message.(type) {
		case *stakingtypes.MsgDelegate:
			x["operation"], x["sender"], x["validator"], x["amountSmallestUnits"] = "delegate", m.DelegatorAddress, m.ValidatorAddress, m.Amount.Amount.String()
		case *stakingtypes.MsgUndelegate:
			x["operation"], x["sender"], x["validator"], x["amountSmallestUnits"] = "undelegate", m.DelegatorAddress, m.ValidatorAddress, m.Amount.Amount.String()
		case *rewardmsg.MsgClaimG:
			x["operation"], x["sender"] = "claim-g", m.Sender
		case *rewardmsg.MsgTransferG:
			x["operation"], x["sender"], x["recipient"], x["amountSmallestUnits"] = "transfer-g", m.Sender, m.Recipient, m.Amount
		case *rewardmsg.MsgDonateGod:
			x["operation"], x["sender"], x["amountSmallestUnits"] = "donate-god", m.Sender, m.Amount
		case *rewardmsg.MsgRedeemG:
			x["operation"], x["sender"], x["recipient"], x["amountSmallestUnits"], x["minGodOutSmallestUnits"], x["deadlineUnixNanos"] = "redeem-g", m.Sender, m.Beneficiary, m.Amount, m.MinGodOut, strconv.FormatInt(m.DeadlineUnixNanos, 10)
		default:
			// Preserve counts without exposing unknown message or text payloads.
			x["operation"] = "unsupported-summary"
		}
		operations = append(operations, x)
	}
	v["operations"] = operations
	if fee, ok := tx.(sdk.FeeTx); ok {
		v["fee"] = fee.GetFee().String()
	}
	return v, nil
}
