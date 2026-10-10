package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godtestnet"
)

func runReadLoad(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("read-load", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	values := map[string]string{"max-block-age-seconds": "60", "minimum-peers": "0"}
	seen := map[string]bool{}
	for _, name := range []string{"bundle", "expected-bundle", "rpc", "samples", "concurrency", "interval-ms", "maximum-latency-ms", "budget-seconds", "max-block-age-seconds", "minimum-peers"} {
		flags.Func(name, "", func(value string) error {
			if seen[name] {
				return godtestnet.ErrReadLoad
			}
			seen[name], values[name] = true, value
			return nil
		})
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return godtestnet.ErrReadLoad
	}
	ints := map[string]int{}
	for _, name := range []string{"samples", "concurrency", "interval-ms", "maximum-latency-ms", "budget-seconds", "max-block-age-seconds", "minimum-peers"} {
		if !seen[name] && name != "max-block-age-seconds" && name != "minimum-peers" {
			return godtestnet.ErrReadLoad
		}
		n, e := strconv.Atoi(values[name])
		if e != nil || n < 0 || n > 8000 || strconv.Itoa(n) != values[name] {
			return godtestnet.ErrReadLoad
		}
		ints[name] = n
	}
	o := godtestnet.ReadLoadOptions{BundlePath: values["bundle"], ExpectedBundle: values["expected-bundle"], RPC: values["rpc"],
		Samples: ints["samples"], Concurrency: ints["concurrency"], Interval: time.Duration(ints["interval-ms"]) * time.Millisecond,
		MaximumLatency: time.Duration(ints["maximum-latency-ms"]) * time.Millisecond, Budget: time.Duration(ints["budget-seconds"]) * time.Second,
		MaxBlockAge: time.Duration(ints["max-block-age-seconds"]) * time.Second, MinimumPeers: ints["minimum-peers"]}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	r, e := godtestnet.ReadLoad(ctx, o)
	if r.Reason == "configuration" {
		return godtestnet.ErrReadLoad
	}
	if json.NewEncoder(out).Encode(r) != nil {
		return godtestnet.ErrReadLoad
	}
	return e
}
