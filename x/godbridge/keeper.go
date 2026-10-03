package godbridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"math"
	"math/big"
	"time"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

type Accounts interface {
	GetModuleAddress(string) sdk.AccAddress
	GetAccount(context.Context, sdk.AccAddress) sdk.AccountI
}

type Keeper struct {
	key      *storetypes.KVStoreKey
	bank     godrewards.Bank
	accounts Accounts
	config   Config
	binding  []byte
}

var stateKey = []byte{1}
var busyKey = []byte{2}

const depositPrefix byte = 20
const eventPrefix byte = 21
const withdrawalPrefix byte = 22

type windowEntry struct {
	At     time.Time
	Amount sdkmath.Int
}
type state struct {
	Binding       []byte
	Released      sdkmath.Int
	Pending       sdkmath.Int
	Deposited     sdkmath.Int
	Paid          sdkmath.Int
	Next          uint64
	Head          uint64
	Open          uint32
	LastHeight    int64
	LastTime      time.Time
	ControlNonce  uint64
	IntakePaused  bool
	OutflowPaused bool
	Window        []windowEntry
}

type Snapshot struct {
	ReleasedGod     sdkmath.Int `json:"nativeGodOutsideReserveSmallestUnits"`
	PendingGod      sdkmath.Int `json:"pendingWithdrawalsSmallestUnits"`
	ReserveGod      sdkmath.Int `json:"restrictedReserveSmallestUnits"`
	TotalDeposited  sdkmath.Int `json:"attestedDepositsSmallestUnits"`
	TotalPaid       sdkmath.Int `json:"attestedPaymentsSmallestUnits"`
	RollingOutflow  sdkmath.Int `json:"reservedRollingOutflowSmallestUnits"`
	OpenWithdrawals uint32      `json:"openWithdrawals"`
	HeadSequence    uint64      `json:"headSequence"`
	NextSequence    uint64      `json:"nextSequence"`
	IntakePaused    bool        `json:"intakePaused"`
	OutflowPaused   bool        `json:"outflowPaused"`
}

func NewKeeper(key *storetypes.KVStoreKey, bank godrewards.Bank, accounts Accounts, config Config) (Keeper, error) {
	if key == nil || bank == nil || accounts == nil {
		return Keeper{}, ErrConfig
	}
	config, err := config.canonical()
	if err != nil {
		return Keeper{}, err
	}
	seen := map[string]bool{}
	for _, name := range []string{PendingModule, godrewards.ReserveModule, godrewards.PendingModule, godrewards.PoolModule, authtypes.FeeCollectorName} {
		address := accounts.GetModuleAddress(name)
		if len(address) != 20 || seen[string(address)] {
			return Keeper{}, ErrConfig
		}
		seen[string(address)] = true
	}
	binding, err := json.Marshal(config)
	if err != nil {
		return Keeper{}, ErrConfig
	}
	return Keeper{key, bank, accounts, config, binding}, nil
}

func (k Keeper) store(ctx sdk.Context) storetypes.KVStore { return ctx.KVStore(k.key) }
func numbered(prefix byte, sequence uint64) []byte {
	b := make([]byte, 9)
	b[0] = prefix
	binary.BigEndian.PutUint64(b[1:], sequence)
	return b
}

// Replay identity deliberately excludes the block hash: re-inclusion of the
// same transaction/log after a reorganization cannot create a second deposit.
// One shared namespace also prevents deposit/payment/cancellation event reuse.
func (k Keeper) eventKey(e Evidence) []byte {
	p := k.domain("source-event")
	p.Write(e.TransactionHash[:])
	p.u32(e.LogIndex)
	id := sha256.Sum256(p.Bytes())
	return append([]byte{eventPrefix}, id[:]...)
}

func nonnegative(n sdkmath.Int) bool { return !n.IsNil() && !n.IsNegative() }
func add(a, b sdkmath.Int) (sdkmath.Int, error) {
	n := new(big.Int).Add(a.BigInt(), b.BigInt())
	if n.Sign() < 0 || n.BitLen() > 256 {
		return sdkmath.Int{}, ErrState
	}
	return sdkmath.NewIntFromBigInt(n), nil
}

func (k Keeper) getState(ctx sdk.Context) (state, error) {
	raw := k.store(ctx).Get(stateKey)
	if raw == nil {
		return state{}, ErrNotInitialized
	}
	var s state
	if len(raw) > 256<<10 || json.Unmarshal(raw, &s) != nil || !bytes.Equal(s.Binding, k.binding) ||
		!nonnegative(s.Released) || !nonnegative(s.Pending) || !nonnegative(s.Deposited) || !nonnegative(s.Paid) ||
		s.Released.GT(ExposureLimit()) || s.Pending.GT(s.Released) || s.Paid.GT(s.Deposited) ||
		!s.Deposited.Sub(s.Paid).Equal(s.Released) || s.Next == 0 || s.Head == 0 || s.Head > s.Next || s.Next-s.Head > MaxOpenWithdrawals ||
		s.Open > MaxOpenWithdrawals || len(s.Window) > MaxWindowEntries || s.LastHeight < 0 || !validTime(s.LastTime) {
		return state{}, ErrState
	}
	previous := time.Time{}
	total := sdkmath.ZeroInt()
	for _, entry := range s.Window {
		if !validTime(entry.At) || entry.At.Before(previous) || entry.At.After(s.LastTime) || !amountValid(entry.Amount) {
			return state{}, ErrState
		}
		previous = entry.At
		total = total.Add(entry.Amount)
	}
	if total.GT(OutflowLimit()) {
		return state{}, ErrState
	}
	return s, nil
}

func (k Keeper) checkBank(ctx sdk.Context, s state) error {
	if !k.bank.GetSupply(ctx, godrewards.GodDenom).Amount.Equal(godrewards.FixedGodSupply()) ||
		!k.bank.GetBalance(ctx, k.accounts.GetModuleAddress(godrewards.ReserveModule), godrewards.GodDenom).Amount.Equal(godrewards.FixedGodSupply().Sub(s.Released)) ||
		!k.bank.GetBalance(ctx, k.accounts.GetModuleAddress(PendingModule), godrewards.GodDenom).Amount.Equal(s.Pending) {
		return ErrState
	}
	return nil
}

func (k Keeper) participant(ctx sdk.Context, address sdk.AccAddress) bool {
	if len(address) != 20 || bytes.Equal(address, make([]byte, 20)) {
		return false
	}
	for _, name := range []string{PendingModule, godrewards.ReserveModule, godrewards.PendingModule, godrewards.PoolModule, authtypes.FeeCollectorName} {
		if address.Equals(k.accounts.GetModuleAddress(name)) {
			return false
		}
	}
	_, module := k.accounts.GetAccount(ctx, address).(sdk.ModuleAccountI)
	return !module
}

// Init permits no synthetic participant allocation or assumed backing. The
// entire fixed native supply must still be restricted. Backed genesis needs a
// separately reviewed reconciliation format; this method does not invent one.
func (k Keeper) Init(ctx sdk.Context) error {
	if k.store(ctx).Has(stateKey) {
		return ErrInitialized
	}
	iterator := k.store(ctx).Iterator(nil, nil)
	nonempty := iterator.Valid()
	iterator.Close()
	if nonempty || ctx.BlockHeight() < 0 || !validTime(ctx.BlockTime()) {
		return ErrState
	}
	s := state{Binding: k.binding, Released: sdkmath.ZeroInt(), Pending: sdkmath.ZeroInt(), Deposited: sdkmath.ZeroInt(), Paid: sdkmath.ZeroInt(), Next: 1, Head: 1, LastHeight: ctx.BlockHeight(), LastTime: ctx.BlockTime().UTC()}
	if err := k.checkBank(ctx, s); err != nil {
		return err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return ErrState
	}
	k.store(ctx).Set(stateKey, raw)
	return nil
}

func (k Keeper) change(ctx sdk.Context, transition func(sdk.Context, *state) error) error {
	if k.store(ctx).Has(busyKey) {
		return ErrReentrant
	}
	s, err := k.getState(ctx)
	if err != nil {
		return err
	}
	if ctx.BlockHeight() < s.LastHeight || !validTime(ctx.BlockTime()) || ctx.BlockTime().Before(s.LastTime) {
		return ErrState
	}
	if err := k.checkBank(ctx, s); err != nil {
		return err
	}
	cache, write := ctx.CacheContext()
	k.store(cache).Set(busyKey, []byte{1})
	s.LastHeight, s.LastTime = ctx.BlockHeight(), ctx.BlockTime().UTC()
	if err := transition(cache, &s); err != nil {
		return err
	}
	if err := k.checkBank(cache, s); err != nil {
		return err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return ErrState
	}
	k.store(cache).Set(stateKey, raw)
	k.store(cache).Delete(busyKey)
	if _, err := k.getState(cache); err != nil {
		return err
	}
	write()
	return nil
}

// AcceptDeposit trusts only quorum attestations over the received escrow
// amount and finalized source event. Signers must independently check RH's
// source evidence; this code cannot establish that their statement is true.
func (k Keeper) AcceptDeposit(ctx sdk.Context, d Deposit, signatures [][]byte) error {
	digest, err := k.DepositDigest(d)
	if err != nil {
		return err
	}
	return k.change(ctx, func(cache sdk.Context, s *state) error {
		if s.IntakePaused {
			return ErrPaused
		}
		if !k.participant(cache, d.Recipient[:]) {
			return ErrRequest
		}
		if k.store(cache).Has(numbered(depositPrefix, d.Sequence)) || k.store(cache).Has(k.eventKey(d.Evidence)) {
			return ErrReplay
		}
		if s.Released.Add(d.Amount).GT(ExposureLimit()) {
			return ErrLimit
		}
		if err := k.verify(digest, signatures); err != nil {
			return err
		}
		deposited, err := add(s.Deposited, d.Amount)
		if err != nil {
			return err
		}
		if err := k.bank.SendCoinsFromModuleToAccount(cache, godrewards.ReserveModule, d.Recipient[:], sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, d.Amount))); err != nil {
			return err
		}
		s.Released = s.Released.Add(d.Amount)
		s.Deposited = deposited
		raw, err := json.Marshal(d)
		if err != nil {
			return ErrState
		}
		k.store(cache).Set(numbered(depositPrefix, d.Sequence), raw)
		k.store(cache).Set(k.eventKey(d.Evidence), digest[:])
		return nil
	})
}

// RequestWithdrawal is a trusted internal API. A future transaction router
// MUST authenticate sender; supplying an address here is not authorization.
func (k Keeper) RequestWithdrawal(ctx sdk.Context, sender sdk.AccAddress, recipient [20]byte, amount sdkmath.Int) (Withdrawal, error) {
	if !amountValid(amount) || recipient == [20]byte{} {
		return Withdrawal{}, ErrRequest
	}
	var created Withdrawal
	err := k.change(ctx, func(cache sdk.Context, s *state) error {
		if s.OutflowPaused {
			return ErrPaused
		}
		if !k.participant(cache, sender) {
			return ErrRequest
		}
		if s.Open >= MaxOpenWithdrawals || s.Next-s.Head >= MaxOpenWithdrawals || s.Next == math.MaxUint64 {
			return ErrLimit
		}
		w := Withdrawal{Sequence: s.Next, Sender: [20]byte(sender), Recipient: recipient, Amount: amount, QueuedAt: cache.BlockTime().UTC(), Status: Queued}
		id, err := k.withdrawalID(w)
		if err != nil {
			return err
		}
		w.ID = id
		if err := k.bank.SendCoinsFromAccountToModule(cache, sender, PendingModule, sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, amount))); err != nil {
			return err
		}
		s.Pending = s.Pending.Add(amount)
		s.Open++
		s.Next++
		if err := k.putWithdrawal(cache, w); err != nil {
			return err
		}
		created = w
		return nil
	})
	if err != nil {
		return Withdrawal{}, err
	}
	return created, nil
}

func (k Keeper) Withdrawal(ctx sdk.Context, sequence uint64) (Withdrawal, error) {
	if _, err := k.getState(ctx); err != nil {
		return Withdrawal{}, err
	}
	return k.withdrawal(ctx, sequence)
}

func (k Keeper) withdrawal(ctx sdk.Context, sequence uint64) (Withdrawal, error) {
	var w Withdrawal
	raw := k.store(ctx).Get(numbered(withdrawalPrefix, sequence))
	if len(raw) == 0 || len(raw) > 4096 || json.Unmarshal(raw, &w) != nil || w.Sequence != sequence {
		return Withdrawal{}, ErrRequest
	}
	id, err := k.withdrawalID(w)
	if err != nil || id != w.ID {
		return Withdrawal{}, ErrState
	}
	switch w.Status {
	case Queued:
		if !w.AuthorizedAt.IsZero() || !w.ResolvedAt.IsZero() || w.ResolutionEvidence != (Evidence{}) {
			return Withdrawal{}, ErrState
		}
	case Authorized:
		if !validTime(w.AuthorizedAt) || w.AuthorizedAt.Before(w.QueuedAt.Add(WithdrawalDelay)) || !w.ResolvedAt.IsZero() || w.ResolutionEvidence != (Evidence{}) {
			return Withdrawal{}, ErrState
		}
	case Paid, Cancelled:
		if !validTime(w.ResolvedAt) || w.ResolvedAt.Before(w.QueuedAt) || !w.ResolutionEvidence.valid() {
			return Withdrawal{}, ErrState
		}
		if w.Status == Paid && w.AuthorizedAt.IsZero() || !w.AuthorizedAt.IsZero() &&
			(!validTime(w.AuthorizedAt) || w.AuthorizedAt.Before(w.QueuedAt.Add(WithdrawalDelay)) || w.ResolvedAt.Before(w.AuthorizedAt)) {
			return Withdrawal{}, ErrState
		}
	default:
		return Withdrawal{}, ErrState
	}
	return w, nil
}

func (k Keeper) putWithdrawal(ctx sdk.Context, w Withdrawal) error {
	raw, err := json.Marshal(w)
	if err != nil {
		return ErrState
	}
	k.store(ctx).Set(numbered(withdrawalPrefix, w.Sequence), raw)
	return nil
}

func activeWindow(entries []windowEntry, now time.Time) ([]windowEntry, sdkmath.Int) {
	remaining := make([]windowEntry, 0, len(entries))
	total := sdkmath.ZeroInt()
	for _, entry := range entries {
		if entry.At.Add(OutflowWindow).After(now) {
			remaining = append(remaining, entry)
			total = total.Add(entry.Amount)
		}
	}
	return remaining, total
}

// AuthorizeNext reserves capacity once, never on payment acknowledgement. Only
// one unresolved head can be authorized. Cancelled capacity is not refunded.
func (k Keeper) AuthorizeNext(ctx sdk.Context, sequence uint64, signatures [][]byte) error {
	return k.change(ctx, func(cache sdk.Context, s *state) error {
		if s.OutflowPaused {
			return ErrPaused
		}
		if sequence != s.Head {
			return ErrQueue
		}
		w, err := k.withdrawal(cache, sequence)
		if err != nil {
			return err
		}
		if w.Status != Queued {
			return ErrTerminal
		}
		if cache.BlockTime().Before(w.QueuedAt.Add(WithdrawalDelay)) {
			return ErrDelay
		}
		digest, err := k.AuthorizationDigest(w)
		if err != nil {
			return err
		}
		if err := k.verify(digest, signatures); err != nil {
			return err
		}
		entries, used := activeWindow(s.Window, cache.BlockTime())
		if len(entries) >= MaxWindowEntries || used.Add(w.Amount).GT(OutflowLimit()) {
			return ErrLimit
		}
		s.Window = append(entries, windowEntry{cache.BlockTime().UTC(), w.Amount})
		w.Status, w.AuthorizedAt = Authorized, cache.BlockTime().UTC()
		return k.putWithdrawal(cache, w)
	})
}

// ResolvePayment moves pending GOD back to reserve only after the quorum's
// finalized RH payment statement. Receipt finality and escrow behavior remain
// external trust and integration requirements, not a trusted RPC shortcut.
func (k Keeper) ResolvePayment(ctx sdk.Context, sequence uint64, evidence Evidence, signatures [][]byte) error {
	return k.resolve(ctx, sequence, Paid, evidence, signatures)
}

// CancelWithdrawal never refunds on elapsed time. The quorum must attest a
// finalized RH tombstone that permanently disables payment for this exact ID.
// Until RH enforces such tombstones this method cannot be safely exposed.
func (k Keeper) CancelWithdrawal(ctx sdk.Context, sequence uint64, evidence Evidence, signatures [][]byte) error {
	return k.resolve(ctx, sequence, Cancelled, evidence, signatures)
}

func (k Keeper) resolve(ctx sdk.Context, sequence uint64, status Status, evidence Evidence, signatures [][]byte) error {
	return k.change(ctx, func(cache sdk.Context, s *state) error {
		w, err := k.withdrawal(cache, sequence)
		if err != nil {
			return err
		}
		if w.Status == Paid || w.Status == Cancelled || status == Paid && w.Status != Authorized {
			return ErrTerminal
		}
		if status == Paid && sequence != s.Head {
			return ErrQueue
		}
		digest, err := k.ResolutionDigest(w, status, evidence)
		if err != nil {
			return err
		}
		if k.store(cache).Has(k.eventKey(evidence)) {
			return ErrReplay
		}
		if err := k.verify(digest, signatures); err != nil {
			return err
		}
		coins := sdk.NewCoins(sdk.NewCoin(godrewards.GodDenom, w.Amount))
		if status == Paid {
			paid, err := add(s.Paid, w.Amount)
			if err != nil {
				return err
			}
			if err := k.bank.SendCoinsFromModuleToModule(cache, PendingModule, godrewards.ReserveModule, coins); err != nil {
				return err
			}
			s.Released = s.Released.Sub(w.Amount)
			s.Paid = paid
		} else {
			if err := k.bank.SendCoinsFromModuleToAccount(cache, PendingModule, w.Sender[:], coins); err != nil {
				return err
			}
		}
		s.Pending = s.Pending.Sub(w.Amount)
		s.Open--
		w.Status, w.ResolvedAt, w.ResolutionEvidence = status, cache.BlockTime().UTC(), evidence
		if err := k.putWithdrawal(cache, w); err != nil {
			return err
		}
		k.store(cache).Set(k.eventKey(evidence), digest[:])
		// At most the bounded open queue plus resolved intervening entries are
		// visited. Queue span is bounded at admission below, not just open count.
		for s.Head < s.Next {
			head, err := k.withdrawal(cache, s.Head)
			if err != nil {
				return err
			}
			if head.Status == Queued || head.Status == Authorized {
				break
			}
			s.Head++
		}
		return nil
	})
}

// Pause is monotonic and asset-neutral. No resume, signer rotation, limit edit,
// administrative withdrawal or reserve-spending API exists in this prototype.
func (k Keeper) Pause(ctx sdk.Context, nonce uint64, intake, outflow bool, signatures [][]byte) error {
	digest, err := k.PauseDigest(nonce, intake, outflow)
	if err != nil {
		return err
	}
	return k.change(ctx, func(cache sdk.Context, s *state) error {
		if s.ControlNonce == math.MaxUint64 || nonce != s.ControlNonce+1 || s.IntakePaused && !intake || s.OutflowPaused && !outflow {
			return ErrRequest
		}
		if s.IntakePaused == intake && s.OutflowPaused == outflow {
			return ErrRequest
		}
		if err := k.verify(digest, signatures); err != nil {
			return err
		}
		s.ControlNonce, s.IntakePaused, s.OutflowPaused = nonce, intake, outflow
		return nil
	})
}

func (k Keeper) Snapshot(ctx sdk.Context) (Snapshot, error) {
	s, err := k.getState(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	if err := k.checkBank(ctx, s); err != nil {
		return Snapshot{}, err
	}
	if !validTime(ctx.BlockTime()) || ctx.BlockTime().Before(s.LastTime) || ctx.BlockHeight() < s.LastHeight {
		return Snapshot{}, ErrState
	}
	_, used := activeWindow(s.Window, ctx.BlockTime())
	return Snapshot{s.Released, s.Pending, godrewards.FixedGodSupply().Sub(s.Released), s.Deposited, s.Paid, used, s.Open, s.Head, s.Next, s.IntakePaused, s.OutflowPaused}, nil
}

// CheckInvariants is an exhaustive diagnostic, not a per-block hook. Durable
// transfer and event tombstones cannot be discarded to reclaim replay space.
// Their long-term growth needs benchmarks and a reviewed retention design.
func (k Keeper) CheckInvariants(ctx sdk.Context) error {
	s, err := k.getState(ctx)
	if err != nil {
		return err
	}
	if err := k.checkBank(ctx, s); err != nil {
		return err
	}
	deposited, paid, pending := sdkmath.ZeroInt(), sdkmath.ZeroInt(), sdkmath.ZeroInt()
	var open uint32
	var sequence uint64 = 1
	store := k.store(ctx)
	deposits := store.Iterator([]byte{depositPrefix}, []byte{depositPrefix + 1})
	defer deposits.Close()
	for ; deposits.Valid(); deposits.Next() {
		var d Deposit
		if len(deposits.Key()) != 9 || len(deposits.Value()) > 4096 || json.Unmarshal(deposits.Value(), &d) != nil || d.Sequence != binary.BigEndian.Uint64(deposits.Key()[1:]) {
			return ErrState
		}
		digest, err := k.DepositDigest(d)
		if err != nil || !bytes.Equal(store.Get(k.eventKey(d.Evidence)), digest[:]) {
			return ErrState
		}
		deposited, err = add(deposited, d.Amount)
		if err != nil {
			return err
		}
	}
	withdrawals := store.Iterator([]byte{withdrawalPrefix}, []byte{withdrawalPrefix + 1})
	defer withdrawals.Close()
	for ; withdrawals.Valid(); withdrawals.Next() {
		if len(withdrawals.Key()) != 9 || binary.BigEndian.Uint64(withdrawals.Key()[1:]) != sequence {
			return ErrState
		}
		w, err := k.withdrawal(ctx, sequence)
		if err != nil {
			return ErrState
		}
		if w.Status == Queued || w.Status == Authorized {
			if sequence < s.Head || w.Status == Authorized && sequence != s.Head {
				return ErrState
			}
			pending = pending.Add(w.Amount)
			open++
		} else {
			digest, err := k.ResolutionDigest(w, w.Status, w.ResolutionEvidence)
			if err != nil || !bytes.Equal(store.Get(k.eventKey(w.ResolutionEvidence)), digest[:]) {
				return ErrState
			}
			if w.Status == Paid {
				paid, err = add(paid, w.Amount)
				if err != nil {
					return err
				}
			}
		}
		sequence++
	}
	if sequence != s.Next || open != s.Open || !pending.Equal(s.Pending) || !deposited.Equal(s.Deposited) || !paid.Equal(s.Paid) {
		return ErrState
	}
	if s.Head < s.Next {
		head, err := k.withdrawal(ctx, s.Head)
		if err != nil || head.Status != Queued && head.Status != Authorized {
			return ErrState
		}
	} else if s.Open != 0 {
		return ErrState
	}
	return nil
}
