//go:build go1.25

package godtestnet

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
)

var ErrWatch = errors.New("synthetic testnet observation did not pass")

type WatchOptions struct {
	BundlePath, ExpectedBundle, RPC string
	Samples, MinimumPeers           int
	Interval, MaxBlockAge           time.Duration
	MaximumNoProgress               time.Duration
}

// Reports contain only bounded counters and fixed reasons, never provider
// errors, endpoints, identities, hashes, account data, paths or signing keys.
type WatchSample struct {
	Kind              string       `json:"kind"`
	Sample            int          `json:"sample"`
	ElapsedSeconds    int64        `json:"elapsedSeconds"`
	ProgressObserved  bool         `json:"progressObserved"`
	ObservationPassed bool         `json:"observationPassed"`
	Reason            string       `json:"reason"`
	Health            HealthReport `json:"health"`
}

type WatchReport struct {
	Kind                  string `json:"kind"`
	Synthetic             bool   `json:"synthetic"`
	RealAssets            bool   `json:"realAssets"`
	PublicAcceptance      bool   `json:"publicAcceptance"`
	TransactionsSubmitted bool   `json:"transactionsSubmitted"`
	ChecksPassed          bool   `json:"checksPassed"`
	Reason                string `json:"reason"`
	SamplesRequested      int    `json:"samplesRequested"`
	SamplesAttempted      int    `json:"samplesAttempted"`
	SamplesPassed         int    `json:"samplesPassed"`
	ProgressEvents        int    `json:"progressEvents"`
	ElapsedSeconds        int64  `json:"elapsedSeconds"`
	FirstHeight           string `json:"firstHeight,omitempty"`
	LastHeight            string `json:"lastHeight,omitempty"`
}

func watchOptionsOK(o WatchOptions) bool {
	if !healthEndpoint(o.RPC) || o.Samples < 2 || o.Samples > 10080 || o.MinimumPeers < 0 || o.MinimumPeers > 32 ||
		o.Interval < time.Second || o.Interval > 10*time.Minute || o.MaxBlockAge < 10*time.Second || o.MaxBlockAge > 10*time.Minute ||
		o.MaximumNoProgress < 10*time.Second || o.MaximumNoProgress > 10*time.Minute || o.MaximumNoProgress < o.Interval {
		return false
	}
	// Includes the maximum eight-second RPC budget per sample. No arithmetic
	// involving unbounded caller integers/durations is performed before checks.
	return time.Duration(o.Samples)*8*time.Second+time.Duration(o.Samples-1)*o.Interval <= 7*24*time.Hour
}

func watchWait(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Watch observes only an explicitly selected, bundle-pinned synthetic RPC.
// Every sample has two fixed read methods and one eight-second total budget.
// The first fault emits a redacted sample and stops, without retries, failover,
// restart, repair, signing, submission, funding or external notifications.
// A successful finite run is provider-reported continuity, not authenticated
// finality, an uptime SLA, independent operators or public launch acceptance.
func Watch(ctx context.Context, o WatchOptions, emit func(WatchSample) error) (WatchReport, error) {
	r := WatchReport{Kind: "summary", Synthetic: true, Reason: "configuration"}
	if ctx == nil || emit == nil || !watchOptionsOK(o) {
		return r, ErrWatch
	}
	if ctx.Err() != nil {
		r.Reason = "cancelled"
		return r, ErrWatch
	}
	b, err := loadBundle(o.BundlePath, o.ExpectedBundle)
	if err != nil {
		return r, ErrWatch
	}
	ctx, cancel := context.WithTimeout(ctx, 7*24*time.Hour)
	defer cancel()
	transport := healthTransport()
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrWatch }}
	poll := func(ctx context.Context) (HealthReport, godnode.CommittedView, error) {
		ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		live, err := healthCall(ctx, client, o.RPC, "god_liveness", 1)
		if err != nil {
			return HealthReport{Synthetic: true, Reason: "unavailable"}, godnode.CommittedView{}, ErrWatch
		}
		network, err := healthCall(ctx, client, o.RPC, "god_network", 2)
		if err != nil {
			return HealthReport{Synthetic: true, Reason: "unavailable"}, godnode.CommittedView{}, ErrWatch
		}
		health, err := evaluateHealth(&b, live, network, time.Now().UTC(), o.MaxBlockAge, o.MinimumPeers)
		if err != nil {
			return health, godnode.CommittedView{}, ErrWatch
		}
		var view godnode.NetworkView
		if decode(network, &view) != nil {
			return HealthReport{Synthetic: true, Reason: "invalid-response"}, godnode.CommittedView{}, ErrWatch
		}
		return health, view.Commit, nil
	}
	return watchLoop(ctx, o, poll, time.Now, watchWait, emit)
}

// The clock/wait seams support deterministic policy tests only. Exported Watch
// always uses actual elapsed time and never changes the chain/host clock.
func watchLoop(ctx context.Context, o WatchOptions,
	poll func(context.Context) (HealthReport, godnode.CommittedView, error),
	now func() time.Time, wait func(context.Context, time.Duration) error,
	emit func(WatchSample) error) (WatchReport, error) {
	r := WatchReport{Kind: "summary", Synthetic: true, SamplesRequested: o.Samples, Reason: "cancelled"}
	started := now()
	progressAt := started
	var previous godnode.CommittedView
	for i := 0; i < o.Samples; i++ {
		if i > 0 && wait(ctx, o.Interval) != nil || ctx.Err() != nil {
			r.ElapsedSeconds = int64(now().Sub(started) / time.Second)
			return r, ErrWatch
		}
		r.SamplesAttempted++
		health, commit, err := poll(ctx)
		at := now()
		r.ElapsedSeconds = int64(at.Sub(started) / time.Second)
		s := WatchSample{Kind: "sample", Sample: i + 1, ElapsedSeconds: r.ElapsedSeconds, Health: health, Reason: health.Reason}
		if ctx.Err() != nil {
			s.Reason = "cancelled"
			err = ErrWatch
		} else if err == nil {
			if i > 0 && !smokeProgress(previous, commit) {
				s.Reason = "committed-view-regression"
				err = ErrWatch
			} else if i > 0 && commit.Height == previous.Height && at.Sub(progressAt) >= o.MaximumNoProgress {
				s.Reason = "no-block-progress"
				err = ErrWatch
			} else {
				s.ObservationPassed = true
				if i == 0 {
					r.FirstHeight = strconv.FormatInt(commit.Height, 10)
					progressAt = at
				} else if commit.Height > previous.Height {
					s.ProgressObserved = true
					r.ProgressEvents++
					progressAt = at
				}
				r.LastHeight = strconv.FormatInt(commit.Height, 10)
				previous = commit
				r.SamplesPassed++
			}
		}
		if emit(s) != nil {
			r.Reason = "report-unavailable"
			return r, ErrWatch
		}
		if err != nil {
			r.Reason = s.Reason
			return r, ErrWatch
		}
	}
	if r.ProgressEvents == 0 {
		r.Reason = "no-block-progress"
		return r, ErrWatch
	}
	r.ChecksPassed, r.Reason = true, "observation-passed"
	return r, nil
}
