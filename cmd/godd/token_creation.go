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

func runTokenCreation(command string, args []string, out io.Writer) error {
	if command == "rh-token-creation-template" {
		if len(args) != 1 {
			return godrh.ErrTokenCreation
		}
		raw, err := godrh.TokenCreationReplayTemplate(args[0])
		if err != nil {
			return err
		}
		_, err = out.Write(raw)
		return err
	}
	if command != "replay-rh-token-creation" || len(args) != 11 {
		return godrh.ErrTokenCreation
	}
	var pins [6][32]byte
	for i, index := range []int{1, 3, 5, 7, 9, 10} {
		pin, err := godrh.SourceMaterialPin(args[index])
		if err != nil {
			return godrh.ErrTokenCreation
		}
		pins[i] = pin
	}
	compiler, err := godrh.NewTokenCompiler(args[4], pins[2], args[6], pins[3])
	if err != nil {
		return godrh.ErrTokenCreation
	}
	files := godrh.TokenCreationFiles{ConfigPath: args[0], ConfigPin: pins[0], BundlePath: args[2], BundlePin: pins[1],
		ReplayPath: args[8], ReplayPin: pins[4], InitPin: pins[5]}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	report, err := godrh.ReviewPrivateTokenCreation(ctx, files, compiler)
	if err != nil {
		return godrh.ErrTokenCreation
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
