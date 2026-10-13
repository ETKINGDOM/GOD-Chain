package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godrh"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtestnet"
)

func runLaunchObserver(command string, args []string, out io.Writer) error {
	if out == nil || command != "init-launch-observer" && command != "serve-launch-observer" || len(args)%2 != 0 {
		return godtestnet.ErrLaunchRehearsal
	}
	seen := map[string]bool{}
	for i := 0; i < len(args); i += 2 {
		if !strings.HasPrefix(args[i], "--") || strings.Contains(args[i], "=") || seen[args[i]] {
			return godtestnet.ErrLaunchRehearsal
		}
		seen[args[i]] = true
	}
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	home := f.String("home", "", "")
	input := f.String("input", "", "")
	inputHash := f.String("input-pin", "", "")
	candidate := f.String("candidate", "", "")
	candidateHash := f.String("candidate-pin", "", "")
	var listen, peers, rpc, hosts, origins *string
	var reportSeconds *int
	if command == "init-launch-observer" {
		listen, peers = f.String("p2p", "", ""), f.String("peers", "", "")
		rpc, hosts, origins = f.String("rpc", "", ""), f.String("hosts", "", ""), f.String("origins", "", "")
	} else {
		reportSeconds = f.Int("report-seconds", 0, "")
	}
	if f.Parse(args) != nil || f.NArg() != 0 {
		return godtestnet.ErrLaunchRehearsal
	}
	inputPin, err := godrh.SourceMaterialPin(*inputHash)
	if err != nil {
		return godtestnet.ErrLaunchRehearsal
	}
	candidatePin, err := godrh.SourceMaterialPin(*candidateHash)
	if err != nil {
		return godtestnet.ErrLaunchRehearsal
	}
	inputs := godtestnet.LaunchRehearsalInputs{InputPath: *input, InputPin: inputPin, CandidatePath: *candidate, CandidatePin: candidatePin}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if command == "serve-launch-observer" {
		if *reportSeconds < 1 || *reportSeconds > 300 {
			return godtestnet.ErrLaunchRehearsal
		}
		return godtestnet.ServeLaunchObserver(ctx, *home, inputs, godtestnet.LaunchServiceOptions{ReportInterval: time.Duration(*reportSeconds) * time.Second}, out)
	}
	list := func(value string) []string {
		if value == "" {
			return []string{}
		}
		return strings.Split(value, ",")
	}
	report, err := godtestnet.InitializeLaunchObserver(ctx, *home, inputs, godtestnet.LaunchObserverOptions{P2PListen: *listen, Peers: list(*peers), RPCListen: *rpc, RPCHosts: list(*hosts), RPCOrigins: list(*origins)})
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(report)
}
