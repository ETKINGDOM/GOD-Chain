package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/ETKINGDOM/GOD-Chain/internal/godrh"
)

func runLaunchEconomics(command string, args []string, out io.Writer) error {
	if out == nil {
		return godrh.ErrLaunchEconomics
	}
	if command == "launch-economics-template" && len(args) == 0 {
		_, err := out.Write(godrh.LaunchEconomicsTemplate())
		return err
	}
	if command != "check-launch-economics" || len(args) != 2 {
		return godrh.ErrLaunchEconomics
	}
	pin, err := godrh.SourceMaterialPin(args[1])
	if err != nil {
		return godrh.ErrLaunchEconomics
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	r, err := godrh.ReviewPrivateLaunchEconomics(ctx, args[0], pin)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(out).Encode(r); err != nil {
		return err
	}
	// A well-formed inconsistent plan is shown but cannot get a successful
	// process exit. Even consistent arithmetic is explicitly not approval.
	if !r.ArithmeticConsistent {
		return godrh.ErrLaunchEconomics
	}
	return nil
}
