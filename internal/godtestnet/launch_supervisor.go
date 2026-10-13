//go:build go1.25

package godtestnet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ETKINGDOM/GOD-Chain/internal/goddeploy"
)

var ErrLaunchSupervisor = errors.New("stopped private candidate service check rejected; no service installed")

type LaunchServiceCheckOptions struct {
	Home, SignerGuard, BinaryPath, BinaryPin, TargetArch string
	Inputs                                               LaunchRehearsalInputs
}

// Point-in-time stopped checks only; no DB, listener, signature or installation.
type LaunchServiceCheckReport struct {
	Scope                           string `json:"scope"`
	Synthetic                       bool   `json:"synthetic"`
	CandidateMatched                bool   `json:"candidateMatched"`
	SigningGuardMatched             bool   `json:"signingGuardMatched"`
	BinaryMatched                   bool   `json:"binaryMatched"`
	UnitStaged                      bool   `json:"unitStaged"`
	UnitSHA256                      string `json:"unitSHA256,omitempty"`
	ServicesInstalled               bool   `json:"servicesInstalled"`
	NodeStarted                     bool   `json:"nodeStarted"`
	ResourceLimitsApplied           bool   `json:"resourceLimitsApplied"`
	SupervisorCompatibilityVerified bool   `json:"supervisorCompatibilityVerified"`
	ProductionStartEnabled          bool   `json:"productionStartEnabled"`
	RealAssets                      bool   `json:"realAssets"`
}

func launchSupervisorPaths(paths ...string) bool {
	resolved := make([]string, 0, len(paths))
	for _, p := range paths {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p || p == string(filepath.Separator) {
			return false
		}
		physical, err := filepath.EvalSymlinks(p)
		if err != nil {
			return false
		}
		resolved = append(resolved, physical)
	}
	for i, p := range paths {
		for j := 0; j < i; j++ {
			if !separateGuard(p, paths[j]) || !separateGuard(resolved[i], resolved[j]) {
				return false
			}
		}
	}
	return true
}

func stoppedLaunchInputs(r *os.Root, home string) (map[string]stoppedInput, error) {
	if !launchRootMatches(r, home) {
		return nil, ErrLaunchSupervisor
	}
	result := map[string]stoppedInput{}
	for name, limit := range map[string]int64{launchRehearsalFile: 16 << 10, signingGuardFile: 2048,
		keyFile: 4096, stateFile: 16 << 10, peerFile: 4096} {
		input, err := readStoppedFile(r, name, limit)
		if err != nil {
			return nil, ErrLaunchSupervisor
		}
		result[name] = input
	}
	return result, nil
}

// CheckPrivateLaunchService holds the existing home/key-wide leases during
// exact candidate, binary and original signer/witness checks. No file is
// created or repaired; startup must independently check again after this call.
func CheckPrivateLaunchService(ctx context.Context, o LaunchServiceCheckOptions) (LaunchServiceCheckReport, error) {
	return withPrivateLaunchService(ctx, o, nil)
}

func withPrivateLaunchService(ctx context.Context, o LaunchServiceCheckOptions, inspect func() error) (report LaunchServiceCheckReport, result error) {
	defer func() {
		if recover() != nil || result != nil {
			report, result = LaunchServiceCheckReport{}, ErrLaunchSupervisor
		}
	}()
	if ctx == nil || ctx.Err() != nil || !launchSupervisorPaths(o.Home, o.SignerGuard, o.BinaryPath, o.Inputs.InputPath, o.Inputs.CandidatePath) {
		return report, ErrLaunchSupervisor
	}
	r, err := openLaunchRoot(o.Home)
	if err != nil {
		return report, ErrLaunchSupervisor
	}
	defer func() {
		if r.Close() != nil {
			report, result = LaunchServiceCheckReport{}, ErrLaunchSupervisor
		}
	}()
	lease, err := acquireExistingNodeLease(r)
	if err != nil {
		return report, ErrLaunchSupervisor
	}
	defer func() {
		if lease.Close() != nil {
			report, result = LaunchServiceCheckReport{}, ErrLaunchSupervisor
		}
	}()
	before, err := stoppedLaunchInputs(r, o.Home)
	if err != nil {
		return report, ErrLaunchSupervisor
	}
	l, err := loadLaunchHome(ctx, r, o.Home, o.Inputs)
	if err != nil {
		return report, ErrLaunchSupervisor
	}
	g, err := openNodeSigningGuard(r, o.Home, o.SignerGuard, l)
	if err != nil || g == nil {
		return report, ErrLaunchSupervisor
	}
	defer func() {
		if g.Close() != nil {
			report, result = LaunchServiceCheckReport{}, ErrLaunchSupervisor
		}
	}()
	if g.Check() != nil || checkLaunchServiceInputs(ctx, r, o.Home, o.SignerGuard, o.Inputs, l) != nil ||
		goddeploy.CheckCandidateServiceBinary(o.BinaryPath, o.BinaryPin, o.TargetArch) != nil {
		return report, ErrLaunchSupervisor
	}
	if inspect != nil && inspect() != nil {
		return report, ErrLaunchSupervisor
	}
	after, err := stoppedLaunchInputs(r, o.Home)
	if err != nil || !sameStoppedInputs(before, after) || g.Check() != nil ||
		checkLaunchServiceInputs(ctx, r, o.Home, o.SignerGuard, o.Inputs, l) != nil ||
		goddeploy.CheckCandidateServiceBinary(o.BinaryPath, o.BinaryPin, o.TargetArch) != nil || ctx.Err() != nil {
		return report, ErrLaunchSupervisor
	}
	return LaunchServiceCheckReport{Scope: "stopped-private-synthetic-candidate", Synthetic: true,
		CandidateMatched: true, SigningGuardMatched: true, BinaryMatched: true}, nil
}

// PrepareLaunchSupervisor stages one new PRIVATE unit file under both leases.
// The executing owner must equal the explicit non-root service UID/GID. It
// installs nothing, invokes no shell/systemd and never overwrites output.
// Interrupted/failed staging is retained for review, not treated as usable.
func PrepareLaunchSupervisor(ctx context.Context, o goddeploy.LaunchSupervisorOptions, output string) (LaunchServiceCheckReport, error) {
	unit, err := goddeploy.RenderLaunchSupervisor(o)
	if err != nil || os.Geteuid() == 0 || o.UserID != strconv.Itoa(os.Geteuid()) || o.GroupID != strconv.Itoa(os.Getegid()) ||
		!filepath.IsAbs(output) || filepath.Clean(output) != output || filepath.Base(output) == "." {
		return LaunchServiceCheckReport{}, ErrLaunchSupervisor
	}
	for _, p := range []string{o.Home, o.SignerGuard, o.BinaryPath, o.Input, o.Candidate} {
		if !separateGuard(p, output) {
			return LaunchServiceCheckReport{}, ErrLaunchSupervisor
		}
	}
	parent, err := openLaunchRoot(filepath.Dir(output))
	if err != nil {
		return LaunchServiceCheckReport{}, ErrLaunchSupervisor
	}
	defer parent.Close()
	if _, err = parent.Lstat(filepath.Base(output)); !os.IsNotExist(err) {
		return LaunchServiceCheckReport{}, ErrLaunchSupervisor
	}
	// The new file cannot be an alias nested under writable signing data.
	physical, err := filepath.EvalSymlinks(filepath.Dir(output))
	if err != nil {
		return LaunchServiceCheckReport{}, ErrLaunchSupervisor
	}
	for _, p := range []string{o.Home, o.SignerGuard} {
		actual, e := filepath.EvalSymlinks(p)
		if e != nil || !separateGuard(actual, filepath.Join(physical, filepath.Base(output))) {
			return LaunchServiceCheckReport{}, ErrLaunchSupervisor
		}
	}
	// The same unit schema must hold for physical targets, including aliases
	// into a namespace-hidden tmp or kernel tree. Render still uses original
	// reviewed paths, never silently substitutes physical paths in the unit.
	physicalOptions := o
	for target, original := range map[*string]string{&physicalOptions.BinaryPath: o.BinaryPath, &physicalOptions.Home: o.Home,
		&physicalOptions.SignerGuard: o.SignerGuard, &physicalOptions.Input: o.Input, &physicalOptions.Candidate: o.Candidate} {
		actual, e := filepath.EvalSymlinks(original)
		if e != nil {
			return LaunchServiceCheckReport{}, ErrLaunchSupervisor
		}
		*target = actual
	}
	if goddeploy.ValidateLaunchSupervisorOptions(physicalOptions) != nil {
		return LaunchServiceCheckReport{}, ErrLaunchSupervisor
	}
	inputPin, err := hex.DecodeString(o.InputPin)
	candidatePin, e := hex.DecodeString(o.CandidatePin)
	if err != nil || e != nil || len(inputPin) != 32 || len(candidatePin) != 32 {
		return LaunchServiceCheckReport{}, ErrLaunchSupervisor
	}
	check := LaunchServiceCheckOptions{Home: o.Home, SignerGuard: o.SignerGuard, BinaryPath: o.BinaryPath,
		BinaryPin: o.BinaryPin, TargetArch: o.TargetArch, Inputs: LaunchRehearsalInputs{InputPath: o.Input,
			CandidatePath: o.Candidate, InputPin: [32]byte(inputPin), CandidatePin: [32]byte(candidatePin)}}
	report, err := withPrivateLaunchService(ctx, check, func() error {
		if ctx.Err() != nil || !launchRootMatches(parent, filepath.Dir(output)) || writePrivate(parent, filepath.Base(output), unit) != nil {
			return ErrLaunchSupervisor
		}
		got, e := readLaunchFile(parent, filepath.Base(output), 32<<10)
		if e != nil || sha256.Sum256(got) != sha256.Sum256(unit) {
			return ErrLaunchSupervisor
		}
		return nil
	})
	if err != nil {
		return LaunchServiceCheckReport{}, ErrLaunchSupervisor
	}
	report.UnitStaged = true
	hash := sha256.Sum256(unit)
	report.UnitSHA256 = hex.EncodeToString(hash[:])
	return report, nil
}
