# GOD Chain local node runtime

`internal/godnode` connects God EVM execution, God SDK accounts and staking, and GodCometBFT consensus in an isolated synthetic-asset runtime. Four separate validator processes have produced matching committed state on the same computer. This is local integration evidence, not a live GOD Chain, independent operators, RH backing or mainnet readiness.

## Private launch candidate assembly

`PrepareBridgeLaunch` derives native balances and consensus validators directly
from the canonical, quorum-approved bootstrap plan. There is no separately
entered allocation that can diverge from the deposit recipients. It admits
explicit startup/working-Gas budgets, validator possession policy and bridge
transactions together. Total credited GOD must equal initial self-stake plus
the declared Gas budgets, within the explicit startup budget. GOD still comes
from the restricted fixed reserve; the declaration is never a second mint.
This candidate starts with an empty redemption pool and refuses an unimplemented
nonzero seed rather than displaying it as funded.

The proposal produces the existing node-authorization challenge. Completion
requires supplied quorum signatures, owner consent and consensus-key possession
for the exact runtime, funding, genesis time and consensus parameters. It does
not create/sign with an operator key. Explicit payload access returns detached
runtime/genesis for private rehearsal; ordinary formatting/reports are redacted.
The source statement remains a synthetic quorum input, not authenticated RH
backing. A declared block interval is a candidate operating assumption, not a
measured cadence or new consensus-enforced timing rule.

The operator commands `launch-assembly-template`, `check-launch-assembly` and
`stage-launch-candidate` combine these inputs with the economic candidate in one
private document. Blank template fields cannot pass. Exact nested field names,
duplicate/trailing/deep JSON refusal, runtime emission/minimum-fee matching,
validator count, declared cadence and fee-budget reconciliation are checked
before staging. All monetary and identity inputs are explicit; no production
Chain ID, Gas price or initial wallet balance is guessed.

Review/staging uses separately retained exact input hashes and owner-only
Linux/macOS files. Staging creates only a new mode-0600 file in an existing
mode-0700 private directory, verifies readback and never overwrites, repairs or
removes an existing/partial artifact. The artifact contains the actual runtime
and consensus genesis, not only a success report. Keep it ignored and private.
Creation/recovery does not open a chain database or a listener. Windows private
file staging remains closed pending ACL acceptance.

The output is explicitly a `synthetic-bridge-launch-candidate`. Ordinary mainnet
`init`, `start`, `bridge` and `mint`, non-prototype runtime and public bridge
peering remain refused. This composes the launch materials but does not complete
the production executable, key custody, independent source finality, funded
custody, governance/upgrade policy or intended-host installation. Do not rename
the pilot database or promote synthetic balances into production assets.

Use `make test-launch-assembly check-launch-assembly test-launch-assembly-cli
compile-launch-assembly-linux` for scoped local verification. The command-line
round trip stages fresh generated fixtures only. Linux cross-compilation is not
Linux runtime acceptance or authorization to replace a deployed node.

## Run a privately staged candidate

`init-launch-rehearsal` and `run-launch-rehearsal` now consume the actual staged
candidate, not its success report. Every invocation independently pins the
original complete signed assembly and the candidate file, re-derives the exact
candidate bytes and checks the original files again. Changing a candidate and
providing a new checksum does not authorize changed funding, runtime, genesis
or cadence. This remains a prototype-only launch rehearsal, not mainnet start.

Initialization requires an existing mode-0700 identity-only home with the
original mode-0600 consensus key, empty signing state and P2P key. The consensus
key must match an authorized genesis validator. It creates a new private launch
profile and a mandatory separate signing-witness requirement under the node
lease. It does not generate/copy keys, adopt used signing state, replace a joined
pilot workspace or repair an interrupted initialization. Keep the witness
outside the node backup scope; alternate paths cannot hide a nested witness.

The operator interfaces are:

```text
godd init-launch-rehearsal --home <fresh-identity-only-home> --signer-guard <separate-existing-private-directory> --input <original-signed-assembly> --input-pin <independently-reviewed-SHA256> --candidate <staged-candidate> --candidate-pin <independently-reviewed-SHA256> --p2p <explicit-loopback-listen> --peers <explicit-loopback-peer-list>
godd run-launch-rehearsal --home <same-retained-home> --signer-guard <same-retained-directory> --input <same-original-assembly> --input-pin <same-reviewed-SHA256> --candidate <same-staged-candidate> --candidate-pin <same-reviewed-SHA256> --seconds <2..180>
```

Each validator runs in a separate process with persistent application and
consensus stores. Restart retains the same identities, candidate binding,
committed state and key-wide signing witness. Existing cooperating home/key
leases exclude duplicate starts; missing/wrong/stale signing witnesses fail
closed before application opening. Consensus shutdown joins its workers before
application closure and lease release. The duration is cooperative, not a hard
deadline for a blocked filesystem or shutdown; a reviewed supervisor remains
necessary for operational resource/deadline limits.

All peers and listeners are explicit loopback endpoints. There is no HTTP RPC,
source client, wallet signer, real funding request, public bridge peering,
production switch, automatic restart or financial relay. Redacted JSON lines
report initialization/start and actual committed heights, fixed-supply match
and observed on-chain G. Pending G remains unclaimed, not a spendable reward.
A successful process exit without committed progress is not launch acceptance.
The witness is a local cooperating-process safeguard, not protection from the
host owner, whole-host rollback or another unreviewed signing executable.

`make test-launch-rehearsal test-launch-rehearsal-cli check-launch-rehearsal
compile-launch-rehearsal-linux` checks the complete local path. Four generated
temporary validator identities have started from one staged candidate, produced
matching committed ledger/bridge/staking/G state and continued after whole-group
restart. Rollback and duplicate startup are refused. These processes share one
computer and one fixture custodian; they are not independent operators. Linux
compilation does not install or run this candidate on the VPS. Public testnet
startup remains bridge-disabled and cannot consume this private profile.

## Private launch candidate observer

A separate observer can replicate the same exact staged bridge-funded candidate
as the four guarded validators. It uses fresh pre-existing consensus and peer
identities that are distinct from each other and from all genesis validators;
its persisted signing state must remain empty. The runtime replaces the supplied
signer with a permanently refusing observer adapter. No signing witness is
created or accepted for this role, and validator keys are never copied into it.

```text
godd init-launch-observer --home <fresh-non-validator-identity> --input <original-signed-assembly> --input-pin <reviewed-SHA256> --candidate <same-staged-candidate> --candidate-pin <reviewed-SHA256> --p2p <loopback-listen> --peers <loopback-peer-list> --rpc <loopback-listen> --hosts <exact-loopback-hosts> --origins <exact-loopback-browser-origins>
godd serve-launch-observer --home <same-retained-observer> --input <same-original-assembly> --input-pin <same-reviewed-SHA256> --candidate <same-staged-candidate> --candidate-pin <same-reviewed-SHA256> --report-seconds <1..300>
```

Initialization checks the original signed assembly, candidate and identity-only
home before creating a lease or profile; it repeats checks under ownership.
Existing pilot/validator profiles, used signing state, data, aliases or partial
files are not converted or repaired. Startup and periodic rechecks re-derive the
candidate and retain the original profile, keys and endpoints. Coordinated key
and profile replacement does not authorize changing a running service.

The loopback listener shares existing body, connection, concurrency, rate and
timeout limits. A positive read-method allowlist exposes committed account,
staking/G, bridge/withdrawal and native/compatible block/transaction reads.
The private persistent observer alone enables the consensus transaction index;
validator indexer behavior is unchanged. `eth_sendRawTransaction`, native
submission, pending sequences, `eth_call`, `eth_estimateGas`, signer/admin and
future unknown methods remain refused. Exact host/origin checks admit only
localhost or literal loopback addresses, with no network opt-in.

SIGINT/SIGTERM joins HTTP requests before consensus, application stores and the
retained home lease close. Reports identify actual increased committed height;
unchanged/no committed state is a heartbeat, not progress or source backing.
Four-validator integration verifies equal ledger/staking/bridge state and
consensus block history, read-only refusal and restart with unchanged observer
signing state. A validator-admitted signed synthetic GOD transfer separately
verifies indexed native/compatible TX details, receipt and block summary at the
exact successful inclusion height; the observer rejects that same signed wire.
This remains local synthetic acceptance. Production wallet
configuration, NFT contract reads, signed submission boundaries, public access,
real source funding and intended-host installation are not enabled here.

## Private candidate supervisor preparation

The stopped `check-launch-service` command holds the original home lease and
key-wide signing lease while rechecking the exact independently pinned assembly,
candidate/profile, original key/state/witness and a separately pinned static
Linux node executable. It opens no application store or listener, signs nothing
and creates no missing lease, data or witness. Active homes, wrong pins,
unsupported binaries and rolled-back signing progress are refused. Original
inputs and the binary are checked again before success; the report is redacted
and does not grant later startup permission.

```text
godd check-launch-service --home <same-private-home> --signer-guard <same-separate-witness> --input <original-signed-assembly> --input-pin <reviewed-SHA256> --candidate <original-candidate> --candidate-pin <reviewed-SHA256> --binary <reviewed-static-linux-godd> --binary-pin <reviewed-SHA256> --arch <amd64|arm64>
godd prepare-launch-supervisor <same-nine-flag-bindings> --uid <current-non-root-UID> --gid <current-GID> --report-seconds <1..300> --memory-bytes <explicit-budget> --cpu-percent <explicit-budget> --tasks <explicit-budget> --open-files <explicit-budget> --start-seconds <15..300> --stop-seconds <15..300> --output <new-private-unit-file>
```

Preparation writes one new mode-0600 **private** systemd unit in an existing
owner-only directory, checks readback and keeps both original leases during
staging. An existing output is never replaced; interrupted/failed material is
retained for manual review, not reported as usable. The explicit UID/GID must
match the current non-root owner; no account or administrative authority is
created. Installation using the existing root management connection remains a
separately authorized operation. A generated checksum is byte continuity, not
independent release authentication.

The unit runs stopped preflight before the same guarded persistent service.
It has no shell, environment injection, Install section or automatic restart.
CPU, memory, task/file limits and startup/stop timeouts are explicit. SIGTERM
requests joined shutdown; expiry may terminate the process, requiring original
signing-progress checks and manual review, never a reset or automatic retry.
Only the exact home and separate witness are writable; binary, original assembly
and candidate are read-only. No capabilities, new privileges, executable-memory
loader or non-loopback IP access is requested. Four units still share one
founder-operated host, not independent operators or failure domains.

Argv/list paths are quoted, while the scalar working-directory directive is
literal; interior spaces remain supported and boundary whitespace is rejected.
Paths reject systemd expansion, directive injection
and ambiguous nested/aliased writable inputs. Operational paths in hidden
temporary/kernel trees are refused; use retained private storage, not `/tmp`.
Linux fixtures preparing units must select a disposable private `TMPDIR` outside
those hidden trees. Build metadata and checksum checks do not prove a binary's
command compatibility, dependency-overlay provenance or trust of the checksum
source. An isolated intended-host four-process fixture has passed the generated
unit parser, actual constrained execution and retained restart; the initially
rejected scalar-path quoting was corrected without relaxing isolation. This
does not install the four generated per-node units or accept production capacity,
sustained load, alerts or signing-safe whole-host recovery. Preparation
applies no limits and installs/starts no service; all production/real-asset
flags remain false. No existing public service or key is changed.

Private verification targets are `test-launch-supervisor`,
`test-launch-supervisor-cli`, `check-launch-supervisor` and
`compile-launch-supervisor-linux`; private fixtures are not part of public
source snapshots. Linux compilation does not run or install a service.

## Persistent private candidate service

`serve-launch-candidate` adds a cancellable continuous service path for the same
already initialized, independently pinned candidate. It shares the bounded
rehearsal's startup, consensus, persistent stores and shutdown implementation;
it does not replace a time limit with production authority. No new keys, homes,
genesis, funding, public peers or asset modes are created. Ordinary mainnet
`init`/`start` remain refused. The selected topology is one founder-operated
host with separate validator processes, not independent regions/operators.

```text
godd serve-launch-candidate --home <same-retained-private-home> --signer-guard <same-retained-separate-directory> --input <original-signed-assembly> --input-pin <independently-reviewed-SHA256> --candidate <staged-private-candidate> --candidate-pin <independently-reviewed-SHA256> --report-seconds <1..300>
```

All seven flag/value pairs are explicit, with no repeated or last-flag-wins
bindings. The command has no `--production`, `--allow-network`, RPC, signing-key,
reset or duration override. The existing fresh initialization ceremony is
still mandatory; an ordinary testnet home cannot silently become this service.
Missing/wrong/stale witnesses and duplicate home ownership fail before
application opening. Retain original reviewed input/candidate pins separately
from restored node data; a pin supplied by the same unreviewed file is not trust.

The caller supplies cancellation; the CLI maps SIGINT and SIGTERM to that
context. A successful exit joins consensus workers, closes the application and
signing witness, then releases the home lease. Startup cancellation, output
failure and shutdown/guard failure return a fixed error. Cancellation is
cooperative, not a hard deadline for blocked filesystem access, output or
engine shutdown. There is no automatic restart or service-manager installation;
an independently reviewed supervisor must provide resource/shutdown limits.
Never turn an unclean stop into a fresh key, empty witness or signing reset.

The active witness and home identity are checked every half second. At the
explicit reporting cadence, the original signed input and staged candidate are
revalidated, along with the canonical launch profile and witness requirement.
A changed/missing/unsafe input stops the service; no file, pin or balance is
repaired. These checks detect changes between observations, not hostile-owner
modification, whole-host rollback or physical-power-loss safety.

Redacted reports explicitly use `private-synthetic-launch-service` and
`service: true`, while production, real-asset and public-peer flags remain
false. Actual observed committed height, fixed-supply match and on-chain G
remain distinct from a `heartbeat` with no new committed height. A node without
quorum can emit a healthy process heartbeat; that is not network readiness,
another block, spendable rewards or source funding/finality. Reports before the
first committed version do not claim a supply match or G issuance. Report
cadence is operational only and does not change block timing or reward policy.

Repeat `make test-launch-service test-launch-service-cli test-launch-service-regression check-launch-service
compile-launch-service-linux`. Compiled CLI acceptance starts four disposable
validator processes, stops them with SIGTERM, checks equal committed ledger,
bridge-funded staking and signature-derived G, then restarts the same stores
and identities. Duplicate startup, rolled-back signing progress and missing
witnesses are refused. A lone restarted validator cannot advance the ledger;
its heartbeat creates no G. Changing the original input or candidate makes it
exit without repair, and the retained ledger remains unchanged. The old bounded
rehearsal also passes the shared path. No live service, operator key or public
network is involved. Linux compilation is not intended-host execution or
production installation; source-backed funding and mainnet activation remain
separate release requirements.

The ordinary synthetic node startup also performs read-only configuration and
exposure preflight before creating a home lease, then repeats full loading
under the lease before any signing/runtime use. Invalid observer-to-validator
role edits cannot add a lease file or open application data. Observer recovery
acceptance waits for the exact second transfer receipt before checking its
recipient balance; an old relative height alone is not proof of catch-up.

## Execution and fixed supply

The runtime uses the locked execution keeper and Ethereum signature checks. It accepts legacy, access-list and dynamic-fee Ethereum transactions in a bounded SDK envelope. Contract deployment, bytecode execution and persistent contract storage use the same committed application state as GOD balances. Native participant transactions retain the reviewed single-owner direct-signature rules. Both paths share account sequences, so mixing transaction formats does not create a second nonce balance.

Native ecosystem precompiles, execution hooks, IBC, automatic ERC20 conversion, blob transactions and delegated-code transactions are not enabled. A restricted synthetic JSON-RPC adapter adds bounded account, block, receipt, raw-transaction, call and gas-estimation methods. A persistent wallet and full wallet compatibility remain separate deliverables. See [TESTNET.md](TESTNET.md#available-rpc-methods) for exact method limits.

The reference balance setter ordinarily changes bank supply while reconciling EVM account deltas. The GOD adapter instead stages those deltas and atomically transfers existing GOD from debited accounts to credited accounts. Debits precede credits, irrespective of address order. It neither borrows the bridge reserve nor grants mint or burn permission. Failed business execution rolls back the message cache. EVM gas refunds are paid before residual fees enter pending rewards; native signed fees retain their separate full-requested-fee rule.

EVM self-destruction-to-self can discard native value under Ethereum semantics. GOD Chain preserves that value in a protected quarantine account instead of burning it. This is an intentional native-value policy difference, not general Ethereum equivalence. Quarantine has no authenticated release or reward route. GOD supply remains 1,000,000,000 throughout execution. Fixed supply alone does not establish backing.

## Conditional exact native execution

The private in-process `ExportNativeExecution` reads an explicit stored
transaction block, subsequent confirmation header and signed commit from its
following block. It exports only consensus-committed result fields, never SDK
logs, events or codespace diagnostics. There is no HTTP route, index search,
latest fallback, new signer or transaction submission. Reports and ordinary
material/trust formatting are redacted.

`ReviewNativeExecution` verifies an exact envelope at a selected index against
the full transaction/result roots and quorum signatures. Independent exact
header/validator trust is mandatory; the exporter cannot authenticate that
trust's origin, freshness, validator transitions, current account state or RH
payment. It supports bounded multi-transaction material and distinguishes an
included failed SDK transaction from code-zero execution. SDK code zero is
not a general God EVM-success claim. The bridge consumer separately checks its
single native resolution message before any conditional receipt review.

Actual four-validator process acceptance covers successful/failed bridge
transactions, cancellation, offline catch-up and restart using these exports.
The shared return book composes the exact original dispatch with retained
execution observations and unknown-result recovery. See
[native execution recovery](BRIDGE.md#exact-native-return-submission-and-execution-recovery).
This does not enable public bridge peering, production signing or financial
relay activation. All acceptance assets and source statements remain synthetic.

## Committed state queries

`QueryNetwork(height)` and `QueryAccount(address, height)` read an explicit committed application version under the runtime mutex. Zero selects the latest committed height; a nonzero height must match that current version. Older, future and negative heights are rejected. Account queries accept either reviewed native or EVM encoding and return canonical presentations of the same account. No account is created by a read.

The local synthetic RPC adapter separately allows code-only `eth_getCode` reads
at an explicit retained committed application height for NFT history acquisition.
It validates that version's stored height metadata and never substitutes latest
code for missing/pruned state. Predeployment code stays absent even after later
deployment. Other account/storage/call query height rules remain unchanged. This
is not an archive or header/state proof. The separately authorized NFT history
pilot now upgrades only its nonsigning observer for these code reads, preserving
its identity/data and the four validator processes. Generic public RPC does not
gain code/call/submission methods from this observer-only change.

The network view includes chain identifiers, application height, committed time, application hash, fixed GOD bank supply and reward-pool totals. The account view includes the auth-account existence and module-account flags, account number, sequence, bank GOD balance, spendable G, settled unclaimed G and pending earned G. GOD and G amounts are decimal strings in smallest units. Height, chain ID number, account number and sequence also use JSON strings to preserve exact integer precision. The application hash is not a consensus block hash or an authenticated proof.

Pending earned G is not claimable or redeemable. The three account G fields do not include application locks and must not be labeled as total G. Bank GOD balance is not a spendability estimate. Querying a protected module does not grant permission to spend its funds. Every returned view explicitly marks synthetic operation and disables real assets.

CheckTx caches and finalized but uncommitted changes remain invisible to these queries, including across a UTC-day settlement boundary. Committed metadata supplies the timestamp rather than wall time or a newer working header. Results contain detached values; reading neither settles rewards nor changes fees, sequences, clocks or ledger state. Invalid state returns a static error without a partial result or private panic details.

The trusted in-process ABCI adapter exposes `/god/network` with empty data and `/god/account` with bounded address text. Both reject proof requests and unsupported heights; `/god/status` retains its existing behavior. It does not open external ABCI, store access, unsigned simulation or transaction sockets. The separate testnet RPC uses detached committed queries and an explicitly restricted transaction boundary; adding this service does not make a publicly available network.

`QueryDelegation` adds one bounded committed staking lookup for a canonical validator and native/EVM delegator. It returns the shares and truncated GOD equivalent at that height, not spendable GOD or completed unbonding. It does not enumerate all historical validators or delegators.

`QueryValidator(operator, height)` adds one canonical lowercase operator lookup,
with exact stake/shares, fixed commission/minimum self-delegation, staking status,
jailed state and stored permanent exclusion. It returns explicit absence for an
unknown operator and no partial record for invalid state. It scans no validator
or delegator collection and exposes no descriptions, contact data, signing key
or private content. The trusted `/god/validator` and opt-in local
`god_validator` routes use the same committed view. Bonded status is not proof
of signing a particular block, independent ownership, uptime or location.
This source-level interface has local race/HTTP/loopback acceptance, not public
pilot activation. The deployed public read-only gateway and wallet routes are
unchanged; bounded participant discovery and UI integration remain separate.

## Synthetic node commands and RPC

`internal/godtestnet` provisions private persistent synthetic workspaces and supplies independent identity, bundle assembly, join, check, startup and offline native-signing tools. Profiles are signed by both consensus and P2P keys; account keys remain externally held. Joining pins the complete reviewed bundle, not only genesis, and refuses existing data or nonempty signing progress. This initialization protocol does not establish independent human control or account ownership.

`StartTestnet` permits explicit nonloopback peers only in persistent, bridge-disabled synthetic mode. `StartLocal` retains its loopback-only scope. Engine RPC, gRPC, metrics, profiling and peer discovery stay disabled. Standalone validators must keep their restricted RPC on loopback; a separate observer can serve a secured gateway. Its consensus signer always rejects votes and proposals, even if its key later appears in staking state. Operator configuration and actual public exposure require separate review and authorization.

Fresh local bridge-enabled runtimes may also require validator-key possession
on registration. Registration bytes normalize the bridge signer order exactly
as application assembly does, while binding the asset, configured signer set,
approval Gas and complete runtime. Genesis owner/consensus consents stay separate
from later registration signatures. The local candidate guard still requires
committed operator ownership and exact-height active-set eligibility; it does
not fund a candidate, register it automatically or reset signing progress.
Enabling this policy requires new launch approvals and cannot reopen an existing
database under changed policy. Public bridge peering and production startup
remain disabled. See [combined integration](VERIFICATION.md#combined-bridge-and-validator-possession-policy).

The HTTP adapter requires exact host and origin lists, bounds connection count, request size, response size, aggregate request rate, concurrent work and execution gas, and suppresses request/panic logging. Call and gas-estimation writes occur only in a discarded cache. It never stores wallet signing keys or exposes account unlocking, personal/admin/debug routes, subscriptions, arbitrary state overrides or fabricated proofs. A hardened HTTPS gateway and per-client abuse controls remain external requirements.

Signed Ethereum submission uses the reviewed SDK envelope and shared sequence rules. Receipts verify the actual indexed hash, block, SDK wire and EVM response; a VM revert reports status zero even if the outer SDK result has code zero. Ethereum receipt and log indexes match the Ethereum lane exposed in block queries. The native consensus index is separately identified. SDK and Ethereum gas are distinguished; consensus block hashes are not Ethereum RLP header hashes, and nonexistent Ethereum state roots are not fabricated.

Native submission accepts only externally signed bounded wire. Its result distinguishes admission from inclusion, and SDK success is not a general EVM-success signal. The offline helper supports delegation, undelegation, G claim/transfer/redemption and GOD donation against a pinned synthetic bundle. It signs into a new owner-only file and never submits automatically. Existing typed protocol adapters still enforce amount, minimum-output, liquidity, fee and sequence rules.

The command tools require Go 1.25 or newer for confined private storage, with Go 1.26.8 used for verification. Windows private disk operations fail closed pending ACL support. Redacted startup phases distinguish busy service ports without printing underlying errors. `god_liveness` reports actual application and block-store heights, peer count and consensus progress without exposing identities or endpoints. A standard local client passed preparation, signing, inclusion and balance checks with replacement detection disabled; browser-wallet acceptance remains separate. The deployment workflow and acceptance gates are in [TESTNET.md](TESTNET.md).

## Validators and staking

The runtime uses the locked staking keeper, with `agod` as the bond denomination, a maximum active set of 32 and a 21-day unbonding period. Signed creation requires at least 1,000 GOD of self-delegation and a fixed 10 percent commission. Signed delegation requires at least 1 GOD. Undelegation must not leave a positive position below 1 GOD. Consensus keys are Ed25519; account signing keys remain EVM compatible. Their roles are separate.

Creation, delegation, undelegation and signed reactivation are added to the native participant allowlist. Redelegation, commission edits, parameter updates, governance and generic bank messages remain unavailable. A local resource bound allows at most 256 delegators per validator; this is not a finalized worldwide participation limit.

Validator updates take effect after the consensus engine's two-block delay. The application stores the expected signing sets by effective height and validates the preceding commit against the set for the block actually signed. It does not substitute newly selected validators for prior signers.

## Validator penalties

The local policy evaluates downtime after 10,000 expected signing opportunities and then uses a rolling window of that size. Exactly 1,000 missing signatures is permitted; more than 1,000 suspends the validator for at least one hour. Heights while outside the expected set do not count. A verified nil precommit is participation, not downtime, although it earns no block G. The full-window evaluation convention is local prototype policy and must be included in production review.

Ordinary downtime calls no principal-slashing function. Reactivation requires an authenticated transaction from the operator, expiry of the suspension, at least 1,000 GOD in self-delegation and a non-tombstoned identity. It resets the downtime window; elapsed time alone never reactivates a validator. Missing signing information fails closed. No participant parameter-update route is enabled.

Counters advance only when blocks commit. If more than one third of voting power becomes unavailable, the network cannot commit blocks and these counters cannot automatically repair that loss of quorum. This runtime offers no emergency validator replacement or signer-reset command; a production recovery policy remains a separate review requirement.

Duplicate-vote evidence enters only through the local GodCometBFT client after both signatures, historical validator membership and evidence eligibility have been checked by consensus. ABCI summaries are not independently verifiable proofs and are never accepted through a public socket or participant transaction. Light-client-attack evidence has no approved penalty policy and remains unsupported; this is a production release gate.

Double signing applies the locked staking keeper's 5 percent infraction-height accounting to applicable stake, including retained unbonding and redelegation exposure. The staking bank adapter transfers every affected bonded or unbonded pool amount into protected quarantine instead of burning it. It never routes these funds to G redemption liquidity. Consensus-key tombstones and operator-account exclusion are permanent; repeated evidence cannot charge the identity again. They identify keys/accounts, not a human person, and cannot prevent someone creating a genuinely new identity. Quarantine release policy and governance remain unimplemented.

The preceding block's eligible signing contribution is accounted before newly detected penalties; previously earned G is not erased. Jailed identities are excluded from future eligible stake snapshots immediately, while consensus removal observes the normal two-block update delay. Local evidence and cached historical-exposure checks are described in [VERIFICATION.md](VERIFICATION.md); they are not an audit or proof of adversarial-network safety.

## Contribution rewards

GOD does not issue block inflation. G allocations are derived internally from the last commit delivered by the local consensus client. GodCometBFT verifies the real signatures; ABCI vote flags alone are not cryptographic proof. The runtime has no remote ABCI socket through which a participant can supply forged contribution flags or reward amounts.

An eligible delegation snapshot is taken before transactions in the block being signed. Later stake changes cannot retroactively earn that block's reward. A signing validator receives its share according to the consensus voting power expected for that height. Its 10 percent operator portion is separated first; the remainder is divided among eligible self-delegation and other delegation shares. The operator's self-stake participates in that remainder. Absent or nil votes receive no allocation, and their unused shares are not redistributed.

`GPerSignedBlock` must be explicitly provided for synthetic testing; no production value is selected. The allocation budget is limited to the remaining 10,000 G UTC-day ceiling. All divisions round down, and rounding dust remains unissued. The fast network fixture deliberately uses a test budget, not a published emission schedule. A roughly six-second production cadence would still require an approved, measured per-block rule.

Only the preceding block is considered. A signature from a prior UTC day receives no catch-up allocation in the new day. Pending earned G settles through the existing day-boundary mechanism; it is not immediately spendable or redeemable. Current GOD fees also remain pending until that boundary. The voluntary redemption rule and G's other ecosystem uses are unchanged.

## Isolation and lifecycle

Configuration and synthetic genesis must select prototype mode. There is no backed genesis pathway or mainnet activation flag. Genesis is bounded, strictly decoded and deterministically ordered; account balances plus restricted reserve equal the fixed GOD supply. Initial stake is transferred out of those synthetic balances, not created in addition to supply.

Only explicit `BridgeGenesis` mounts `x/godbridge` for synthetic initialization. A separate positive `BridgeApprovalGas` selects `NewBridgeExecutionEncoding` and the six authenticated routes; zero preserves ledger-only behavior. Ordinary native/execution codecs and nil-policy nodes reject bridge envelopes. Synthetic statements cannot be relabeled as independently verified source backing; a still-unreleased deposit cannot pay its own native fee.

The isolated bridge has an attested initial reconciliation API that checks deposit completeness, source checkpoint credit, validator stake and fee budgets before reserve release. The node's optional `BridgeGenesis` policy now checks an authorization certificate against its synthetic genesis. Deposit sums must match every participant balance, and planned owners, keys and self-stakes must match the actual validator declarations. A separate node quorum binds complete node configuration, consensus parameters and the current execution, fee, staking and penalty genesis policy. Each validator supplies separate account-owner consent and consensus-key possession signatures.

The gate runs before engine assignment and persistent binding, and again before application genesis initialization. Duplicate, case-aliased or unknown nested JSON fields, noncanonical byte arrays, missing proofs and required consensus components are rejected. The successful authorization digest is committed in node metadata and retained across block updates and process reopen; a resumed local engine checks the incoming genesis against that record. Canonical ordinary synthetic JSON and nil-policy configuration encoding remain unchanged; previously tolerated noncanonical byte-array representations are now rejected.

The certificate authorizes synthetic genesis only. In this mode the entire fixed supply starts in restricted reserve, then `InitBootstrap` funds fresh accounts before self-staking. Participant balance declarations are exact reconciliation inputs, not a duplicate bank allocation. All node initialization writes share one cache; failed bank releases or later invariant checks leave no partial participant, staking, replay or metadata state. An internal failure disables the instance rather than retrying sealed execution globals.

Genesis completion, block preflight/completion, committed snapshots and reopen check aggregate reserve/pending accounting and the immutable bootstrap record. Replayed deposits and repeated bootstrap do not release GOD again. Opt-in routes retain the native owner/fee/sequence boundary and charge explicit gas per quorum signature. Ethereum, G and bridge messages share one account sequence. Reopen validates ledger progress using committed height and time. Actual source verification and independent signer operations remain required. The current 10,000 GOD exposure limit cannot fund ten minimum-self-stake validators plus positive fee budgets; no limit was increased. See [BRIDGE.md](BRIDGE.md) for signature domains and these boundaries.

The ABCI adapter bounds transaction bytes, count and requested gas in proposals and finalized blocks. It enforces sequential heights, monotonic time and commit order. Internal execution failures poison the instance so a recovered panic cannot authorize a partial commit. Snapshots read explicit committed versions, not the SDK's pre-commit working state. Unsupported vote extensions and state-sync restore are rejected.

The pinned execution configuration is process-global and sealed. One runtime is allowed per process; a second constructor fails before modifying it. Independent validators require separate processes. A fresh process reopening committed application storage checks the configuration binding, fixed supply and stored coin metadata, then restores execution globals without repeating genesis. The same application cannot be attached to two consensus engines, including after stop. Application close is rejected while its engine is running.

`StartLocal` accepts loopback peers only. It disables peer discovery, external RPC and gRPC, profiling, metrics and transaction indexing. The default uses memory consensus databases. Explicit `Persistent` mode uses disk-backed block, state and evidence databases plus the consensus WAL in a caller-owned private directory. Its persisted identity binding includes the execution configuration, genesis digest, validator key and peer key; incompatible input is rejected rather than replaced. No populated address, key, genesis or endpoint is supplied in source.

Persistent mode requires a caller-provided file signer with existing key and last-sign-state storage. It checks Ed25519 key consistency and verifies nonempty signing progress against the key, chain, height, round and step. The committed record of signatures already observed by consensus is a lower bound; the application height plus one is the upper bound. This permits an excluded or offline identity's legitimate signing progress to lag its application's synchronized height without permitting observed progress to be forgotten. It cannot detect every lost uncommitted signature or make backup rollback safe. Callers must never reset signing progress, create a missing replacement key or use empty-state loading to recover an identity. New synthetic provisioning creates fresh workspaces only; it supplies no backup-rollback or signer-repair command.

Ordinary nil-policy runtime configuration retains binding version 2. Explicit ledger-only bridge initialization uses version 3; opt-in participant routes use version 4 and bind the exact approval gas. Node authorization digest domain version 2 includes these distinctions and the complete configuration. Incompatible databases or certificates require separately reviewed migration; they must not be silently reinterpreted, cleared or assigned replacement keys. No migration or production upgrade pathway is supplied. The separate native-only application retains its existing binding.

### Independent retained signing witness candidate

`GuardedSigner` adds an opt-in second witness outside the node home. One shared,
owner-only directory must be retained separately from all node backups. A
public-key-derived file lease excludes cooperating processes using that key in
the same directory, including a copied home or different chain binding. It does
not exclude another directory, host, older executable or privileged owner.

The witness binds the key digest, chain ID, canonical genesis digest and exact
FilePV height/round/step/signature/sign-bytes digest. Before each signature it
checks both the in-memory signer and the retained witness. The guarded synthetic
node workflow additionally binds the original private key/progress files as
described below. The file signer persists first; original progress is checked
and retained before the second witness is written, synced, atomically replaced
and directory-synced. Both bindings are rechecked before the caller receives a
signature. Ordinary conflicting
or regressing requests remain refused; identical votes reuse the original
signature and timestamp. Vote extensions remain disabled. Inputs, file sizes,
private permissions, effective-user ownership, hard links, symlinks and retained
lease identity are checked. Diagnostics never echo paths, key material or payloads.

Any uncertain write, panic, missing/corrupt record or progress mismatch stops
signing. A crash in the gap between the two writes can therefore stop the
validator on reopen; availability is not preferred over unknown signing safety.
There is no automatic adoption, reset, repair or restore command. Initialization
accepts only an empty signer in a never-started synthetic home with no data or
existing requirement marker. That check cannot independently prove a key has
never been used elsewhere. Operators must retain the original fresh-key
provenance and must never roll back/delete the witness with a node backup.

The private diagnostic workflow is:

```text
godd testnet guard-init --home <fresh-private-validator> --signer-guard <existing-separate-private-directory>
godd testnet start --home <private-validator> --signer-guard <same-retained-directory>
```

Initialize each fresh validator once, using the same retained directory for all
cooperating identities on that host. The directory must be outside the entire
backup scope, not merely outside one validator subdirectory. A requirement
marker makes this runtime refuse omitted, changed or overlapping directory
arguments before opening application storage/listeners. That marker is synced
under the key lease before witness creation, so an interrupted initialization
cannot silently fall back to an unguarded start. Non-signing observers
are not initialized with this feature. Existing deployed validators are not
automatically migrated; their used signing state cannot be adopted by this CLI.
Direct library callers must exclusively own their FilePV and close the guard
only after consensus shutdown and worker joins. Linux/macOS disk adapters require
Go 1.25 or newer; unsupported platforms fail closed rather than use an unlocked
fallback. This candidate does not make whole-VM rollback or host power loss safe,
provide authenticated independent retention, certify Linux runtime acceptance,
upgrade existing services or enable mainnet/real assets.

#### Bound original signer files

The synthetic `guard-init`, `guard-check` and guarded `start` workflow uses
`InitializeFileSigningGuard` and `OpenFileSigningGuard`, not the bare witness
adapter. These explicitly bind the original key and progress files in the
original node home. The home must remain owner-held `0700`, and each file must
remain regular, owner-held `0600`, single-link and within its retained root.
Nonblocking, no-follow opens refuse pipes and symlinks; path/directory entry
identity is checked before and after reads. The immutable key inode is retained
for this adapter's lifetime; state replacement by the existing atomic FilePV
writer remains allowed when the complete expected progress matches.

Before signing, disk key/progress bytes must match their checked in-memory
encodings as well as the separate witness. Compacted and indented existing
encodings are accepted, but duplicate/unknown/aliased keys, missing required
fields and alternate encodings are refused. Reads are bounded to 4 KiB for the
key and 16 KiB for progress; temporary key-containing comparison buffers are
cleared on ordinary return. This is not guaranteed process-memory erasure.

After a new FilePV signature, the exact original state file is reread and
synced, then its containing directory is synced. Original files are checked
again before advancing the separate witness. A final original-file/witness
check precedes returning either a new or reused signature. A reverted or
corrupt disk state cannot be hidden by newer in-memory progress, silently
overwritten by the next vote or bypassed through identical-signature reuse.
Unsafe files, original retention failure/panic and post-retention disagreement
poison the adapter without returning a signature or retrying. There is no
automatic repair; a failure after the first write can leave irreconcilable
local progress pending separately reviewed incident handling.

Direct library users of bare `InitializeSigningGuard`,
`InitializeBoundSigningGuard` and `OpenSigningGuard` retain their narrower
in-memory/witness scope; those functions do not discover original FilePV paths
or supply this file binding. The file-bound API expects an exclusively owned
FilePV configured to write the named original files. It neither initializes
missing files nor resets progress. Offline open/check performs no file or
directory sync and changes no file contents. A stopped check releases these
bindings; the next startup independently acquires and checks them again.

The added retention sequence is a local software check, not certification of
physical power-loss durability, malicious filesystem behavior, whole-host
rollback, remote fencing or an independently retained checkpoint. Matching
bytes on a later open do not authenticate historical key inode continuity.
The existing two-write gap and privileged-owner boundaries remain. Current VPS
keys are not adopted, copied or upgraded, and real-asset readiness stays false.

#### Stopped-workspace signing preflight

After a guarded synthetic validator has fully stopped, the diagnostic can check
its original retained material without starting a node:

```text
godd testnet guard-check --home <stopped-private-validator> --signer-guard <same-retained-directory>
```

The check exclusively acquires the existing home lease and key-wide witness
lease, validates the synthetic configuration and required directory binding,
checks the original key/progress files and compares exact FilePV progress with
the original witness. It reports the
redacted scope and signing height, round and step with
`check: retained-progress-matches`. A cooperating running home or copied key
holding the witness lease is refused. Missing, corrupt, stale, pending,
unmarked, shared or linked material cannot trigger file creation, adoption,
reset or repair. Reopening the signer now requires an existing retained lease file;
only fresh initialization may create that file.

This command opens no application database or network listener and performs no
signature or state update. Private tests compare file contents, permissions,
size, modification time and inode before and after both successful and refused
checks. Filesystem-managed access-time metadata may still change on reads.
A successful result is only point-in-time local signing evidence, not a health
check, restart permit, backup-restore certificate or mainnet readiness verdict.
Every later start acquires its own leases and revalidates the witness. Never
restore, copy or reset an operator's validator keys or signing records based on
this result; independently retained durability and recovery review remain
required. The check cannot authenticate historical inode continuity or detect
lease replacement by the privileged owner. Existing deployed unguarded keys
are not migrated by either command.

Private disk fixtures provision random synthetic keys only in test-owned temporary files. A process restart reopens both the same application state and original file signer. Single-validator normal restart and abrupt process termination have been checked on macOS arm64. Four-validator disk checks cover one stopped validator catching up and the whole network stopping and restarting at different committed heights, with matching state, G, contract storage, replay rejection and newly executed EVM work. A permanently excluded identity also restarts as a following node with its older original signing progress and unchanged penalty custody. The separate file-signer test rejects conflicting same-height data and reuses an identical signature after reload. This does not certify power-loss durability, snapshot rollback, malicious local files, disk corruption or remote signer operations.

Expanded private tests exposed a consensus peer-query routine reading the block database after shutdown closed it, causing `panic: leveldb: closed`. The local checksum-bound lifecycle patch now seals worker admission and joins all three admitted peer-worker classes before the node closes stores. Registration and the stop seal use the same mutex, preventing additions from racing the join. Each worker reports its actual exit; no fixed delay or swallowed database error establishes completion.

`cmd/godbuild` applies only reviewed edits to the exact pinned reactor, with original and patched SHA-256 checks. Generated compiler input, an exact verified module copy and the derived module/overlay configuration remain ignored private build material. Original dependency cache and project module pins remain unchanged. Make targets select this build; the node's join API makes an unpatched build fail compilation. A private controlled-peer fixture blocks six real workers and checks that stores stay open and application close stays prohibited until they exit. Verification scope and results are in [VERIFICATION.md](VERIFICATION.md). This is not operator-grade or adversarial-network shutdown certification. Real assets remain disabled.

The diagnostic reports `local-consensus-execution-prototype`, `localNodePrototype: true`, `nodeReady: false` and `realAssets: false`, with separate implemented synthetic-tool flags. Generic mainnet startup and financial commands remain disabled; explicit `testnet` startup permits only synthetic operation without bridge routes. Only the separate internal synthetic approval-gas mode mounts participant bridge routes; nothing is connected to RH. A four-validator P2P fixture verifies bridge-message propagation, signed result commitments, pending custody, fee/sequence accounting, offline catch-up and restart with each original database and signer. Those checks do not establish RH backing or independent operator acceptance.

`CheckTx` refuses the mixed-state interval after `FinalizeBlock` and before `Commit` refreshes its context. It returns a distinct no-write admission status, not a fee/sequence change or successful transaction. `LocalNode.Submit` maps only that status to `ErrCommitPending`; callers may retry after commitment, but must not blindly retry authentication or policy failures. The check never waits while holding the mempool lock needed by commit. Bounded retry polling exists only in the private test harness; no production sleep or weakened timestamp rule was added.

### Stopped candidate package inspection

`godd testnet upgrade-check` combines the existing stopped signing preflight
with exact private archive/staging checks while retaining both leases throughout.
Original node inputs are compared before and after inspection, and the original
file-bound signer/witness is revalidated before leases are released. A caught
inspection panic or changed input returns no partial report. The read-only
`GuardedSigner.Check` poisons that in-memory adapter after an uncertain retained
state check; it never signs or repairs files.

See [the private command and limits](DEPLOYMENT.md#stopped-candidate-integrity-preflight).
This inspects package integrity only. It does not run the candidate, open a
database, prove retained-state compatibility, migrate or restore anything, or
authorize upgrade/mainnet activation. Original checks remain necessary on every
later start. Same-host privileged control and whole-host rollback remain outside
this cooperating-process protection.

## Local evidence and remaining gates

Local tests exercise all three enabled Ethereum transaction families, signature replay, revert fees, protected module rejection, GOD transfers in both address orders, contract deployment and storage, self-destruction quarantine, signed validator creation, staking delegation and delayed exit, supply conservation and no halt catch-up. Direct application fixtures contain synthetic commit flags and are not signature evidence. The separate four-process network test independently verifies stored cryptographic commits, propagates a signed contract deployment over P2P and compares persisted code and storage across processes. After one validator stops, the other three continue; the absent validator's G does not increase and the actual signers' G does.

Before production, verify adversarial execution and networks, review the full-window downtime convention and every admissible evidence class, approve emission and bridge-gas rules, extend durable signing and recovery to physical failure boundaries, verify source backing and successful delayed bridge payments among independent operators, implement governance, and measure resource use on ordinary computers. Network bridge tests reject early authorization/payment without shortening the 24-hour delay; successful delayed flows are direct synthetic fixtures. Long-period settlement scans, delegation scans and retained tombstones still need performance and growth limits. Local processes on one computer are not geographically independent validators or a demonstrated decentralized launch.
