package godrh

import (
	"context"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// ReceiptSource supplies provider-attested receipts without any signing,
// transaction-submission, ledger or replay-acceptance API.
type ReceiptSource interface {
	Source
	Receipt(context.Context, [32]byte) (Receipt, error)
}

type receiptObservation struct {
	receipt    Receipt
	log        ReceiptLog
	checkpoint Block
}

// observeReceipt centralizes bounded receipt and code-pin checks. The selected
// log is detached before decoding; these checks are not independent finality.
func observeReceipt(ctx context.Context, c Config, source ReceiptSource, transaction [32]byte, index uint32, binding *relayReceiptBinding) (receiptObservation, error) {
	fail := func() (receiptObservation, error) { return receiptObservation{}, ErrSource }
	if ctx == nil || ctx.Err() != nil || source == nil || transaction == [32]byte{} ||
		!c.Report().ConfigurationReady || !Probe(ctx, c, source).ReadOnlyProbePassed {
		return fail()
	}
	expectedChain, _ := chainID(c.private.SourceChainID)
	chain, err := source.ChainID(ctx)
	if err != nil || chain == nil || chain.Cmp(expectedChain) != 0 {
		return fail()
	}
	checkpoint, err := source.FinalizedBlock(ctx)
	if err != nil || checkpoint.Height == 0 || checkpoint.Hash == [32]byte{} {
		return fail()
	}
	receipt, err := source.Receipt(ctx, transaction)
	if err != nil || !receipt.Success || receipt.TransactionHash != transaction || receipt.Block.Height == 0 ||
		receipt.Block.Height > checkpoint.Height || receipt.Block.Hash == [32]byte{} ||
		len(receipt.Logs) == 0 || len(receipt.Logs) > MaxReceiptLogs {
		return fail()
	}
	canonical, err := source.Block(ctx, receipt.Block.Height)
	if err != nil || canonical != receipt.Block {
		return fail()
	}
	token, _ := address(c.private.TokenContract)
	custody, _ := address(c.private.CustodyContract)
	for _, target := range []struct {
		address [20]byte
		pin     string
	}{
		{token, c.private.ExpectedTokenCodeHash}, {custody, c.private.ExpectedCustodyCodeHash},
	} {
		code, err := source.Code(ctx, target.address, receipt.Block.Hash)
		pin, _ := hash(target.pin)
		if err != nil || len(code) == 0 || len(code) > maxCodeBytes || ethcrypto.Keccak256Hash(code) != pin {
			return fail()
		}
	}
	var selected ReceiptLog
	found := false
	for i, log := range receipt.Logs {
		if log.Removed || log.Emitter == [20]byte{} || len(log.Topics) > 4 || len(log.Data) > MaxLogDataBytes ||
			i > 0 && log.Index <= receipt.Logs[i-1].Index {
			return fail()
		}
		if log.Index == index {
			selected = log
			selected.Topics = append([][32]byte(nil), log.Topics...)
			selected.Data = append([]byte(nil), log.Data...)
			found = true
		}
	}
	if !found || ctx.Err() != nil {
		return fail()
	}
	if binding != nil && (receipt.Block != binding.Block || receipt.TransactionIndex != binding.TransactionIndex ||
		receiptEventDigest(SourceLog{Block: receipt.Block, TransactionHash: receipt.TransactionHash,
			TransactionIndex: receipt.TransactionIndex, Log: selected}) != binding.EventDigest) {
		return fail()
	}
	// Retain only receipt identity, not provider-owned log slices.
	receipt.Logs = nil
	return receiptObservation{receipt: receipt, log: selected, checkpoint: checkpoint}, nil
}

// recheck runs after decoding and any additional view, permitting forward
// progress only while the original block identities and network still hold.
func (o receiptObservation) recheck(ctx context.Context, c Config, source ReceiptSource) error {
	canonical, err := source.Block(ctx, o.receipt.Block.Height)
	if err != nil || canonical != o.receipt.Block {
		return ErrSource
	}
	canonical, err = source.Block(ctx, o.checkpoint.Height)
	if err != nil || canonical != o.checkpoint {
		return ErrSource
	}
	expectedChain, _ := chainID(c.private.SourceChainID)
	chain, err := source.ChainID(ctx)
	if err != nil || chain == nil || chain.Cmp(expectedChain) != 0 || ctx.Err() != nil {
		return ErrSource
	}
	return nil
}

var _ ReceiptSource = (*HTTPSource)(nil)
