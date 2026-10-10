package main

import (
	"encoding/json"
	"io"

	"github.com/ETKINGDOM/GOD-Chain/internal/godrh"
)

// New commands explicitly select pinned offline files. No transaction signing,
// submission, provider reads, wallet inputs, activation or existing-file repair.
func runRHCustody(command string, args []string, out io.Writer) error {
	if out == nil {
		return godrh.ErrCustodyWorkflow
	}
	if command == "rh-custody-request-template" {
		if len(args) != 1 {
			return godrh.ErrCustodyWorkflow
		}
		raw, err := godrh.CustodyRequestTemplate(godrh.CustodyAction(args[0]))
		if err != nil {
			return godrh.ErrCustodyWorkflow
		}
		if n, err := out.Write(raw); err != nil || n != len(raw) {
			return godrh.ErrCustodyWorkflow
		}
		return nil
	}
	wanted := map[string]int{"check-rh-custody-request": 4, "prepare-rh-custody-call": 5, "check-rh-custody-call": 6}
	if wanted[command] == 0 || len(args) != wanted[command] {
		return godrh.ErrCustodyWorkflow
	}
	configPin, err := godrh.SourceMaterialPin(args[1])
	if err != nil {
		return godrh.ErrCustodyWorkflow
	}
	requestPin, err := godrh.SourceMaterialPin(args[3])
	if err != nil {
		return godrh.ErrCustodyWorkflow
	}
	inputs := godrh.CustodyFileInputs{ConfigurationPath: args[0], ConfigurationSHA256: configPin,
		RequestPath: args[2], RequestSHA256: requestPin}
	var report godrh.CustodyWorkflowReport
	switch command {
	case "check-rh-custody-request":
		report, err = godrh.CheckCustodyRequestFiles(inputs)
	case "prepare-rh-custody-call":
		report, err = godrh.PrepareCustodyCallFile(inputs, args[4])
	case "check-rh-custody-call":
		var packetPin [32]byte
		packetPin, err = godrh.SourceMaterialPin(args[5])
		if err == nil {
			report, err = godrh.CheckCustodyCallFile(inputs, args[4], packetPin)
		}
	}
	if err != nil {
		return godrh.ErrCustodyWorkflow
	}
	raw, marshalErr := json.MarshalIndent(report, "", "  ")
	if marshalErr != nil {
		return godrh.ErrCustodyWorkflow
	}
	raw = append(raw, '\n')
	if n, writeErr := out.Write(raw); writeErr != nil || n != len(raw) {
		// A successfully retained file may exist after report delivery fails.
		// Never wrap writer errors: they can contain private caller/provider text.
		return godrh.ErrCustodyWorkflow
	}
	return nil
}
