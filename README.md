# GOD Chain

GOD Chain is the independent blockchain project for ETERNAL KINGDOM, a global faith-centered world for prayer, confession, praise and fellowship. This repository contains the [Whitepaper Draft](WHITEPAPER.md) and source candidates for an isolated, synthetic-asset prototype. God EVM execution, God SDK staking and GodCometBFT consensus have been connected in local tests. No public network, RH connection or real assets are enabled.

God EVM + God SDK + GodCometBFT are the project component names. Necessary upstream import paths, licenses and attribution remain intact; these names do not claim independent invention of imported infrastructure. All project-owned public content is in English.

See the [Implementation Status and Roadmap](ROADMAP.md) for locally verified capabilities, six remaining core workstreams, ten supporting modules and the proposed delivery sequence. The plan distinguishes implemented prototypes from production acceptance and does not enable a network or real assets.

## Execution and validators

`internal/godnode` assembles accounts, bank storage, staking, execution and the local consensus adapter. It accepts bounded legacy, access-list and dynamic-fee Ethereum transactions, including contract deployment and storage. Native direct-signature transactions share account sequences with the Ethereum path. There is no Ethereum JSON-RPC server or wallet interface.

Signed validator creation requires 1,000 GOD self-stake and a fixed 10 percent commission. Delegation requires 1 GOD; unbonding takes 21 days. The prototype selects up to 32 active validators and bounds delegators per validator to 256 for local resource control, not as a finalized worldwide limit. G allocation uses the preceding verified commit and stake captured before that block's transactions. Missed signatures receive no allocation. The per-block budget must be explicitly configured; no mainnet emission rule is selected.

The balance adapter transfers existing GOD instead of granting mint or burn permission. Fees remaining after Ethereum gas refunds enter pending rewards. Native signed fees retain their full-requested-fee rule. Value otherwise discarded by self-destruction-to-self is quarantined, an intentional native-value difference. Default inflation and burning penalties are not imported as economic rules.

Consensus-verified duplicate votes apply a 5 percent historical-stake penalty into protected quarantine, never burning GOD or funding G redemption. The consensus key and operator are permanently excluded. More than 1,000 misses in a full 10,000-opportunity signing window suspends a validator for at least one hour without reducing principal. Reactivation requires the authenticated operator and adequate self-stake. Other evidence classes and production recovery governance remain release gates. See [NODE_RUNTIME.md](NODE_RUNTIME.md).

Opt-in disk storage preserves application state, consensus databases, WAL and original file-signing progress. Private fixtures cover normal restart, abrupt process termination and four-validator recovery. The checksum-bound lifecycle build joins admitted peer workers before closing stores and passed the observed shutdown regression and controlled worker tests. This is not operator-grade reliability under every failure. Ordinary nil-policy nodes keep binding version 2; explicit synthetic bridge initialization uses version 3, or version 4 with authenticated routes enabled. Incompatible local data requires reviewed migration, not a reset or replacement signer.

Transaction checking defers during the interval between block finalization and commitment, before ante processing or store writes. `LocalNode.Submit` returns `ErrCommitPending` only for that distinct status and makes no internal retry. This avoids checking a newer working ledger against an older checking timestamp without weakening authentication or time rules. The check never waits under a mempool lock needed by commitment.

## Local committed queries

`QueryNetwork` and `QueryAccount` provide detached latest-committed network metadata, GOD balances, auth account numbers/sequences and three G buckets. Native and EVM address encodings query the same account. CheckTx and finalized but uncommitted changes are excluded, including across daily settlement. Queries create no account, settle no reward and change no fee or sequence. Responses mark synthetic operation, keep real assets disabled and use exact decimal strings for amounts and signing-related integers.

The account G fields exclude application locks and must not be labeled total G. GOD bank balance is not a spendability estimate; an application hash is not a block hash or proof. Only the trusted in-process ABCI query adapter exposes the bounded `/god/network` and `/god/account` routes. No external ABCI socket, HTTP or Ethereum JSON-RPC server, public startup or wallet connection is enabled. See [NODE_RUNTIME.md](NODE_RUNTIME.md#committed-state-queries).

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

Read-only authorization, resolution and pause digest helpers build unsigned review proposals from explicit configuration. They do not read ledger state, establish eligibility, grant approval or prove source execution. Independent source-finality verification, independent signers, production-backed node genesis and a financial relayer remain absent. The 10,000 GOD exposure limit cannot fund ten minimum-self-stake validators plus positive fees; a 32-validator launch is not established. No deployment, public RPC or funding command is provided. See [BRIDGE.md](BRIDGE.md).

## Private RH connection and receipt checks

`internal/godrh` provides replaceable, read-only source interfaces and a private configuration loader. Operational fields remain blank until configured privately: source/native names, source network ID, RPC endpoint, token/custody contracts, reviewed runtime-code pins and seven distinct public signer identities. No address, endpoint or credential value is supplied. The loader rejects ambiguous fields, signing-key fields, symlinks, shared permissions and oversized input. Populated files stay ignored and owner-readable; Windows ACL behavior still needs native verification.

`godd rh-template` emits a blank simulation-mode template. `godd check-rh <private-file>` checks configuration offline; `godd probe-rh <private-file>` explicitly performs bounded source reads. Code and contract view calls are pinned to one provider-reported finalized block hash with canonical-block requirements. Checks cover network identity, code pins, 18 decimals, fixed supply, custody asset/domain/signers and bridge limits, then recheck the original block. Reports, formatting and errors redact operational values. No command deploys, signs, funds a bridge or initializes a node.

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

The diagnostic reports `local-consensus-execution-prototype`, `localNodePrototype: true` and `localCommittedQueriesImplemented: true`, while retaining `nodeReady: false` and `realAssets: false`. Startup and financial commands fail closed. A build does not start a chain or verify backing. `make compile-targets` cross-compiles only this diagnostic; foreign-platform compilation is not runtime certification.

Generated protobuf messages are included, so ordinary builds need no protobuf compiler. `make generate-proto` uses protoc 33.0 and the locked generator. Solidity custody source requires the separately verified compiler described in the lock; no compiler, private fixture, bytecode or operational build script is distributed.

Private test suites, fixtures, runtime data, keys, addresses, binaries and logs are intentionally excluded. Public build commands do not reproduce the private verification suite. See [VERIFICATION.md](VERIFICATION.md) for observed checks and their limits.

## Remaining release gates

Operator-grade shutdown, physical-durability testing, signer lifecycle, adversarial peers and evidence, and governance remain incomplete. Production emission and gas policy, successful delayed network bridge payments, source verification and backing, independent signer operations, recoverable relaying, wallet support and ordinary-computer resource measurements need separate approval and verification. Daily settlement scans, delegation work and retained tombstones need growth bounds.

There is no independent audit, guaranteed return, mainnet safety claim or launch date. Publication does not authorize chain activation or real assets. Private faith text must be encrypted by the application on the user's device and must not enter public chat, telemetry or logs.

## Attribution

See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). Imported dependency names are technical and legal identifiers, not alternative project branding. Source publication does not complete license review for future binary redistribution.
