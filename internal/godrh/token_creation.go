package godrh

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
)

const (
	MaxTokenCreationInputBytes = 48 << 10
	MaxTokenConstructorBytes   = 16 << 10
	maxTokenImmutableGroups    = 64
	maxTokenImmutableSites     = 256
	maxTokenCreatedCodeBytes   = 24 << 10
)

var ErrTokenCreation = errors.New("GOD Chain offline token creation replay rejected")

type tokenImmutableSite struct {
	Start  uint32 `json:"start"`
	Length uint32 `json:"length"`
}

type tokenImmutableGroup struct{ sites []tokenImmutableSite }

// Only a completed pinned compiler invocation constructs an admitted artifact.
// Runtime bytes and constructor parameters remain private. This artifact is not
// a creation-transaction proof, source-state proof or financial admission token.
type CompiledTokenCreation struct {
	creation, template, expected []byte
	groups                       []tokenImmutableGroup
	chain                        string
	token                        [20]byte
	valid                        bool
}

func (CompiledTokenCreation) String() string     { return "GOD Chain private compiled creation (redacted)" }
func (a CompiledTokenCreation) GoString() string { return a.String() }
func (a CompiledTokenCreation) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		CreationArtifactReady bool `json:"creationArtifactReady"`
		RealAssetsReady       bool `json:"realAssetsReady"`
	}{CreationArtifactReady: a.valid})
}

// Input is an explicit private-byte export, not transaction construction. It
// contains no nonce, fee, signature, source destination or broadcast capability.
// The full init-code pin used for replay must be retained separately.
func (a CompiledTokenCreation) Input(arguments []byte) ([]byte, error) {
	if !a.valid || len(arguments) > MaxTokenConstructorBytes || len(a.creation)+len(arguments) > MaxTokenCreationInputBytes {
		return nil, ErrTokenCreation
	}
	return append(bytes.Clone(a.creation), arguments...), nil
}

type TokenCreationCompilationReport struct {
	TokenCompilationReport
	CreationArtifactReady bool `json:"creationArtifactReady"`
	ImmutableGroups       int  `json:"immutableGroups"`
}

type tokenCreationCompileOutput struct {
	tokenCompileOutput
	Creation               string                     `json:"creation"`
	CreationLinkReferences map[string]json.RawMessage `json:"creationLinkReferences"`
}

func parseTokenCreationCompileOutput(raw []byte) (tokenCreationCompileOutput, error) {
	var out tokenCreationCompileOutput
	if len(raw) == 0 || len(raw) > maxTokenCompilerOutputBytes || !uniqueTokenSourceJSON(raw) ||
		!exactTokenSourceObject(raw, "version", "hasErrors", "runtime", "linkReferences", "immutableReferences", "creation", "creationLinkReferences") {
		return out, ErrTokenCreation
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(&out) != nil || d.Decode(new(any)) != io.EOF || !tokenCompilerVersion.MatchString(out.Version) {
		return tokenCreationCompileOutput{}, ErrTokenCreation
	}
	return out, nil
}

func tokenCreationHex(text string, limit int) ([]byte, error) {
	if len(text) == 0 || len(text) > 2*limit || strings.ToLower(text) != text {
		return nil, ErrTokenCreation
	}
	raw, err := hex.DecodeString(text)
	if err != nil || len(raw) == 0 {
		return nil, ErrTokenCreation
	}
	return raw, nil
}

func tokenImmutableGroups(template []byte, refs map[string]json.RawMessage) ([]tokenImmutableGroup, error) {
	if len(refs) > maxTokenImmutableGroups {
		return nil, ErrTokenCreation
	}
	ids := make([]string, 0, len(refs))
	for id := range refs {
		n, err := strconv.ParseUint(id, 10, 32)
		if err != nil || n == 0 || strconv.FormatUint(n, 10) != id {
			return nil, ErrTokenCreation
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	used, count := make([]bool, len(template)), 0
	groups := make([]tokenImmutableGroup, 0, len(ids))
	for _, id := range ids {
		var rawSites []json.RawMessage
		if json.Unmarshal(refs[id], &rawSites) != nil || len(rawSites) == 0 || len(rawSites) > maxTokenImmutableSites-count {
			return nil, ErrTokenCreation
		}
		group := tokenImmutableGroup{}
		var previous uint64
		for i, raw := range rawSites {
			var site tokenImmutableSite
			if !exactTokenSourceObject(raw, "start", "length") || json.Unmarshal(raw, &site) != nil || site.Length != 32 ||
				uint64(site.Start)+32 > uint64(len(template)) || i > 0 && uint64(site.Start) < previous {
				return nil, ErrTokenCreation
			}
			previous = uint64(site.Start) + 32
			for j := int(site.Start); j < int(site.Start)+32; j++ {
				if used[j] || template[j] != 0 {
					return nil, ErrTokenCreation
				}
				used[j] = true
			}
			group.sites = append(group.sites, site)
		}
		count += len(group.sites)
		groups = append(groups, group)
	}
	return groups, nil
}

// CompileTokenCreation retains exact creation code and compiler-declared
// immutable sites. Unlike ordinary recompilation, it does not compare an
// uninitialized runtime template with deployed code. No caller-provided
// replacement words, wildcard matches or compiler-report substitutes are used.
// This candidate supports explicit Cancun compilation only, without libraries.
func CompileTokenCreation(ctx context.Context, c Config, b TokenSourceBundle, compiler TokenCompiler) (artifact CompiledTokenCreation, report TokenCreationCompilationReport) {
	defer func() {
		if recover() != nil {
			artifact, report = CompiledTokenCreation{}, TokenCreationCompilationReport{}
		}
	}()
	base, raw := runTokenCompiler(ctx, c, b, compiler, true)
	r := TokenCreationCompilationReport{TokenCompilationReport: base}
	check := func(name string, ok bool) bool {
		ok = ok && ctx != nil && ctx.Err() == nil
		r.Checks = append(r.Checks, Check{name, ok})
		return ok
	}
	if raw == nil {
		return CompiledTokenCreation{}, r
	}
	out, err := parseTokenCreationCompileOutput(raw)
	if !check("bounded_creation_result", err == nil) || !check("actual_compiler_version", out.Version == tokenCompilerReleases[hex.EncodeToString(compiler.compilerPin[:])]) {
		return CompiledTokenCreation{}, r
	}
	r.CompilerVersionMatched = true
	if !check("compilation_success", !out.HasErrors) {
		return CompiledTokenCreation{}, r
	}
	r.CompilationSucceeded = true
	var settings struct {
		EVMVersion string `json:"evmVersion"`
	}
	if !check("supported_creation_fork", json.Unmarshal(b.private.CompilerSettings, &settings) == nil && settings.EVMVersion == "cancun") ||
		!check("unlinked_creation_and_runtime", len(out.LinkReferences) == 0 && len(out.CreationLinkReferences) == 0) {
		return CompiledTokenCreation{}, r
	}
	creation, err := tokenCreationHex(out.Creation, MaxTokenCreationInputBytes)
	if !check("bounded_creation_code", err == nil) {
		return CompiledTokenCreation{}, r
	}
	template, err := tokenCreationHex(out.Runtime, maxTokenCreatedCodeBytes)
	if !check("bounded_runtime_template", err == nil && len(template) == len(b.runtime)) {
		return CompiledTokenCreation{}, r
	}
	groups, err := tokenImmutableGroups(template, out.ImmutableReferences)
	if !check("bounded_immutable_layout", err == nil) {
		return CompiledTokenCreation{}, r
	}
	token, err := address(b.private.TokenContract)
	if !check("retained_creation_binding", err == nil) {
		return CompiledTokenCreation{}, r
	}
	r.CreationArtifactReady, r.ImmutableGroups = true, len(groups)
	return CompiledTokenCreation{creation: creation, template: template, expected: bytes.Clone(b.runtime), groups: groups, chain: b.private.SourceChainID, token: token, valid: true}, r
}

// Only words actually returned by the bounded constructor replay may fill
// these compiler-declared zero sites. Repeated sites of one immutable must have
// identical values. Every other byte must equal the compiled template.
func tokenCreationRuntimeMatches(a CompiledTokenCreation, runtime []byte) bool {
	if !a.valid || len(runtime) != len(a.template) || !bytes.Equal(runtime, a.expected) {
		return false
	}
	exact := bytes.Clone(a.template)
	for _, group := range a.groups {
		if len(group.sites) == 0 {
			return false
		}
		first := int(group.sites[0].Start)
		word := runtime[first : first+32]
		for _, site := range group.sites {
			start := int(site.Start)
			if !bytes.Equal(word, runtime[start:start+32]) {
				return false
			}
			copy(exact[start:start+32], word)
		}
	}
	return bytes.Equal(exact, runtime)
}
