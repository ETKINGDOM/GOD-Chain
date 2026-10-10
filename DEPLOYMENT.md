# GOD Chain private host testing tools

The deployment tools prepare a private Linux acceptance candidate for God EVM + God SDK + GodCometBFT. They package reviewed binaries, safely unpack a fixed inventory into a new private staging directory, retain dependency notices and provide read-only staging and health checks. They do not install services, initialize wallets, distribute validator keys, authorize public deployment or connect real assets. A target machine is still required to establish Linux runtime behavior.

Use [TESTNET.md](TESTNET.md) for independent initialization and recovery, and [COMPANION.md](COMPANION.md) for participant testing. Test GOD and G have no monetary value or guaranteed mainnet conversion. Keep RH routes and real-asset commands disabled.

The selected initial topology is [one founder-operated host with multiple
services](SINGLE_HOST.md). Separate validator/observer processes, gateway
boundaries and private stores share that machine; they are not separate physical
servers. Measure their aggregate resources and preserve quorum/signing safety.
Additional validator machines are a later expansion, not a requirement to
continue local development. Whole-host failure, independent backup custody and
out-of-host outage detection still require an explicit recovery review.

## Build and review

### Cooperating-process node workspace lease

The local `godd testnet start` candidate acquires an exclusive, nonblocking
advisory lease before loading node identities, opening databases or binding
listeners. The empty owner-only `.god-node-lease` file remains at its original
inode across normal exits. Never delete, replace, truncate or copy an active
lease to bypass refusal. The descriptor stays held through consensus stop/join
and application-storage close. Duplicate startup refuses rather than resetting
a signer or reopening the same working data.

Linux/macOS candidates reject linked, shared, populated and wrong-owner lease
files; other platforms have no unverified fallback. `make test-node-lease
check-node-lease` covers unsafe-file refusal, reacquisition and an actual private
daemon duplicate/refusal/restart with retained identities and advancing signing
progress. This work remains local and does not upgrade the deployed pilot.

This is one same-home cooperating-process guard, not a complete immutable
release/migration/rollback workflow. Older executables and direct internal
runtime users do not take this lease. It cannot fence copied identities on
another host, resist the host owner, establish backup freshness or make an old
signing-state restore safe. Linux target execution, retained-state upgrade
compatibility, external fencing and independently retained checkpoints remain
separate gates. Do not restore chain or signer state when rolling back software.

### Checksum-bound candidate build

Use Go 1.26.8 and the checksum-bound lifecycle build with unchanged module pins:

```sh
make check build compile-targets
```

The native `build/godpack` command prepares, verifies or safely stages archives without network access. Linux amd64 and arm64 builds contain separate `godd` and `godpack` executables. Only an intended host can validate those executables' runtime behavior. Windows private node operations remain disabled pending ACL support; cross-compilation is not runtime acceptance.

For a package-tool-only check and build, use `make test-node-package check-node-package build-node-package compile-node-package`. These targets retain the checksum-bound lifecycle review and unchanged dependency pins. The native tool is written to `build/node-package/godpack`; diagnostic foreign builds stay under `build/node-package/`. This neither rebuilds a running node nor distributes an installer. The examples below use the ordinary full-build tool path.

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

Verification checks the trusted archive digest, canonical manifest, fixed inventory, file hashes and sizes, target ELF header, permissions and absence of links, duplicate paths, traversal, extra members and trailing data. It neither extracts nor executes content. Digest verification and parsing use one immutable bounded snapshot rather than reopening a mutable file. The compressed archive limit is 288 MiB; snapshot allocation and parser overhead need an operator-reviewed memory budget. This is a tool bound, not a measured node hardware requirement. Verification is not a publisher signature or source audit. Identical inputs produce an identical archive.

These files remain private and ignored. They are not a GitHub binary release. The manifest explicitly retains `runtimeVerified: false` and `binaryRedistributionReviewed: false`. Existing notices are preserved, but complete dependency-license and source obligations require separate review before public binary distribution.

## Prepare the target machine

Transfer only the reviewed acceptance archive and an independently reviewed package tool for the matching host architecture through a private operational channel. Compare their digests over a trusted channel before execution. On Linux or macOS, use the explicit safe staging command after review. It is not a service installer or an automatic node launcher.

```sh
./build/godpack unpack \
  --archive "$GOD_PRIVATE_ARCHIVE" \
  --expected-archive "$GOD_REVIEWED_ARCHIVE_SHA256" \
  --destination "$GOD_NEW_PRIVATE_STAGING_DIR"
./build/godpack check-unpacked \
  --archive "$GOD_PRIVATE_ARCHIVE" \
  --expected-archive "$GOD_REVIEWED_ARCHIVE_SHA256" \
  --destination "$GOD_PRIVATE_STAGING_DIR"
```

The staging destination must be a new, absolute, clean path beneath an existing owner-only parent. Do not pre-create it or point at an existing node, wallet or home directory. The unpack command verifies the complete archive before creating that directory and writes only its fixed inventory through a confined filesystem root. It refuses existing destinations, symlinks and unsafe parents. Directories and `bin/godd` are mode 0700; other files are mode 0600. Files and directories are synchronized and rechecked before success. Archive bytes and dependency notices remain unchanged; executable permission does not mean execution approval.

This is not an atomic whole-directory installation or physical power-loss certification. An interrupted or failed write leaves a new private partial directory for manual review; no success report is returned, nothing is removed and retrying into that same path is refused. Never start a node from a failed directory. The read-only `check-unpacked` command compares the exact archive pin, bytes, inventory and permissions, rejecting missing, altered or additional material and special privilege bits. It neither repairs files nor refreshes the trusted pin.

Successful staging reports `unpacked: true` and `directoryVerified: true`; a successful later read-only check sets only the latter. Both retain `servicesInstalled: false`, `nodeStarted: false`, `runtimeVerified: false` and `binaryRedistributionReviewed: false`. Checks are point-in-time evidence on trusted owner-controlled storage, not protection against subsequent writes by the same owner, authenticated configuration delivery or permission to activate a public peer. Windows private staging remains disabled pending ACL support.

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

## Finite Linux resource observation

`godd testnet host-observe` samples only explicitly reviewed same-user process
leaders and an existing owner-only data directory. Run it as the selected
unprivileged service user, not root. Select PIDs locally from reviewed service
metadata; the command has no discovery, shell or service-manager integration.
Do not run it against another operator's processes. Separate service users need
separate observations; this is not an all-service/cgroup inventory.

```sh
./build/godd testnet host-observe \
  --data-dir "$GOD_PRIVATE_DATA_PARENT" \
  --pids "$GOD_REVIEWED_PROCESS_LEADERS" \
  --samples "$GOD_REVIEWED_SAMPLE_COUNT" \
  --interval-ms "$GOD_REVIEWED_SAMPLE_INTERVAL_MS" \
  --budget-seconds "$GOD_REVIEWED_OBSERVATION_BUDGET_SECONDS" \
  --minimum-free-bytes "$GOD_REVIEWED_FREE_SPACE_BUDGET" \
  --minimum-memory-bytes "$GOD_REVIEWED_MEMORY_HEADROOM_BYTES" \
  --maximum-rss-bytes "$GOD_REVIEWED_AGGREGATE_RSS_BYTES" \
  --maximum-open-files "$GOD_REVIEWED_AGGREGATE_FILE_COUNT"
```

Every flag is required exactly once. Select 1–32 distinct process leaders,
2–60 samples, intervals of 250–5,000 milliseconds and a 2–180-second overall
budget. The requested intervals plus one second must fit the budget. Disk,
available-memory and approximate aggregate-RSS budgets each accept 1 MiB–1 PiB;
aggregate open descriptors accept 64–65,536. These are tool limits, not
recommended service sizing, measured hardware requirements or user capacity.
One baseline plus the requested samples is the maximum; there are no retries.

The fixed Linux procfs reader checks ownership, leader identity and start time,
directory identity and permissions, bounded input sizes and checked arithmetic.
It stops on a missing process, changed identity/scope, regressing or inconsistent
counters, insufficient disk/memory, excessive RSS/descriptors, cancellation or
elapsed budget. Its descriptor reader counts names, never their target contents.
It does not read keys, node stores, command lines or environments, use the
network, write files, send process signals, change limits or restart anything.
SIGINT/SIGTERM cancel the foreground command. Kernel/filesystem calls can block;
the deadline is cooperative between reads. Use a separately reviewed external
supervisor when a hard syscall deadline is needed, not an automatic node reset.

JSON lines contain aggregates and fixed reasons, not PIDs, names, start times,
private paths or raw kernel data. Byte counts use exact decimal strings. The
CPU ratios are rounded down to permille of the full procfs CPU view, not one
core: 250 means one quarter of that view. CPU is observed, not budget-tested.
Guest CPU time is not counted twice. Available memory is a kernel estimate;
summed process RSS is approximate and can double-count shared pages. The procfs
view may exceed a container/service limit and does not establish cgroup
headroom. These semantics follow the [Linux procfs documentation](https://docs.kernel.org/filesystems/proc.html).

`checksPassed` means only that this short series met its explicit budgets.
`hardwareCapacityVerified`, `publicAcceptance`, `servicesChanged` and
`realAssets` remain false, even on Linux. macOS/Windows fail with
`unsupported-runtime` and do not substitute fixture metrics. Sustained workload,
disk growth/bandwidth, actual ingress, cgroup limits, backup/restore and physical
host acceptance remain separate. Never publish private operational reports or
turn a passing short series into a participant/TPS promise.

`make test-host-observe check-host-observe fuzz-host-observe` runs portable
policy/parser and CLI checks. `make build-host-observe` prepares a private native
binary. `make compile-host-observe-linux` compiles Linux amd64/arm64 binaries and
kernel-adapter test executables without executing or releasing them. Linux
adapter runtime checks must still run as the intended user on the intended host;
foreign compilation alone is not acceptance. All build artifacts stay ignored,
and these targets neither install services nor update the public website.

## Private stopped-observer cold-copy drill

`make test-cold-observer-copy check-cold-observer-copy` uses only disposable
synthetic workspaces, the checksum-bound lifecycle/logging build and numeric
loopback processes. It is a private acceptance fixture, **not an operator
snapshot/restore command**, a public archive or an automatic backup service.
Four validators and one non-signing observer run on one computer.

The fixture commits a signed GOD payment, joins the stopped observer's process
and closes its stores before inspecting any database. It records a bounded
file/size/digest inventory and copies that closed workspace into a new private
directory without changing the original. Validator/candidate roles are refused;
source changes, symbolic links and existing destinations fail rather than being
overwritten. The fixture limits depth, directories, file count and read bytes;
its caller establishes quiescence, not an inferred PID or a filesystem checksum.

While the original observer stays stopped, the four validators commit another
reviewed synthetic payment. Only the good retained observer copy is then
started. Checks require both real receipts, the exact recipient balance and
sender sequence, the old checkpoint block and a later block matching all four
validators. Consensus/peer identities, genesis/configuration and zero observer
signing progress remain unchanged. A separate disposable copy receives logical
configuration corruption; inspection must reject it without repair. This is
not physical disk/page corruption or fault injection into live storage.

These are cold-file-copy and local consensus checks, not proof of secure
off-host backup custody, authenticated retained checkpoints, power-loss
durability, storage encryption, adversarial local filesystem safety, target
Linux restoration or protocol upgrade/rollback. The fixture copies only its own
synthetic observer identity. Never copy a running node or use this test helper
on operator directories. Never resume a validator from an older backup: lost
uncommitted signatures and rollback require a separately reviewed fencing and
signer-state recovery policy. There is no validator restore, database reset,
automatic repair, public service change or real-asset operation in this target.

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

## Bounded concurrent read checks

Use `read-load` for a short, explicit load on a reviewed **synthetic operator
RPC**, not as a continuous benchmark or an unrestricted public traffic source.
The complete private bundle digest is verified before any network request. The
only methods are `god_liveness` and `god_network`; an operator endpoint must
already permit those diagnostics. This command does not expand the public
gateway's method allowlist or expose validator RPC.

```sh
./build/godd testnet read-load \
  --bundle "$GOD_BUNDLE_FILE" \
  --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256" \
  --rpc "$GOD_REVIEWED_OPERATOR_RPC_URL" \
  --samples 16 --concurrency 2 --interval-ms 200 \
  --maximum-latency-ms 2000 --budget-seconds 30 \
  --max-block-age-seconds 60 \
  --minimum-peers "$GOD_REVIEWED_MINIMUM_PEERS"
```

Sample count, concurrency, interval, maximum sample latency and overall budget
are mandatory. Their bounds are 2–64 samples, 1–8 concurrent samples (no more
than the sample count), 50–2,000 milliseconds between dispatches,
1–8,000 milliseconds per measured pair and 10–180 seconds overall. The nominal
dispatch intervals must fit strictly inside the budget; controls and request
time can still exhaust it. The interval is a pause between dispatches. Waiting
for a concurrency slot can extend it; there are no queued catch-up bursts. The
overall monotonic-time deadline and per-sample deadlines cancel pending reads.
Freshness and peer bounds/defaults match `health`.

One unmeasured baseline precedes the samples and one unmeasured final control
follows them. Each control has an eight-second request-pair deadline within the
overall budget. Total RPC attempts are at most `2 × (samples + 2)`, or 132 at
the maximum count. Redirects, environment proxies, cookies, compression and
retries remain disabled; standard TLS validation and the health response-size
and schema limits remain enabled. No wallet, key, signing, transaction, faucet,
contract action, node-management operation or configuration write is involved.

Every pair must pass synthetic flags, exact chain binding, fixed GOD supply,
freshness, overlap, usable state and peer checks. Concurrent replies may arrive
out of height order, but must be consistent with the baseline and reported
height/time ordering. Repeated heights must retain the entire network snapshot,
not only a matching hash. The final control cannot regress behind any measured
view, and must show height progress beyond the baseline. First failure stops
new dispatch, cancels in-flight reads, joins them and returns a nonzero status;
there is no automatic retry, failover, repair or restart.

The single redacted JSON summary includes started/completed/passed samples,
maximum in-flight pairs, RPC **attempts**, actual elapsed milliseconds, endpoint-
reported height progress and minimum/P50/P95/maximum pair latency. RPC attempts
are not a count of requests delivered or transactions. Latencies include every
completed measurement, including failed/canceled partial pairs, but not controls
or time waiting for a slot. Percentiles use nearest rank, rounded up to whole
milliseconds. A small or failed series is not a capacity estimate. No URL,
bundle identity, path, account, state hash or raw provider error is reported.

`checksPassed` establishes only this bounded provider-reported read check.
`realAssets`, `publicAcceptance`, `transactionsSubmitted` and
`hardwareCapacityVerified` remain false. TLS and consistency checks do not
authenticate consensus proofs. The command does not measure CPU/RAM, disk growth,
bandwidth, gateway queues, full wallet/NFT write traffic, long-run uptime or
physical recovery; those intended-host release gates remain separate. Run
`make test-read-load check-read-load` for the private loopback acceptance and
unchanged checksum-bound build, without contacting a public service.

## Bounded local mixed-wallet acceptance

`make test-mixed-wallet-load check-mixed-wallet-load` exercises disposable
synthetic consensus and the existing independent plain-transfer, NFT, read-only
and automatic-faucet boundaries on numeric loopback. It requires the retained
private `build/nft/GodTestNFT.json` from the reviewed `compile-nft` workflow.
The fixture checks compiler/library metadata, locked dependency sources and the
current collection/receiver source digests. Missing or mismatched artifacts
fail; there is no automatic download, compilation, public collection deployment
or configured-token use. The checksum-bound consensus build remains required.

Four validators and one non-signing observer run on one computer. Disposable
accounts and the compiled collection are funded/deployed only inside that
private synthetic fixture. Twelve mutation-boundary requests start together:
four copies each of one signed GOD transfer, one signed NFT mint and one claim
ID. Four network reads follow at a controlled 600-millisecond offset. A local
relay caps upstream connections at three; setup and later phases wait for real
budget replenishment. These controls do not change production quotas, block
intervals, chain time, gateway allowlists or the intended public ingress.

The fixture loses only acknowledgments of actual admitted submissions, keeping
the original hashes and obtaining genuine committed receipts. It checks one
GOD payment, one mint, one faucet payment and a separately reviewed NFT owner
send; final ownership and the mint's original recipient remain distinct. A
reopened private faucet journal and its HTTP claim lookup must recover the same
confirmed claim. The original ID stays idempotent and a new ID cannot bypass the
account cooldown. Exactly four transactions cross the relay, once each.

Keyless-gateway recreation is tested **after committed inclusion**: old intents
must fail preflight without another broadcast. This does not establish durable
gateway reservations across a pre-inclusion crash. Only reads may retry during
lookup recovery; there is no automatic wallet/NFT re-signing, replacement or
resubmission. The observer then restarts, preserving committed balances,
receipts and NFT ownership. All nodes retain their original keys, genesis and
signer roles/progress; no live data directory is opened or reset.

Rate/capacity refusal is not payment. Read-side temporary unavailability is an
explicit failed read, not a fabricated successful network result. The redacted
summary records controlled request counts, limit/unavailability counts, exact
forward counts and recovery checks. `publicAcceptance`, `realAssets` and
`hardwareCapacityVerified` remain false. This short correctness drill is not a
concurrent-user limit, production throughput measurement, browser/device test,
native staking/G workflow or target-Linux acceptance. Full simultaneous traffic,
intended-host resource sizing, sustained settlement, storage/power failure and
safe upgrade/restore gates remain separate. No public service, repository,
binary release or real bridge is activated by this target.

## Bounded local native concurrency and unknown recovery

`make test-native-concurrency check-native-concurrency` uses the checksum-bound
native-gateway build and five disposable persistent loopback processes: four
validators and one non-signing observer on one computer. The fixture funds two
ordinary accounts only with worthless synthetic GOD. It does not contact a
configured public endpoint or change quorum, quotas, fees, settlement clocks,
unbonding delays, keys, live data directories or service configuration.

Stopping two validators genuinely pauses commits. Eight identical signed
delegation requests start together; exactly one reaches node submission and
seven are limited. A relay discards only that node's actual successful admission
acknowledgment. The committed transaction lookup must still be empty, so the
result is genuinely **unknown before inclusion**, not a fabricated receipt or
post-inclusion gateway test. The relay allows at most three upstream connections
and only fixed reads and individually reviewed signed wires.

After draining/closing the gateway, an exclusive offline audit and checksum-pinned
copy retain its unknown hash. A gateway opened on that fresh private copy refuses
the original intent without another submission; status remains unknown while
commits are paused. Restoring three-validator quorum produces a real matching
receipt through read-only lookup. Never turn an unknown write into an automatic
retry, replacement or re-signature. This tests identical-hash protection, not a
cross-hash account/nonce reservation or complete crash-safe wallet coordination.

Separately signed GOD donation and undelegation commit once each. Checks retain
the exact offered fees, balances and pending principal with the unchanged
21-day unbonding delay. A never-delegated account with zero settled/pending G
then attempts claim, transfer and redemption. All three enter real committed-state
preflight with successful upstream reads and fail without a node submission,
fee or sequence change. The full committed network snapshot, including GOD
pool/supply and G totals, stays unchanged; RPC failure is not counted as a
successful negative control. The pending exit's HTTP view matches its actual
committed query. This is
negative G-state acceptance, **not positive G settlement or completed payout**.

Closed-store audits retain exactly three attempted hashes and leave the original
one-record source unchanged. A deliberately malformed logical record in a
separate disposable copy makes gateway open and offline audit fail without
repair; the good copy remains usable. No physical pages, node databases or
original stores are damaged. This does not certify filesystem/power-loss
durability, authenticated checkpoint retention or rollback detection.

All five nodes retain consensus/peer identities and genesis; validators retain
positive signing progress and the observer retains none. Race regressions,
scoped vet and native-wallet client checks complement the new fixture.
`publicAcceptance`, `realAssets` and `hardwareCapacityVerified` remain false.
Full mixed intended-ingress load, target-host resources, sustained real elapsed
G settlement, physical devices, signer-safe upgrade/restore and real bridge
acceptance remain separate. No public service, repository or binary is updated.

## Finite read only observation and fault reports

Use `watch-health` for an explicitly bounded foreground run against one reviewed
synthetic RPC. A sample reads only `god_liveness` and `god_network` after the
complete private bundle has been verified. It uses the same TLS, request-size,
fixed-supply, freshness and peer checks as `health`. No account or node key is
required, and no process, file, chain clock or service is changed.

```sh
./build/godd testnet watch-health \
  --bundle "$GOD_BUNDLE_FILE" \
  --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256" \
  --rpc "$GOD_REVIEWED_RPC_URL" \
  --samples 60 --interval-seconds 5 \
  --max-block-age-seconds 60 \
  --maximum-no-progress-seconds 120 \
  --minimum-peers "$GOD_REVIEWED_MINIMUM_PEERS"
```

Count and interval are mandatory. Counts are 2 through 10,080, intervals are 1
through 600 seconds, and the conservative total interval plus eight-second
per-sample budget must fit within seven days. The no-progress threshold is 10
through 600 seconds and must be at least the interval. The interval is a pause
after each completed sample, not a promise of an exact wall-clock schedule.
Actual elapsed seconds are recorded; no clock acceleration or synthetic uptime
is used by the command. Cancellation interrupts waits and in-flight requests.

Each output JSON line is a redacted `sample`, followed by one `summary` for the
run. The first unavailable, stale, mismatched or regressing view stops the run
with a nonzero exit code. Repeated heights cannot change their reported time or
application hash. No advancing height within the selected threshold fails;
even a shorter run with no observed progress fails its final summary. A health
sample may pass while the separate continuity check fails, so inspect
`observationPassed`, `reason` and the summary rather than `health.healthy` alone.
Output failure also fails the run. Fixed reasons are suitable for a separately
reviewed alert adapter; raw provider errors, account data, keys, URLs and hashes
are not copied into the report.

This is a first-fault observation tool, not a continuous alerting service. It
never retries, changes providers, sends notifications, repairs databases,
restarts validators or resets signing progress. An incident destination and
schedule require their own review. Point-in-time provider continuity is not
authenticated finality, quorum, operator independence, a resource benchmark or
an uptime SLA. Real sustained runs, CPU/memory/disk/network measurements,
independent-node comparison and signer-safe backup/recovery remain acceptance
gates. No such long run or external alert is activated by the local tests.

## Host acceptance before public testing

Complete native Linux startup, database locking, graceful stop/restart and reboot checks on the selected single host. Exercise observer propagation, individual validator process outage, lost quorum and whole-group restart/catch-up without resetting signatures. Check all services' aggregate resource limits and actual disposable-wallet receipts/funding/entry through secured gateways. Independently controlled hosts and remote admission/routing remain later distributed-stage acceptance, not evidence supplied by this single-host drill.

Run through a real UTC-day boundary to verify pending-G settlement, claim, transfer, empty-liquidity preservation and voluntary redemption. Measure ordinary-computer resources and define funding budgets, incident contacts and signer-safe recovery. Short local tests and a valid package do not satisfy these gates. Public deployment, binary publication and real-asset activation require their own explicit approval.
