package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnativegateway"
)

type bootstrap struct {
	initialize  bool
	audit       bool
	destination string
	capacity    uint64
	expected    string
	hashes      []string
}

func parseMode(args []string) (godnativegateway.Config, bootstrap, error) {
	var c godnativegateway.Config
	var b bootstrap
	f := flag.NewFlagSet("godnative", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.StringVar(&c.Listen, "listen", "", "numeric loopback listen endpoint")
	f.StringVar(&c.Upstream, "upstream", "", "numeric loopback observer RPC URL")
	f.StringVar(&c.Origin, "origin", "", "exact HTTPS website origin")
	f.StringVar(&c.ExtensionOrigin, "extension-origin", "", "optional exact reviewed Chrome test origin")
	f.StringVar(&c.ChainID, "chain", "", "private synthetic chain identity")
	f.StringVar(&c.CompatibleChainID, "compatible-chain", "", "decimal compatible chain identity")
	f.Uint64Var(&c.Gas, "gas", 0, "fixed reviewed native gas limit")
	f.StringVar(&c.MinFeePerGas, "min-price", "", "positive agod per gas floor")
	f.BoolVar(&c.ValidatorOnly, "validator-only", false, "serve only one bounded unsigned validator lookup")
	f.StringVar(&c.AttemptDirectory, "attempt-directory", "", "existing owner-only native attempt directory")
	f.Uint64Var(&c.AttemptCapacity, "attempt-capacity", 0, "exact durable store capacity; zero retains the 1000-record default")
	f.BoolVar(&b.initialize, "initialize-attempts", false, "offline creation only; never start an HTTP server")
	f.BoolVar(&b.audit, "audit-attempts", false, "offline aggregate inventory only; never start an HTTP server")
	f.StringVar(&b.destination, "copy-attempts-to", "", "existing empty owner-only destination for an offline copy")
	f.Uint64Var(&b.capacity, "copy-attempt-capacity", 0, "explicit equal or larger destination capacity, at most 100000")
	f.StringVar(&b.expected, "expected-source-sha256", "", "reviewed exact lowercase source file SHA256 for offline copy")
	f.Func("blocked-hash", "exact reviewed legacy uncertain hash; repeat only during offline initialization", func(s string) error {
		if len(b.hashes) >= 1000 || len(s) != 66 || !strings.HasPrefix(s, "0x") || strings.Trim(s[2:], "0123456789abcdef") != "" {
			return godnativegateway.ErrGateway
		}
		b.hashes = append(b.hashes, s)
		return nil
	})
	if f.Parse(args) != nil || f.NArg() != 0 {
		return c, b, godnativegateway.ErrGateway
	}
	modes := 0
	for _, on := range []bool{b.initialize, b.audit, b.destination != ""} {
		if on {
			modes++
		}
	}
	copyFlag := false
	f.Visit(func(flag *flag.Flag) {
		if flag.Name == "copy-attempts-to" || flag.Name == "copy-attempt-capacity" || flag.Name == "expected-source-sha256" {
			copyFlag = true
		}
	})
	invalidMode := modes > 1 || len(b.hashes) != 0 && !b.initialize || modes != 0 && (c.ValidatorOnly || c.AttemptDirectory == "")
	invalidCapacity := c.AttemptCapacity != 0 && (c.AttemptCapacity < 1000 || c.AttemptCapacity > godnativegateway.MaxAttemptCapacity)
	invalidReadOnly := c.ValidatorOnly && (c.AttemptDirectory != "" || c.AttemptCapacity != 0)
	invalidCopy := copyFlag && (b.destination == "" || b.capacity < 1000 || b.capacity > godnativegateway.MaxAttemptCapacity || b.capacity < c.AttemptCapacity || len(b.expected) != 64 || strings.Trim(b.expected, "0123456789abcdef") != "")
	if invalidMode || invalidCapacity || invalidReadOnly || invalidCopy {
		return c, b, godnativegateway.ErrGateway
	}
	return c, b, nil
}

func parse(args []string) (godnativegateway.Config, error) {
	c, _, err := parseMode(args)
	return c, err
}

func main() {
	c, b, err := parseMode(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "native gateway configuration rejected")
		os.Exit(1)
	}
	if b.initialize {
		if godnativegateway.InitializeAttempts(c, b.hashes) != nil {
			fmt.Fprintln(os.Stderr, "native attempt initialization rejected")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, "native attempt store initialized; no server or node request started")
		return
	}
	if b.audit || b.destination != "" {
		var report any
		if b.audit {
			report, err = godnativegateway.AuditAttempts(c)
		} else {
			report, err = godnativegateway.CopyAttempts(c, b.destination, b.capacity, b.expected)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "native attempt maintenance rejected")
			os.Exit(1)
		}
		if json.NewEncoder(os.Stdout).Encode(report) != nil {
			fmt.Fprintln(os.Stderr, "native attempt report unavailable")
			os.Exit(1)
		}
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if godnativegateway.Serve(ctx, c) != nil {
		fmt.Fprintln(os.Stderr, "native gateway unavailable")
		os.Exit(1)
	}
}
