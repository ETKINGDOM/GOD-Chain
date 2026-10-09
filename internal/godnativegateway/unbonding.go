package godnativegateway

import (
	"context"
	"time"
)

type unbondingEntry struct {
	ID      string `json:"id"`
	Height  string `json:"creationHeight"`
	Time    string `json:"completionTime"`
	Initial string `json:"initialBalanceGod"`
	Balance string `json:"balanceGod"`
	Hold    string `json:"onHoldRefCount"`
}
type unbonding struct {
	Commit    commit           `json:"commit"`
	Owner     string           `json:"delegator"`
	Validator string           `json:"validator"`
	Delay     string           `json:"unbondingSeconds"`
	Max       uint32           `json:"maxEntries"`
	Entries   []unbondingEntry `json:"entries"`
}
type unbondingSnapshot struct {
	Network network   `json:"network"`
	View    unbonding `json:"unbonding"`
}

// A bounded keyless lookup, with the same identity/freshness policy as state.
// This does not consume or clear a signed operation's attempt reservation.
func (g *Gateway) unbondingSnapshot(ctx context.Context, owner, validator string) (unbondingSnapshot, error) {
	var out unbondingSnapshot
	n, err := g.network(ctx, "")
	if err != nil {
		return out, ErrGateway
	}
	if _, err := g.ordinary(ctx, owner, n, false); err != nil {
		return out, ErrGateway
	}
	raw, err := g.call(ctx, "god_unbonding", []any{owner, validator, n.Commit.Height})
	var u unbonding
	if err != nil || strict(raw, &u) != nil || u.Commit != n.Commit || u.Owner != owner || u.Validator != validator || u.Delay != "1814400" || u.Max != 7 || u.Entries == nil || len(u.Entries) > 7 {
		return out, ErrGateway
	}
	ids := map[string]bool{}
	for _, e := range u.Entries {
		id, a := integer(e.ID, maxUint, true)
		h, b := integer(e.Height, maxInt, true)
		initial, c := integer(e.Initial, fixedGod, true)
		balance, d := integer(e.Balance, fixedGod, false)
		_, hold := integer(e.Hold, maxInt, false)
		when, t := time.Parse(time.RFC3339Nano, e.Time)
		if a != nil || b != nil || c != nil || d != nil || hold != nil || t != nil || when.IsZero() || when.UTC().Format(time.RFC3339Nano) != e.Time || ids[e.ID] || h.String() != e.Height || id.String() != e.ID || balance.Cmp(initial) > 0 {
			return out, ErrGateway
		}
		tip, _ := integer(n.Commit.Height, maxInt, true)
		if h.Cmp(tip) > 0 {
			return out, ErrGateway
		}
		ids[e.ID] = true
	}
	end, err := g.network(ctx, n.Commit.Height)
	if err != nil || end != n {
		return out, ErrGateway
	}
	return unbondingSnapshot{n, u}, nil
}
