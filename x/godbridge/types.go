// Package godbridge implements an isolated native bridge ledger prototype.
// Its transaction adapter is tested in isolation, not mounted in a node or RH
// connection. Initialization APIs remain trusted internal calls. Attestations
// establish a 5-of-7 federated trust boundary, not cryptographic RH finality.
package godbridge

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/big"
	"regexp"
	"sort"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

const (
	StoreKey        = "godbridge"
	PendingModule   = "god_bridge_pending"
	Threshold       = 5
	SignerCount     = 7
	WithdrawalDelay = 24 * time.Hour
	OutflowWindow   = 24 * time.Hour
	// Prototype workload bounds, not approved mainnet admission parameters.
	MaxOpenWithdrawals = 1024
	MaxWindowEntries   = 1024
)

var (
	ErrConfig         = errors.New("invalid bridge configuration")
	ErrState          = errors.New("bridge state or backing accounting invariant violated")
	ErrNotInitialized = errors.New("bridge ledger is not initialized")
	ErrInitialized    = errors.New("bridge ledger is already initialized")
	ErrRequest        = errors.New("invalid bridge request")
	ErrApprovals      = errors.New("bridge requires distinct valid federated approvals")
	ErrReplay         = errors.New("bridge transfer or source event was already recorded")
	ErrLimit          = errors.New("bridge exposure, outflow or workload limit exceeded")
	ErrPaused         = errors.New("bridge direction is paused")
	ErrQueue          = errors.New("withdrawal is not the eligible queue head")
	ErrDelay          = errors.New("withdrawal public delay is incomplete")
	ErrTerminal       = errors.New("withdrawal cannot enter the requested state")
	ErrReentrant      = errors.New("bridge transition is already in progress")
)

func ExposureLimit() sdkmath.Int { return godrewards.Unit().MulRaw(10_000) }
func TransferLimit() sdkmath.Int { return godrewards.Unit().MulRaw(1_000) }
func OutflowLimit() sdkmath.Int  { return godrewards.Unit().MulRaw(1_000) }

// AssetID binds the reviewed source chain, token, escrow, decimals and protocol
// deployment. Its operational preimage and signer addresses are not supplied in
// source. No method changes this binding or rotates signers without migration.
type Config struct {
	Version     uint32
	SourceChain string
	NativeChain string
	AssetID     [32]byte
	Signers     [SignerCount][20]byte
}

var chainPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,50}$`)

func (c Config) canonical() (Config, error) {
	if c.Version != 1 || !chainPattern.MatchString(c.SourceChain) || !chainPattern.MatchString(c.NativeChain) ||
		c.SourceChain == c.NativeChain || c.AssetID == [32]byte{} {
		return Config{}, ErrConfig
	}
	sort.Slice(c.Signers[:], func(i, j int) bool { return bytes.Compare(c.Signers[i][:], c.Signers[j][:]) < 0 })
	for i, signer := range c.Signers {
		if signer == [20]byte{} || i > 0 && signer == c.Signers[i-1] {
			return Config{}, ErrConfig
		}
	}
	return c, nil
}

type Evidence struct {
	Height          uint64
	BlockHash       [32]byte
	TransactionHash [32]byte
	LogIndex        uint32
}

func (e Evidence) valid() bool {
	return e.Height > 0 && e.BlockHash != [32]byte{} && e.TransactionHash != [32]byte{}
}

type Deposit struct {
	Sequence  uint64
	Recipient [20]byte
	Amount    sdkmath.Int
	Evidence  Evidence
}

type Status string

const (
	Queued     Status = "queued"
	Authorized Status = "authorized"
	Paid       Status = "paid"
	Cancelled  Status = "cancelled"
)

type Withdrawal struct {
	ID                 [32]byte
	Sequence           uint64
	Sender             [20]byte
	Recipient          [20]byte
	Amount             sdkmath.Int
	QueuedAt           time.Time
	Status             Status
	AuthorizedAt       time.Time
	ResolvedAt         time.Time
	ResolutionEvidence Evidence
}

func amountValid(n sdkmath.Int) bool { return !n.IsNil() && n.IsPositive() && n.LTE(TransferLimit()) }
func validTime(t time.Time) bool     { return t.Unix() > 0 && t.UTC().Year() <= 9999 }

// Encoding is fixed-width except chain names, which have uint16 byte lengths.
// SHA-256 is used directly, without an Ethereum personal-sign prefix. Both
// custody implementations must use precisely the same reviewed encoding.
type packet struct{ bytes.Buffer }

func (p *packet) u64(n uint64) { var b [8]byte; binary.BigEndian.PutUint64(b[:], n); p.Write(b[:]) }
func (p *packet) u32(n uint32) { var b [4]byte; binary.BigEndian.PutUint32(b[:], n); p.Write(b[:]) }
func (p *packet) text(s string) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(len(s)))
	p.Write(b[:])
	p.WriteString(s)
}
func (p *packet) amount(n sdkmath.Int) { var b [32]byte; n.BigInt().FillBytes(b[:]); p.Write(b[:]) }
func (p *packet) clock(t time.Time)    { p.u64(uint64(t.Unix())); p.u32(uint32(t.Nanosecond())) }
func (p *packet) evidence(e Evidence) {
	p.u64(e.Height)
	p.Write(e.BlockHash[:])
	p.Write(e.TransactionHash[:])
	p.u32(e.LogIndex)
}

func (k Keeper) domain(action string) *packet {
	p := new(packet)
	p.text("GOD Chain bridge attestation")
	p.u32(k.config.Version)
	p.text(k.config.SourceChain)
	p.text(k.config.NativeChain)
	p.Write(k.config.AssetID[:])
	p.text(action)
	return p
}

func (k Keeper) DepositDigest(d Deposit) ([32]byte, error) {
	if d.Sequence == 0 || d.Recipient == [20]byte{} || !amountValid(d.Amount) || !d.Evidence.valid() {
		return [32]byte{}, ErrRequest
	}
	p := k.domain("deposit-final")
	p.u64(d.Sequence)
	p.Write(d.Recipient[:])
	p.amount(d.Amount)
	p.evidence(d.Evidence)
	return sha256.Sum256(p.Bytes()), nil
}

func (k Keeper) withdrawalPacket(w Withdrawal, action string) (*packet, error) {
	if w.Sequence == 0 || w.Sender == [20]byte{} || w.Recipient == [20]byte{} || !amountValid(w.Amount) || !validTime(w.QueuedAt) {
		return nil, ErrRequest
	}
	p := k.domain(action)
	p.u64(w.Sequence)
	p.Write(w.Sender[:])
	p.Write(w.Recipient[:])
	p.amount(w.Amount)
	p.clock(w.QueuedAt)
	return p, nil
}

func (k Keeper) withdrawalID(w Withdrawal) ([32]byte, error) {
	p, err := k.withdrawalPacket(w, "withdrawal-id")
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(p.Bytes()), nil
}

func (k Keeper) AuthorizationDigest(w Withdrawal) ([32]byte, error) {
	p, err := k.withdrawalPacket(w, "withdrawal-authorize")
	if err != nil {
		return [32]byte{}, err
	}
	id, _ := k.withdrawalID(w)
	if id != w.ID {
		return [32]byte{}, ErrRequest
	}
	p.Write(w.ID[:])
	return sha256.Sum256(p.Bytes()), nil
}

// CancellationRequestDigest authorizes an asset-neutral RH tombstone. It is
// deliberately different from the finalized cancellation receipt accepted by
// CancelWithdrawal, so a request alone can never authorize a native refund.
func (k Keeper) CancellationRequestDigest(w Withdrawal) ([32]byte, error) {
	p, err := k.withdrawalPacket(w, "withdrawal-cancel-request")
	if err != nil {
		return [32]byte{}, err
	}
	id, _ := k.withdrawalID(w)
	if id != w.ID {
		return [32]byte{}, ErrRequest
	}
	p.Write(w.ID[:])
	return sha256.Sum256(p.Bytes()), nil
}

func AssetBinding(chainID sdkmath.Int, token, escrow [20]byte, decimals uint8) ([32]byte, error) {
	if chainID.IsNil() || !chainID.IsPositive() || token == [20]byte{} || escrow == [20]byte{} || token == escrow || decimals != godrewards.Decimals {
		return [32]byte{}, ErrConfig
	}
	p := new(packet)
	p.text("GOD Chain bridge asset")
	p.u32(1)
	p.amount(chainID)
	p.Write(token[:])
	p.Write(escrow[:])
	p.WriteByte(decimals)
	return sha256.Sum256(p.Bytes()), nil
}

func (k Keeper) ResolutionDigest(w Withdrawal, status Status, evidence Evidence) ([32]byte, error) {
	if status != Paid && status != Cancelled || !evidence.valid() {
		return [32]byte{}, ErrRequest
	}
	p, err := k.withdrawalPacket(w, "withdrawal-"+string(status)+"-final")
	if err != nil {
		return [32]byte{}, err
	}
	id, _ := k.withdrawalID(w)
	if id != w.ID {
		return [32]byte{}, ErrRequest
	}
	p.Write(w.ID[:])
	p.evidence(evidence)
	return sha256.Sum256(p.Bytes()), nil
}

func (k Keeper) PauseDigest(nonce uint64, intake, outflow bool) ([32]byte, error) {
	if nonce == 0 || !intake && !outflow {
		return [32]byte{}, ErrRequest
	}
	p := k.domain("pause-only")
	p.u64(nonce)
	var flags byte
	if intake {
		flags |= 1
	}
	if outflow {
		flags |= 2
	}
	p.WriteByte(flags)
	return sha256.Sum256(p.Bytes()), nil
}

func (k Keeper) verify(digest [32]byte, signatures [][]byte) error {
	if len(signatures) < Threshold || len(signatures) > SignerCount {
		return ErrApprovals
	}
	seen := map[[20]byte]bool{}
	for _, signature := range signatures {
		if len(signature) != 65 || !ethcrypto.ValidateSignatureValues(signature[64], new(big.Int).SetBytes(signature[:32]), new(big.Int).SetBytes(signature[32:64]), true) {
			return ErrApprovals
		}
		pub, err := ethcrypto.SigToPub(digest[:], append([]byte(nil), signature...))
		if err != nil {
			return ErrApprovals
		}
		address := [20]byte(ethcrypto.PubkeyToAddress(*pub))
		member := false
		for _, allowed := range k.config.Signers {
			if address == allowed {
				member = true
				break
			}
		}
		if !member || seen[address] {
			return ErrApprovals
		}
		seen[address] = true
	}
	return nil
}
