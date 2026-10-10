package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/internal/godcompanion"
	"github.com/ETKINGDOM/GOD-Chain/internal/godrh"
	"github.com/ETKINGDOM/GOD-Chain/internal/godtestnet"
	"github.com/ETKINGDOM/GOD-Chain/internal/stack"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func run(args []string, out io.Writer) error {
	if len(args) > 0 && args[0] == "wallet" {
		return runWallet(args[1:], out)
	}
	if len(args) > 0 && args[0] == "companion" {
		flags := flag.NewFlagSet("companion", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		listen := flags.String("listen", "", "")
		if flags.Parse(args[1:]) != nil || flags.NArg() != 0 {
			return godcompanion.ErrCompanion
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return godcompanion.Serve(ctx, *listen, out)
	}
	if len(args) > 0 && args[0] == "testnet" {
		return runTestnet(args[1:], out)
	}
	if len(args) == 0 || (len(args) == 1 && args[0] == "help") {
		_, err := fmt.Fprintln(out, "GOD Chain core prototype\nCommands: status, version, rh-template, check-rh <private-file>, probe-rh <private-file>, testnet, wallet, companion --listen <explicit-loopback>\nRH checks are read-only and do not authorize activation.\nTestnet and wallet commands use synthetic assets only. Mainnet and real-asset operations are not enabled.")
		return err
	}
	if len(args) == 2 && (args[0] == "check-rh" || args[0] == "probe-rh") {
		return checkRH(args[0], args[1], out)
	}
	if len(args) != 1 {
		return fmt.Errorf("expected one command")
	}
	switch args[0] {
	case "status", "version":
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		return encoder.Encode(stack.BuildStatus())
	case "rh-template":
		_, err := out.Write(godrh.Template())
		return err
	case "start", "init", "bridge", "mint":
		return fmt.Errorf("not enabled: only isolated synthetic node tests are available; public activation and real-asset release gates are not verified")
	default:
		return fmt.Errorf("unknown command; use help")
	}
}

func runTestnet(args []string, out io.Writer) error {
	if len(args) == 0 || args[0] == "help" {
		if _, err := fmt.Fprintln(out, "Explicit configuration download: fetch-bundle --source <reviewed-HTTPS-bundle-URL> --expected-bundle <independently-reviewed-digest> --output <new-private-testnet-bundle.json>\nOne bounded request, no redirects, cookies, proxies or TLS bypass. Verifies before writing; never initializes or starts a node. Downloading a checksum from the same server is not authentication."); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "New synthetic network policy: assemble --home <new-private-bundle> --profiles <reviewed-private-profiles> --require-validator-proof\nExplicitly enables on-chain consensus-key possession checks. Candidate setup/signing requires this policy; legacy bundles remain unchanged. Never toggle an existing runtime/database or reset signing state to enable it."); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "Fresh self-service setup: init --home <new-private-node> --role <observer|candidate> [--owner <separately-held-wallet>] --endpoint <advertised-ip:port> --p2p <tcp-listen> --rpc <loopback-listen> --hosts <exact-hosts> --bundle <private-file> --expected-bundle <independently-reviewed-digest>\nDoes not start, submit or overwrite. Candidate signing requires locally replayed committed operator ownership and the actual signing set for that height."); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "Candidate registration: join-candidate --home <fresh-private-validator-identity> --bundle <private-file> --expected-bundle <reviewed-digest>; prepare-registration --home <private-candidate> --bundle <private-file> --expected-bundle <reviewed-digest> --output <new-private-request> --account-number <fresh-number> --sequence <fresh-sequence> --gas <reviewed-limit> --fee <agod> --stake <agod>; sign-registration --request <private-request> --key <separate-private-test-wallet> --output <new-private-transaction> --bundle <private-file> --expected-bundle <reviewed-digest>\nRegistration signing is offline and separate from the six browser/gateway actions. No automatic funding, submission or production activation."); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "Self-service non-signing join: join-observer --home <fresh-private-observer> --bundle <private-file> --expected-bundle <independently-reviewed-digest>\nKeeps local keys and pinned genesis/peers without launch-profile registration. Requires loopback RPC and empty signing/data state; it does not start, submit, earn rewards or authorize validation."); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "Offline configuration preview: inspect-bundle --bundle <private-file> --expected-bundle <independently-reviewed-digest>\nValidates a synthetic bundle without node keys, joining, writes or network access. Distinct profiles or IPs are not independent ownership or launch approval."); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "Read-only host/service checks: host-check --data-dir <existing-private-directory> --minimum-free-bytes <reviewed-budget> --minimum-open-files <reviewed-budget>; smoke --bundle <private-file> --expected-bundle <reviewed-digest> --rpc <reviewed-url> --companion <reviewed-url> [--samples <2..10>] [--interval-seconds <1..10>] [--minimum-peers <0..32>]\nThese checks do not install, restart, sign, submit or authorize public deployment."); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "Read-only health: health --bundle <private-file> --expected-bundle <reviewed-digest> --rpc <reviewed-url> [--max-block-age-seconds <10..600>] [--minimum-peers <0..32>]\nA passed sample is not public acceptance, quorum or authenticated state."); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "Finite read-only observation: watch-health --bundle <private-file> --expected-bundle <reviewed-digest> --rpc <reviewed-url> --samples <2..10080> --interval-seconds <1..600> [--max-block-age-seconds <10..600>] [--maximum-no-progress-seconds <10..600>] [--minimum-peers <0..32>]\nRequires an explicit count and interval, bounded to seven days including request budgets. Emits redacted JSON lines and stops on the first fault. Never retries, restarts, repairs, signs, submits or sends external notifications; an observed run is not an uptime SLA or public acceptance."); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "Offline funding: sign-funding --request <private-file> --key <private-test-wallet> --output <new-private-file> --bundle <private-file> --expected-bundle <reviewed-digest> --nonce <fresh-sequence> --gas-price <smallest-units>\nFunding requests require manual review. Signing never submits, mints or releases reserve."); err != nil {
			return err
		}
		_, err := fmt.Fprintln(out, "GOD Chain synthetic testnet\nCommands: create --home <new-private-cluster> --validators <count> --first-port <port>; identity --home <new-private-node> --role <validator|observer> --owner <wallet> --endpoint <advertised-ip:port> --p2p <tcp-listen> --rpc <listen> --hosts <exact-hosts>; assemble --home <new-private-ceremony> --profiles <private-profile-files>; join --home <private-node> --bundle <private-file> --expected-bundle <reviewed-digest>; sign --request <private-file> --key <private-test-wallet> --output <new-private-file> --bundle <private-file> --expected-bundle <reviewed-digest>; check --home <private-node>; start --home <private-node> [--allow-network]\nExisting data and signing progress are never replaced. Local clusters are not independently operated networks. No RH or real assets are enabled.")
		return err
	}
	command := args[0]
	if command == "host-check" || command == "smoke" {
		return runAcceptance(command, args[1:], out)
	}
	if command == "fetch-bundle" {
		return runBundleFetch(args[1:], out)
	}
	if command == "watch-health" {
		return runWatch(args[1:], out)
	}
	if command != "create" && command != "check" && command != "start" && command != "identity" && command != "init" && command != "assemble" && command != "join" && command != "join-observer" && command != "join-candidate" && command != "prepare-registration" && command != "sign-registration" && command != "sign" && command != "sign-funding" && command != "health" && command != "inspect-bundle" {
		return godtestnet.ErrConfig
	}
	flags := flag.NewFlagSet("testnet", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	home := flags.String("home", "", "")
	var count, port *int
	var network *bool
	var requireProof *bool
	var role, owner, endpoint, p2pListen, rpcListen, hosts, origins, profiles, bundle, expected *string
	var request, key, output *string
	var nonce, price *string
	var number, sequence, gas, fee, stake *string
	var rpc *string
	var maxAge, minPeers *int
	offline := command == "sign" || command == "sign-funding" || command == "sign-registration"
	noHome := offline || command == "health" || command == "inspect-bundle"
	if command == "create" {
		count = flags.Int("validators", 4, "")
		port = flags.Int("first-port", 0, "")
	}
	if command == "start" {
		network = flags.Bool("allow-network", false, "")
	}
	if command == "identity" || command == "init" {
		role = flags.String("role", "validator", "")
		if command == "init" {
			*role = "observer"
		}
		owner = flags.String("owner", "", "")
		endpoint = flags.String("endpoint", "", "")
		p2pListen = flags.String("p2p", "", "")
		rpcListen = flags.String("rpc", "", "")
		hosts = flags.String("hosts", "", "")
		origins = flags.String("origins", "", "")
	}
	if command == "assemble" {
		profiles = flags.String("profiles", "", "")
		requireProof = flags.Bool("require-validator-proof", false, "")
	}
	if command == "join" || command == "join-observer" || command == "join-candidate" || command == "init" || command == "prepare-registration" || noHome {
		bundle = flags.String("bundle", "", "")
		expected = flags.String("expected-bundle", "", "")
	}
	if offline {
		request = flags.String("request", "", "")
		key = flags.String("key", "", "")
		output = flags.String("output", "", "")
	}
	if command == "sign-funding" {
		nonce = flags.String("nonce", "", "")
		price = flags.String("gas-price", "", "")
	}
	if command == "prepare-registration" {
		output = flags.String("output", "", "")
		number = flags.String("account-number", "", "")
		sequence = flags.String("sequence", "", "")
		gas = flags.String("gas", "", "")
		fee = flags.String("fee", "", "")
		stake = flags.String("stake", "", "")
	}
	if command == "health" {
		rpc = flags.String("rpc", "", "")
		maxAge = flags.Int("max-block-age-seconds", 30, "")
		minPeers = flags.Int("minimum-peers", 0, "")
	}
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 || (!noHome && (*home == "" || !filepath.IsAbs(*home))) || (noHome && *home != "") {
		return godtestnet.ErrConfig
	}
	switch command {
	case "prepare-registration":
		report, err := godtestnet.PrepareRegistration(*home, *bundle, *expected, *output, godtestnet.RegistrationRequest{AccountNumber: *number, Sequence: *sequence, Gas: *gas, Fee: *fee, Stake: *stake})
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(report)
	case "sign-registration":
		report, err := godtestnet.SignRegistration(*request, *key, *output, *bundle, *expected)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(report)
	case "inspect-bundle":
		report, err := godtestnet.InspectBundle(*bundle, *expected)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(report)
	case "health":
		if *maxAge < 10 || *maxAge > 600 {
			return godtestnet.ErrHealth
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		report, err := godtestnet.Health(ctx, *bundle, *expected, *rpc, time.Duration(*maxAge)*time.Second, *minPeers)
		if encodeErr := json.NewEncoder(out).Encode(report); encodeErr != nil {
			return encodeErr
		}
		return err
	case "sign-funding":
		report, err := godtestnet.SignFunding(*request, *key, *output, *bundle, *expected, *nonce, *price)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(report)
	case "sign":
		report, err := godtestnet.SignNative(*request, *key, *output, *bundle, *expected)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(report)
	case "identity", "init":
		allowedOrigins := []string{}
		if *origins != "" {
			allowedOrigins = strings.Split(*origins, ",")
		}
		options := godtestnet.IdentityOptions{Role: *role, Owner: *owner, Endpoint: *endpoint, P2PListen: *p2pListen, RPCListen: *rpcListen, RPCHosts: strings.Split(*hosts, ","), RPCOrigins: allowedOrigins}
		var report godtestnet.Report
		var err error
		if command == "init" {
			report, err = godtestnet.InitParticipant(*home, *bundle, *expected, options)
		} else {
			report, err = godtestnet.CreateIdentity(*home, options)
		}
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(report)
	case "assemble":
		assemble := godtestnet.Assemble
		if *requireProof {
			assemble = godtestnet.AssembleWithValidatorProof
		}
		report, err := assemble(*home, strings.Split(*profiles, ","))
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(report)
	case "join":
		report, err := godtestnet.Join(*home, *bundle, *expected)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(report)
	case "join-observer":
		report, err := godtestnet.JoinObserver(*home, *bundle, *expected)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(report)
	case "join-candidate":
		report, err := godtestnet.JoinCandidate(*home, *bundle, *expected)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(report)
	case "create":
		report, err := godtestnet.CreateLocal(*home, *count, *port)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(report)
	case "check":
		loaded, err := godtestnet.Load(*home)
		if err != nil {
			return err
		}
		return json.NewEncoder(out).Encode(loaded.Report)
	default:
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		return godtestnet.Run(ctx, *home, *network, out)
	}
}

func checkRH(command, path string, out io.Writer) error {
	config, err := godrh.LoadPrivate(path)
	if err != nil {
		return err
	}
	report := config.Report()
	if command == "probe-rh" && report.ConfigurationReady {
		source, err := godrh.NewHTTPSource(config)
		if err != nil {
			return err
		}
		defer source.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		report = godrh.Probe(ctx, config, source)
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return err
	}
	if !report.ConfigurationReady || command == "probe-rh" && !report.ReadOnlyProbePassed {
		return fmt.Errorf("RH read-only readiness checks did not pass; activation remains disabled")
	}
	return nil
}

func main() {
	config := sdk.GetConfig()
	godaddress.ConfigureSDK(config)
	config.Seal()
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
