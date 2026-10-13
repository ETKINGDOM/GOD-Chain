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

// Offline only: no RPC endpoint, key, nonce allocation, signing or dispatch
// flags. Every populated input and toolchain file needs an explicit retained pin.
func runCustodyDeployment(command string, args []string, out io.Writer) error {
	if out == nil {
		return godrh.ErrCustodyDeployment
	}
	if command == "rh-custody-deployment-template" {
		if len(args) != 0 {
			return godrh.ErrCustodyDeployment
		}
		raw := godrh.CustodyDeploymentTemplate()
		if n, err := out.Write(raw); err != nil || n != len(raw) {
			return godrh.ErrCustodyDeployment
		}
		return nil
	}
	wanted := map[string]int{"review-rh-custody-deployment": 8, "prepare-rh-custody-deployment": 9, "check-rh-custody-deployment": 10}
	if wanted[command] == 0 || len(args) != wanted[command] {
		return godrh.ErrCustodyDeployment
	}
	var pins [4][32]byte
	for i, index := range []int{1, 3, 5, 7} {
		pin, err := godrh.SourceMaterialPin(args[index])
		if err != nil {
			return godrh.ErrCustodyDeployment
		}
		pins[i] = pin
	}
	c, err := godrh.NewTokenCompiler(args[4], pins[2], args[6], pins[3])
	if err != nil {
		return godrh.ErrCustodyDeployment
	}
	f := godrh.CustodyDeploymentFiles{SourcePath: args[0], SourcePin: pins[0], RequestPath: args[2], RequestPin: pins[1]}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	var r godrh.CustodyDeploymentReport
	switch command {
	case "review-rh-custody-deployment":
		r, err = godrh.ReviewCustodyDeploymentFiles(ctx, f, c)
	case "prepare-rh-custody-deployment":
		r, err = godrh.PrepareCustodyDeploymentFile(ctx, f, c, args[8])
	case "check-rh-custody-deployment":
		var pin [32]byte
		pin, err = godrh.SourceMaterialPin(args[9])
		if err == nil {
			r, err = godrh.CheckCustodyDeploymentFile(ctx, f, c, args[8], pin)
		}
	}
	if err != nil {
		return godrh.ErrCustodyDeployment
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return godrh.ErrCustodyDeployment
	}
	raw = append(raw, '\n')
	if n, err := out.Write(raw); err != nil || n != len(raw) {
		return godrh.ErrCustodyDeployment
	}
	return nil
}
