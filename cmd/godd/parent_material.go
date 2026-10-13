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

// Offline conditional verification only. No default paths, inferred pins,
// current-time assumptions, source RPC, persistence or activation side effects.
func runParentMaterial(command string, args []string, out io.Writer) error {
	if out == nil {
		return godrh.ErrParentMaterial
	}
	switch command {
	case "parent-trust-template":
		if len(args) != 0 {
			return godrh.ErrParentMaterial
		}
		_, err := out.Write(godrh.ParentTrustTemplate())
		return err
	case "parent-proof-template":
		if len(args) != 1 {
			return godrh.ErrParentMaterial
		}
		raw, err := godrh.ParentProofTemplate(args[0])
		if err != nil {
			return err
		}
		_, err = out.Write(raw)
		return err
	case "check-parent-proof":
		if len(args) != 5 {
			return godrh.ErrParentMaterial
		}
		trustPin, err := godrh.ParentMaterialPin(args[1])
		if err != nil {
			return err
		}
		proofPin, err := godrh.ParentMaterialPin(args[3])
		if err != nil {
			return err
		}
		current, err := godrh.ParentMaterialSlot(args[4])
		if err != nil || current == 0 {
			return godrh.ErrParentMaterial
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		r, err := godrh.ReviewPrivateParentMaterials(ctx, args[0], args[2], trustPin, proofPin, current)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(r)
	default:
		return godrh.ErrParentMaterial
	}
}
