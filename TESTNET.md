# GOD Chain synthetic testnet deployment guide

The synthetic GOD Chain testnet runs God EVM + God SDK + GodCometBFT with persistent node storage and a restricted JSON-RPC service. It is intended for public testing with disposable accounts and assets that have no external backing or monetary value. It does not connect to RH, activate a bridge, or authorize further deployment. Mainnet and real-asset commands remain disabled. Public test-GOD claims use the automatic service in FAUCET.md (separate pilot guide, not included in this core snapshot), with no registration or human approval. The separate local unsigned-request interface and offline operator funding helpers are described in [COMPANION.md](COMPANION.md).

GOD Chain's public testnet operates with four validator nodes. Deployment across four geographic regions is planned. The current pilot is not an independently operated, geographically distributed network. A non-signing observer serves restricted public read-only RPC and committed block/account views. The separate automatic faucet does not enable general public transaction submission. The connected [test wallet](WALLET.md) signs plain GOD transfers and restricted collection NFT operations locally, using separate keyless submission gateways. The Chrome extension (separate pilot guide, not included in this core snapshot) is a downloadable desktop test alpha connected through one reviewed exact origin; it is not a Store release or an external website signer.

Local checks cover four persistent validators, independently initialized identities, a non-signing observer, signed EVM transfers and contracts, native delegation and recovery. The first encrypted disposable [test wallet](WALLET.md) and bounded [test explorer](EXPLORER.md) are available locally. These ran on one macOS computer. An independently operated multi-region deployment still requires separate hosts, secured infrastructure and the acceptance checks below; production wallet and explorer acceptance remain separate work.

## Public pilot scope and remaining work

The current HTTPS pilot supports basic participant testing, not every mainnet
feature. Its website, network and transaction tools use worthless synthetic
assets; they do not promise persistence, independent validation or conversion
into real GOD. Participants can begin with the web wallet without waiting for
a browser extension or an RH bridge.

| Area | Current pilot scope | Remaining work |
| --- | --- | --- |
| Web wallet and funding | Local encrypted creation/recovery, fixed automatic test claims, reviewed GOD transfers, known TX IDs and committed recipient checks. | Broader browsers/devices, sustained participant acceptance and independent security review. |
| Restricted NFTs | One configured collection, locally reviewed mint/send, checked receipts and current ownership cards. | General collection tools, NFT history and independent indexing. |
| Explorer | Committed blocks/transactions, checksum-matched watch-only accounts and manual five-block sender/recipient scans. | Complete address/NFT history, complete validator indexing and proofs. A bounded scan is not a full index. |
| Chrome wallet | Downloadable alpha with a fixed reviewed identity, exact-origin service activation, English installation/hash guide and actual HTTPS create/recover/claim/transfer/NFT acceptance. | Physical Chrome/device acceptance, transient read recovery, website connection permissions, external provider signing, independent security review and a separately authorized Store release. |
| Staking and G | Connected web/Chrome six-action signing, bounded committed unbonding progress and locked one-operator validator lookup; public delegation and start-unbonding receipts with a real pending entry. | Full delayed payout, positive settled-G claim/transfer/redemption and fuller validator indexing over sustained consensus. A configured record is not an uptime, ownership or location proof, and delegation is not web mining. |
| Operations | Supervised services, bounded health snapshots, abuse limits and safe frontend rollback. | Sustained load/resource measurements, alerting, storage-growth policy and independently reviewed recovery drills. |
| Independent validators | Four pilot validator processes. | Independently controlled hosts, cross-host acceptance and signing-safety review. Four geographic deployments remain planned, not established. |

Real assets and the RH bridge remain disabled. Bridge backing, finality,
custody and signer activation are separate gates, not prerequisites for basic
worthless-asset wallet testing. No readiness table substitutes for the broader
acceptance checklist below.

## Build requirements

Use the reviewed Go 1.26.8 toolchain and the existing dependency lock. The testnet package requires Go 1.25 or newer for confined private-directory access; explicit build constraints reject older toolchains. This does not upgrade the protocol dependencies or change the module lock.

```sh
make check build
make compile-targets
```

Every target selects the checksum-bound GodCometBFT lifecycle build. Never bypass it with an unpatched node build. Compilation for Linux, macOS or Windows does not establish native runtime support. Windows private node operations are disabled until owner-only ACL behavior is implemented and verified. Generated node binaries remain private; there is no node installer, binary publication or automatic deployment. The separately authorized Chrome test-alpha ZIP is not a node binary release.

## Network layout

Use at least four independently controlled validators with equal initial voting power and one separate observer for public RPC. Four equal validators can keep committing with one unavailable validator, but not two. A cluster generated by one developer has multiple processes, not independent ownership or demonstrated decentralization.

Validator RPC must bind loopback. Restrict validator P2P admission with host firewalls and reviewed peers. The observer replicates state and accepts transactions for propagation, but its signer refuses votes and proposals. Expose its RPC only through a separately hardened HTTPS gateway with per-client request limits, connection limits and timeouts. Never put a wallet account signing key on that service. Observer nodes are not a complete sentry architecture.

P2P listeners and peer endpoints require explicit numeric IPs and nonzero ports. Discovery, external engine RPC, gRPC, profiling and metrics remain disabled. Public reachability and NAT rules must be tested on the intended hosts. Network listeners or remote peers require the explicit `--allow-network` option; it does not enable RH or real assets.

Non-loopback operation rejects duplicate peer IPs; the private loopback cluster is the explicit exception. Multiple peers behind one public NAT address therefore need a reviewed network layout rather than only different ports. Assembly verifies distinct endpoints, not router reachability or independent ownership. Test the intended validator and observer topology before launch.

## Private storage

Create a dedicated owner-only parent directory outside the public checkout. Node directories require mode 0700 and operational files require mode 0600 on supported Unix systems. Commands reject shared files, symlinks, ambiguous JSON and incompatible identities. They do not automatically change broad-directory permissions, overwrite existing workspaces, repair missing signers or reset signing progress.

Keep profiles, bundles, node configuration, genesis, wallets, signing progress, databases and logs out of source publication. Exchange only `profile.json` and the final bundle through a reviewed operational channel. Public profile fields still contain addresses and endpoints, so they do not belong in the sanitized public repository. Never exchange consensus private keys, P2P private keys or wallet keys.

Commands print redacted capability reports, not endpoints, addresses or private paths. Requests, raw transactions, private faith content and credential values must not be copied into server logs or issue reports.

Startup failures expose only fixed phase and category labels. A `rpc-listener (port-in-use)` failure occurs before the application database or consensus engine is opened. Resolve the conflicting service or review a new consistent port allocation; never clear signing progress, databases or identity files to remedy a port conflict.

## Local development cluster

Choose an unused contiguous port range and a new cluster path within an existing private parent. Reserve service ports outside the host's ephemeral outgoing-connection range where possible; releasing a temporary port probe does not reserve it for a later process. Variables below are operator-supplied placeholders, not published settings.

```sh
./build/godd testnet create --home "$GOD_CLUSTER_DIR" --validators 4 --first-port "$GOD_FIRST_PORT"
./build/godd testnet check --home "$GOD_NODE_DIR"
./build/godd testnet start --home "$GOD_NODE_DIR"
```

`create` provisions `node-1` through `node-4` for this local example. Start each in a separate process. It also writes disposable operator wallet keys at the cluster root, outside all node directories. This shortcut is for private development only; do not distribute these developer-held keys as a public launch.

Each validator receives 1,100 synthetic GOD, of which 1,000 is self-staked. The remaining fixed supply stays in restricted reserve. The synthetic policy uses 18 decimal places, a one-G network allocation budget per block before eligibility, rounding and commission, a 10,000 G UTC-day ceiling, and the existing daily settlement rules. These are test settings, not a finalized mainnet schedule. Test funding does not mint beyond the fixed 1,000,000,000 GOD supply.

## Independent operator initialization

Each validator uses an externally held disposable EVM-compatible account as its staking owner. The identity command creates only consensus and P2P keys. Its two profile signatures prove possession of those node keys, not wallet ownership, human independence or mainnet launch consent.

Run on each operator's machine with a new private node directory:

```sh
./build/godd testnet identity \
  --home "$GOD_NODE_DIR" --role validator --owner "$GOD_TEST_OWNER" \
  --endpoint "$GOD_ADVERTISED_P2P" --p2p "$GOD_P2P_LISTEN" \
  --rpc "$GOD_LOOPBACK_RPC_LISTEN" --hosts "$GOD_EXACT_RPC_HOSTS"
```

The advertised endpoint is an IP and port, without a scheme; the P2P listener uses the TCP scheme. The advertised endpoint may differ from a local bind address behind NAT. Independently verify that it reaches the correct peer key.

Initialize the separate observer without a wallet owner:

```sh
./build/godd testnet identity \
  --home "$GOD_OBSERVER_DIR" --role observer \
  --endpoint "$GOD_OBSERVER_ADVERTISED_P2P" --p2p "$GOD_OBSERVER_P2P_LISTEN" \
  --rpc "$GOD_OBSERVER_RPC_LISTEN" --hosts "$GOD_EXACT_RPC_HOSTS" \
  --origins "$GOD_EXACT_WALLET_ORIGINS"
```

Origins are optional exact HTTPS or supported extension origins; loopback HTTP is permitted for local testing. Wildcards and arbitrary remote HTTP origins are rejected. No default public hostname, wallet origin or contract setting is supplied.

## Assemble and review the launch bundle

Place the independently exported profiles in private files on the coordinator. Supply a comma-separated list of absolute profile paths. Assembly requires four to sixteen validators, permits additional observers within a maximum of 32 participants, and rejects duplicate operators, keys, endpoints or invalid profile signatures.

```sh
./build/godd testnet assemble --home "$GOD_CEREMONY_DIR" --profiles "$GOD_PROFILE_FILES"
```

The coordinator receives `testnet-bundle.json`, `bundle.sha256` and `genesis.sha256`. The bundle contains public identities, test funding, genesis and complete application configuration; it contains no private keys. Review the network identifiers, validator owners, voting powers, gas limits, G policy and all peers before acceptance.

Every operator must independently compare the complete bundle digest over a trusted channel. A digest copied from the same untrusted source is not authentication. Genesis alone does not bind all runtime parameters; `join` therefore requires the full bundle digest.

```sh
./build/godd testnet join \
  --home "$GOD_NODE_DIR" --bundle "$GOD_BUNDLE_FILE" \
  --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256"
./build/godd testnet check --home "$GOD_NODE_DIR"
```

Joining works only for a fresh initialized identity with empty signing progress and no data directory. It retains the operator's keys, requires an exact signed-profile match, and creates genesis and configuration without replacement. A second join fails. A partial or rejected setup requires inspection, not automatic deletion or signer reset. Run the offline `check` while the node is stopped so that its atomic signing-state replacement cannot race file inspection.

Start nodes only after separate deployment approval and infrastructure review:

```sh
./build/godd testnet start --home "$GOD_NODE_DIR" --allow-network
```

Use supervised processes with graceful SIGTERM shutdown and sufficient shutdown time. Do not run two copies of one validator identity. Database locks prevent concurrent use of one directory; they do not protect copies on different machines.

## Linux service template

This uninstalled systemd template is a starting point for an operator review, not evidence of Linux runtime acceptance. Replace every placeholder privately and validate the unit on the intended host before enabling it. The binary and node directory must be owned by the dedicated service user; protect the binary against modification by the running process. Confirm that the node directory is the one accepted by `check`.

```ini
[Unit]
Description=GOD Chain synthetic testnet node
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=@NODE_USER@
Group=@NODE_GROUP@
UMask=0077
ExecStart=@GODD_BINARY@ testnet start --home @NODE_HOME@ --allow-network
Restart=on-failure
RestartSec=10
KillSignal=SIGTERM
TimeoutStopSec=60
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=@NODE_HOME@
RestrictSUIDSGID=true
LimitNOFILE=4096

[Install]
WantedBy=multi-user.target
```

Place node data outside protected home directories when using this template. Never add reset, key-copy or database-deletion commands to startup. A repeatedly failing service requires operator investigation; automatic restarts cannot make a broken signer safe. Test graceful shutdown, database locking, host reboot and resource limits before public exposure.

## Available RPC methods

RPC accepts a single JSON-RPC 2.0 POST request at the root path. Batch calls and notifications are unsupported. Exact hosts and origins are required. The service bounds request bytes, response bytes, execution gas, open connections, concurrent requests and aggregate rate. These limits do not replace per-client controls at the HTTPS gateway.

| Methods | Scope |
| --- | --- |
| `web3_clientVersion`, `eth_chainId`, `net_version`, `eth_blockNumber`, `eth_syncing`, `eth_accounts` | Network identification and committed height. No server-held wallet accounts. Syncing returns an explicit error when the remote tip is unavailable rather than inventing it. |
| `eth_getBalance`, `eth_getTransactionCount`, `eth_getCode`, `eth_getStorageAt` | Latest committed application state. An exact current height is accepted; older state and arbitrary pending reads are not. Pending account sequence uses the ordered admission cache. |
| `eth_gasPrice`, `eth_maxPriorityFeePerGas`, `eth_call`, `eth_estimateGas` | Bounded gas information and discarded-cache simulation. Calls require an explicit sender. Unknown, blob and delegated-code arguments are rejected. |
| `eth_sendRawTransaction`, `eth_getTransactionReceipt` | Externally signed legacy, access-list and dynamic-fee transactions; actual indexed inclusion and EVM success or revert. Admission is not inclusion. |
| `eth_getBlockByNumber` | Stored committed block identifiers and ordered Ethereum transaction hashes. Full transaction objects are unsupported. No fabricated Ethereum state or receipt roots. |
| `god_network`, `god_account`, `god_delegation` | Exact decimal committed network, account and individual delegation views. Native and EVM account encodings read the same account. |
| `god_validator` | One canonical operator's committed stake/shares, commission, staking status and stored suspension/exclusion flags. The pilot wallet uses it only through a separately restricted query-only gateway; general public RPC is unchanged. No list scan, signing/uptime/ownership/location proof or complete validator index is implied. |
| `god_liveness` | Redacted running state, application/block-store heights, commit phase, connected peer count and consensus progress. No peer identities, endpoints, keys or private paths. |
| `god_submitTransaction`, `god_transaction` | Externally signed native SDK wire and consensus-indexed results. `sdkSuccessful` alone must not be interpreted as EVM success. |

Ethereum receipt and log indexes refer to the Ethereum transaction lane, matching the block's Ethereum hash list. `godConsensusIndex` retains the separate native consensus index; `godSdkGasUsed` distinguishes total SDK block gas. Consensus block hashes are GodCometBFT identifiers, not Ethereum RLP header hashes. This is a basic compatibility adapter, not a complete Ethereum API or a verified wallet integration.

Unknown receipt hashes return a null result. Unsupported signing, unlock, personal, admin, debug, log-filter, subscription and historical-state methods are not enabled. On `-32001`, retry only after commitment. A cancelled submission returns `-32005` with an unknown outcome: check the locally known transaction hash before deciding whether to retry, because cancellation does not retract an in-flight admission. Other failures must not be blindly retried. RPC results are not authenticated light-client proofs.

A local standard-client check used viem 2.56.9 to prepare a legacy transfer with automatic fee, gas and nonce queries, sign outside the node, submit it and confirm its actual receipt and balance change. Configure single requests, no transport-level submission retries, and receipt polling with replacement detection disabled. Replacement detection, full transaction lookup and browser-wallet compatibility are not implemented or verified. A receipt confirms this synthetic chain's inclusion, not RH backing or real-asset value.

## Offline native signing and test funding

Participants can use encrypted `godd wallet sign` through [WALLET.md](WALLET.md). The raw-key command below remains a separate private operator-fixture helper; do not export a personal key to use it.

Use disposable test wallets only. Review freshly queried `god_account` account number and sequence, chain identity, amount, fee and operation before signing. Store the binary 32-byte wallet key in a separate owner-only offline directory, never the node or public RPC workspace. The helper is not an encrypted wallet, mnemonic recovery system or production custody service.

The private `sign-request.json` schema has version, mode, chain ID, account number, sequence, gas, fee, action, validator, recipient, amount, minimum GOD output and deadline fields. Numeric values are canonical decimal strings. Unused operation fields must be empty. Supported actions are `delegate`, `undelegate`, `claim-g`, `transfer-g`, `donate-god` and `redeem-g`. Redeeming requires a positive minimum output and deadline; an empty pool does not erase G. Native fees are paid in full, unlike EVM unused-gas refunds.

```sh
./build/godd testnet sign \
  --request "$GOD_PRIVATE_SIGN_REQUEST" --key "$GOD_OFFLINE_TEST_WALLET_KEY" \
  --output "$GOD_NEW_SIGNED_OUTPUT" --bundle "$GOD_BUNDLE_FILE" \
  --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256"
```

The command signs offline, creates a new private output and reports `submitted: false`. Send its wire separately through `god_submitTransaction` and verify the consensus result. Never treat a signed file or admission response as payment confirmation. Native transactions and EVM transfers consume the same account sequence.

Offline operators can transfer their unbonded synthetic funds through externally signed compatible GOD transfers. The `testnet sign-funding` helper binds a separately reviewed private operator request to the complete synthetic bundle, bounds its amount and fee, and creates a signed output without broadcast. It can seed a dedicated disposable faucet account without putting operator or consensus keys on the faucet service. The local companion's unsigned-request export is not an automatic claim; see [COMPANION.md](COMPANION.md).

Participants use the separately deployed automatic faucet instead: exactly 1 test GOD per address every 24 hours, with bounded IP/global quotas, a finite already-funded pool and durable idempotency. There is no participant account registration or manual approval. Its dedicated synthetic signer is separate from the keyless read-only public RPC. Private Linux transfers, persisted quotas, restart recovery, an actual public HTTPS claim and desktop/mobile status display have passed developer-run checks. Browser transfer signing and the restricted NFT collection use the separate boundaries described in [WALLET.md](WALLET.md) and NFT.md (separate pilot guide, not included in this core snapshot); general contract submission and native browser staking are not enabled by the faucet. The exact-origin downloadable extension is a separate activation, not a faucet permission. Do not release restricted reserve, borrow RH funds, shorten unbonding, or fund testing by changing the fixed GOD supply.

The public portal provides a manual read-only delegation check for the already
matched watch-only account and one canonical `godvaloper1` operator address.
Both addresses and the committed response are checked; stale, mismatched and
unavailable replies never become a zero staked balance or a completed exit.
Network refresh also displays committed GOD-pool and G totals, distinguishing
pending settlement/issuance from available balances. These are service views,
not proofs, validator endorsements, yield promises or redemption quotes.
Browser delegation, unbonding, G claims and redemption remain disabled until
their native signing/admission flows pass separate acceptance. See the full
public-testnet gates in [ROADMAP.md](ROADMAP.md).

## Shutdown and recovery

Stop gracefully, wait for engine workers and database closure, then restart using the same complete node workspace. Consensus signing progress is mandatory; missing or stale progress rejects startup. There is no reset, empty-state recovery, emergency validator replacement, unsafe rollback, snapshot restore or automatic migration command.

Never restore an old validator snapshot as an active signer without a separately reviewed recovery procedure. The local signing floor cannot detect every lost uncommitted signature. Copying keys, clearing databases or resetting signing state can cause double signing even if the software starts. Keep a validator identity on only one active machine.

Monitor `god_liveness` and independently compare committed heights and a stored block identifier across nodes. The bundle-pinned `godd testnet health` helper supplies bounded read-only availability samples. The `host-check` and `smoke` helpers add explicit budget snapshots, reviewed static-resource/origin checks and short consecutive progress checks; [DEPLOYMENT.md](DEPLOYMENT.md) describes them and the private Linux package workflow. None signs, submits, resets or automatically restarts a service. A responding RPC, connected peers or `running: true` alone does not establish progress, quorum or correct state. Some startup synchronization is expected; alert on sustained height stalls or process exits, and investigate before restarting. The endpoint is a diagnostic snapshot, not a proof or a metrics/alerting service.

## Public test acceptance checklist

- [ ] Review and authorize the exact candidate source, native build, configuration and complete bundle digest.
- [ ] Verify Linux startup and recovery on the intended host; cross-compilation alone is insufficient. Windows operation remains disabled.
- [ ] Recruit and verify independently controlled validators, with no single operator holding quorum keys.
- [ ] Exercise cross-host P2P reachability, restart, one-validator outage, prolonged catch-up and loss of quorum without resetting signers.
- [ ] Verify the observer, HTTPS gateway, firewall, exact origins, per-client abuse controls and signed transaction submission from a real disposable-wallet client.
- [ ] Run a sustained test through a real UTC-day settlement boundary, including G claim, transfer, empty-pool rejection and successful voluntary redemption.
- [ ] Measure CPU, memory, disk growth, bandwidth and settlement costs on ordinary computers before publishing minimum requirements.
- [ ] Establish test funding and onboarding, a manual faucet or reviewed rate-limited faucet, health checks, incident contacts and a rollback policy that never resets validator signatures.
- [ ] Keep real assets, RH backing, bridge routes, paid services and mainnet activation disabled unless separately authorized and verified.

Passing short local tests produces a deployment candidate, not completed public acceptance, an independent audit, certified decentralization or mainnet safety.
