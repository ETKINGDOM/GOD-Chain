# GOD Chain bridge prototype

`x/godbridge` and `contracts/GodBridgeEscrow.sol` are ledger and custody components for one-for-one GOD transfer accounting. Authenticated message adapters are tested separately; explicit synthetic node genesis now mounts and initializes the ledger atomically before staking. Participant bridge routes remain disabled and nothing connects to Robinhood Chain. No deployment, relayer, observer, populated signer configuration or real assets are supplied.

## Trust and backing

RH means Robinhood Chain. Independent signers must verify successful finalized source execution, not merely wait a timer. The finalized-source observer remains unimplemented.

Five distinct approvals from seven configured signers authorize bridge actions. Signatures are canonical secp256k1 with low S. This is federated custody trust: a dishonest quorum can attest false source events. Limits constrain exposure but do not remove that trust. Seven keys do not prove independent operators.

Native supply is fixed at 1,000,000,000 GOD with 18 decimals. The keeper transfers existing reserve funds, never mints or burns. Initialization requires the full supply in restricted reserve and no unresolved withdrawals; synthetic node balances cannot be relabeled as backed genesis. Native GOD outside reserve must equal attested deposits minus attested finalized payments. Pending withdrawals stay segregated from spendable balances and G redemption liquidity.

## State transitions

A finalized deposit quorum releases the exact matched reserve amount once. The isolated authenticated adapter binds withdrawal ownership to the native sender before locking GOD; direct keeper calls remain trusted internal APIs. FIFO authorization requires the 24-hour delay and available rolling capacity. Source payment requires matching approvals and exact custody decrease and recipient increase. Finalized payment returns the locked native GOD to reserve.

Payment and permanent cancellation are mutually exclusive. Only a finalized irreversible cancellation tombstone permits native refund; timeout or cancellation-request signatures alone cannot. Replay binds the asset domain, source transaction and log index, so re-inclusion under a different block hash cannot credit twice. Pauses only restrict directions; they cannot extract assets or resume operations.

Prototype limits are 10,000 GOD exposure, 1,000 GOD per transfer and 1,000 GOD outflow per rolling 24 hours. Midnight is not a reset. Native authorization and source payment reserve capacity at their own timestamps; cancellation does not return native rolling capacity. Queue span and recent-window records are bounded to 1,024 for resource control, not measured production gas capacity. Permanent replay records still grow.

Custody rejects failed, false-return and inexact token transfers. Raw donations do not create escrow credit. Supply and decimals are checked, but actual issuer permissions, rebasing, upgrade powers and RH compatibility require separate review. There is no administrative withdrawal, mint, custody upgrade, signer rotation or resumption path.

## Authenticated transaction adapter

The bridge schema defines six messages: accept deposit, request withdrawal, authorize withdrawal, acknowledge payment, acknowledge cancellation and restrict bridge directions. The outer sender uses native direct signing with chain ID, account number and ordered sequence. Withdrawal ownership comes from that authenticated sender. The other five operations also require distinct valid quorum approvals over their action-specific content; an outer signature alone cannot release reserve or refund funds.

`NewBridgeEncoding` is an explicit native-only opt-in. The normal native and execution codecs reject bridge envelopes, and neither application mounts the adapter. Server construction requires a configured keeper and an explicit positive bounded gas cost per supplied approval. No production gas default is selected. Direct server methods must not be exposed as unsigned public gRPC.

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

This mode uses runtime binding version 3 and node digest domain version 2; the certificate schema remains version 1. Earlier gate-only data/signatures must not be silently reinterpreted. Ordinary nil-policy nodes retain version-2 binding without a bridge store. No migration, signer reset, signing service or populated certificate is supplied. This is synthetic accounting, not independent RH source verification. Node participant bridge envelopes and routes remain disabled.

## Packet encoding

Packets use SHA-256, not personal signing, EIP-712 or JSON. Integers are unsigned big-endian. ASCII chain names and action tags use uint16 byte-length prefixes. Addresses are raw 20 bytes; amounts and asset bindings are 32 bytes; sequences and heights are uint64; log indexes and nanoseconds are uint32. Signatures are 65 bytes with recovery ID 0 or 1. Supplied approvals must all be valid and distinct.

The domain binds `GOD Chain bridge attestation`, version, source/native names, asset binding and action. Distinct actions identify deposits, withdrawals, authorizations, cancellation requests, finalized outcomes, event identity, pauses and initial reconciliation. Withdrawal identity includes sequence, sender, recipient, amount and exact queue time. Resolution adds source height, block hash, transaction hash and log index.

The `genesis-bootstrap` packet appends a SHA-256 of canonical configuration, uint32 plan version, genesis seconds/nanoseconds, checkpoint height/block hash/next deposit/credited amount and a one-byte no-outcomes assertion. Three uint32-length lists follow: deposit digests, owner/public-key/stake tuples and owner/fee-budget tuples. Deposits sort by sequence; validator and budget lists sort by raw owner bytes. Approval byte order is excluded, not the content approved. This digest is native initialization input, never a custody payment packet.

The asset domain binds `GOD Chain bridge asset`, version 1, source chain ID, token, custody instance and decimals. A fresh custody deployment has a different binding, not permission to reset replay state. No populated domain configuration is included.

## Verification and integration gates

Private checks exercised compiled custody bytecode and native stores with synthetic assets, including conservation and hostile token behavior. Compiler provenance is in [UPSTREAM.lock.json](UPSTREAM.lock.json): Solidity 0.8.37, optimizer 200, no IR and a Cancun local execution target. Private tests, synthetic-token source, compiler files, artifacts and operational scripts are not distributed. Public Go builds do not rerun those custody checks. See [VERIFICATION.md](VERIFICATION.md).

Private signed-message checks cover ownership, envelope tampering, quorum verification, fees, gas, rollback and disk replay. Seven initial reconciliation tests cover complete funding, real staking custody, plan binding, canonical ordering, atomic rejection and record integrity. None fetches an actual RH checkpoint or proves source backing.

Authenticated node bridge transactions, production-backed validators/genesis, finalized-source observation, independent signer operations and recoverable relaying remain required. The checksum-bound shutdown repair passed local regression checks, not operator-grade reliability. Custody continuity and governance need additional design. Deployment, bridge funding and real-asset activation require separate explicit authorization.
