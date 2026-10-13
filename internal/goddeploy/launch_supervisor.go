//go:build go1.25

package goddeploy

import (
	"debug/buildinfo"
	"debug/elf"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var ErrLaunchSupervisor = errors.New("private candidate supervisor preparation rejected; no service installed")

// Explicit budgets are operator inputs, not recommended capacity or economics.
// Only Home and SignerGuard may be writable in the rendered unit.
type LaunchSupervisorOptions struct {
	BinaryPath, BinaryPin, TargetArch                           string
	Home, SignerGuard, Input, InputPin, Candidate, CandidatePin string
	UserID, GroupID                                             string
	ReportSeconds, MemoryMaxBytes, CPUQuotaPercent              uint64
	TasksMax, OpenFiles, StartSeconds, StopSeconds              uint64
}

func supervisorPath(p string) bool {
	if !filepath.IsAbs(p) || filepath.Clean(p) != p || p == string(filepath.Separator) || len(p) > 2048 || strings.TrimSpace(p) != p {
		return false
	}
	// PrivateTmp and protected kernel filesystems hide these paths in the unit.
	// Require persistent, explicitly owned operational storage instead.
	for _, hidden := range []string{"/tmp", "/var/tmp", "/private/tmp", "/dev", "/proc", "/sys"} {
		if p == hidden || strings.HasPrefix(p, hidden+"/") {
			return false
		}
	}
	// Quoted systemd argv/paths, never a shell. Refuse expansion/specifiers,
	// directives, escaped quotes, non-ASCII and alternate path separators.
	for _, c := range p {
		if c != '/' && c != ' ' && c != '-' && c != '_' && c != '.' &&
			(c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func supervisorID(s string) bool {
	v, err := strconv.ParseUint(s, 10, 31)
	return err == nil && v > 0 && strconv.FormatUint(v, 10) == s
}

func supervisorSeparate(a, b string) bool {
	return a != b && !strings.HasPrefix(a, b+string(filepath.Separator)) && !strings.HasPrefix(b, a+string(filepath.Separator))
}

func supervisorOptions(o LaunchSupervisorOptions) bool {
	paths := []string{o.BinaryPath, o.Home, o.SignerGuard, o.Input, o.Candidate}
	for i, p := range paths {
		if !supervisorPath(p) {
			return false
		}
		for _, other := range paths[:i] {
			if !supervisorSeparate(p, other) {
				return false
			}
		}
	}
	return (o.TargetArch == "amd64" || o.TargetArch == "arm64") &&
		digestOK(o.BinaryPin) && digestOK(o.InputPin) && digestOK(o.CandidatePin) &&
		supervisorID(o.UserID) && supervisorID(o.GroupID) &&
		o.ReportSeconds >= 1 && o.ReportSeconds <= 300 &&
		o.MemoryMaxBytes >= 64<<20 && o.MemoryMaxBytes <= 1<<40 &&
		o.CPUQuotaPercent >= 1 && o.CPUQuotaPercent <= 6400 &&
		o.TasksMax >= 32 && o.TasksMax <= 4096 &&
		o.OpenFiles >= 64 && o.OpenFiles <= 65536 && o.StartSeconds >= 15 && o.StartSeconds <= 300 && o.StopSeconds >= 15 && o.StopSeconds <= 300
}

// ValidateLaunchSupervisorOptions is pure schema/injection validation. It does
// not inspect node state, capacity, users, systemd or authorize installation.
func ValidateLaunchSupervisorOptions(o LaunchSupervisorOptions) error {
	if !supervisorOptions(o) {
		return ErrLaunchSupervisor
	}
	return nil
}

// CheckCandidateServiceBinary reads one explicitly pinned static Linux binary.
// It never executes it. Build metadata is compatibility, not release provenance
// or independent proof that the expected checksum was reviewed correctly.
func CheckCandidateServiceBinary(path, pin, arch string) (result error) {
	defer func() {
		if recover() != nil {
			result = ErrLaunchSupervisor
		}
	}()
	if !supervisorPath(path) || !digestOK(pin) || arch != "amd64" && arch != "arm64" {
		return ErrLaunchSupervisor
	}
	r, err := root(filepath.Dir(path), false)
	if err != nil {
		return ErrLaunchSupervisor
	}
	defer r.Close()
	f, size, err := regular(r, filepath.Base(path), maxBinary, false)
	if err != nil {
		return ErrLaunchSupervisor
	}
	defer f.Close()
	first, err := f.Stat()
	if err != nil || !supervisorBinaryRegular(first) || first.Mode()&os.ModeType != 0 || first.Mode().Perm()&0022 != 0 || first.Mode().Perm()&0100 == 0 ||
		first.Mode()&(os.ModeSetuid|os.ModeSetgid) != 0 {
		return ErrLaunchSupervisor
	}
	object, err := elf.NewFile(f)
	if err != nil || object.Class != elf.ELFCLASS64 || object.Data != elf.ELFDATA2LSB || object.Type != elf.ET_EXEC ||
		(arch == "amd64" && object.Machine != elf.EM_X86_64) || (arch == "arm64" && object.Machine != elf.EM_AARCH64) {
		return ErrLaunchSupervisor
	}
	info, err := buildinfo.Read(f)
	if err != nil || !buildOK(info, arch) {
		return ErrLaunchSupervisor
	}
	digest, n, err := hashReader(io.NewSectionReader(f, 0, size))
	last, statErr := f.Stat()
	listed, listErr := r.Lstat(filepath.Base(path))
	if err != nil || statErr != nil || listErr != nil || n != size || digest != pin ||
		!samePackageFile(first, last) || !samePackageFile(first, listed) || !supervisorBinaryRegular(last) || !supervisorBinaryRegular(listed) {
		return ErrLaunchSupervisor
	}
	return nil
}

// RenderLaunchSupervisor returns PRIVATE operational bytes. The caller must
// check the exact stopped candidate and binary, stage securely, and obtain
// separate installation authority. There is deliberately no Install section,
// shell, automatic restart, key generation, public RPC/peer or asset switch.
func RenderLaunchSupervisor(o LaunchSupervisorOptions) ([]byte, error) {
	if !supervisorOptions(o) {
		return nil, ErrLaunchSupervisor
	}
	q := func(p string) string { return `"` + p + `"` }
	bindings := fmt.Sprintf("--home %s --signer-guard %s --input %s --input-pin %s --candidate %s --candidate-pin %s",
		q(o.Home), q(o.SignerGuard), q(o.Input), o.InputPin, q(o.Candidate), o.CandidatePin)
	unit := fmt.Sprintf(`# Private synthetic candidate only. Manual installation is separately authorized.
# Never publish this populated file; never restore both signer progress copies.
[Unit]
Description=GOD Chain private guarded candidate
After=network.target

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=%s
ExecStartPre=%s check-launch-service %s --binary %s --binary-pin %s --arch %s
ExecStart=%s serve-launch-candidate %s --report-seconds %d
Restart=no
KillSignal=SIGTERM
KillMode=control-group
TimeoutStartSec=%ds
TimeoutStopSec=%ds
SendSIGKILL=yes
UMask=0077
NoNewPrivileges=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectSystem=strict
ProtectHome=read-only
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectKernelLogs=yes
ProtectControlGroups=yes
ProtectClock=yes
RestrictSUIDSGID=yes
RestrictNamespaces=yes
RestrictRealtime=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
IPAddressDeny=any
IPAddressAllow=localhost
CapabilityBoundingSet=
AmbientCapabilities=
ReadWritePaths=%s %s
ReadOnlyPaths=%s %s %s
LimitCORE=0
LimitNOFILE=%d
TasksMax=%d
MemoryMax=%d
CPUQuota=%d%%
StandardOutput=journal
StandardError=journal
	`, o.UserID, o.GroupID, o.Home, q(o.BinaryPath), bindings, q(o.BinaryPath), o.BinaryPin, o.TargetArch,
		q(o.BinaryPath), bindings, o.ReportSeconds, o.StartSeconds, o.StopSeconds, q(o.Home), q(o.SignerGuard),
		q(o.BinaryPath), q(o.Input), q(o.Candidate), o.OpenFiles, o.TasksMax, o.MemoryMaxBytes, o.CPUQuotaPercent)
	if len(unit) > 32<<10 {
		return nil, ErrLaunchSupervisor
	}
	return []byte(unit), nil
}
