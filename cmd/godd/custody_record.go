package main

import (
	"context"
	"encoding/json"
	"io"

	"github.com/ETKINGDOM/GOD-Chain/internal/godrh"
)

func runRHCustodyRecord(command string, args []string, out io.Writer) error {
	if out == nil || len(args) != 6 {
		return godrh.ErrCustodyRecordState
	}
	if command != "init-rh-custody-record" && command != "export-rh-custody-record" && command != "recover-rh-custody-record" && command != "check-rh-custody-record" {
		return godrh.ErrCustodyRecordState
	}
	configPin, err := godrh.SourceMaterialPin(args[1])
	if err != nil {
		return godrh.ErrCustodyRecordState
	}
	requestPin, err := godrh.SourceMaterialPin(args[3])
	if err != nil {
		return godrh.ErrCustodyRecordState
	}
	inputs := godrh.CustodyFileInputs{ConfigurationPath: args[0], ConfigurationSHA256: configPin, RequestPath: args[2], RequestSHA256: requestPin}
	r, err := godrh.OpenPrivateCustodyRecord(context.Background(), inputs, args[4], args[5], command == "init-rh-custody-record")
	if err != nil {
		return err
	}
	report := r.Report()
	switch command {
	case "export-rh-custody-record":
		report, err = r.Export(context.Background())
	case "recover-rh-custody-record":
		report, err = r.Recover(context.Background())
	}
	closeErr := r.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return godrh.ErrCustodyRecordStorage
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return godrh.ErrCustodyRecordState
	}
	raw = append(raw, '\n')
	if n, err := out.Write(raw); err != nil || n != len(raw) {
		return godrh.ErrCustodyRecordReport
	}
	return nil
}
