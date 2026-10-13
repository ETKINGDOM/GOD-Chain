package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
)

type CustodyDeploymentFiles struct {
	SourcePath  string
	SourcePin   [32]byte
	RequestPath string
	RequestPin  [32]byte
}

func (CustodyDeploymentFiles) String() string {
	return "RH private custody deployment inputs (redacted)"
}
func (f CustodyDeploymentFiles) GoString() string           { return f.String() }
func (CustodyDeploymentFiles) MarshalJSON() ([]byte, error) { return []byte(`{"redacted":true}`), nil }

func loadCustodyDeployment(ctx context.Context, f CustodyDeploymentFiles, compiler TokenCompiler) (CustodyDeployment, error) {
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(f.SourcePath) || filepath.Clean(f.SourcePath) != f.SourcePath || len(f.SourcePath) > 4096 || strings.ContainsAny(f.SourcePath, "\x00\r\n") {
		return CustodyDeployment{}, ErrCustodyDeployment
	}
	source, err := tokenCompilerFile(ctx, f.SourcePath, f.SourcePin, 64<<10, false)
	if err != nil {
		return CustodyDeployment{}, ErrCustodyDeployment
	}
	request, err := readRHPrivateBytes(f.RequestPath, MaxCustodyRequestBytes, f.RequestPin)
	if err != nil {
		return CustodyDeployment{}, ErrCustodyDeployment
	}
	// Refuse bad private requests before invoking the compiler.
	if _, err := parseCustodyDeploymentRequest(request); err != nil {
		return CustodyDeployment{}, ErrCustodyDeployment
	}
	a, _, err := CompileCustodyDeployment(ctx, source, f.SourcePin, compiler)
	if err != nil {
		return CustodyDeployment{}, ErrCustodyDeployment
	}
	d, err := PrepareCustodyDeployment(a, request)
	if err != nil || !custodyDeploymentFilesStable(ctx, f, source, request) {
		return CustodyDeployment{}, ErrCustodyDeployment
	}
	d.report.PrivateInputsChecked = true
	return d, nil
}

func custodyDeploymentFilesStable(ctx context.Context, f CustodyDeploymentFiles, source, request []byte) bool {
	a, err := tokenCompilerFile(ctx, f.SourcePath, f.SourcePin, 64<<10, false)
	if err != nil || !bytes.Equal(a, source) {
		return false
	}
	b, err := readRHPrivateBytes(f.RequestPath, MaxCustodyRequestBytes, f.RequestPin)
	return err == nil && bytes.Equal(b, request) && ctx.Err() == nil
}

// Values below exist only in a new private packet. The default JSON/string
// formatting of the public opaque inputs/output never exports them.
func custodyDeploymentPacket(f CustodyDeploymentFiles, c TokenCompiler, d CustodyDeployment) ([]byte, error) {
	creator, predicted, raw, err := d.Payload()
	if err != nil {
		return nil, ErrCustodyDeployment
	}
	doc := struct {
		Version             uint32 `json:"version"`
		Kind                string `json:"kind"`
		SourceSHA256        string `json:"sourceSHA256"`
		RequestSHA256       string `json:"requestSHA256"`
		NodeSHA256          string `json:"nodeSHA256"`
		CompilerSHA256      string `json:"compilerSHA256"`
		Creator             string `json:"creator"`
		PredictedCustody    string `json:"predictedCustody"`
		UnsignedTransaction string `json:"unsignedTransaction"`
		ConstructionOnly    bool   `json:"constructionOnly"`
		Signed              bool   `json:"signed"`
		BroadcastEnabled    bool   `json:"broadcastEnabled"`
		RealAssetsReady     bool   `json:"realAssetsReady"`
	}{Version: 1, Kind: "GOD Chain unsigned custody deployment v1", SourceSHA256: hex.EncodeToString(f.SourcePin[:]), RequestSHA256: hex.EncodeToString(f.RequestPin[:]),
		NodeSHA256: hex.EncodeToString(c.nodePin[:]), CompilerSHA256: hex.EncodeToString(c.compilerPin[:]), Creator: "0x" + hex.EncodeToString(creator[:]),
		PredictedCustody: "0x" + hex.EncodeToString(predicted[:]), UnsignedTransaction: "0x" + hex.EncodeToString(raw), ConstructionOnly: true}
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil || len(encoded)+1 > MaxCustodyDeploymentPacketBytes {
		return nil, ErrCustodyDeployment
	}
	return append(encoded, '\n'), nil
}

func ReviewCustodyDeploymentFiles(ctx context.Context, f CustodyDeploymentFiles, c TokenCompiler) (CustodyDeploymentReport, error) {
	d, err := loadCustodyDeployment(ctx, f, c)
	if err != nil {
		return CustodyDeploymentReport{}, ErrCustodyDeployment
	}
	return d.Report(), nil
}

// Exclusive owner-only file creation, fsync and pinned readback. Partial files
// are retained on failure for review; no overwrite, delete, retry or dispatch.
func PrepareCustodyDeploymentFile(ctx context.Context, f CustodyDeploymentFiles, c TokenCompiler, output string) (CustodyDeploymentReport, error) {
	d, err := loadCustodyDeployment(ctx, f, c)
	if err != nil {
		return CustodyDeploymentReport{}, ErrCustodyDeployment
	}
	raw, err := custodyDeploymentPacket(f, c, d)
	if err != nil || ctx.Err() != nil || writeRHPrivateFile(output, raw, MaxCustodyDeploymentPacketBytes) != nil {
		return CustodyDeploymentReport{}, ErrCustodyDeployment
	}
	retained, err := readRHPrivateBytes(output, MaxCustodyDeploymentPacketBytes, sha256.Sum256(raw))
	if err != nil || !bytes.Equal(retained, raw) || ctx.Err() != nil {
		return CustodyDeploymentReport{}, ErrCustodyDeployment
	}
	d.report.UnsignedFileWritten, d.report.RetainedFileMatched = true, true
	return d.Report(), nil
}

// Recompile from all original pinned inputs and compare the retained packet
// byte for byte. Parsing an arbitrary packet never creates an admitted output.
func CheckCustodyDeploymentFile(ctx context.Context, f CustodyDeploymentFiles, c TokenCompiler, packet string, pin [32]byte) (CustodyDeploymentReport, error) {
	retained, err := readRHPrivateBytes(packet, MaxCustodyDeploymentPacketBytes, pin)
	if err != nil {
		return CustodyDeploymentReport{}, ErrCustodyDeployment
	}
	d, err := loadCustodyDeployment(ctx, f, c)
	if err != nil {
		return CustodyDeploymentReport{}, ErrCustodyDeployment
	}
	raw, err := custodyDeploymentPacket(f, c, d)
	if err != nil || !bytes.Equal(raw, retained) || ctx.Err() != nil {
		return CustodyDeploymentReport{}, ErrCustodyDeployment
	}
	second, err := readRHPrivateBytes(packet, MaxCustodyDeploymentPacketBytes, pin)
	if err != nil || !bytes.Equal(second, retained) {
		return CustodyDeploymentReport{}, ErrCustodyDeployment
	}
	d.report.RetainedFileMatched = true
	return d.Report(), nil
}
