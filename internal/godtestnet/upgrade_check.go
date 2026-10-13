//go:build go1.25

package godtestnet

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ETKINGDOM/GOD-Chain/internal/goddeploy"
)

var ErrUpgradeCheck = errors.New("stopped candidate preflight rejected")

type UpgradeCheckOptions struct {
	Home, SignerGuard, Archive, ExpectedArchive, CandidateDir string
}

// A stopped synthetic integrity check only. Database migration, runtime
// compatibility, release provenance and activation are deliberately unproved.
type UpgradeCheckReport struct {
	SigningGuardCheckReport
	CandidateTarget            string `json:"candidateTarget"`
	CandidateArchiveVerified   bool   `json:"candidateArchiveVerified"`
	CandidateDirectoryVerified bool   `json:"candidateDirectoryVerified"`
	CandidateExecuted          bool   `json:"candidateExecuted"`
	StateCompatibilityVerified bool   `json:"stateCompatibilityVerified"`
	MigrationVerified          bool   `json:"migrationVerified"`
	UpgradeAuthorized          bool   `json:"upgradeAuthorized"`
}

// CheckUpgradeCandidate holds the stopped home and key-wide leases throughout
// exact package verification and rechecks the original inputs afterward. It
// never unpacks, installs, executes, opens a database, signs or starts a node.
func CheckUpgradeCandidate(o UpgradeCheckOptions) (UpgradeCheckReport, error) {
	return checkUpgradeCandidate(o, goddeploy.CheckUnpacked)
}

func checkUpgradeCandidate(o UpgradeCheckOptions, check func(string, string, string) (goddeploy.Report, error)) (UpgradeCheckReport, error) {
	paths := []string{o.Home, o.SignerGuard, o.Archive, o.CandidateDir}
	resolved := make([]string, 0, len(paths))
	for i, p := range paths {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p || p == string(filepath.Separator) {
			return UpgradeCheckReport{}, ErrUpgradeCheck
		}
		for _, other := range paths[:i] {
			if !separateGuard(other, p) {
				return UpgradeCheckReport{}, ErrUpgradeCheck
			}
		}
		physical, err := filepath.EvalSymlinks(p)
		if err != nil {
			return UpgradeCheckReport{}, ErrUpgradeCheck
		}
		for _, other := range resolved {
			if !separateGuard(other, physical) {
				return UpgradeCheckReport{}, ErrUpgradeCheck
			}
		}
		resolved = append(resolved, physical)
	}
	var candidate goddeploy.Report
	guard, err := withStoppedSigningGuard(o.Home, o.SignerGuard, func() error {
		var err error
		candidate, err = check(o.Archive, o.ExpectedArchive, o.CandidateDir)
		if err != nil || !candidate.Verified || !candidate.DirectoryVerified || !candidate.Synthetic ||
			candidate.RealAssets || candidate.Created || candidate.Unpacked || candidate.ServicesInstalled || candidate.NodeStarted ||
			candidate.RuntimeVerified || candidate.BinaryRedistributionReviewed || candidate.SHA256 != o.ExpectedArchive ||
			(candidate.Target != "linux/amd64" && candidate.Target != "linux/arm64") {
			return ErrUpgradeCheck
		}
		return nil
	})
	if err != nil {
		return UpgradeCheckReport{}, ErrUpgradeCheck
	}
	guard.Check = "stopped-candidate-integrity-matches"
	return UpgradeCheckReport{SigningGuardCheckReport: guard, CandidateTarget: candidate.Target,
		CandidateArchiveVerified: true, CandidateDirectoryVerified: true}, nil
}

type stoppedInput struct {
	info os.FileInfo
	hash [32]byte
}

// Fixed, bounded original node inputs only; no recursive database/key search.
// Retain identities and metadata as well as bytes across a long inspection.
func stoppedInputs(root *os.Root, home string) (map[string]stoppedInput, error) {
	top, err := root.Stat(".")
	listed, listErr := os.Lstat(home)
	if err != nil || listErr != nil || !os.SameFile(top, listed) || listed.Mode() != os.ModeDir|0700 {
		return nil, ErrUpgradeCheck
	}
	result := map[string]stoppedInput{".": {info: top}}
	for name, limit := range map[string]int64{configFile: 64 << 10, genesisFile: 2 << 20, keyFile: 4096,
		stateFile: 16 << 10, peerFile: 4096, signingGuardFile: 2048, identityFile: 32 << 10} {
		_, err := root.Lstat(name)
		if name == identityFile && os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, ErrUpgradeCheck
		}
		input, err := readStoppedFile(root, name, limit)
		if err != nil {
			return nil, ErrUpgradeCheck
		}
		result[name] = input
	}
	return result, nil
}

func sameStoppedFile(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}

func sameStoppedInputs(a, b map[string]stoppedInput) bool {
	if len(a) != len(b) {
		return false
	}
	for name, first := range a {
		last, ok := b[name]
		if !ok || first.hash != last.hash || !sameStoppedFile(first.info, last.info) {
			return false
		}
	}
	return true
}
