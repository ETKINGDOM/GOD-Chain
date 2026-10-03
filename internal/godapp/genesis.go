package godapp

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"unicode/utf8"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const maxGenesisBytes = 1 << 20
const maxGenesisParticipants = 4096

// Genesis is exclusively for a local native-application prototype. It cannot
// assert RH backing. No mainnet genesis, export command or populated file is
// supplied. Any unassigned synthetic GOD stays in the separate reserve.
type Genesis struct {
	Version   uint32           `json:"version"`
	Prototype bool             `json:"prototype"`
	Balances  []GenesisBalance `json:"balances"`
}

type GenesisBalance struct {
	Address string `json:"address"`
	Amount  string `json:"godSmallestUnits"`
}

type initialBalance struct {
	address sdk.AccAddress
	amount  sdkmath.Int
}

func decodeGenesis(raw []byte) ([]initialBalance, error) {
	if len(raw) == 0 || len(raw) > maxGenesisBytes || !utf8.Valid(raw) || uniqueJSONFields(raw) != nil {
		return nil, ErrGenesis
	}
	var genesis Genesis
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&genesis); err != nil {
		return nil, ErrGenesis
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, ErrGenesis
	}
	if genesis.Version != 1 || !genesis.Prototype || len(genesis.Balances) > maxGenesisParticipants {
		return nil, ErrGenesis
	}
	seen := make(map[string]bool, len(genesis.Balances))
	balances := make([]initialBalance, 0, len(genesis.Balances))
	total := sdkmath.ZeroInt()
	for _, balance := range genesis.Balances {
		address, err := msg.Account(balance.Address)
		if err != nil || seen[string(address)] || protectedAddress(address) {
			return nil, ErrGenesis
		}
		// Bound the text before integer parsing; total cannot exceed fixed supply.
		if len(balance.Amount) == 0 || len(balance.Amount) > 28 || balance.Amount[0] == '0' {
			return nil, ErrGenesis
		}
		for _, character := range balance.Amount {
			if character < '0' || character > '9' {
				return nil, ErrGenesis
			}
		}
		amount, ok := sdkmath.NewIntFromString(balance.Amount)
		if !ok || !amount.IsPositive() || amount.GT(godrewards.FixedGodSupply()) {
			return nil, ErrGenesis
		}
		total = total.Add(amount)
		if total.GT(godrewards.FixedGodSupply()) {
			return nil, ErrGenesis
		}
		seen[string(address)] = true
		balances = append(balances, initialBalance{address, amount})
	}
	// Account numbering and app hashes must not depend on input or map order.
	sort.Slice(balances, func(i, j int) bool { return bytes.Compare(balances[i].address, balances[j].address) < 0 })
	return balances, nil
}

// encoding/json accepts duplicate object keys. Reject them, excessive nesting
// and trailing values before decoding so ambiguous input cannot select a mode,
// address or amount by last-key-wins interpretation.
func uniqueJSONFields(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 8 {
			return ErrGenesis
		}
		token, err := decoder.Token()
		if err != nil {
			return ErrGenesis
		}
		delimiter, nested := token.(json.Delim)
		if !nested {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				text, ok := key.(string)
				allowed := depth == 0 && (text == "version" || text == "prototype" || text == "balances") ||
					depth == 2 && (text == "address" || text == "godSmallestUnits")
				if err != nil || !ok || seen[text] || !allowed {
					return ErrGenesis
				}
				seen[text] = true
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := value(depth + 1); err != nil {
					return err
				}
			}
		default:
			return ErrGenesis
		}
		_, err = decoder.Token()
		if err != nil {
			return ErrGenesis
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return ErrGenesis
	}
	return nil
}

func moduleNames() []string {
	return []string{authtypes.FeeCollectorName, godrewards.ReserveModule, godrewards.PendingModule, godrewards.PoolModule, authorityModule}
}

func protectedAddress(address sdk.AccAddress) bool {
	for _, name := range moduleNames() {
		if address.Equals(authtypes.NewModuleAddress(name)) {
			return true
		}
	}
	return false
}

func (a *App) initialize(ctx sdk.Context, balances []initialBalance) error {
	cache, write := ctx.CacheContext()
	a.accounts.InitGenesis(cache, *authtypes.DefaultGenesisState())
	names := moduleNames()
	sort.Strings(names)
	for _, name := range names {
		a.accounts.GetModuleAccount(cache, name)
	}
	bankGenesis := banktypes.DefaultGenesisState()
	bankGenesis.Supply = sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, godrewards.FixedGodSupply()))
	total := sdkmath.ZeroInt()
	for _, balance := range balances {
		a.accounts.SetAccount(cache, a.accounts.NewAccountWithAddress(cache, balance.address))
		text, err := (godaddress.Codec{}).BytesToString(balance.address)
		if err != nil {
			return ErrGenesis
		}
		bankGenesis.Balances = append(bankGenesis.Balances, banktypes.Balance{Address: text, Coins: sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, balance.amount))})
		total = total.Add(balance.amount)
	}
	reserve := godrewards.FixedGodSupply().Sub(total)
	if reserve.IsPositive() {
		text, err := (godaddress.Codec{}).BytesToString(a.accounts.GetModuleAddress(godrewards.ReserveModule))
		if err != nil {
			return ErrGenesis
		}
		bankGenesis.Balances = append(bankGenesis.Balances, banktypes.Balance{Address: text, Coins: sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, reserve))})
	}
	if err := bankGenesis.Validate(); err != nil {
		return ErrGenesis
	}
	a.bank.InitGenesis(cache, bankGenesis)
	if err := a.rewards.Init(cache); err != nil {
		return err
	}
	metadata := metadata{Binding: a.binding, Height: 0, Time: ctx.BlockTime().UTC()}
	if err := a.putMetadata(cache, metadata); err != nil {
		return err
	}
	write()
	return nil
}
