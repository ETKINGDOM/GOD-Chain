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

## Remaining first-workstream milestones

1. Independently authenticate and retain parent-chain identity, a recent trusted
   checkpoint, weak-subjectivity policy, current clock and complete reviewed fork
   schedule. Implement authenticated committee/fork transitions and durable
   conflict/recovery handling; do not accept a new trust root from the proof.
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
