package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/ETKINGDOM/GOD-Chain/internal/godtestnet"
)

func runBundleFetch(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("fetch-bundle", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var source, pin, output string
	seen := map[string]bool{}
	for name, value := range map[string]*string{"source": &source, "expected-bundle": &pin, "output": &output} {
		flags.Func(name, "", func(text string) error {
			if seen[name] {
				return godtestnet.ErrBundleFetch
			}
			seen[name], *value = true, text
			return nil
		})
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return godtestnet.ErrBundleFetch
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	report, err := godtestnet.FetchBundle(ctx, source, pin, output)
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(report)
}
