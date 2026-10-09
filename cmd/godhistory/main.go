// godhistory is an opt-in local keyless index tool. It exposes no listener,
// runs no background job and never activates a chain or a public route.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/ETKINGDOM/GOD-Chain/internal/godhistory"
)

func uintValue(s string) (uint64, error) {
	n, e := strconv.ParseUint(s, 10, 64)
	if e != nil || strconv.FormatUint(n, 10) != s {
		return 0, godhistory.ErrHistory
	}
	return n, nil
}
func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) < 1 || args[0] != "sync" && args[0] != "status" && args[0] != "page" && args[0] != "audit" {
		return godhistory.ErrHistory
	}
	f := flag.NewFlagSet("godhistory", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	home := f.String("home", "", "")
	chain := f.String("chain-id", "", "")
	compatible := f.String("compatible-chain-id", "", "")
	pin := f.String("bundle-sha256", "", "")
	first := f.String("first-height", "", "")
	blocks := f.String("max-blocks", "", "")
	transactions := f.String("max-transactions", "", "")
	var upstream, limit, address, snapshotHeight, snapshotHash, beforeHeight, beforeIndex *string
	if args[0] == "sync" {
		upstream = f.String("upstream", "", "")
		limit = f.String("limit", "", "")
	}
	if args[0] == "page" {
		address = f.String("address", "", "")
		snapshotHeight = f.String("snapshot-height", "", "")
		snapshotHash = f.String("snapshot-hash", "", "")
		beforeHeight = f.String("before-height", "", "")
		beforeIndex = f.String("before-index", "", "")
	}
	seen := map[string]bool{}
	for i := 1; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "--") {
			return godhistory.ErrHistory
		}
		name, _, inline := strings.Cut(strings.TrimPrefix(args[i], "--"), "=")
		if seen[name] || f.Lookup(name) == nil {
			return godhistory.ErrHistory
		}
		seen[name] = true
		if !inline {
			i++
			if i >= len(args) || strings.HasPrefix(args[i], "--") {
				return godhistory.ErrHistory
			}
		}
	}
	if f.Parse(args[1:]) != nil || f.NArg() != 0 {
		return godhistory.ErrHistory
	}
	h, e := uintValue(*first)
	b, failure := uintValue(*blocks)
	t, te := uintValue(*transactions)
	if e != nil || failure != nil || te != nil || h > 9223372036854775807 {
		return godhistory.ErrHistory
	}
	c := godhistory.Config{ChainID: *chain, CompatibleChainID: *compatible, BundleSHA256: *pin, FirstHeight: int64(h), MaxBlocks: b, MaxTransactions: t}
	var rpc *godhistory.RPC
	var batch uint64
	if args[0] == "sync" {
		batch, e = uintValue(*limit)
		if e != nil || batch < 1 || batch > godhistory.BatchLimit {
			return godhistory.ErrHistory
		}
		rpc, e = godhistory.NewRPC(*upstream)
		if e != nil {
			return e
		}
		defer rpc.Close()
	}
	var cursor *godhistory.Cursor
	if args[0] == "page" && (*snapshotHeight != "" || *snapshotHash != "" || *beforeHeight != "" || *beforeIndex != "") {
		cursor = &godhistory.Cursor{SnapshotHeight: *snapshotHeight, SnapshotHash: *snapshotHash, BeforeHeight: *beforeHeight, BeforeIndex: *beforeIndex}
	}
	x, e := godhistory.Open(*home, c, args[0] != "sync")
	if e != nil {
		return e
	}
	var result any
	switch args[0] {
	case "sync":
		result, e = x.Sync(ctx, rpc, int(batch))
	case "page":
		result, e = x.Page(*address, cursor)
	case "audit":
		result, e = x.Audit(ctx)
	case "status":
		var s godhistory.State
		s, e = x.Status()
		if e == nil {
			result = map[string]any{"version": 1, "synthetic": true, "realAssets": false, "firstHeight": strconv.FormatInt(s.Config.FirstHeight, 10), "indexedThrough": strconv.FormatInt(s.Height, 10), "blocks": strconv.FormatUint(s.Blocks, 10), "transactions": strconv.FormatUint(s.Transactions, 10), "unsupportedTransactions": strconv.FormatUint(s.Unsupported, 10), "verified": s.Verified, "checkedAt": s.CheckedAt, "publicRoute": false}
		}
	}
	closed := x.Close()
	if e != nil || closed != nil {
		return godhistory.ErrHistory
	}
	if json.NewEncoder(out).Encode(result) != nil {
		return godhistory.ErrHistory
	}
	return nil
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if run(ctx, os.Args[1:], os.Stdout) != nil {
		fmt.Fprintln(os.Stderr, "synthetic history index unavailable; retained progress requires review")
		os.Exit(1)
	}
}
