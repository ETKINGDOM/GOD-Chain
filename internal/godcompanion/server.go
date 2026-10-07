// Package godcompanion serves the local synthetic-test participant interface.
// It embeds only static assets: no node files, RPC proxy or account signer.
package godcompanion

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed assets/index.html assets/styles.css assets/client.mjs assets/main.mjs assets/wallet.html assets/explorer.html assets/explorer.mjs
var assets embed.FS
var ErrCompanion = errors.New("local synthetic companion unavailable")

var securityHeaders = map[string]string{
	"Content-Security-Policy": "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self' https: http://localhost:* http://127.0.0.1:* http://[::1]:*; base-uri 'none'; frame-ancestors 'none'; form-action 'none'; object-src 'none'",
	"X-Content-Type-Options":  "nosniff",
	"Referrer-Policy":         "no-referrer",
	"Cache-Control":           "no-store",
	"Permissions-Policy":      "camera=(), microphone=(), geolocation=()",
}

func assetForPath(path string) (string, string) {
	switch path {
	case "/", "/index.html":
		return "index.html", "text/html; charset=utf-8"
	case "/wallet.html", "/explorer.html":
		return strings.TrimPrefix(path, "/"), "text/html; charset=utf-8"
	case "/styles.css":
		return "styles.css", "text/css; charset=utf-8"
	case "/client.mjs", "/main.mjs", "/explorer.mjs":
		return strings.TrimPrefix(path, "/"), "text/javascript; charset=utf-8"
	default:
		return "", ""
	}
}

// MatchesAssetResponse compares bytes and security headers with this reviewed
// build. It does not execute browser code or accept caller-provided resources.
func MatchesAssetResponse(path string, header http.Header, raw []byte) bool {
	file, kind := assetForPath(path)
	if file == "" || len(header.Values("Content-Type")) != 1 || header.Get("Content-Type") != kind || len(header.Values("Content-Encoding")) != 0 || len(header.Values("Set-Cookie")) != 0 {
		return false
	}
	for key, value := range securityHeaders {
		if len(header.Values(key)) != 1 || header.Get(key) != value {
			return false
		}
	}
	expected, err := assets.ReadFile("assets/" + file)
	return err == nil && bytes.Equal(raw, expected)
}

func localHost(host string) bool {
	if h, p, err := net.SplitHostPort(host); err == nil {
		n, e := strconv.Atoi(p)
		if e != nil || n < 1 || n > 65535 || strconv.Itoa(n) != p {
			return false
		}
		host = h
	} else {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	return host == "localhost" || net.ParseIP(host).IsLoopback()
}

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key, value := range securityHeaders {
			w.Header().Set(key, value)
		}
		if !localHost(r.Host) {
			http.Error(w, "host rejected", http.StatusForbidden)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "method rejected", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.RawQuery != "" || r.URL.ForceQuery {
			http.NotFound(w, r)
			return
		}
		file, kind := assetForPath(r.URL.Path)
		if file == "" {
			http.NotFound(w, r)
			return
		}
		raw, err := assets.ReadFile("assets/" + file)
		if err != nil {
			http.Error(w, "asset unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", kind)
		w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
		if r.Method == "GET" {
			_, _ = w.Write(raw)
		}
	})
}

type limitedListener struct {
	net.Listener
	slots chan struct{}
	done  chan struct{}
	once  sync.Once
}
type limitedConn struct {
	net.Conn
	release func()
	once    sync.Once
}

func (c *limitedConn) Close() error { err := c.Conn.Close(); c.once.Do(c.release); return err }
func (l *limitedListener) Accept() (net.Conn, error) {
	select {
	case l.slots <- struct{}{}:
	case <-l.done:
		return nil, net.ErrClosed
	}
	c, err := l.Listener.Accept()
	if err != nil {
		<-l.slots
		return nil, err
	}
	return &limitedConn{Conn: c, release: func() { <-l.slots }}, nil
}
func (l *limitedListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return l.Listener.Close()
}

// Serve binds only a caller-selected numeric loopback address. Starting the
// companion never starts a chain or permits public hosting. Reports are redacted.
func Serve(ctx context.Context, listen string, out io.Writer) error {
	host, port, err := net.SplitHostPort(listen)
	n, e := strconv.Atoi(port)
	if ctx == nil || out == nil || err != nil || e != nil || n < 1024 || n > 65535 || strconv.Itoa(n) != port || !net.ParseIP(host).IsLoopback() {
		return ErrCompanion
	}
	base, err := net.Listen("tcp", listen)
	if err != nil {
		return ErrCompanion
	}
	l := &limitedListener{Listener: base, slots: make(chan struct{}, 32), done: make(chan struct{})}
	defer l.Close()
	server := &http.Server{Handler: Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 15 * time.Second, MaxHeaderBytes: 8192, ErrorLog: log.New(io.Discard, "", 0)}
	if json.NewEncoder(out).Encode(map[string]bool{"syntheticInterface": true, "loopbackOnly": true, "serverSigner": false, "chainStarted": false, "realAssets": false}) != nil {
		return ErrCompanion
	}
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(l) }()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if server.Shutdown(shutdown) != nil {
			_ = server.Close()
			<-finished
			return ErrCompanion
		}
		if err := <-finished; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return ErrCompanion
		}
		return nil
	case err := <-finished:
		_ = server.Close()
		if !errors.Is(err, http.ErrServerClosed) {
			return ErrCompanion
		}
		return nil
	}
}
