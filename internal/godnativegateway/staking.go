package godnativegateway

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godnative"
)

type stakingPosition struct {
	Delegation delegation `json:"delegation"`
	Unbonding  unbonding  `json:"unbonding"`
}
type stakingSnapshot struct {
	Network   network           `json:"network"`
	Account   account           `json:"account"`
	Page      validatorsPage    `json:"page"`
	Positions []stakingPosition `json:"positions"`
}
type atomicStakingSnapshot struct {
	Network         network           `json:"network"`
	Account         account           `json:"account"`
	Page            validatorsPage    `json:"page"`
	Positions       []stakingPosition `json:"positions"`
	ContractAccount bool              `json:"contractAccount"`
}

// One atomic loopback read. No transaction state, retries, cached balances,
// historical/latest fallback, source signing, admission or payment authority.
func (g *Gateway) readStaking(ctx context.Context, owner string, operators []string) (stakingSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	raw, err := g.call(ctx, "god_stakingSnapshot", []any{owner, operators})
	var v atomicStakingSnapshot
	if err != nil || strict(raw, &v) != nil || v.ContractAccount || !g.validNetwork(v.Network) || !validDisplayAccount(owner, v.Network, v.Account) || !validDisplayPage(v.Network, v.Page) || len(v.Positions) != len(operators) {
		return stakingSnapshot{}, ErrGateway
	}
	for i, p := range v.Positions {
		if !validStakingDelegation(v.Network, owner, operators[i], p.Delegation) || !validUnbonding(v.Network, owner, operators[i], p.Unbonding) {
			return stakingSnapshot{}, ErrGateway
		}
	}
	return stakingSnapshot{v.Network, v.Account, v.Page, v.Positions}, nil
}

func validDisplayAccount(owner string, n network, a account) bool {
	raw, err := godnative.Account(owner)
	if err != nil {
		return false
	}
	evm, err := godaddress.ToEVM(raw)
	if err != nil || a.Commit != n.Commit || a.Native != owner || a.Compatible != evm || a.Module {
		return false
	}
	if _, err := uint64Text(a.Number); err != nil {
		return false
	}
	if _, err := uint64Text(a.Sequence); err != nil {
		return false
	}
	if _, err := integer(a.Balance, fixedGod, false); err != nil {
		return false
	}
	for _, value := range []string{a.Spendable, a.Unclaimed, a.Pending} {
		if _, err := integer(value, maxG, false); err != nil {
			return false
		}
	}
	return validBalances(n, a) && (a.Exists || a.Number == "0" && a.Sequence == "0" && a.Balance == "0")
}

func validDisplayPage(n network, p validatorsPage) bool {
	if p.Commit != n.Commit || p.After != "" || p.Limit != 8 || p.Validators == nil || len(p.Validators) > 8 {
		return false
	}
	var previous []byte
	for _, v := range p.Validators {
		b := operatorBytes(v.Operator)
		if !validValidator(v) || !v.Exists || v.Commit != n.Commit || bytes.Compare(previous, b) >= 0 {
			return false
		}
		previous = b
	}
	return p.NextAfter == "" || len(p.Validators) == 8 && p.NextAfter == p.Validators[len(p.Validators)-1].Operator
}

func validStakingDelegation(n network, owner, operator string, d delegation) bool {
	if d.Commit != n.Commit || d.Owner != owner || d.Validator != operator {
		return false
	}
	value, e := integer(d.God, fixedGod, false)
	if e != nil {
		return false
	}
	var whole, fraction string
	for i, c := range d.Shares {
		if c == '.' {
			whole = d.Shares[:i]
			fraction = d.Shares[i+1:]
			break
		}
	}
	if whole == "" {
		whole = d.Shares
	}
	if _, e = integer(whole, maxG, false); e != nil {
		return false
	}
	if len(fraction) != 0 && len(fraction) != 18 {
		return false
	}
	nonzero := whole != "0"
	for _, c := range fraction {
		if c < '0' || c > '9' {
			return false
		}
		nonzero = nonzero || c != '0'
	}
	if len(d.Shares) > len(whole) && len(fraction) != 18 {
		return false
	}
	return d.Exists == nonzero && (d.Exists || value.Sign() == 0)
}

func stakingJSON(view stakingSnapshot) ([]byte, error) {
	raw, err := json.Marshal(view)
	if err != nil || len(raw) > 32768 {
		return nil, ErrGateway
	}
	return raw, nil
}
