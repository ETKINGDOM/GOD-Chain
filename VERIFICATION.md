# GOD Chain core verification

This record separates exact public-core source checks from acceptance of the
separately deployed synthetic pilot. It is not an independent audit, source-
finality proof or mainnet safety certification. Private tests, browser/extension
source, separate pilot gateways and generated operational inputs are not
distributed in this exact source snapshot. Publication activates no service or
real assets. God EVM + God SDK + GodCometBFT pins remain unchanged.

## Current synthetic participant acceptance

Web/Chrome alpha 0.3.7 is deployed under explicit authorization. The participant
tools support disposable encrypted accounts, recovery, bounded automatic
funding, reviewed GOD transfers, a restricted test NFT collection and six native
staking/G scopes. General public RPC writes, arbitrary contract signing,
website-provider signing, RH activation and mainnet assets remain disabled.

- The read-only G readiness view separates spendable, settled unclaimed and
  pending earned G, settled/pending pools and settled/pending supply. It derives
  the next UTC boundary from committed time without promising settlement or
  payment. The minimum-one-G quote uses only settled liquidity and outstanding
  G with integer flooring. Empty or zero-output liquidity has no positive quote;
  reads burn no G and start no signer. Twenty-three client/profile/installer
  tests and desktop/mobile/actual Chrome fixtures passed, including pending,
  empty-pool and unknown-result guards. Live desktop, mobile and downloaded
  Chrome checks matched all values to exact committed snapshots, with no
  claim, signature, submission, native signer, page error, overflow or browser
  vault storage. This is not positive settled-G workflow acceptance.
- A captured fresh-account reply proved a quota denial before node admission:
  a confirmation lookup had consumed the IP cooldown for the next submission.
  Status/submission now have independent five-second IP windows while keeping
  the combined global request budget. Definite non-admission is distinguished
  only through exact closed response contracts; duplicates, malformed replies,
  generic errors and transport uncertainty preserve unknown signing guards.
  Nothing is automatically resent or retroactively declared rejected.
- Race-enabled gateway/codec/CLI and privacy checks passed. Browser signatures
  passed independent pinned SDK wire checks. Four persistent local validators
  and an observer accepted a fresh browser-signed delegation, start-unbonding
  and donation; its pending entry survived observer restart. These fixtures are
  not independent operator or sustained daily-settlement acceptance.
- One new public disposable account's captured never-admitted signed wire was
  submitted once after the fix, with the same original native ID and no new
  signature. Its matching successful receipt, account sequence advancement,
  removed delegation and real one-GOD pending entry were verified. The existing
  21-day completion time was not shortened or bypassed; initiation is not payout.
  Earlier unknown accounts lacked that proof and were not replaced or resent.
- HTTPS acceptance matched 45 release files, ten preflights and four closed-
  origin denials, with public RPC writes still disabled. The downloaded Chrome
  alpha retained thirteen reviewed files, its stable identity and two host
  permissions. Locked desktop/mobile/Chrome recovery preserved unknown guards
  and recovered the new committed receipt and pending entry without a claim,
  signature or submission. No page error or horizontal overflow was observed.
- Version 0.3.7 activation changed only frontend files and the nginx site root;
  bounded static hash handover checks accounted for graceful old-worker exit.
  Recoverable rollback remained available. All node and gateway processes
  remained unchanged; no signer, economic rule or dependency pin changed.
- Final read-only reconciliation found eleven active services and five node
  views at one fresh committed height and app hash. All returned the matching
  undelegation receipt and post-operation account state. Validator processes,
  identities and signing state were not reset or restarted. Private recoverable
  backups and frontend rollback were retained. Geography is not independently
  distributed, and this is not sustained-load or adversarial acceptance.

Full 21-day release/payout, positive settled-G claim/transfer/redemption,
physical devices, independent operators, sustained resource measurements and
independent security review remain incomplete. No Store release, RH bridge or
real-asset operation is established by these checks. Later historical sections
retain the earlier 2026-10-07 macOS arm64/Go 1.26.8 local source verification;
their browser/host exclusions describe that earlier check, not the current pilot.

## Local one-operator validator-query foundation

The included `QueryValidator`, trusted ABCI and opt-in local RPC interfaces read
one detached current committed operator record, with bounded canonical input
and exact optional height pinning. Unknown operators are explicit absence;
invalid state returns no partial record. Exact stake/shares, fixed commission
and minimum self-delegation, staking status and stored suspension/permanent
exclusion are included. Arbitrary descriptions, contact data, signing keys,
validator/delegator collection scans and promised returns are excluded.

Fifteen selected private query/HTTP RPC regressions passed twice with race
detection in 23.922 seconds. Commit isolation, disk reopen, exact HTTP results,
malformed input, concurrent non-mutating reads, missing operators and closed
transport budgets passed. Actual single-validator loopback execution returned
matching network/account/validator views after signed contract work. Controlled
keeper jail/tombstone fixtures expose changes only after commit; corrupt status
returns no record and changes no store. These fixtures are not live misconduct
evidence. Bonded status is not proof of actual signing, uptime, ownership or
geography. Public gateway discovery and wallet integration are not activated;
no signer, economic rule, validator-selection rule or dependency pin changed.

## Testnet and participant checks

The private module defines 396 substantive named Go tests and two subprocess harnesses. Eight compiled-custody checks use an optional tag; one actual-Linux-binary package check is enabled separately. The checksum-bound whole-module race/coverage run passed with `bridgeevm`, including all fuzz seeds. The node, RH, testnet, CLI, companion and deployment-tool packages took 355.109, 31.711, 171.528, 2.834, 1.706 and 1.853 seconds. Node/testnet statement coverage was 62.7/83.5 percent. Coverage and durations are not safety scores, throughput results or hardware specifications. Nine additional Node client guards passed.

Four persistent validators accepted contract deployment, storage changes and externally signed transfers through restricted RPC, with actual success and revert receipts. Calls and gas estimation discarded their writes. Independent identity initialization and complete-bundle verification were exercised with four validators and a non-signing observer. Signed native delegation and funding propagated, shared native/EVM sequences remained exact and fixed GOD supply was preserved. Catch-up and restart retained original databases and signing progress. All processes ran on one computer, not independent operators or Linux machines.

Private input and RPC tests cover strict JSON, permissions, identity and bundle binding, replaced/missing signers, occupied-port rejection before store opening, bounded hosts/origins/connections, unsupported signing/admin methods, oversized requests and unknown submission outcomes. Only explicit commit-pending rejection permits a simple post-commit retry. Successful reads and provider agreement do not establish authenticated state or independently verified finality.

Encrypted-wallet tests cover fresh exclusive creation, matching native/EVM identities, full-bundle binding, wrong passwords, ciphertext tampering, work parameters, explicit terminal approval, fee/sequence/address limits and exact native/EVM signatures. Passwords are not accepted through browser, arguments, environment or files. Terminal interruption checks restored echo. Personal-key import, recovery, graphical signing and a Chrome extension remain absent.

A fresh four-validator/observer browser workflow passed terminal wallet creation/signing, manual synthetic funding, a GOD transfer, native delegation, actual failure receipts, bounded explorer pages, known/unknown details, input clearing, desktop/mobile layout and graceful shutdown. The browser sent only read requests and produced unsigned downloads; a separate harness submitted signed transactions. Its address-only injected wallet requested only `eth_requestAccounts`. This does not verify a real extension or browser-held signing key. Explorer summaries omit raw memo, calldata, logs and error payloads; other public-chain data is not thereby private.

## Host and package checks

Four host-budget tests, a reviewed-resource test, six service-smoke tests and CLI checks cover explicit thresholds, unprivileged operation, safe directories, exact filesystem units/overflow, matching static resources/security headers, exact-origin access, opaque-origin rejection and consecutive committed progress. Stalls, regressing height/time, repeated-height hash/time changes, chain/supply/asset mismatches, unsafe CORS, redirects, oversized responses, untrusted TLS and cancellation fail closed. Invalid bundles make no requests. Reports use fixed labels and bounded counts, not operational material. The browser workflow also exercised the actual `host-check` and `smoke` commands.

Linux amd64/arm64 binaries passed separately enabled native packaging race checks in 47.348 and 45.819 seconds. Private CLI candidates contained exactly sixteen members, verified against trusted digests, rejected replacement and reproduced identical archives from identical inputs. No archive was extracted or foreign executable run. Runtime and binary-redistribution approval flags remain false. The tools install no service, change no limits, collect no keys and submit no transactions.

Lifecycle vet, module checksums, native compilation, five node-target and two Linux-packager-target builds passed. Only macOS arm64 executed; foreign compilation is not runtime support. The pinned dependencies, lifecycle checksum and founding whitepapers stayed unchanged. Public build commands omit the private test targets and cannot reproduce these private suites. Native Linux, rendered TLS/firewall/gateway configuration, independent hosts, sustained settlement and resource-growth measurements remain release gates.

## Observed local checks

- Core evidence, retained-proof source checks, task-fetch checks and compiled-custody event checks passed repeated private race runs. Setup functions, rejection subcases, subprocess harnesses and fuzz seeds are not counted as substantive named tests. Coverage is not a security score.
- Native authentication checks cover six G operations, single-owner signatures, chain/account binding, ordered sequences, fee tampering, gas exhaustion, replay and whole-batch rollback. Ante rejection commits no fee or sequence; accepted business failure retains both while reverting business writes.
- Execution checks cover legacy, access-list and dynamic-fee Ethereum transactions, shared native/Ethereum sequences, contract deployment and storage, revert fees, protected module accounts and self-destruction quarantine. GOD supply stays fixed without bank mint or burn permission.
- Four separate loopback validator processes produced matching committed application state. Stored commits were independently checked against validator keys and powers. Signed contract deployment propagated over P2P and produced matching code and storage. Three validators continued after one stopped; the absent validator earned no new own-validator G.
- Disk fixtures cover normal process restart, abrupt process termination, original file-signer reuse, conflicting-signature rejection, single-validator recovery and four-validator catch-up and restart. App hash, sequences, G, contract storage and replay protection persisted. Controlled shutdown checks passed with the lifecycle build, not operator-grade or physical-durability certification. Ordinary nodes keep binding version 2; explicit bridge initialization uses version 3, or version 4 with participant routes enabled. Incompatible local data needs reviewed migration, never reset or replacement signers.
- Real consensus evidence checks rejected a forged duplicate vote and accepted two conflicting signed votes. A 5 percent historical-stake penalty entered quarantine without burning GOD or funding redemption; key and operator exclusion persisted. Private keeper fixtures cover unbonding/redelegation exposure, the full 10,000-opportunity downtime window, one-hour suspension and authenticated reactivation. Seeded historical windows are not 10,000 real network signatures.
- Isolated custody checks use compiled bytecode and synthetic tokens. They cover one-for-one conservation, five-of-seven approvals, replay, FIFO/delay/rolling limits, exclusive payment/cancellation outcomes, failing transfers and hostile token behavior. No RH backing or finality was verified.
- All six bridge message adapters executed behind native authentication in a private assembly and in the explicitly enabled node router. Tests cover envelope tampering, ownership, separate quorum validation, approval gas, business rollback, disk replay and default-codec isolation. An unfunded account could not pay its fee from a still-unreleased deposit.
- Initial reconciliation checks use eight synthetic deposits, four planned validators and explicit fee budgets. Exactly 4,004 GOD left the full reserve; trusted staking calls placed 4,000 GOD into bonded custody and left each owner its declared fee budget. No GOD or G was issued. Invalid plans and bad final approvals rolled back the entire initialization, including accounts and replay writes. Canonical ordering, record integrity and committed disk reopen were checked.
- Separate fuzz windows exercised bounded execution, node-genesis and bridge-envelope inputs without failure. The schema-aware node genesis decoder passed 10,607 inputs in a 10-second window with two workers. These are parser inputs, not wallets, chain transactions or source proofs. Both message schemas regenerated byte-for-byte with locked tools.
- Private test binaries compiled with CGO disabled for Linux amd64/arm64, macOS amd64/arm64 and Windows amd64. Native macOS arm64 bridge binaries passed; foreign binaries were not executed. Compilation is not foreign-platform runtime support or ordinary-computer resource certification.
- Go vet, module checksum verification and formatting checks passed. Measured statement coverage was 82.8 percent for the node, 89.4 percent for the bridge keeper and 59.9 percent for bridge messages including generated code. Coverage is not a security score or source-backing proof.

All network processes ran on one computer, not independent geographic operators. Direct application fixtures use synthetic vote flags; only consensus fixtures provide actual signature evidence. Initial funding and custody fixtures use synthetic source statements. The standalone bootstrap API does not install validators; the optional node certificate and atomic initialization supply the separate consent, policy and staking checks below.

## Committed query checks

Six added named private tests exercise latest-committed Go and trusted in-process ABCI queries. Three repeated rounds passed with race detection in 30.065 seconds; repeated CLI status and disabled-command checks passed in 4.792 seconds. Native and EVM encodings return the same account, GOD balance, sequence and G buckets. Network/account results agree on height, committed time and application hash. Large JSON integer metadata remains exact; absent accounts are not created and protected module accounts are identified without granting spending authority.

Queries exclude CheckTx caches and finalized but uncommitted writes, including day-boundary settlement. Signed claims and locks preserve the distinction between spendable, unclaimed and pending earned G; these fields exclude locks and are not total G. Concurrent reads change no working stores. Wrong binding, invalid heights, oversized inputs, unsupported routes and proof requests return no successful result. A deliberately corrupt private amount yields a static error without partial data or panic details. A changed diagnostic clock cannot override committed time.

Graceful process reopen retains identical views, and a single-validator loopback fixture checks queries during signed contract execution. Direct ABCI fixtures use synthetic vote flags; query results are not authenticated state or source-finality proofs. Restricted synthetic HTTP/Ethereum RPC, receipts and gas estimation are exercised separately. Historical reads, public activation and real assets remain disabled or unimplemented. Compilation passed for five OS/architecture targets; only native macOS arm64 was executed. Pins and founding whitepapers remain unchanged, and populated operational configuration stays private.

## Private RH configuration and receipt checks

Private checks cover blank and complete configurations, strict schema parsing, redacted output and owner-readable file loading. Duplicate/unknown/case-aliased fields, signing-key fields, malformed network/address/hash values, incomplete/duplicate signers, unsafe endpoint forms, shared permissions, symlinks and oversized files were rejected. A blank template contains no operational values. Complete configuration and successful probes never enable startup or real assets; Windows ACL behavior still requires native verification.

Synthetic sources exercised all 24 compatibility checks and a loopback client checked their 25 read requests, explicit block-hash pins and canonical requirements. Changed network, code, supply, decimals, custody asset/domain/signers, limits or checkpoint failed closed. Compiled private custody bytecode matched the probe without changing EVM state. Configuration parsing passed 35,724 fuzz inputs in a separate 10-second window with two workers; these are parser inputs, not wallets or transfers.

Nine additional named tests cover strict receipt parsing, exact custody-event selection and unsigned deposit observation. A complete loopback observation used 34 read requests, including the compatibility probe. Pending/null receipts, failed execution, inconsistent transaction/block/log metadata, duplicate or case-aliased selected fields, invalid quantities, removed/duplicate/unordered logs, wrong emitter/recipient/amount, event-padding errors, excessive data, changed code/network, late block/checkpoint reorg and cancellation were rejected. A newer synthetic finalized checkpoint did not invalidate the original stable checkpoint. Broadcast methods stayed disabled, and operational formatting/JSON/errors were redacted.

Actual compiled synthetic custody emitted the event used to build the exact existing deposit digest. Explicit proposal access returned detached values; the entire EVM root, credited escrow and token custody balance stayed unchanged by observation. The receipt envelope and checkpoint were synthetic, not source inclusion or finality proofs. Receipt parsing passed 23,030 fuzz inputs in a 15-second window with two workers. Repeating observation did not grant replay acceptance. `independentFinalityVerified`, `approvalReady` and `realAssetsReady` remained false in simulation and production configuration modes. No real RH endpoint or source receipt was used, and no signing service, relay or native release was introduced.

## Simulation journal and discovery checks

Private journal checks cover deposit, paid and cancelled tasks, immutable request binding, deduplication, concurrency, retained retry budgets, interrupted reads and explicit cache refresh. Fault-injected writes failed before or after persistence; affected instances stopped, and reopen recovered complete old or new state without reporting financial success. Invalid schema/checksum/binding/cache records, changed payloads and production mode were rejected. Private database checks cover exclusive locks, owner-only directories, unsafe paths, wrong binding and graceful reopen. They do not certify process-kill, power-loss or backup rollback; Windows private disk support is disabled.

Discovery checks scanned eight, eight and three synthetic blocks from an explicit origin, retained three expected candidates across reopen and separately checked their unsigned deposit/paid/cancelled observations. Invalid or missing native lookups, duplicate/removed/malformed hints, excessive results, capacity and conflicts rejected whole batches. Provider errors/panics, cancellation, reorgs, changed network and finalized-height regression did not advance progress. Loopback RPC checked exact block-hash filters and no write methods. Hint omissions and false empty responses remain undetectable by this path.

Complete-set checks cover all five supported transaction envelope types, both trie roots, canonical headers, gas totals and blooms. Hashes and global log indexes were derived from complete material rather than receipt metadata. Missing, reordered, mutated, oversized or unsupported data returned no partial candidates. One exact-limit block with 256 transactions and 512 logs plus seven empty blocks matched an eight-block batch and retained progress across reopen. A corrupt second block rejected the whole batch; uncertain saves recovered only old or new task/cursor state. Earlier hint-only history was not promoted by a later complete-set call.

Actual compiled custody deposit, paid and cancelled event bytes passed through complete-set extraction, unsigned observation, proof preparation/retention, material fetching, retained source comparison and combined task evidence review without changing the entire EVM state root. Transactions, gas accounting, headers, checkpoints and native withdrawals remained synthetic. Updated diagnostic and tagged private RH-test binaries compiled for five OS/architecture targets; only native macOS arm64 was executed. No actual RH endpoint or real asset was used, and source-specific RPC/fork compatibility remains unverified.

`receiptSetsMatched` counts only one successful saved batch relative to supplied header commitments. Header ancestry/authenticity, independent settlement, RH-specific fork/receipt/header rules, transaction execution/signatures, code/storage proofs, authentic native withdrawals and historical completeness remain unverified. The later observer cache retains no complete-set inclusion certificate. Financial readiness stays false; there is no signing, broadcast, native payment/refund or automatic service.

## Private receipt proof and material checks

Originally complete-set tasks retained exact event bindings through retry, refresh and reopen. Changed requests, event digests, global positions, unsigned caches and current retained origin/through/checkpoint references failed closed. Manual/hint tasks remained unbound. Provider-reference checks allowed ordinary progress but rejected network changes, retained forks and checkpoint retreats; older references no longer retained by the latest cursor were deliberately not checked. These checks do not prove full ancestry or independently authenticate source truth.

Standalone inclusion and complete-material preparation passed canonical transaction/receipt checks and bounded exact trie-path verification, including unselected material. Changed, omitted, malformed, oversized or unsupported bytes returned no partial witness. Task preparation matched original deposit/payment/cancellation ABI and cache without changing journal state or retry budgets. The selected paths alone remained receipt-local, not a transferable global log-index certificate.

Private proof slots passed creation, exact-repeat saving, regeneration on read/reopen, detached accessors and redacted output. Fault-injected unknown writes stopped the slot until verified reopen; journal bytes stayed unchanged. Separate owner-only disk tests checked exclusive locks and queue/proof name isolation, not physical crashes or cross-database atomicity. Retained data is unencrypted; trusted local ownership and storage are required. Windows disk use stayed disabled.

An opted-in temporary loopback HTTP source exercised exact hash-selected raw methods, strict JSON-RPC/UTF-8 checks, canonical RLP block bodies, receipt arrays, before/after provider references, full root checks and response/material limits. Missing methods/data, redirects, malformed replies, cancellation and unsupported formats failed without fallback. A fabricated self-consistent block still passed relative root checks, explicitly showing that agreement is not authentic finality. The default source rejected both raw methods before requesting them.

Task fetching and retained source comparison passed for deposit, paid and cancelled tasks before/after unsigned caching and after reopen. Exactly one material read matched the original task, complete stored bytes and regenerated witness. Both stores, cache/cursor/clock and attempts stayed unchanged. Provider failures preserved otherwise sound local reads; shortened deadlines and recovered private panics released locks. Concurrent mixed fetch/source-review calls passed race detection, with owned snapshots unchanged by later provider/accessor mutations. A source without single-receipt support passed; forbidden code/view/receipt/log queries were never used. Changing provider code could pass these material APIs and then fail separate ordinary observation.

Existing parser/proof fuzz seeds were included in regression tests. Earlier short receipt-set smoke windows sampled only twenty executions each. The raw-block fuzz window performed only nineteen executions and is limited smoke coverage; a separate raw-receipt format window performed 18,763 inputs, not valid-consensus or financial certification. No new fuzz window is claimed for task fetching, retained source comparison or combined task evidence review. Generated corpora and operational material are excluded.

All finality, approval, signing, broadcast and real-asset flags remained false. The read-only APIs supplied no automatic service, financial acceptance, ledger mutation or source signing. Actual RH methods/encodings, authenticated headers/finality, execution, authenticated code/terminal/native state and independent signers remain separate release gates.

## Private combined task evidence checks

Twelve additional named tests cover deposit, paid and cancelled evidence before/after unsigned caching and after reopen. Each successful call made exactly one material read and derived global log positions across unrelated preceding logs; provider receipt/log methods were forbidden. The same material and regenerated witness produced detached unsigned proposals while both stores, every task stage, all eight lifetime attempts, cache, cursor and clocks stayed unchanged.

Invalid task/state/pins, code/configuration/terminal mismatches, late network/reference changes, transient checkpoint retreats, malformed selected or unselected material and conflicting caches returned no partial result. Short caller deadlines and private provider panics released locks; eight concurrent calls serialized with race detection. Accessor mutation, journal closure and later provider changes did not alter owned results. Redacted formatting/JSON and a temporary loopback HTTP path were checked. Compiled synthetic custody events matched exact unsigned digests without changing the EVM root.

The new report marks complete-set receipt derivation, contract-view checks and unsigned proposal preparation only for a successful call. Terminal matching is resolution-only. Contract-state proof, native withdrawal verification, independent finality, approval, signing, broadcast and real-asset readiness stayed false. There is no financial acceptance, automatic retention or service.

## Checksum-bound shutdown repair

Expanded duplicate-vote and four-validator persistent-recovery runs intermittently returned child exit status 2 during stop. Address-redacted stderr identified `queryMaj23Routine` calling `LoadCommit` and `LoadSeenCommit` after block storage closed, producing `panic: leveldb: closed`. The pinned consensus reactor does not join every peer-query routine before node store closure; a sleeping routine can resume its current iteration afterward.

The reviewed lifecycle build seals worker admission and joins all three admitted peer-worker classes before stores close. Five private helper tests check source checksums, deterministic edits, changed pins, compiler failure, exact module mirrors and repeated preparation. A controlled disk fixture holds six actual workers, confirms stop cannot close stores prematurely and releases them before completion. A negative control without the join failed the strengthened regression. Full and repeated shutdown/recovery checks passed. No error suppression, weakened assertion, original-cache modification, upgraded pin or production timed-delay workaround was installed. This addresses the observed defect, not all adversarial transports or operator failure conditions.

## Authorized synthetic node initialization

- Fifteen node tests verify exact deposit/validator funding, node and plan quorums, every deposit approval, owner consent and consensus-key possession. Separate role domains bind the actual configuration, consensus and genesis policy. Missing, forged, repeated, high-S or wrong-role proofs, ambiguous JSON and missing required consensus fields reject initialization. A freshly re-signed replacement certificate still cannot replace committed genesis.
- The explicit synthetic mode starts with full restricted reserve and calls `InitBootstrap` once before staking; participant declarations are not duplicate bank allocations. Bank, bridge, staking, rewards, execution and metadata writes share one cache. A final bank transfer that wrote and then failed, a later reserve mismatch after bonding and a missing keeper left every mounted store unchanged.
- Committed accounting recorded 4,004 GOD released, 4,000 GOD bonded, exact remaining reserve and zero pending withdrawals, without changing fixed supply or issuing G at genesis. Record integrity, initial-deposit replay and repeated-bootstrap rejection persisted after EVM execution and process reopen. Drift in reserve, pending custody or bootstrap state rejected block preflight.
- Four persistent validator processes committed EVM work, continued with one offline, let it catch up and restarted the whole network using each original database and file signer. State, G, bridge record and reserve reconciled without reinitialization. Commits were independently signature-verified.
- Runtime binding version 3 and authorization digest domain version 2 separate ledger-only initialization from gate-only data. The certificate schema remains version 1. Explicit participant routes use binding version 4 and bind their exact approval-gas policy. Nil-policy nodes retain byte-identical version-2 configuration encoding. No migration or signing-state reset is supplied.
- Updated diagnostic and node-test binaries compiled for five OS/architecture targets. Seven targeted native CGO-disabled macOS arm64 checks passed, including disabled routes, policy/codec isolation, direct bridge execution and reopen, approval gas and admission boundaries. Foreign binaries were not executed. Zero approval gas leaves participant bridge routes disabled.

## Authenticated node bridge checks

Explicit positive bounded `BridgeApprovalGas` requires authorized synthetic bridge genesis; no production gas default is selected. All six node routes retain native owner authentication and ordered account sequences, separately checked bridge quorums, and per-supplied-approval gas charging. Native, bridge and EVM transactions share the account sequence. Wrong envelopes reject before fees; accepted business failures retain fees and sequence while reverting the entire business batch. Committed bridge state and replay protection survive fresh-process reopen without reinitialization.

Four persistent loopback validators accepted signed deposits, withdrawal locks, finalized-cancellation acknowledgements and pauses submitted to one peer. Every peer produced matching application and bridge hashes, balances, fees and sequences. Private checks independently verified each observed transaction commitment, predecessor-header linkage, result Merkle commitment and following header's actual commit signatures. The single-transaction receipt helper is private test code, not a production RPC or general receipt-proof service.

Early withdrawal authorization and queued payment were rejected with accepted fee/sequence accounting and unchanged business state. The network fixture did not wait 24 hours. Successful delayed authorization and payment were checked only in direct synthetic-node and isolated custody fixtures; they are not successful delayed four-node or RH transfers. Time and liquidity limits were not reduced for testing.

Three validators continued with actual three-signature commits while one was offline; the absent validator received no new own-validator G during the same UTC day. The original database and file signer caught up over P2P. All four then restarted with pending custody and the immutable bootstrap record intact. Replay and duplicate outcomes remained rejected; a new cancellation and EVM transaction executed after recovery.

Read-only authorization, paid/cancelled resolution and pause digest helpers were checked for exact configuration binding and unchanged ledger state. They produce unsigned review proposals, not eligibility decisions, valid approvals or independent source-finality proofs.

## Transaction admission boundary checks

Transaction checking defers between successful block finalization and commitment because working stores may already be newer than the checking timestamp. The distinct namespaced response enters no ante processing, charges no gas and changes no mounted store. The check never waits under a mempool lock needed by commitment. `LocalNode.Submit` returns `ErrCommitPending` only for that exact status and makes no internal retry; private callers retry only that no-write status outside the call with a bounded deadline.

A deterministic private negative control bypassing only the wrapper reproduced the base application's time-regression error. Guarded native and EVM envelopes deferred for both new and rechecked transactions; malformed outer input remained rejected. After commitment, the identical native transaction executed once with its expected fee and sequence. Replay and genuinely backwards ledger time remained rejected. The full race-enabled suite and both repeated race-enabled and native CGO-disabled trios passed as recorded above. No source-evidence, reserve, reward or withdrawal-delay rule was weakened. Production throughput and adversarial retry behavior remain unverified.

## Public source checks

The public snapshot contains exactly 104 reviewed files: production-source candidates, synthetic node/RPC/wallet/companion/explorer and host-testing interfaces, eight embedded resources, English guides, an unpopulated nginx review template, schemas, generated messages, pinned modules, checksums, the checksum-bound build helper and required license/notice, sixteen RH read-only/simulation interface files and the preserved public whitepaper. Private tests, fixtures, operational deployment scripts, populated configuration, genesis, addresses, keys, runtime data, dependency folders, logs and binaries are excluded. The template is not an enabled service. Private application or source-review ancestry must not become a public parent.

`make check` verifies the pinned lifecycle compiler input, then runs vet, module checksum verification and compilation. `make build` compiles the node/tool and packager commands. `make status` runs the diagnostic without starting consensus or bridge operations. Public commands do not reproduce the private tests, fuzz windows or custody-bytecode checks. Publication screening checks exact file scopes, both commit identities, file bytes, commit messages and every reachable public ancestor. It is not a comprehensive secret audit or independent code review.

Public source verification requires lifecycle vet, original-module checksum verification, compilation, exact reviewed source bytes and byte-for-byte preservation of the public whitepaper. Native diagnostics must retain blank operational fields, disabled mainnet/real-asset commands and false source-finality/real-asset flags. Foreign binaries are not executed or distributed by source publication.

## Unverified release gates

Operator-grade shutdown and signer lifecycle, power loss, torn writes, corrupt disks, backup rollback and adversarial networks remain unverified. Light-client-attack penalties, quarantine release and governance are incomplete. Production emissions and gas rules, ordinary-computer specifications and sustained state-growth bounds are not finalized.

Successful delayed network bridge payments, verified source observation, independent signers, recoverable relaying and production-backed genesis remain unverified or absent. Positive fee budgets are not locked or certified sufficient; synthetic approvals do not establish independent source truth. Graphical and extension wallets, wallet recovery, complete explorer indexing, actual extension compatibility and secured public-service acceptance remain incomplete. Public deployment, bridge funding, binary distribution and real-asset activation require separate authorization and completed release gates.
