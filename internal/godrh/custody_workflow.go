package godrh

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
)

var ErrCustodyWorkflow = errors.New("RH offline custody file workflow refused; no signing, submission or asset activation")

const MaxCustodyPacketBytes = 12 << 10

// Independently reviewed whole-file checksums, not runtime code pins. The CLI
// never obtains a trusted checksum from the same selected, unreviewed input.
type CustodyFileInputs struct {
	ConfigurationPath   string
	ConfigurationSHA256 [32]byte
	RequestPath         string
	RequestSHA256       [32]byte
}

func (CustodyFileInputs) String() string               { return "RH offline custody file inputs (redacted)" }
func (i CustodyFileInputs) GoString() string           { return i.String() }
func (CustodyFileInputs) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

type CustodyWorkflowReport struct {
	Call                         CustodyCallReport `json:"call"`
	PrivateInputsChecked         bool              `json:"privateInputsChecked"`
	ConfigurationChecksumMatched bool              `json:"configurationChecksumMatched"`
	RequestChecksumMatched       bool              `json:"requestChecksumMatched"`
	UnsignedFileWritten          bool              `json:"unsignedFileWritten"`
	RetainedFileMatched          bool              `json:"retainedFileMatched"`
	PacketBytes                  int               `json:"packetBytes"`
}

// This serialization exists only inside new owner-only private files. It is a
// call description, NOT a signed transaction or approved source/native state.
type custodyPacketDocument struct {
	Version             uint32        `json:"version"`
	Kind                string        `json:"kind"`
	Action              CustodyAction `json:"action"`
	ConfigurationSHA256 string        `json:"configurationSHA256"`
	RequestSHA256       string        `json:"requestSHA256"`
	SourceChainID       string        `json:"sourceChainId"`
	CustodyTarget       string        `json:"custodyTarget"`
	Calldata            string        `json:"calldata"`
	Value               string        `json:"value"`
	ConstructionOnly    bool          `json:"constructionOnly"`
	Signed              bool          `json:"signed"`
	BroadcastEnabled    bool          `json:"broadcastEnabled"`
	RealAssetsReady     bool          `json:"realAssetsReady"`
}

func loadCustodyInputs(inputs CustodyFileInputs) (CustodyCall, error) {
	configRaw, err := readCustodyPrivateSnapshot(inputs.ConfigurationPath, maxConfigBytes, inputs.ConfigurationSHA256)
	if err != nil {
		return CustodyCall{}, ErrCustodyWorkflow
	}
	requestRaw, err := readCustodyPrivateSnapshot(inputs.RequestPath, MaxCustodyRequestBytes, inputs.RequestSHA256)
	if err != nil {
		return CustodyCall{}, ErrCustodyWorkflow
	}
	c, err := Parse(configRaw)
	if err != nil {
		return CustodyCall{}, ErrCustodyWorkflow
	}
	r, err := ParseCustodyRequest(requestRaw)
	if err != nil {
		return CustodyCall{}, ErrCustodyWorkflow
	}
	call, err := PrepareCustodyCall(c, r)
	if err != nil {
		return CustodyCall{}, ErrCustodyWorkflow
	}
	return call, nil
}

func custodyWorkflowReport(call CustodyCall) CustodyWorkflowReport {
	return CustodyWorkflowReport{Call: call.Report(), PrivateInputsChecked: true,
		ConfigurationChecksumMatched: true, RequestChecksumMatched: true}
}

func custodyPacketBytes(inputs CustodyFileInputs, call CustodyCall) ([]byte, error) {
	chain, target, data, err := call.Payload()
	if err != nil {
		return nil, ErrCustodyWorkflow
	}
	targetText, err := godaddress.ToEVM(target[:])
	if err != nil {
		return nil, ErrCustodyWorkflow
	}
	d := custodyPacketDocument{Version: 1, Kind: "GOD Chain unsigned custody call v1", Action: call.Report().Action,
		ConfigurationSHA256: hex.EncodeToString(inputs.ConfigurationSHA256[:]), RequestSHA256: hex.EncodeToString(inputs.RequestSHA256[:]),
		SourceChainID: chain.String(), CustodyTarget: targetText, Calldata: "0x" + hex.EncodeToString(data), Value: "0", ConstructionOnly: true}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil || len(raw)+1 > MaxCustodyPacketBytes {
		return nil, ErrCustodyWorkflow
	}
	return append(raw, '\n'), nil
}

// Read-only pinned request review. No source adapter, wallet, output file or
// submission object is created; reported readiness remains construction-only.
func CheckCustodyRequestFiles(inputs CustodyFileInputs) (CustodyWorkflowReport, error) {
	call, err := loadCustodyInputs(inputs)
	if err != nil {
		return CustodyWorkflowReport{}, ErrCustodyWorkflow
	}
	return custodyWorkflowReport(call), nil
}

// Writes only a new exclusive mode-0600 call file inside existing trusted
// owner-only storage. Failure never deletes/overwrites an existing or partial
// file. Review such a file separately; this workflow has no transaction retry.
func PrepareCustodyCallFile(inputs CustodyFileInputs, outputPath string) (CustodyWorkflowReport, error) {
	call, err := loadCustodyInputs(inputs)
	if err != nil {
		return CustodyWorkflowReport{}, ErrCustodyWorkflow
	}
	raw, err := custodyPacketBytes(inputs, call)
	if err != nil || writeCustodyPrivateFile(outputPath, raw) != nil {
		return CustodyWorkflowReport{}, ErrCustodyWorkflow
	}
	r := custodyWorkflowReport(call)
	r.UnsignedFileWritten, r.PacketBytes = true, len(raw)
	return r, nil
}

// Independently pins one retained private file and reconstructs every byte
// from separately pinned configuration/request snapshots. Even a freshly
// checksum-pinned altered target, selector, value, flag or extra field refuses.
// A match is not source finality, freshness, a signature or permission to submit.
func CheckCustodyCallFile(inputs CustodyFileInputs, packetPath string, expected [32]byte) (CustodyWorkflowReport, error) {
	call, err := loadCustodyInputs(inputs)
	if err != nil {
		return CustodyWorkflowReport{}, ErrCustodyWorkflow
	}
	retained, err := readCustodyPrivateSnapshot(packetPath, MaxCustodyPacketBytes, expected)
	if err != nil {
		return CustodyWorkflowReport{}, ErrCustodyWorkflow
	}
	wanted, err := custodyPacketBytes(inputs, call)
	if err != nil || !bytes.Equal(wanted, retained) {
		return CustodyWorkflowReport{}, ErrCustodyWorkflow
	}
	r := custodyWorkflowReport(call)
	r.RetainedFileMatched, r.PacketBytes = true, len(retained)
	return r, nil
}
