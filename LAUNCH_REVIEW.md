# GOD Chain launch review foundations

This local candidate advances three workstreams: authentic RH bridge evidence,
source-token/custody review, and explicit economics/funding review. None is
complete. No production configuration, live service, public website, deployment,
custody, signing key, balance or GitHub publication is changed by these tools.
The stack remains God EVM + God SDK + GodCometBFT.

## 1. Bind contract state to the parent execution header

`VerifyContractStateProof` checks a canonical execution header against a
separately retained block height/hash, account inclusion against its state root,
exact deployed runtime against both the account code hash and a separate runtime
pin, and exact supplied storage words against that account's storage root.
Account keys and storage keys use the pinned upstream Keccak/MPT verifier.
The [account/storage proof standard](https://eips.ethereum.org/EIPS/eip-1186)
describes these root commitments; RPC metadata does not replace them.

`VerifyParentDeploymentState` additionally requires all supplied contract proofs
to use the **exact execution-header bytes** verified by the existing conditional
parent committee/finality proof. It accepts the original single-period proof or
an explicitly supplied adjacent-period handoff; there is no old-key fallback.
The separately retained block pin is still required. All checked contracts must
share this same execution root, with no duplicate address. One inconsistent
contract rejects the entire result without a partial success report.

Limits are four contracts, eight distinct slots per contract, 256 KiB total
material per contract, 128 KiB runtime, a 4 KiB execution header, and at most 65
root-to-leaf nodes per account/storage proof, each no larger than 1 KiB. The
account-proof component also retains its existing 64 KiB bound. Duplicate,
reordered, unused, missing, malformed or hash-inconsistent nodes fail closed.
All variable material is detached before caller context callbacks. Cancellation
and recovered panics return a static error and a zero report.

Storage expectations are private review inputs, not guessed Solidity layout.
A word is zero only after a complete non-inclusion proof, or an authenticated
empty storage root with no supplied nodes. Stored zero, leading-zero scalars,
oversized words and noncanonical RLP are rejected. No proxy/role slots are
automatically assigned. Even checking the standard proxy slots described in
[EIP-1967](https://eips.ethereum.org/EIPS/eip-1967) would not rule out other
upgrade, delegate-call or permission mechanisms.

Remaining: independently authenticated checkpoint/fork/clock provenance,
continuing fork-aware synchronization, reviewed actual RH deployment/code/layout
bindings, Nitro assertion/batch ancestry, RH execution/state verification and
production admission. These proof verifiers have no RPC fetcher, state store, signer,
broadcaster or financial capability. Accounts bound to a conditional parent root
are **not** proof that an RH token deposit is finalized or that custody is safe.
See [PARENT_FINALITY.md](PARENT_FINALITY.md).

## 2. Bind token evidence without inventing permission conclusions

`VerifyTokenStateProof` requires the already parsed private token source capsule,
separately retained compiler/runtime pins and exact chain/token configuration.
Its declared runtime must equal the proof's runtime and account code hash at the
retained RH header. Each storage expectation must be supplied from a separately
reviewed source layout. There is no generic guess for `totalSupply`, owner,
minting, roles, proxy implementation or custody balances.

This does not execute Solidity, recompile the capsule, authenticate its source
provider, authenticate the RH header, prove immutable supply or approve token
permissions. Existing explicit pinned-local-compiler reproduction remains a
separate check; a declared or manually constructed compilation report is not an
approval token. Matching runtime can still contain unsafe mutable behavior.

### Offline token-state operator workflow

```text
godd rh-token-state-template
godd check-rh-token-state <absolute-private-config> <config-SHA256> <absolute-private-bundle> <bundle-SHA256> <absolute-private-proof> <proof-SHA256> <compiler-SHA256> <reviewed-block-height> <reviewed-0x-block-hash>
```

The template is intentionally unusable, with no fabricated header, runtime,
account nodes or storage slots. A completed proof file has exactly `purpose`,
`version`, `header`, `runtime`, `accountNodes` and `slots`. Purpose is the exact
template value and version is numeric `1`. Proof bytes are nonempty canonical
lowercase `0x` hex strings. Each optional slot has exactly `key`, `value` and
`nodes`; the key/value are 32-byte words and nodes is a bounded array. Slot keys
must be distinct. An empty `slots` array requests **code/account checking only**,
not a permission or total-supply check. Empty storage nodes are valid only when
the subsequent verifier authenticates an empty storage root; nonempty roots
require complete inclusion or non-inclusion proofs even for an expected zero.

The expected block height/hash are separately reviewed command inputs, not
claims inside the proof file. Height is a positive canonical uint64 decimal
string; the hash is nonzero lowercase `0x` plus 64 hex digits. It is an execution
block hash, not a SHA-256 file checksum. Each file's independent SHA-256 pin is
64 lowercase hex digits without `0x`; none is inferred from a provider. A file
pin proves retained integrity, not independent chain identity or finality.

All three inputs must be regular single-link owner-held `0600` files within
owner-held `0700` private leaf directories on Linux/macOS. Source capsules keep
their 1 MiB bound; config remains 16 KiB. The proof JSON is at most 544 KiB,
decoded proof material at most 256 KiB, runtime at most 128 KiB, and header at
most 4 KiB. Each account/storage path has at most 65 nodes of at most 1 KiB,
with the separate 64 KiB account component budget retained. Eight optional
slot bindings are supported. The parser rejects aliases, duplicate/unknown
keys, nulls, malformed hex, duplicate slots and oversized nested arrays.

The workflow reads the exact pinned snapshots, binds the parsed capsule and
configured token/runtime to the proof, verifies account and optional storage
paths, then reads every original file again before emitting a redacted report.
Failure returns no partial success. It does not run a compiler, connect to a
source, infer layout or certify permissions. Compiler execution, independent
header/source identity, RH finality, signing, broadcasting and real assets stay
false even after a successful conditional state match. No journal, custody
record or release configuration is changed. This check cannot replace original
creation reproduction or independently authenticated RH settlement evidence.

Use `test-launch-foundations`, `test-token-state-cli`, `test-token-state-evm`,
`check-launch-foundations` and `fuzz-launch-foundations` through the
checksum-bound lifecycle. Private synthetic proofs and locally compiled CLI
tests are not actual deployed-token evidence. Windows compilation does not
enable the POSIX private-file workflow or certify Windows runtime behavior.

### Explicit read-only token-state acquisition

```text
godd rh-token-state-read-template
godd read-rh-token-state <absolute-private-config> <config-SHA256> <absolute-private-bundle> <bundle-SHA256> <absolute-private-read-request> <request-SHA256> <compiler-SHA256> <reviewed-block-height> <reviewed-0x-block-hash>
```

This is a separate, explicitly requested network read, not the offline proof
command. `HTTPTokenStateSource` contacts only the endpoint in the pinned private
configuration. Production mode requires HTTPS and is **diagnosis only**;
simulation mode allows literal-loopback HTTP for disposable local fixtures.
The client has no ambient proxy, redirect following, retries, debug namespace,
compiler, head selector, signer, broadcaster or persistent state.

The private request has exactly `purpose`, `version`, `header` and `slots`.
Use the exact template purpose and numeric version `1`. Supply separately
retained canonical execution-header bytes as lowercase `0x` hex, and up to eight
distinct source-reviewed slots with exactly `key` and `value`, both fixed
32-byte hex words. The positive height and nonzero execution block hash are
separate reviewed command inputs. The request is at most 16 KiB; the header is
at most 4 KiB. A blank template fails before networking. Empty slots explicitly
request account/code checking only, never a supply or permission conclusion.
The command does not discover storage layout or fetch a trusted header for you.

The successful HTTP workflow makes eight requests using only four methods:

- Two `eth_chainId` checks, before and after acquisition.
- Four `eth_getBlockByNumber` reads at the fixed reviewed height, checking the
  exact hash before/after the outer workflow and the code/proof read.
- One `eth_getCode` and one `eth_getProof`, both pinned to
  `{blockHash, requireCanonical: true}`. The proof request names only the
  configured token and the exact supplied storage keys.

The [hash-pinned block interface](https://eips.ethereum.org/EIPS/eip-1898)
defines that selector. A provider that does not support it or cannot serve the
proof causes a failure; there is no height-only, `latest`, `pending` or
`finalized` fallback. Block/chain changes, code mismatches, RPC errors, malformed
or ambiguous JSON, missing/reordered/extra slots and incomplete proofs return a
static failure with no partial success report. Account metadata is checked
against the proved account. Slot metadata is checked against the requested
word and the actual root-bound MPT proof, including authenticated absence.
Returned slot keys may be canonical uint256 quantities or fixed 32-byte data;
values must be canonical quantities. Other aliases and padding are refused.

HTTP requests have ten-second timeouts within one thirty-second acquisition
context. Each proof response envelope is bounded to 544 KiB; decoded material
retains the 256 KiB aggregate, 128 KiB code, 64 KiB account component, and
65-node/1 KiB-per-node path limits. Other RPC responses retain their existing
512 KiB limit. Injected `TokenStateSource` implementations must honor context
cancellation and must not concurrently mutate returned material; they are not
an OS sandbox. The workflow detaches headers, slots and returned proof bytes
before further source/context callbacks.

All three owner-only input snapshots and independent pins use the same POSIX
protections as the offline workflow and are rechecked after acquisition.
Configuration remains 16 KiB and source capsules 1 MiB. CLI stdout is a redacted
report only; it does not save raw provider data, a proof artifact, a journal,
an approval file or new configuration. An explicit in-process `Material()` call
returns detached private proof bytes for further review, not release authority.

Success means only that the configured provider's chain/block lookups and
code/account/storage proof matched these retained inputs. A provider can
fabricate a consistent chain and header together. Independent source identity,
authenticated RH settlement/finality, compiler execution, token permissions,
custody backing, signing, broadcast and real-asset readiness remain false.
This does not replace conditional parent verification or actual RH execution
and ancestry verification, and it cannot activate the bridge.

Use `test-token-state-reads`, `test-token-state-cli`, `test-token-state-evm`,
`check-launch-foundations`, `fuzz-token-state-reads` and the full retained
`test-parent-finality-integration` regression through the checksum-bound build.
No actual RH endpoint or deployed token evidence is established by these local
fixtures. Keep requests, operational endpoints and proof bytes private.

The actual issuance evidence dossier still needs:

- Complete original source, compiler/settings/input inventory, constructor and
  initial allocation, then exact runtime reproduction.
- Mint/supply authority, ownership/roles, proxy/beacon/upgrade authority and any
  pause, blacklist, tax, rebase, hooks or nonstandard transfer behavior.
- Source-reviewed storage bindings at an authenticated state root, exact custody
  compatibility, actual deployed custody review and one-for-one backing evidence.

An explorer access failure or a missing public source record proves neither
that a contract is safe nor that it is unsafe. An issuance page or complete
source/compilation capsule is still required for the actual token review. Keep
the deployed token/custody addresses and populated materials out of public
source/history; never provide wallet secrets. See [BRIDGE.md](BRIDGE.md).

### Candidate launch-platform sources, not deployed-token identification

The platform's published sources contain different token generations:

- [Pons V1 token source](https://github.com/ponsdotdev/pons-labs/blob/main/contractsV1/src/PonsLauncherToken.sol)
  contains constructor immutables, an initial mint to the deploying factory,
  launch-window pool-buy restrictions and a factory-only seed-buy exemption.
- [Pons V2 token source](https://github.com/ponsdotdev/pons-labs/blob/main/contractsV2/src/v2/PonsV2LauncherToken.sol)
  contains constructor immutables, an initial mint to a supplied curve and
  `ERC20Burnable` inheritance. Holder burning can reduce supply in this candidate
  generation; no claim is made that the actual GODCOIN deployment uses it.

These moving-branch references are discovery material, not retained release
pins or proof of the deployed token's generation. Complete dependencies,
creation/factory evidence, compiler settings, constructor inputs and exact
runtime must be bound to the actual deployment. Platform branding is not enough.

The ordinary recompilation gate rejects nonempty immutable/link references.
The separate [constructor compatibility APIs](TOKEN_CREATION.md) now compile
unlinked Cancun creation code and replay explicit constructor inputs in a
disposable local VM, requiring exact runtime and CREATE/CREATE2 address matches.
Original creation provenance remains separate; do not ignore immutable bytes
or accept wildcard matches. If the actual deployment supports
burning, its compatibility with the current exact fixed-supply checks needs an
explicit review. No supply check or economic rule is weakened by this candidate.

## 3. Explicit economics and initial funding candidate

`CurrentEconomicBaseline` exposes detached descriptions of existing prototype
constants, directly from the current staking, penalty, G and bridge policies.
It does not change those constants or select production parameters.

| Current prototype item | Value; still subject to production approval |
| --- | --- |
| Fixed GOD economic supply | 1,000,000,000 GOD, 18 decimals; no extra supply from bridging |
| Minimum validator self-stake / maximum active set | 1,000 GOD / 32 validators |
| Unbonding / commission | 21 days / fixed 10% current registration commission |
| Signing policy | 10,000 expected opportunities, 90% minimum signing, one-hour suspension |
| Penalties | 5% double-sign penalty; zero downtime slash; GOD penalties are not supply burning |
| G | No lifetime cap; current network ceiling 10,000 G per UTC day; per-block budget explicit |
| Redemption | Voluntary; `floor(poolGOD * offeredG / outstandingG)`; burn G only after success |
| Empty pool | No redemption payment; preserve G and its other supported uses |
| Bridge | 5 distinct keys of 7; 10,000 GOD exposure, 1,000 GOD transfer/daily outflow, 24-hour return delay |

Multiple keys or validator identities held by the founder do not create
independent custodians. The selected initial topology is one founder-operated
physical host, potentially multiple isolated processes, without whole-host
failover. The review reports this as a **declaration**, not independent evidence
of hosting, key ownership or decentralization. Authority remains founder-controlled
until a separately reviewed transition; the tool creates no upgrade/pause grant.

The offline commands are:

```text
godd launch-economics-template
godd check-launch-economics <absolute-private-candidate-file> <retained-SHA256>
```

The template declares one physical host and one operator custodian. Validator
count, emission, block-cadence assumption, fee floor, startup GOD, working Gas,
redemption seed and RH-side ETH fee budget remain **blank**, not test defaults.
Fill these only as a reviewed candidate. Exact required fields are the template's
fields; `version` is numeric 1, all quantities are canonical decimal strings.
No unknown/missing/duplicate/case-aliased fields, nulls, floats, signs, exponents,
whitespace, leading zeros, invalid Unicode or injected approval/credential fields
are accepted. Zero is allowed for declared budgets/seed, but missing working Gas
or source fees makes the arithmetic plan inconsistent. An explicit zero seed is possible
without a promised GOD redemption payment.

Review is bounded to a 16 KiB owner-only private regular POSIX file with a
separately retained whole-file SHA-256. The original pin is rechecked after
review. There is no write, approval file, network lookup, mainnet configuration
generation, service restart or activation. A syntactically valid inconsistent
plan prints its redacted checks but returns a failing command exit status.
Consistent arithmetic still reports `ownerApproved`, `sourceBackingVerified`,
`productionConfigurationApplied` and `realAssetsReady` as false.

The checker uses wide integer arithmetic:

- Minimum stake need = chosen initial validators × current minimum self-stake.
- Minimum planned startup GOD = stake need + working Gas + voluntary seed.
- Nominal daily G = `ceil(86400 / assumedBlockSeconds)` × network per-signed-block
  G budget, capped by the existing network daily ceiling. This is an assumption,
  not a measured block rate, exact epoch projection or guaranteed reward.
- The 21,000-gas plain-transfer fee floor is only an arithmetic illustration,
  not a current fee-market quote, native staking Gas estimate, contract estimate
  or complete source-chain fee budget.

For example, 1 validator needs at least 1,000 GOD staked; 4 need 4,000 GOD; 32
need 32,000 GOD **before** working Gas or seed. The last example exceeds the
current 10,000 GOD bridge exposure and must not silently trigger a limit increase.
Even a smaller mathematically compatible budget must come from proven source
deposits and the fixed restricted reserve, not minting or converted test balances.
The declared RH ETH fee budget is separate and remains unmeasured/unverified;
the [official connection guide](https://docs.robinhood.com/chain/connecting/)
identifies ETH as the source network Gas asset.

Remaining owner decisions: exact validator count, network G budget and cadence,
Gas floor/limits, source-backed startup allocation, optional seed, source ETH fee
budget, and explicit review of existing stake/commission/penalty/bridge controls.
Actual key custody, signing/upgrade/emergency procedures, deposit backing and
the exact immutable production candidate need separate evidence and approval.

## Local checks and publication boundary

Use checksum-bound `test-launch-foundations`, `check-launch-foundations` and
`fuzz-launch-foundations`; actual offline command round trips use
`test-launch-economics-cli` and `test-parent-material-cli`. Full RH/CLI regression
uses `test-parent-finality-integration`. The private proof fixtures use synthetic MPT
state and independently generated blst/fastssz parent signatures. They are not
actual RH evidence. Foreign compilation is not target-machine runtime acceptance.

These additions are local candidate source, not a GitHub release. Before a later
authorized upload, review every added/imported production file in the isolated
GOD Chain publication allowlist. Do not upload private tests, populated evidence,
operations, keys, endpoints or development history. Publication, website updates,
mainnet launch and real-asset operations remain separately authorized actions.
