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

func runLaunchRehearsal(command string, args []string, out io.Writer) error {
	if out == nil || command != "init-launch-rehearsal" && command != "run-launch-rehearsal" && command != "serve-launch-candidate" {
		return godtestnet.ErrLaunchRehearsal
	}
	if command == "serve-launch-candidate" {
		// No last-flag-wins ambiguity in a persistent service's private bindings.
		seen := map[string]bool{}
		if len(args)%2 != 0 {
			return godtestnet.ErrLaunchRehearsal
		}
		for i := 0; i < len(args); i += 2 {
			if !strings.HasPrefix(args[i], "--") || strings.Contains(args[i], "=") || seen[args[i]] {
				return godtestnet.ErrLaunchRehearsal
			}
			seen[args[i]] = true
		}
	}
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	home := f.String("home", "", "")
	guard := f.String("signer-guard", "", "")
	input := f.String("input", "", "")
	inputHash := f.String("input-pin", "", "")
	candidate := f.String("candidate", "", "")
	candidateHash := f.String("candidate-pin", "", "")
	var listen, peers *string
	var seconds *int
	var reportSeconds *int
	if command == "init-launch-rehearsal" {
		listen = f.String("p2p", "", "")
		peers = f.String("peers", "", "")
	} else if command == "run-launch-rehearsal" {
		seconds = f.Int("seconds", 0, "")
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
	if command == "run-launch-rehearsal" {
		if *seconds < 2 || *seconds > 180 {
			return godtestnet.ErrLaunchRehearsal
		}
		return godtestnet.RunLaunchRehearsal(ctx, *home, *guard, inputs, time.Duration(*seconds)*time.Second, out)
	}
	if command == "serve-launch-candidate" {
		if *reportSeconds < 1 || *reportSeconds > 300 {
			return godtestnet.ErrLaunchRehearsal
		}
		return godtestnet.ServeLaunchCandidate(ctx, *home, *guard, inputs, godtestnet.LaunchServiceOptions{ReportInterval: time.Duration(*reportSeconds) * time.Second}, out)
	}
	opts := godtestnet.LaunchRehearsalOptions{P2PListen: *listen, Peers: []string{}}
	if *peers != "" {
		opts.Peers = strings.Split(*peers, ",")
	}
	report, err := godtestnet.InitializeLaunchRehearsal(ctx, *home, *guard, inputs, opts)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(report)
}
