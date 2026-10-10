# GOD Chain

GOD Chain is the independent blockchain project for ETERNAL KINGDOM, a global faith-centered world for prayer, confession, praise and fellowship. This repository contains the [Whitepaper Draft](WHITEPAPER.md) and source candidates for a synthetic-asset prototype. God EVM execution, God SDK staking and GodCometBFT consensus have been connected in local tests. Mainnet, RH bridging and real assets remain disabled.

God EVM + God SDK + GodCometBFT are the project component names. Necessary upstream import paths, licenses and attribution remain intact; these names do not claim independent invention of imported infrastructure. All project-owned public content is in English.

## Public testnet status

GOD Chain's public testnet operates with four validator nodes. Deployment across four geographic regions is planned. The current pilot is not an independently operated, geographically distributed network.

A non-signing observer provides restricted public read-only RPC and committed block/account views. A separate automatic faucet distributes existing synthetic GOD without registration or human approval, subject to fixed amounts, rate limits and available pilot funds. Separately deployed web and downloadable Chrome test wallets support locally reviewed signing, plain GOD transfers, restricted NFT mint/send and six bounded native staking/G actions. General public RPC submission, arbitrary contract signing and external website-provider signing remain disabled. Test assets have no monetary value, external backing or guaranteed mainnet conversion; the network may be reset.

Web/Chrome alpha 0.3.9 includes the existing native status/submission cooldown separation, exact definite-non-admission distinction and no-replacement guards. Public disposable-account delegation and start-unbonding produced matching committed results and a real pending entry; locked web/Chrome recovery passed. Its read-only G view distinguishes settled/pending balances and pools, derives a UTC boundary from committed time and displays only a settled-liquidity indicative quote. Live desktop/mobile/Chrome values matched exact committed snapshots. A boundary or quote promises no reward or payment. The 21-day delay is unchanged, and initiation is not payout. Earlier unknown IDs remain unresolved rather than automatically resent.

The locked-wallet registration directory reads at most eight records per page
with raw operator-key ordering, exclusive cursors and exact commit pins. Changed
tips or unavailable reads require a new first page; different commits are never
merged. Live desktop/mobile/downloaded Chrome views verified all four current
records without signing or submission. Registration is not consensus signing,
uptime, independent ownership or geographic evidence. Historical indexing and
the production transition gates in [ROADMAP.md](ROADMAP.md) remain incomplete.

This source snapshot does not include all separately deployed pilot services. GitHub publication and public build commands do not deploy or activate a chain. Operational endpoints, populated addresses and private configuration are deliberately excluded.

The local-only Chrome 0.4.0 candidate adds explicit, revocable public-address
sharing and same-profile recovery guards for GOD transfers, restricted NFT
mint/send and six staking/G actions. It saves the original public review and
verified TX ID before one submission; uncertain outcomes block replacement
signatures across reload and browser-process restart. Website signing stays
disabled. The existing downloadable alpha is unchanged. Candidate browser
source, fixtures and distribution files remain outside this exact core scope;
the private developer checks and remaining gates are summarized in
[VERIFICATION.md](VERIFICATION.md#local-only-wallet-recovery-candidate).

See the [Implementation Status and Roadmap](ROADMAP.md) for verified capabilities, six remaining core workstreams, ten supporting modules and the proposed delivery sequence. The plan distinguishes prototype and pilot checks from production acceptance; it does not authorize further deployment or real assets.

## Execution and validators

`internal/godnode` assembles accounts, bank storage, staking, execution and the local consensus adapter. It accepts bounded legacy, access-list and dynamic-fee Ethereum transactions, including contract deployment and storage. Native direct-signature transactions share account sequences with the Ethereum path. Explicit synthetic-testnet commands provide a restricted JSON-RPC adapter and an offline terminal wallet; no public service is activated by a build or upload.

Signed validator creation requires 1,000 GOD self-stake and a fixed 10 percent commission. Delegation requires 1 GOD; unbonding takes 21 days. The prototype selects up to 32 active validators and bounds delegators per validator to 256 for local resource control, not as a finalized worldwide limit. G allocation uses the preceding verified commit and stake captured before that block's transactions. Missed signatures receive no allocation. The per-block budget must be explicitly configured; no mainnet emission rule is selected.

New synthetic bundles can opt into consensus-key possession checks with
`testnet assemble --require-validator-proof`. A bounded proof inside the
standard staking message binds the immutable runtime, intended operator, key
and registration terms. The chain verifies it before authenticated admission
and staking writes. Candidate tools require this policy; legacy bundles retain
their existing behavior and non-signing observer path. This is not an in-place
upgrade, bridge-enabled runtime approval, audit or public activation. Never
reset a signer/database or edit a live runtime to enable it.

The balance adapter transfers existing GOD instead of granting mint or burn permission. Fees remaining after Ethereum gas refunds enter pending rewards. Native signed fees retain their full-requested-fee rule. Value otherwise discarded by self-destruction-to-self is quarantined, an intentional native-value difference. Default inflation and burning penalties are not imported as economic rules.

Consensus-verified duplicate votes apply a 5 percent historical-stake penalty into protected quarantine, never burning GOD or funding G redemption. The consensus key and operator are permanently excluded. More than 1,000 misses in a full 10,000-opportunity signing window suspends a validator for at least one hour without reducing principal. Reactivation requires the authenticated operator and adequate self-stake. Other evidence classes and production recovery governance remain release gates. See [NODE_RUNTIME.md](NODE_RUNTIME.md).

Opt-in disk storage preserves application state, consensus databases, WAL and original file-signing progress. Private fixtures cover normal restart, abrupt process termination and four-validator recovery. The checksum-bound lifecycle build joins admitted peer workers before closing stores and passed the observed shutdown regression and controlled worker tests. This is not operator-grade reliability under every failure. Ordinary nil-policy nodes keep binding version 2; explicit synthetic bridge initialization uses version 3, or version 4 with authenticated routes enabled. Incompatible local data requires reviewed migration, not a reset or replacement signer.

Transaction checking defers during the interval between block finalization and commitment, before ante processing or store writes. `LocalNode.Submit` returns `ErrCommitPending` only for that distinct status and makes no internal retry. This avoids checking a newer working ledger against an older checking timestamp without weakening authentication or time rules. The check never waits under a mempool lock needed by commitment.

## Local committed queries

`QueryNetwork`, `QueryAccount` and `QueryDelegation` provide detached latest-committed network metadata, GOD balances, auth account numbers/sequences, three G buckets and individual staking views. Native and EVM address encodings query the same account. CheckTx and finalized but uncommitted changes are excluded, including across daily settlement. Queries create no account, settle no reward and change no fee or sequence. Responses mark synthetic operation, keep real assets disabled and use exact decimal strings for amounts and signing-related integers.

`QueryValidator` adds a bounded one-operator view with committed stake/shares,
fixed commission/minimum self-delegation, staking status and stored suspension/
permanent exclusion. It scans no collections and omits arbitrary descriptions.
Local Go, trusted ABCI and opt-in RPC interfaces are included. Bonded status is
not actual signing, uptime, ownership or geography proof. The separately
deployed web/Chrome alpha 0.3.8 checks one configured pilot operator through a
keyless query-only gateway, including while locked. Selection only fills an
unsigned delegation form; it never signs or clears an unknown transaction.
The configured list is not a complete validator index. General public RPC is
unchanged, and publication itself activates no route.

The account G fields exclude application locks and must not be labeled total G. GOD bank balance is not a spendability estimate; an application hash is not a block hash or proof. Trusted ABCI routes remain internal. A separately configured synthetic RPC adapter exposes bounded committed reads and externally signed submission, without a server-held wallet or authenticated light-client proofs. See [NODE_RUNTIME.md](NODE_RUNTIME.md#committed-state-queries).

## Synthetic testnet and participant tools

`godd testnet create`, `identity`, `assemble`, `join`, `check` and `start` provide private synthetic setup and persistent operation. Independent initialization keeps consensus and P2P keys with each operator. Earlier private checks exercised four validators and a non-signing observer; they did not establish independent cross-host operation. The public synthetic pilot runs under separate deployment authorization. Validators keep RPC on loopback and the observer is exposed only through a restricted gateway. Mainnet, RH bridge startup and real assets remain disabled. See [TESTNET.md](TESTNET.md).

The `join-observer` candidate also permits a fresh non-signing local identity
without advance inclusion in the exact reviewed synthetic launch bundle. It
preserves original keys/genesis, uses bounded pinned peers and requires loopback
RPC and empty node/signing state. Private persistent tests cover joining after
a committed transfer, catch-up and restart without votes. This does not change
the validator set, earn G or open public peering. A fresh `testnet init` now also
supports a distinct validator-candidate role. Candidate registration is prepared
offline with a node-key proof and signed by the exact separately held wallet;
`wallet sign-registration` offers encrypted interactive review without plaintext
key export. The guarded FilePV signs only when locally committed ownership and
the exact-height signing set match, preserving the two-block delay and durable
progress. Private persistent checks cover post-genesis catch-up, authenticated
registration, actual commit contributions/G, restart, changed-owner refusal and
non-signing replication after exit. This candidate path is not publicly activated
and does not expand the six-action gateway. Installers, configuration delivery,
target-host/abuse acceptance, stake funding and chain-wide consensus-key collision
policy remain separate work. Founder-operated bootstrap is not decentralized
control. Publication is not a new public node or mainnet activation.

The basic Ethereum RPC subset supports external signing, bounded call/gas simulation, raw submission and actual success or revert receipts. Native direct-signature submission shares the same account sequence. Admission is not commitment. Only the explicit commit-pending response permits a post-commit retry; an unknown submission outcome requires checking the known hash before retrying. There is no full historical API, subscription service or unrestricted debug RPC.

`godd companion --listen <explicit-loopback>` serves eight embedded English resources for read-only account, staking, block and receipt views, unsigned requests and local feedback downloads. It starts no chain, handles no wallet password and contains no browser signer, automatic broadcaster, analytics or automatic faucet. Manual test funding moves existing synthetic GOD rather than minting. See [COMPANION.md](COMPANION.md).

The [encrypted terminal test wallet](WALLET.md) creates disposable, bundle-bound accounts and signs GOD transfers or six native operations only after explicit terminal approval. Its signing stays outside the browser and RPC server. The separately maintained web/Chrome alpha signs on the participant's device and never sends a vault or password to the node. Browser/extension source and other pilot gateways remain excluded; the restricted native gateway candidate is included below. Personal-key import, mnemonic recovery, website-provider signing and production-wallet acceptance remain incomplete. The [test explorer](EXPLORER.md) shows bounded committed blocks and actual transaction outcomes, not a complete archive. Test GOD and G have no monetary value or guaranteed mainnet conversion.

The included [keyless history CLI](HISTORY.md), `godhistory`, indexes selected
complete blocks from an explicitly configured numeric-loopback observer. It
atomically stores compact public transaction identifiers, execution status,
supported address membership and a durable checkpoint. Queries return twenty
matches with an exclusive stored-anchor cursor and explicit retained coverage.
Memos, calldata, raw wire, events, logs and faith text are not stored. It opens
no listener, signs nothing and leaves unknown participant submissions untouched.
Private pagination, interruption/reopen and persistent-node checks passed;
bounded Linux acceptance read ten committed pilot blocks without changing any
existing service. Its included offline `audit` checks retained logical storage
without source reads, repair or query activation. The separately deployed
history API/browser integration is excluded from this source snapshot. Neither
is full-genesis backfill, NFT event history, authenticated proof or production-
storage acceptance.

The [restricted native gateway candidate](NATIVE_GATEWAY.md), `godnative`,
includes six-action signature/envelope admission, coherent bounded preparation
and status reads, and synchronous attempt guards before one upstream send.
Unknown outcomes are never automatically resent or replaced. A bounded disk
inventory survives reopen independently of its 1,000-entry RAM cache. Explicit
offline audit/copy can preserve all hashes at an equal/larger reviewed capacity,
up to 100,000 records, without changing the source or activating the copy.
The offline CLI checks page/freelist invariants in a supervised process and
verifies a selected exact image against a separately retained reviewed checksum.
This can reject a stale valid backup against a current approved pin; an unpinned
audit cannot establish freshness or reconstruct missing unknown records.
The deployed pilot still uses its earlier volatile reservation map; this source
publication is not its migration. Complete legacy inventory, controlled drain,
intended-host cutover and physical-durability acceptance remain required.

`godpack` prepares and verifies private Linux acceptance archives without installation or execution. The included nginx file is an unpopulated review template, not an enabled service or operational configuration. `godd testnet host-check`, `health` and `smoke` provide read-only host budgets, pinned availability checks and matching-resource/origin/progress samples. They never sign, submit or restart a service. The pilot has short native Linux and HTTPS checks; independent hosts, sustained daily settlement, failure recovery and ordinary-computer resource measurements remain acceptance gates. See [DEPLOYMENT.md](DEPLOYMENT.md). No binaries, node data, keys or configured endpoints are distributed here.

## G rewards and fixed GOD supply

G has no fixed lifetime cap. The prototype bounds issuance to 10,000 G per UTC day. Earned G and GOD fees settle at the next UTC-day boundary; skipped days create no catch-up issuance. Settled unclaimed and locked G count in outstanding supply exactly once. Participants cannot submit issuance allocations.

Voluntary redemption pays `floor(P * g / T)` from the settled GOD pool, with at least 1 G, positive minimum output and a consensus-time deadline. Only a successful GOD payment burns the offered G. Empty liquidity, stale quotes and failed payments retain G, which can also be transferred or locked. An accepted transaction can still pay its normal fee on business failure.

GOD bank supply remains 1,000,000,000 with 18 decimal places. Ordinary synthetic genesis assigns local balances and holds the remainder in restricted reserve. Explicit bridge genesis instead starts with the full reserve and releases authorized synthetic deposits once before staking. Equal supply or quorum statements do not prove RH backing. Protected protocol funds cannot be treated as participant balances or redemption liquidity.

## Addresses and authentication

Native accounts use canonical lowercase Bech32 with prefix `god`, shown schematically as `god1…`. EVM compatibility uses a complete EIP-55 address with a `0x` prefix. Both encode the same 20 bytes; conversion creates no wallet and moves no assets. No populated address or operational configuration is supplied.

The native protobuf path supports claim, transfer, lock, unlock, redeem and donate, plus the node's restricted staking messages and six explicitly enabled bridge messages. It checks the single owner, chain ID, account number, ordered sequence, fee denomination and explicit gas bounds. Ante rejection commits neither fee nor sequence; accepted business failure retains both while reverting message writes. Keeper methods are internal protocol APIs, not unsigned public services.

## Bridge scope

`x/godbridge` and `contracts/GodBridgeEscrow.sol` implement isolated one-for-one ledger and custody state machines with five-of-seven approvals, replay protection, segregated withdrawals, FIFO order, a 24-hour delay, rolling limits and permanent cancellation records. Timeout alone cannot authorize a refund. Quorum attestations are a custody trust model, not cryptographic source-finality proofs.

An opt-in execution codec connects six authenticated bridge message adapters to the local node. Positive bounded `Config.BridgeApprovalGas` requires explicit `BridgeGenesis` and charges each supplied approval; zero leaves ordinary and ledger-only participant routes disabled. The optional node certificate checks complete deposit funding, stake/fee budgets, owner consent, consensus-key possession and actual node/consensus policy, including the exact route gas setting. It initializes the synthetic ledger from full reserve and applies staking in one cache; failed initialization rolls back financial and replay state. Initial deposits and bootstrap cannot release GOD twice. Aggregate accounting and the immutable record are checked through commits and reopen.

Four persistent loopback validators processed signed deposits, withdrawal locks, finalized-cancellation acknowledgements and pauses with matching state. Private checks independently verified transaction/result commitments, following-header linkage and actual commit signatures, then exercised one-validator catch-up and full restart with original databases and signers. Early authorization and queued payment were rejected. Successful delayed authorization and payment were checked in direct synthetic-node and isolated-custody fixtures, not after a real 24-hour wait on the four-node network.

Read-only authorization, resolution and pause digest helpers build unsigned review proposals from explicit configuration. They do not read ledger state, establish eligibility, grant approval or prove source execution. Independent source-finality verification, independent signers, production-backed node genesis and a financial relayer remain absent. The 10,000 GOD exposure limit cannot fund ten minimum-self-stake validators plus positive fees; a 32-validator launch is not established. Synthetic node and funding tools do not deploy or fund this bridge. See [BRIDGE.md](BRIDGE.md).

## Private RH connection and receipt checks

`internal/godrh` provides replaceable, read-only source interfaces and a private configuration loader. Operational fields remain blank until configured privately: source/native names, source network ID, RPC endpoint, token/custody contracts, reviewed runtime-code pins and seven distinct public signer identities. No address, endpoint or credential value is supplied. The loader rejects ambiguous fields, signing-key fields, symlinks, shared permissions and oversized input. Populated files stay ignored and owner-readable; Windows ACL behavior still needs native verification.

`godd rh-template` emits a blank simulation-mode template. `godd check-rh <private-file>` checks configuration offline; `godd probe-rh <private-file>` explicitly performs bounded source reads. Code and contract view calls are pinned to one provider-reported finalized block hash with canonical-block requirements. Checks cover network identity, code pins, 18 decimals, fixed supply, custody asset/domain/signers and bridge limits, then recheck the original block. Reports, formatting and errors redact operational values. These RH diagnostic commands never deploy, sign, fund a bridge or initialize a node.

`ObserveDeposit` checks a successful source receipt, exact transaction and block-wide log index, the configured custody event, expected recipient/amount, reviewed code at the receipt block and stable canonical block/checkpoint lookups. Its detached result is an unsigned deposit proposal using the existing protocol digest. It has no ledger, signer or broadcast access. A provider can fabricate mutually consistent receipts and finalized tags; independent source finality, approval and real-asset readiness remain false even in production configuration mode. Supplying only a token CA cannot activate a bridge. These interfaces are exercised with synthetic sources and compiled private custody events, not actual RH backing.

`ObserveResolution` checks paid or cancelled receipts against the complete expected native withdrawal. Paid events must match sequence, ID, recipient and amount; cancellations must have no data payload. Both require the custody terminal view at the receipt block to agree. Deposit and resolution use the same bounded `ReceiptSource` checks, with `DepositSource` retained as an alias. Proposals remain detached and unsigned; matching provider responses neither prove native committed state nor acknowledge payment or refund GOD.

`RelayJournal` is a simulation-only persistent queue for those read-only checks. It binds the private configuration, deduplicates immutable requests and retains bounded retry attempts through reopen. Any uncertain save stops the instance. Its dedicated owner-only database uses an exclusive lock; Windows disk use fails closed until private ACL handling is implemented. Cached observations remain historical unsigned data, not approvals. There is no automatic service, signing, broadcast or native ledger write.

`EnableDiscovery` binds an explicit first block. `ScanNext` reads hash-pinned event hints in batches of at most eight blocks and atomically saves candidate tasks with progress. The HTTP adapter permits only six read methods and uses exact custody, block-hash and event-topic log filters. Resolution hints require an exact withdrawal lookup, which is not a native-state proof. Malformed data, conflicts, capacity, reorgs and changed identity reject the whole batch. Provider omissions and fabricated empty blocks cannot be detected by hints alone; `ObserveNext` must separately check receipts.

`ScanReceiptSets` optionally extracts events from complete canonical binary transactions and receipts after matching both tries against the requested header hash. Transaction hashes and global log indexes are derived locally. New tasks retain immutable event bindings; subsequent receipt reads must match them and recheck retained scan references. `receiptSetsMatched` counts only that successfully saved batch, not earlier cursor history or independent finality. Manual/hint tasks are never promoted. Unsupported formats fail closed; historical completeness, approval and real-asset flags stay false.

## Private receipt proofs and material reads

`VerifyReceiptInclusion` checks bounded transaction and receipt paths against a caller-pinned binary header. `PrepareReceiptInclusion` constructs them from a complete set; `RelayJournal.PrepareTaskReceiptInclusion` also matches the original task, global position, event ABI and any unsigned cache. The selected paths alone cannot certify counts in preceding receipts or a transferable global log index. These APIs do not authenticate source finality or authorize transfers.

`TaskReceiptProofStore` separately retains one immutable private slot containing complete material and its witness. Reopen and `Read` regenerate and compare the proof; uncertain saves stop the instance until verified reopen. Local retention is neither authenticated evidence, encryption nor anti-rollback protection. Windows private-disk use stays disabled. Raw operational bytes must never be published or logged.

`NewHTTPReceiptSetSource` explicitly opts into bounded `debug_getRawBlock` and `debug_getRawReceipts` calls on the privately configured endpoint. The default source and diagnostic probe do not enable them. It checks canonical binary encodings and complete roots with before/after height/hash lookups, without enabling a server debug namespace or falling back to hints. Actual RH availability, encodings and fork compatibility remain unverified.

`RelayJournal.FetchTaskReceiptProof` reads material selected by the original task, checks provider references before and after, and returns detached proof/material bytes. `TaskReceiptProofStore.RecheckSource` first revalidates the loaded private slot, then compares every material and witness byte with that fresh read. Neither changes the journal, consumes attempts, creates a cache or automatically saves a proof. Failed source checks do not overwrite or disable sound local storage. These calls do not query code or terminal/native state and do not replace `ObserveNext`. Provider agreement during one call is not independent finality or a future freshness guarantee. See [BRIDGE.md](BRIDGE.md) for bounds and trust limits.

`RelayJournal.ReviewTaskReceiptEvidence` combines complete-material/task/proof checks with an unsigned deposit or paid/cancelled proposal. It derives the receipt and block-wide log indexes from that same detached complete set rather than asking for a separate provider receipt or log hint. Explicit compatibility, code and terminal-view reads compare provider claims; they are not contract-state proofs. Retained references and every later observed checkpoint must remain consistent. The simulation-only call changes no store, cache, cursor, clock or attempt budget. Detached results use redacted reports; source finality, native-state truth, approval, signing, broadcast and real-asset readiness remain unverified or disabled.

## Build and inspect

Versions and tool provenance are recorded in [UPSTREAM.lock.json](UPSTREAM.lock.json); module checksums are in `go.sum`. Local verification used Go 1.26.8 with the recorded JSON-library compatibility override.

```sh
go mod download
make check
make status
```

Make targets use `cmd/godbuild` to verify the exact pinned reactor, prepare an ignored private module copy and select the reviewed compiler overlay. Original dependency caches and pins are unchanged. A plain unpatched node build fails the worker-join API check; do not bypass it. Required upstream patch license and notice are retained under `licenses/`.

`make build-native-gateway` and `make build-native-gateway-linux` separately
compile the keyless candidate without initializing a store or opening a listener.
The optional `cmd/godbuild -native-gateway` retains the lifecycle overlay and
checksum-binds two logging files to standard-library JSON, excluding runtime
Sonic/base64x JIT dependencies. Changed pins, altered/extra mirror files and
unsafe outputs fail closed. This does not alter other node builds or relax
service hardening. Public `make check` also verifies this isolated build graph.

The diagnostic reports `local-consensus-execution-prototype`, `localNodePrototype: true` and `localCommittedQueriesImplemented: true`, while retaining `nodeReady: false` and `realAssets: false`. Mainnet and real-asset startup commands fail closed; the explicit synthetic-testnet path is separate. A build does not start a chain or verify backing. `make build` creates `godd` and `godpack`; `make build-history-index` separately builds the opt-in history CLI without syncing or opening a listener. `make compile-targets` cross-compiles five node targets and two Linux packager targets without executing foreign binaries. Go 1.25 or newer is required; verification used Go 1.26.8. Windows private operations remain disabled pending ACL acceptance.

Generated protobuf messages are included, so ordinary builds need no protobuf compiler. `make generate-proto` uses protoc 33.0 and the locked generator. Solidity custody source requires the separately verified compiler described in the lock; no compiler, private fixture, bytecode or operational build script is distributed.

Private test suites, fixtures, runtime data, keys, addresses, binaries and logs are intentionally excluded. Public build commands do not reproduce the private verification suite. See [VERIFICATION.md](VERIFICATION.md) for observed checks and their limits.

## Remaining release gates

Operator-grade shutdown, physical-durability testing, signer lifecycle, adversarial peers and evidence, and governance remain incomplete. Production emission and gas policy, successful delayed network bridge payments, source verification and backing, independent signer operations, recoverable relaying, complete delayed withdrawal and positive settled-G workflows, production graphical/extension wallets, full explorer indexing and ordinary-computer resource measurements need separate approval and verification. Daily settlement scans, delegation work and retained tombstones need growth bounds.

There is no independent audit, guaranteed return, mainnet safety claim or launch date. Publication does not authorize chain activation or real assets. Private faith text must be encrypted by the application on the user's device and must not enter public chat, telemetry or logs.

## Attribution

See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). Imported dependency names are technical and legal identifiers, not alternative project branding. Source publication does not complete license review for future binary redistribution.
