package godhistory

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godorigin"
	bolt "go.etcd.io/bbolt"
)

// QueryConfig is supplied privately. Querying cannot create or update an index,
// request a sync, select an upstream, read signing keys or activate real assets.
type QueryConfig struct {
	Index           Config
	Home            string
	Listen          string
	Upstream        string
	Origin          string
	ExtensionOrigin string
	MaxAge          time.Duration
}

type queryRequest struct {
	Version int     `json:"version"`
	Address string  `json:"address"`
	Cursor  *Cursor `json:"cursor"`
}

type QueryReply struct {
	Version           int    `json:"version"`
	ChainID           string `json:"chainId"`
	CompatibleChainID string `json:"evmChainId"`
	BundleSHA256      string `json:"bundleSha256"`
	Address           string `json:"address"`
	ObservedTip       string `json:"observedTip"`
	SourceCheckedAt   string `json:"sourceCheckedAt"`
	Page              Page   `json:"page"`
}

type Query struct {
	config QueryConfig
	rpc    *RPC
	source Source
	slots  chan struct{}
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

func queryEndpoint(s string) bool {
	host, port, e := net.SplitHostPort(s)
	p, f := strconv.Atoi(port)
	return e == nil && f == nil && net.ParseIP(host).IsLoopback() && p >= 1024 && p <= 65535 && strconv.Itoa(p) == port
}

func queryOrigin(s string) bool {
	u, e := url.Parse(s)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.Host != strings.ToLower(u.Host) || u.User != nil || u.Port() != "" || u.Path != "" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || len(u.Host) > 253 || len(strings.Split(u.Host, ".")) < 2 {
		return false
	}
	for _, label := range strings.Split(u.Host, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}

func NewQuery(c QueryConfig) (*Query, error) {
	u, e := url.Parse(c.Upstream)
	if !c.Index.valid() || !queryEndpoint(c.Listen) || !queryOrigin(c.Origin) || !godorigin.ValidExtension(c.ExtensionOrigin) || c.MaxAge < time.Second || c.MaxAge > 300*time.Second || c.MaxAge%time.Second != 0 || e != nil || u.Host == c.Listen {
		return nil, ErrHistory
	}
	rpc, e := NewRPC(c.Upstream)
	if e != nil {
		return nil, ErrHistory
	}
	// Fail before binding if the database does not already exist under the
	// reviewed namespace. This opens read-only and cannot initialize or repair it.
	x, e := Open(c.Home, c.Index, true)
	if e != nil {
		rpc.Close()
		return nil, ErrHistory
	}
	closed := x.Close()
	if closed != nil {
		rpc.Close()
		return nil, ErrHistory
	}
	return &Query{config: c, rpc: rpc, source: rpc, slots: make(chan struct{}, 1), tokens: 2, last: time.Now()}, nil
}

func (q *Query) Close() { q.rpc.Close() }

func (q *Query) allow() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	q.tokens += now.Sub(q.last).Seconds()
	q.last = now
	if q.tokens > 2 {
		q.tokens = 2
	}
	if q.tokens < 1 {
		return false
	}
	q.tokens--
	return true
}

func queryInput(raw []byte) (queryRequest, error) {
	var r queryRequest
	if len(raw) > 2048 || exact(raw, &r) != nil || r.Version != 1 || len(r.Address) > 42 {
		return r, ErrHistory
	}
	if _, e := account(r.Address); e != nil {
		return r, ErrHistory
	}
	if r.Cursor != nil {
		c := r.Cursor
		h, e := integer(c.SnapshotHeight, 1, 9223372036854775807)
		if e != nil || !hash(c.SnapshotHash) {
			return r, ErrHistory
		}
		if _, e := integer(c.BeforeHeight, 1, h); e != nil {
			return r, ErrHistory
		}
		if _, e := integer(c.BeforeIndex, 0, BlockLimit-1); e != nil {
			return r, ErrHistory
		}
	}
	return r, nil
}

func (q *Query) page(ctx context.Context, r queryRequest) (reply QueryReply, err error) {
	x, e := Open(q.config.Home, q.config.Index, true)
	if e != nil {
		return reply, ErrHistory
	}
	defer func() {
		recovered := recover()
		if x.Close() != nil || recovered != nil {
			reply, err = QueryReply{}, ErrHistory
		}
	}()
	s, e := x.Status()
	checked, f := stamp(s.CheckedAt)
	if e != nil || f != nil || !s.Verified || time.Since(checked) < 0 || time.Since(checked) > q.config.MaxAge {
		return reply, ErrHistory
	}
	p, e := x.Page(r.Address, r.Cursor)
	if e != nil {
		return reply, ErrHistory
	}
	raw, e := q.source.Read(ctx, "god_network", []string{})
	n, tip, f := parseNetwork(raw, q.config.Index)
	if e != nil || f != nil || tip < s.Height {
		return reply, ErrHistory
	}
	// Check the retained tip, and an older selected cursor anchor when needed.
	// This is observer consistency, not authenticated ancestry or finality.
	seen := map[string]bool{}
	for _, anchor := range []string{strconv.FormatInt(s.Height, 10), p.Through} {
		h, _ := integer(anchor, 1, s.Height)
		if seen[anchor] {
			continue
		}
		seen[anchor] = true
		b, e := readPage(ctx, q.source, h, 0)
		var stored Block
		local := x.db.View(func(tx *bolt.Tx) error { var e error; stored, e = storedBlock(tx, h); return e })
		if e != nil || local != nil || b.Hash != stored.Hash || b.Time != stored.Time || b.Parent != stored.Parent || b.Count != stored.Transactions {
			return reply, ErrHistory
		}
	}
	raw, e = q.source.Read(ctx, "god_network", []string{})
	last, lastTip, f := parseNetwork(raw, q.config.Index)
	firstTime, _ := stamp(n.Commit.Time)
	lastTime, _ := stamp(last.Commit.Time)
	if e != nil || f != nil || lastTip < tip || lastTip == tip && last.Commit != n.Commit || lastTime.Before(firstTime) || ctx.Err() != nil || time.Since(checked) > q.config.MaxAge {
		return reply, ErrHistory
	}
	a, _ := account(r.Address)
	native, _ := godaddress.ToNative(a)
	return QueryReply{1, q.config.Index.ChainID, q.config.Index.CompatibleChainID, q.config.Index.BundleSHA256, native, strconv.FormatInt(lastTip, 10), time.Now().UTC().Format(time.RFC3339Nano), p}, nil
}

func (q *Query) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	peer, _, e := net.SplitHostPort(r.RemoteAddr)
	if e != nil || !net.ParseIP(peer).IsLoopback() || r.Host != q.config.Listen || len(r.Header.Values("Origin")) != 1 || !godorigin.Allowed(q.config.Origin, q.config.ExtensionOrigin, r.Header.Get("Origin")) {
		http.Error(w, "history boundary rejected", http.StatusForbidden)
		return
	}
	if r.URL.Path != "/" || r.URL.RawPath != "" || r.URL.RawQuery != "" || r.URL.ForceQuery || r.RequestURI != "/" {
		http.Error(w, "history path rejected", http.StatusNotFound)
		return
	}
	if godorigin.CORS(w, r) {
		return
	}
	if r.Method != "POST" {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	kind, params, e := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if e != nil || len(r.Header.Values("Content-Type")) != 1 || kind != "application/json" || len(params) > 1 || len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8") || len(r.Header.Values("Content-Encoding")) != 0 || len(r.Header.Values("Authorization")) != 0 || len(r.Header.Values("Cookie")) != 0 {
		http.Error(w, "credential-free JSON required", http.StatusUnsupportedMediaType)
		return
	}
	if !q.allow() {
		w.Header().Set("Retry-After", "1")
		http.Error(w, "history read budget exhausted", http.StatusTooManyRequests)
		return
	}
	select {
	case q.slots <- struct{}{}:
		defer func() { <-q.slots }()
	default:
		w.Header().Set("Retry-After", "1")
		http.Error(w, "history read capacity exhausted", http.StatusTooManyRequests)
		return
	}
	raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 2048))
	if e != nil {
		http.Error(w, "history request too large", http.StatusRequestEntityTooLarge)
		return
	}
	input, e := queryInput(raw)
	if e != nil {
		http.Error(w, "invalid bounded history request", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	reply, e := q.page(ctx, input)
	if e != nil {
		http.Error(w, "retained history unavailable", http.StatusServiceUnavailable)
		return
	}
	encoded, e := json.Marshal(reply)
	if e != nil || len(encoded) > 32<<10 {
		http.Error(w, "retained history unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(encoded)
}

type queryListener struct {
	net.Listener
	slots chan struct{}
	done  chan struct{}
	once  sync.Once
}
type queryConn struct {
	net.Conn
	release func()
	once    sync.Once
}

func (c *queryConn) Close() error { e := c.Conn.Close(); c.once.Do(c.release); return e }
func (l *queryListener) Accept() (net.Conn, error) {
	select {
	case l.slots <- struct{}{}:
	case <-l.done:
		return nil, net.ErrClosed
	}
	c, e := l.Listener.Accept()
	if e != nil {
		<-l.slots
		return nil, e
	}
	return &queryConn{Conn: c, release: func() { <-l.slots }}, nil
}
func (l *queryListener) Close() error {
	l.once.Do(func() { close(l.done) })
	return l.Listener.Close()
}

// ServeQuery opens only an explicitly selected loopback listener. Independent
// TLS/proxy admission and public activation must be reviewed separately.
func ServeQuery(ctx context.Context, c QueryConfig, out io.Writer) error {
	if ctx == nil || ctx.Err() != nil || out == nil {
		return ErrHistory
	}
	q, e := NewQuery(c)
	if e != nil {
		return ErrHistory
	}
	defer q.Close()
	base, e := net.Listen("tcp", c.Listen)
	if e != nil {
		return ErrHistory
	}
	l := &queryListener{Listener: base, slots: make(chan struct{}, 16), done: make(chan struct{})}
	defer l.Close()
	server := &http.Server{Handler: q, ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 8 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 8192, ErrorLog: log.New(io.Discard, "", 0)}
	if json.NewEncoder(out).Encode(map[string]bool{"readOnly": true, "loopbackOnly": true, "serverSigner": false, "indexSyncEnabled": false, "realAssets": false}) != nil {
		return ErrHistory
	}
	finished := make(chan error, 1)
	go func() { finished <- server.Serve(l) }()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if server.Shutdown(shutdown) != nil {
			_ = server.Close()
			<-finished
			return ErrHistory
		}
		if e := <-finished; e != nil && e != http.ErrServerClosed {
			return ErrHistory
		}
		return nil
	case <-finished:
		return ErrHistory
	}
}
