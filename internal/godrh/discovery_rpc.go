package godrh

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
)

// Logs filters ONLY the configured caller-selected custody and three bridge
// topics in one exact block hash. Never use an implicit latest/range cursor.
func (s *HTTPSource) Logs(ctx context.Context, custody [20]byte, block Block) ([]SourceLog, error) {
	if custody == [20]byte{} || !validDiscoveryBlock(block) {
		return nil, ErrSource
	}
	topics := bridgeEventTopics()
	filterTopics := make([]string, len(topics))
	for i, topic := range topics {
		filterTopics[i] = "0x" + hex.EncodeToString(topic[:])
	}
	raw, err := s.rpc(ctx, "eth_getLogs", []any{map[string]any{
		"address": "0x" + hex.EncodeToString(custody[:]), "blockHash": "0x" + hex.EncodeToString(block.Hash[:]),
		"topics": []any{filterTopics},
	}})
	if err != nil {
		return nil, ErrSource
	}
	return parseSourceLogs(raw, custody, block)
}

func parseSourceLogs(raw []byte, custody [20]byte, block Block) ([]SourceLog, error) {
	if len(raw) == 0 || len(raw) > maxRPCBytes || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, ErrSource
	}
	var entries []json.RawMessage
	if json.Unmarshal(raw, &entries) != nil || len(entries) > MaxDiscoveryLogs {
		return nil, ErrSource
	}
	logs := make([]SourceLog, 0, len(entries))
	for _, raw := range entries {
		fields, err := selectedObject(raw, "blockHash", "blockNumber", "transactionHash", "transactionIndex")
		if err != nil {
			return nil, ErrSource
		}
		var event SourceLog
		event.Block.Hash, err = rpcHash(fields["blockHash"])
		if err != nil {
			return nil, ErrSource
		}
		event.Block.Height, err = rpcUint(fields["blockNumber"], 64)
		if err != nil {
			return nil, ErrSource
		}
		event.TransactionHash, err = rpcHash(fields["transactionHash"])
		if err != nil {
			return nil, ErrSource
		}
		index, err := rpcUint(fields["transactionIndex"], 32)
		if err != nil {
			return nil, ErrSource
		}
		event.TransactionIndex = uint32(index)
		event.Log, err = parseReceiptLog(raw, Receipt{Block: event.Block, TransactionHash: event.TransactionHash, TransactionIndex: event.TransactionIndex})
		if err != nil {
			return nil, ErrSource
		}
		logs = append(logs, event)
	}
	if validateSourceLogs(logs, custody, block) != nil {
		return nil, ErrSource
	}
	return logs, nil
}

var _ DiscoverySource = (*HTTPSource)(nil)
