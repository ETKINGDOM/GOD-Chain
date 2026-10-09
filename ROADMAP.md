# GOD Chain Implementation Status and Roadmap

GOD Chain is the independent blockchain project for ETERNAL KINGDOM. This plan separates capabilities implemented and tested locally from work still required for a usable network and its supporting tools. The technical stack is God EVM + God SDK + GodCometBFT.

The current implementation is a synthetic-asset prototype, not a live mainnet or a working Robinhood Chain bridge. GOD Chain's public testnet operates with four validator nodes. Deployment across four geographic regions is planned. The current pilot is not an independently operated, geographically distributed network. An observer preview, automatic synthetic faucet and connected browser test wallet operate under explicit deployment authorization. HTTPS acceptance covered creation, encrypted recovery, an automatic claim, locally signed plain GOD transfers, committed recipient balances and matching explorer data. The public RPC remains read-only. Plain GOD transfers use a separate keyless submission boundary; restricted test collection minting and owner sends use an independent NFT gateway. A separate keyless native gateway and connected web/Chrome review and local signing now expose six fixed staking/G actions. Arbitrary contract calls, validator creation, mainnet and real-asset operations remain disabled. Publishing this plan does not authorize further deployment, custody funding or chain activation.

Detailed local evidence and limitations are in [VERIFICATION.md](VERIFICATION.md), [NODE_RUNTIME.md](NODE_RUNTIME.md) and [BRIDGE.md](BRIDGE.md). The synthetic deployment workflow and public-test gates are in [TESTNET.md](TESTNET.md). No production economic parameter or release date is selected here.

## Status definitions

- **Implemented and locally tested** means code exists and the recorded private checks exercised it with synthetic inputs. It does not mean production acceptance, independent audit or live operation.
- **Partial** means a relevant component exists, but the complete deliverable is missing or unverified.
- **Pending** means the usable deliverable has not been implemented or accepted. It may reuse existing core components.

The remaining plan groups work into six core workstreams and ten supporting modules. These are planning categories, not sixteen independent applications or a completion percentage. Some modules can share a service, interface or wallet core.

## Implemented and locally tested capabilities

| Capability | Recorded implementation | Remaining boundary |
| --- | --- | --- |
| God EVM execution | Signed legacy, access-list and dynamic-fee transactions, contract deployment and storage, fee accounting and business rollback; restricted RPC calls and gas estimation. | Actual wallet-client acceptance, full RPC compatibility and production gas policy remain incomplete. |
| Accounts and authentication | Canonical native `god1…` addresses, compatible EVM identities, ownership checks, shared ordered sequences, bundle-bound encrypted disposable-wallet recovery and same-account backup re-encryption. | Production recovery, signing-key rotation, HD accounts and production-wallet acceptance remain incomplete. Password changes do not revoke old backups. Format conversion is not bridging. |
| God SDK staking and penalties | Authenticated validator creation, delegation, unbonding, signed reactivation, downtime handling and verified duplicate-vote quarantine. | Other misconduct classes, production recovery and governance remain incomplete. |
| GodCometBFT local consensus | Four persistent loopback validators, contract propagation and recovery; independent identity initialization and a non-signing observer tested on the same host. | Cross-host operation, independent human control and adversarial-network reliability are not established. |
| Committed state queries | Latest-committed network, account, G, individual delegation and bounded pending-unbonding views; restricted synthetic HTTP/Ethereum RPC and indexed transaction receipts. | No historical application queries, light-client proofs, full explorer API or accepted wallet product. G fields exclude locks and are not total G. Unbonding timestamps/absence are not payout proofs. |
| G rewards and GOD redemption | Commit-based contribution accounting, bounded prototype G issuance, daily settlement, transfers and locks, and voluntary pool-based redemption. | Production emission approval and scalable settlement remain required. No guaranteed return is implied. |
| Bridge ledger and custody | Synthetic one-for-one accounting, quorum approvals, replay protection, segregated withdrawals, delayed authorization, cancellation rules and opt-in authenticated node routes. | No production backing, independent source verification, live signer operation or financial relayer. Successful delayed payments were direct synthetic checks, not a delayed four-node transfer. |
| Read-only RH evidence interfaces | Private configuration, compatibility probes, deposit and resolution observations, simulation journals, receipt-set checks, proof preparation and retention, and combined task evidence review. | Matching provider data and header commitments are not authenticated finality or contract/native-state proofs. Results remain unsigned. |
| Verification and build tooling | Developer-run regression checks, a checksum-bound lifecycle build, diagnostic compilation for five targets and deployed Linux amd64 pilot services. | Other target runtimes and full cross-platform node acceptance remain unverified; foreign compilation is not runtime support. Private tests and generated material are not distributed. |

These capabilities share a fixed GOD supply of 1,000,000,000 in the prototype. Equal bank supply does not establish external backing. G has no fixed lifetime cap at this stage; its prototype issuance bounds are not a finalized mainnet schedule. Empty redemption liquidity preserves G, and only successful redemption burns the offered G.

Private assembled-application acceptance now covers positive signed daily G
settlement, claim, transfer, proportional redemption, exact rejection reasons,
batch rollback and process-level disk recovery. An empty settled GOD pool
preserves G and its transfer use. Two independent local application runs produce
identical final committed state. Explicit synthetic timestamps exercise the
unchanged 21-day boundary and one-time bank release; this does not complete the
actual public waiting-period gate. The locally verified native-gateway candidate
also refuses contradictory account/network bucket totals and changed network
data under the same claimed commit. It is not deployed, an authenticated state
proof, sustained consensus settlement or production acceptance.

The native-gateway local candidate now requires a separately initialized,
policy-bound private attempt store. It synchronously retains exact hashes before
admission and checks them from disk after process exit without TTL or read-side release.
Invalid/missing storage refuses new submissions rather than resetting unknown
records. Explicit offline seeding never contacts a node or starts a listener.
The local candidate separates its 1,000-entry memory cache from retained disk
membership. Offline aggregate audit and checksum-pinned atomic copy preserve
every hash and leave the original unchanged. Explicit equal/larger copies can
hold up to 100,000 records without changing chain/gas/fee policy; exhausted
capacity still refuses admission instead of deleting records. Version-one
stores retain their original capacity and require an explicit copy to expand.
Isolated offline initialization/audit/copy and refusal cases also pass on the
authorized Linux pilot host as the existing non-root service account, with
network access denied and current consensus data inaccessible. Existing public
processes remain unchanged; this is not native-gateway HTTP/cutover acceptance.
This is not yet a public migration: the deployed pilot's earlier process map,
complete legacy-attempt inventory, sustained-load/storage sizing and safe
intended-host cutover/restore remain separate gates. No chain retry, economy,
waiting period or public activation is authorized by local recovery checks.

## Six remaining core workstreams

All six remain incomplete for production, even where local foundations exist.

1. **Node operation and networking.** Complete reviewed node controls, synchronization, peer admission and safe validator signing lifecycle. Keep local simulation separate from public operation. Recovery must retain the original signing identity and progress rather than reset it.
2. **Production economic parameters.** Approve the G emission schedule, gas and fee policy, bridge approval gas, and source-backed initial validator and fee funding. Preserve fixed GOD supply and the selected reward-pool routing unless a substantive change is explicitly authorized.
3. **Performance and state growth.** Measure and bound reward settlement, delegation work, retained records and historical queries. Publish ordinary-computer requirements only after sustained measurements; being able to compile is not a resource benchmark.
4. **Bridge source verification and backing.** Authenticate source headers, ancestry and finality; verify relevant execution and contract/native state; review token permissions and custody compatibility; reconcile reserve, credited deposits and unresolved withdrawals. Complete successful delayed network flows without weakening timing or liquidity rules.
5. **Governance and controlled upgrades.** Define upgrade authority, approval rules, migration checks, quarantine policy and incident recovery. Decentralization requires independently operated infrastructure, not merely multiple keys or processes.
6. **Production acceptance.** Exercise power loss, torn writes, corrupt storage, backup rollback, adverse networks and prolonged multi-validator operation. Verify signing safety and native runtime behavior on Windows, macOS and Linux. Record unresolved cases honestly and keep activation disabled until the relevant gates pass.

## Ten supporting modules

| Module | Current status | Remaining deliverable |
| --- | --- | --- |
| 1. Node installation and management | Partial | Synthetic identity, assembly, join, startup/check commands, private Linux package creation/verification and recovery guidance exist. Native target-host acceptance, installers, upgrade controls and operator recovery remain required. |
| 2. RPC services | Partial | The public pilot has a keyless read-only HTTPS gateway and a separate plain-transfer-only submission boundary. Actual browser transfer acceptance passed. Full compatible RPC coverage, sustained public-service acceptance, proofs and complete explorer interfaces remain required. |
| 3. GOD Chain wallet | Partial | The connected browser alpha supports encrypted disposable accounts, recovery, funding, plain transfers, a restricted test NFT collection and six reviewed native staking/G actions. HTTPS transfer and NFT acceptance cover the existing submission flows. Ownership cards, same-key backup re-encryption, public-address copying, separate known-native-ID recovery and locked-wallet unbonding progress queries are deployed. Full delayed withdrawal completion, positive settled-G workflows, production recovery, HD accounts, signing-key rotation and independent security review remain required. |
| 4. Browser wallet extension | Partial | A downloadable Manifest V3 test alpha packages the shared connected wallet with exact network pins, a fixed reviewed identity, toolbar entry and constrained transport. Exact-origin public activation and actual HTTPS creation/recovery, claim, committed GOD transfer and restricted NFT mint/send acceptance passed; the English installer provides ZIP/asset hashes and the expected ID. Version 0.3.9 retains six restricted native actions and consolidated read-only state queries, and adds locked-wallet registration pages bounded to eight current records with same-commit cursors. Fresh reads are paced, quota errors are explicit, and staking/unbonding guards match the existing chain policy before signing. Local desktop/mobile and actual unpacked Chrome fixtures cover all six actions, rejected/stale/unknown results and recovery. Physical browser/device acceptance, complete exit and settled-G workflows, website connection permissions, external provider signing, independent security review and separately authorized Store publication remain pending. See EXTENSION.md (separate pilot guide, not included in this core snapshot). |
| 5. Blockchain explorer | Partial | A read-only test explorer displays paginated committed blocks and native/EVM details with actual execution outcomes. The public portal provides checksum-checked, identity-matched account lookup/share links, manual five-block sender/recipient activity scans, observer diagnostics and a redacted local report. A separate opt-in keyless address-index CLI passes complete-block pagination, atomic checkpoints, anchored address pages, process interruption and persistent local-node recovery checks; bounded Linux acceptance read ten committed pilot blocks without changing services. Its production source and English usage guide are in the reviewed core scope, not a public route, complete backfill or NFT index. Full public indexing, a complete historical validator index, proofs and sustained acceptance remain required. |
| 6. Staking and rewards interface | Partial | Connected web/Chrome preparation, six-action explicit signing, restricted native admission, known-hash recovery and bounded committed unbonding progress plus locked single-operator validator lookup are deployed. The browser signatures pass independent pinned SDK byte/signature checks. The portal's separate delegation/pool views remain watch-only. Public disposable-account delegation and start-unbonding produced matching committed receipts and a real pending entry. Locked web/Chrome recovery passed; the progress lookup does not imply payout. Full delayed withdrawal completion, positive settled-G claim/transfer/redemption, fuller validator discovery and sustained public acceptance remain required. This can be part of the wallet or APP. |
| 7. Bridge website | Pending | RH to GOD Chain and return-transfer workflows, progress, limits and explicit failure or cancellation states. Live operations remain gated. |
| 8. Bridge relay and signer tools | Partial | Independently reviewed approvals, safe submission, recovery, reconciliation and duplicate prevention. Existing read-only simulation helpers are not an operating financial service. |
| 9. ETERNAL KINGDOM integration | Pending for GOD Chain | A replaceable chain adapter, wallet connection, GOD and G operations, and submission and retrieval of client-encrypted faith content. This status does not describe unrelated existing APP features. |
| 10. Monitoring and operator documentation | Partial | Synthetic deployment/recovery guides, bundle-pinned health and short service checks, explicit host budget snapshots, redacted diagnostics and a Linux supervision template exist. Target-host verification, alerting, sustained resource measurements and reviewed incident recovery remain required. |

The encrypted terminal wallet, local offline browser build, connected public test wallet and downloadable Chrome test alpha share the fixed encrypted format. The public alpha connects creation, backup/recovery, fresh balance/sequence queries, automatic test funding, reviewed plain GOD and restricted native signing, one-attempt submission and known-hash confirmation. The restricted collection also supports synthetic NFT minting and owner sends through a separate gateway. Signing and admission are not payment: only matching committed execution establishes the displayed success. Faucet claims need no registration or human approval; FAUCET.md (separate pilot guide, not included in this core snapshot) specifies durable idempotency, pilot limits and pool depletion. Complete native exit/G workflows, general NFT tools, external website signing and production-wallet acceptance remain unfinished. Existing compatible wallets can still be evaluated against required interfaces. Public query services and indexing have operating costs; decentralized validation does not make them cost-free.

## Implementation sequence and acceptance

### Complete public-testnet delivery gates

The working pilot is not the complete testnet deliverable. Each gate needs
recorded positive and negative acceptance; a visible page or locally passing
keeper test is not enough. No completion percentage or deadline is implied.

| Gate | Pilot coverage | Still required for complete testing |
| --- | --- | --- |
| Participants and recovery | Connected web/Chrome alpha, encrypted backup/recovery, GOD transfer, automatic faucet, restricted NFT mint/send and known-ID checks | Physical target devices, complete onboarding and incident/feedback workflow; reviewed website-provider permissions if delivered |
| Native staking and G | Connected web/Chrome six-action signing, exact fresh account/sequence checks, restricted admission, ID recovery and bounded unbonding progress; independent SDK byte/signature checks, public committed delegation and start-unbonding with a real pending entry | Full delayed withdrawal completion and failure recovery, fuller validator discovery and positive settled-G claims/transfers/redemption over sustained consensus; failures must preserve balances and existing IDs |
| Explorer and API | Stored blocks, exact transaction outcomes, latest account state, limited scans and read-only retained address/collection-NFT snapshot pages with pinned pagination | Full backfill, complete validator indexing, sustained capacity/load/recovery and explicit retention/pruning rules; no private memo/calldata/plaintext faith feed |
| Node participation | Four-validator synthetic consensus, private assembly/join checks and a non-signing observer | Intended-host installs, reviewed external peering/synchronization, independent operator acceptance, signer identity/progress preservation and safe upgrades; geography must reflect actual infrastructure |
| Reliability and operations | Supervision, developer smoke/resource checks, constrained gateways and frontend rollback | Sustained settlement/traffic measurements, bounded public-load tests, finite-pool depletion recovery, monitoring/alerts and backup/restore drills without replaying validator signatures |
| Economic edge cases | Fixed GOD supply and bounded synthetic G issuance; keeper redemption/locks tests | End-to-end settlement and pool cases on persistent multi-validator operation, exact conservation after failure/restart and no guaranteed G-to-GOD liquidity or reward promises |
| Bridge simulation | Read-only evidence helpers, synthetic custody/ledger and authenticated private routes | Complete UI/relay/signer simulation with source finality, deposits, delayed return payments, cancellation, replay/reorg and interrupted-operation recovery; no real backing or RH activation implied |
| Release security | Pinned builds, exact origins, restricted operations, local secret handling and developer regressions | Adversarial acceptance, independent review as available, published limitations and release/rollback procedures; real assets and mainnet remain separately gated |

A separate [restricted native-operation boundary](NATIVE_GATEWAY.md) passes
fresh-state/signature guards, bounded submission/result tests and pinned
browser-byte authentication. Persistent local consensus acceptance covers
delegation, undelegation, donation and a matching failed delegation. The keyless
gateway and connected review/signing/result/recovery UI are now publicly deployed
under explicit authorization. Settled G workflows were not accepted over
sustained consensus in this milestone.

The bounded progress lookup and native-admission fix are deployed. A captured
fresh-account failure established that a status read had consumed the next
submission's IP cooldown before node admission. Independent IP windows and
closed definite-non-admission handling now preserve the combined global budget
and unknown-result guards. That account's exact never-admitted signed wire was
accepted once without a new signature; its receipt and one-GOD pending entry
were verified across node and locked web/Chrome views. Earlier unknown IDs lack
that evidence and remain unresolved; none was replaced or resent.

Web/Chrome 0.3.7 adds a read-only readiness view for settled versus pending G and
GOD pool buckets, the committed-time UTC boundary and a settled-only indicative
minimum-one-G output. Local and real web/mobile/Chrome checks passed without a
new claim, signature or submission. This is display acceptance, not positive
G settlement, guaranteed redemption or completed payout.

The validator-view foundation provides one bounded committed operator lookup
through local Go/ABCI/opt-in RPC interfaces, without scanning collections or
changing staking. Exact amounts, staking status and stored suspension/exclusion
are not proof of actual signing, operator independence or geography. Public
integration is separately deployed in web/Chrome 0.3.8 through a keyless
query-only gateway. Locked desktop/mobile/Chrome checks cover all four
configured operators without signing. Selection only fills an unsigned form;
unknown TX guards, economic rules and public RPC restrictions are unchanged.
Source publication itself activates no route, and the configured list is not
a complete validator index.

The bounded directory foundation now enumerates current registration records
in pages of at most eight, with an exclusive raw-key cursor and exact commit
pin. Seventeen-validator 8/8/1 pagination and disk reopen passed local race
checks. Local desktop/mobile/actual Chrome fixtures reject changed or malformed
pages and retain unknown-ID guards. This is not a persistent historical index,
an actual consensus-signing set or independent operator evidence. Public
activation and HTTPS acceptance are recorded separately from these local checks.

Web/Chrome 0.3.9 publicly exposes those bounded registration pages through the
existing independent keyless query-only process. Locked desktop/mobile and the
actual downloaded Chrome package checked all four current records in one exact
committed page. No signatures, submissions or claims occurred. The observer and
query process were updated; validators and every existing submission/faucet
process retained their PIDs and original service definitions. Historical address,
NFT and validator indexing, actual signing history and sustained acceptance are
still separate unfinished gates. Public RPC write restrictions are unchanged.

The next functional gates are full delayed withdrawal completion, positive
settled-G workflows, fuller validator discovery and sustained public-service
measurements. Do not open general transaction submission, shorten the exit
delay, reset earlier unknown accounts or fake settled rewards to complete them.
Existing GOD/NFT policies and read-only RPC retain their limits.

The independent address-history foundation now has a local opt-in CLI. It
reads all bounded pages of a selected block and persists compact supported
address membership, execution identifiers and a checkpoint atomically. Local
race checks cover pagination, partial-source/write rollback, corrupt data,
anchored queries during append and abrupt process exit between blocks. A real
persistent local-node transfer record survived index reopen and observer
restart. Bounded Linux acceptance indexed ten already committed pilot blocks,
recovered an existing native exit once and changed no existing process,
listener or gateway configuration. This is not full backfill or public API/UI
acceptance. The reviewed core scope includes its production source and English
usage guide, not private tests or operational data. Complete retained-range catch-up,
independent constrained query-service/browser integration, NFT event indexing,
capacity/retention and storage-failure acceptance remain open.

The separate read-only history query boundary and opt-in browser integration
now pass local schema/admission, stale-source, exclusive cursor, privacy and
desktop/mobile checks. Actual persistent local consensus returned its indexed
fixture transfer before and after observer restart, without writing index
bytes through the query boundary. The authorized synthetic pilot now deploys an
independently hardened query service, bounded working-index sync timer and
atomically published read-only snapshots. Version-two website configuration
enables the manual history panel; version one remains supported with the panel
hidden. Actual Linux concurrency, public HTTPS request/rate controls and
desktop/mobile-width TX lookup passed without claims, signatures or submissions.
Retained coverage is partial and catch-up continues. Complete backfill,
intended-host capacity/load, NFT events, storage-failure review and independent
operation remain separate gates. These checks do not activate production or
expand the earlier GitHub source allowlist.

The independent history CLI now adds a read-only offline logical storage audit.
It checks all declared retained blocks/transactions and reverse-index links,
not only the final checkpoint; cancellation, malformed copies and logical
inconsistency never produce a partial success or repair. Private restore-copy
acceptance preserved exact database bytes and a committed local-consensus TX,
including fresh observer/anchor query checks after restoring the copy. This is
not a live restore, physical-page/power-loss test, authenticated backup or full
address/NFT history. A trusted digest, fresh reconciliation, physical storage
drills and intended-host service-switch acceptance remain separate requirements.
The deployed binaries and website are unchanged by this local tool milestone.

The next NFT-history foundation is a separate pure projection boundary for the
configured synthetic collection. It checks supplied receipt/block/runtime
bindings, distinct compatible-lane indexes and ordered event identities, then
emits only bounded mint/transfer/self-transfer identifiers. Approved-operator calls,
contract recipients and nested events are represented without changing wallet
admission. Missing/failed/malformed results never become fabricated transfers;
raw payloads are not retained. This local helper is not a complete NFT index or
source/finality proof. An independent collection-bound stored index now adds
local atomic block/receipt/event/address/token progress, namespace/budget refusal,
pinned twenty-event pagination and a read-only logical retained-range audit.
Private corruption, interruption between commits, closed-copy restore and actual
committed SDK mint/send/revert acceptance passed. These are stored snapshots,
not fresh source checks or current ownership. A separate opt-in bounded loopback
acquisition now checks all consensus pages, exact retained-height runtime and
matching compatible receipts, then reconciles network/anchors before and after
the run. Actual persistent local consensus mint/send history survived observer
restart, index reopen and isolated restore. Missing/pruned/contradictory evidence
stops without skipping or fabricated events; success is dated service consistency,
not finality or an enduring query grant. Explicit SDK-failure representation now
reconciles the native consensus-hash detail, preserves its lane and accounts for
used gas without a receipt or event. Actual committed SDK stale-nonce rejection,
later normal events, storage corruption refusal, publication and query checks
passed. Sustained intended-host concurrency/load/restore acceptance remains required.
The private Linux package executable additionally
passed acquisition/storage/refusal/recovery fixtures on the intended host under
the existing low-privilege user, isolated loopback, no-new-privileges, memory
write/execute protection and bounded resources. Those package fixtures alone do
not establish deployed acquisition or sustained capacity/physical recovery.

The separately authorized pilot now has a read-only collection-history page,
independent query service, owner-only working/published images and an explicit
bounded refresh timer. It preserves the address-history schema, wallet/extension
assets and generic public RPC write restrictions. HTTPS desktop/mobile checks
match newly committed mint/send TX IDs through token, sender and recipient
filters and the existing explorer. Query flags retain partial stored-event scope,
not current ownership or fresh per-event proofs. Each query reconciles synthetic
network identity, retained headers and exact-height runtime independently.
Missing/pruned/contradictory inputs and replaced snapshots remain unavailable rather
than invented empty history. Full backfill, retention/pruning, sustained
capacity/load/recovery and independent operation remain open. This does not
expand a public source allowlist or activate mainnet/RH/real assets.

The pilot observer and NFT-history reader/writer now include explicit SDK-failure
handling without a data reset or validator/signing change. Actual SDK execution
and storage/browser regressions cover that negative case; no rejected SDK slot
was present in the audited public range. A later low-rate observation exposed a
device/server timestamp disagreement: replies remained unconfirmed under the
unchanged freshness window. An explicit English clock-window warning and private
boundary tests distinguish this refusal without weakening authentication or
inventing activity. Correct time agreement and sustained availability remain
required before production acceptance.

The sequence below is the proposed delivery order. Security and regression checks apply at every stage, not only at the end.

1. **Local node controls and basic RPC.** Begin with private synthetic operation and committed-state queries. Acceptance requires consistent block, account and transaction results, bounded requests, blank public defaults and no unintended public startup or real-asset path.
2. **Local transaction and reward workflows.** Connect a compatible wallet or private signing harness and native interfaces. Exercise transfers, staking, G claims and redemption end to end, including fee/sequence accounting, failed transactions and restart. Show simulated assets explicitly as simulated.
3. **Explorer and staking interfaces.** Reuse reviewed mature tooling where permitted and add the required GOD Chain adapters. Acceptance requires indexed data to match committed node results and honest pending, failed and confirmed states. Missing data must never be presented as a successful payment.
4. **Complete bridge simulation.** Connect the bridge interface, source-verification adapters, independent approval workflow and recoverable relay under synthetic conditions. Exercise deposit, withdrawal, payment, cancellation, reorg rejection, interruption and replay. Longer multi-node delay and backing checks remain separate acceptance requirements.
5. **Wallet products and APP integration.** Deliver the shared wallet core, extension and ETERNAL KINGDOM adapter. Verify account formats, recovery, signing consent, network selection and client-side encryption without exposing secrets or private faith text.
6. **Production release review.** Complete the six core workstreams and acceptance evidence relevant to a proposed release. Actual RH integration additionally requires privately reviewed network and contract configuration, independent signer arrangements, backing and finality verification, and explicit activation authorization. A token contract address alone is insufficient.

The next acceptance milestone is an independently operated synthetic test on intended hosts, with secured observer RPC, disposable-wallet checks, sustained daily settlement, test funding and measured resource use. Existing short local tests do not satisfy those gates. Public deployment still requires separate authorization; mainnet and real assets remain disabled.

### Production transition gates

Testnet iteration does not authorize changing its asset mode or turning its
balances into real GOD. A proposed production release needs a separately
reviewed candidate and explicit final activation approval. These gates are open,
not completed by version 0.3.9 or a passing developer regression suite.

| Gate | Required evidence before production activation |
| --- | --- |
| Participant workflows | Positive daily-settled G claim/transfer/redemption, actual 21-day unbonding payout and failure/recovery checks on persistent consensus; no clock fast-forwarding or fabricated settlement |
| Independently operated validation | Intended-host installation, actual independent key/control custody, secured cross-host peering, catch-up and safe restart/upgrade; reported locations must match real infrastructure |
| Capacity and recovery | Sustained settlement and public traffic, storage-growth/retention budgets, abuse and depletion recovery, alerts and restore drills that never roll back signing progress |
| Economics and initial funding | Explicitly approved production emissions, gas/fee and authority rules, source-backed validator/Gas funding and exact fixed-GOD conservation; test parameters are not approval |
| Bridge and reserve | Independently authenticated source identity/ancestry/finality, verified custody/contract permissions, one-for-one backing, delayed returns, replay/reorg rejection and interrupted-operation reconciliation |
| Release and control | Reviewed immutable candidate, migrations, independent/adversarial review as available, documented unresolved risks and upgrade/rollback authority; no claim that upstream maturity transfers an audit |
| Activation decision | Review all required evidence and remaining risks with the owner, approve the exact production configuration privately, then explicitly authorize activation; keep synthetic and real-asset environments separate |

No mainnet date, guaranteed return, public safety certification or test-to-mainnet
asset conversion is selected here. Development can continue on open gates without
buying infrastructure, moving real assets or weakening the existing test rules.

## Reuse and publication requirements

Prefer mature open-source components and standard interfaces where they fit God EVM + God SDK + GodCometBFT. Before adoption, review the exact license, pinned version, maintenance and compatibility. Preserve required copyright, license and attribution. Rebranding or reusing code does not transfer another project's audit or production assurances to GOD Chain.

Keep populated addresses, contract settings, operational endpoints, credentials, keys, private tests, synthetic fixtures, runtime data and binaries out of the public repository and its reachable history. The existing whitepaper remains preserved; this plan does not silently revise its economic model.

Wallet signing keys must remain under user control on the user's device and must not be collected by the APP or query services. Private prayer, confession and praise content must be encrypted before submission and must not enter public chat, telemetry, server logs or explorer plaintext views. Encryption does not make public chain data erasable.

GitHub publication is separate from website deployment, network launch and real-money operation. Each requires its own explicit authorization and completed release gates. There is no independent audit, mainnet safety certification, guaranteed yield, fixed launch date or assurance that a distributed launch is already decentralized.
