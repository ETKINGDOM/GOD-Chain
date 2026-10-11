# GOD Chain core verification

This record separates exact public-core source checks from acceptance of the
separately deployed synthetic pilot. It is not an independent audit, source-
finality proof or mainnet safety certification. Private tests, browser/extension
source, other pilot gateways and generated operational inputs are not
distributed in this exact source snapshot. Publication activates no service or
real assets. God EVM + God SDK + GodCometBFT pins remain unchanged.

## Current synthetic participant acceptance

### Local custody candidate acceptance

The custody/account and conditional parent-finality candidate passed 35 source-bound private
local acceptance phases with no skipped checks. Independently replayed evidence
matched source inventories before/after, phase-log digests, retained compiler,
module/overlay inputs and compiled synthetic-custody artifacts. The related
race run passed 370 named tests (2,179 including subtests); the compiled-custody/parent
race run passed 117 named tests (938 including subtests). These counts overlap
earlier checks and are not additive safety scores or an independent audit.

Eleven finite fuzz checks exercised bounded requests, records, signed envelopes,
attempts, nonce/review, receipt metadata, intrinsic Gas, account proofs/RPC and
parent proofs. Ten requested five-second windows and one fixed 30,000-execution
Gas budget passed; Go workers reported 30,007 executions for the fixed budget.
An earlier timed Gas fuzz run ended with `context deadline exceeded` and remains
a retained failure. An identical isolated timed run passed before the fresh full
fixed-budget acceptance. No Gas assertion, rule or dependency was relaxed.
Static analysis, native command build, diagnostic Linux amd64/arm64 and Windows
amd64 compilation, Windows test compilation and disabled-command checks passed.
Foreign binaries were not run. Diagnostic status retained `nodeReady: false`,
`realAssets: false` and `rhSourceFinalityImplemented: false`.

The shared simulation book reserves consecutive nonce strings and aggregate
execution-Gas cost for at most 64 exact envelopes. Race/fault/disk checks cover
competing reservations and takes, exact-budget/capacity boundaries, private
input changes, errors/panics with old/new save outcomes, post-save corruption,
independent latest-head pins and unchanged reservations. Codec acceptance covers
64 retained observations in 193 bounded monotonic states and unchanged old
no-review v1 bytes. Original nonces and costs cannot be overwritten or repriced.
Disposable child-process termination and latest-pinned disk reopen retain
unknown without another signed-byte handoff. These are local process checks,
not power-loss, hostile-owner rollback or production nonce-state acceptance.

Local receipt review covers all four custody actions with success and revert,
exact signed bytes, canonical complete roots, selected Gas/base-fee arithmetic,
forged events even under self-consistent supplied roots, malformed selected and
unselected material, immutable observation conflicts and concurrent idempotence.
Retained compiled custody fixtures assert actual synthetic signed execution,
nonce/EOA checks, sender Gas debit and execution-generated logs before review.
All observations remain dispatch unknown, never payment acknowledgments,
refunds, fee releases or retry permissions.

Explicit fetch covers reserved/unbound/closed refusal, provider errors and
panics before/after material reads, cancellation and an actual short deadline,
all selected normalized logs/global indexes, changed private inputs/head,
detached borrowed buffers, serialized concurrent reads and ordinary checkpoint
progress. It changes no stored bytes or latest pin. Separate explicit
reconciliation can retain its owned material after reopen. Temporary loopback
HTTP fixtures verify exactly five existing read methods and fifteen requests per
successful one-shot fetch; pending receipts, missing raw methods, wrong chain,
aliased fields, late receipt changes, inconsistent raw receipts and postflight
forks yield no partial result or write. Actual RH endpoints and method support
were not tested.

The source includes earlier offline unsigned call/file/record tools, exact signed
envelope review, pre-custody token inspection and bounded private source-material
integrity/local recompilation. Those local checks do not authenticate source
code provenance, compiler/host trust, issuer permissions, authenticated live account state,
full source-fork/RH/L1 fees or native withdrawal truth. The latest scoped custody
run does not certify every unrelated browser, deployment or token-tool path.
Production account ownership, signing/broadcast recovery, authentic RH
ancestry/finality, backing and explicit activation remain separate release gates.
Private fixtures, artifacts and evidence are not distributed; public build
commands do not rerun these tests. See [BRIDGE.md](BRIDGE.md).

### Conditional account and parent-proof verification

The exact signed custody calldata now passes a London intrinsic-Gas baseline
before new attempt/book reservations. Compiled synthetic execution distinguishes
below-minimum rejection from exact-minimum contract out-of-Gas, paid execution
fees and consumed nonce. This baseline is not a verified RH fork schedule,
contract execution estimate or complete source/L1 fee budget.

Offline sender account inclusion checks an ordered canonical secure-trie proof
against an explicit caller-pinned full header. Tests cover precise large nonces,
balance equality/insufficiency, empty-code-only admission, 65-node paths,
malformed/reordered/unused material, detached ownership, private-input changes,
redaction and cancellation. Compiled state proofs show that a later transaction
can consume the nonce while an old proof still matches its old pin. The explicit
simulation fetch and four-method HTTP adapter check pre/post provider references
and exact proof-derived RPC metadata without persistence, pending/latest
fallback, retries or production reads. Self-consistent fabricated headers still
pass conditional checks; no authentic account freshness is claimed.

The parent verifier checks one explicitly allowed Deneb/Electra fork and one
committee period under supplied checkpoint/genesis/fork/clock trust. Private
fastssz fixtures independently build roots/branches; blst signs/verifies fixtures
instead of the production gnark verifier. Both supported fork layouts exercise
341-position refusal and 342/343/511/512 acceptance, positional multiplicity,
canonical subgroup checks, non-subgroup/infinity refusal, trust/domain/branch/
slot/header tampering, complete canonical execution-hash binding, detachment,
cancellation, concurrency and bounded fuzzing. These are synthetic cryptographic
containers, not real parent/RH execution or independent operators.

No independently authenticated checkpoint, fork/clock policy, transition,
RH batch/execution binding or token/custody backing is supplied. Successful
parent checks leave source-chain identity, RH finality, execution replay,
signing, broadcast and real-asset authority false. Windows builds do not execute
the CGO-only independent fixture tests. See [PARENT_FINALITY.md](PARENT_FINALITY.md).

### Cooperating workspace and finite read tools

The included node-start lease refuses duplicate cooperating processes using the
same private workspace before identity/database/listener startup. It preserves
original signing progress and is not remote-signer fencing, backup restore or
two-version safe-upgrade certification. Finite `read-load` is an explicit
bounded liveness/network read tool, not wallet-write capacity acceptance.
`host-observe` reads fixed same-user Linux procfs/filesystem aggregates and never
reads keys, argv or environment, signals processes, restarts services or opens a
network connection. Native Linux runtime/resource acceptance remains incomplete;
foreign compilation and short samples do not prove server capacity. These tools
are opt-in and source publication runs none of their configured operational
reads. Original protocol/economic pins and activation gates are unchanged.

### Local-only wallet recovery candidate

The opt-in Chrome 0.4.0 candidate is not the published 0.3.9 alpha. Its
address-connection worker shares only explicitly approved public accounts and
cannot sign, submit, access vaults or read the transaction journal. External
website signing and arbitrary RPC remain disabled. Browser source and private
fixtures are not included in this exact core publication.

GOD transfers, restricted NFT mint/send and six native staking/G operations
share one network/account lock and unresolved record in the same browser
profile. The record contains the original canonical public review, approval
digest and verified TX ID, never a password, private key, vault or signed wire.
An unsigned reservation must commit before crypto; the signed hash must commit
before one submission. Missing or mismatched results preserve the guard.
Only matching committed inclusion resolves a signed attempt. Cancellation is
restricted to an explicitly checked still-unsigned reservation.

Thirty-seven private state-machine checks passed. Original signed unknown
recovery-file restoration covers all nine operation paths, exact account/network
pins, empty-store admission, unchanged existing records, contention, storage
faults and cancellation. It restores only a previously saved public guard,
never signs or submits, and cannot reconstruct later missing history.
Actual isolated Chromium
fixtures exercised local wallet crypto, IndexedDB, Web Locks, mixed-operation
contention, reload and browser-process restart, exact confirmation and refused
storage writes. The mixed fixture submitted two NFT and six native operations
to intercepted synthetic services only. Verified same-account backup
re-encryption retained the coordinator; expired signed redemption metadata
remained recoverable without enabling expired new signatures. Gateway deferral
did not release a signed unknown hash. Default web and older-profile browser
regressions also passed; those clients do not open the candidate database.

One plain GOD recovery-file browser run passed download, empty-profile import,
another browser restart and exact committed reconciliation, with no recovery
worker or request. Separate import-only Chromium fixtures restore an original
signed NFT mint and expired G redemption into fresh disposable profiles with
the same encrypted wallet and pins, retaining the full review/TX ID through
another browser restart. Changed bundle pins, duplicate fields and oversized
files leave the empty journal unchanged without a worker or request. Exact
reconciliation resolves without resubmission. These fixtures construct input
from the original canonical record, not a successful browser export, and cannot
accept the download gate. The ordinary full UI command still requires actual
downloads and has no import-only fallback.
The recovery-file browser gate is not reliably accepted: three earlier reopened
plain-GOD test Chromium runs and the full NFT-mint export fixture closed during
download on macOS arm64; a separate empty-JSON
probe without wallet code also encountered a failure. This does not determine
the browser defect's cause or establish a production fix, even after a later
complete pass. No test-browser pin,
security permission or chain dependency was changed to bypass the gate.

Private wallet-free controls reproduce a restart download failure with both
ordinary managed downloads and an otherwise same-launch CDP-managed browser.
Two native-download controls save exact real files across three process
launches with normal exit and retained history, using the unchanged browser.
This is consistent with an upstream browser download-history regression, not
proof of an installed fix or general stability. A separate explicit native
wallet fixture passed actual NFT-mint and expired-G-redemption exports,
malformed-file refusals, empty-profile import, further process restart and
exact reconciliation without resubmission. It restores only ordinary page-focus
emulation and requires normal owned-process exit. Failed teardown rounds remain
failures; plain-GOD native teardown and repeated full-target acceptance remain
under validation. The original managed gate is not replaced or automatically
retried with another mode. These private diagnostics change no application
code, public package, pin, permission, chain or live service.

A distinct explicit crash fixture passes plain GOD recovery after terminating
only the owned temporary browser process with `SIGKILL`. Actual saved files
restore the original guard into empty storage without a signer/request and
survive another process interruption; only exact committed reconciliation
resolves it, with no resubmission. Evidence explicitly refuses a graceful-close
claim. This is not a fallback for failed normal shutdown or managed downloads,
physical power-loss proof, storage-erasure recovery or production acceptance.
A separate mixed crash fixture passes actual NFT-mint and expired-G-redemption
exports, malformed-file refusal, empty-profile import, further interrupted
restart and exact committed reconciliation, without a recovery worker/request
or resubmission. Both crash fixtures are local macOS arm64 checks only.
Three private launch guards reject non-fixture/private-path and symlink misuse
without starting a browser or editing linked targets.

A separate explicit fixed-browser comparison uses official Chrome for Testing
155.0.8059.39 on local macOS arm64. Official HTTPS metadata/object integrity and
retained archive/source/full-tree pins are verified before execution; the exact
source tag includes the upstream download-history fix. This ad-hoc-signed test
artifact is not accepted Developer ID/notarization or publisher-signature
authentication. Two complete managed-download comparisons in fresh disposable
fixtures each passed three retained-profile launches and full GOD/NFT/native
actual-file recovery, empty-profile refusal/import, further process relaunch
and exact reconciliation,
without recovery signing, requests or resubmission. Ordinary Playwright context
teardown completed, not the separate native helper's direct zero-exit-status
assertion. Both full-target commands exited successfully; eighty-one associated
unit/build/packaging/publication-policy checks passed. Five further private
guards refuse
unreviewed selection, changed self-updated pins, signature claims, linked/shared
paths and changed or escaping resources. Existing bundled-runtime failures
remain failures. No default browser/dependency pin, wallet source, ZIP,
permission, chain or live service changed; distribution and physical-device/
independent-review gates remain closed.

A stricter separate target passed twice in fresh fixtures, each with three
retained-history managed-download launches and a full GOD/NFT/native round
while directly observing zero-status,
unsignaled exit of only its owned test-browser main process after every close.
Forced cleanup fails this target. All actual-file recovery and no-resubmission
assertions remain required. Both full-target commands exited successfully;
this is not physical power-loss or cross-device acceptance.

A separate private retained-input reproduction check rebuilds all thirteen
candidate assets in memory and compares them with the reviewed build pins.
Its private source record binds compiler-loaded source, copied UI assets,
package descriptors/lockfile, licenses, build/review scripts and the installed
compiler implementation/binary; no profile values or endpoints are included.
Six named tests cover actual reproduction, a separate matching rebuild,
self-updated asset hashes, altered/incomplete input records, unsafe record
paths, wrong pins and unsupported overrides. Capturing a checksum is not
approval; checking requires the retained record pin. This is not a hermetic or
authenticated toolchain, whole-source behavior review or independent audit.
No ZIP, public package, permission or live service changes. Version-three
distribution remains refused; publication and real-asset flags remain false.

A separate private candidate bundle adds a fixed fifteen-file stored ZIP:
thirteen reproduced runtime assets, an English private-review warning and an
asset checksum list. Full independently retained profile/build/source/archive
pins, fixed metadata and exact contents are checked before fresh owner-only
unpacking. Private inputs, reports, fixtures, keys and browser data are excluded.
Compiled runtime configuration is not an address-free public-source snapshot.
Eight archive/staging/argument guards and the ninety-five-check combined local
regression pass. This tool does not broaden this core snapshot's exact scope;
candidate assets and private tooling are not included here.

Explicit isolated reviewed-browser fixtures load the actual checked package for
public-address consent and GOD/NFT/native recovery. Actual saved encrypted
backups and original signed-attempt files restore the same TX IDs/reviews into
empty profiles and survive further restarts without recovery signing, requests
or resubmission. Bad imports and mismatched results remain blocked. Each round
has one initial GOD, two NFT and six native submissions to intercepted services.
Every owned main process must exit normally with zero status and no signal;
package/source/archive pins are rechecked before acceptance. Adapter-fault
injection is a separately identified copy, not an edit to the participant package.
This is local integrity/recovery evidence, not physical-device/live-service,
authenticated publisher, audit or mainnet acceptance. Public 0.3.9 and the
version-three public packaging refusal remain unchanged.

These checks are not live-service or physical-device acceptance, power-loss
durability, multi-device coordination, comprehensive storage-erasure recovery, authenticated
receipts or independent audit. Native result replies do not independently
prove account number/nonce; failed NFT replies do not expose successful
movement fields. A non-included signed hash can remain blocked indefinitely.
The distribution command still refuses this candidate. Source/distribution
review, required-browser/device recovery acceptance, comprehensive storage-loss handling
and physical target devices remain release gates.
This upload changes no service, public ZIP, signing key or real asset.

### Self-service validator-candidate workflow

New synthetic bundles can explicitly opt into chain-enforced consensus-key
possession with `testnet assemble --require-validator-proof`. The bounded
versioned Ed25519 proof binds the immutable runtime, operator, key, stake,
minimum, commission and remaining description fields inside the standard
staking message. Both authenticated admission and the staking handler verify
it before writes. Offline preparation retains a separate full-request proof,
and encrypted wallet review displays the enforced policy. Candidates refuse
legacy bundles; their observers and runtime serialization remain unchanged.
The bridge/custody runtime is outside this synthetic opt-in acceptance.

Private macOS arm64 race checks cover proof encoding and owner/key/runtime/terms
tampering, missing proof despite a freshly re-signed offline request, exact
wallet approval and legacy refusal without writes. Direct block execution
bypasses the mempool: invalid registrations change no affected account balance,
sequence or stake; the valid proof registers once with the exact stake/fee.
Synthetic commit vote flags in that application check are not signature proof.
Separate persistent five-process consensus proves actual signatures/G,
restart, immutable policy refusal and non-signing exit. The possession unit and
five-process lifecycle also passed on isolated Linux amd64 with a checksum-
verified executable. Legacy execution/recovery/signer-conflict regressions and
vet passed. No deployed network was upgraded by these private checks.

Fresh observer/candidate initialization pins the full bundle before creating
private keys/configuration and never starts or overwrites a node. A candidate's
guarded FilePV requires locally committed matching operator ownership and the
actual signing set for the exact height, preserving the two-block delay and
original anti-double-sign progress. Its expected owner is also storage-bound.
Local role edits and unsigned remote status cannot grant signing authority.

Registration preparation produces a new private node-proof-bound request; the
separate offline account signer requires the exact intended wallet. The encrypted
terminal variant shows the consensus public key, network/genesis, self-stake,
minimum stake, commission and financial metadata before explicit approval. It
signs the once-read approved intent in memory without plaintext-key export or
submission. Private refusal, approval-race and SDK signature/hash checks retain
the ordinary/browser six-action allowlist and public gateway restrictions.

Private persistent five-process consensus verifies a candidate created after
earlier blocks, replay without signatures/G, wallet-authenticated registration,
actual verified commit contribution/G, restart with preserved keys/progress,
changed-owner storage rejection and continued non-signing replay after normal
self-undelegation and removal. Fixed GOD supply and the two-block set delay remain;
the 21-day withdrawal rule is neither elapsed nor bypassed. Scoped race checks
use the checksum-bound lifecycle build. These private tests are not distributed.

The same lifecycle passed in a new private Linux amd64 VPS fixture directory
with checksum-verified executables and the reviewed logging-only overlay. Actual
node/wallet CLI help also ran; existing fourteen service process identities and
restart counters were unchanged, with both sync timers active. macOS arm64 race
checks, existing signed-execution/disk-recovery/signer-conflict regressions and
vet passed. This isolated runtime check is not public candidate activation,
participant-host coverage, Windows acceptance or sustained load/durability proof.

This is not a deployed registration path, independently operated network,
target-host/resource/durability acceptance or production release. The separate
opt-in chain rule rejects public-key-only registration on newly assembled
synthetic networks. No deployed/legacy network is silently upgraded. Cross-host
abuse acceptance, stake funding, installers, signed
configuration delivery, public peering and production economic, governance,
bridge and acceptance gates remain. Publication activates no service or assets.

### Self-service non-signing join candidate

`godd testnet join-observer` admits a fresh local observer against the exact
reviewed synthetic bundle without advance launch-profile registration. It
retains local keys/progress, exact genesis and bounded explicit peers, requires
loopback RPC and refuses validator roles, launch-key reuse, unsafe/mismatched
files or existing node/signing state. Ordinary launch `join` retains its exact
profile requirement. CLI/private race fixtures cover admission, unchanged
retained bytes and no activation or input disclosure on refusal.

Four persistent loopback validators commit a disposable GOD transfer before
the new identity exists. The observer then catches up, reads the same committed
block/balance and fixed supply, follows further blocks and restarts with original
keys and empty signing progress. No launch bundle, validator set, economics or
deployed service is changed. This is not intended-host/public peering acceptance,
installer/configuration delivery, secure state-sync checkpoints, public active-
validator admission, rewards or decentralized control. The private candidate
workflow above does not activate those services. Those remain separate gates.

### Keyless operator bundle preview

`godd testnet inspect-bundle` validates a selected private synthetic bundle
against its independently reviewed complete digest without keys, a node
workspace, sidecars, joining, writes or peer access. Its bounded review fields
include exact identities/policies and aggregate topology counts, not participant
addresses, endpoints, node keys/proofs or paths. Private race checks cover
bundle-only storage, full-digest binding beyond unchanged genesis, precise large
integers, canonical IP aliases, invalid proofs/settings, unsafe permissions and
symlinks. Node/signing/network/activation flags are refused. Declared IPs and
signed profiles do not prove independent people, machines, geography, routing,
wallet-owner authentication or production economic/launch approval.

### Restricted native gateway and durable inventory candidate

Included source now covers the six-action codec and keyless native boundary,
synchronous blocked-hash storage, offline audit/copy CLI, retained-height code
lookup and the history CLI's existing offline storage audit. Browser signing,
other gateways, private tests, configuration and binaries remain excluded.
The deployed native gateway has not been replaced by this durable candidate.

The offline CLI adds separately supervised page/freelist checks, bounded
canonical reports and an exact-image verification mode against a separately
retained checkpoint. Private race fixtures reject duplicate free-page references
and a checker-goroutine panic without disclosure, repair or destination writes;
they also cover worker timeout/nonzero exit, oversized/partial/duplicate reports
and the complete 100,000-record bound. A valid stale whole image passes an
unpinned inventory audit but fails against the newer approved checksum. Exact
restore and source/target copy checks retain original bytes and counts. No
listener, node request, signing progress or public unknown outcome is changed.
This is not authenticated checkpoint custody/freshness, power-loss certification,
protection against rollback of both database and checkpoint or live migration.
The worker timeout/file bound are not hard memory limits; require unprivileged
OS confinement with resource limits and no network or node/signing-data access.

Private race checks cover all six actions, exact signature/wire rebuilding,
coherent snapshots and disjoint fixed-supply/G buckets, acknowledged/unknown/
pre-ante outcomes, corrupt/unsafe storage, exclusive locking, seed/capacity
refusal, version-one compatibility and independent RAM/disk limits. Fixtures
retain 100,000 records without placing the entire inventory in RAM. Offline
copies preserve source bytes, exact count and logical digest; wrong source
checksums, shrink, overwrite and implicit policy changes are refused. Full
local Go regressions, uncached gateway/CLI race checks and private browser
signing/build guards passed. These are developer-run tests, not an audit.

Fresh processes exit without closing stores after reservation, unknown send
and pre-ante response before release. Copy fixtures exit before initialization
commit (a private format fixture) and after the actual atomic copy commits.
Recovery refuses the uncommitted target and retains every committed record.
These are process boundaries around completed commits, not power cuts during
fsync. Persistent local validators and an observer also exercised gateway
reopen after successful and failed execution without clearing guards.

The current candidate built through the Go logging helper passed eighteen
isolated Linux offline cases under the existing unprivileged account with a
768-MiB hard memory limit, disabled core dumps, write/execute-memory and network
denial, and inaccessible consensus storage. It verifies/copies 1,000 fixture
hashes at capacity 2,000, refuses a valid 999-record image against the retained
latest pin, accepts exact restore and rejects both freelist faults with static
refusal and no repair or target writes. All sixteen services/timers, existing
service PIDs and listening sockets stayed unchanged. This concerns offline
tools, not public HTTP migration. The public Go build helper retains an
explicitly selectable checksum-bound logging overlay;
private race tests verify deterministic edits, original-cache preservation,
exact mirror scope, missing compiler/pin failures and symlink-output refusal.
Public compilation is a separate requirement, not rerunning private tests.

The original module pins, GodCometBFT patch, economic rules and signing identities
are unchanged. Complete legacy inventory/drain, actual elapsed 21-day release,
public positive settled-G workflows, independent hosts/operators, sustained
capacity, physical restoration and valid-image rollback remain release gates.

### Opt-in durable address-history foundation

The exact core source includes `internal/godhistory`, `cmd/godhistory` and the
English [history guide](HISTORY.md), not private tests, runtime databases,
operational settings or binaries. The keyless CLI reads only the selected
numeric-loopback observer's `god_network` and `god_block`, with bounded replies,
complete-block pagination and paced requests. Compact public identifiers,
supported address membership and checkpoint commit synchronously together.
It stores no memo, calldata, raw wire, arbitrary message, event, log or faith
text. Address queries have twenty-row exclusive stored-anchor cursors and
explicit retained-range/unsupported-summary counts; empty does not mean no
activity. Interrupted or rejected reconciliation disables queries without
rewinding durable progress or touching unknown participant submissions.

Private race checks cover 20/20/5 block/address pages, anchored append, native/
compatible account equivalence, partial-page and late-write rollback, abrupt
process exit between blocks, malformed/changed source, corrupt membership and
owner-only storage. Actual persistent local consensus accepted a fixture GOD
transfer whose indexed sender/recipient records survived index reopen and
observer restart. The complete local Go suite and 101 wallet/browser checks
passed; unchanged Go packages may retain valid test-cache results. These are
developer-run synthetic checks, not independent audit or public payout proof.

A bounded Linux candidate indexed ten already committed pilot blocks over two
sync invocations and recovered an existing native exit record exactly once
across reopen. Ten CLI rejection cases passed without changing database bytes.
All twelve existing service processes, listeners and nginx config remained
unchanged. No claim, submission, validator restart, public-route activation,
complete backfill or NFT-history acceptance occurred. The existing pinned
bbolt alpha dependency is unchanged, not accepted production storage. Public
query-service/browser integration, NFT events, capacity/retention, power loss,
torn writes, independent review and sustained measurements remain open gates.

### Existing web and Chrome pilot

Web/Chrome alpha 0.3.9 is deployed under explicit authorization. The participant
tools support disposable encrypted accounts, recovery, bounded automatic
funding, reviewed GOD transfers, a restricted test NFT collection and six native
staking/G scopes. General public RPC writes, arbitrary contract signing,
website-provider signing, RH activation and mainnet assets remain disabled.

- Version 0.3.9 adds keyless current registration pages, at most eight validated
  records per request with an exclusive cursor and one exact commit. The
  101-test wallet suite, separate actual Chrome six-action fixture and selected
  node/gateway race regressions passed. Seventeen-validator 8/8/1 pagination,
  disk reopen, malformed/null inputs, corrupted records, changed tips and
  unknown-ID guards were checked privately. No dependency or economic rule
  changed.
- Live locked desktop/mobile/downloaded Chrome directory views verified all
  four current records at exact displayed commits, without native signers,
  claims or submissions. Fifty-one HTTPS files matched reviewed hashes;
  fourteen exact-origin preflights and six closed-origin denials passed.
  Existing disposable recovery and old unknown-ID guards passed on 0.3.9.
  Twelve services stayed active; five nodes had a matching fresh height/app
  hash and retained the original receipt, bank state and pending 21-day entry.
  The observer and keyless query process were updated; validator and existing
  submission/faucet processes retained their PIDs and original service files.
  A read-only registration page is not a persistent historical index, actual
  consensus participation, independent operation or completed exit/G workflow.

- Version 0.3.8 adds locked-wallet queries for one configured pilot operator
  through an independently constrained, keyless query-only process. The
  response's exact operator, quantities, commission, status and flags are
  checked against one fresh committed network snapshot. No list scan,
  arbitrary descriptions or signing work occurs. A fresh bonded record only
  fills an unsigned delegation form; absent, stale, jailed and tombstoned
  records cannot enable that shortcut. Unknown transactions keep signing
  blocked. Bonded status is not signing, uptime, ownership or geography proof.
  The configured list is not a complete validator index.
- One hundred private wallet unit/browser regressions passed with bounded
  concurrency; the actual unpacked Chrome native-action fixture and native
  query/gateway race regressions also passed. Negative checks reject mixed,
  missing, case-aliased, null, excessive and changed-policy records. Query-only
  mode cannot reach a transaction route or an upstream submission.
- Live desktop/mobile/actual downloaded Chrome acceptance checked all four
  configured operators, twelve exact displayed committed records, zero
  native signing workers and zero submissions/claims. Forty-eight HTTPS
  resources matched their hashes, twelve exact-origin preflights passed and
  five closed-origin requests were denied. The non-signing observer alone
  was upgraded. Four validator and all existing submission/faucet processes
  retained their PIDs and original service definitions; request reservations
  were not reset. Twelve services were active, five nodes shared a fresh
  height/app hash, and the original successful exit receipt, unchanged bank
  state and real pending 21-day entry were retained. These checks do not
  establish withdrawal completion, positive settled-G execution, independent
  operators or production-grade reliability.

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

The source also includes bounded current registration pages through
`QueryValidators`, trusted ABCI and opt-in local RPC. At most eight validated
records and one look-ahead key are read per request. Exclusive cursors use raw
operator-key ordering and exact latest-commit pins; a changed tip requires a new
first page. No offset walk, total-count scan, arbitrary metadata or keys are
exposed. Pages are registration records, not a consensus signing set, uptime
ranking or proof of independent operators or geography.

Private race checks passed seventeen-validator 8/8/1 pagination, empty ranges,
exact ABCI/RPC results, working/committed isolation, disk reopen and corruption
rejection. Normal EOF follows the pinned SDK iterator's `Valid` contract; read
panics and close failures return no partial page. Null RPC strings are rejected.
Private gateway checks preserve unknown-attempt reservations and reject mixed,
malformed, excessive and changed-policy responses. The 101-test wallet suite
and separate actual unpacked Chrome native fixture passed, including locked
pagination, no automatic retry and existing signing guards. Browser/other-gateway
source remains outside this exact public-core allowlist. These local checks do
not establish historical indexing or production readiness.

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
geography. Separately deployed pilot wallet integration is described above;
its browser source and other gateways are outside this exact public-core allowlist.
No signer, economic rule, validator-selection rule or dependency pin changed.

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

The public snapshot contains exactly 174 reviewed files: 173 fixed production/guide paths and the unchanged public whitepaper. This includes the keyless native codec/gateway, bounded durable reservations, offline maintenance and optional logging build helper; bundle-download/watch tools, archive unpacking, bounded history backfill and single-host guide; the custody/token candidate with its exact embedded compiler and existing CLI dependencies; and conditional account/parent verification with its English trust guide. Browser signers, other gateways and the standalone history HTTP service remain excluded. Private tests, fixtures, operational scripts, populated configuration, genesis, addresses, keys, runtime data, evidence, dependency folders, logs and binaries are excluded. The nginx template is not an enabled service. Private application or source-review ancestry must not become a public parent.

`make check` verifies the pinned lifecycle compiler input, then runs vet, module checksum verification and compilation. `make build` compiles the node/tool and packager commands; `make build-history-index` separately builds the local history CLI. Neither build starts a node or a history sync. `make status` runs the diagnostic without starting consensus or bridge operations. Public commands do not reproduce the private tests, fuzz windows or custody-bytecode checks. Publication screening checks exact file scopes, both commit identities, file bytes, commit messages and every reachable public ancestor. It is not a comprehensive secret audit or independent code review.

The native-specific check/build targets also select the logging-only overlay
and reject executable-memory dependencies in the gateway graph. Public guide
commands refer only to included production targets; operational values remain
unset. The source package does not distribute binaries or activate storage.

Public source verification requires lifecycle vet, original-module checksum verification, compilation, exact reviewed source bytes and byte-for-byte preservation of the public whitepaper. Native diagnostics must retain blank operational fields, disabled mainnet/real-asset commands and false source-finality/real-asset flags. Foreign binaries are not executed or distributed by source publication.

## Unverified release gates

Operator-grade shutdown and signer lifecycle, power loss, torn writes, corrupt disks, backup rollback and adversarial networks remain unverified. Light-client-attack penalties, quarantine release and governance are incomplete. Production emissions and gas rules, ordinary-computer specifications and sustained state-growth bounds are not finalized.

Successful delayed network bridge payments, verified source observation, independent signers, recoverable relaying and production-backed genesis remain unverified or absent. Positive fee budgets are not locked or certified sufficient; synthetic approvals do not establish independent source truth. Graphical and extension wallets, wallet recovery, complete explorer indexing, actual extension compatibility and secured public-service acceptance remain incomplete. Public deployment, bridge funding, binary distribution and real-asset activation require separate authorization and completed release gates.
