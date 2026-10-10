//go:build go1.25

package goddeploy

import (
	"context"
	"errors"
	"math"
	"math/big"
	"path/filepath"
	"strconv"
	"time"
)

var ErrHostObserve = errors.New("finite synthetic host observation did not pass")

type HostObserveOptions struct {
	DataDir                              string
	PIDs                                 []int
	Samples                              int
	Interval, Budget                     time.Duration
	MinimumFreeBytes, MinimumMemoryBytes uint64
	MaximumRSSBytes, MaximumOpenFiles    uint64
}

// Aggregates only: no PID, process name/start time, path, command line, key or
// environment appears in reports. RSS can double-count shared pages and is an
// approximate kernel counter, not unique memory or cgroup-limit headroom.
type HostObserveSample struct {
	Kind                     string `json:"kind"`
	Sample                   int    `json:"sample"`
	ElapsedMilliseconds      int64  `json:"elapsedMilliseconds"`
	ObservationPassed        bool   `json:"observationPassed"`
	Reason                   string `json:"reason"`
	AvailableDiskBytes       string `json:"availableDiskBytes,omitempty"`
	AvailableMemoryBytes     string `json:"availableMemoryBytes,omitempty"`
	SelectedRSSBytesApprox   string `json:"selectedRSSBytesApprox,omitempty"`
	SelectedOpenFilesApprox  string `json:"selectedOpenFilesApprox,omitempty"`
	HostNonIdlePermille      uint64 `json:"hostNonIdlePermille"`
	SelectedCPUSharePermille uint64 `json:"selectedCPUSharePermille"`
}

type HostObserveReport struct {
	Kind                            string `json:"kind"`
	Scope                           string `json:"scope"`
	Synthetic                       bool   `json:"synthetic"`
	RealAssets                      bool   `json:"realAssets"`
	PublicAcceptance                bool   `json:"publicAcceptance"`
	HardwareCapacityVerified        bool   `json:"hardwareCapacityVerified"`
	ServicesChanged                 bool   `json:"servicesChanged"`
	ChecksPassed                    bool   `json:"checksPassed"`
	Reason                          string `json:"reason"`
	SelectedProcesses               int    `json:"selectedProcesses"`
	SamplesRequested                int    `json:"samplesRequested"`
	SamplesAttempted                int    `json:"samplesAttempted"`
	SamplesPassed                   int    `json:"samplesPassed"`
	SnapshotsAttempted              int    `json:"snapshotsAttempted"`
	ElapsedMilliseconds             int64  `json:"elapsedMilliseconds"`
	MinimumAvailableDiskBytes       string `json:"minimumAvailableDiskBytes,omitempty"`
	MinimumAvailableMemoryBytes     string `json:"minimumAvailableMemoryBytes,omitempty"`
	PeakSelectedRSSBytesApprox      string `json:"peakSelectedRSSBytesApprox,omitempty"`
	PeakSelectedOpenFilesApprox     string `json:"peakSelectedOpenFilesApprox,omitempty"`
	MaximumHostNonIdlePermille      uint64 `json:"maximumHostNonIdlePermille"`
	MaximumSelectedCPUSharePermille uint64 `json:"maximumSelectedCPUSharePermille"`
}

type observedProcess struct{ start, user, system, rss, files uint64 }
type hostObservationState struct {
	cpu                       [8]uint64
	cpuIDs                    [64]uint64
	logicalCPUs               int
	totalMemory, memory, disk uint64
	processes                 map[int]observedProcess
}
type hostObservationSource interface {
	snapshot(context.Context) (hostObservationState, error)
	close() error
}

func hostObserveOptionsOK(o HostObserveOptions) bool {
	if !filepath.IsAbs(o.DataDir) || filepath.Clean(o.DataDir) != o.DataDir || o.DataDir == string(filepath.Separator) ||
		len(o.PIDs) < 1 || len(o.PIDs) > 32 || o.Samples < 2 || o.Samples > 60 ||
		o.Interval < 250*time.Millisecond || o.Interval > 5*time.Second || o.Budget < 2*time.Second || o.Budget > 180*time.Second ||
		o.MinimumFreeBytes < 1<<20 || o.MinimumFreeBytes > 1<<50 || o.MinimumMemoryBytes < 1<<20 || o.MinimumMemoryBytes > 1<<50 ||
		o.MaximumRSSBytes < 1<<20 || o.MaximumRSSBytes > 1<<50 || o.MaximumOpenFiles < 64 || o.MaximumOpenFiles > 65536 {
		return false
	}
	seen := map[int]bool{}
	for _, pid := range o.PIDs {
		if pid < 2 || pid > math.MaxInt32 || seen[pid] {
			return false
		}
		seen[pid] = true
	}
	return time.Duration(o.Samples)*o.Interval+time.Second <= o.Budget
}

func newHostObserveReport(o HostObserveOptions) HostObserveReport {
	return HostObserveReport{Kind: "host-observe-summary", Scope: "procfs-view-not-cgroup-capacity", Synthetic: true,
		Reason: "configuration", SelectedProcesses: len(o.PIDs), SamplesRequested: o.Samples}
}

// HostObserve is an opt-in finite foreground reader. Only the fixed Linux
// procfs metrics and filesystem space of one private directory are inspected.
// It never scans directory contents, reads argv/environ/keys, sends a signal,
// changes limits, invokes a shell, uses network or restarts/replaces a service.
// Cancellation is checked between bounded reads; supervise the process with an
// external deadline when a hard syscall timeout is required.
func HostObserve(ctx context.Context, o HostObserveOptions, emit func(HostObserveSample) error) (r HostObserveReport, err error) {
	r = newHostObserveReport(o)
	if ctx == nil || emit == nil || !hostObserveOptionsOK(o) {
		return r, ErrHostObserve
	}
	o.PIDs = append([]int(nil), o.PIDs...)
	if ctx.Err() != nil {
		r.Reason = "cancelled"
		return r, ErrHostObserve
	}
	ctx, cancel := context.WithTimeout(ctx, o.Budget)
	defer cancel()
	source, reason, err := openHostObservation(o)
	if err != nil {
		r.Reason = reason
		return r, ErrHostObserve
	}
	defer func() {
		if source.close() != nil {
			r.ChecksPassed, r.Reason, err = false, "reader-close", ErrHostObserve
		}
	}()
	return hostObserveLoop(ctx, o, source.snapshot, time.Now, hostObserveWait, emit)
}

func hostObserveWait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func hostObserveCancelled(ctx context.Context) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "budget-exhausted"
	}
	return "cancelled"
}

func checkedHostAdd(a, b uint64) (uint64, error) {
	if a > math.MaxUint64-b {
		return 0, ErrHostObserve
	}
	return a + b, nil
}

func hostObservationTotals(s hostObservationState, o HostObserveOptions) (rss, files uint64, reason string) {
	if s.logicalCPUs < 1 || s.logicalCPUs > 4096 || s.totalMemory == 0 || s.memory > s.totalMemory || len(s.processes) != len(o.PIDs) {
		return 0, 0, "invalid-metrics"
	}
	for _, pid := range o.PIDs {
		p, ok := s.processes[pid]
		if !ok || p.start == 0 {
			return 0, 0, "process-unavailable"
		}
		var err error
		rss, err = checkedHostAdd(rss, p.rss)
		if err != nil {
			return 0, 0, "counter-overflow"
		}
		files, err = checkedHostAdd(files, p.files)
		if err != nil || files > 65536 {
			return 0, 0, "file-count-bound"
		}
	}
	return rss, files, ""
}

func hostObserveBudget(s hostObservationState, rss, files uint64, o HostObserveOptions) string {
	switch {
	case s.disk < o.MinimumFreeBytes:
		return "disk-budget"
	case s.memory < o.MinimumMemoryBytes:
		return "memory-budget"
	case rss > o.MaximumRSSBytes:
		return "rss-budget"
	case files > o.MaximumOpenFiles:
		return "file-handle-budget"
	}
	return ""
}

// Ratios are rounded down to permille of the complete procfs CPU view, not of
// one CPU. No guessed CLK_TCK or floating-point conversion is needed. Guests
// are already included in user/nice counters and are not summed a second time.
func hostCPUShare(part, total uint64) (uint64, error) {
	if total == 0 || part > total {
		return 0, ErrHostObserve
	}
	n := new(big.Int).Mul(new(big.Int).SetUint64(part), big.NewInt(1000))
	return n.Quo(n, new(big.Int).SetUint64(total)).Uint64(), nil
}

func hostObservationDelta(before, after hostObservationState, o HostObserveOptions) (host, selected uint64, reason string) {
	if before.logicalCPUs != after.logicalCPUs || before.cpuIDs != after.cpuIDs || before.totalMemory != after.totalMemory {
		return 0, 0, "metric-scope-changed"
	}
	var total, idle, used uint64
	for i, value := range after.cpu {
		if value < before.cpu[i] {
			return 0, 0, "counter-regression"
		}
		delta := value - before.cpu[i]
		var err error
		total, err = checkedHostAdd(total, delta)
		if err != nil {
			return 0, 0, "counter-overflow"
		}
		if i == 3 || i == 4 {
			idle, err = checkedHostAdd(idle, delta)
			if err != nil {
				return 0, 0, "counter-overflow"
			}
		}
	}
	for _, pid := range o.PIDs {
		a, b := before.processes[pid], after.processes[pid]
		if a.start != b.start {
			return 0, 0, "process-identity-changed"
		}
		if b.user < a.user || b.system < a.system {
			return 0, 0, "counter-regression"
		}
		for _, d := range []uint64{b.user - a.user, b.system - a.system} {
			var err error
			used, err = checkedHostAdd(used, d)
			if err != nil {
				return 0, 0, "counter-overflow"
			}
		}
	}
	var err, selectedErr error
	host, err = hostCPUShare(total-idle, total)
	selected, selectedErr = hostCPUShare(used, total)
	if err != nil || selectedErr != nil {
		return 0, 0, "counter-inconsistent"
	}
	return host, selected, ""
}

// Internal seams are for deterministic fault tests, not exported source/clock
// overrides. Public observation uses the real kernel, elapsed time and waits.
func hostObserveLoop(ctx context.Context, o HostObserveOptions,
	poll func(context.Context) (hostObservationState, error), now func() time.Time,
	wait func(context.Context, time.Duration) error, emit func(HostObserveSample) error) (HostObserveReport, error) {
	r := newHostObserveReport(o)
	started := now()
	finish := func(reason string) (HostObserveReport, error) {
		r.Reason = reason
		r.ElapsedMilliseconds = now().Sub(started).Milliseconds()
		return r, ErrHostObserve
	}
	read := func() (hostObservationState, uint64, uint64, string) {
		if ctx.Err() != nil || now().Sub(started) >= o.Budget {
			if ctx.Err() == nil {
				return hostObservationState{}, 0, 0, "budget-exhausted"
			}
			return hostObservationState{}, 0, 0, hostObserveCancelled(ctx)
		}
		r.SnapshotsAttempted++
		s, err := poll(ctx)
		if ctx.Err() != nil {
			return hostObservationState{}, 0, 0, hostObserveCancelled(ctx)
		}
		if now().Sub(started) >= o.Budget {
			return hostObservationState{}, 0, 0, "budget-exhausted"
		}
		if err != nil {
			return hostObservationState{}, 0, 0, "metrics-unavailable"
		}
		rss, files, reason := hostObservationTotals(s, o)
		return s, rss, files, reason
	}
	previous, rss, files, reason := read()
	if reason != "" {
		return finish(reason)
	}
	if reason = hostObserveBudget(previous, rss, files, o); reason != "" {
		return finish(reason)
	}
	minDisk, minMemory, maxRSS, maxFiles := previous.disk, previous.memory, rss, files
	for i := 1; i <= o.Samples; i++ {
		if wait(ctx, o.Interval) != nil || ctx.Err() != nil {
			return finish(hostObserveCancelled(ctx))
		}
		r.SamplesAttempted++
		current, rss, files, reason := read()
		sample := HostObserveSample{Kind: "host-observe-sample", Sample: i, ElapsedMilliseconds: now().Sub(started).Milliseconds(), Reason: reason}
		if reason == "" {
			sample.AvailableDiskBytes, sample.AvailableMemoryBytes = strconv.FormatUint(current.disk, 10), strconv.FormatUint(current.memory, 10)
			sample.SelectedRSSBytesApprox, sample.SelectedOpenFilesApprox = strconv.FormatUint(rss, 10), strconv.FormatUint(files, 10)
			sample.HostNonIdlePermille, sample.SelectedCPUSharePermille, reason = hostObservationDelta(previous, current, o)
			if reason == "" {
				reason = hostObserveBudget(current, rss, files, o)
			}
			minDisk, minMemory, maxRSS, maxFiles = min(minDisk, current.disk), min(minMemory, current.memory), max(maxRSS, rss), max(maxFiles, files)
			r.MinimumAvailableDiskBytes, r.MinimumAvailableMemoryBytes = strconv.FormatUint(minDisk, 10), strconv.FormatUint(minMemory, 10)
			r.PeakSelectedRSSBytesApprox, r.PeakSelectedOpenFilesApprox = strconv.FormatUint(maxRSS, 10), strconv.FormatUint(maxFiles, 10)
			if reason == "" {
				r.MaximumHostNonIdlePermille = max(r.MaximumHostNonIdlePermille, sample.HostNonIdlePermille)
				r.MaximumSelectedCPUSharePermille = max(r.MaximumSelectedCPUSharePermille, sample.SelectedCPUSharePermille)
				sample.ObservationPassed, sample.Reason = true, "sample-budgets-passed"
				r.SamplesPassed++
			} else {
				sample.Reason = reason
			}
		}
		if emit(sample) != nil {
			return finish("report-unavailable")
		}
		if ctx.Err() != nil {
			return finish(hostObserveCancelled(ctx))
		}
		if now().Sub(started) >= o.Budget {
			return finish("budget-exhausted")
		}
		if reason != "" {
			return finish(reason)
		}
		previous = current
	}
	r.ChecksPassed, r.Reason, r.ElapsedMilliseconds = true, "finite-observation-passed", now().Sub(started).Milliseconds()
	return r, nil
}
