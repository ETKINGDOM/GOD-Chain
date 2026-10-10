//go:build go1.25

package godtestnet

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
)

var ErrReadLoad = errors.New("bounded synthetic read-load check did not pass")

type ReadLoadOptions struct {
	BundlePath, ExpectedBundle, RPC               string
	Samples, Concurrency, MinimumPeers            int
	Interval, MaximumLatency, Budget, MaxBlockAge time.Duration
}

// Reports contain counters, fixed reasons and measured times only. They never
// contain endpoints, bundle identities, hashes, paths, account data or errors.
type ReadLoadReport struct {
	Kind                       string `json:"kind"`
	Synthetic                  bool   `json:"synthetic"`
	RealAssets                 bool   `json:"realAssets"`
	PublicAcceptance           bool   `json:"publicAcceptance"`
	TransactionsSubmitted      bool   `json:"transactionsSubmitted"`
	HardwareCapacityVerified   bool   `json:"hardwareCapacityVerified"`
	ChecksPassed               bool   `json:"checksPassed"`
	Reason                     string `json:"reason"`
	SamplesRequested           int    `json:"samplesRequested"`
	SamplesStarted             int    `json:"samplesStarted"`
	SamplesCompleted           int    `json:"samplesCompleted"`
	SamplesPassed              int    `json:"samplesPassed"`
	MaximumInFlight            int    `json:"maximumInFlight"`
	RPCCallsAttempted          int    `json:"rpcCallsAttempted"`
	ElapsedMilliseconds        int64  `json:"elapsedMilliseconds"`
	LatencySamples             int    `json:"latencySamples"`
	MinimumLatencyMilliseconds int64  `json:"minimumLatencyMilliseconds"`
	P50LatencyMilliseconds     int64  `json:"p50LatencyMilliseconds"`
	P95LatencyMilliseconds     int64  `json:"p95LatencyMilliseconds"`
	MaximumLatencyMilliseconds int64  `json:"maximumLatencyMilliseconds"`
	FirstHeight                string `json:"firstHeight,omitempty"`
	LastHeight                 string `json:"lastHeight,omitempty"`
	BlockProgressObserved      bool   `json:"blockProgressObserved"`
}

func readLoadOptionsOK(o ReadLoadOptions) bool {
	return healthEndpoint(o.RPC) && o.Samples >= 2 && o.Samples <= 64 &&
		o.Concurrency >= 1 && o.Concurrency <= 8 && o.Concurrency <= o.Samples &&
		o.MinimumPeers >= 0 && o.MinimumPeers <= 32 &&
		o.Interval >= 50*time.Millisecond && o.Interval <= 2*time.Second &&
		o.MaximumLatency >= time.Millisecond && o.MaximumLatency <= 8*time.Second &&
		o.Budget >= 10*time.Second && o.Budget <= 3*time.Minute &&
		o.MaxBlockAge >= 10*time.Second && o.MaxBlockAge <= 10*time.Minute &&
		time.Duration(o.Samples-1)*o.Interval < o.Budget
}

func readLoadContextReason(parent, current context.Context) string {
	if parent.Err() != nil {
		return "cancelled"
	}
	if current.Err() != nil {
		return "time-budget"
	}
	return "unavailable"
}

// The collector accepts out-of-order concurrent responses. Heights identify
// immutable snapshots; arrival order is not chain order. Even matching commit
// metadata must not conceal changed network fields under that same height.
type readLoadViews struct {
	mu             sync.Mutex
	first, highest godnode.CommittedView
	seen           map[int64]struct {
		commit godnode.CommittedView
		digest [32]byte
	}
}

func (v *readLoadViews) add(n godnode.NetworkView, final bool) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	n.Commit.Time = n.Commit.Time.UTC()
	if v.first.Height > 0 && !smokeProgress(v.first, n.Commit) {
		return false
	}
	if final && v.highest.Height > 0 && !smokeProgress(v.highest, n.Commit) {
		return false
	}
	encoded, err := json.Marshal(n)
	if err != nil {
		return false
	}
	digest := sha256.Sum256(encoded)
	for height, old := range v.seen {
		if height == n.Commit.Height && old.digest != digest ||
			height < n.Commit.Height && n.Commit.Time.Before(old.commit.Time) ||
			height > n.Commit.Height && n.Commit.Time.After(old.commit.Time) {
			return false
		}
	}
	if v.seen == nil {
		v.seen = make(map[int64]struct {
			commit godnode.CommittedView
			digest [32]byte
		})
		v.first = n.Commit
	}
	v.seen[n.Commit.Height] = struct {
		commit godnode.CommittedView
		digest [32]byte
	}{n.Commit, digest}
	if n.Commit.Height > v.highest.Height {
		v.highest = n.Commit
	}
	return true
}

func readLoadLatency(r *ReadLoadReport, durations []time.Duration) {
	if len(durations) == 0 {
		return
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	ms := func(d time.Duration) int64 { return int64((d + time.Millisecond - 1) / time.Millisecond) }
	r.LatencySamples = len(durations)
	r.MinimumLatencyMilliseconds = ms(durations[0])
	r.P50LatencyMilliseconds = ms(durations[(len(durations)*50+99)/100-1])
	r.P95LatencyMilliseconds = ms(durations[(len(durations)*95+99)/100-1])
	r.MaximumLatencyMilliseconds = ms(durations[len(durations)-1])
}

// ReadLoad sends only god_liveness and god_network to an explicitly reviewed
// synthetic operator RPC. A baseline, bounded concurrent samples and a final
// control have at most 2*(Samples+2) RPC attempts. There are no retries, proxy,
// redirects, cookies, write methods or node-management actions. The first fault
// cancels outstanding reads and stops dispatch; already-started work is drained.
// A pass is a short provider-reported read check, not an uptime or capacity SLA.
func ReadLoad(parent context.Context, o ReadLoadOptions) (r ReadLoadReport, result error) {
	r = ReadLoadReport{Kind: "read-load-summary", Synthetic: true, Reason: "configuration"}
	if parent == nil || !readLoadOptionsOK(o) {
		return r, ErrReadLoad
	}
	if parent.Err() != nil {
		r.Reason = "cancelled"
		return r, ErrReadLoad
	}
	b, err := loadBundle(o.BundlePath, o.ExpectedBundle)
	if err != nil {
		return r, ErrReadLoad
	}
	r.SamplesRequested = o.Samples
	started := time.Now()
	ctx, cancel := context.WithTimeout(parent, o.Budget)
	defer cancel()
	transport := healthTransport()
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrReadLoad }}
	var calls atomic.Int32
	defer func() {
		r.ElapsedMilliseconds = time.Since(started).Milliseconds()
		r.RPCCallsAttempted = int(calls.Load())
	}()
	poll := func(measured bool) (godnode.NetworkView, string, error) {
		var view godnode.NetworkView
		if ctx.Err() != nil {
			return view, readLoadContextReason(parent, ctx), ErrReadLoad
		}
		limit := 8 * time.Second
		if measured {
			limit = o.MaximumLatency
		}
		pair, stop := context.WithTimeout(ctx, limit)
		defer stop()
		failedReason := func() string {
			if measured && ctx.Err() == nil && pair.Err() != nil {
				return "latency-budget"
			}
			return readLoadContextReason(parent, ctx)
		}
		calls.Add(1)
		live, e := healthCall(pair, client, o.RPC, "god_liveness", 1)
		if e != nil {
			return view, failedReason(), ErrReadLoad
		}
		if pair.Err() != nil {
			return view, failedReason(), ErrReadLoad
		}
		calls.Add(1)
		network, e := healthCall(pair, client, o.RPC, "god_network", 2)
		if e != nil {
			return view, failedReason(), ErrReadLoad
		}
		h, e := evaluateHealth(&b, live, network, time.Now().UTC(), o.MaxBlockAge, o.MinimumPeers)
		if e != nil {
			return view, h.Reason, ErrReadLoad
		}
		if decode(network, &view) != nil {
			return view, "invalid-response", ErrReadLoad
		}
		if pair.Err() != nil {
			return view, failedReason(), ErrReadLoad
		}
		return view, "sample-passed", nil
	}
	views := &readLoadViews{}
	first, reason, e := poll(false)
	if e != nil {
		r.Reason = reason
		return r, ErrReadLoad
	}
	if !views.add(first, false) {
		r.Reason = "committed-view-conflict"
		return r, ErrReadLoad
	}
	r.FirstHeight = strconv.FormatInt(first.Commit.Height, 10)
	type sample struct {
		elapsed time.Duration
		passed  bool
	}
	completed := make(chan sample, o.Samples)
	semaphore := make(chan struct{}, o.Concurrency)
	var wg sync.WaitGroup
	var active, peak atomic.Int32
	var fault sync.Once
	failure := ""
	fail := func(reason string) { fault.Do(func() { failure = reason; cancel() }) }
dispatch:
	for i := 0; i < o.Samples; i++ {
		if i > 0 && watchWait(ctx, o.Interval) != nil {
			break
		}
		select {
		case <-ctx.Done():
			break dispatch
		case semaphore <- struct{}{}:
		}
		if ctx.Err() != nil {
			<-semaphore
			break
		}
		r.SamplesStarted++
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-semaphore }()
			n := active.Add(1)
			for old := peak.Load(); n > old; old = peak.Load() {
				if peak.CompareAndSwap(old, n) {
					break
				}
			}
			defer active.Add(-1)
			at := time.Now()
			view, why, e := poll(true)
			elapsed := time.Since(at)
			if e == nil && elapsed > o.MaximumLatency {
				why, e = "latency-budget", ErrReadLoad
			}
			if e == nil && !views.add(view, false) {
				why, e = "committed-view-conflict", ErrReadLoad
			}
			if e != nil {
				fail(why)
			}
			completed <- sample{elapsed, e == nil}
		}()
	}
	wg.Wait()
	close(completed)
	var latencies []time.Duration
	for s := range completed {
		r.SamplesCompleted++
		latencies = append(latencies, s.elapsed)
		if s.passed {
			r.SamplesPassed++
		}
	}
	r.MaximumInFlight = int(peak.Load())
	readLoadLatency(&r, latencies)
	if failure != "" {
		r.Reason = failure
		return r, ErrReadLoad
	}
	if ctx.Err() != nil || r.SamplesStarted != o.Samples {
		r.Reason = readLoadContextReason(parent, ctx)
		return r, ErrReadLoad
	}
	last, reason, e := poll(false)
	if e != nil {
		r.Reason = reason
		return r, ErrReadLoad
	}
	if !views.add(last, true) {
		r.Reason = "committed-view-conflict"
		return r, ErrReadLoad
	}
	r.LastHeight = strconv.FormatInt(last.Commit.Height, 10)
	r.BlockProgressObserved = last.Commit.Height > first.Commit.Height
	if !r.BlockProgressObserved {
		r.Reason = "no-block-progress"
		return r, ErrReadLoad
	}
	if ctx.Err() != nil {
		r.Reason = readLoadContextReason(parent, ctx)
		return r, ErrReadLoad
	}
	r.ChecksPassed, r.Reason = true, "bounded-reads-passed"
	return r, nil
}
