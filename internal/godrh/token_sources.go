package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	MaxTokenSourceBundleBytes = 1 << 20
	maxTokenSourceBytes       = 512 << 10
	maxTokenSourceFileBytes   = 128 << 10
	maxTokenSettingsBytes     = 32 << 10
	tokenSourcePurpose        = "GOD Chain private token source material v1"
)

var ErrTokenSourceMaterial = errors.New("RH private token source material or retained pin rejected")

type tokenSourceFile struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Content string `json:"content"`
}

// This is a private evidence capsule, not an explorer response, compiler job,
// certification or permission verdict. The compiler output is a declaration
// until separately reproduced with an independently reviewed compiler.
type tokenSourceDocument struct {
	Purpose          string            `json:"purpose"`
	Version          uint32            `json:"version"`
	SourceChainID    string            `json:"sourceChainId"`
	TokenContract    string            `json:"tokenContract"`
	CompilerVersion  string            `json:"compilerVersion"`
	CompilerSHA256   string            `json:"compilerSHA256"`
	CompilerSettings json.RawMessage   `json:"compilerSettings"`
	TargetSource     string            `json:"targetSource"`
	TargetContract   string            `json:"targetContract"`
	Sources          []tokenSourceFile `json:"sources"`
	CompiledRuntime  string            `json:"compiledRuntime"`
}

type TokenSourceBundle struct {
	private tokenSourceDocument
	runtime []byte
	valid   bool
}

func (TokenSourceBundle) String() string     { return "GOD Chain private token source material (redacted)" }
func (b TokenSourceBundle) GoString() string { return b.String() }
func (b TokenSourceBundle) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		MaterialValidated bool `json:"materialValidated"`
		ApprovalReady     bool `json:"approvalReady"`
		RealAssetsReady   bool `json:"realAssetsReady"`
	}{MaterialValidated: b.valid})
}

// Template contains no source, compiler pin, endpoint, address or runtime.
// It is intentionally not a valid evidence capsule until separately prepared.
func TokenSourceTemplate() []byte {
	raw, _ := json.MarshalIndent(tokenSourceDocument{
		Purpose: tokenSourcePurpose, Version: 1, CompilerSettings: json.RawMessage(`{}`), Sources: []tokenSourceFile{},
	}, "", "  ")
	return append(raw, '\n')
}

// SourceMaterialPin admits exactly a nonzero lowercase SHA-256 digest. Pins
// must be retained independently, not obtained from the capsule being checked.
func SourceMaterialPin(text string) ([32]byte, error) {
	var pin [32]byte
	if len(text) != 64 || strings.ToLower(text) != text {
		return pin, ErrTokenSourceMaterial
	}
	raw, err := hex.DecodeString(text)
	if err != nil {
		return pin, ErrTokenSourceMaterial
	}
	copy(pin[:], raw)
	if pin == [32]byte{} {
		return pin, ErrTokenSourceMaterial
	}
	return pin, nil
}

var tokenCompilerVersion = regexp.MustCompile(`^0\.[0-9]{1,2}\.[0-9]{1,2}\+commit\.[0-9a-f]{8}(\.Emscripten\.clang)?$`)
var tokenContractName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,99}$`)

// Paths are only canonical source labels. They are never extracted, opened,
// passed to a compiler or fetched. Limit case aliases for portable inventories.
func tokenSourcePath(text string) bool {
	if len(text) == 0 || len(text) > 512 || path.IsAbs(text) || path.Clean(text) != text || !strings.HasSuffix(text, ".sol") {
		return false
	}
	for _, ch := range text {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || strings.ContainsRune("/_-.@", ch)) {
			return false
		}
	}
	for _, part := range strings.Split(text, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

// Detect duplicate members (including escaped aliases), explicit null, trailing
// values and excessive depth before typed decoding. Source text remains inert
// string data; compiler settings are a bounded JSON object, never instructions.
func uniqueTokenSourceJSON(raw []byte) bool {
	if !tokenJSONUnicode(raw) {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value func(int) bool
	value = func(depth int) bool {
		if depth > 24 {
			return false
		}
		token, err := decoder.Token()
		if err != nil || token == nil {
			return false
		}
		delim, ok := token.(json.Delim)
		if !ok {
			return true
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				name, ok := key.(string)
				if err != nil || !ok || seen[name] || !value(depth+1) {
					return false
				}
				seen[name] = true
			}
			end, err := decoder.Token()
			return err == nil && end == json.Delim('}')
		case '[':
			for decoder.More() {
				if !value(depth + 1) {
					return false
				}
			}
			end, err := decoder.Token()
			return err == nil && end == json.Delim(']')
		default:
			return false
		}
	}
	if !value(0) {
		return false
	}
	_, err := decoder.Token()
	return err == io.EOF
}

// encoding/json replaces unpaired UTF-16 escapes silently. Refuse them so
// retained source bytes cannot be normalized into a different compiler input.
func tokenJSONUnicode(raw []byte) bool {
	inString := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			inString = !inString
			continue
		}
		if !inString || raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return false
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return false
		}
		unit, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if unit >= 0xdc00 && unit <= 0xdfff {
			return false
		}
		if unit < 0xd800 || unit > 0xdbff {
			continue
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return false
		}
		low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
		if err != nil || low < 0xdc00 || low > 0xdfff {
			return false
		}
		i += 6
	}
	return !inString
}

func exactTokenSourceObject(raw []byte, names ...string) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != len(names) {
		return false
	}
	for _, name := range names {
		if fields[name] == nil {
			return false
		}
	}
	return true
}

// ParseTokenSourceBundle first checks the independent whole-file pin. The
// bounded closed schema admits no approval flags, keys or explorer shortcuts.
// Source digests, a sorted inventory and exact compiled runtime are retained;
// this function performs no compilation, semantic audit or filesystem writes.
func ParseTokenSourceBundle(raw []byte, expected [32]byte) (TokenSourceBundle, error) {
	if expected == [32]byte{} || len(raw) == 0 || len(raw) > MaxTokenSourceBundleBytes || !utf8.Valid(raw) || sha256.Sum256(raw) != expected || !uniqueTokenSourceJSON(raw) ||
		!exactTokenSourceObject(raw, "purpose", "version", "sourceChainId", "tokenContract", "compilerVersion", "compilerSHA256", "compilerSettings", "targetSource", "targetContract", "sources", "compiledRuntime") {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	var d tokenSourceDocument
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&d) != nil || decoder.Decode(new(any)) != io.EOF || d.Version != 1 || d.Purpose != tokenSourcePurpose ||
		!tokenCompilerVersion.MatchString(d.CompilerVersion) || !tokenContractName.MatchString(d.TargetContract) || !tokenSourcePath(d.TargetSource) || len(d.Sources) == 0 || len(d.Sources) > 64 {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	if _, err := chainID(d.SourceChainID); err != nil {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	token, err := address(d.TokenContract)
	if err != nil || d.TokenContract != "0x"+hex.EncodeToString(token[:]) {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	if _, err := SourceMaterialPin(d.CompilerSHA256); err != nil {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	settings := bytes.TrimSpace(d.CompilerSettings)
	if len(settings) < 2 || len(settings) > maxTokenSettingsBytes || settings[0] != '{' || settings[len(settings)-1] != '}' {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	var sourceFields []json.RawMessage
	var root map[string]json.RawMessage
	_ = json.Unmarshal(raw, &root)
	if json.Unmarshal(root["sources"], &sourceFields) != nil {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	seen, target, total := map[string]bool{}, false, 0
	previous := ""
	for i, source := range d.Sources {
		key := strings.ToLower(source.Path)
		if !exactTokenSourceObject(sourceFields[i], "path", "sha256", "content") || !tokenSourcePath(source.Path) || source.Path <= previous || seen[key] ||
			len(source.Content) > maxTokenSourceFileBytes || strings.TrimSpace(source.Content) == "" || strings.ContainsRune(source.Content, 0) {
			return TokenSourceBundle{}, ErrTokenSourceMaterial
		}
		pin, err := SourceMaterialPin(source.SHA256)
		if err != nil || sha256.Sum256([]byte(source.Content)) != pin {
			return TokenSourceBundle{}, ErrTokenSourceMaterial
		}
		total += len(source.Content)
		if total > maxTokenSourceBytes {
			return TokenSourceBundle{}, ErrTokenSourceMaterial
		}
		seen[key], previous = true, source.Path
		target = target || source.Path == d.TargetSource
	}
	if !target || len(d.CompiledRuntime) < 4 || len(d.CompiledRuntime) > 2+2*maxCodeBytes || !strings.HasPrefix(d.CompiledRuntime, "0x") || strings.ToLower(d.CompiledRuntime) != d.CompiledRuntime {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	runtime, err := hex.DecodeString(d.CompiledRuntime[2:])
	if err != nil || len(runtime) == 0 {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	return TokenSourceBundle{private: d, runtime: runtime, valid: true}, nil
}

// LoadPrivateTokenSourceBundle is a bounded read-only file check. Final symlinks,
// shared permissions and changed file identity are refused. It does not extract
// source paths, follow imports, create output or claim hostile-filesystem safety.
func LoadPrivateTokenSourceBundle(filePath string, expected [32]byte) (TokenSourceBundle, error) {
	if expected == [32]byte{} {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	info, err := os.Lstat(filePath)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() <= 0 || info.Size() > MaxTokenSourceBundleBytes {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	file, err := os.Open(filePath)
	if err != nil {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() || opened.Mode().Perm()&0077 != 0 {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	raw, err := io.ReadAll(io.LimitReader(file, MaxTokenSourceBundleBytes+1))
	if err != nil {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	retained, err := os.Lstat(filePath)
	if err != nil || !os.SameFile(opened, retained) || retained.Mode().Perm()&0077 != 0 || retained.Size() != opened.Size() || !retained.ModTime().Equal(opened.ModTime()) {
		return TokenSourceBundle{}, ErrTokenSourceMaterial
	}
	return ParseTokenSourceBundle(raw, expected)
}

type TokenSourceReport struct {
	MaterialValidated           bool     `json:"materialValidated"`
	OfflineMaterialsMatched     bool     `json:"offlineMaterialsMatched"`
	ObservedRuntimeMatched      bool     `json:"observedRuntimeMatched"`
	CompilerExecutionVerified   bool     `json:"compilerExecutionVerified"`
	TokenPermissionsReviewed    bool     `json:"tokenPermissionsReviewed"`
	IndependentFinalityVerified bool     `json:"independentFinalityVerified"`
	ApprovalReady               bool     `json:"approvalReady"`
	RealAssetsReady             bool     `json:"realAssetsReady"`
	Checks                      []Check  `json:"checks"`
	RequiredReviews             []string `json:"requiredReviews"`
}

func tokenSourceReport() TokenSourceReport {
	return TokenSourceReport{Checks: []Check{}, RequiredReviews: []string{
		"compiler_execution_and_complete_inputs", "constructor_and_initial_allocation", "mint_and_supply_controls",
		"proxy_and_upgrade_authority", "pause_blacklist_and_roles", "transfer_tax_rebase_and_hooks",
		"custody_exact_transfer_compatibility", "authenticated_source_state_and_finality",
	}}
}

// CheckTokenSourceMaterials compares declarations with separately retained
// compiler/runtime pins and the private source binding. A pass is integrity
// evidence only, not execution of the compiler or proof that a token is safe.
func CheckTokenSourceMaterials(c Config, b TokenSourceBundle, compiler [32]byte) TokenSourceReport {
	r := tokenSourceReport()
	check := func(name string, ok bool) bool { r.Checks = append(r.Checks, Check{name, ok}); return ok }
	if !check("material_bundle_validated", b.valid) {
		return r
	}
	r.MaterialValidated = true
	tokenReport := c.TokenReport()
	if !check("source_configuration", tokenReport.ConnectionReady && tokenReport.RuntimePinConfigured) {
		return r
	}
	if !check("source_chain_binding", b.private.SourceChainID == c.private.SourceChainID) {
		return r
	}
	token, _ := address(c.private.TokenContract)
	declared, _ := address(b.private.TokenContract)
	if !check("source_token_binding", token == declared) {
		return r
	}
	declaredCompiler, _ := SourceMaterialPin(b.private.CompilerSHA256)
	if !check("compiler_declaration_pin", compiler != [32]byte{} && compiler == declaredCompiler) {
		return r
	}
	expected, _ := hash(c.private.ExpectedTokenCodeHash)
	if !check("compiled_runtime_declaration_pin", expected == tokenRuntimeDigest(b.runtime)) {
		return r
	}
	r.OfflineMaterialsMatched = true
	return r
}

// CompareTokenSourceMaterials uses only an already successful, fully pinned
// inspection from the exact same minimal configuration. It makes no new read,
// accepts no report-shaped substitute, and never approves compiler execution,
// token permissions, source finality or activation.
func CompareTokenSourceMaterials(ctx context.Context, c Config, inspection TokenInspection, b TokenSourceBundle, compiler [32]byte) TokenSourceReport {
	r := CheckTokenSourceMaterials(c, b, compiler)
	if !r.OfflineMaterialsMatched {
		return r
	}
	check := func(name string, ok bool) bool {
		ok = ok && ctx != nil && ctx.Err() == nil
		r.Checks = append(r.Checks, Check{name, ok})
		return ok
	}
	if !check("pinned_inspection_binding", inspection.report.PinnedTokenProbePassed && inspection.binding == tokenInspectionBinding(c)) {
		return r
	}
	if !check("exact_observed_runtime", bytes.Equal(inspection.runtime, b.runtime)) {
		return r
	}
	r.ObservedRuntimeMatched = true
	return r
}
