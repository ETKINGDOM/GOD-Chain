package main

import (
	"encoding/json"
	"io"

	"github.com/ETKINGDOM/GOD-Chain/internal/godrh"
)

// Offline review only. No attempt initialization/dispatch CLI is installed.
func runRHCustodyTransaction(command string, args []string, out io.Writer) error {
	if out == nil {
		return godrh.ErrCustodyTransaction
	}
	if command == "rh-custody-transaction-template" {
		if len(args) != 0 {
			return godrh.ErrCustodyTransaction
		}
		raw := godrh.CustodyTransactionTemplate()
		if n, err := out.Write(raw); err != nil || n != len(raw) {
			return godrh.ErrCustodyTransaction
		}
		return nil
	}
	if command != "check-rh-custody-transaction" || len(args) != 8 {
		return godrh.ErrCustodyTransaction
	}
	var pins [4][32]byte
	for n := range pins {
		var err error
		pins[n], err = godrh.SourceMaterialPin(args[n*2+1])
		if err != nil {
			return godrh.ErrCustodyTransaction
		}
	}
	i := godrh.CustodyTransactionInputs{CallInputs: godrh.CustodyFileInputs{ConfigurationPath: args[0], ConfigurationSHA256: pins[0], RequestPath: args[2], RequestSHA256: pins[1]},
		PlanPath: args[4], PlanSHA256: pins[2], TransactionPath: args[6], TransactionSHA256: pins[3]}
	report, err := godrh.CheckCustodyTransactionFiles(i)
	if err != nil {
		return godrh.ErrCustodyTransaction
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return godrh.ErrCustodyTransaction
	}
	raw = append(raw, '\n')
	if n, err := out.Write(raw); err != nil || n != len(raw) {
		return godrh.ErrCustodyTransaction
	}
	return nil
}
