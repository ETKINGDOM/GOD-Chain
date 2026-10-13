# Parent finality verification boundary

This is a local development component of the RH bridge, not an enabled bridge,
production light client, independent audit or mainnet activation. The network
stack remains God EVM + God SDK + GodCometBFT. All RH finality, signing,
broadcast and real-asset activation gates remain closed.

## Why parent verification is required

Robinhood's official documentation distinguishes sequencer confirmation, batch
submission to Ethereum and Ethereum finality. The canonical bridge's withdrawal
challenge window is a separate condition, not the time required for an Ethereum
block to become final. See [Robinhood transaction finality](https://docs.robinhood.com/chain/transaction-finality/).
Reference reviewed on 2026-10-11; no populated RPC endpoint or contract address
is included here.

A provider's `finalized` tag, agreement between provider responses, valid receipt
proofs against a supplied header and bridge-quorum signatures are not independent
proof of source-chain origin/finality. An authenticated parent block alone also
does not prove an RH execution result. The eventual bridge must bind the reviewed
RH deployment, its batch data and execution or confirmed assertion to the parent
chain, then verify actual token/custody state and one-for-one backing.

## Implemented local API

`internal/godrh/parent_finality.go` provides `VerifyParentFinalityProof`. It has no
HTTP client, automatic anchor discovery, file writer, persistent store, signer,
transaction dispatcher or financial authority. Inputs are typed private material;
ordinary string/debug/JSON representations are redacted. Returned reports contain
only conditional checks, not roots, account identifiers, proof bytes or a reusable
activation certificate.

The caller supplies a separately retained checkpoint root and slot, genesis
validator root, explicit fork name/version and inclusive/exclusive slot interval,
current slot and maximum permitted checkpoint age. This API cannot authenticate
the provenance of those trust inputs or the caller's clock. Selecting these
values from the same untrusted response defeats the trust boundary. Checkpoint
freshness bounds are local refusal policy, not independent weak-subjectivity
validation or approval of the actual Ethereum/RH configuration.

The bounded verifier checks:

- The bootstrap SSZ beacon header matches the exact retained checkpoint.
- The 512-position committee, including its aggregate key, is included in the
  bootstrap state. Every public key is a canonical, non-infinity, prime-subgroup
  BLS12-381 point, and the aggregate matches all committee positions.
- At least 342 committee positions participate. Position bits and repeated
  public keys retain protocol multiplicity; they are not independent operators.
  The signature uses the sync-committee domain, explicit fork version and genesis
  validator root, with canonical non-infinity subgroup checks before pairing.
- The signed attested state contains the supplied finalized beacon-header root.
- The finalized body includes the exact hash of the bounded canonical execution
  RLP header. Deneb/Electra `execution_payload.block_hash` has generalized index
  812: body field 9, payload field 12, in a 32-leaf payload container. Binding the
  full RLP hash also binds its state/receipt/transaction roots and other fields;
  this does not reexecute or independently validate the execution state.
- All slots fit one committee period and one explicitly allowed fork interval,
  with current >= signature > attested >= finalized > checkpoint. Future, stale,
  regressing and cross-period/fork input is rejected. There is no timeout-based
  promotion of optimistic headers.

Only lowercase `deneb` and `electra` are supported. Committee/finality generalized
indices are 54/105 and 86/169 respectively. Every branch has its exact depth;
extra/missing branch items are refused. Fork transitions, previous/future forks,
next-committee updates, forced updates and automatic fallback are unsupported.
This support set is **not** a claim of compatibility with the current live parent
fork or actual RH deployment; those must be independently reviewed before use.

Committee/signature/header fields are fixed width; branches are exactly sized;
the canonical execution header is limited to 4 KiB. Variable-length material is
copied before any caller context callback. Verification is cooperatively
cancellable between bounded operations, not a hard real-time execution deadline.
Malformed input, cancellation or panic returns a zero report and one generic
redacted error, with no partial-success authority or externally visible writes.

Success sets `conditionalParentFinalityVerified` under the **supplied** trust
assumptions. `anchorIndependentlyAuthenticated`, `forkScheduleIndependentlyVerified`,
`clockIndependentlyVerified`, `sourceChainIdentityVerified`, `rhBatchBindingVerified`,
`rhSourceFinalityVerified`, `executionReplayed`, `signingEnabled`, `broadcastEnabled`
and `realAssetsReady` remain false. Existing account/receipt/provider checks are
not silently promoted or connected to this report.

## Reuse and local testing

No dependency pin, lifecycle overlay, checksum or compiler version was changed.
The verifier uses the existing pinned gnark-crypto BLS12-381 primitives and
go-ethereum binary Merkle proof checker. Fixed-width SSZ roots, domains and proof
paths follow the [Ethereum light-client protocol](https://github.com/ethereum/consensus-specs/blob/master/specs/altair/light-client/sync-protocol.md),
[Deneb containers](https://github.com/ethereum/consensus-specs/blob/master/specs/deneb/beacon-chain.md)
and [Electra light-client changes](https://github.com/ethereum/consensus-specs/blob/master/specs/electra/light-client/sync-protocol.md).
Required upstream licenses and attribution remain unchanged. Protocol reuse and
developer tests do not transfer a third-party audit or security assurance.

Private fixtures independently construct SSZ roots/branches with the already
pinned fastssz implementation and generate/verify BLS signatures with blst, rather
than reusing the production gnark verifier to sign its own fixture. Boundary,
tampering, trust/domain, cancellation, detachment, concurrency and fuzz checks
remain private. The fixtures model cryptographic containers, not actual Ethereum
or RH execution, deployed validators or source backing.

## Explicit adjacent-period committee proof

The separate `VerifyParentCommitteeTransitionProof` API accepts one bounded
handoff into the immediately following sync-committee period. The original
`VerifyParentFinalityProof` API still refuses every cross-period input. Neither
API accepts a replacement checkpoint selected by a provider or automatically
updates a persistent light-client store.

The handoff supplies both committee inclusions against the **same original
checkpoint state**: the current committee at generalized index 54/86 and the
next committee at 55/87 for Deneb/Electra. Next-committee proof branches have
exact depth 5/6. Both full committees and their aggregate keys undergo canonical
non-infinity, prime-subgroup and aggregate-multiplicity checks. At least 342 of
512 next-committee positions must sign the attested header with the original
reviewed domain. A valid signature from only the old committee is refused.
These proof paths follow the [official light-client committee rules](https://github.com/ethereum/consensus-specs/blob/master/specs/altair/light-client/sync-protocol.md)
and [Electra state indices](https://github.com/ethereum/consensus-specs/blob/master/specs/electra/light-client/sync-protocol.md).

Finalized, attested, signature and current slots must all be in that exact next
period. This is deliberately stricter than accepting a next-period signature
on an old finalized header. The original checkpoint age remains capped at 8192
slots and the entire proof, including the signature-domain slot minus one, must
fit the same explicitly reviewed fork interval. Missing committee proofs,
skipped periods, stale/future inputs and fork transitions fail without fallback.
Finalized-header and full canonical execution-header-hash inclusion checks remain
unchanged. This is not full block reexecution, source ancestry or an RH assertion.

The report adds only conditional `nextCommitteeIncluded`,
`nextCommitteeAggregateMatched`, `adjacentPeriodSignatureVerified` and
`conditionalCommitteeTransitionVerified` facts. Independently authenticated
anchor/fork/clock, RH source finality, source identity, batch binding, execution,
signing, broadcast and real-asset readiness remain false. No root, committee,
new trust object, file, financial capability or writable state is returned.
Private typed proof representations are redacted; bounded variable slices are
detached before any caller context callback. Cancellation or panic returns a
zero report and the existing generic error.

Local checks independently build a dual-leaf SSZ state and use distinct blst
fixture keys for the next committee. They cover threshold boundaries, incorrect
old-committee signatures, missing/wrong-state/wrong-index proofs, malformed
current and next keys, aggregate mismatch, age/fork/period limits, redaction,
detachment, cancellation, panic and concurrent read-only verification. Use the
unchanged checksum-bound lifecycle build through `make test-parent-finality
check-parent-finality fuzz-parent-transition`. These remain synthetic developer
tests, not independently obtained parent/RH evidence or production acceptance.

This stateless one-period handoff does not establish indefinite synchronization,
renew the initial weak-subjectivity checkpoint or alter bridge admission. The
separate bounded tracker below retains derived heads without extending trust.

## Bounded private head tracking and recovery

`internal/godrh/parent_tracking.go` provides `ParentFinalityTracker`. It is a
bounded conditional proof-chain journal, **not** a complete production light
client. Every new bootstrap must match the retained finalized head; its committee
is proved against that exact state. A proof can use either the unchanged
single-period rule or the adjacent-period handoff. Finalized slots must strictly
increase. Same-slot replays/conflicts, skipped periods, changed original bindings,
regressing clock input and fork transitions are refused without head replacement.

The immutable original checkpoint, genesis, fork interval/version and age policy
remain bound to every snapshot. Derived heads do not refresh weak-subjectivity
trust: the caller's current slot must still be within the **original** age limit
of at most 8192 slots. At most 16 updates and 4 MiB of canonical private snapshot
bytes are accepted. Reaching a bound requires separately reviewed trust/recovery
work, not dropping proof history or silently choosing a new provider anchor.

`Review` detaches all nested proof slices before context callbacks, verifies the
entire candidate chain and performs no writes. A private candidate belongs only
to its original live tracker and exact retained revision. Before `Commit`, the
operator must separately retain both the old snapshot pin and the candidate's
planned pin. Commit requires the same explicit clock slot and exact unchanged
storage revision. Only one competing candidate can advance that revision.
Cancellation before storage writes does not claim a write occurred.

A Save error or panic has an unknown old/new outcome, not an assumed rollback.
It disables the live instance and supplies no retry/reset. Close and reopen must
select an independently retained old or planned pin and reverify every retained
signature, Merkle branch and derived head from the original checkpoint. A matching
checksum alone is insufficient: malformed or cryptographically altered history
is still rejected. The restore clock must pass original-anchor freshness and
cannot be regressed during that live instance.

The pin must not be taken from the same potentially rolled-back storage. Supplying
an old separately retained pin permits historical replay; this component cannot
prove that the caller chose its newest external retention record. Durable
independent monotonic pin retention, clock/anchor provenance, adversarial-host
protection and indefinite synchronization remain operational/release gates.

`OpenPrivateParentFinalityTracker` uses a separate owner-only POSIX database,
exclusive process lock and synchronous atomic writes through the existing pinned
database implementation. It does not create an administrator, copy a signer,
reset node state or share the relay/custody database. Caller supplies an existing
trusted private ignored directory. Windows disk use fails closed pending a
reviewed owner-only ACL implementation; the portable verification API is separate.
The directory check does not certify every ancestor or hostile-owner filesystem
races. Proof bytes and retention pins require explicit private access; ordinary
debug/JSON reports remain redacted and never set RH finality, signing, broadcast
or real-asset readiness.

Local tests exercise independently signed same-period chains, adjacent handoff
followed by a second authenticated update, full reopen replay, stale original
trust despite a newer head, conflicting/repeated heads, external rollback pins,
canonical JSON tampering, uncertain old/new writes, competitive commits, context
cancellation/panics, actual process-locked disk reopen and unsafe directories.
`make test-parent-finality check-parent-finality fuzz-parent-transition
fuzz-parent-tracking` uses the unchanged checksum-bound build. These are synthetic
developer checks, not actual parent/RH evidence, production monitoring or an audit.

## Remaining first-workstream milestones

The offline operator ingress described below makes existing conditional checks
usable from explicitly selected private files. It does not complete any missing
real-network trust, RH execution or financial-admission requirement.

1. Independently authenticate and retain parent-chain identity, a recent trusted
   checkpoint, weak-subjectivity policy, current clock and complete reviewed fork
   schedule. The bounded adjacent-period proof is a local foundation; implement
   continuing authenticated committee/fork updates beyond the bounded tracker,
   independently retained monotonic pins and production recovery without accepting
   a new trust root from the proof.
2. Bind the exact reviewed RH rollup deployment, parent ancestry, batch origin/data
   availability and deterministic execution or confirmed assertions to parent
   finality. A provider finality tag or a quorum statement alone is insufficient.
3. Bind actual token/proxy implementation, upgrade/mint/pause/rebase/transfer
   permissions and custody accounting/storage to authenticated RH state, with
   explicit discrepancy and changing-permission refusal.
4. Connect these verified results to bridge admission through a separately
   reviewed fail-closed capability, persistent replay/conflict reconciliation and
   source-backed reserve checks. No current diagnostic report grants admission.
5. Validate against independently obtained real-network evidence without signing
   or moving assets; review unresolved risks and the exact release configuration
   before any separately authorized activation.

This implementation advances the first workstream; it does not complete it.
It does not require or authorize installation of an RH full node, extra servers,
custody deployment, real-asset transactions, website publication or GitHub upload.

## Offline private material review

A separate typed contract-state API can now bind exact account/code/storage
proofs to the very same canonical execution header verified here, with a separate
block/address/runtime/slot pin and all-or-nothing bounded contract checks. It is
not an RH assertion/batch decoder, source execution replay or authentic deployment
registry. Independent trust, source permissions and financial gates remain false.
See [the launch review](LAUNCH_REVIEW.md#1-bind-contract-state-to-the-parent-execution-header).

`internal/godrh/parent_material.go` and the `godd` command provide a bounded,
read-only ingress to the existing single-period and adjacent-period verifiers.
This is a GOD Chain private review envelope, **not** a raw Beacon REST response,
automatic RPC light-client fetcher or independently authenticated checkpoint.
No library/CLI command chooses trust, computes a review pin for the operator,
infers a clock/fork, updates a journal, signs, dispatches or activates assets.

The command interface is:

```text
godd parent-trust-template
godd parent-proof-template single-period
godd parent-proof-template adjacent-period
godd check-parent-proof <absolute-private-trust-file> <retained-trust-SHA256> <absolute-private-proof-file> <retained-proof-SHA256> <reviewed-current-slot>
```

Templates contain only empty fields/arrays. They are deliberately rejected until
separately reviewed inputs are supplied; they contain no invented checkpoint,
genesis, fork, current slot, committee, signature, endpoint or signing credential.
Each checksum is exactly 64 lowercase hexadecimal SHA-256 characters and must
be retained independently of the unreviewed input. Matching pins establish only
whole-file integrity, not trustworthy provenance, weak subjectivity or approval.
Never publish populated material files, pins or operational configuration.

The trust file is at most 4 KiB. Its exact fields are `purpose` (the template's
literal), `version` (numeric literal 1), `checkpointRoot`, `genesisValidatorsRoot`,
`checkpointSlot`, `maxCheckpointAgeSlots`, `forkName`, `forkVersion`,
`forkStartSlot` and `forkEndSlot`. There is no file-supplied `currentSlot` or
approval flag. Roots are lowercase `0x`-prefixed 32-byte values; fork version is
4 bytes. Fork names/intervals and maximum age retain the existing verifier's
explicit Deneb/Electra and original-checkpoint-age limits. The final command
argument supplies a separately reviewed current slot, not a provider clock.

All uint64 wire values are canonical decimal **strings**, including each
header's `slot`/`proposerIndex` and `signatureSlot`. This preserves values above
JavaScript's safe-integer range. Numeric JSON values, leading zeroes, signs,
exponents, whitespace and overflow are rejected. The `version` field is the
only numeric JSON value. Byte material uses exact-width lowercase hex strings.

The proof file is at most 128 KiB. Both modes require exactly the template's
`purpose`, `version`, `mode`, `bootstrap`, `attested`, `finalized`, `syncCommittee`,
`committeeBranch`, `finalityBranch`, `executionHashBranch`, `participation`,
`signature`, `signatureSlot` and `finalizedExecutionHeader` fields. Each header
has exactly `slot`, `proposerIndex`, `parentRoot`, `stateRoot` and `bodyRoot`.
Each committee has exactly `pubkeys` (512 entries of 48 bytes) and
`aggregatePubkey` (48 bytes). Participation is 64 bytes, signature 96 bytes,
and the canonical execution RLP header is nonempty and at most 4 KiB. Header
byte canonicality, BLS encoding/subgroup/aggregate rules and every proof/signature
still pass through the original cryptographic verifier, not just the JSON parser.

Committee branches contain exactly 5/6 roots and finality branches 6/7 roots
for Deneb/Electra. The execution-hash branch always contains 9 roots. Only
`adjacent-period` additionally requires both `nextSyncCommittee` and
`nextCommitteeBranch` with the same exact committee format and 5/6-root branch.
`single-period` refuses either next-committee field; neither mode auto-falls back.
The next committee still has to be proved in the same original checkpoint state.

Unknown/missing/case-aliased/duplicate members, including escaped duplicates,
nulls, invalid Unicode, excessive nesting, short/extra arrays and trailing JSON
fail closed. Arrays are count-bounded before decoding entries into fixed-width
committees. Public byte review detaches bounded inputs before context callbacks;
cancellation/panic yields a zero report and a generic error without raw material.

Private disk reads reuse the owner-only POSIX reader: absolute clean paths,
trusted existing leaf directory mode 0700, regular same-owner file mode 0600,
one hard link, no leaf/directory symlink, whole-file pin, bounded stable reads.
There is no fallback to public files, shared directories or unreviewed Windows
ACLs. This does not audit all ancestors or defend against the storage owner.
The custody request reader keeps its unchanged 16 KiB limit; shared reader reuse
does not enlarge custody admission. Unsupported disk platforms refuse the command
while the portable bounded byte API remains separate.

Successful output contains conditional booleans and `offlineMaterialsPinned`/
`offlineOnly`, never paths, pins, committee keys, roots, signatures or raw headers.
Independent anchor/fork/clock authentication, RH identity/batch/finality,
execution replay, signing, broadcast and `realAssetsReady` remain false. A
successful exit is not a transferable bridge-admission certificate. This command
does not retain a head or resolve competing updates; the separately bounded
tracker and its external-pin/clock policy remain necessary for stateful review.

Use the checksum-bound `make test-parent-finality check-parent-finality
fuzz-parent-material test-parent-material-cli` targets. The CLI target compiles
only an ignored local diagnostic executable, then runs disposable independent
blst/fastssz fixtures through the real command. It does not execute foreign
builds, install a service, fetch production evidence or publish populated files.
