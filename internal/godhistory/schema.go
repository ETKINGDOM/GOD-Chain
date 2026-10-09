// Package godhistory builds an independent, keyless synthetic address index.
// It stores only validated public identifiers, membership and execution status;
// never transaction payloads, memos, calldata, events, logs or signing material.
package godhistory

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
)

var ErrHistory = errors.New("synthetic history index unavailable")

const (
	PageLimit  = 20
	BlockLimit = 128
	BatchLimit = 128
	// Empty blocks still consume retained coverage. The independently selected
	// policy remains bounded by this ceiling and the unchanged 512 MiB file cap.
	MaxBlockBudget = 1000000
	maxReply       = 512 << 10
	maxFile        = 512 << 20
)

type Config struct {
	ChainID           string `json:"chainId"`
	CompatibleChainID string `json:"compatibleChainId"`
	BundleSHA256      string `json:"bundleSha256"`
	FirstHeight       int64  `json:"firstHeight"`
	MaxBlocks         uint64 `json:"maxBlocks"`
	MaxTransactions   uint64 `json:"maxTransactions"`
}

func (c Config) valid() bool {
	if len(c.ChainID) != len("god-test-")+32 || !strings.HasPrefix(c.ChainID, "god-test-") || !digest(c.ChainID[9:], 16) || !digest(c.BundleSHA256, 32) || c.FirstHeight < 1 || c.MaxBlocks < 1 || c.MaxBlocks > MaxBlockBudget || c.MaxTransactions < 1 || c.MaxTransactions > 100000 {
		return false
	}
	n, err := strconv.ParseUint(c.CompatibleChainID, 10, 64)
	return err == nil && n > 0 && strconv.FormatUint(n, 10) == c.CompatibleChainID
}

func digest(s string, n int) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == n && hex.EncodeToString(b) == s
}
func hash(s string) bool { return len(s) == 66 && strings.HasPrefix(s, "0x") && digest(s[2:], 32) }
func integer(s string, min, max int64) (int64, error) {
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil || n < min || n > max || strconv.FormatInt(n, 10) != s {
		return 0, ErrHistory
	}
	return n, nil
}
func stamp(s string) (time.Time, error) {
	t, e := time.Parse(time.RFC3339Nano, s)
	if e != nil || !strings.HasSuffix(s, "Z") || t.IsZero() {
		return time.Time{}, ErrHistory
	}
	return t, nil
}

// Reject duplicate keys recursively before decoding. Unknown source fields may
// be ignored only by the deliberate public-summary projection, never persisted.
func validJSON(raw []byte) bool {
	if len(raw) == 0 || len(raw) > maxReply {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var walk func(int) bool
	walk = func(depth int) bool {
		if depth > 16 {
			return false
		}
		token, e := d.Token()
		if e != nil {
			return false
		}
		if x, ok := token.(json.Delim); ok {
			switch x {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					k, e := d.Token()
					s, ok := k.(string)
					if e != nil || !ok || seen[s] || !walk(depth+1) {
						return false
					}
					seen[s] = true
				}
				end, e := d.Token()
				return e == nil && end == json.Delim('}')
			case '[':
				for d.More() {
					if !walk(depth + 1) {
						return false
					}
				}
				end, e := d.Token()
				return e == nil && end == json.Delim(']')
			default:
				return false
			}
		}
		return true
	}
	if !walk(0) {
		return false
	}
	_, e := d.Token()
	return e == io.EOF
}
func exact(raw []byte, out any) error {
	if !validJSON(raw) {
		return ErrHistory
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return ErrHistory
	}
	canonical, e := json.Marshal(out)
	if e != nil {
		return ErrHistory
	}
	var original, roundtrip any
	first, second := json.NewDecoder(bytes.NewReader(raw)), json.NewDecoder(bytes.NewReader(canonical))
	first.UseNumber()
	second.UseNumber()
	if first.Decode(&original) != nil || second.Decode(&roundtrip) != nil {
		return ErrHistory
	}
	a, _ := json.Marshal(original)
	b, _ := json.Marshal(roundtrip)
	if !bytes.Equal(a, b) {
		return ErrHistory
	}
	return nil
}

type commit struct {
	Height     string `json:"height"`
	Time       string `json:"time"`
	AppHash    string `json:"appHash"`
	Synthetic  bool   `json:"synthetic"`
	RealAssets bool   `json:"realAssets"`
}
type network struct {
	Commit      commit `json:"commit"`
	ChainID     string `json:"chainId"`
	Compatible  string `json:"evmChainId"`
	Denom       string `json:"nativeDenom"`
	Decimals    uint8  `json:"decimals"`
	Total       string `json:"totalGod"`
	Pool        string `json:"rewardPoolGod"`
	PendingPool string `json:"pendingRewardGod"`
	Outstanding string `json:"outstandingG"`
	Pending     string `json:"pendingG"`
}

func quantity(s string) bool {
	if len(s) < 1 || len(s) > 78 {
		return false
	}
	n, ok := new(big.Int).SetString(s, 10)
	return ok && n.Sign() >= 0 && n.BitLen() <= 256 && n.String() == s
}
func parseNetwork(raw []byte, c Config) (network, int64, error) {
	var n network
	if exact(raw, &n) != nil || n.ChainID != c.ChainID || n.Compatible != c.CompatibleChainID || n.Denom != "agod" || n.Decimals != 18 || n.Total != "1000000000000000000000000000" || !n.Commit.Synthetic || n.Commit.RealAssets || !digest(n.Commit.AppHash, 32) || !quantity(n.Pool) || !quantity(n.PendingPool) || !quantity(n.Outstanding) || !quantity(n.Pending) {
		return n, 0, ErrHistory
	}
	h, e := integer(n.Commit.Height, 1, 9223372036854775807)
	_, te := stamp(n.Commit.Time)
	if e != nil || te != nil {
		return n, 0, ErrHistory
	}
	return n, h, nil
}

type blockPage struct {
	Height       string            `json:"height"`
	Hash         string            `json:"hash"`
	Parent       string            `json:"parentHash"`
	Time         string            `json:"time"`
	Count        int               `json:"transactionCount"`
	Offset       string            `json:"offset"`
	Next         *string           `json:"nextOffset"`
	Transactions []json.RawMessage `json:"transactions"`
	Synthetic    bool              `json:"synthetic"`
	RealAssets   bool              `json:"realAssets"`
}
type Block struct {
	Height       string `json:"height"`
	Hash         string `json:"hash"`
	Parent       string `json:"parentHash"`
	Time         string `json:"time"`
	Transactions int    `json:"transactions"`
	Unsupported  uint64 `json:"unsupportedTransactions"`
}
type Record struct {
	Height         string `json:"height"`
	Index          string `json:"index"`
	ConsensusHash  string `json:"consensusHash"`
	CompatibleHash string `json:"compatibleHash"`
	Kind           string `json:"kind"`
	Execution      string `json:"execution"`
}
type projected struct {
	Record       Record
	Participants map[string]byte
	Unsupported  bool
}

func text(m map[string]json.RawMessage, k string) (string, error) {
	var s *string
	if json.Unmarshal(m[k], &s) != nil || s == nil {
		return "", ErrHistory
	}
	return *s, nil
}
func account(s string) ([]byte, error) {
	if strings.HasPrefix(s, "0x") {
		return godaddress.FromEVM(s)
	}
	b, e := godaddress.FromNative(s)
	if e != nil || strings.ToLower(s) != s {
		return nil, ErrHistory
	}
	return b, nil
}
func project(raw json.RawMessage, height int64, index int) (projected, error) {
	out := projected{Participants: map[string]byte{}}
	if !validJSON(raw) {
		return out, ErrHistory
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil || m == nil {
		return out, ErrHistory
	}
	get := func(k string) string { s, _ := text(m, k); return s }
	r := Record{Height: get("height"), Index: get("consensusIndex"), ConsensusHash: strings.ToLower(get("consensusHash")), Kind: get("kind")}
	var code *uint32
	var successful, synthetic, real *bool
	if json.Unmarshal(m["code"], &code) != nil || code == nil || json.Unmarshal(m["sdkSuccessful"], &successful) != nil || successful == nil || *successful != (*code == 0) || json.Unmarshal(m["synthetic"], &synthetic) != nil || synthetic == nil || !*synthetic || json.Unmarshal(m["realAssets"], &real) != nil || real == nil || *real || r.Height != strconv.FormatInt(height, 10) || r.Index != strconv.Itoa(index) || !hash(r.ConsensusHash) {
		return out, ErrHistory
	}
	add := func(s string, role byte) error {
		b, e := account(s)
		if e != nil {
			return ErrHistory
		}
		out.Participants[string(b)] |= role
		return nil
	}
	if r.Kind == "ethereum" {
		r.CompatibleHash = strings.ToLower(get("ethereumHash"))
		r.Execution = get("evmExecution")
		if !hash(r.CompatibleHash) || (r.Execution != "succeeded" && r.Execution != "failed" && r.Execution != "sdk-failed") || (*code != 0) != (r.Execution == "sdk-failed") || add(get("sender"), 1) != nil {
			return out, ErrHistory
		}
		a, e := account(get("sender"))
		b, f := godaddress.FromEVM(get("senderEVM"))
		if e != nil || f != nil || !bytes.Equal(a, b) {
			return out, ErrHistory
		}
		var input *int
		if json.Unmarshal(m["inputBytes"], &input) != nil || input == nil || *input < 0 || *input > 1048576 {
			return out, ErrHistory
		}
		if len(m["recipient"]) == 0 {
			return out, ErrHistory
		}
		if string(m["recipient"]) != "null" {
			s, e := text(m, "recipient")
			b, f := godaddress.FromEVM(s)
			if e != nil || f != nil {
				return out, ErrHistory
			}
			if !bytes.Equal(b, make([]byte, 20)) {
				out.Participants[string(b)] |= 2
			}
		}
	} else if r.Kind == "native" {
		r.Execution = "sdk-succeeded"
		if *code != 0 {
			r.Execution = "sdk-failed"
		}
		var ops []map[string]json.RawMessage
		if json.Unmarshal(m["operations"], &ops) != nil || len(ops) < 1 || len(ops) > 64 {
			return out, ErrHistory
		}
		for _, op := range ops {
			name, e := text(op, "operation")
			if e != nil {
				return out, ErrHistory
			}
			if name == "unsupported-summary" {
				out.Unsupported = true
				continue
			}
			if name != "delegate" && name != "undelegate" && name != "claim-g" && name != "transfer-g" && name != "donate-god" && name != "redeem-g" {
				return out, ErrHistory
			}
			s, e := text(op, "sender")
			if e != nil || !strings.HasPrefix(s, "god1") || add(s, 1) != nil {
				return out, ErrHistory
			}
			if name == "transfer-g" || name == "redeem-g" {
				s, e := text(op, "recipient")
				if e != nil || !strings.HasPrefix(s, "god1") || add(s, 2) != nil {
					return out, ErrHistory
				}
			}
		}
	} else {
		return out, ErrHistory
	}
	out.Record = r
	return out, nil
}
