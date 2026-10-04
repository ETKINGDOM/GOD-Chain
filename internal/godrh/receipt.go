package godrh

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"
)

// These are local admission bounds, not approved mainnet workload parameters.
const (
	MaxReceiptLogs  = 128
	MaxLogDataBytes = 4096
)

// ReceiptLog inherits the enclosing receipt's block and transaction identity.
// HTTP decoding verifies those identities on every log before normalization.
// Operational contents must remain private; ordinary formatting is redacted.
type ReceiptLog struct {
	Emitter [20]byte
	Index   uint32
	Topics  [][32]byte
	Data    []byte
	Removed bool
}

func (ReceiptLog) String() string               { return "RH receipt log (redacted)" }
func (l ReceiptLog) GoString() string           { return l.String() }
func (ReceiptLog) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type Receipt struct {
	Block            Block
	TransactionHash  [32]byte
	TransactionIndex uint32
	Success          bool
	Logs             []ReceiptLog
}

func (Receipt) String() string               { return "RH source receipt (redacted)" }
func (r Receipt) GoString() string           { return r.String() }
func (Receipt) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

// selectedObject permits other RPC metadata but rejects duplicate keys, aliases
// of selected fields, null/missing selected values and trailing JSON. Selecting
// fields this way avoids last-key-wins or case-insensitive struct decoding.
func selectedObject(raw []byte, selected ...string) (map[string]json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > maxRPCBytes || !utf8.Valid(raw) {
		return nil, ErrSource
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return nil, ErrSource
	}
	seen := map[string]bool{}
	fields := map[string]json.RawMessage{}
	for d.More() {
		key, err := d.Token()
		name, ok := key.(string)
		if err != nil || !ok || seen[name] {
			return nil, ErrSource
		}
		seen[name] = true
		var value json.RawMessage
		if d.Decode(&value) != nil {
			return nil, ErrSource
		}
		for _, field := range selected {
			if strings.EqualFold(name, field) {
				if name != field || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
					return nil, ErrSource
				}
				fields[field] = value
			}
		}
	}
	last, err := d.Token()
	if err != nil || last != json.Delim('}') {
		return nil, ErrSource
	}
	if _, err := d.Token(); err != io.EOF || len(fields) != len(selected) {
		return nil, ErrSource
	}
	return fields, nil
}

func rpcText(raw json.RawMessage) (string, error) {
	var text string
	if json.Unmarshal(raw, &text) != nil {
		return "", ErrSource
	}
	return text, nil
}

func rpcUint(raw json.RawMessage, bits int) (uint64, error) {
	text, err := rpcText(raw)
	n, parseErr := quantity(text)
	if err != nil || parseErr != nil || n.BitLen() > bits {
		return 0, ErrSource
	}
	return n.Uint64(), nil
}

func rpcHash(raw json.RawMessage) ([32]byte, error) {
	text, err := rpcText(raw)
	if err != nil {
		return [32]byte{}, ErrSource
	}
	value, err := hash(text)
	if err != nil {
		return [32]byte{}, ErrSource
	}
	return value, nil
}

func rpcData(raw json.RawMessage, maxBytes int) ([]byte, error) {
	text, err := rpcText(raw)
	if err != nil || len(text) < 2 || len(text) > 2+maxBytes*2 || !strings.HasPrefix(text, "0x") {
		return nil, ErrSource
	}
	value, err := hex.DecodeString(text[2:])
	if err != nil {
		return nil, ErrSource
	}
	return value, nil
}

func parseReceipt(raw []byte, transaction [32]byte) (Receipt, error) {
	f, err := selectedObject(raw, "blockHash", "blockNumber", "transactionHash", "transactionIndex", "status", "logs")
	if err != nil || transaction == [32]byte{} {
		return Receipt{}, ErrSource
	}
	var receipt Receipt
	receipt.Block.Hash, err = rpcHash(f["blockHash"])
	if err != nil {
		return Receipt{}, ErrSource
	}
	receipt.Block.Height, err = rpcUint(f["blockNumber"], 64)
	if err != nil || receipt.Block.Height == 0 {
		return Receipt{}, ErrSource
	}
	receipt.TransactionHash, err = rpcHash(f["transactionHash"])
	if err != nil || receipt.TransactionHash != transaction {
		return Receipt{}, ErrSource
	}
	index, err := rpcUint(f["transactionIndex"], 32)
	if err != nil {
		return Receipt{}, ErrSource
	}
	receipt.TransactionIndex = uint32(index)
	status, err := rpcUint(f["status"], 1)
	if err != nil {
		return Receipt{}, ErrSource
	}
	receipt.Success = status == 1
	var logs []json.RawMessage
	if json.Unmarshal(f["logs"], &logs) != nil || len(logs) > MaxReceiptLogs {
		return Receipt{}, ErrSource
	}
	for _, rawLog := range logs {
		log, err := parseReceiptLog(rawLog, receipt)
		if err != nil || len(receipt.Logs) > 0 && log.Index <= receipt.Logs[len(receipt.Logs)-1].Index {
			return Receipt{}, ErrSource
		}
		receipt.Logs = append(receipt.Logs, log)
	}
	return receipt, nil
}

func parseReceiptLog(raw []byte, receipt Receipt) (ReceiptLog, error) {
	f, err := selectedObject(raw, "blockHash", "blockNumber", "transactionHash", "transactionIndex", "address", "logIndex", "topics", "data", "removed")
	if err != nil {
		return ReceiptLog{}, ErrSource
	}
	blockHash, err := rpcHash(f["blockHash"])
	if err != nil || blockHash != receipt.Block.Hash {
		return ReceiptLog{}, ErrSource
	}
	height, err := rpcUint(f["blockNumber"], 64)
	if err != nil || height != receipt.Block.Height {
		return ReceiptLog{}, ErrSource
	}
	transaction, err := rpcHash(f["transactionHash"])
	if err != nil || transaction != receipt.TransactionHash {
		return ReceiptLog{}, ErrSource
	}
	index, err := rpcUint(f["transactionIndex"], 32)
	if err != nil || uint32(index) != receipt.TransactionIndex {
		return ReceiptLog{}, ErrSource
	}
	var log ReceiptLog
	text, err := rpcText(f["address"])
	if err != nil {
		return ReceiptLog{}, ErrSource
	}
	log.Emitter, err = address(text)
	if err != nil {
		return ReceiptLog{}, ErrSource
	}
	index, err = rpcUint(f["logIndex"], 32)
	if err != nil {
		return ReceiptLog{}, ErrSource
	}
	log.Index = uint32(index)
	if json.Unmarshal(f["removed"], &log.Removed) != nil || log.Removed {
		return ReceiptLog{}, ErrSource
	}
	log.Data, err = rpcData(f["data"], MaxLogDataBytes)
	if err != nil {
		return ReceiptLog{}, ErrSource
	}
	var topics []json.RawMessage
	if json.Unmarshal(f["topics"], &topics) != nil || len(topics) > 4 {
		return ReceiptLog{}, ErrSource
	}
	for _, topic := range topics {
		data, err := rpcData(topic, 32)
		if err != nil || len(data) != 32 {
			return ReceiptLog{}, ErrSource
		}
		var value [32]byte
		copy(value[:], data)
		log.Topics = append(log.Topics, value)
	}
	return log, nil
}

// Receipt is a bounded, read-only provider response, not a receipt inclusion
// proof or independently verified RH finality. A null/pending result fails.
func (s *HTTPSource) Receipt(ctx context.Context, transaction [32]byte) (Receipt, error) {
	if transaction == [32]byte{} {
		return Receipt{}, ErrSource
	}
	raw, err := s.rpc(ctx, "eth_getTransactionReceipt", []any{"0x" + hex.EncodeToString(transaction[:])})
	if err != nil {
		return Receipt{}, ErrSource
	}
	return parseReceipt(raw, transaction)
}
