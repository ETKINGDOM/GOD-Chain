package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godrh"
	"github.com/ETKINGDOM/GOD-Chain/internal/stack"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func run(args []string, out io.Writer) error {
	if len(args) == 0 || (len(args) == 1 && args[0] == "help") {
		_, err := fmt.Fprintln(out, "GOD Chain core prototype\nCommands: status, version, rh-template, check-rh <private-file>, probe-rh <private-file>\nRH checks are read-only and do not authorize activation.\nNode startup and real-asset operations are not enabled.")
		return err
	}
	if len(args) == 2 && (args[0] == "check-rh" || args[0] == "probe-rh") {
		return checkRH(args[0], args[1], out)
	}
	if len(args) != 1 {
		return fmt.Errorf("expected one command")
	}
	switch args[0] {
	case "status", "version":
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(stack.BuildStatus())
	case "rh-template":
		_, err := out.Write(godrh.Template())
		return err
	case "start", "init", "bridge", "mint":
		return fmt.Errorf("not enabled: only isolated synthetic node tests are available; public activation and real-asset release gates are not verified")
	default:
		return fmt.Errorf("unknown command; use help")
	}
}

func checkRH(command, path string, out io.Writer) error {
	config, err := godrh.LoadPrivate(path)
	if err != nil {
		return err
	}
	report := config.Report()
	if command == "probe-rh" && report.ConfigurationReady {
		source, err := godrh.NewHTTPSource(config)
		if err != nil {
			return err
		}
		defer source.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		report = godrh.Probe(ctx, config, source)
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return err
	}
	if !report.ConfigurationReady || command == "probe-rh" && !report.ReadOnlyProbePassed {
		return fmt.Errorf("RH read-only readiness checks did not pass; activation remains disabled")
	}
	return nil
}

func main() {
	config := sdk.GetConfig()
	godaddress.ConfigureSDK(config)
	config.Seal()
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
