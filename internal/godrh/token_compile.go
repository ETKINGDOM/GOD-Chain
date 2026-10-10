package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	maxTokenCompilerBytes       = 32 << 20
	maxTokenRuntimeBytes        = 256 << 20
	maxTokenCompilerOutputBytes = 1 << 20
	TokenCompilationTimeout     = 50 * time.Second
)

var ErrTokenCompilation = errors.New("GOD Chain private offline token recompilation rejected")

// Existing project release pins, not a new compiler/dependency selection. Only
// these self-contained soljson bytes are executable by this worker. Extending
// this list requires new source review and actual compiler acceptance tests.
var tokenCompilerReleases = map[string]string{
	"3bb491a777f76dff9b2fe73ce29ca20e21a971b59a2f2cf8f73415424f00be06": "0.8.30+commit.73712a01.Emscripten.clang",
	"d73fead3cbc860438f56f4d1723677499cdef212d0f8890241748d2d5afc1c5f": "0.8.37+commit.f401782d.Emscripten.clang",
}

//go:embed token_compiler_worker.cjs
var tokenCompilerWorker string

// Private path/pin selection is explicit; there is no PATH lookup, automatic
// download, wrapper require, endpoint, key or provider fallback. The runtime
// binary pin does not authenticate its dynamic libraries or the host OS.
type TokenCompiler struct {
	nodePath, compilerPath string
	nodePin, compilerPin   [32]byte
}

func NewTokenCompiler(nodePath string, nodePin [32]byte, compilerPath string, compilerPin [32]byte) (TokenCompiler, error) {
	if !filepath.IsAbs(nodePath) || !filepath.IsAbs(compilerPath) || len(nodePath) > 4096 || len(compilerPath) > 4096 ||
		strings.ContainsAny(nodePath+compilerPath, "\x00\r\n") || nodePin == [32]byte{} || compilerPin == [32]byte{} {
		return TokenCompiler{}, ErrTokenCompilation
	}
	return TokenCompiler{nodePath: nodePath, nodePin: nodePin, compilerPath: compilerPath, compilerPin: compilerPin}, nil
}

func (TokenCompiler) String() string {
	return "GOD Chain private offline compiler selection (redacted)"
}
func (c TokenCompiler) GoString() string { return c.String() }
func (TokenCompiler) MarshalJSON() ([]byte, error) {
	return []byte(`{"approvalReady":false,"realAssetsReady":false}`), nil
}

type TokenCompilationReport struct {
	OfflineMaterialsMatched     bool     `json:"offlineMaterialsMatched"`
	RuntimeFileMatched          bool     `json:"runtimeFileMatched"`
	CompilerFileMatched         bool     `json:"compilerFileMatched"`
	CompilerWorkerCompleted     bool     `json:"compilerWorkerCompleted"`
	CompilerVersionMatched      bool     `json:"compilerVersionMatched"`
	CompilationSucceeded        bool     `json:"compilationSucceeded"`
	RecompiledRuntimeMatched    bool     `json:"recompiledRuntimeMatched"`
	CompilerExecutionVerified   bool     `json:"compilerExecutionVerified"`
	ToolchainAuthenticated      bool     `json:"toolchainAuthenticated"`
	TokenPermissionsReviewed    bool     `json:"tokenPermissionsReviewed"`
	IndependentFinalityVerified bool     `json:"independentFinalityVerified"`
	ApprovalReady               bool     `json:"approvalReady"`
	RealAssetsReady             bool     `json:"realAssetsReady"`
	Checks                      []Check  `json:"checks"`
	RequiredReviews             []string `json:"requiredReviews"`
}

// Compile only the supplied bounded source inventory. Refuse nonempty external
// linking/remappings and unsupported settings rather than silently removing
// semantic inputs. Only outputSelection is replaced by minimal exact-target
// runtime outputs; other admitted compiler settings remain unchanged.
func tokenCompilationInput(b TokenSourceBundle) ([]byte, error) {
	var settings map[string]json.RawMessage
	if !b.valid || json.Unmarshal(b.private.CompilerSettings, &settings) != nil {
		return nil, ErrTokenCompilation
	}
	for name := range settings {
		switch name {
		case "optimizer", "viaIR", "evmVersion", "metadata", "outputSelection":
		case "libraries":
			var v map[string]json.RawMessage
			if json.Unmarshal(settings[name], &v) != nil || len(v) != 0 {
				return nil, ErrTokenCompilation
			}
		case "remappings":
			var v []string
			if json.Unmarshal(settings[name], &v) != nil || len(v) != 0 {
				return nil, ErrTokenCompilation
			}
		default:
			return nil, ErrTokenCompilation
		}
	}
	var fork string
	if json.Unmarshal(settings["evmVersion"], &fork) != nil || fork == "" {
		return nil, ErrTokenCompilation
	}
	selection, err := json.Marshal(map[string]any{b.private.TargetSource: map[string]any{b.private.TargetContract: []string{
		"evm.deployedBytecode.object", "evm.deployedBytecode.linkReferences", "evm.deployedBytecode.immutableReferences",
	}}})
	if err != nil {
		return nil, ErrTokenCompilation
	}
	settings["outputSelection"] = selection
	sources := map[string]any{}
	for _, source := range b.private.Sources {
		sources[source.Path] = struct {
			Content string `json:"content"`
		}{source.Content}
	}
	input, err := json.Marshal(struct {
		Language string                     `json:"language"`
		Sources  map[string]any             `json:"sources"`
		Settings map[string]json.RawMessage `json:"settings"`
	}{"Solidity", sources, settings})
	if err != nil || len(input) > MaxTokenSourceBundleBytes {
		return nil, ErrTokenCompilation
	}
	return input, nil
}

// Read bounded quiescent regular files and recheck identity, permissions and
// digest. Final symlinks and shared-writable files are refused. Source files are
// never opened by their labels. This is not a hostile-filesystem execution lock.
func tokenCompilerFile(ctx context.Context, path string, pin [32]byte, limit int64, executable bool) ([]byte, error) {
	if ctx == nil || ctx.Err() != nil || pin == [32]byte{} {
		return nil, ErrTokenCompilation
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0022 != 0 || info.Size() <= 0 || info.Size() > limit ||
		executable && runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		return nil, ErrTokenCompilation
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, ErrTokenCompilation
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, ErrTokenCompilation
	}
	raw, err := io.ReadAll(io.LimitReader(file, limit+1))
	retained, statErr := os.Lstat(path)
	if err != nil || statErr != nil || int64(len(raw)) > limit || ctx.Err() != nil || !os.SameFile(opened, retained) ||
		retained.Mode().Perm()&0022 != 0 || retained.Size() != opened.Size() || !retained.ModTime().Equal(opened.ModTime()) || sha256.Sum256(raw) != pin {
		return nil, ErrTokenCompilation
	}
	return raw, nil
}

type tokenCompileOutput struct {
	Version             string                     `json:"version"`
	HasErrors           bool                       `json:"hasErrors"`
	Runtime             string                     `json:"runtime"`
	LinkReferences      map[string]json.RawMessage `json:"linkReferences"`
	ImmutableReferences map[string]json.RawMessage `json:"immutableReferences"`
}

func parseTokenCompileOutput(raw []byte) (tokenCompileOutput, error) {
	var output tokenCompileOutput
	if len(raw) == 0 || len(raw) > maxTokenCompilerOutputBytes || !uniqueTokenSourceJSON(raw) ||
		!exactTokenSourceObject(raw, "version", "hasErrors", "runtime", "linkReferences", "immutableReferences") {
		return output, ErrTokenCompilation
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&output) != nil || decoder.Decode(new(any)) != io.EOF || !tokenCompilerVersion.MatchString(output.Version) {
		return tokenCompileOutput{}, ErrTokenCompilation
	}
	return output, nil
}

// Do not embed bytes.Buffer: its promoted ReadFrom could bypass Write's bound
// when exec copies stdout with io.Copy.
type tokenCompileBuffer struct{ buffer bytes.Buffer }

func (b *tokenCompileBuffer) Len() int      { return b.buffer.Len() }
func (b *tokenCompileBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *tokenCompileBuffer) Write(raw []byte) (int, error) {
	if len(raw) > maxTokenCompilerOutputBytes-b.Len() {
		return 0, ErrTokenCompilation
	}
	return b.buffer.Write(raw)
}

// RecompileToken executes only an explicitly selected hash-checked local Node
// runtime and known self-contained soljson bytes. The embedded worker receives
// bytes through stdin, has no import/SMT callbacks, and emits no diagnostics.
// A complete match is local reproduction, not toolchain authenticity, original
// publisher identity, permissions, provider state, finality or financial use.
func RecompileToken(ctx context.Context, c Config, b TokenSourceBundle, compiler TokenCompiler) TokenCompilationReport {
	r := TokenCompilationReport{Checks: []Check{}, RequiredReviews: tokenSourceReport().RequiredReviews}
	check := func(name string, ok bool) bool {
		ok = ok && ctx != nil && ctx.Err() == nil
		r.Checks = append(r.Checks, Check{name, ok})
		return ok
	}
	materials := CheckTokenSourceMaterials(c, b, compiler.compilerPin)
	if !check("offline_materials", materials.OfflineMaterialsMatched) {
		return r
	}
	r.OfflineMaterialsMatched = true
	knownVersion := tokenCompilerReleases[hex.EncodeToString(compiler.compilerPin[:])]
	if !check("known_compiler_release", knownVersion != "" && knownVersion == b.private.CompilerVersion) {
		return r
	}
	input, err := tokenCompilationInput(b)
	if !check("bounded_compiler_input", err == nil) {
		return r
	}
	ctx, cancel := context.WithTimeout(ctx, TokenCompilationTimeout)
	defer cancel()
	_, err = tokenCompilerFile(ctx, compiler.nodePath, compiler.nodePin, maxTokenRuntimeBytes, true)
	if !check("runtime_file_pin", err == nil) {
		return r
	}
	r.RuntimeFileMatched = true
	code, err := tokenCompilerFile(ctx, compiler.compilerPath, compiler.compilerPin, maxTokenCompilerBytes, false)
	if !check("compiler_file_pin", err == nil) {
		return r
	}
	r.CompilerFileMatched = true
	job, err := json.Marshal(struct {
		Purpose        string `json:"purpose"`
		Compiler       []byte `json:"compiler"`
		CompilerSHA256 string `json:"compilerSHA256"`
		Input          string `json:"input"`
		TargetSource   string `json:"targetSource"`
		TargetContract string `json:"targetContract"`
	}{"GOD Chain private offline compiler job v1", code, hex.EncodeToString(compiler.compilerPin[:]), string(input), b.private.TargetSource, b.private.TargetContract})
	if !check("private_compiler_job", err == nil) {
		return r
	}
	command := exec.CommandContext(ctx, compiler.nodePath, "--max-old-space-size=512", "-e", tokenCompilerWorker)
	command.Env = []string{"TZ=UTC"}
	if runtime.GOOS == "windows" {
		command.Env = append(command.Env, "SystemRoot="+os.Getenv("SystemRoot"))
	}
	command.Stdin = bytes.NewReader(job)
	var output tokenCompileBuffer
	command.Stdout = &output
	command.Stderr = io.Discard
	command.WaitDelay = time.Second
	if !check("compiler_worker", command.Run() == nil) {
		return r
	}
	r.CompilerWorkerCompleted = true
	// Re-read both inputs after the subprocess; no digest is automatically saved.
	_, err = tokenCompilerFile(ctx, compiler.nodePath, compiler.nodePin, maxTokenRuntimeBytes, true)
	if !check("runtime_file_stable", err == nil) {
		return r
	}
	retained, err := tokenCompilerFile(ctx, compiler.compilerPath, compiler.compilerPin, maxTokenCompilerBytes, false)
	if !check("compiler_file_stable", err == nil && bytes.Equal(code, retained)) {
		return r
	}
	result, err := parseTokenCompileOutput(output.Bytes())
	if !check("bounded_compiler_result", err == nil) {
		return r
	}
	if !check("actual_compiler_version", result.Version == knownVersion) {
		return r
	}
	r.CompilerVersionMatched = true
	if !check("compilation_success", !result.HasErrors) {
		return r
	}
	r.CompilationSucceeded = true
	if !check("no_unreviewed_replacements", len(result.LinkReferences) == 0 && len(result.ImmutableReferences) == 0) {
		return r
	}
	actual, err := hex.DecodeString(result.Runtime)
	if !check("exact_recompiled_runtime", err == nil && len(actual) > 0 && len(actual) <= maxCodeBytes && strings.ToLower(result.Runtime) == result.Runtime && bytes.Equal(actual, b.runtime)) {
		return r
	}
	r.RecompiledRuntimeMatched, r.CompilerExecutionVerified = true, true
	return r
}
