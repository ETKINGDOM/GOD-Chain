//go:build go1.25

package godtestnet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const maxBundleBytes = 2 << 20

var ErrBundleFetch = errors.New("synthetic testnet configuration download rejected")

// This report contains no URL, path, chain identity, key, participant or digest.
// Downloading authenticated bytes does not authorize peers or start a node.
type BundleFetchReport struct {
	Synthetic           bool `json:"synthetic"`
	RealAssets          bool `json:"realAssets"`
	Downloaded          bool `json:"downloaded"`
	BundleVerified      bool `json:"bundleVerified"`
	Bytes               int  `json:"bytes"`
	ValidatorCount      int  `json:"validatorCount"`
	NodeInitialized     bool `json:"nodeInitialized"`
	NodeStarted         bool `json:"nodeStarted"`
	PublicAcceptance    bool `json:"publicAcceptance"`
	RequireNodeKeyProof bool `json:"requireNodeKeyProof"`
}

func bundlePinOK(pin string) bool {
	raw, err := hex.DecodeString(pin)
	return err == nil && len(raw) == 32 && hex.EncodeToString(raw) == pin
}

func validatedBundle(raw []byte, pin string) (Bundle, error) {
	var b Bundle
	if !bundlePinOK(pin) || len(raw) == 0 || len(raw) > maxBundleBytes {
		return b, ErrConfig
	}
	hash := sha256.Sum256(raw)
	if hex.EncodeToString(hash[:]) != pin || decode(raw, &b) != nil {
		return Bundle{}, ErrConfig
	}
	if _, _, err := validateBundle(&b); err != nil {
		return Bundle{}, ErrConfig
	}
	return b, nil
}

func bundleDownloadURL(text string) bool {
	if len(text) > 2048 || strings.Contains(text, "#") {
		return false
	}
	u, err := url.Parse(text)
	if err != nil || u.RawPath != "" || path.Clean(u.Path) != u.Path || path.Base(u.Path) != bundleFile {
		return false
	}
	for _, segment := range strings.Split(strings.TrimPrefix(u.Path, "/"), "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		for _, ch := range segment {
			if !(ch >= 'a' && ch <= 'z') && !(ch >= 'A' && ch <= 'Z') && !(ch >= '0' && ch <= '9') && ch != '-' && ch != '_' && ch != '.' {
				return false
			}
		}
	}
	// Reuse the strict root URL policy: HTTPS or numeric-loopback-only HTTP,
	// no userinfo, queries/fragments, unsafe ports or special network targets.
	u.Path = ""
	return healthEndpoint(u.String())
}

// FetchBundle performs one bounded GET of an explicitly selected configuration.
// The full digest must be reviewed through an independent trusted channel; the
// download server's checksum is not authentication. No discovery, redirects,
// cookies, proxy, TLS bypass, retries, key generation, joining or execution.
// The existing owner-only output parent must contain no file at that name.
func FetchBundle(ctx context.Context, source, pin, output string) (BundleFetchReport, error) {
	if ctx == nil || ctx.Err() != nil || !bundleDownloadURL(source) || !bundlePinOK(pin) ||
		!filepath.IsAbs(output) || filepath.Clean(output) != output || filepath.Base(output) != bundleFile {
		return BundleFetchReport{}, ErrBundleFetch
	}
	r, err := openPrivate(filepath.Dir(output))
	if err != nil {
		return BundleFetchReport{}, ErrBundleFetch
	}
	defer r.Close()
	if _, err := r.Lstat(bundleFile); !os.IsNotExist(err) {
		return BundleFetchReport{}, ErrBundleFetch
	}
	transport := healthTransport()
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return ErrBundleFetch }}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return BundleFetchReport{}, ErrBundleFetch
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "identity")
	response, err := client.Do(req)
	if err != nil {
		return BundleFetchReport{}, ErrBundleFetch
	}
	defer response.Body.Close()
	media, params, mimeErr := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if response.StatusCode != http.StatusOK || len(response.Header.Values("Content-Type")) != 1 || mimeErr != nil || media != "application/json" || len(params) > 1 ||
		(len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8")) || response.ContentLength > maxBundleBytes ||
		len(response.Header.Values("Content-Encoding")) != 0 || len(response.Header.Values("Set-Cookie")) != 0 || len(response.Header.Values("Location")) != 0 {
		return BundleFetchReport{}, ErrBundleFetch
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxBundleBytes+1))
	if err != nil || ctx.Err() != nil {
		return BundleFetchReport{}, ErrBundleFetch
	}
	b, err := validatedBundle(raw, pin)
	if err != nil {
		return BundleFetchReport{}, ErrBundleFetch
	}
	// Exclusive creation after all network/format checks; interrupted disk writes
	// may leave an unusable private partial file, never a successful report.
	if ctx.Err() != nil || writePrivate(r, bundleFile, raw) != nil {
		return BundleFetchReport{}, ErrBundleFetch
	}
	validators := 0
	for _, p := range b.Profiles {
		if p.Role == "validator" {
			validators++
		}
	}
	return BundleFetchReport{Synthetic: true, Downloaded: true, BundleVerified: true,
		Bytes: len(raw), ValidatorCount: validators, RequireNodeKeyProof: b.Runtime.RequireValidatorProof}, nil
}
