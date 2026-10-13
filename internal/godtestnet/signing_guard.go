//go:build go1.25

package godtestnet

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
	cmtjson "github.com/cometbft/cometbft/libs/json"
)

const signingGuardFile = "signing-guard-required.json"

type guardRequirement struct {
	Version   uint32   `json:"version"`
	Directory [32]byte `json:"directory"`
}

// A redacted, point-in-time local synthetic check, not a restart authorization.
// It includes no key, signature, witness identity, path, address or endpoint.
type SigningGuardCheckReport struct {
	Report
	Check  string `json:"check"`
	Height int64  `json:"height"`
	Round  int32  `json:"round"`
	Step   int8   `json:"step"`
}

func separateGuard(home, guard string) bool {
	if !filepath.IsAbs(guard) || filepath.Clean(guard) != guard || guard == string(filepath.Separator) {
		return false
	}
	home = filepath.Clean(home)
	return home != guard && !strings.HasPrefix(guard, home+string(filepath.Separator)) && !strings.HasPrefix(home, guard+string(filepath.Separator))
}

func guardGenesis(loaded *Loaded) ([32]byte, error) {
	raw, err := cmtjson.Marshal(loaded.Genesis)
	if err != nil {
		return [32]byte{}, godnode.ErrSigningGuard
	}
	return sha256.Sum256(raw), nil
}

// InitializeNodeSigningGuard is a stopped, fresh synthetic-workspace operation.
// It never adopts a used validator or repairs a lost witness. The requirement
// marker makes future cooperating starts refuse an omitted/wrong guard path.
func InitializeNodeSigningGuard(home, guard string) (Report, error) {
	if !separateGuard(home, guard) {
		return Report{}, godnode.ErrSigningGuard
	}
	r, err := openPrivate(home)
	if err != nil {
		return Report{}, err
	}
	defer r.Close()
	lease, err := acquireNodeLease(r)
	if err != nil {
		return Report{}, err
	}
	defer lease.Close()
	loaded, err := Load(home)
	if err != nil {
		return Report{}, err
	}
	if loaded.Document.Role == "observer" || loaded.Signer.LastSignState.Height != 0 {
		return Report{}, godnode.ErrSigningGuard
	}
	for _, name := range []string{"data", signingGuardFile} {
		if _, err := r.Lstat(name); !os.IsNotExist(err) {
			return Report{}, godnode.ErrSigningGuard
		}
	}
	genesis, err := guardGenesis(loaded)
	if err != nil {
		return Report{}, err
	}
	marker, _ := json.Marshal(guardRequirement{1, sha256.Sum256([]byte(guard))})
	// Require the witness before it is created. Interrupted initialization
	// leaves a fail-closed marker, never a silent unguarded fallback.
	if godnode.InitializeFileSigningGuard(guard, loaded.Document.Runtime.ChainID, genesis, loaded.Signer,
		godnode.SigningGuardFiles{Home: home, KeyFile: keyFile, StateFile: stateFile}, func() error { return writePrivate(r, signingGuardFile, marker) }) != nil {
		return Report{}, godnode.ErrSigningGuard
	}
	loaded.Report.SigningGuard = true
	return loaded.Report, nil
}

// CheckNodeSigningGuard holds both existing cooperating-process leases while
// validating the stopped synthetic workspace and exact retained witness. It
// opens no application database or listener, signs nothing and writes no file.
// An omitted guard, unmarked home, absent lease or mismatch never gets repaired.
// Starting later must validate again; this does not prove protection from the
// host owner, whole-host rollback, another guard directory or an old executable.
func CheckNodeSigningGuard(home, guard string) (SigningGuardCheckReport, error) {
	return withStoppedSigningGuard(home, guard, nil)
}

// The optional inspection runs with both leases held. No caller may retain the
// signer or node root, mutate files, or treat the report as an upgrade permit.
func withStoppedSigningGuard(home, guard string, inspect func() error) (report SigningGuardCheckReport, err error) {
	defer func() {
		if recover() != nil {
			report, err = SigningGuardCheckReport{}, godnode.ErrSigningGuard
		}
	}()
	if !separateGuard(home, guard) {
		return SigningGuardCheckReport{}, godnode.ErrSigningGuard
	}
	r, err := openPrivate(home)
	if err != nil {
		return SigningGuardCheckReport{}, err
	}
	defer r.Close()
	lease, err := acquireExistingNodeLease(r)
	if err != nil {
		return SigningGuardCheckReport{}, err
	}
	defer lease.Close()
	before, err := stoppedInputs(r, home)
	if err != nil {
		return SigningGuardCheckReport{}, godnode.ErrSigningGuard
	}
	loaded, err := Load(home)
	if err != nil {
		return SigningGuardCheckReport{}, err
	}
	g, err := openNodeSigningGuard(r, home, guard, loaded)
	if err != nil || g == nil {
		return SigningGuardCheckReport{}, godnode.ErrSigningGuard
	}
	defer g.Close()
	state := loaded.Signer.LastSignState
	if g.Check() != nil {
		return SigningGuardCheckReport{}, godnode.ErrSigningGuard
	}
	if inspect != nil && inspect() != nil {
		return SigningGuardCheckReport{}, godnode.ErrSigningGuard
	}
	after, err := stoppedInputs(r, home)
	if err != nil || !sameStoppedInputs(before, after) || g.Check() != nil {
		return SigningGuardCheckReport{}, godnode.ErrSigningGuard
	}
	if g.Close() != nil || lease.Close() != nil {
		return SigningGuardCheckReport{}, godnode.ErrSigningGuard
	}
	loaded.Report.SigningGuard = true
	return SigningGuardCheckReport{loaded.Report, "retained-progress-matches", state.Height, state.Round, state.Step}, nil
}

func openNodeSigningGuard(r *os.Root, home, guard string, loaded *Loaded) (*godnode.GuardedSigner, error) {
	_, err := r.Lstat(signingGuardFile)
	if os.IsNotExist(err) && guard == "" {
		return nil, nil
	}
	if err != nil || !separateGuard(home, guard) || loaded.Document.Role == "observer" {
		return nil, godnode.ErrSigningGuard
	}
	raw, err := readPrivate(r, signingGuardFile, 2048)
	var marker guardRequirement
	if err != nil || decode(raw, &marker) != nil || marker.Version != 1 || marker.Directory != sha256.Sum256([]byte(guard)) {
		return nil, godnode.ErrSigningGuard
	}
	genesis, err := guardGenesis(loaded)
	if err != nil {
		return nil, err
	}
	return godnode.OpenFileSigningGuard(guard, loaded.Document.Runtime.ChainID, genesis, loaded.Signer,
		godnode.SigningGuardFiles{Home: home, KeyFile: keyFile, StateFile: stateFile})
}
