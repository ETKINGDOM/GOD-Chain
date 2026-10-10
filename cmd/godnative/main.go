package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godnativegateway"
)

type bootstrap struct {
	initialize  bool
	audit       bool
	verify      bool
	pageWorker  bool
	destination string
	capacity    uint64
	expected    string
	hashes      []string
}

func parseMode(args []string) (godnativegateway.Config, bootstrap, error) {
	var c godnativegateway.Config
	var b bootstrap
	f := flag.NewFlagSet("godnative", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.StringVar(&c.Listen, "listen", "", "numeric loopback listen endpoint")
	f.StringVar(&c.Upstream, "upstream", "", "numeric loopback observer RPC URL")
	f.StringVar(&c.Origin, "origin", "", "exact HTTPS website origin")
	f.StringVar(&c.ExtensionOrigin, "extension-origin", "", "optional exact reviewed Chrome test origin")
	f.StringVar(&c.ChainID, "chain", "", "private synthetic chain identity")
	f.StringVar(&c.CompatibleChainID, "compatible-chain", "", "decimal compatible chain identity")
	f.Uint64Var(&c.Gas, "gas", 0, "fixed reviewed native gas limit")
	f.StringVar(&c.MinFeePerGas, "min-price", "", "positive agod per gas floor")
	f.BoolVar(&c.ValidatorOnly, "validator-only", false, "serve only one bounded unsigned validator lookup")
	f.StringVar(&c.AttemptDirectory, "attempt-directory", "", "existing owner-only native attempt directory")
	f.Uint64Var(&c.AttemptCapacity, "attempt-capacity", 0, "exact durable store capacity; zero retains the 1000-record default")
	f.BoolVar(&b.initialize, "initialize-attempts", false, "offline creation only; never start an HTTP server")
	f.BoolVar(&b.audit, "audit-attempts", false, "offline aggregate inventory only; never start an HTTP server")
	f.BoolVar(&b.verify, "verify-attempts", false, "offline page and inventory check against a separately retained exact file SHA256")
	f.BoolVar(&b.pageWorker, "check-attempt-pages-worker", false, "internal isolated offline page-check worker; never serve")
	f.StringVar(&b.destination, "copy-attempts-to", "", "existing empty owner-only destination for an offline copy")
	f.Uint64Var(&b.capacity, "copy-attempt-capacity", 0, "explicit equal or larger destination capacity, at most 100000")
	f.StringVar(&b.expected, "expected-source-sha256", "", "reviewed exact lowercase source file SHA256 for offline copy or verification")
	f.Func("blocked-hash", "exact reviewed legacy uncertain hash; repeat only during offline initialization", func(s string) error {
		if len(b.hashes) >= 1000 || len(s) != 66 || !strings.HasPrefix(s, "0x") || strings.Trim(s[2:], "0123456789abcdef") != "" {
			return godnativegateway.ErrGateway
		}
		b.hashes = append(b.hashes, s)
		return nil
	})
	if f.Parse(args) != nil || f.NArg() != 0 {
		return c, b, godnativegateway.ErrGateway
	}
	modes := 0
	for _, on := range []bool{b.initialize, b.audit, b.verify, b.destination != ""} {
		if on {
			modes++
		}
	}
	copyFlag := false
	expectedFlag := false
	f.Visit(func(flag *flag.Flag) {
		if flag.Name == "copy-attempts-to" || flag.Name == "copy-attempt-capacity" {
			copyFlag = true
		}
		if flag.Name == "expected-source-sha256" {
			expectedFlag = true
		}
	})
	invalidMode := modes > 1 || len(b.hashes) != 0 && !b.initialize || modes != 0 && (c.ValidatorOnly || c.AttemptDirectory == "")
	invalidCapacity := c.AttemptCapacity != 0 && (c.AttemptCapacity < 1000 || c.AttemptCapacity > godnativegateway.MaxAttemptCapacity)
	invalidReadOnly := c.ValidatorOnly && (c.AttemptDirectory != "" || c.AttemptCapacity != 0)
	validExpected := expectedFlag && len(b.expected) == 64 && strings.Trim(b.expected, "0123456789abcdef") == ""
	invalidCopy := copyFlag && (b.destination == "" || b.capacity < 1000 || b.capacity > godnativegateway.MaxAttemptCapacity || b.capacity < c.AttemptCapacity || !validExpected)
	invalidVerify := b.verify && !validExpected || expectedFlag && !(b.verify || b.destination != "")
	invalidWorker := b.pageWorker && !b.audit
	if invalidMode || invalidCapacity || invalidReadOnly || invalidCopy || invalidVerify || invalidWorker {
		return c, b, godnativegateway.ErrGateway
	}
	return c, b, nil
}

// The physical checker never shares the gateway's process or a database
// handle. Its stdout is bounded and strictly decoded; stderr (including a
// panic's storage diagnostics) is never forwarded to users or server logs.
type auditOutput struct {
	bytes.Buffer
}

func (w *auditOutput) Write(p []byte) (int, error) {
	if len(p) > 4096-w.Len() {
		return 0, godnativegateway.ErrGateway
	}
	return w.Buffer.Write(p)
}

func readAuditWorker(cmd *exec.Cmd) (godnativegateway.AttemptAudit, error) {
	var out godnativegateway.AttemptAudit
	var output auditOutput
	cmd.Stdout, cmd.Stderr, cmd.Stdin = &output, io.Discard, nil
	cmd.WaitDelay = time.Second
	if cmd.Run() != nil {
		return out, godnativegateway.ErrGateway
	}
	payload := output.Bytes()
	d := json.NewDecoder(bytes.NewReader(payload))
	d.DisallowUnknownFields()
	if d.Decode(&out) != nil || d.Decode(new(any)) != io.EOF || !out.PagesChecked || out.SchemaVersion < 1 || out.SchemaVersion > 2 || out.Capacity < 1000 || out.Capacity > godnativegateway.MaxAttemptCapacity || out.Records > out.Capacity || out.Remaining != out.Capacity-out.Records || out.FileBytes < 8192 || out.FileBytes > out.FileBudget || out.FileBudget != 4<<20 && out.FileBudget != 32<<20 || len(out.FileSHA256) != 64 || strings.Trim(out.FileSHA256, "0123456789abcdef") != "" || len(out.LogicalSHA256) != 64 || strings.Trim(out.LogicalSHA256, "0123456789abcdef") != "" {
		return godnativegateway.AttemptAudit{}, godnativegateway.ErrGateway
	}
	canonical, err := json.Marshal(out)
	if err != nil || !bytes.Equal(bytes.TrimSpace(payload), canonical) {
		return godnativegateway.AttemptAudit{}, godnativegateway.ErrGateway
	}
	return out, nil
}

func checkedAudit(c godnativegateway.Config) (godnativegateway.AttemptAudit, error) {
	exe, err := os.Executable()
	if err != nil {
		return godnativegateway.AttemptAudit{}, godnativegateway.ErrGateway
	}
	args := []string{"--audit-attempts", "--check-attempt-pages-worker", "--listen", c.Listen, "--upstream", c.Upstream, "--origin", c.Origin, "--chain", c.ChainID, "--compatible-chain", c.CompatibleChainID, "--gas", fmt.Sprint(c.Gas), "--min-price", c.MinFeePerGas, "--attempt-directory", c.AttemptDirectory, "--attempt-capacity", fmt.Sprint(c.AttemptCapacity)}
	if c.ExtensionOrigin != "" {
		args = append(args, "--extension-origin", c.ExtensionOrigin)
	}
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signals, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Env = append(os.Environ(), "GOTRACEBACK=none")
	out, err := readAuditWorker(cmd)
	capacity, budget := c.AttemptCapacity, int64(32<<20)
	if capacity == 0 {
		capacity = 1000
	}
	if capacity == 1000 {
		budget = 4 << 20
	}
	if err != nil || out.Capacity != capacity || out.FileBudget != budget || out.SchemaVersion == 1 && capacity != 1000 {
		return godnativegateway.AttemptAudit{}, godnativegateway.ErrGateway
	}
	return out, nil
}

func checkedCopy(c godnativegateway.Config, b bootstrap) (godnativegateway.AttemptCopyAudit, error) {
	var empty godnativegateway.AttemptCopyAudit
	before, err := checkedAudit(c)
	if err != nil || before.FileSHA256 != b.expected {
		return empty, godnativegateway.ErrGateway
	}
	out, err := godnativegateway.CopyAttempts(c, b.destination, b.capacity, b.expected)
	if err != nil {
		return empty, godnativegateway.ErrGateway
	}
	logicalSource := before
	logicalSource.PagesChecked = false
	if out.Source != logicalSource {
		return empty, godnativegateway.ErrGateway
	}
	target := c
	target.AttemptDirectory, target.AttemptCapacity = b.destination, b.capacity
	after, err := checkedAudit(target)
	logicalTarget := after
	logicalTarget.PagesChecked = false
	if err != nil || out.Destination != logicalTarget {
		return empty, godnativegateway.ErrGateway
	}
	out.Source, out.Destination = before, after
	return out, nil
}

func parse(args []string) (godnativegateway.Config, error) {
	c, _, err := parseMode(args)
	return c, err
}

func main() {
	c, b, err := parseMode(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "native gateway configuration rejected")
		os.Exit(1)
	}
	if b.initialize {
		if godnativegateway.InitializeAttempts(c, b.hashes) != nil {
			fmt.Fprintln(os.Stderr, "native attempt initialization rejected")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, "native attempt store initialized; no server or node request started")
		return
	}
	if b.audit || b.verify || b.destination != "" {
		var report any
		if b.audit || b.verify {
			var audit godnativegateway.AttemptAudit
			if b.pageWorker {
				audit, err = godnativegateway.CheckAttempts(c)
			} else {
				audit, err = checkedAudit(c)
			}
			if b.verify && audit.FileSHA256 != b.expected {
				err = godnativegateway.ErrGateway
			}
			report = audit
		} else {
			report, err = checkedCopy(c, b)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "native attempt maintenance rejected")
			os.Exit(1)
		}
		if json.NewEncoder(os.Stdout).Encode(report) != nil {
			fmt.Fprintln(os.Stderr, "native attempt report unavailable")
			os.Exit(1)
		}
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if godnativegateway.Serve(ctx, c) != nil {
		fmt.Fprintln(os.Stderr, "native gateway unavailable")
		os.Exit(1)
	}
}
