# GOD Chain Implementation Status and Roadmap

GOD Chain is the independent blockchain project for ETERNAL KINGDOM. This plan separates capabilities implemented and tested locally from work still required for a usable network and its supporting tools. The technical stack is God EVM + God SDK + GodCometBFT.

The current implementation is a synthetic-asset prototype, not a live mainnet or a working Robinhood Chain bridge. The pilot uses four validator processes and an observer on one founder-operated machine. The selected initial release topology remains single-host and multi-service; independent hosts and geographic distribution are later expansion work, not established infrastructure. An observer preview, automatic synthetic faucet and connected browser test wallet operate under explicit deployment authorization. HTTPS acceptance covered creation, encrypted recovery, an automatic claim, locally signed plain GOD transfers, committed recipient balances and matching explorer data. The public RPC remains read-only. Plain GOD transfers use a separate keyless submission boundary; restricted test collection minting and owner sends use an independent NFT gateway. A separate keyless native gateway and connected web/Chrome review and local signing now expose six fixed staking/G actions. Arbitrary contract calls, validator creation, mainnet and real-asset operations remain disabled. Publishing this plan does not authorize further deployment, custody funding or chain activation.

Detailed local evidence and limitations are in [VERIFICATION.md](VERIFICATION.md), [NODE_RUNTIME.md](NODE_RUNTIME.md) and [BRIDGE.md](BRIDGE.md). The synthetic deployment workflow and public-test gates are in [TESTNET.md](TESTNET.md). No production economic parameter or release date is selected here.

## Mainnet delivery critical path

A functioning pilot and passing component tests are not a completed mainnet.
The following are integrated deliverables, not a completion percentage or an
indefinite list of additional prototype milestones. Initial operation uses the
selected founder-operated, single-host topology; independent hosts are later
expansion work and must not be claimed as deployed infrastructure.

Initial bridge custody is also founder-controlled: the founder controls all
seven member keys, while the existing contract still requires five distinct
member signatures. This is one custodian, not seven independent operators.
Finding independent custodians is not an initial-release condition. Version-2
custody preparation explicitly records this declaration; it does not establish
key possession, authorize key generation/ETH spending or enable real assets.
Signer rotation is absent from the immutable prototype and must not be
promised as an automatic later change.

| Deliverable | Current boundary | Completion evidence required |
| --- | --- | --- |
| Production node assembly | Economic/runtime/bootstrap/owner-consent inputs compose into one privately staged candidate. Bounded rehearsal and a cancellable persistent service share the guarded runtime; four validator processes agree, stop with SIGTERM and restart using the same stores/witnesses. Continuous service rechecks original pins/profile and refuses changed input or signing rollback. Non-prototype configuration and ordinary `init`/`start` remain refused. | Reviewed economic/upgrade/control policy and production installation/launch entry point; authenticated source-funded genesis, fee/stake funding, immutable validator/bridge policy and intended-host replay/restart acceptance. No conversion of synthetic balances into backed balances. |
| Live two-way bridge | Credit/return accounting and retained source/native evidence exist. The shared source book reserves unsigned nonce/Gas intent, durably consumes signing and recovers exact private output after interruption. A one-use loopback HTTP handoff now composes that same book with complete HTTP receipt recovery; compiled custody verifies token movement, lost responses and restart without redispatch. Production configuration, non-loopback writes and real-network financial activation remain refused. | Actual source/custody binding, current backing and finality, reviewed key custody, coordinated account/nonce use, bounded production broadcast, exact successful receipts, unknown-result recovery and delayed return/cancellation acceptance. A token CA alone does not supply this workflow. |
| Production wallet and APIs | The hosted wallet, history, explorer and staking tools operate on the pilot. A separate private launch observer now replicates the bridge-funded candidate and exposes bounded committed account/staking/G/bridge and block-history reads. It rejects write RPC and cached contract execution. Production wallet routing and real bridge submission are not enabled. | Official production configuration; committed GOD/G asset and NFT views; signed operations with exact TX links; bridge request/progress/failure views without synthetic balances or fake completion. |
| Intended-host release and recovery | Local and isolated Linux process drills exist. Actual production installation, signing-safe restore and whole-host recovery are not accepted. | Bounded simultaneous load/resource measurement, alerts and retained backups, signing-safe recovery/reboot/rollback, followed by separately authorized limited real-asset acceptance. |

### Source custody creation preparation

Creation now occupies the first nonce and execution-Gas reservation in the same
bounded source sender book used by later custody calls. Original creator,
source/native names, token, predicted custody, compiler-bound runtime and seven
members must match. Pre-retained checkpoints cover signing, signed-file binding,
one dispatch and conditional observation. A successful creation observation
allows subsequent nonce-offset-one calls; unknown/reverted creation cannot.
Existing method-only histories stay byte-compatible. This closes cooperative
creation-versus-call reservation composition, not production account/key fencing,
live nonce/fee availability, source finality, real signing or financial activation.
The selected key-custody release gate no longer requires independent custodians.

The reviewed contract now has an offline creation workflow: pinned actual
compilation, fixed constructor ABI, seven ordered members, explicit sender/nonce
and Gas execution-fee budget, predicted CREATE address and a privately retained
unsigned packet. Rechecking recompiles original materials rather than trusting
supplied bytecode. This closes creation preparation, not a real deployment,
nonce reservation, live fee estimate or custody/asset activation. Exact signed
creation review now composes complete receipt roots, compiler-bound runtime and
same-header account proof with an append-only private simulation journal.
Pre-retained next-state pins and forced-process recovery preserve UNKNOWN with
no second export. Production account coordination, broadcaster, authenticated
source receipt/state/finality and financial activation remain open.
Source-chain funding/finality, source custody limits
and control policy, safe key custody and exact production financial handoff
remain required. See [custody creation](CUSTODY_DEPLOYMENT.md).

### Launch materials compose into a private candidate

`init-launch-observer` and `serve-launch-observer` bind a fresh non-validator
identity to the same independently pinned assembly and staged candidate as the
guarded validators. The observer cannot sign consensus. Explicit loopback HTTP
serves committed network/account, staking/G, bridge/withdrawal and indexed
block/transaction views under a positive read-method allowlist; transaction
submission and cached contract execution remain closed. Exact ledger agreement,
block-history agreement, retained restart and write/stale-input refusal have
been tested with four validator processes and one observer on one computer.
This connects the private running candidate to a query boundary, not the
production browser wallet, public gateway, live source bridge or mainnet.
See [candidate observer](NODE_RUNTIME.md#private-launch-candidate-observer).

The stopped service preflight now checks the original candidate, signing
witness and exact reviewed Linux executable under both existing leases.
Private supervisor preparation stages a no-restart, loopback-only unit with
explicit CPU/memory/task/file budgets and startup/stop deadlines. It installs
nothing, creates no account/key and refuses active/rolled-back homes or unsafe
paths. An isolated intended-host hardened four-process fixture now passes
parser and retained-state checks without changing public services. Actual
per-node production installation and sustained capacity/recovery acceptance
remain required; these checks do not turn synthetic funding into backing.
See [supervisor preparation](NODE_RUNTIME.md#private-candidate-supervisor-preparation).

The economic candidate and bridge genesis no longer have to be reviewed as
unrelated materials. One private assembly matches validator count, per-block G,
minimum fee, declared cadence and actual stake/Gas allocations, derives the
native/consensus genesis from approved deposits and verifies the exact quorum
and owner/consensus consents. The staging command writes the actual detached
runtime/genesis to a fresh private file, preserving existing files and refusing
incomplete or changed inputs. Actual synthetic ledger initialization, fixed
supply and disk reopen pass; no real RH backing is asserted. This closes a
launch-material composition gap, not production start or intended-host release.
See [candidate assembly](NODE_RUNTIME.md#private-launch-candidate-assembly).

The candidate also has an executable private rehearsal path. Startup re-derives
it from the independently pinned signed assembly, checks original validator and
peer identities and requires the same retained signing witness. Four temporary
validator processes produce matching fixed-supply, bridge-funded staking and
signature-derived G state, then continue from the same stores after restart.
This closes the staged-file-to-running-ledger rehearsal gap, not authenticated
RH funding, live financial relay, production launch or VPS installation.
See [candidate execution](NODE_RUNTIME.md#run-a-privately-staged-candidate).

The same guarded candidate now has a persistent `serve-launch-candidate`
entry point rather than requiring a repeated timed rehearsal. SIGINT/SIGTERM
joins consensus before closing stores and releasing ownership. Explicitly
paced reports distinguish actual committed progress from a non-progress
heartbeat; immutable candidate/profile changes stop without repair. Four
compiled processes preserve equal bridge-funded staking and on-chain G across
whole-group restart. This closes the private continuous-service lifecycle gap,
not authenticated RH funding, public installation, production asset mode or
mainnet launch. Intended-host resource/supervisor/recovery acceptance and actual
financial activation remain separate. See
[persistent candidate service](NODE_RUNTIME.md#persistent-private-candidate-service).

The source-side workflow now includes exact unsigned preparation, one explicitly
injected local signing attempt, private signed-envelope retention and the
existing shared nonce/Gas journal's submission/complete-receipt recovery.
Signed responses cannot change the original target, sender, request or fee
plan. The shared sender journal now owns unsigned nonce/Gas intent and retains
signing UNKNOWN before the callback, so restart cannot recreate a signable
attempt. A complete original private signed file can be explicitly recovered
without signing again. The standalone helper remains process-local; the new
journal protection is not hardware key ownership or protection from bypassed
tools/whole-history-and-pin rollback. No production signer or signing CLI is
installed. Submission UNKNOWN is retained before the
callback; successful admission is not payment. Actual compiled-custody runs
cover deposit/payment balances, cancellation and pause, reverted execution,
lost responses and unchanged exact recovery across restarts. Actual child
termination at handoff and observation cannot turn the old envelope into a
second send. This closes local source signing/handoff/recovery composition,
not safe live signing, authenticated RH finality or financial activation. See
[private signing](BRIDGE.md#exact-private-source-signing-integration) and
[durable signing](BRIDGE.md#durable-signing-in-the-shared-sender-book) and
[source submission/recovery](BRIDGE.md#composed-source-submission-and-receipt-recovery).

Source handoff also has a bounded HTTP composition rather than only an injected
function. A private simulation configuration and literal loopback endpoint select
one exact signed request. The transport cannot follow redirects, reuse a
connection, replay its body or inherit ambient proxy/TLS settings. Only the
original TX hash acknowledges admission; payment remains unverified. The same
book prevents a fresh transport from redispatching an UNKNOWN envelope after
restart. Compiled-custody acceptance now executes the received wire and recovers
its full receipts through HTTP for all four actions and both outcomes. This
closes the isolated network handoff/recovery gap, not live RH broadcast, source
finality, production signing authority or mainnet activation. See
[one-use HTTP handoff](BRIDGE.md#one-use-http-submission-and-exact-receipt-recovery).

### G is an on-chain ecosystem asset

Production G must be owned, settled, claimed and transferred in the
consensus-managed reward module, not credited by a web server or fabricated by
a wallet display. Pending contribution estimates are not spendable balances.
Claiming settled G moves it into the holder's spendable balance without issuing
it twice; signed transfers debit and credit the committed chain state. Wallets
read that state, retain transaction identities and expose confirmed transaction
details. Gas remains GOD. Voluntary successful redemption burns only the offered
G; empty liquidity preserves G. Current module-native G does not imply an ERC-20
contract, exchange listing or market price. Pilot G remains a worthless test
asset and must not be represented as production G.

### Bridge participant interface

The official test wallet now has a dedicated `/bridge/` preview route and a
wallet toolkit entry. Participants select RH-to-GOD or GOD-to-RH, connect an
existing wallet's public address and enter an amount. The recipient is derived
from the checked account encoding; no contract address, node, account number,
sequence or typed approval is required. A compact review presents the route,
recipient, gross amount, separate Gas/bridge fees and waiting terms.

Current fee, net-receive, waiting-time and live-history services are not
configured. The interface displays their absence rather than zero fees,
fabricated transaction records or a completed transfer. The progress board is
explicitly a planned journey. Connecting is address-only and the confirm action
cannot sign or submit. The static preview is published on the existing test
website; publication does not activate either bridge direction. Existing Test
Swap routes/configuration and signing workers are preserved. The live financial
relay, fresh quotes, original-transfer lookup and exact two-network TX links remain
part of the production deliverables above.

Fresh local acceptance now combines bridge genesis/routes and immutable
validator-key possession policy. A candidate funded only through synthetic
bridge deposits registers, contributes verified consensus signatures, accrues
G, retains its original signing identity across restart and stops signing after
exit. The same combined policy passes the signed G claim/transfer/redemption and
bridge-return disk workflow. These close a concrete composition gap, not any
of the four production deliverables above. Public peering with bridge policy,
production start, existing databases and deployed services remain unchanged.

Both RH-to-GOD credit and GOD-to-RH return accounting exist in the prototype.
The candidate now supplies latest-committed bridge overview and one-withdrawal
queries, with exact waiting, authorization and attested payment/cancellation
states. Signed fixture/disk-reopen acceptance passed locally and in an isolated
Linux VPS unit without changing the fourteen public services. These are native
ledger views, not an independently verified RH payment or live relay; the
existing public gateway still refuses them. Mainnet and real bridge movement
remain disabled. See [two-way status acceptance](VERIFICATION.md#committed-two-way-bridge-status-query-candidate).

The candidate relay additionally matches one retained RH return receipt to an
unfinished native withdrawal using a separate bounded native read adapter.
Binding, identity, accounting, queue eligibility and same-commit pre/post checks
refuse inconsistent or changed snapshots. This is simulation-only operator-view
agreement, not authenticated native/source finality or an automatic payment
queue. No deployed service or real asset is changed. See
[paired return evidence](BRIDGE.md#paired-native-withdrawal-and-rh-return-evidence).

The return candidate also has a durable, unsigned simulation outbox. It
retains the original paired evidence, records unknown before one local-simulator
handoff and checks the exact original native terminal attestation without
retrying payment/refunding. Private database and forced-process-exit recovery
checks preserve the original stage and reject another handoff. Separately
authorized, resource-limited Linux VPS acceptance passed all thirteen selected
checks without changing any of the fourteen existing services; its temporary
unit was stopped. The records and source/native views remained synthetic.
This is scoped to one exclusively owned slot, not global duplicate protection,
a native state proof or an activated financial relayer. See
[durable outbox scope](BRIDGE.md#durable-unsigned-return-resolution-outbox).

The offline return-message candidate now connects an observation/outbox proposal
and the configured quorum approvals to the exact native payment or cancellation
message. Signed SDK checks cover ownership, exact-once accounting, terminal
conflict refusal and database reopen. Invalid approvals cannot release an
unknown outbox. This adds message assembly, not an outer signing/submission
workflow, authenticated finality or a live financial relayer. No deployed
service or real asset is changed. See
[native return message assembly](BRIDGE.md#offline-native-return-message-assembly).

The next return candidate verifies the exact already-signed native envelope and
retains it in a separate private attempt bound to the original consumed outbox.
Unknown is saved before one local-simulator handoff; failed acknowledgment,
concurrent takes and fresh-process recovery cannot make that slot retryable.
Retained terminal correspondence does not prove inclusion of the same outer
transaction or authenticate RH/native finality. The wallet allowlist and
deployed services are unchanged. Production sender-wide sequence coordination,
signing/submission authority and financial receipt acceptance remain separate.
See [native attempt scope](BRIDGE.md#reviewed-native-transaction-and-durable-simulation-attempt).

The shared native return journal now composes consecutive sender sequences,
exact signed payment/cancellation envelopes, original consumed outboxes and a
lifetime maximum-fee budget. Local concurrency and abrupt-process-exit checks
retain unknown before one ordered simulator handoff and refuse replacement,
repricing, gaps or duplicate withdrawals. This closes the cooperating simulation
queue's per-slot composition gap, not production account-state authentication,
confirmed sequence progression, reviewed key custody or live broadcast. All
producers must use the same book; independent tools/stores are not fenced. See
[shared native sequence scope](BRIDGE.md#shared-native-return-sequence-and-fee-budget).

An explicit local submission adapter and exact native execution reconciliation
now share that same sender book. Unknown is saved before the call; admission
cannot claim completion. Indexed transaction/result roots, the subsequent
header and quorum signatures are checked under independently supplied native
trust. The original envelope, outbox and stored execution observation survive
disk reopen and abrupt process exit without resubmission. Actual four-process
consensus exports cover success/failure and recovery; cryptographic fixtures
add multi-transaction indexing and fault injection. This closes an exact-native-
receipt recovery gap, not RH financial execution, authenticated trust discovery,
account progression, key custody or production bridge activation. See
[native execution recovery](BRIDGE.md#exact-native-return-submission-and-execution-recovery).

The official web wallet now presents button-only staking: configured node
cards, GOD amount presets, an upper G/day illustration, current positions,
verified-history duration, settled-G claiming and unstaking progress. It uses
the original local signing, fresh review and unresolved-attempt safeguards.
After unlock, one confirmation popup shows Gas; no per-operation password,
operator address or typed approval is required. Missing history or unavailable
data remains explicit, not invented. Existing native/G advanced tools are
retained in a closed section. Only static wallet JS/CSS changed; services,
configuration, economic rules and signing workers are unchanged. See
[staking-interface verification](VERIFICATION.md#button-only-official-wallet-staking).

Staking display now uses one atomic latest-committed query for the configured
nodes, G and exits. Background refresh retains checked values and node focus;
failed refreshes are explicitly stale and cannot enable signing. The authorized
non-signing observer/query update leaves the four signing validators and all
economic rules unchanged. See
[atomic staking verification](VERIFICATION.md#atomic-staking-display-and-background-refresh).

Successful reviewed GOD transfers reset their send draft and return to
Overview, while preserving the committed TX and recovery record. Failed,
unknown and unrelated lookup results do not trigger successful-send cleanup.
Staking node/amount selections are local, retain button focus and no longer
repeat all node reads on every click; fresh signing checks remain unchanged.
See [wallet completion verification](VERIFICATION.md#official-wallet-transfer-completion-and-staking-selection).

Ordinary GOD transfers in the hosted web wallet, Chrome 0.3.10 and transfer
gateway have no application-level single-transfer amount cap. Positive
protocol-valid amounts still require enough committed funds for amount plus
Gas; all existing nonce, role, request-quota and unresolved-TX protections
remain. Actual hosted acceptance confirmed one 15-GOD transfer, the receiving
wallet balance/history and the matching explorer TX. Faucet quotas and mainnet
activation gates are unchanged.

Separate hosted native acceptance now confirms naturally settled G claim,
one-G transfer and one-G proportional redemption/burning using only
project-generated worthless test wallets. Original committed receipts,
account/sequence changes, pool/outstanding-G changes and the exact hosted
explorer links passed. The source retained its other spendable G. No public
clock, economic parameter or service was changed. This closes that narrow
positive G pilot acceptance, not sustained participant staking, actual elapsed
unbonding, RH backing or mainnet activation. See
[positive G acceptance](VERIFICATION.md#hosted-positive-g-claim-transfer-and-redemption).

Intended-host private acceptance additionally passes an integrated signed
bridge-ledger credit/stake/G/return/unbonding workflow with exact fresh-process
state recovery. Its source statements, vote flags and delayed timestamps are
synthetic. A separate actual four-process GodCometBFT bridge test passes quorum
receipt agreement, offline catch-up and checkpoint recovery on the same VPS.
Original services and deployed keys remain unchanged. Neither result executes
real RH custody or completes actual elapsed-period payout. See
[integrated business acceptance](VERIFICATION.md#isolated-linux-bridge-staking-and-g-business-acceptance).

The local candidate additionally protects cooperating node starts with an
exclusive retained-workspace lease and offers a bounded offline constructor for
four exact source-custody methods. The offline CLI additionally checks separately
pinned private requests, creates only new unsigned call files and reconstructs
retained files without source reads, signing or submission.
Fresh synthetic validators can additionally opt into an independently retained
signing witness and same-directory public-key-wide process lease. It refuses
stale/missing progress before startup and withholds signatures until both
progress copies are retained. Local single-validator and four-validator restart
checks do not complete signer-safe backup restoration, physical durability,
remote fencing or independently authenticated retention. Used deployed keys
cannot be automatically adopted. See
[signing witness boundaries](NODE_RUNTIME.md#independent-retained-signing-witness-candidate).
The stopped synthetic `guard-check` preflight now holds both existing local
leases, compares exact signing progress and emits a redacted result without
starting databases/listeners or changing files. Missing leases are never
recreated during checks or signer reopen. Private normal-stop/check/restart and
stale-backup refusal tests do not authorize mainnet activation, validator
restoration or migration of deployed keys.
The guarded synthetic node workflow also binds its original key/progress files,
refuses live disk rollback even when memory is newer, and checks/syncs the exact
state file and containing directory before advancing the separate witness.
New and reused signatures require a final original-file/witness match. This
does not certify physical durability, authenticate independent retention or
provide an upgrade/restore procedure. See
[original signer file binding](NODE_RUNTIME.md#bound-original-signer-files).
The stopped synthetic `upgrade-check` candidate now holds both existing leases
through exact private archive/staging verification and checks original node
inputs again before returning redacted integrity evidence. It neither executes
nor installs the candidate. Database compatibility/migration, retained-state
upgrade/rollback drills and production activation remain unverified and are
explicitly false in its report. See
[candidate integrity scope](DEPLOYMENT.md#stopped-candidate-integrity-preflight).
Separate intended-Linux-host acceptance now runs two distinct reviewed
programs against one fresh isolated synthetic validator: baseline, candidate,
then baseline again without restoring signer or database state. Six committed
transactions, exact balances/sequences, contract storage, earlier receipts,
block hashes, retained identities and monotonic signing progress passed on the
VPS; all fourteen existing services were unchanged. The candidate also refuses
already-cancelled startup before private-state preparation. This narrow
compatible-pair result does not establish arbitrary migrations, deployed-key
adoption, four-validator rolling upgrades, whole-host recovery or mainnet
activation. See [retained-state drill](DEPLOYMENT.md#private-two-program-retained-state-drill).
Separate intended-Linux-host acceptance now also passes four fresh validators
through eight one-at-a-time executable switches and seventeen one-shot
committed transactions. Original receipts, balances/sequences, contract state,
identities, checkpoint hashes and signing witnesses were retained. Eight outage
commits carried exactly three independently verified original signatures and
none from the stopped validator. The run used one isolated bounded temporary
unit; all fourteen existing services were unchanged. This closes that exact
compatible-pair rolling-drill gap, not arbitrary migrations, deployed-key
adoption, signer-safe backup restore, physical durability or mainnet activation.
See [rolling acceptance](VERIFICATION.md#isolated-linux-four-validator-rolling-upgrade-acceptance).
An optional immutable unsigned record reserves the original request/output
before export and supports explicit complete-file recovery after interruption;
it is not a financial retry journal or global duplicate fence.
An offline signed-envelope check additionally binds exact sender, nonce, gas,
fees and custody data to separately reviewed private inputs. It also refuses
Gas limits below the pinned London calldata admission cost before any new
attempt/nonce reservation. This is not an execution estimate or verified RH
fork policy; exact-minimum contract failures still retain unknown without retry
or fee release. See [intrinsic Gas admission](BRIDGE.md#pinned-london-intrinsic-gas-admission).
A separate offline sender account proof now validates inclusion, nonce equality,
execution-Gas budget fit and empty code hash relative to a caller-pinned header's
state root. Original private files are rechecked before returning only redacted
conditional facts. It does not authenticate source chain/finality/current state,
check pending transactions or authorize dispatch. Existing book/attempt state
is unchanged. See [account-proof limits](BRIDGE.md#conditional-sender-account-state-proof).
A separate simulation-only one-shot fetch now reads that proof against an
explicit historical block pin, compares all RPC metadata to the root-proven
account and rechecks provider references and original private files. The optional
HTTP adapter is narrowed to four read methods without default endpoints,
latest/pending fallback, retry, persistence, signing or broadcast. Matching
provider references still does not authenticate headers, ancestry, source fees
or current account availability. See [account fetch limits](BRIDGE.md#simulation-only-read-only-account-proof-fetch).
A simulation-only
single-dispatch record retains unknown before exposing bytes to a local fixture;
intact unknown records refuse re-dispatch after restart. It is not an
authenticated latest head, sender-wide nonce allocator, live source broadcaster
or finality/payment verifier, and has no retry or release transition.
A separate bounded shared simulation account journal now serializes consecutive
nonce reservations across exact reviewed envelopes and retains their aggregate
maximum execution-Gas cost. It persists ordered unknown handoffs, immutable
hash-linked history and independently pinned recovery without refunds/retries.
Local receipt reconciliation additionally binds complete supplied block material
to exact unknown envelope bytes, validates execution-Gas arithmetic and exact
successful custody effects, and retains one immutable success/revert observation.
Exact repeats do not write; conflicting observations stop without replacement.
Neither outcome authenticates finality or resolves payment, fees or nonce reuse.
An explicit bounded read-only fetch additionally locates the exact unknown
envelope, obtains complete raw material, matches every selected normalized log
and rechecks provider references and private inputs before returning an owned
snapshot. It does not poll, sign, dispatch, retain a review or resolve unknown;
explicit local retention remains separate. Provider consistency is not source
finality, code/state truth or a live financial relayer. See
[read-only fetch controls](BRIDGE.md#explicit-read-only-custody-receipt-fetching).
This fences the same book only, not other tools/journals/hosts; a supplied starting
nonce is not authenticated state and earlier unknown transactions may stall later
ones. Complete RH fees, enforced production account ownership, source finality,
safe signing and uncertain-broadcast reconciliation are still required. See
[shared-journal controls](BRIDGE.md#shared-simulation-sender-nonce-and-execution-gas-reservation).
Private daemon/compiled-custody checks do not
complete software upgrade/rollback, source-finality verification or financial
relaying. No live service, real asset or production gate is activated. See
[DEPLOYMENT.md](DEPLOYMENT.md#cooperating-process-node-workspace-lease) and
[BRIDGE.md](BRIDGE.md#persistent-offline-unsigned-request-record).

## Status definitions

- **Implemented and locally tested** means code exists and the recorded private checks exercised it with synthetic inputs. It does not mean production acceptance, independent audit or live operation.
- **Partial** means a relevant component exists, but the complete deliverable is missing or unverified.
- **Pending** means the usable deliverable has not been implemented or accepted. It may reuse existing core components.

The remaining plan groups work into six core workstreams and ten supporting modules. These are planning categories, not sixteen independent applications or a completion percentage. Some modules can share a service, interface or wallet core.

### Local-only completion scope

The selected operating plan is [single-host and multi-service](SINGLE_HOST.md),
with four separate validator identities/processes and a non-signing observer.
Cross-machine acceptance moves to a later distributed stage; it is not required
to continue this local candidate and is not relabeled as passed. A shared host
still has correlated control, resource and whole-machine failure risks.

The current development round is limited to code and local acceptance. No
external Mac observer, VPS connection, public cutover or real-asset operation is
authorized by these checks. The requested five deliverables remain partial:

| Deliverable | Local foundation verified | Remaining completion evidence or code |
| --- | --- | --- |
| Self-service installation and joining | Trusted-pin bundle fetch, offline inspection, private package staging, fresh observer/candidate setup and signer-safe local recovery | Accepted public configuration/pin delivery, reviewed participant installers and target-machine startup/update checks |
| Single-host node operation | Persistent five-process loopback join, catch-up, guarded candidate admission and process/quorum/whole-group restart checks; isolated intended-Linux-host exact-pair four-validator rolling upgrade/return with verified outage signatures and retained state | Deployed-key adoption, production installation/upgrade authority, reboot, physical durability and signing-safe backup restoration; external-host admission and routing become later distributed-stage gates |
| Staking and G lifecycle | Signed application settlement, claim, transfer, redemption, empty-pool preservation and disk recovery; unchanged synthetic 21-day boundary; hosted naturally settled G claim, transfer and proportional redemption | Sustained participant staking/reward cycles and real elapsed-period payout evidence; no accelerated public clock or invented funding |
| Explorer and wallets | Fixed-target bounded summary backfill with retained progress, history/query/recovery foundations, web/Chrome local cryptography, an opt-in address-only connection candidate and shared same-profile GOD/NFT/native attempt recovery across reload/browser restart | Public range acquisition/coverage, complete participant/NFT semantics, storage-loss and multi-device handling, safe resolution of non-included signed attempts, website transaction authorization/review, public connection activation, and physical target devices |
| Long-run operations | Finite first-fault read-only observation, bounded same-user Linux resource collector with portable policy/parser fault checks and foreign compilation only, bounded synthetic read checks, controlled local claim/transfer/NFT concurrency with committed lost-acknowledgment and observer-restart recovery, identical-native-request concurrency with actual pre-inclusion unknown/offline-copy recovery and zero-G refusal, private attempt/history audit-copy checks, stopped disposable observer cold-copy retention/catch-up with zero signing progress, and bounded health/host helpers | Native Linux collector/runtime acceptance, full simultaneous intended-host wallet/native/write load and sustained stress/resource/storage measurements, reviewed external alert delivery, authenticated independently retained backups, validator fencing/signer-safe restore and safe-upgrade drills |

## Implemented and locally tested capabilities

| Capability | Recorded implementation | Remaining boundary |
| --- | --- | --- |
| God EVM execution | Signed legacy, access-list and dynamic-fee transactions, contract deployment and storage, fee accounting and business rollback; restricted RPC calls and gas estimation. | Actual wallet-client acceptance, full RPC compatibility and production gas policy remain incomplete. |
| Accounts and authentication | Canonical native `god1…` addresses, compatible EVM identities, ownership checks, shared ordered sequences, bundle-bound encrypted disposable-wallet recovery and same-account backup re-encryption. | Production recovery, signing-key rotation, HD accounts and production-wallet acceptance remain incomplete. Password changes do not revoke old backups. Format conversion is not bridging. |
| God SDK staking and penalties | Authenticated validator creation, delegation, unbonding, signed reactivation, downtime handling and verified duplicate-vote quarantine. | Other misconduct classes, production recovery and governance remain incomplete. |
| GodCometBFT local consensus | Four persistent loopback validators, contract propagation and recovery; independent identity initialization and a non-signing observer tested on the same host. | Cross-host operation, independent human control and adversarial-network reliability are not established. |
| Committed state queries | Latest-committed network, account, G, individual delegation and bounded pending-unbonding views; restricted synthetic HTTP/Ethereum RPC and indexed transaction receipts. | No historical application queries, light-client proofs, full explorer API or accepted wallet product. G fields exclude locks and are not total G. Unbonding timestamps/absence are not payout proofs. |
| G rewards and GOD redemption | Commit-based contribution accounting, bounded prototype G issuance, daily settlement, transfers and locks, and voluntary pool-based redemption. | Production emission approval and scalable settlement remain required. No guaranteed return is implied. |
| Bridge ledger and custody | Synthetic one-for-one accounting, quorum approvals, replay protection, segregated withdrawals, delayed authorization, cancellation rules and opt-in authenticated node routes; integrated application return/unbonding recovery and intended-host four-process receipt/catch-up checks. | No production backing, independent source verification, live signer operation or financial relayer. Successful delayed payment is a synthetic application check, not real source payment or an elapsed-delay four-node transfer. |
| Read-only RH evidence interfaces | Private configuration, separate pre-custody token inspection, offline source-material integrity, explicit known-compiler local runtime reproduction and exact retained-runtime comparison, full-binding compatibility probes, deposit and resolution observations, simulation journals, receipt-set checks, proof preparation and retention, and combined task evidence review. | Material integrity alone is not compiler execution; local reproduction is not toolchain authentication or token-permission approval. Matching provider data and header commitments are not authenticated finality or contract/native-state proofs. Original source and authenticated state remain required. Results remain unsigned; real assets remain disabled. |
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
The offline CLI also checks physical page/freelist invariants in a separately
supervised process and verifies a selected image against an independently
retained exact file checksum. Valid stale images are refused against a current
approved checkpoint. Neither an unpinned audit nor a checksum computed from the
backup itself establishes freshness; checkpoint custody and completeness remain
operator gates. Damaged pages, worker panic/timeout and malformed reports fail
closed without repair or RPC. This is not power-loss certification, automatic
restore, validator signing-state recovery or a public gateway cutover.
Isolated offline initialization/audit/copy and refusal cases also pass on the
authorized Linux pilot host as the existing non-root service account, with
network access denied and current consensus data inaccessible. Existing public
processes remain unchanged; this is not native-gateway HTTP/cutover acceptance.
This is not yet a public migration: the deployed pilot's earlier process map,
complete legacy-attempt inventory, sustained-load/storage sizing and safe
intended-host cutover/restore remain separate gates. No chain retry, economy,
waiting period or public activation is authorized by local recovery checks.

The operator CLI also offers a keyless, offline full-bundle preview before a
fresh synthetic join. It displays exact chain/policy identities, bounded
participant/topology counts, block/transaction limits and prototype economic
quantities without exposing participant addresses, endpoints, node keys or
paths. Private tests cover bundle-only storage, full-digest binding, malformed
profiles, unsafe files, precise large integers and canonical IP aliases. A
preview neither initializes a node nor proves independent ownership, routing,
geography, production approval or an installed cross-host network.

The requested rollout starts with founder-operated validation while keeping
participant onboarding open. The synthetic `join-observer` candidate now allows
a fresh local non-signing identity to join the original pinned genesis/peers
without advance launch-profile registration. It preserves keys/progress and
rejects role escalation, key reuse, unsafe RPC and existing state. Private
persistent consensus checks cover joining after a committed transfer, catch-up,
matching block/balance and restart without votes. The launch bundle and economic
rules are unchanged. A fresh `init --role observer|candidate` now combines local
identity creation and pinned joining. The distinct candidate registration
request binds node-key possession, canonical owner and exact network/amounts;
the separate encrypted terminal wallet reviews and signs it without exporting
an account key. A candidate signs only after locally committed ownership and
the exact-height active set authorize it, preserving the two-block delay and
original FilePV/storage binding. Private five-process consensus checks cover
post-genesis catch-up, authenticated registration, actual signed contribution/G,
restart, changed-owner refusal and self-undelegation followed by non-signing
replication. These are local synthetic candidates, not deployed registration
tools or production acceptance. The public six-action gateway remains closed to
registration. An explicit new-bundle opt-in now verifies consensus-key possession
on-chain before standard validator registration; candidate tools refuse legacy
unprotected bundles without altering their observer paths. Cross-host abuse
acceptance, circulating stake funding, accepted public configuration delivery, installers, secured
public peering and intended-host acceptance remain incomplete. Founder-operated
startup does not establish decentralized control or authorize production assets.

The private package tool now adds trusted-digest, fixed-inventory unpacking into
a new owner-only staging directory and a separate read-only directory recheck.
It never overwrites existing state, installs services, executes archive content,
creates keys or starts a node. Failed partial directories remain unusable pending
manual review. This is an installation-preparation component, not a public
installer, authenticated join-bundle delivery or cross-host admission. Linux
package contents and macOS staging are checked locally; intended participant-
host runtime acceptance and Windows private-disk ACL support remain separate.

The local onboarding candidate now downloads one explicitly selected synthetic
bundle with an independently reviewed complete digest, verifies immutable bytes
and full policy/profile/genesis consistency, then creates only a new private
configuration file. It never initializes or starts a node. CLI and negative
transport/storage fixtures pass, and an isolated five-process test covers
download-to-observer initialization, committed transfer replay and retained
identity restart without votes. Public trusted-pin delivery, publisher identity,
external reachability and participant-host acceptance are not established.

Finite read-only observation now emits bounded redacted sample/final reports,
requires an explicit duration budget and stops on the first health, continuity,
stall or output failure without retrying or restarting. Actual-time local
loopback reads and deterministic fault-policy fixtures pass; deterministic time
is test input, not sustained uptime evidence. External alerts, long runs and
native-host resource/backup/upgrade drills remain required. The latest work is
local-only: it does not connect to or change the existing public pilot, publish
source, release binaries, enable website-provider signing or activate assets.

The local history candidate now adds explicit fixed-target backfill over up to
32 bounded batches, with target-hash/checkpoint reconciliation, inter-batch
pauses, durable complete-block progress and manual continuation after a budget
exit. Queries remain unavailable after an interrupted refresh until successful
reconciliation. Reaching the target means declared-range summary coverage, not
complete account/NFT semantics, authenticated finality or public catch-up.
Private pagination, rejection, cancellation, abrupt-exit and actual persistent
consensus/observer-restart checks pass without changing the public worker.

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
| 1. Node installation and management | Partial | Synthetic identity/assembly, launch and self-service join, explicit trusted-pin configuration fetching, fresh observer/candidate initialization, offline registration signing and exact-height guarded candidate admission pass private checks. Newly assembled synthetic bundles can explicitly require chain-enforced consensus-key possession; candidate tools refuse unprotected bundles without migrating legacy data. Private Linux packaging, safe non-overwriting staging, read-only staging checks and recovery tools exist. Public candidate-path activation, cross-host abuse acceptance, stake funding, target-host acceptance, accepted public configuration/pin delivery, installers, upgrade controls and operator recovery remain required. |
| 2. RPC services | Partial | The public pilot has a keyless read-only HTTPS gateway and a separate plain-transfer-only submission boundary. Actual browser transfer acceptance passed. Full compatible RPC coverage, sustained public-service acceptance, proofs and complete explorer interfaces remain required. |
| 3. GOD Chain wallet | Partial | The connected browser alpha supports encrypted disposable accounts, recovery, funding, plain transfers, a restricted test NFT collection and six reviewed native staking/G actions. HTTPS transfer and NFT acceptance cover the existing submission flows. Ownership cards, same-key backup re-encryption, public-address copying, separate known-native-ID recovery and locked-wallet unbonding progress queries are deployed. Full delayed withdrawal completion, positive settled-G workflows, production recovery, HD accounts, signing-key rotation and independent security review remain required. |
| 4. Browser wallet extension | Partial | A downloadable Manifest V3 test alpha packages the shared connected wallet with exact network pins, a fixed reviewed identity, toolbar entry and constrained transport. Exact-origin public activation and actual HTTPS creation/recovery, claim, committed GOD transfer and restricted NFT mint/send acceptance passed; the English installer provides ZIP/asset hashes and the expected ID. Version 0.3.9 retains six restricted native actions and consolidated read-only state queries, and adds locked-wallet registration pages bounded to eight current records with same-commit cursors. Fresh reads are paced, quota errors are explicit, and staking/unbonding guards match the existing chain policy before signing. Local desktop/mobile and actual unpacked Chrome fixtures cover all six actions, rejected/stale/unknown results and recovery. Physical browser/device acceptance, complete exit and settled-G workflows, website connection permissions, external provider signing, independent security review and separately authorized Store publication remain pending. See EXTENSION.md (separate pilot guide, not included in this core snapshot). |
| 5. Blockchain explorer | Partial | A read-only test explorer displays paginated committed blocks and native/EVM details with actual execution outcomes. The public portal provides checksum-checked, identity-matched account lookup/share links, manual five-block sender/recipient activity scans, observer diagnostics and a redacted local report. A separate opt-in keyless address-index CLI passes complete-block pagination, atomic checkpoints, anchored address pages, process interruption and persistent local-node recovery checks; bounded Linux acceptance read ten committed pilot blocks without changing services. Its production source and English usage guide are in the reviewed core scope, not a public route, complete backfill or NFT index. Full public indexing, a complete historical validator index, proofs and sustained acceptance remain required. |
| 6. Staking and rewards interface | Partial | Connected web/Chrome preparation, six-action explicit signing, restricted native admission, known-hash recovery and bounded committed unbonding progress plus locked single-operator validator lookup are deployed. The browser signatures pass independent pinned SDK byte/signature checks. The portal's separate delegation/pool views remain watch-only. Public disposable-account delegation and start-unbonding produced matching committed receipts and a real pending entry. Locked web/Chrome recovery passed; the progress lookup does not imply payout. Full delayed withdrawal completion, positive settled-G claim/transfer/redemption, fuller validator discovery and sustained public acceptance remain required. This can be part of the wallet or APP. |
| 7. Bridge website | Pending | RH to GOD Chain and return-transfer workflows, progress, limits and explicit failure or cancellation states. Live operations remain gated. |
| 8. Bridge relay and signer tools | Partial | Independently reviewed approvals, safe submission, recovery, reconciliation and duplicate prevention remain required. Local tools now check pinned private requests, create exclusive unsigned custody call files and recheck exact retained bytes; they neither sign nor submit. Existing read-only simulation helpers and offline construction are not an operating financial service. |
| 9. ETERNAL KINGDOM integration | Pending for GOD Chain | A replaceable chain adapter, wallet connection, GOD and G operations, and submission and retrieval of client-encrypted faith content. This status does not describe unrelated existing APP features. |
| 10. Monitoring and operator documentation | Partial | Synthetic deployment/recovery guides, bundle-pinned health, finite first-fault observation, short service checks, explicit host budget snapshots, redacted diagnostics and a Linux supervision template exist. Target-host verification, external alerting, sustained resource measurements and reviewed incident recovery remain required. |

The encrypted terminal wallet, local offline browser build, connected public test wallet and downloadable Chrome test alpha share the fixed encrypted format. The public alpha connects creation, backup/recovery, fresh balance/sequence queries, automatic test funding, reviewed plain GOD and restricted native signing, one-attempt submission and known-hash confirmation. The restricted collection also supports synthetic NFT minting and owner sends through a separate gateway. Signing and admission are not payment: only matching committed execution establishes the displayed success. Faucet claims need no registration or human approval; FAUCET.md (separate pilot guide, not included in this core snapshot) specifies durable idempotency, pilot limits and pool depletion. Complete native exit/G workflows, general NFT tools, external website signing and production-wallet acceptance remain unfinished. Existing compatible wallets can still be evaluated against required interfaces. Public query services and indexing have operating costs; decentralized validation does not make them cost-free.

The local version-three Chrome connection candidate adds explicit, revocable
public-address sharing to one reviewed test website, with browser-created
approval tabs and no external signing or arbitrary RPC. Actual isolated
Chromium covers encrypted recovery, consent, forbidden origins/frames,
revocation, worker termination and document navigation. It is not the current
public ZIP or a deployed website provider. Public scope/activation review,
durable unknown-attempt preservation before any website signature, independent
review and physical browser/device acceptance remain required.

The private connection/recovery candidate now has retained-source reproduction
and a separate fifteen-file deterministic review archive. Exact source/build/
archive pins and fixed inventory must match before fresh private unpacking.
Actual isolated browser fixtures load that package for website consent and
GOD/NFT/native recovery, using real saved backups/recovery files and directly
observed normal owned-process exits. Recovery never signs or resubmits.
Adapter-fault injection is separate from the untouched package. This is a local
release-integrity foundation, not approved distribution, live-service/device
acceptance, an independent audit or production readiness. The public 0.3.9 alpha,
source publication allowlist and real-asset gates remain unchanged.

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
| Node participation | Four-validator synthetic consensus, private assembly/launch-join checks and self-service late observer catch-up/restart | Intended-host installs, validated configuration delivery, reviewed external peering, post-launch validator admission, signer identity/progress preservation and safe upgrades; founder bootstrap must be disclosed and geography must reflect actual infrastructure |
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

The next acceptance milestone is a synthetic test on the intended single host, with secured observer RPC, process isolation, self-service late-node catch-up, disposable-wallet checks, sustained daily settlement, test funding and aggregate measured resource use. Founder-operated bootstrap does not require inventing independent participants or renting multiple validator hosts. Independent-control and cross-host evidence remain required for a later decentralized-validation claim, not a substitute for single-host safety. Short local tests do not satisfy host/reboot/durability gates. Public deployment still requires separate authorization; mainnet and real assets remain disabled.

### Production transition gates

Testnet iteration does not authorize changing its asset mode or turning its
balances into real GOD. A proposed production release needs a separately
reviewed candidate and explicit final activation approval. These gates are open,
not completed by version 0.3.9 or a passing developer regression suite.

| Gate | Required evidence before production activation |
| --- | --- |
| Participant workflows | Positive daily-settled G claim/transfer/redemption, actual 21-day unbonding payout and failure/recovery checks on persistent consensus; no clock fast-forwarding or fabricated settlement |
| Operator control and open participation | Intended-host installation, secured peering/catch-up, safe signer restart/upgrade and validated self-service admission; founder-controlled bootstrap and its correlated failure/control risks must be explicitly reviewed and disclosed. Independent key/control custody is required before claiming decentralized validation; reported locations must match real infrastructure |
| Single-host operating scope | All services measured together, separate private data/ports/permissions, process/quorum and host-reboot recovery, explicit no-failover limitation and reviewed independently retained backup/incident evidence; no requirement to invent extra validator machines |
| Capacity and recovery | Sustained settlement and public traffic, storage-growth/retention budgets, abuse and depletion recovery, alerts and restore drills that never roll back signing progress |
| Economics and initial funding | Explicitly approved production emissions, gas/fee and authority rules, source-backed validator/Gas funding and exact fixed-GOD conservation; test parameters are not approval |
| Bridge and reserve | Independently authenticated source identity/ancestry/finality, verified custody/contract permissions, one-for-one backing, delayed returns, replay/reorg rejection and interrupted-operation reconciliation |
| Release and control | Reviewed immutable candidate, migrations, independent/adversarial review as available, documented unresolved risks and upgrade/rollback authority; no claim that upstream maturity transfers an audit |
| Activation decision | Review all required evidence and remaining risks with the owner, approve the exact production configuration privately, then explicitly authorize activation; keep synthetic and real-asset environments separate |

No mainnet date, guaranteed return, public safety certification or test-to-mainnet
asset conversion is selected here. Development can continue on open gates without
buying infrastructure, moving real assets or weakening the existing test rules.

The first bridge/source-trust workstream now includes a bounded offline
parent-finality verifier rooted in explicit caller-retained trust. It checks
single-period Deneb/Electra committee signatures and finalized execution-header
hash inclusion. A separate bounded adjacent-period API proves the next committee
against the same original checkpoint and verifies next-period finality without
relaxing its age/fork policy or accepting an old-committee signature. Neither
API authenticates its own checkpoint/fork/clock inputs or binds an RH batch/
execution result. A separate private tracker now retains up to 16 conditional
updates, replays all cryptographic checks on reopen, refuses conflicting heads
and handles uncertain writes with separately retained old/planned pins. It does
not extend the original anchor's lifetime. Continuing committee/fork updates,
independent monotonic pin retention and production recovery, actual source
identity and token/custody permissions remain release requirements.
An offline operator command now reads separate pinned private trust/proof files
and an explicit reviewed clock slot, enforces closed bounded schemas and calls
the existing conditional verifiers. It performs no network, checkpoint selection,
head persistence, source execution or admission. Pinned integrity is not input
provenance; independent trust and RH binding gates remain unchanged.
This is local conditional cryptographic progress, not production finality or
activation. See
[PARENT_FINALITY.md](PARENT_FINALITY.md).

## Reuse and publication requirements

The first three production-preparation workstreams now also have local
[launch-review foundations](LAUNCH_REVIEW.md): exact parent-execution-root
contract account/code/storage proofs, exact token source/config/runtime binding,
and private pinned economics/funding arithmetic against current prototype
constants. They reject malformed/disconnected evidence and inconsistent funding
without upgrading trial values to approved policy. Actual RH deployment,
batch/execution/finality, source/custody permissions, backed funds and owner
approval remain open. This does not change the single-host operating scope,
claim independent control, apply a new configuration or activate real assets.

The separate [constructor compatibility milestone](TOKEN_CREATION.md) now
supports bounded exact CREATE/CREATE2 replay of unlinked Cancun creation code
with constructor-dependent runtime. Actual compiled synthetic assets exercise
custody deposit and delayed return. Original RH creation/state provenance,
source admission and production activation remain separate integration work.
The offline operator command now integrates explicit source/config/replay pins,
one actual local compile and one bounded constructor replay with unchanged-file
checks and a redacted result. It does not expose a bridge signing or submission
path, change the configured token or approve real assets.

The offline `check-rh-token-state` command now connects the conditional token
account/code/storage verifier to pinned private operator files and separate
reviewed execution block inputs. It checks unchanged files before returning a
redacted result, keeps source compiler execution and independent finality
unverified, and does not invent permission or storage-layout conclusions.
Its code/account-only mode does not prove supply or custody backing. Actual
deployment evidence, authenticated RH state/settlement and financial admission
remain open; no signing, broadcast or activation path is added.

The separate read-only acquisition layer now connects a pinned private config,
source capsule and explicit header/slot request to the configured endpoint.
It binds one hash-pinned code/proof read to exact before/after chain and block
checks, verifies actual account/storage paths and rechecks private files before
a redacted result. There is no public endpoint, automatic head selection,
fallback, persistence, signer or custody writer. Production-mode HTTPS supports
diagnosis only. Actual RH source identity, authenticated execution/settlement,
permissions and source-backed financial admission still need integration and
review; conditional provider consistency does not finish those gates.
See [bounded token-state acquisition](LAUNCH_REVIEW.md#explicit-read-only-token-state-acquisition).

Prefer mature open-source components and standard interfaces where they fit God EVM + God SDK + GodCometBFT. Before adoption, review the exact license, pinned version, maintenance and compatibility. Preserve required copyright, license and attribution. Rebranding or reusing code does not transfer another project's audit or production assurances to GOD Chain.

Keep populated addresses, contract settings, operational endpoints, credentials, keys, private tests, synthetic fixtures, runtime data and binaries out of the public repository and its reachable history. The existing whitepaper remains preserved; this plan does not silently revise its economic model.

Wallet signing keys must remain under user control on the user's device and must not be collected by the APP or query services. Private prayer, confession and praise content must be encrypted before submission and must not enter public chat, telemetry, server logs or explorer plaintext views. Encryption does not make public chain data erasable.

GitHub publication is separate from website deployment, network launch and real-money operation. Each requires its own explicit authorization and completed release gates. There is no independent audit, mainnet safety certification, guaranteed yield, fixed launch date or assurance that a distributed launch is already decentralized.
