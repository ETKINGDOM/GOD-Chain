package godnativegateway

import (
	"bytes"
	"context"
	"strings"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type validator struct {
	Commit     commit `json:"commit"`
	Operator   string `json:"validator"`
	Exists     bool   `json:"exists"`
	Status     string `json:"status"`
	Jailed     bool   `json:"jailed"`
	Tombstoned bool   `json:"tombstoned"`
	Tokens     string `json:"tokensGodSmallestUnits"`
	Shares     string `json:"delegatorShares"`
	Commission string `json:"commissionRate"`
	Minimum    string `json:"minSelfDelegationGodSmallestUnits"`
}
type validatorSnapshot struct {
	Network network   `json:"network"`
	View    validator `json:"validator"`
}

func validOperator(s string) bool {
	const length = len(godaddress.ValidatorOperatorPrefix) + 1 + godaddress.AccountBytes*8/5 + 6
	if len(s) != length {
		return false
	}
	raw, err := sdk.GetFromBech32(s, godaddress.ValidatorOperatorPrefix)
	if err != nil || len(raw) != godaddress.AccountBytes {
		return false
	}
	canonical, err := sdk.Bech32ifyAddressBytes(godaddress.ValidatorOperatorPrefix, raw)
	nonzero := false
	for _, b := range raw {
		nonzero = nonzero || b != 0
	}
	return err == nil && canonical == s && nonzero
}

// One keyless, height-pinned staking record. No enumeration, signing history,
// arbitrary descriptions or submissions. Bonded status is not an uptime proof.
func (g *Gateway) validatorSnapshot(ctx context.Context, operator string) (validatorSnapshot, error) {
	var out validatorSnapshot
	n, err := g.network(ctx, "")
	if err != nil {
		return out, ErrGateway
	}
	raw, err := g.call(ctx, "god_validator", []any{operator, n.Commit.Height})
	var v validator
	if err != nil || strict(raw, &v) != nil || v.Commit != n.Commit || v.Operator != operator {
		return out, ErrGateway
	}
	if !validValidator(v) {
		return out, ErrGateway
	}
	end, err := g.network(ctx, n.Commit.Height)
	if err != nil || end != n {
		return out, ErrGateway
	}
	return validatorSnapshot{n, v}, nil
}

func validValidator(v validator) bool {
	if !validOperator(v.Operator) {
		return false
	}
	if !v.Exists {
		if v.Status != "absent" || v.Jailed || v.Tombstoned || v.Tokens != "0" || v.Shares != "0" || v.Commission != "0" || v.Minimum != "0" {
			return false
		}
	} else {
		if v.Status != "bonded" && v.Status != "unbonding" && v.Status != "unbonded" || v.Commission != "0.100000000000000000" || v.Minimum != "1000000000000000000000" {
			return false
		}
		if _, err := integer(v.Tokens, fixedGod, false); err != nil {
			return false
		}
		parts := strings.Split(v.Shares, ".")
		if len(parts) != 2 || len(parts[1]) != 18 {
			return false
		}
		if _, err := integer(parts[0], maxG, false); err != nil {
			return false
		}
		for _, c := range parts[1] {
			if c < '0' || c > '9' {
				return false
			}
		}
		scaled := strings.TrimLeft(parts[0]+parts[1], "0")
		if scaled == "" {
			scaled = "0"
		}
		if _, err := integer(scaled, maxG, false); err != nil {
			return false
		}
	}
	return true
}

type validatorsPage struct {
	Commit     commit      `json:"commit"`
	After      string      `json:"after"`
	Limit      int         `json:"limit"`
	NextAfter  string      `json:"nextAfter"`
	Validators []validator `json:"validators"`
}
type validatorsSnapshot struct {
	Network network        `json:"network"`
	Page    validatorsPage `json:"page"`
}

func operatorBytes(s string) []byte {
	raw, _ := sdk.GetFromBech32(s, godaddress.ValidatorOperatorPrefix)
	return raw
}

// Fixed-size current registration page; never scans accounts, submissions or
// arbitrary metadata. Both the cursor and every record bind to one commit.
func (g *Gateway) validatorsSnapshot(ctx context.Context, after, height string) (validatorsSnapshot, error) {
	var out validatorsSnapshot
	n, err := g.network(ctx, height)
	if err != nil {
		return out, ErrGateway
	}
	raw, err := g.call(ctx, "god_validators", []any{after, n.Commit.Height})
	var p validatorsPage
	if err != nil || strict(raw, &p) != nil || p.Commit != n.Commit || p.After != after || p.Limit != 8 || p.Validators == nil || len(p.Validators) > 8 {
		return out, ErrGateway
	}
	previous := operatorBytes(after)
	for _, v := range p.Validators {
		if !validValidator(v) || !v.Exists || v.Commit != n.Commit {
			return out, ErrGateway
		}
		next := operatorBytes(v.Operator)
		if bytes.Compare(previous, next) >= 0 {
			return out, ErrGateway
		}
		previous = next
	}
	if p.NextAfter != "" && (len(p.Validators) != 8 || p.NextAfter != p.Validators[len(p.Validators)-1].Operator) {
		return out, ErrGateway
	}
	end, err := g.network(ctx, n.Commit.Height)
	if err != nil || end != n {
		return out, ErrGateway
	}
	return validatorsSnapshot{n, p}, nil
}
