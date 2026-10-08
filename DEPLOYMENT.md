# GOD Chain private host testing tools

The deployment tools prepare a private Linux acceptance candidate for God EVM + God SDK + GodCometBFT. They package reviewed binaries, preserve the existing dependency notices and provide a bounded read-only health check. They do not install services, initialize wallets, distribute validator keys, authorize public deployment or connect real assets. A target machine is still required to establish Linux runtime behavior.

Use [TESTNET.md](TESTNET.md) for independent initialization and recovery, and [COMPANION.md](COMPANION.md) for participant testing. Test GOD and G have no monetary value or guaranteed mainnet conversion. Keep RH routes and real-asset commands disabled.

## Build and review

Use Go 1.26.8 and the checksum-bound lifecycle build with unchanged module pins:

```sh
make check build compile-targets
```

The native `build/godpack` command prepares or verifies archives without network access. Linux amd64 and arm64 builds contain separate `godd` and `godpack` executables. Only an intended host can validate those executables' runtime behavior. Windows private node operations remain disabled pending ACL support; cross-compilation is not runtime acceptance.

Review the exact candidate source and binary digest before packaging. The builder checks the Linux ELF target, Go version, command/module identity, static build settings, critical dependency pins and the expected private lifecycle-module replacement. Binary metadata cannot independently prove every compiler input or patch. The expected binary digest must identify a separately reviewed checksum-bound build; a hash copied from an untrusted download is not authentication.

## Create a private acceptance archive

Choose a new directory inside an existing private parent; never reuse a home directory or broad project root as the package target. All variables below are locally supplied placeholders, not published operational settings.

```sh
mkdir -m 0700 "$GOD_NEW_PACKAGE_DIR"
./build/godpack create \
  --binary "$GOD_REVIEWED_LINUX_BINARY" \
  --source "$GOD_REVIEWED_MODULE_DIR" \
  --output "$GOD_NEW_PRIVATE_ARCHIVE" \
  --arch "$GOD_TARGET_ARCH" \
  --expected-binary "$GOD_REVIEWED_BINARY_SHA256"
./build/godpack verify \
  --archive "$GOD_PRIVATE_ARCHIVE" \
  --expected-archive "$GOD_REVIEWED_ARCHIVE_SHA256"
```

The target architecture is `amd64` or `arm64`. Output is a new mode-0600 tar.gz file in an owner-only directory; existing files are never overwritten. A failed write may leave an incomplete private file. Do not use it or retry into the same filename. Inspect the failure and choose a new output only after review.

The exact sixteen archive members are a manifest, one node binary, placeholder service and HTTPS gateway templates, a private-use README, five operational guides, dependency notices, unchanged lock/module files and the two existing GodCometBFT license/notice files. No application source/history, tests, dependency directory, wallet, node key, genesis, profile, endpoint or populated configuration is included. The interface is embedded in `godd` and still binds loopback only.

Verification checks the trusted archive digest, canonical manifest, fixed inventory, file hashes and sizes, target ELF header, permissions and absence of links, duplicate paths, traversal, extra members and trailing data. It neither extracts nor executes content. It is not a publisher signature or source audit. Identical inputs produce an identical archive.

These files remain private and ignored. They are not a GitHub binary release. The manifest explicitly retains `runtimeVerified: false` and `binaryRedistributionReviewed: false`. Existing notices are preserved, but complete dependency-license and source obligations require separate review before public binary distribution.

## Prepare the target machine

Transfer only the reviewed acceptance archive and an independently reviewed verifier for the matching host architecture through a private operational channel. Compare their digests over a trusted channel before execution. Verify the archive before manual extraction into a new private staging directory; the tools do not install or unpack it automatically.

Use a dedicated unprivileged service account. Place the reviewed executable in a directory that the running node cannot modify. Keep node data in a separate owner-only directory outside the source checkout and wallet workspaces. Supply no consensus keys from a developer-created cluster. Each validator initializes its own identity, reviews the complete bundle digest and joins independently.

First run native diagnostics and offline checks. Start the candidate in the foreground with the reviewed configuration, inspect actual block progress and stop gracefully. Then review every placeholder in the systemd template, validate it on the target system and test supervised shutdown/restart. The template is not installed or enabled by the package builder. Do not repair startup by deleting signer progress or restoring older validator backups.

Keep validator RPC on loopback. The separate observer may serve a reviewed HTTPS gateway only after firewall, exact-host/origin, per-client limits and public-deployment approval. The local companion is not a public website server. No default public domain, gateway credential or paid service is supplied.

## Read only host budget checks

Run `host-check` as the intended unprivileged service user against an existing owner-only directory on the intended data volume. It reads directory/filesystem metadata and the current process's open-file soft limit, without reading node or wallet contents, writing a probe file, changing limits or installing a service.

```sh
./build/godd testnet host-check \
  --data-dir "$GOD_PRIVATE_DATA_PARENT" \
  --minimum-free-bytes "$GOD_REVIEWED_FREE_SPACE_BUDGET" \
  --minimum-open-files "$GOD_REVIEWED_OPEN_FILE_BUDGET"
```

Both budgets are mandatory operator choices. The accepted ranges are 1 MiB through 1 PiB for available space and 64 through 1,048,576 for the open-file limit; these are input bounds, not measured minimum hardware requirements. The space report uses unprivileged availability and filesystem allocation units, with overflow rejection. The platform interfaces are documented in the [Linux filesystem statistics manual](https://www.man7.org/linux/man-pages/man2/statfs.2.html) and [Apple filesystem statistics manual](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/statfs.2.html).

The report contains OS/architecture, logical CPU count, exact decimal-string quantities and fixed reason labels, not paths, user identifiers or file contents. A privileged user, shared/symlink directory, unsupported runtime or insufficient budget fails with a nonzero exit. Linux and macOS can produce budget snapshots; Windows remains disabled. `linuxHost` identifies the actual platform rather than treating a macOS result as Linux evidence.

`preflightPassed` only means those budgets passed at that moment. `writeAccessVerified` and `publicAcceptance` remain false. It does not test filesystem writes, quotas, CPU capacity, memory, throughput, clock synchronization, disk durability or sustained growth. OS filesystem calls depend on the selected mount; use reviewed local storage. Complete those checks during native host acceptance rather than turning these snapshots into an ordinary-computer specification.

## Read only service smoke checks

After manual foreground startup of the reviewed observer and companion, run the bounded service check from the intended participant access location. Choose distinct root URLs: both reviewed HTTPS gateways, or both numeric-loopback HTTP services for local verification.

```sh
./build/godd testnet smoke \
  --bundle "$GOD_BUNDLE_FILE" \
  --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256" \
  --rpc "$GOD_REVIEWED_RPC_URL" \
  --companion "$GOD_REVIEWED_COMPANION_URL" \
  --samples 3 \
  --interval-seconds 5 \
  --max-block-age-seconds 30 \
  --minimum-peers "$GOD_REVIEWED_MINIMUM_PEERS"
```

The tool verifies the entire bundle before making requests. It checks all eight fixed companion resources against the bytes, MIME types and security headers embedded in its own reviewed build. Use matching binaries for this comparison; an intentional interface update requires a reviewed matching verifier. Redirects, credential URLs, environment proxies, insecure TLS bypasses and unexpected resource compression are not accepted. The checker has no cookie jar and sends no credentials; resource and origin-response checks also reject `Set-Cookie`. Resource checks share an eight-second deadline and a 128-KiB per-resource limit.

The RPC checks include the companion's exact browser-origin preflight, rejection of the opaque `null` origin and an actual read response with the same origin. Wildcard, ambiguous or credential-sharing permissions fail. Each availability sample reads only `god_liveness` and `god_network`, checks the pinned synthetic network and fixed supply, and applies the same freshness/peer rules as `health`. Heights cannot regress; time cannot regress; a repeated height cannot change its reported time or application hash. At least one new committed height must be observed.

Choose 2 through 10 samples and 1 through 10 seconds between them. Each pair of reads shares an eight-second deadline and the whole run is limited to three minutes. Failures and cancellation stop the check without retrying, restarting anything or writing configuration. Redacted reports contain counts, heights and fixed labels; they do not copy URLs, bundle identities, hashes or provider errors. A transient unhealthy sample can fail the run; investigate and rerun manually rather than restarting a validator automatically.

`checksPassed` means these short service samples passed, not authenticated state, finality, operator independence or launch approval. `browserExecutionVerified`, `transactionsSubmitted`, `publicAcceptance` and `realAssets` remain false. Continue with an actual disposable-wallet/browser workflow, a sustained run across UTC-day settlement, secured target-host checks and independent nodes. The checker does not create accounts, request signatures, submit funding or replace those acceptance gates.

## Foreground startup and safe shutdown

Keep the node and companion in separate foreground terminals during initial host acceptance. Use the reviewed node workspace and explicit loopback companion listener:

```sh
./build/godd testnet check --home "$GOD_NODE_DIR"
./build/godd testnet start --home "$GOD_NODE_DIR"
./build/godd companion --listen "$GOD_COMPANION_LOOPBACK_LISTEN"
```

The last two commands are long-running processes, not a single sequential script. A reviewed non-loopback peer configuration additionally requires the existing explicit `--allow-network` option and separate public-deployment approval. Use Ctrl-C or SIGTERM to stop each foreground process and wait for its exit before reopening the same workspace. Check subsequent progress against the existing signing state. A smoke-check failure is not permission to delete a database, reset a signer, restore older signing progress or enable public listeners.

Review and validate supervision separately after foreground checks pass. Neither read-only command installs, enables, starts, stops or restarts systemd/nginx services.

## HTTPS gateway template

The placeholder `deploy/nginx-http.conf.template` belongs inside a reviewed nginx `http` block. It separates observer RPC from the companion, proxies only to local services, preserves exact origins and refuses unknown hosts/paths. RPC accepts POST and browser preflight OPTIONS; the companion accepts only GET/HEAD of its eight fixed static assets, including the wallet guide and test explorer. Upstream retries and payload logs are disabled, forwarded credentials are stripped and per-client request/active-request limits apply through the official [request limiting](https://nginx.org/en/docs/http/ngx_http_limit_req_module.html) and [connection limiting](https://nginx.org/en/docs/http/ngx_http_limit_conn_module.html) interfaces. Active-request limits do not count every idle socket.

Supply private TLS files, explicit listen settings, distinct reviewed hostnames and the selected local ports. The observer must allow the numeric loopback host and the companion's exact HTTPS origin. The local companion must run separately under a supervised unprivileged process. Check the rendered configuration with `nginx -t`, then test valid and invalid origins, unsupported methods, large requests, rate limits, wallet submissions and unknown outcomes on the intended host. Request/response behavior and retry controls follow the official [proxy documentation](https://nginx.org/en/docs/http/ngx_http_proxy_module.html); TLS setup follows the [SSL documentation](https://nginx.org/en/docs/http/ngx_http_ssl_module.html).

This is a direct-client template. It deliberately does not trust forwarded-IP headers or configure a CDN/proxy chain. Adding a CDN requires a reviewed trusted-proxy model and firewall layout; otherwise per-client limits can misidentify everyone as one proxy or accept spoofed identities. A certificate and template do not establish availability, DDoS protection, resource suitability or public-deployment approval. Nothing is installed or exposed by these tools.

## Read only health checks

The health command reads a reviewed observer or local node using exactly `god_liveness` followed by `god_network`. It first verifies the complete private bundle digest. No wallet or node signing key is required.

```sh
./build/godd testnet health \
  --bundle "$GOD_BUNDLE_FILE" \
  --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256" \
  --rpc "$GOD_REVIEWED_RPC_URL" \
  --max-block-age-seconds 30 \
  --minimum-peers "$GOD_REVIEWED_MINIMUM_PEERS"
```

The URL must be HTTPS or numeric loopback HTTP at the root, without credentials, query or fragment. Redirects and environment proxies are disabled; standard TLS validation remains enabled. Requests share an eight-second deadline, bounded headers and 16-KiB responses. This command never signs, submits, starts/stops a node or writes configuration.

Checks include exact chain binding, fixed GOD supply, synthetic flags, positive committed height, canonical quantities, fresh block time, usable running state, synchronization and a configured peer threshold. The default peer threshold is zero for local single-node checks; operators must select a reviewed threshold for their topology. A peer count does not prove voting power or independent ownership. A short sampling overlap or catching-up result can be transient; inspect repeated samples rather than restarting a validator blindly.

The JSON report contains only fixed reason labels, height, age, peer count and commit phase. A failed check returns a nonzero exit code and a static error without copying endpoints, paths, identities, hashes or response material into logs. `healthy: true` means that sample passed, not authenticated state, quorum, bridge backing or public-launch acceptance. `publicAcceptance` and `realAssets` remain false.

Use an operator-reviewed scheduler for repeated sampling and an incident channel for persistent failures. No recurring task, alert destination or automatic recovery action is configured here. Track CPU, memory, disk growth and bandwidth on the target host separately; the health report does not invent resource measurements.

## Host acceptance before public testing

Complete native Linux startup, database locking, graceful stop/restart and reboot checks. Then exercise independently controlled hosts, observer propagation, validator outage, loss of quorum and catch-up without resetting signatures. Check actual disposable-wallet submission, receipts, manual test funding and participant entry through the intended secured gateway.

Run through a real UTC-day boundary to verify pending-G settlement, claim, transfer, empty-liquidity preservation and voluntary redemption. Measure ordinary-computer resources and define funding budgets, incident contacts and signer-safe recovery. Short local tests and a valid package do not satisfy these gates. Public deployment, binary publication and real-asset activation require their own explicit approval.
