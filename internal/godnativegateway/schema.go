package godnativegateway

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnative"
)

var maxUint = new(big.Int).SetUint64(^uint64(0))
var maxInt = big.NewInt(1<<63 - 1)
var maxG = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
var fixedGod, _ = new(big.Int).SetString("1000000000000000000000000000", 10)

// Reject duplicate/case-folded object names at every depth before decoding.
// Bounds apply to trusted-local RPC responses as well as browser input.
func unique(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	count := 0
	var value func(int) error
	value = func(depth int) error {
		count++
		if depth > 16 || count > 1024 {
			return ErrGateway
		}
		t, err := d.Token()
		if err != nil {
			return ErrGateway
		}
		delim, container := t.(json.Delim)
		if !container {
			return nil
		}
		switch delim {
		case '{':
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				s, ok := k.(string)
				lower := strings.ToLower(s)
				if err != nil || !ok || seen[lower] {
					return ErrGateway
				}
				seen[lower] = true
				if value(depth+1) != nil {
					return ErrGateway
				}
			}
		case '[':
			for d.More() {
				if value(depth+1) != nil {
					return ErrGateway
				}
			}
		default:
			return ErrGateway
		}
		end, err := d.Token()
		if err != nil || delim == '{' && end != json.Delim('}') || delim == '[' && end != json.Delim(']') {
			return ErrGateway
		}
		return nil
	}
	if value(0) != nil {
		return ErrGateway
	}
	if _, err := d.Token(); err != io.EOF {
		return ErrGateway
	}
	return nil
}

// All non-optional struct keys are required with their exact spelling. This
// prevents encoding/json's case-insensitive aliases and missing false booleans.
func shape(raw []byte, t reflect.Type) error {
	if t == reflect.TypeOf(json.RawMessage{}) {
		return nil
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ErrGateway
	}
	if t.Kind() == reflect.Struct && t != reflect.TypeOf(time.Time{}) {
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			return ErrGateway
		}
		allowed := map[string]reflect.Type{}
		for j := 0; j < t.NumField(); j++ {
			f := t.Field(j)
			tag := strings.Split(f.Tag.Get("json"), ",")
			name := tag[0]
			if name == "" || name == "-" {
				continue
			}
			allowed[name] = f.Type
			v, ok := fields[name]
			if !ok {
				if strings.Contains(f.Tag.Get("json"), "omitempty") {
					continue
				}
				return ErrGateway
			}
			if shape(v, f.Type) != nil {
				return ErrGateway
			}
		}
		for name := range fields {
			if allowed[name] == nil {
				return ErrGateway
			}
		}
	} else if t.Kind() == reflect.Slice && t != reflect.TypeOf(json.RawMessage{}) {
		var values []json.RawMessage
		if json.Unmarshal(raw, &values) != nil {
			return ErrGateway
		}
		for _, v := range values {
			if shape(v, t.Elem()) != nil {
				return ErrGateway
			}
		}
	}
	return nil
}

func strict(raw []byte, out any) error {
	if unique(raw) != nil || shape(raw, reflect.TypeOf(out).Elem()) != nil || json.Unmarshal(raw, out) != nil {
		return ErrGateway
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
	Chain       string `json:"chainId"`
	Compatible  string `json:"evmChainId"`
	Denom       string `json:"nativeDenom"`
	Decimals    uint8  `json:"decimals"`
	Total       string `json:"totalGod"`
	Pool        string `json:"rewardPoolGod"`
	PendingPool string `json:"pendingRewardGod"`
	Outstanding string `json:"outstandingG"`
	PendingG    string `json:"pendingG"`
}
type account struct {
	Commit     commit `json:"commit"`
	Native     string `json:"nativeAddress"`
	Compatible string `json:"evmAddress"`
	Exists     bool   `json:"exists"`
	Module     bool   `json:"moduleAccount"`
	Number     string `json:"accountNumber"`
	Sequence   string `json:"sequence"`
	Balance    string `json:"balanceGod"`
	Spendable  string `json:"spendableG"`
	Unclaimed  string `json:"unclaimedG"`
	Pending    string `json:"pendingEarnedG"`
}
type delegation struct {
	Commit    commit `json:"commit"`
	Owner     string `json:"delegator"`
	Validator string `json:"validator"`
	Exists    bool   `json:"exists"`
	Shares    string `json:"shares"`
	God       string `json:"godEquivalentSmallestUnits"`
}
type transaction struct {
	Hash       string `json:"hash"`
	Height     string `json:"height"`
	Index      string `json:"index"`
	Code       uint32 `json:"code"`
	GasWanted  string `json:"gasWanted"`
	GasUsed    string `json:"gasUsed"`
	Included   bool   `json:"included"`
	Successful bool   `json:"sdkSuccessful"`
	Synthetic  bool   `json:"synthetic"`
	RealAssets bool   `json:"realAssets"`
}
type details struct {
	Hash       string            `json:"consensusHash"`
	Height     string            `json:"height"`
	Index      string            `json:"consensusIndex"`
	Code       uint32            `json:"code"`
	Successful bool              `json:"sdkSuccessful"`
	GasWanted  string            `json:"gasWanted"`
	GasUsed    string            `json:"gasUsed"`
	Synthetic  bool              `json:"synthetic"`
	RealAssets bool              `json:"realAssets"`
	Kind       string            `json:"kind"`
	Operations []json.RawMessage `json:"operations"`
	Fee        string            `json:"fee"`
	BlockHash  string            `json:"blockHash"`
}

func integer(s string, max *big.Int, positive bool) (*big.Int, error) {
	return godnative.Integer(s, max, positive)
}
func validCommit(c commit, now time.Time) bool {
	_, err := integer(c.Height, maxInt, true)
	t, e := time.Parse(time.RFC3339Nano, c.Time)
	return err == nil && e == nil && t.UTC().Format(time.RFC3339Nano) == c.Time && !t.Before(now.Add(-30*time.Second)) && !t.After(now.Add(5*time.Second)) && lowerHash(c.AppHash, false) && c.Synthetic && !c.RealAssets
}
func (g *Gateway) validNetwork(n network) bool {
	if n.Chain != g.config.ChainID || n.Compatible != g.config.CompatibleChainID || n.Denom != "agod" || n.Decimals != 18 || n.Total != fixedGod.String() || !validCommit(n.Commit, g.now()) {
		return false
	}
	pooled := new(big.Int)
	for _, s := range []string{n.Pool, n.PendingPool} {
		amount, err := integer(s, fixedGod, false)
		if err != nil {
			return false
		}
		pooled.Add(pooled, amount)
	}
	if pooled.Cmp(fixedGod) > 0 {
		return false
	}
	for _, s := range []string{n.Outstanding, n.PendingG} {
		if _, err := integer(s, maxG, false); err != nil {
			return false
		}
	}
	return true
}

// Constant-work consistency check for only the already requested ordinary
// accounts. These disjoint buckets cannot exceed the same committed network
// totals. G locks are deliberately excluded, not counted as spendable G. This
// neither scans the ledger nor authenticates the upstream's state claims.
func validBalances(n network, accounts ...account) bool {
	pool, err := integer(n.Pool, fixedGod, false)
	if err != nil {
		return false
	}
	pendingPool, err := integer(n.PendingPool, fixedGod, false)
	if err != nil {
		return false
	}
	outstanding, err := integer(n.Outstanding, maxG, false)
	if err != nil {
		return false
	}
	pendingG, err := integer(n.PendingG, maxG, false)
	if err != nil {
		return false
	}
	if len(accounts) == 0 || len(accounts) > 2 {
		return false
	}
	god := new(big.Int).Add(pool, pendingPool)
	settled, earned := new(big.Int), new(big.Int)
	seen := make(map[string]bool, len(accounts))
	for _, a := range accounts {
		if a.Commit != n.Commit || a.Module || seen[a.Native] {
			return false
		}
		seen[a.Native] = true
		for _, field := range []struct {
			text  string
			max   *big.Int
			total *big.Int
		}{{a.Balance, fixedGod, god}, {a.Spendable, maxG, settled}, {a.Unclaimed, maxG, settled}, {a.Pending, maxG, earned}} {
			amount, err := integer(field.text, field.max, false)
			if err != nil {
				return false
			}
			// big.Int intermediates cannot wrap when two uint256 buckets add.
			field.total.Add(field.total, amount)
		}
	}
	return god.Cmp(fixedGod) <= 0 && settled.Cmp(outstanding) <= 0 && earned.Cmp(pendingG) <= 0
}
func lowerHash(s string, prefix bool) bool {
	if prefix {
		if !strings.HasPrefix(s, "0x") {
			return false
		}
		s = s[2:]
	}
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func uint64Text(s string) (uint64, error) {
	if _, err := integer(s, maxUint, false); err != nil {
		return 0, ErrGateway
	}
	n, err := strconv.ParseUint(s, 10, 64)
	return n, err
}
