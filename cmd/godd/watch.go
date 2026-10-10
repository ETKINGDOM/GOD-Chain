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

func runWatch(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("watch-health", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	values := map[string]string{"max-block-age-seconds": "60", "maximum-no-progress-seconds": "120", "minimum-peers": "0"}
	seen := map[string]bool{}
	for _, name := range []string{"bundle", "expected-bundle", "rpc", "samples", "interval-seconds", "max-block-age-seconds", "maximum-no-progress-seconds", "minimum-peers"} {
		flags.Func(name, "", func(value string) error {
			if seen[name] {
				return godtestnet.ErrWatch
			}
			seen[name], values[name] = true, value
			return nil
		})
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 || !seen["samples"] || !seen["interval-seconds"] {
		return godtestnet.ErrWatch
	}
	ints := map[string]int{}
	for _, name := range []string{"samples", "interval-seconds", "max-block-age-seconds", "maximum-no-progress-seconds", "minimum-peers"} {
		n, err := strconv.Atoi(values[name])
		if err != nil || strconv.Itoa(n) != values[name] || n < 0 || n > 10080 {
			return godtestnet.ErrWatch
		}
		ints[name] = n
	}
	o := godtestnet.WatchOptions{BundlePath: values["bundle"], ExpectedBundle: values["expected-bundle"], RPC: values["rpc"],
		Samples: ints["samples"], MinimumPeers: ints["minimum-peers"], Interval: time.Duration(ints["interval-seconds"]) * time.Second,
		MaxBlockAge: time.Duration(ints["max-block-age-seconds"]) * time.Second, MaximumNoProgress: time.Duration(ints["maximum-no-progress-seconds"]) * time.Second}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	encoder := json.NewEncoder(out)
	r, err := godtestnet.Watch(ctx, o, func(s godtestnet.WatchSample) error { return encoder.Encode(s) })
	// Invalid input never produces a private error or a misleading run record.
	if r.Reason == "configuration" {
		return godtestnet.ErrWatch
	}
	if encoder.Encode(r) != nil {
		return godtestnet.ErrWatch
	}
	return err
}
