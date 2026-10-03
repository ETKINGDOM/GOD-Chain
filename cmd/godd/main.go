package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/stack"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func run(args []string, out io.Writer) error {
	if len(args) == 0 || (len(args) == 1 && args[0] == "help") {
		_, err := fmt.Fprintln(out, "GOD Chain core prototype\nCommands: status, version\nNode startup and real-asset operations are not enabled.")
		return err
	}
	if len(args) != 1 {
		return fmt.Errorf("expected one command")
	}
	switch args[0] {
	case "status", "version":
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(stack.BuildStatus())
	case "start", "init", "bridge", "mint":
		return fmt.Errorf("not enabled: only isolated synthetic node tests are available; public activation and real-asset release gates are not verified")
	default:
		return fmt.Errorf("unknown command; use help")
	}
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
