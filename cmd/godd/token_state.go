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

func runTokenState(command string, args []string, out io.Writer) error {
	failure := godrh.ErrTokenStateMaterial
	if command == "read-rh-token-state" || command == "rh-token-state-read-template" {
		failure = godrh.ErrTokenStateRead
	}
	if out == nil {
		return failure
	}
	if command == "rh-token-state-template" || command == "rh-token-state-read-template" {
		if len(args) != 0 {
			return failure
		}
		raw := godrh.TokenStateMaterialTemplate()
		if command == "rh-token-state-read-template" {
			raw = godrh.TokenStateReadTemplate()
		}
		_, err := out.Write(raw)
		return err
	}
	if (command != "check-rh-token-state" && command != "read-rh-token-state") || len(args) != 9 {
		return failure
	}
	f := godrh.TokenStateFiles{ConfigPath: args[0], BundlePath: args[2], ProofPath: args[4]}
	for i, out := range []*[32]byte{&f.ConfigPin, &f.BundlePin, &f.ProofPin, &f.ExpectedCompiler} {
		pin, err := godrh.SourceMaterialPin(args[[]int{1, 3, 5, 6}[i]])
		if err != nil {
			return failure
		}
		*out = pin
	}
	block, err := godrh.TokenStateBlock(args[7], args[8])
	if err != nil {
		return failure
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if command == "read-rh-token-state" {
		r, err := godrh.ReviewPrivateTokenStateRead(ctx, godrh.TokenStateReadFiles{
			ConfigPath: f.ConfigPath, ConfigPin: f.ConfigPin, BundlePath: f.BundlePath, BundlePin: f.BundlePin,
			RequestPath: f.ProofPath, RequestPin: f.ProofPin, ExpectedCompiler: f.ExpectedCompiler,
		}, block)
		if err != nil {
			return godrh.ErrTokenStateRead
		}
		return json.NewEncoder(out).Encode(r)
	}
	r, err := godrh.ReviewPrivateTokenState(ctx, f, block)
	if err != nil {
		return godrh.ErrTokenStateMaterial
	}
	return json.NewEncoder(out).Encode(r)
}
