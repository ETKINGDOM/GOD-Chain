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

// Strict offline integrity check. Does not query an endpoint, run a compiler,
// extract source files, accept permission verdicts or update a runtime pin.
func checkRHTokenSources(configPath, bundlePath, bundlePinText, compilerPinText string, out io.Writer) error {
	bundlePin, err := godrh.SourceMaterialPin(bundlePinText)
	if err != nil {
		return err
	}
	compilerPin, err := godrh.SourceMaterialPin(compilerPinText)
	if err != nil {
		return err
	}
	c, err := godrh.LoadPrivate(configPath)
	if err != nil {
		return err
	}
	b, err := godrh.LoadPrivateTokenSourceBundle(bundlePath, bundlePin)
	if err != nil {
		return err
	}
	r := godrh.CheckTokenSourceMaterials(c, b, compilerPin)
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(r); err != nil {
		return err
	}
	if !r.OfflineMaterialsMatched {
		return godrh.ErrTokenSourceMaterial
	}
	return nil
}

// All pins are independently retained inputs. No CLI path is printed, no pin
// is inferred from the selected file, and no runtime is found through PATH.
func recompileRHToken(args []string, out io.Writer) error {
	if len(args) != 7 {
		return godrh.ErrTokenCompilation
	}
	bundlePin, err := godrh.SourceMaterialPin(args[2])
	if err != nil {
		return godrh.ErrTokenCompilation
	}
	nodePin, err := godrh.SourceMaterialPin(args[4])
	if err != nil {
		return godrh.ErrTokenCompilation
	}
	compilerPin, err := godrh.SourceMaterialPin(args[6])
	if err != nil {
		return godrh.ErrTokenCompilation
	}
	selection, err := godrh.NewTokenCompiler(args[3], nodePin, args[5], compilerPin)
	if err != nil {
		return err
	}
	c, err := godrh.LoadPrivate(args[0])
	if err != nil {
		return godrh.ErrTokenCompilation
	}
	b, err := godrh.LoadPrivateTokenSourceBundle(args[1], bundlePin)
	if err != nil {
		return godrh.ErrTokenCompilation
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	r := godrh.RecompileToken(ctx, c, b, selection)
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(r); err != nil {
		return err
	}
	if !r.CompilerExecutionVerified {
		return godrh.ErrTokenCompilation
	}
	return nil
}
