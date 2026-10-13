package main

import (
	"encoding/json"
	"flag"
	"io"

	"github.com/ETKINGDOM/GOD-Chain/internal/godtestnet"
)

func runUpgradeCheck(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("upgrade-check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	values := map[string]string{}
	for _, name := range []string{"home", "signer-guard", "archive", "expected-archive", "candidate-dir"} {
		flags.Func(name, "", func(value string) error {
			if _, exists := values[name]; exists || value == "" {
				return godtestnet.ErrUpgradeCheck
			}
			values[name] = value
			return nil
		})
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 || len(values) != 5 {
		return godtestnet.ErrUpgradeCheck
	}
	r, err := godtestnet.CheckUpgradeCandidate(godtestnet.UpgradeCheckOptions{Home: values["home"],
		SignerGuard: values["signer-guard"], Archive: values["archive"], ExpectedArchive: values["expected-archive"], CandidateDir: values["candidate-dir"]})
	if err != nil || json.NewEncoder(out).Encode(r) != nil {
		return godtestnet.ErrUpgradeCheck
	}
	return nil
}
