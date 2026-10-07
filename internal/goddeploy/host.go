//go:build go1.25

package goddeploy

import (
	"errors"
	"math"
	"runtime"
	"strconv"
)

var ErrHost = errors.New("private synthetic host preflight did not pass")

type HostOptions struct {
	DataDir          string
	MinimumFreeBytes uint64
	MinimumOpenFiles uint64
}

// This is a read-only budget snapshot, not a hardware benchmark, filesystem
// write test, Linux runtime acceptance or authorization to install a service.
type HostReport struct {
	Synthetic           bool   `json:"synthetic"`
	RealAssets          bool   `json:"realAssets"`
	PublicAcceptance    bool   `json:"publicAcceptance"`
	PreflightPassed     bool   `json:"preflightPassed"`
	Reason              string `json:"reason"`
	OperatingSystem     string `json:"operatingSystem"`
	Architecture        string `json:"architecture"`
	LinuxHost           bool   `json:"linuxHost"`
	Unprivileged        bool   `json:"unprivileged"`
	LogicalCPUs         int    `json:"logicalCPUs"`
	AvailableBytes      string `json:"availableBytes,omitempty"`
	OpenFilesSoftLimit  string `json:"openFilesSoftLimit,omitempty"`
	MinimumFreeBytes    string `json:"minimumFreeBytes,omitempty"`
	MinimumOpenFiles    string `json:"minimumOpenFiles,omitempty"`
	WriteAccessVerified bool   `json:"writeAccessVerified"`
}

type hostMetrics struct{ available, openFiles uint64 }

func hostCapacity(blocks, unit uint64) (uint64, error) {
	if unit == 0 || blocks > math.MaxUint64/unit {
		return 0, ErrHost
	}
	return blocks * unit, nil
}

func hostLinuxUnit(blockSize, fragmentSize int64) (uint64, error) {
	if blockSize <= 0 || fragmentSize < 0 {
		return 0, ErrHost
	}
	if fragmentSize > 0 {
		return uint64(fragmentSize), nil
	}
	return uint64(blockSize), nil
}

func evaluateHost(r HostReport, m hostMetrics, o HostOptions) (HostReport, error) {
	r.AvailableBytes = strconv.FormatUint(m.available, 10)
	r.OpenFilesSoftLimit = strconv.FormatUint(m.openFiles, 10)
	r.MinimumFreeBytes = strconv.FormatUint(o.MinimumFreeBytes, 10)
	r.MinimumOpenFiles = strconv.FormatUint(o.MinimumOpenFiles, 10)
	switch {
	case !r.Unprivileged:
		r.Reason = "privileged-user"
	case m.available < o.MinimumFreeBytes:
		r.Reason = "disk-budget"
	case m.openFiles < o.MinimumOpenFiles:
		r.Reason = "file-handle-budget"
	default:
		r.PreflightPassed, r.Reason = true, "budget-snapshot-passed"
		return r, nil
	}
	return r, ErrHost
}

// Host checks an existing owner-only directory without reading its contents,
// creating files, changing limits or invoking network/service-management tools.
// Operators supply both budgets explicitly; no minimum hardware is invented.
func Host(o HostOptions) (HostReport, error) {
	r := HostReport{Synthetic: true, Reason: "configuration", OperatingSystem: runtime.GOOS, Architecture: runtime.GOARCH, LinuxHost: runtime.GOOS == "linux", LogicalCPUs: runtime.NumCPU()}
	if o.MinimumFreeBytes < 1<<20 || o.MinimumFreeBytes > 1<<50 || o.MinimumOpenFiles < 64 || o.MinimumOpenFiles > 1<<20 {
		return r, ErrHost
	}
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		r.Reason = "unsupported-runtime"
		return r, ErrHost
	}
	workspace, err := root(o.DataDir, true)
	if err != nil {
		r.Reason = "private-directory"
		return r, ErrHost
	}
	defer workspace.Close()
	dir, err := workspace.Open(".")
	if err != nil {
		r.Reason = "private-directory"
		return r, ErrHost
	}
	defer dir.Close()
	m, unprivileged, err := readHostMetrics(dir)
	if err != nil {
		r.Reason = "host-metrics"
		return r, ErrHost
	}
	r.Unprivileged = unprivileged
	return evaluateHost(r, m, o)
}
