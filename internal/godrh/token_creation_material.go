package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	MaxTokenCreationMaterialBytes = 40 << 10
	tokenCreationPurpose          = "GOD Chain private token creation replay v1"
)

// Blank operator inputs. No creator, clock, arguments, pins or nonce are
// inferred from a configured endpoint or an explorer certification.
func TokenCreationReplayTemplate(scheme string) ([]byte, error) {
	if scheme != "create" && scheme != "create2" {
		return nil, ErrTokenCreation
	}
	d := map[string]any{"purpose": tokenCreationPurpose, "version": 1, "scheme": scheme,
		"creator": "", "blockNumber": "", "time": "", "gas": "", "arguments": ""}
	if scheme == "create" {
		d["nonce"] = ""
	} else {
		d["salt"] = ""
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, ErrTokenCreation
	}
	return append(raw, '\n'), nil
}

func tokenCreationDecimal(text string, positive bool) (uint64, error) {
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil || strconv.FormatUint(value, 10) != text || positive && value == 0 {
		return 0, ErrTokenCreation
	}
	return value, nil
}

// Exact, separately pinned private JSON. Decimal strings avoid rounding and
// scheme-specific keys forbid silently ignored salt/nonce or input defaults.
func ParseTokenCreationReplay(raw []byte, expected [32]byte) (TokenCreationReplay, error) {
	if expected == [32]byte{} || len(raw) == 0 || len(raw) > MaxTokenCreationMaterialBytes ||
		sha256.Sum256(raw) != expected || !utf8.Valid(raw) || !uniqueTokenSourceJSON(raw) {
		return TokenCreationReplay{}, ErrTokenCreation
	}
	var d struct {
		Purpose     string `json:"purpose"`
		Version     uint32 `json:"version"`
		Scheme      string `json:"scheme"`
		Creator     string `json:"creator"`
		Nonce       string `json:"nonce"`
		Salt        string `json:"salt"`
		BlockNumber string `json:"blockNumber"`
		Time        string `json:"time"`
		Gas         string `json:"gas"`
		Arguments   string `json:"arguments"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&d) != nil || decoder.Decode(new(any)) != io.EOF || d.Purpose != tokenCreationPurpose || d.Version != 1 {
		return TokenCreationReplay{}, ErrTokenCreation
	}
	keys := []string{"purpose", "version", "scheme", "creator", "blockNumber", "time", "gas", "arguments"}
	if d.Scheme == "create" {
		keys = append(keys, "nonce")
	} else if d.Scheme == "create2" {
		keys = append(keys, "salt")
	} else {
		return TokenCreationReplay{}, ErrTokenCreation
	}
	if !exactTokenSourceObject(raw, keys...) {
		return TokenCreationReplay{}, ErrTokenCreation
	}
	r := TokenCreationReplay{Scheme: d.Scheme}
	var err error
	r.Creator, err = address(d.Creator)
	if err != nil || d.Creator != "0x"+hex.EncodeToString(r.Creator[:]) {
		return TokenCreationReplay{}, ErrTokenCreation
	}
	if d.Scheme == "create" {
		r.Nonce, err = tokenCreationDecimal(d.Nonce, false)
		if err != nil || r.Nonce == math.MaxUint64 {
			return TokenCreationReplay{}, ErrTokenCreation
		}
	} else {
		if len(d.Salt) != 66 || !strings.HasPrefix(d.Salt, "0x") || strings.ToLower(d.Salt) != d.Salt {
			return TokenCreationReplay{}, ErrTokenCreation
		}
		salt, err := hex.DecodeString(d.Salt[2:])
		if err != nil {
			return TokenCreationReplay{}, ErrTokenCreation
		}
		copy(r.Salt[:], salt)
	}
	if r.BlockNumber, err = tokenCreationDecimal(d.BlockNumber, true); err != nil {
		return TokenCreationReplay{}, ErrTokenCreation
	}
	if r.Time, err = tokenCreationDecimal(d.Time, true); err != nil {
		return TokenCreationReplay{}, ErrTokenCreation
	}
	if r.Gas, err = tokenCreationDecimal(d.Gas, true); err != nil || r.Gas > MaxTokenCreationGas {
		return TokenCreationReplay{}, ErrTokenCreation
	}
	if len(d.Arguments) < 2 || len(d.Arguments) > 2+2*MaxTokenConstructorBytes || !strings.HasPrefix(d.Arguments, "0x") || strings.ToLower(d.Arguments) != d.Arguments {
		return TokenCreationReplay{}, ErrTokenCreation
	}
	r.Arguments, err = hex.DecodeString(d.Arguments[2:])
	if err != nil {
		return TokenCreationReplay{}, ErrTokenCreation
	}
	return r, nil
}

// Paths/pins are private operator inputs, not authority or source truth.
type TokenCreationFiles struct {
	ConfigPath, BundlePath, ReplayPath       string
	ConfigPin, BundlePin, ReplayPin, InitPin [32]byte
}

func (TokenCreationFiles) String() string     { return "GOD Chain private creation files (redacted)" }
func (f TokenCreationFiles) GoString() string { return f.String() }
func (TokenCreationFiles) MarshalJSON() ([]byte, error) {
	return []byte(`{"signingEnabled":false,"broadcastEnabled":false,"realAssetsReady":false}`), nil
}

type TokenCreationWorkflowReport struct {
	PrivateMaterialsPinned bool                           `json:"privateMaterialsPinned"`
	Compilation            TokenCreationCompilationReport `json:"compilation"`
	Replay                 TokenCreationReplayReport      `json:"replay"`
	PrivateFilesUnchanged  bool                           `json:"privateFilesUnchanged"`
	OfflineReplayMatched   bool                           `json:"offlineReplayMatched"`
	SigningEnabled         bool                           `json:"signingEnabled"`
	BroadcastEnabled       bool                           `json:"broadcastEnabled"`
	RealAssetsReady        bool                           `json:"realAssetsReady"`
}

func readTokenCreationPrivateSnapshot(path string, limit int64, pin [32]byte) ([]byte, error) {
	if limit != maxConfigBytes && limit != MaxTokenSourceBundleBytes && limit != MaxTokenCreationMaterialBytes {
		return nil, ErrTokenCreation
	}
	raw, err := readRHPrivateBytes(path, limit, pin)
	if err != nil {
		return nil, ErrTokenCreation
	}
	return raw, nil
}

// Reads pinned owner-only snapshots, compiles once, replays once, and checks
// every original snapshot again before returning success. No writes, source
// RPC, signed envelope or opaque artifact export. POSIX file support only.
func ReviewPrivateTokenCreation(ctx context.Context, files TokenCreationFiles, compiler TokenCompiler) (report TokenCreationWorkflowReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = TokenCreationWorkflowReport{}, ErrTokenCreation
		}
	}()
	if ctx == nil || ctx.Err() != nil || files.InitPin == [32]byte{} {
		return TokenCreationWorkflowReport{}, ErrTokenCreation
	}
	paths := []string{files.ConfigPath, files.BundlePath, files.ReplayPath}
	pins := [][32]byte{files.ConfigPin, files.BundlePin, files.ReplayPin}
	limits := []int64{maxConfigBytes, MaxTokenSourceBundleBytes, MaxTokenCreationMaterialBytes}
	raw := make([][]byte, len(paths))
	for i := range paths {
		raw[i], err = readTokenCreationPrivateSnapshot(paths[i], limits[i], pins[i])
		if err != nil || ctx.Err() != nil {
			return TokenCreationWorkflowReport{}, ErrTokenCreation
		}
	}
	c, err := Parse(raw[0])
	if err != nil {
		return TokenCreationWorkflowReport{}, ErrTokenCreation
	}
	b, err := ParseTokenSourceBundle(raw[1], files.BundlePin)
	if err != nil {
		return TokenCreationWorkflowReport{}, ErrTokenCreation
	}
	request, err := ParseTokenCreationReplay(raw[2], files.ReplayPin)
	if err != nil {
		return TokenCreationWorkflowReport{}, ErrTokenCreation
	}
	artifact, compiled := CompileTokenCreation(ctx, c, b, compiler)
	if !compiled.CreationArtifactReady {
		return TokenCreationWorkflowReport{}, ErrTokenCreation
	}
	replayed, err := ReproduceTokenCreation(ctx, artifact, request, files.InitPin)
	if err != nil || !replayed.ExactRuntimeReproduced {
		return TokenCreationWorkflowReport{}, ErrTokenCreation
	}
	for i := range paths {
		again, err := readTokenCreationPrivateSnapshot(paths[i], limits[i], pins[i])
		if err != nil || !bytes.Equal(raw[i], again) || ctx.Err() != nil {
			return TokenCreationWorkflowReport{}, ErrTokenCreation
		}
	}
	return TokenCreationWorkflowReport{PrivateMaterialsPinned: true, Compilation: compiled, Replay: replayed,
		PrivateFilesUnchanged: true, OfflineReplayMatched: true}, nil
}
