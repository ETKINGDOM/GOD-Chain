package godrh

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// Creation and subsequent custody calls occupy ONE exclusively owned sender
// book. This is still bounded, cooperative simulation coordination, not a live
// nonce/fee oracle, protection from bypassed tools, or financial authority.
const (
	creationNonceReserved        = "creation_reserved_simulation"
	creationNonceSigningUnknown  = "creation_signing_unknown_simulation"
	creationNonceSigned          = "creation_signed_simulation"
	creationNonceDispatchUnknown = "creation_dispatch_unknown_simulation"
	creationNonceObserved        = "creation_observed_simulation"
)

type CustodyNonceCreationInputs struct {
	Deployment CustodyDeploymentFiles
	Compiler   TokenCompiler
	OutputPath string
}

func (CustodyNonceCreationInputs) String() string     { return "RH shared creation inputs (redacted)" }
func (i CustodyNonceCreationInputs) GoString() string { return i.String() }
func (CustodyNonceCreationInputs) MarshalJSON() ([]byte, error) {
	return []byte(`{"redacted":true}`), nil
}

type custodyNonceCreation struct {
	SourceSHA256      string `json:"sourceSHA256"`
	RequestSHA256     string `json:"requestSHA256"`
	NodeSHA256        string `json:"nodeSHA256"`
	CompilerSHA256    string `json:"compilerSHA256"`
	UnsignedSHA256    string `json:"unsignedSHA256"`
	OutputPathSHA256  string `json:"outputPathSHA256"`
	Nonce             string `json:"nonce"`
	MaximumGasCost    string `json:"maximumGasCost"`
	Stage             string `json:"stage"`
	SignedSHA256      string `json:"signedSHA256,omitempty"`
	TransactionHash   string `json:"transactionHash,omitempty"`
	ObservationSHA256 string `json:"observationSHA256,omitempty"`
	Succeeded         bool   `json:"succeeded,omitempty"`
}

func custodyCreationSlots(s custodyNonceState) int {
	if s.Creation != nil {
		return 1
	}
	return 0
}
func sameCustodyCreationState(a, b custodyNonceState) bool {
	return a.CreationRevision == b.CreationRevision && (a.Creation == nil && b.Creation == nil ||
		a.Creation != nil && b.Creation != nil && *a.Creation == *b.Creation)
}
func sameCustodyCreationIntent(a, b custodyNonceCreation) bool {
	a.Stage, b.Stage = creationNonceReserved, creationNonceReserved
	a.SignedSHA256, b.SignedSHA256, a.TransactionHash, b.TransactionHash = "", "", "", ""
	a.ObservationSHA256, b.ObservationSHA256, a.Succeeded, b.Succeeded = "", "", false, false
	return a == b
}
func custodyCreationStateCost(s custodyNonceState) (*big.Int, error) {
	bad := func() (*big.Int, error) { return nil, ErrCustodyNonceState }
	if s.Creation == nil {
		if s.CreationRevision != 0 {
			return bad()
		}
		return new(big.Int), nil
	}
	c := s.Creation
	if s.CreationRevision < 1 || s.CreationRevision > 5 {
		return bad()
	}
	stages := [...]string{creationNonceReserved, creationNonceSigningUnknown, creationNonceSigned, creationNonceDispatchUnknown, creationNonceObserved}
	if c.Stage != stages[s.CreationRevision-1] {
		return bad()
	}
	for _, p := range []string{c.SourceSHA256, c.RequestSHA256, c.NodeSHA256, c.CompilerSHA256, c.UnsignedSHA256, c.OutputPathSHA256} {
		if _, err := SourceMaterialPin(p); err != nil {
			return bad()
		}
	}
	nonce, err := custodyNonceCounter(c.Nonce)
	start, e := custodyNonceCounter(s.StartingNonce)
	if err != nil || e != nil || nonce != start {
		return bad()
	}
	if s.CreationRevision < 3 {
		if c.SignedSHA256 != "" || c.TransactionHash != "" {
			return bad()
		}
	} else {
		for _, p := range []string{c.SignedSHA256, c.TransactionHash} {
			if _, err := SourceMaterialPin(p); err != nil {
				return bad()
			}
		}
	}
	if s.CreationRevision < 5 {
		if c.ObservationSHA256 != "" || c.Succeeded {
			return bad()
		}
	} else if _, err := SourceMaterialPin(c.ObservationSHA256); err != nil {
		return bad()
	}
	if (s.CreationRevision != 5 || !c.Succeeded) && (len(s.Entries) != 0 || len(s.Reviews) != 0 || s.SigningRevision != 0 || s.SigningIntent != nil) {
		return bad() // no calls to a not-yet-observed or reverted creation
	}
	return custodyNonceAmount(c.MaximumGasCost)
}
func custodyCreationStepTransition(old, next custodyNonceState) bool {
	if next.CreationRevision != old.CreationRevision+1 || next.Creation == nil ||
		len(old.Entries) != 0 || len(next.Entries) != 0 || len(old.Reviews) != 0 || len(next.Reviews) != 0 ||
		!sameCustodySigningState(old, next) || old.SigningIntent != nil {
		return false
	}
	if old.Creation == nil {
		return old.CreationRevision == 0 && next.CreationRevision == 1
	}
	if !sameCustodyCreationIntent(*old.Creation, *next.Creation) {
		return false
	}
	if old.CreationRevision >= 3 && (old.Creation.SignedSHA256 != next.Creation.SignedSHA256 || old.Creation.TransactionHash != next.Creation.TransactionHash) {
		return false
	}
	return old.CreationRevision < 5
}

type custodyNonceCreationGuard struct {
	inputs     CustodyNonceCreationInputs
	deployment CustodyDeployment
	intent     custodyNonceCreation
}

func loadCustodyNonceCreationGuard(ctx context.Context, i CustodyNonceInputs, creation CustodyNonceCreationInputs) (*custodyNonceCreationGuard, error) {
	if ctx == nil || ctx.Err() != nil || !filepath.IsAbs(creation.OutputPath) || filepath.Clean(creation.OutputPath) != creation.OutputPath || len(creation.OutputPath) > 4096 || strings.ContainsAny(creation.OutputPath, "\x00\r\n") {
		return nil, ErrCustodyNonceState
	}
	_, plan, chain, err := loadCustodyNonceInputs(i)
	if err != nil {
		return nil, ErrCustodyNonceState
	}
	raw, err := readCustodyPrivateSnapshot(i.ConfigurationPath, maxConfigBytes, i.ConfigurationSHA256)
	if err != nil {
		return nil, ErrCustodyNonceState
	}
	c, err := Parse(raw)
	if err != nil {
		return nil, ErrCustodyNonceState
	}
	d, err := loadCustodyDeployment(ctx, creation.Deployment, creation.Compiler)
	if err != nil || !d.report.SingleCustodianDeclared || len(d.runtime) == 0 || d.creator != plan.sender {
		return nil, ErrCustodyNonceState
	}
	request, err := readRHPrivateBytes(creation.Deployment.RequestPath, MaxCustodyRequestBytes, creation.Deployment.RequestPin)
	if err != nil {
		return nil, ErrCustodyNonceState
	}
	p, err := parseCustodyDeploymentRequest(request)
	if err != nil || p.chain.Cmp(chain) != 0 || p.plan.nonce != plan.start || p.source != c.private.SourceChain || p.native != c.private.NativeChain {
		return nil, ErrCustodyNonceState
	}
	asset, err := address(c.private.TokenContract)
	custody, e := address(c.private.CustodyContract)
	code, h := hash(c.private.ExpectedCustodyCodeHash)
	if err != nil || e != nil || h != nil || [20]byte(p.asset) != asset || [20]byte(d.predicted) != custody || [32]byte(ethcrypto.Keccak256Hash(d.runtime)) != code {
		return nil, ErrCustodyNonceState
	}
	seen := map[[20]byte]bool{}
	for _, s := range c.private.Signers {
		a, err := address(s)
		if err != nil {
			return nil, ErrCustodyNonceState
		}
		seen[a] = true
	}
	if len(seen) != 7 {
		return nil, ErrCustodyNonceState
	}
	for _, a := range p.members {
		if !seen[[20]byte(a)] {
			return nil, ErrCustodyNonceState
		}
	}
	cost := new(big.Int).Mul(new(big.Int).SetUint64(p.plan.gas), p.plan.fee)
	if cost.Cmp(plan.budget) > 0 {
		return nil, ErrCustodyNonceState
	}
	g := &custodyNonceCreationGuard{inputs: creation, deployment: d, intent: custodyNonceCreation{
		SourceSHA256: creationHexPin(creation.Deployment.SourcePin), RequestSHA256: creationHexPin(creation.Deployment.RequestPin),
		NodeSHA256: creationHexPin(creation.Compiler.nodePin), CompilerSHA256: creationHexPin(creation.Compiler.compilerPin),
		UnsignedSHA256: creationHexPin(sha256.Sum256(d.raw)), OutputPathSHA256: creationHexPin(sha256.Sum256([]byte(creation.OutputPath))),
		Nonce: strconv.FormatUint(p.plan.nonce, 10), MaximumGasCost: cost.String(), Stage: creationNonceReserved}}
	if ctx.Err() != nil {
		return nil, ErrCustodyNonceState
	}
	return g, nil
}
func (g *custodyNonceCreationGuard) matches(s custodyNonceState) bool {
	if s.Creation == nil {
		return s.Sequence == 1 && len(s.Entries) == 0 && s.SigningIntent == nil
	}
	return sameCustodyCreationIntent(g.intent, *s.Creation)
}
func (g *custodyNonceCreationGuard) separated(directory string) bool {
	if directory == "" {
		return true
	}
	physical, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return false
	}
	for _, path := range []string{g.inputs.Deployment.SourcePath, g.inputs.Deployment.RequestPath, g.inputs.Compiler.nodePath, g.inputs.Compiler.compilerPath, g.inputs.OutputPath} {
		parent, err := filepath.EvalSymlinks(filepath.Dir(path))
		if err != nil || parent == physical {
			return false
		}
	}
	return true
}
func (g *custodyNonceCreationGuard) recheck(ctx context.Context, s custodyNonceState, directory string) bool {
	if !g.matches(s) || !g.separated(directory) {
		return false
	}
	for _, f := range []struct {
		path       string
		pin        [32]byte
		limit      int64
		executable bool
	}{
		{g.inputs.Deployment.SourcePath, g.inputs.Deployment.SourcePin, 64 << 10, false},
		{g.inputs.Compiler.nodePath, g.inputs.Compiler.nodePin, maxTokenRuntimeBytes, true},
		{g.inputs.Compiler.compilerPath, g.inputs.Compiler.compilerPin, maxTokenCompilerBytes, false},
	} {
		if _, err := tokenCompilerFile(ctx, f.path, f.pin, f.limit, f.executable); err != nil {
			return false
		}
	}
	if _, err := readRHPrivateBytes(g.inputs.Deployment.RequestPath, MaxCustodyRequestBytes, g.inputs.Deployment.RequestPin); err != nil {
		return false
	}
	if s.CreationRevision < 2 && custodySigningOutputAvailable(g.inputs.OutputPath) != nil {
		return false
	}
	if s.CreationRevision >= 3 {
		pin, err := SourceMaterialPin(s.Creation.SignedSHA256)
		if err != nil {
			return false
		}
		wire, err := readRHPrivateBytes(g.inputs.OutputPath, MaxTokenCreationInputBytes+1024, pin)
		if err != nil {
			return false
		}
		checked, err := checkCustodyDeploymentTransaction(g.deployment, wire)
		if err != nil || creationHexPin(checked.hash) != s.Creation.TransactionHash {
			return false
		}
	}
	return ctx.Err() == nil
}

// Failed construction leaves storage ownership with the caller and any
// complete/partial records intact. Reopen requires original creation inputs
// and a separately retained exact latest pin; the legacy constructor refuses
// a creation-bearing book rather than omitting the creation guard.
func NewCustodyNonceCreationBookForSimulation(ctx context.Context, i CustodyNonceInputs, creation CustodyNonceCreationInputs, store CustodyNonceStorage, create bool, resumePin ...[32]byte) (book *CustodyNonceBook, err error) {
	defer func() {
		if recover() != nil {
			book, err = nil, ErrCustodyNonceState
		}
	}()
	if ctx == nil || ctx.Err() != nil || store == nil || !custodyAttemptResumeValid(create, resumePin) {
		return nil, ErrCustodyNonceState
	}
	g, err := loadCustodyNonceCreationGuard(ctx, i, creation)
	if err != nil {
		return nil, ErrCustodyNonceState
	}
	if create && custodySigningOutputAvailable(creation.OutputPath) != nil {
		return nil, ErrCustodyNonceState
	}
	b, err := newCustodyNonceBook(ctx, i, store, create, g, resumePin...)
	if err != nil {
		return nil, err
	}
	if create {
		if err := b.reserveCreation(ctx); err != nil {
			return nil, err
		}
	}
	if err := b.recheck(ctx); err != nil {
		return nil, err
	}
	return b, nil
}
func (b *CustodyNonceBook) creationAllowsCalls() bool {
	return b.creationGuard == nil || b.state.CreationRevision == 5 && b.state.Creation != nil && b.state.Creation.Succeeded
}
func (b *CustodyNonceBook) reserveCreation(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready(ctx) || b.creationGuard == nil || b.signingActive {
		return ErrCustodyNonceState
	}
	if err := b.recheck(ctx); err != nil {
		return err
	}
	next := b.nextState()
	wanted := b.creationGuard.intent
	next.Creation, next.CreationRevision = &wanted, 1
	if !custodyNonceTransition(b.state, next) {
		return ErrCustodyNonceState
	}
	return b.write(next)
}
func (b *CustodyNonceBook) creationStepPin(ctx context.Context, from uint32, stage string) ([32]byte, error) {
	if b == nil {
		return [32]byte{}, ErrCustodyNonceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready(ctx) || b.creationGuard == nil || b.signingActive || b.state.CreationRevision != from {
		return [32]byte{}, ErrCustodyNonceState
	}
	if err := b.recheck(ctx); err != nil {
		return [32]byte{}, err
	}
	next := b.nextState()
	next.CreationRevision++
	next.Creation.Stage = stage
	if !custodyNonceTransition(b.state, next) {
		return [32]byte{}, ErrCustodyNonceState
	}
	raw, err := encodeCustodyNonce(next)
	if err != nil {
		return [32]byte{}, ErrCustodyNonceState
	}
	return sha256.Sum256(raw), nil
}

// Retain each expected UNKNOWN pin BEFORE invoking the corresponding step.
// Computing a pin writes nothing and exports no signing/submission bytes.
func (b *CustodyNonceBook) ExpectedCreationSigningPin(ctx context.Context) ([32]byte, error) {
	return b.creationStepPin(ctx, 1, creationNonceSigningUnknown)
}
func (b *CustodyNonceBook) ExpectedCreationDispatchPin(ctx context.Context) ([32]byte, error) {
	return b.creationStepPin(ctx, 3, creationNonceDispatchUnknown)
}

// The original creation is necessarily the first slot of this book. Its five
// frames can therefore be re-derived from the independently pinned header and
// original intent, never by trusting a newer unreviewed journal hash. This also
// gives exact idempotent checkpoint comparison after later calls append state.
func custodyCreationFrame(s custodyNonceState, c custodyNonceCreation, revision uint32) (custodyNonceState, error) {
	if revision < 1 || revision > 5 {
		return custodyNonceState{}, ErrCustodyNonceState
	}
	s.Sequence, s.PreviousSHA256, s.SigningRevision, s.CreationRevision = 1, "", 0, 0
	s.Entries, s.Reviews, s.SigningIntent, s.Creation = []custodyNonceEntry{}, nil, nil, nil
	for n := uint32(1); n <= revision; n++ {
		raw, err := encodeCustodyNonce(s)
		if err != nil {
			return custodyNonceState{}, ErrCustodyNonceState
		}
		next := s
		x := c
		stages := [...]string{creationNonceReserved, creationNonceSigningUnknown, creationNonceSigned, creationNonceDispatchUnknown, creationNonceObserved}
		x.Stage = stages[n-1]
		if n < 3 {
			x.SignedSHA256, x.TransactionHash = "", ""
		}
		if n < 5 {
			x.ObservationSHA256, x.Succeeded = "", false
		}
		next.Creation, next.CreationRevision = &x, n
		next.Sequence++
		next.PreviousSHA256 = creationHexPin(sha256.Sum256(raw))
		if !custodyNonceTransition(s, next) {
			return custodyNonceState{}, ErrCustodyNonceState
		}
		s = next
	}
	return s, nil
}
func (b *CustodyNonceBook) checkedCreationSigned(ctx context.Context, signedPin [32]byte) (checkedCustodyDeployment, error) {
	wire, err := readRHPrivateBytes(b.creationGuard.inputs.OutputPath, MaxTokenCreationInputBytes+1024, signedPin)
	if err != nil {
		return checkedCustodyDeployment{}, ErrCustodyNonceState
	}
	checked, err := checkCustodyDeploymentTransaction(b.creationGuard.deployment, wire)
	if err != nil || ctx.Err() != nil {
		return checkedCustodyDeployment{}, ErrCustodyNonceState
	}
	return checked, nil
}
func (b *CustodyNonceBook) ExpectedCreationRetentionPin(ctx context.Context, signedPin [32]byte) ([32]byte, error) {
	if b == nil {
		return [32]byte{}, ErrCustodyNonceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready(ctx) || b.creationGuard == nil || b.signingActive || b.state.CreationRevision < 2 {
		return [32]byte{}, ErrCustodyNonceState
	}
	if err := b.recheck(ctx); err != nil {
		return [32]byte{}, err
	}
	checked, err := b.checkedCreationSigned(ctx, signedPin)
	if err != nil {
		return [32]byte{}, err
	}
	c := *b.state.Creation
	c.SignedSHA256, c.TransactionHash = creationHexPin(signedPin), creationHexPin(checked.hash)
	if b.state.CreationRevision >= 3 && (c.SignedSHA256 != b.state.Creation.SignedSHA256 || c.TransactionHash != b.state.Creation.TransactionHash) {
		return [32]byte{}, ErrCustodyNonceState
	}
	next, err := custodyCreationFrame(b.state, c, 3)
	if err != nil {
		return [32]byte{}, err
	}
	raw, err := encodeCustodyNonce(next)
	if err != nil {
		return [32]byte{}, ErrCustodyNonceState
	}
	return sha256.Sum256(raw), nil
}
func (b *CustodyNonceBook) consumeCreationStep(ctx context.Context, from uint32, stage string, expected [32]byte, signing bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.consumeCreationStepLocked(ctx, from, stage, expected, signing)
}

func (b *CustodyNonceBook) consumeCreationStepLocked(ctx context.Context, from uint32, stage string, expected [32]byte, signing bool) error {
	if !b.ready(ctx) || b.creationGuard == nil || b.signingActive || b.state.CreationRevision != from {
		return ErrCustodyNonceUnknown
	}
	if err := b.recheck(ctx); err != nil {
		return err
	}
	next := b.nextState()
	next.CreationRevision++
	next.Creation.Stage = stage
	raw, err := encodeCustodyNonce(next)
	if err != nil || expected == ([32]byte{}) || sha256.Sum256(raw) != expected || !custodyNonceTransition(b.state, next) || ctx.Err() != nil {
		return ErrCustodyNonceState
	}
	if err := b.write(next); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ErrCustodyNonceUnknown
	}
	b.signingActive = signing
	return nil
}
func (b *CustodyNonceBook) SignCreationForSimulation(ctx context.Context, expected [32]byte, signer LocalCustodySimulationSigner) (pin [32]byte, report CustodySigningReport, err error) {
	active := false
	defer func() {
		if recover() != nil {
			pin, err = [32]byte{}, ErrCustodySigningUnknown
			report.SigningOutcomeUnknown = true
		}
		if active {
			b.mu.Lock()
			b.signingActive = false
			b.mu.Unlock()
		}
	}()
	if b == nil || ctx == nil || ctx.Err() != nil || signer == nil {
		return pin, report, ErrCustodyNonceState
	}
	deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := b.consumeCreationStep(deadline, 1, creationNonceSigningUnknown, expected, true); err != nil {
		return pin, report, err
	}
	active = true
	report = CustodySigningReport{SimulationOnly: true, ExactUnsignedEnvelopePrepared: true, LocalSigningAttemptConsumed: true,
		DurableSigningAttempt: true, SharedNonceReserved: true, SigningOutcomeUnknown: true}
	wire, err := signer.SignLocalCustody(deadline, bytes.Clone(b.creationGuard.deployment.raw))
	if err != nil || len(wire) == 0 || len(wire) > MaxTokenCreationInputBytes+1024 {
		return pin, report, ErrCustodySigningUnknown
	}
	wire = bytes.Clone(wire)
	if deadline.Err() != nil {
		return pin, report, ErrCustodySigningUnknown
	}
	checked, err := checkCustodyDeploymentTransaction(b.creationGuard.deployment, wire)
	if err != nil {
		return pin, report, ErrCustodySigningUnknown
	}
	report.SignerResultMatched = true
	if writeRHPrivateFile(b.creationGuard.inputs.OutputPath, checked.raw, MaxTokenCreationInputBytes+1024) != nil {
		return pin, report, ErrCustodySigningUnknown
	}
	pin = sha256.Sum256(checked.raw)
	// Leave the durable signing-UNKNOWN frame intact. Binding the signed file
	// is a SEPARATE step so its expected next checkpoint can be retained before
	// the journal changes. A lost signing reply can reopen the known UNKNOWN.
	b.mu.Lock()
	checkErr := b.recheck(deadline)
	b.mu.Unlock()
	if checkErr != nil {
		return [32]byte{}, report, ErrCustodySigningUnknown
	}
	report.PrivateEnvelopeRetained, report.SigningOutcomeUnknown = true, false
	return pin, report, nil
}
func (b *CustodyNonceBook) ReconcileCreationSignedForSimulation(ctx context.Context, signedPin, expected [32]byte) error {
	return b.retainCreationSigned(ctx, signedPin, expected)
}
func (b *CustodyNonceBook) retainCreationSigned(ctx context.Context, signedPin, expected [32]byte) error {
	if b == nil {
		return ErrCustodyNonceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready(ctx) || b.creationGuard == nil || b.signingActive || b.state.CreationRevision < 2 {
		return ErrCustodyNonceState
	}
	if err := b.recheck(ctx); err != nil {
		return err
	}
	checked, err := b.checkedCreationSigned(ctx, signedPin)
	if err != nil {
		return err
	}
	c := *b.state.Creation
	c.SignedSHA256, c.TransactionHash = creationHexPin(signedPin), creationHexPin(checked.hash)
	next, err := custodyCreationFrame(b.state, c, 3)
	if err != nil {
		return ErrCustodyNonceState
	}
	raw, err := encodeCustodyNonce(next)
	if err != nil || expected == ([32]byte{}) || sha256.Sum256(raw) != expected {
		return ErrCustodyNonceState
	}
	if b.state.CreationRevision >= 3 {
		if b.state.Creation.SignedSHA256 != creationHexPin(signedPin) || b.state.Creation.TransactionHash != creationHexPin(checked.hash) {
			return ErrCustodyNonceState
		}
		return nil
	}
	if !custodyNonceTransition(b.state, next) {
		return ErrCustodyNonceState
	}
	if err := b.write(next); err != nil {
		return err
	}
	if err := b.recheck(ctx); err != nil {
		return ErrCustodySigningUnknown
	}
	return nil
}
func (b *CustodyNonceBook) TakeCreationForSimulation(ctx context.Context, expected [32]byte) (wire []byte, err error) {
	defer func() {
		if recover() != nil {
			wire, err = nil, ErrCustodyNonceUnknown
		}
	}()
	if b == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrCustodyNonceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	// Keep ownership across the durable transition and detached export. A
	// concurrent observation must not advance the book between those steps.
	if err := b.consumeCreationStepLocked(ctx, 3, creationNonceDispatchUnknown, expected, false); err != nil {
		return nil, err
	}
	if !b.ready(ctx) || b.creationGuard == nil {
		return nil, ErrCustodyNonceUnknown
	}
	if err := b.recheck(ctx); err != nil {
		return nil, ErrCustodyNonceUnknown
	}
	pin, err := SourceMaterialPin(b.state.Creation.SignedSHA256)
	if err != nil {
		return nil, ErrCustodyNonceUnknown
	}
	wire, err = readRHPrivateBytes(b.creationGuard.inputs.OutputPath, MaxTokenCreationInputBytes+1024, pin)
	if err != nil || ctx.Err() != nil {
		return nil, ErrCustodyNonceUnknown
	}
	return bytes.Clone(wire), nil
}
func (b *CustodyNonceBook) ExpectedCreationObservationPin(ctx context.Context, block Block, material BlockMaterial, proof ContractStateProof) (pin [32]byte, err error) {
	_, pin, err = b.observeCreation(ctx, block, material, proof, [32]byte{}, false)
	return pin, err
}
func (b *CustodyNonceBook) ObserveCreationForSimulation(ctx context.Context, block Block, material BlockMaterial, proof ContractStateProof, expected [32]byte) (CustodyCreationExecutionReport, error) {
	r, _, err := b.observeCreation(ctx, block, material, proof, expected, true)
	return r, err
}
func (b *CustodyNonceBook) observeCreation(ctx context.Context, block Block, material BlockMaterial, proof ContractStateProof, expected [32]byte, retain bool) (report CustodyCreationExecutionReport, pin [32]byte, err error) {
	defer func() {
		if recover() != nil {
			report, pin, err = CustodyCreationExecutionReport{}, [32]byte{}, ErrCustodyNonceState
		}
	}()
	if b == nil {
		return report, pin, ErrCustodyNonceState
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.ready(ctx) || b.creationGuard == nil || b.signingActive || b.state.CreationRevision < 4 {
		return report, pin, ErrCustodyNonceState
	}
	if err := b.recheck(ctx); err != nil {
		return report, pin, err
	}
	signedPin, err := SourceMaterialPin(b.state.Creation.SignedSHA256)
	if err != nil {
		return report, pin, ErrCustodyNonceState
	}
	checked, err := b.checkedCreationSigned(ctx, signedPin)
	if err != nil {
		return report, pin, ErrCustodyNonceState
	}
	r, digest, err := checkCustodyCreationExecution(ctx, checked, block, material, proof)
	if err != nil {
		return report, pin, ErrCustodyNonceState
	}
	if err := b.recheck(ctx); err != nil {
		return report, pin, err
	}
	c := *b.state.Creation
	c.ObservationSHA256, c.Succeeded = creationHexPin(digest), r.ReceiptSucceeded
	next, err := custodyCreationFrame(b.state, c, 5)
	if err != nil {
		return report, pin, ErrCustodyNonceState
	}
	raw, err := encodeCustodyNonce(next)
	if err != nil {
		return report, pin, ErrCustodyNonceState
	}
	pin = sha256.Sum256(raw)
	if retain && (expected == ([32]byte{}) || pin != expected) {
		return report, [32]byte{}, ErrCustodyNonceState
	}
	if b.state.CreationRevision == 5 {
		if b.state.Creation.ObservationSHA256 != creationHexPin(digest) || b.state.Creation.Succeeded != r.ReceiptSucceeded {
			return report, [32]byte{}, ErrCustodyNonceState
		}
	} else if retain {
		if !custodyNonceTransition(b.state, next) {
			return report, [32]byte{}, ErrCustodyNonceState
		}
		if err := b.write(next); err != nil {
			return report, [32]byte{}, err
		}
	}
	r.PrivateInputsChecked = true
	return r, pin, nil
}

// Only status flags are serializable; signing output stays in the private file.
var _ json.Marshaler = CustodyNonceCreationInputs{}
