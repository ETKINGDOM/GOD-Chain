# GOD Chain synthetic testnet deployment guide

The synthetic GOD Chain testnet runs God EVM + God SDK + GodCometBFT with persistent node storage and a restricted JSON-RPC service. It is intended for public testing with disposable accounts and assets that have no external backing or monetary value. It does not connect to RH, activate a bridge, or authorize further deployment. Mainnet and real-asset commands remain disabled. Public test-GOD claims use the automatic service in FAUCET.md (separate pilot guide, not included in this core snapshot), with no registration or human approval. The separate local unsigned-request interface and offline operator funding helpers are described in [COMPANION.md](COMPANION.md).

GOD Chain's public pilot uses four validator processes and a non-signing observer on one founder-operated machine. The selected initial topology is [single-host and multi-service](SINGLE_HOST.md); additional independent machines/regions are a later expansion, not an installed network. The observer serves restricted public read-only RPC and committed block/account views. The separate automatic faucet does not enable general public transaction submission. The connected [test wallet](WALLET.md) signs plain GOD transfers and restricted collection NFT operations locally, using separate keyless submission gateways. The Chrome extension (separate pilot guide, not included in this core snapshot) is a downloadable desktop test alpha connected through one reviewed exact origin; it is not a Store release or an external website signer.

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
| Restricted NFTs | One configured collection, locally reviewed mint/send, checked receipts, current ownership cards and watch-only retained event pages. | General collection tools, full backfill, independent indexing and sustained storage/load acceptance. Historical events do not prove current ownership. |
| Explorer | Committed blocks/transactions, checksum-matched watch-only accounts, limited scans and independent retained address/NFT snapshot readers. | Complete address/NFT history, complete validator indexing, retention/pruning and proofs. Declared partial coverage is not a full index. |
| Chrome wallet | Downloadable alpha with a fixed reviewed identity, exact-origin service activation, English installation/hash guide and actual HTTPS create/recover/claim/transfer/NFT acceptance. | Physical Chrome/device acceptance, transient read recovery, website connection permissions, external provider signing, independent security review and a separately authorized Store release. |
| Staking and G | Connected web/Chrome six-action signing, bounded committed unbonding progress, locked one-operator lookup and eight-record current registration pages; public delegation and start-unbonding receipts with a real pending entry. | Full delayed payout, positive settled-G claim/transfer/redemption and historical validator indexing over sustained consensus. Registration is not the consensus signing set, uptime, ownership or location proof, and delegation is not web mining. |
| Operations | Supervised services, bounded health snapshots, abuse limits and safe frontend rollback. | Sustained load/resource measurements, alerting, storage-growth policy and independently reviewed recovery drills. |
| Single-host validators | Four pilot validator processes with distinct identities under founder control. | Intended-host process/resource isolation, quorum/reboot/durability and signing-safe recovery; independent hosts and cross-host admission are later expansion work. |

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

For the selected single-host layout, retain four validator processes with distinct identities and equal initial voting power, plus a separate observer process for public RPC on the same machine. Keep private directories and ports separate without weakening signing/quorum rules. Three of four equal validators can commit; two cannot. Failure of the host stops all of them. Founder-operated startup is not independent validation; distributed deployment and remote admission require later separate acceptance. See [SINGLE_HOST.md](SINGLE_HOST.md) for shared resource, recovery and release boundaries.

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

A wallet user sends transactions but does not produce blocks. An observer
follows the chain without signing consensus. A validator runs the node, keeps
its own consensus-signing identity and participates in proposals/votes while
online. Independent operation means different participants control their own
machines and signing keys; several processes, profiles, IPs or location labels
do not establish that control. Ordinary-computer eligibility still needs real
resource/connectivity/uptime measurements. Never ask participants to send their
private keys to a coordinator or use a shared developer-created signer.

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

Preview the selected bundle before joining, using the independently reviewed
full digest, without providing a node directory or any key:

```sh
./build/godd testnet inspect-bundle \
  --bundle "$GOD_PRIVATE_BUNDLE" \
  --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256"
```

The bounded read-only report shows the native/compatible chain identities,
bundle/genesis checksums, validator/observer counts, aggregate non-loopback/IP
counts, transaction/block limits, fee floor, prototype G budget/daily ceiling
and fixed GOD supply. Signing-sized integers use exact decimal strings. It
omits participant addresses, endpoints, node public/private keys and local paths.
The reader validates the complete synthetic bundle and profile proofs, needs
only that private bundle file, and opens no operator workspace. It does not
write, join, contact a peer, start a node or select production economics.

Non-loopback/IP counts describe declared endpoints, not independently controlled
machines, reachable routers or geographic regions. A matching checksum is not
trusted provenance without the separate channel. Review ownership, connectivity,
funding, fees and control independently. Keep populated reports/bundles out of
source publication. This is a fresh synthetic ceremony, not a command to alter
the deployed pilot's validator set or activate a backed mainnet.

```sh
./build/godd testnet join \
  --home "$GOD_NODE_DIR" --bundle "$GOD_BUNDLE_FILE" \
  --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256"
./build/godd testnet check --home "$GOD_NODE_DIR"
```

Joining works only for a fresh initialized identity with empty signing progress and no data directory. It retains the operator's keys, requires an exact signed-profile match, and creates genesis and configuration without replacement. A second join fails. A partial or rejected setup requires inspection, not automatic deletion or signer reset. Run the offline `check` while the node is stopped so that its atomic signing-state replacement cannot race file inspection.

## Self-service replication after launch

The intended rollout may begin with founder-operated validators, then allow
participants to run their own nodes without sending private keys or obtaining
individual approval. Founder-controlled validators are not independent
operators. Active validation and rewards remain separate from replication.

### Explicit verified configuration download

An operator may download one selected synthetic configuration before any node
setup. Obtain the complete lowercase SHA-256 digest through an independently
authenticated, reviewed channel. A checksum served beside the configuration is
not enough. Review the publisher and the intended network separately; this tool
does not supply an official endpoint, discover peers or establish trust for you.
Create an owner-only private download directory with no existing bundle file:

```sh
./build/godd testnet fetch-bundle \
  --source "$GOD_REVIEWED_BUNDLE_URL" \
  --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256" \
  --output "$GOD_PRIVATE_DOWNLOAD_DIR/testnet-bundle.json"
./build/godd testnet inspect-bundle \
  --bundle "$GOD_PRIVATE_DOWNLOAD_DIR/testnet-bundle.json" \
  --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256"
```

Use an explicit HTTPS URL ending in `testnet-bundle.json`; numeric loopback HTTP
is permitted only for local fixtures. Credentials, query strings, fragments,
ambiguous paths, redirects, cookies, compressed bodies and environment proxies
are refused. Standard TLS validation stays enabled. The one request has an
eight-second total deadline and a two-MiB content bound. Full digest, strict
format, runtime/genesis binding, economic policy and profile proofs are checked
before exclusive creation of a mode-0600 file. Shared or linked storage parents
and existing destinations are refused. An interrupted disk write may leave an
unusable private partial file; inspect it manually instead of overwriting it.

The report is redacted and never implies node initialization, startup, signing,
rewards or public acceptance. Download and offline review are preparation steps;
`init` or `join-observer`, followed by explicit startup, remain separate. Private
loopback tests cover download, fresh observer initialization, committed-history
replay and restart without votes. This is not accepted public configuration
delivery, publisher-signature distribution or cross-machine testing. Keep
populated configurations and their trusted pins out of the public source tree.

### Retained identity and explicit join

`join-observer` lets a fresh locally generated observer use the existing
checksum-pinned synthetic launch bundle without adding its profile to that
bundle. Create an observer identity as above, independently review the complete
bundle digest, then run:

```sh
./build/godd testnet join-observer \
  --home "$GOD_OBSERVER_DIR" --bundle "$GOD_BUNDLE_FILE" \
  --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256"
./build/godd testnet check --home "$GOD_OBSERVER_DIR"
```

Setup is offline: it retains local keys and empty signing progress, copies the
exact existing genesis and derives at most 32 explicit peers from the reviewed
bundle. It does not contact a coordinator or rewrite genesis, validator
membership, funding, G issuance or existing node workspaces. The ordinary
`join` command still requires an exact launch-profile match. Existing node data,
signing progress or configuration cannot be overwritten or reset. Launch-key
reuse across consensus/P2P roles and non-loopback observer RPC are refused.
Leave node directories private, and inspect partial setup failures rather than
deleting them automatically.

Startup is separate and remains opt-in. With existing peers reachable, the
observer replays retained blocks and follows new ones but cannot vote or propose.
Private persistent loopback acceptance covers joining after a committed GOD
transfer, matching its balance/block hash, further progress and restart with
unchanged local identity and empty signing records. This is not cross-host
acceptance, a public seed/discovery service, state-sync checkpoint authentication,
an unlimited archive or an ordinary-computer performance guarantee. The reviewed
peer/firewall layout must permit outbound connections; it does not become open
merely because this offline setup exists. Duplicate-IP restrictions still apply
outside the explicit loopback test layout, including participants behind NAT.

Running an observer requires no stake and grants no G reward. A synchronized
observer cannot be promoted by editing its role or resetting signing state.
The distinct candidate workflow below is opt-in synthetic local work; it has
not replaced a deployed node or widened any public submission gateway.

## Fresh participant and validator-candidate workflow

Candidate admission requires a NEW synthetic bundle assembled explicitly with
`testnet assemble --require-validator-proof`. Preview it with `inspect-bundle`
and require `requireValidatorProof: true` before setup. This immutable opt-in
binds all operators to the same on-chain consensus-key possession rule; it is
not an in-place migration. Legacy bundles retain their behavior and allow
permanently non-signing observers, but the candidate tools refuse them before
creating new keys. Never edit a live configuration or reset data/signing progress
to enable this rule.

`testnet init` combines fresh local key creation and pinned joining. Choose
`observer` for permanently non-signing replication, or `candidate` with a
separately held disposable wallet's public owner address. It validates the
complete bundle and options before creating a workspace, never overwrites an
existing one and leaves partial write failures for inspection. RPC must remain
loopback; this command neither contacts peers nor starts a node:

```sh
./build/godd testnet init --home "$GOD_CANDIDATE_DIR" --role candidate \
  --owner "$GOD_CANDIDATE_OWNER" --endpoint "$GOD_CANDIDATE_ENDPOINT" \
  --p2p "$GOD_CANDIDATE_P2P_LISTEN" --rpc "$GOD_CANDIDATE_LOOPBACK_RPC" \
  --hosts "$GOD_CANDIDATE_RPC_HOSTS" --bundle "$GOD_BUNDLE_FILE" \
  --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256"
./build/godd testnet check --home "$GOD_CANDIDATE_DIR"
```

An existing fresh `identity --role validator` can instead use `join-candidate`;
it must not be a launch validator and must retain empty signing/data state.
`candidate` means pending eligibility, not active validation. Its profile proves
local node-key possession and pins the intended canonical owner. The wallet
key must stay outside node/web workspaces. After separately approved startup,
the candidate replays the original chain with its own keys. It signs only when
locally committed state binds this consensus key to this operator **and** places
it in the actual signing set for the exact requested height. Remote RPC results
and a local role flag cannot grant authority. The two-block validator-update
delay is preserved; current bonded status alone is not the signing set.

Acquire sufficient existing synthetic GOD without minting or unlocking the
reserve. This prototype requires 1,000 GOD self-stake, fixed 10 percent commission
and a maximum of 32 active validators, selected by the existing staking rules.
These are test parameters, not newly approved production economics. Genesis
ceremonies give each founding validator only 100 liquid GOD after staking;
a faucet's one-GOD claim is not enough to register a validator. Initial stake
funding/circulating supply must be reviewed separately, not bypassed by this tool.

Review fresh account number, sequence, stake, gas and fee from a trusted node.
Use the offline `prepare-registration` command while the candidate is stopped
for private-file inspection. It creates a new request with a domain-separated
node-key proof over the pinned chain/genesis, owner and exact fields, without
altering consensus signing progress. Transfer only that request to the separate
wallet workspace, then sign with the encrypted terminal wallet:

```sh
./build/godd testnet prepare-registration --home "$GOD_CANDIDATE_DIR" \
  --bundle "$GOD_BUNDLE_FILE" --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256" \
  --output "$GOD_REGISTRATION_REQUEST" --account-number "$GOD_ACCOUNT_NUMBER" \
  --sequence "$GOD_FRESH_SEQUENCE" --gas "$GOD_REGISTRATION_GAS" \
  --fee "$GOD_REGISTRATION_FEE_AGOD" --stake "$GOD_SELF_STAKE_AGOD"
./build/godd wallet sign-registration --wallet "$GOD_OPERATOR_ENCRYPTED_WALLET" \
  --bundle "$GOD_BUNDLE_FILE" --expected-bundle "$GOD_REVIEWED_BUNDLE_SHA256" \
  --request "$GOD_REGISTRATION_REQUEST" --output "$GOD_SIGNED_REGISTRATION"
```

The private terminal displays the owner/operator, consensus public key, network,
genesis, stake, minimum self-delegation, commission and exact fee/metadata before
requiring `SIGN`. Passwords and account keys are never arguments, files passed
as passwords or server inputs. The separate `testnet sign-registration` binary-
key utility remains for disposable offline test fixtures. Both signers verify
the local node proof and exact wallet owner, cap the synthetic fee at 0.001 GOD,
write without replacement and **never submit**. Signing does not check current
spendability, guarantee active selection or establish inclusion.

Submit only through a separately reviewed private synthetic transaction path.
Match the known native transaction hash to successful committed execution;
an unknown outcome requires hash recovery, not another registration signature.
The public read-only RPC and six-action browser/native gateway remain unchanged
and do not accept validator registration. Registration creates a standard
staking message with a compact, versioned Ed25519 possession proof in its
bounded description details. The proof binds the immutable runtime, operator,
consensus public key, stake, minimum, commission and remaining description.
Both admission checks and the staking message handler verify it before state
writes on a proof-enabled network. The private full-request node proof also
binds genesis and account/fee metadata; the separate wallet signature authorizes
the actual transaction. A public consensus key alone cannot satisfy this rule.
Legacy networks have not been upgraded; public deployment and cross-host abuse
acceptance still require review. Never treat this synthetic check as an audit.

After approved restart, a matching registered candidate automatically begins
signing when selected; no individual coordinator approval or genesis rewrite
is required by this path. Original FilePV anti-double-sign progress and the
owner/storage binding survive restart. Loss of signing-set eligibility stops
signing, while replication continues. G comes only from verified contributions
under unchanged reward rules. Starting unbonding is not withdrawal; the existing
21-day delay remains. Never copy a running validator's key/state into a second
instance or reconstruct missing signing progress.

In the private verification workspace, `make test-self-service-candidate`
covers initialization, proof/wallet
signing and persistent local consensus admission, contribution, restart,
changed-owner rejection and exit. Installers, authenticated configuration
delivery, secured public peering, intended-host runtime/abuse/resource checks,
production funding and complete production acceptance remain unfinished.
Private tests and their Makefile targets are excluded from public source snapshots.
Mainnet/RH/real-asset commands remain disabled.

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
| `eth_getBalance`, `eth_getTransactionCount`, `eth_getStorageAt` | Latest committed application state. An exact current height is accepted; older state and arbitrary pending reads are not. Pending account sequence uses the ordered admission cache. |
| `eth_getCode` | Latest or exact retained committed code only in the local candidate. Missing/pruned/future state is unavailable, never replaced with latest code. This narrow history-acquisition support does not enable historical account/call queries or archive/state proofs, and is not installed in existing pilot binaries. |
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

Monitor `god_liveness` and independently compare committed heights and a stored block identifier across nodes. The bundle-pinned `godd testnet health` helper supplies bounded read-only availability samples. The finite `watch-health` helper adds actual elapsed-time samples, progress checks and redacted first-fault JSON lines without automatic retries or external notifications. The `host-check` and `smoke` helpers add explicit budget snapshots, reviewed static-resource/origin checks and short consecutive progress checks; [DEPLOYMENT.md](DEPLOYMENT.md) describes them and the private Linux package workflow. None signs, submits, resets or automatically restarts a service. A responding RPC, connected peers or `running: true` alone does not establish progress, quorum or correct state. Some startup synchronization is expected; alert on sustained height stalls or process exits, and investigate before restarting. The endpoint is a diagnostic snapshot, not a proof or an independently operated metrics/alerting service.

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
