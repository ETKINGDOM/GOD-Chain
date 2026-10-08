//go:build go1.25

package godtestnet

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godcompanion"
	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
)

var ErrSmoke = errors.New("synthetic service smoke check did not pass")

type SmokeOptions struct {
	BundlePath, ExpectedBundle, RPC, Companion string
	Samples, MinimumPeers                      int
	Interval, MaxBlockAge                      time.Duration
}

type SmokeReport struct {
	Synthetic                bool   `json:"synthetic"`
	RealAssets               bool   `json:"realAssets"`
	PublicAcceptance         bool   `json:"publicAcceptance"`
	ChecksPassed             bool   `json:"checksPassed"`
	Reason                   string `json:"reason"`
	SamplesPassed            int    `json:"samplesPassed"`
	FirstHeight              string `json:"firstHeight,omitempty"`
	LastHeight               string `json:"lastHeight,omitempty"`
	BlockProgressObserved    bool   `json:"blockProgressObserved"`
	CompanionResourcesPassed int    `json:"companionResourcesPassed"`
	BrowserOriginPassed      bool   `json:"browserOriginPassed"`
	OpaqueOriginRejected     bool   `json:"opaqueOriginRejected"`
	BrowserExecutionVerified bool   `json:"browserExecutionVerified"`
	TransactionsSubmitted    bool   `json:"transactionsSubmitted"`
}

func headerTokens(h http.Header, name string) []string {
	var result []string
	for _, line := range h.Values(name) {
		for _, token := range strings.Split(line, ",") {
			result = append(result, strings.ToLower(strings.TrimSpace(token)))
		}
	}
	return result
}

func smokeCORS(h http.Header, origin string) bool {
	if len(h.Values("Access-Control-Allow-Origin")) != 1 || h.Get("Access-Control-Allow-Origin") != origin || len(h.Values("Access-Control-Allow-Credentials")) != 0 || len(h.Values("Set-Cookie")) != 0 {
		return false
	}
	headers, methods := headerTokens(h, "Access-Control-Allow-Headers"), headerTokens(h, "Access-Control-Allow-Methods")
	if len(headers) != 1 || headers[0] != "content-type" || len(methods) != 2 || !(methods[0] == "post" && methods[1] == "options" || methods[0] == "options" && methods[1] == "post") {
		return false
	}
	for _, token := range headerTokens(h, "Vary") {
		if token == "origin" {
			return true
		}
	}
	return false
}

func smokeOrigin(ctx context.Context, client *http.Client, endpoint, origin string, allowed bool) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodOptions, endpoint, nil)
	if err != nil {
		return ErrSmoke
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	response, err := client.Do(req)
	if err != nil {
		return ErrSmoke
	}
	defer response.Body.Close()
	if allowed {
		raw, err := io.ReadAll(io.LimitReader(response.Body, 1025))
		if err != nil || len(raw) != 0 || response.StatusCode != http.StatusNoContent || !smokeCORS(response.Header, origin) {
			return ErrSmoke
		}
	} else if response.StatusCode != http.StatusForbidden || len(response.Header.Values("Access-Control-Allow-Origin")) != 0 {
		return ErrSmoke
	}
	return nil
}

func smokeAssets(ctx context.Context, client *http.Client, endpoint string, r *SmokeReport) error {
	u, _ := url.Parse(endpoint)
	for _, path := range []string{"/", "/wallet.html", "/explorer.html", "/styles.css", "/client.mjs", "/query-errors.mjs", "/main.mjs", "/explorer.mjs"} {
		u.Path = path
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		req.Header.Set("Accept-Encoding", "identity")
		response, err := client.Do(req)
		if err != nil {
			return ErrSmoke
		}
		raw, readErr := io.ReadAll(io.LimitReader(response.Body, (128<<10)+1))
		_ = response.Body.Close()
		if readErr != nil || len(raw) > 128<<10 || response.StatusCode != http.StatusOK || !godcompanion.MatchesAssetResponse(path, response.Header, raw) {
			return ErrSmoke
		}
		r.CompanionResourcesPassed++
	}
	return nil
}

func smokeProgress(previous, next godnode.CommittedView) bool {
	return next.Height >= previous.Height && !next.Time.Before(previous.Time) &&
		(next.Height != previous.Height || next.AppHash == previous.AppHash && next.Time.Equal(previous.Time))
}

// Smoke never signs, submits, changes files, starts/stops a service or retries
// failures. It checks one reviewed RPC and this build's fixed static resources.
// Provider-reported continuity is not authenticated finality or independence.
func Smoke(ctx context.Context, o SmokeOptions) (SmokeReport, error) {
	r := SmokeReport{Synthetic: true, Reason: "configuration"}
	if ctx == nil || !healthEndpoint(o.RPC) || !healthEndpoint(o.Companion) || o.Samples < 2 || o.Samples > 10 || o.Interval < time.Second || o.Interval > 10*time.Second || o.MaxBlockAge < 10*time.Second || o.MaxBlockAge > 10*time.Minute || o.MinimumPeers < 0 || o.MinimumPeers > 32 {
		return r, ErrSmoke
	}
	if ctx.Err() != nil {
		r.Reason = "cancelled"
		return r, ErrSmoke
	}
	rpcURL, _ := url.Parse(o.RPC)
	companionURL, _ := url.Parse(o.Companion)
	origin := companionURL.Scheme + "://" + companionURL.Host
	if rpcURL.Scheme != companionURL.Scheme || rpcURL.Scheme+"://"+rpcURL.Host == origin {
		return r, ErrSmoke
	}
	b, err := loadBundle(o.BundlePath, o.ExpectedBundle)
	if err != nil {
		return r, ErrSmoke
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	transport := healthTransport()
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrSmoke }}
	r.Reason = "companion-resources"
	assetCtx, cancelAssets := context.WithTimeout(ctx, 8*time.Second)
	err = smokeAssets(assetCtx, client, o.Companion, &r)
	cancelAssets()
	if err != nil {
		return r, ErrSmoke
	}
	r.Reason = "browser-origin"
	if smokeOrigin(ctx, client, o.RPC, origin, true) != nil || smokeOrigin(ctx, client, o.RPC, "null", false) != nil {
		return r, ErrSmoke
	}
	r.OpaqueOriginRejected = true
	var previous godnode.CommittedView
	for i := 0; i < o.Samples; i++ {
		if i > 0 {
			timer := time.NewTimer(o.Interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				r.Reason = "cancelled"
				return r, ErrSmoke
			case <-timer.C:
			}
		}
		r.Reason = "health-sample"
		sampleCtx, cancelSample := context.WithTimeout(ctx, 8*time.Second)
		live, liveErr := healthCall(sampleCtx, client, o.RPC, "god_liveness", 1)
		if liveErr != nil {
			cancelSample()
			return r, ErrSmoke
		}
		network, networkErr := healthOriginCall(sampleCtx, client, o.RPC, "god_network", 2, origin)
		cancelSample()
		if liveErr != nil || networkErr != nil {
			return r, ErrSmoke
		}
		if _, err := evaluateHealth(&b, live, network, time.Now().UTC(), o.MaxBlockAge, o.MinimumPeers); err != nil {
			return r, ErrSmoke
		}
		r.BrowserOriginPassed = true
		var view godnode.NetworkView
		if json.Unmarshal(network, &view) != nil {
			return r, ErrSmoke
		}
		if i > 0 && !smokeProgress(previous, view.Commit) {
			r.Reason = "committed-view-regression"
			return r, ErrSmoke
		}
		previous = view.Commit
		r.LastHeight = strconv.FormatInt(view.Commit.Height, 10)
		if i == 0 {
			r.FirstHeight = r.LastHeight
		}
		r.SamplesPassed++
	}
	r.BlockProgressObserved = r.LastHeight != r.FirstHeight
	if !r.BlockProgressObserved {
		r.Reason = "no-block-progress"
		return r, ErrSmoke
	}
	r.ChecksPassed, r.Reason = true, "service-samples-passed"
	return r, nil
}
