# GOD Chain bridge prototype

`x/godbridge` and `contracts/GodBridgeEscrow.sol` are ledger and custody components for one-for-one GOD transfer accounting. Explicit synthetic node genesis mounts and initializes the ledger atomically before staking. A separate positive approval-gas policy enables six authenticated participant routes in the local node prototype. Ordinary and ledger-only configurations leave them disabled. Private read-only source configuration, compatibility probes and deposit receipt observation are supplied without configured RH access. No deployment, public RPC, independent finality verifier, relayer, populated signer configuration or real assets are supplied.

## Trust and backing

RH means Robinhood Chain. Independent signers must verify successful finalized source execution, not merely wait a timer. Read-only receipt observation does not implement independent source-finality verification.

Five distinct approvals from seven configured signers authorize bridge actions. Signatures are canonical secp256k1 with low S. This is federated custody trust: a dishonest quorum can attest false source events. Limits constrain exposure but do not remove that trust. Seven keys do not prove independent operators.

Native supply is fixed at 1,000,000,000 GOD with 18 decimals. The keeper transfers existing reserve funds, never mints or burns. Initialization requires the full supply in restricted reserve and no unresolved withdrawals; synthetic node balances cannot be relabeled as backed genesis. Native GOD outside reserve must equal attested deposits minus attested finalized payments. Pending withdrawals stay segregated from spendable balances and G redemption liquidity.

## Private source checks and unsigned deposit proposals

`internal/godrh.Source` supplies replaceable bounded reads for network identity, provider-reported finalized checkpoints, canonical block lookup, runtime code and contract views. `DepositSource` adds transaction receipts. `HTTPSource` permits only five read-only RPC methods, disables ambient proxies, refuses redirects, bounds request/response sizes and pins code/view calls by block hash with `requireCanonical`. It has no signing or submission method. Configuration reserves blank private fields for the network, endpoint, contracts, code pins and seven public signer identities. Template, offline configuration check and explicit source probe commands never activate a chain. Populated files must remain private, ignored and owner-readable; signing-key and standalone credential fields are rejected.

Compatibility probes check 18 decimals, fixed supply, source network, contract code pins, custody token and asset/domain/signer bindings, transfer/exposure/outflow limits, delay and queue bound. All contract views use one provider checkpoint; the final lookup rechecks its original height/hash without rejecting ordinary forward progress. Matching views do not prove immutable token permissions, proxy implementations or independent settlement. Formatting, reports and errors redact operational values.

`ObserveDeposit` requires the compatibility probe and exact expected recipient/amount. It checks successful execution, matching transaction identity, an explicit block-wide log index, the canonical receipt block at or below a provider-reported finalized checkpoint and both reviewed runtime-code pins at that receipt block. The selected log must be from the configured custody contract and match the exact `Deposited(uint64,address,bytes20,uint256)` ABI, sequence/sender encoding, recipient padding and transfer bound. The RPC log index is not a receipt-array offset. Before returning, it rechecks receipt block, original checkpoint and source network. Local admission bounds are 128 logs per receipt, 4,096 data bytes per log and a 512 KiB RPC response; ambiguous selected fields, mismatched metadata, removed/duplicate/unordered logs and malformed quantities fail closed.

The detached `Proposal()` output contains a private deposit, its existing protocol digest and the provider checkpoint. It is unsigned and must not be logged or published. `independentFinalityVerified`, `approvalReady` and `realAssetsReady` remain false even with complete production-mode configuration. A provider can fabricate consistent receipts and block lookups. No receipt inclusion proof, independent settlement verification, event-discovery cursor, replay acceptance, signing, relaying or native balance change occurs here. Repeated observation is not repeated ledger acceptance; the ledger retains its separate replay rules. Independent signers, actual token review, source-backed genesis and initial fees, recoverable relay, production node services, governance and explicit authorization remain activation gates. A token CA alone cannot satisfy them.

## State transitions

A finalized deposit quorum releases the exact matched reserve amount once. The isolated authenticated adapter binds withdrawal ownership to the native sender before locking GOD; direct keeper calls remain trusted internal APIs. FIFO authorization requires the 24-hour delay and available rolling capacity. Source payment requires matching approvals and exact custody decrease and recipient increase. Finalized payment returns the locked native GOD to reserve.

Payment and permanent cancellation are mutually exclusive. Only a finalized irreversible cancellation tombstone permits native refund; timeout or cancellation-request signatures alone cannot. Replay binds the asset domain, source transaction and log index, so re-inclusion under a different block hash cannot credit twice. Pauses only restrict directions; they cannot extract assets or resume operations.

Prototype limits are 10,000 GOD exposure, 1,000 GOD per transfer and 1,000 GOD outflow per rolling 24 hours. Midnight is not a reset. Native authorization and source payment reserve capacity at their own timestamps; cancellation does not return native rolling capacity. Queue span and recent-window records are bounded to 1,024 for resource control, not measured production gas capacity. Permanent replay records still grow.

Custody rejects failed, false-return and inexact token transfers. Raw donations do not create escrow credit. Supply and decimals are checked, but actual issuer permissions, rebasing, upgrade powers and RH compatibility require separate review. There is no administrative withdrawal, mint, custody upgrade, signer rotation or resumption path.

## Authenticated transaction adapter

The bridge schema defines six messages: accept deposit, request withdrawal, authorize withdrawal, acknowledge payment, acknowledge cancellation and restrict bridge directions. The outer sender uses native direct signing with chain ID, account number and ordered sequence. Withdrawal ownership comes from that authenticated sender. The other five operations also require distinct valid quorum approvals over their action-specific content; an outer signature alone cannot release reserve or refund funds.

`NewBridgeEncoding` is an explicit native-only opt-in; `NewBridgeExecutionEncoding` is the combined node opt-in. The normal native and execution codecs reject bridge envelopes. The native-only application remains unmounted; the local node mounts routes only with explicit positive bounded approval gas and authorized synthetic bridge genesis. Server construction requires a configured keeper and charges every supplied approval. No production gas default is selected. Direct server methods must not be exposed as unsigned public gRPC.

Canonical GOD accounts, positive bounded integer amounts, nonzero source recipients, exact hash lengths, positive sequences and bounded approval shapes are checked before keeper execution. Structure does not prove source finality. Ante rejection commits no fee or sequence. Accepted business failure or message-stage gas exhaustion retains the signed fee and sequence while reverting the entire business batch. An incoming unreleased deposit cannot finance its own sender's fee.

## Initial reserve reconciliation

`InitBootstrap` is a trusted initialization-only API, not a participant message or a migration. The optional synthetic node calls it at genesis. It requires height zero, the configured native chain and exact genesis time, an empty bridge store and the entire fixed supply in reserve. Credited accounts must not preexist; initialized or synthetic allocations cannot be reset or relabeled as backing.

The complete source-deposit sequence must start at one, contain no gaps or repeated events and agree with the checkpoint's heights and block identities. Its sum must equal attested credited escrow, not raw token balance. The checkpoint additionally asserts no prior source payment or cancellation outcomes. The code cannot independently verify these source statements; a quorum must do so.

Every deposit keeps its own quorum approvals. The complete canonical plan separately needs five to seven valid distinct approvals, binding configuration, chain and asset domains, genesis time, checkpoint, ordered deposits, validator owners and keys, self-stake and fee budgets. Every funded account requires a positive fee budget; each unique validator owner must cover at least 1,000 GOD self-stake plus its budget from its own deposits. Budgets describe intended uses, not locked funds, sponsorship or measured gas sufficiency. Key shape does not prove possession or owner consent.

An outer cache makes initialization, account creation, reserve releases and replay records atomic. Fresh base accounts receive exact deposit-funded balances. A hash-bound initial reconciliation record survives committed disk reopen and detects missing or altered records. It describes initial funding, not current escrow or spendable fee balances. The API neither stakes funds nor installs validators.

Prototype input bounds are 256 deposits, 256 funded accounts and 32 planned validators. Existing exposure limits are unchanged: 10,000 GOD cannot fund ten 1,000-GOD validators plus positive fee budgets. No 32-validator launch is established. Private fixtures stake reconciled synthetic balances through the real keeper without changing supply. Actual source finality and production backing remain integration gates.

## Node authorization and atomic initialization

Explicit `Config.BridgeGenesis` requires complete-plan and individual-deposit quorums, exact participant/validator funding and separate node-policy approvals. Every validator supplies account-owner consent and consensus-key possession in distinct domains; neither is a consensus vote. The node digest binds canonical configuration, consensus parameters, exact genesis time and actual execution, fee, staking, penalty and bridge policy. Missing, repeated or invalid proofs reject initialization before engine assignment.

The node initializes full restricted reserve rather than preallocating participant declarations, then calls `InitBootstrap` and applies self-staking. Account, bank, bridge replay, staking, rewards, execution and metadata writes share one cache. Failures discard the financial batch; internally failed ABCI instances cannot commit or retry sealed execution globals. The authorization digest and immutable bootstrap record persist across blocks and reopen. Replayed deposits and repeated initialization cannot release reserve twice. Snapshots and block boundaries check aggregate reserve and pending custody.

Ledger-only initialization uses runtime binding version 3 and node digest domain version 2; the certificate schema remains version 1. Earlier gate-only data/signatures must not be silently reinterpreted. Ordinary nil-policy nodes retain version-2 binding without a bridge store. Zero approval gas leaves participant envelopes and routes disabled. No migration, signer reset, signing service or populated certificate is supplied. This is synthetic accounting, not independent RH source verification.

## Authenticated node integration

Positive `Config.BridgeApprovalGas` requires `BridgeGenesis`, fits signed gas arithmetic and stays within the configured transaction gas bound for the signer count. Runtime binding version 4 and the node authorization digest bind the exact gas setting. A re-signed certificate cannot replace committed genesis or silently enable routes over ledger-only data. The six routes retain authenticated outer ownership and sequence, separate action-specific quorum checks and cached business execution. Fees and sequence persist on accepted business failure; failed batches retain no successful business prefix. EVM, G, staking and bridge messages share the existing authenticated transaction boundary.

Four local persistent validators accepted signed deposits, withdrawal locks, finalized-cancellation acknowledgements and pauses over P2P. State, fees and sequences matched, with transaction/result commitments, following-header linkage and actual signatures independently checked by private fixtures. Three validators continued while one was offline; its original database and file signer caught up. Whole-network restart preserved pending custody, immutable initialization, replay records and continued EVM execution.

Early authorization and queued payment were rejected. The network fixture did not wait 24 hours: successful delayed authorization and payment were checked only in direct synthetic-node and isolated custody fixtures. Those direct checks use synthetic consensus inputs, not a delayed four-node transfer or RH source proof. The private single-transaction receipt helper is not a production proof endpoint.

Checking during the interval between finalization and commitment returns a distinct no-write deferral, avoiding newer ledger state with an older checking timestamp. `LocalNode.Submit` reports `ErrCommitPending` without internal retry; the check does not wait under the mempool lock. Authentication, actual time-regression rejection and withdrawal delay remain unchanged.

`AuthorizationAttestationDigest`, `ResolutionAttestationDigest` and `PauseAttestationDigest` build exact unsigned proposals from explicit configuration without ledger context, bank access or state mutation. They do not determine current eligibility, approve an action or verify source execution.

## Packet encoding

Packets use SHA-256, not personal signing, EIP-712 or JSON. Integers are unsigned big-endian. ASCII chain names and action tags use uint16 byte-length prefixes. Addresses are raw 20 bytes; amounts and asset bindings are 32 bytes; sequences and heights are uint64; log indexes and nanoseconds are uint32. Signatures are 65 bytes with recovery ID 0 or 1. Supplied approvals must all be valid and distinct.

The domain binds `GOD Chain bridge attestation`, version, source/native names, asset binding and action. Distinct actions identify deposits, withdrawals, authorizations, cancellation requests, finalized outcomes, event identity, pauses and initial reconciliation. Withdrawal identity includes sequence, sender, recipient, amount and exact queue time. Resolution adds source height, block hash, transaction hash and log index.

The `genesis-bootstrap` packet appends a SHA-256 of canonical configuration, uint32 plan version, genesis seconds/nanoseconds, checkpoint height/block hash/next deposit/credited amount and a one-byte no-outcomes assertion. Three uint32-length lists follow: deposit digests, owner/public-key/stake tuples and owner/fee-budget tuples. Deposits sort by sequence; validator and budget lists sort by raw owner bytes. Approval byte order is excluded, not the content approved. This digest is native initialization input, never a custody payment packet.

The asset domain binds `GOD Chain bridge asset`, version 1, source chain ID, token, custody instance and decimals. A fresh custody deployment has a different binding, not permission to reset replay state. No populated domain configuration is included.

## Verification and integration gates

Private checks exercised compiled custody bytecode and native stores with synthetic assets, including conservation and hostile token behavior. Compiler provenance is in [UPSTREAM.lock.json](UPSTREAM.lock.json): Solidity 0.8.37, optimizer 200, no IR and a Cancun local execution target. Private tests, synthetic-token source, compiler files, artifacts and operational scripts are not distributed. Public Go builds do not rerun those custody checks. See [VERIFICATION.md](VERIFICATION.md).

Private signed-message checks cover ownership, envelope tampering, quorum verification, fees, gas, rollback and disk replay. Seven initial reconciliation tests cover complete funding, real staking custody, plan binding, canonical ordering, atomic rejection and record integrity. None fetches an actual RH checkpoint or proves source backing.

Successful delayed network bridge payments, production-backed validators/genesis, finalized-source observation, independent signer operations and recoverable relaying remain required. The checksum-bound shutdown repair and admission guard passed local regression checks, not operator-grade reliability or throughput certification. Custody continuity and governance need additional design. Deployment, bridge funding and real-asset activation require separate explicit authorization.
