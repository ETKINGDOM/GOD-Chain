// Package godrh provides private RH connection configuration and read-only
// compatibility checks. Neither configuration nor an RPC response authorizes
// source finality, bridge signatures, backed genesis or real-asset activation.
package godrh

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
)

const maxConfigBytes = 16 << 10

var (
	ErrConfig      = errors.New("RH private configuration rejected")
	ErrPrivateFile = errors.New("RH configuration requires a bounded private regular file")
)

type document struct {
	Version                 uint32   `json:"version"`
	Mode                    string   `json:"mode"`
	SourceChain             string   `json:"sourceChain"`
	NativeChain             string   `json:"nativeChain"`
	SourceChainID           string   `json:"sourceChainId"`
	RPCEndpoint             string   `json:"rpcEndpoint"`
	TokenContract           string   `json:"tokenContract"`
	CustodyContract         string   `json:"custodyContract"`
	ExpectedTokenCodeHash   string   `json:"expectedTokenCodeHash"`
	ExpectedCustodyCodeHash string   `json:"expectedCustodyCodeHash"`
	Signers                 []string `json:"signers"`
}

// Config owns detached private values. Formatting and JSON serialization are
// deliberately redacted; callers must never put operational values in logs.
type Config struct{ private document }

func (Config) String() string                 { return "GOD Chain RH configuration (redacted)" }
func (c Config) GoString() string             { return c.String() }
func (c Config) MarshalJSON() ([]byte, error) { return json.Marshal(c.Report()) }

// Template contains no addresses, endpoint, signer values or credentials.
func Template() []byte {
	raw, _ := json.MarshalIndent(document{Version: 1, Mode: "simulation", Signers: []string{}}, "", "  ")
	return append(raw, '\n')
}

// Parse accepts intentionally incomplete configuration for offline readiness
// reporting. Nonempty malformed values, unknown/duplicate/case-aliased fields
// and credential fields are rejected without echoing any input.
func Parse(raw []byte) (Config, error) {
	if len(raw) == 0 || len(raw) > maxConfigBytes || !utf8.Valid(raw) || validObject(raw) != nil {
		return Config{}, ErrConfig
	}
	var d document
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&d) != nil || decoder.Decode(new(any)) != io.EOF || d.Version != 1 ||
		(d.Mode != "simulation" && d.Mode != "production") {
		return Config{}, ErrConfig
	}
	c := Config{d}
	if c.validate() != nil {
		return Config{}, ErrConfig
	}
	return c, nil
}

func validObject(raw []byte) error {
	allowed := map[string]bool{}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(Template(), &fields)
	for field := range fields {
		allowed[field] = true
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return ErrConfig
	}
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok || !allowed[name] || seen[name] {
			return ErrConfig
		}
		seen[name] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return ErrConfig
		}
	}
	last, err := decoder.Token()
	if err != nil || last != json.Delim('}') || !seen["version"] || !seen["mode"] {
		return ErrConfig
	}
	_, err = decoder.Token()
	if err != io.EOF {
		return ErrConfig
	}
	return nil
}

// LoadPrivate refuses symlinks, shared permissions, oversized files and a
// changed file identity. Its errors never contain paths or file contents.
func LoadPrivate(path string) (Config, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() <= 0 || info.Size() > maxConfigBytes {
		return Config{}, ErrPrivateFile
	}
	file, err := os.Open(path)
	if err != nil {
		return Config{}, ErrPrivateFile
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() || opened.Mode().Perm()&0077 != 0 {
		return Config{}, ErrPrivateFile
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	if err != nil || len(raw) > maxConfigBytes {
		return Config{}, ErrPrivateFile
	}
	return Parse(raw)
}

var chainName = regexp.MustCompile(`^[A-Za-z0-9._-]{1,50}$`)

func chainID(text string) (*big.Int, error) {
	if len(text) == 0 || len(text) > 78 || text[0] == '0' {
		return nil, ErrConfig
	}
	for _, ch := range text {
		if ch < '0' || ch > '9' {
			return nil, ErrConfig
		}
	}
	n, ok := new(big.Int).SetString(text, 10)
	if !ok || n.Sign() <= 0 || n.BitLen() > 256 {
		return nil, ErrConfig
	}
	return n, nil
}

func address(text string) ([20]byte, error) {
	raw, err := godaddress.FromEVM(text)
	var value [20]byte
	if err != nil {
		return value, ErrConfig
	}
	copy(value[:], raw)
	if value == [20]byte{} {
		return value, ErrConfig
	}
	return value, nil
}

func hash(text string) ([32]byte, error) {
	var value [32]byte
	if len(text) != 66 || !strings.HasPrefix(text, "0x") {
		return value, ErrConfig
	}
	raw, err := hex.DecodeString(text[2:])
	if err != nil {
		return value, ErrConfig
	}
	copy(value[:], raw)
	if value == [32]byte{} {
		return value, ErrConfig
	}
	return value, nil
}

func endpoint(text, mode string) bool {
	if len(text) > 4096 || strings.TrimSpace(text) != text {
		return false
	}
	u, err := url.Parse(text)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return mode == "simulation" && u.Scheme == "http" && ip != nil && ip.IsLoopback()
}

func (c Config) validate() error {
	d := c.private
	for _, name := range []string{d.SourceChain, d.NativeChain} {
		if name != "" && !chainName.MatchString(name) {
			return ErrConfig
		}
	}
	if d.SourceChain != "" && d.SourceChain == d.NativeChain || d.RPCEndpoint != "" && !endpoint(d.RPCEndpoint, d.Mode) {
		return ErrConfig
	}
	if d.SourceChainID != "" {
		if _, err := chainID(d.SourceChainID); err != nil {
			return err
		}
	}
	for _, text := range []string{d.TokenContract, d.CustodyContract} {
		if text != "" {
			if _, err := address(text); err != nil {
				return err
			}
		}
	}
	if d.TokenContract != "" && d.CustodyContract != "" {
		token, _ := address(d.TokenContract)
		custody, _ := address(d.CustodyContract)
		if token == custody {
			return ErrConfig
		}
	}
	for _, text := range []string{d.ExpectedTokenCodeHash, d.ExpectedCustodyCodeHash} {
		if text != "" {
			if _, err := hash(text); err != nil {
				return err
			}
		}
	}
	if len(d.Signers) != 0 && len(d.Signers) != godbridge.SignerCount {
		return ErrConfig
	}
	seen := map[[20]byte]bool{}
	for _, text := range d.Signers {
		signer, err := address(text)
		if err != nil || seen[signer] {
			return ErrConfig
		}
		seen[signer] = true
	}
	return nil
}

type Check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
}

// Report contains only fixed status identifiers, never operational values.
type Report struct {
	Mode                string   `json:"mode"`
	ConfigurationReady  bool     `json:"configurationReady"`
	SourceChecked       bool     `json:"sourceChecked"`
	ReadOnlyProbePassed bool     `json:"readOnlyProbePassed"`
	RealAssetsReady     bool     `json:"realAssetsReady"`
	Missing             []string `json:"missing"`
	Checks              []Check  `json:"checks"`
	ReleaseGates        []string `json:"releaseGates"`
}

func (c Config) Report() Report {
	d := c.private
	r := Report{Mode: d.Mode, Missing: []string{}, Checks: []Check{}, ReleaseGates: []string{
		"independent_source_finality", "token_permissions_review", "independent_signer_operations",
		"recoverable_relayer", "source_backed_genesis_and_fee_funding", "delayed_network_payment",
		"production_node_wallet_and_rpc", "governance_and_operational_verification", "explicit_activation_authorization",
	}}
	for _, field := range []struct{ name, value string }{
		{"source_chain", d.SourceChain}, {"native_chain", d.NativeChain}, {"source_chain_id", d.SourceChainID},
		{"rpc_endpoint", d.RPCEndpoint}, {"token_contract", d.TokenContract}, {"custody_contract", d.CustodyContract},
		{"token_runtime_code_pin", d.ExpectedTokenCodeHash}, {"custody_runtime_code_pin", d.ExpectedCustodyCodeHash},
	} {
		if field.value == "" {
			r.Missing = append(r.Missing, field.name)
		}
	}
	if len(d.Signers) != godbridge.SignerCount {
		r.Missing = append(r.Missing, "seven_distinct_signers")
	}
	r.ConfigurationReady = d.Version == 1 && (d.Mode == "simulation" || d.Mode == "production") && len(r.Missing) == 0 && c.validate() == nil
	return r
}

// Binding returns a detached protocol binding only for a complete valid
// configuration. It does not install a keeper, fund accounts or authorize use.
func (c Config) Binding() (godbridge.Config, error) {
	if !c.Report().ConfigurationReady {
		return godbridge.Config{}, ErrConfig
	}
	d := c.private
	chain, _ := chainID(d.SourceChainID)
	token, _ := address(d.TokenContract)
	custody, _ := address(d.CustodyContract)
	id, err := godbridge.AssetBinding(sdkmath.NewIntFromBigInt(chain), token, custody, godrewards.Decimals)
	if err != nil {
		return godbridge.Config{}, ErrConfig
	}
	binding := godbridge.Config{Version: 1, SourceChain: d.SourceChain, NativeChain: d.NativeChain, AssetID: id}
	for i, text := range d.Signers {
		binding.Signers[i], _ = address(text)
	}
	return godbridge.CanonicalConfig(binding)
}

var _ fmt.Stringer = Config{}
