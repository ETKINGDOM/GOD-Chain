package main

import (
	"context"
	"encoding/json"
	"github.com/ETKINGDOM/GOD-Chain/internal/godrh"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func runLaunchAssembly(command string, args []string, out io.Writer) error {
	if out == nil {
		return godrh.ErrLaunchAssembly
	}
	if command == "launch-assembly-template" && len(args) == 0 {
		_, err := out.Write(godrh.LaunchAssemblyTemplate())
		return err
	}
	if (command != "check-launch-assembly" || len(args) != 2) && (command != "stage-launch-candidate" || len(args) != 3) {
		return godrh.ErrLaunchAssembly
	}
	pin, err := godrh.SourceMaterialPin(args[1])
	if err != nil {
		return godrh.ErrLaunchAssembly
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var r godrh.LaunchAssemblyReport
	if command == "check-launch-assembly" {
		r, err = godrh.ReviewPrivateLaunchAssembly(ctx, args[0], pin)
	} else {
		r, err = godrh.StagePrivateLaunchAssembly(ctx, args[0], pin, args[2])
	}
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(r)
}
