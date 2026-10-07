package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/goddeploy"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtestnet"
)

func runAcceptance(command string, args []string, out io.Writer) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	if command == "host-check" {
		dir := flags.String("data-dir", "", "")
		space := flags.Uint64("minimum-free-bytes", 0, "")
		files := flags.Uint64("minimum-open-files", 0, "")
		if flags.Parse(args) != nil || flags.NArg() != 0 {
			return goddeploy.ErrHost
		}
		report, err := goddeploy.Host(goddeploy.HostOptions{DataDir: *dir, MinimumFreeBytes: *space, MinimumOpenFiles: *files})
		if encodeErr := json.NewEncoder(out).Encode(report); encodeErr != nil {
			return encodeErr
		}
		return err
	}
	bundle := flags.String("bundle", "", "")
	pin := flags.String("expected-bundle", "", "")
	rpc := flags.String("rpc", "", "")
	companion := flags.String("companion", "", "")
	samples := flags.Int("samples", 3, "")
	interval := flags.Int("interval-seconds", 5, "")
	age := flags.Int("max-block-age-seconds", 30, "")
	peers := flags.Int("minimum-peers", 0, "")
	if command != "smoke" || flags.Parse(args) != nil || flags.NArg() != 0 || *interval < 1 || *interval > 10 || *age < 10 || *age > 600 {
		return godtestnet.ErrSmoke
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	report, err := godtestnet.Smoke(ctx, godtestnet.SmokeOptions{BundlePath: *bundle, ExpectedBundle: *pin, RPC: *rpc, Companion: *companion, Samples: *samples, MinimumPeers: *peers, Interval: time.Duration(*interval) * time.Second, MaxBlockAge: time.Duration(*age) * time.Second})
	if encodeErr := json.NewEncoder(out).Encode(report); encodeErr != nil {
		return encodeErr
	}
	return err
}
