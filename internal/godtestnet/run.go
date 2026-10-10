//go:build go1.25

package godtestnet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnode"
	dbm "github.com/cosmos/cosmos-db"
)

// Run starts one explicit, persistent synthetic node and a restricted RPC
// listener. The caller supplies cancellation and may opt into network exposure
// only after separately reviewing the host, firewall and TLS gateway. Config
// errors and startup failures never include private path/endpoint/key values.
func Run(ctx context.Context, home string, allowNetwork bool, out io.Writer) (result error) {
	if ctx == nil || out == nil {
		return ErrConfig
	}
	r, err := openPrivate(home)
	if err != nil {
		return err
	}
	defer r.Close()
	lease, err := acquireNodeLease(r)
	if err != nil {
		return err
	}
	// Registered before the application/engine defers: release only after their
	// shutdown and storage close. A refused duplicate never opens node databases.
	defer lease.Close()
	loaded, err := Load(home)
	if err != nil {
		return err
	}
	if loaded.Report.NetworkExposure && !allowNetwork {
		return ErrConfig
	}
	for _, name := range []string{"data", "data/sdk"} {
		if r.MkdirAll(name, 0700) != nil {
			return ErrPrivate
		}
		info, err := r.Lstat(name)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
			return ErrPrivate
		}
	}
	// Bind the listener before opening the application or consuming its global
	// runtime. A busy port does not leave a half-running consensus engine.
	listener, err := net.Listen("tcp", loaded.Document.RPCListen)
	if err != nil {
		return startFailure("rpc-listener", err)
	}
	defer listener.Close()
	db, err := dbm.NewDB("god_app", dbm.GoLevelDBBackend, filepath.Join(home, "data", "sdk"))
	if err != nil {
		return startFailure("application-database", err)
	}
	app, err := godnode.New(db, loaded.Document.Runtime)
	if err != nil {
		_ = db.Close()
		return startFailure("application-initialization", err)
	}
	defer func() {
		if err := app.Close(); err != nil && result == nil {
			result = startFailure("application-close", err)
		}
	}()
	n, err := godnode.StartTestnet(ctx, app, godnode.LocalOptions{Directory: home, Listen: loaded.Document.P2PListen, Peers: loaded.Document.Peers,
		BlockInterval: time.Duration(loaded.Document.BlockIntervalMillis) * time.Millisecond, Genesis: loaded.Genesis, Signer: loaded.Signer, NodeKey: loaded.NodeKey, Persistent: true, Observer: loaded.Document.Role == "observer", CandidateOwner: loaded.Document.CandidateOwner})
	if err != nil {
		return startFailure("consensus-start", err)
	}
	stopped := false
	defer func() {
		if !stopped {
			_ = n.Stop()
		}
	}()
	if json.NewEncoder(out).Encode(loaded.Report) != nil {
		return startFailure("report-write", nil)
	}
	if err := godnode.ServeTestnetRPC(ctx, n, listener, godnode.RPCOptions{Hosts: loaded.Document.RPCHosts, Origins: loaded.Document.RPCOrigins, AllowNetwork: allowNetwork}); err != nil {
		return startFailure("rpc-service", err)
	}
	// Stop and join the engine before the deferred application store close.
	if err := n.Stop(); err != nil {
		return startFailure("consensus-stop", err)
	}
	stopped = true
	return nil
}

// Only fixed phase/category labels reach command-line diagnostics. Never wrap
// the underlying error: it may contain a private path, endpoint or key value.
func startFailure(phase string, cause error) error {
	switch phase {
	case "rpc-listener", "application-database", "application-initialization", "application-close", "consensus-start", "report-write", "rpc-service", "consensus-stop":
	default:
		phase = "unknown"
	}
	category := "failed"
	if errors.Is(cause, syscall.EADDRINUSE) {
		category = "port-in-use"
	}
	return fmt.Errorf("%w: %s (%s)", ErrStart, phase, category)
}
