# GOD Chain bridge prototype

`x/godbridge` and `contracts/GodBridgeEscrow.sol` are isolated ledger and custody components for one-for-one GOD transfer accounting. They are not mounted in the local node and do not connect to Robinhood Chain. No deployment, relayer, observer, signer configuration or real assets are supplied.

## Trust and backing

RH means Robinhood Chain. Independent signers must verify successful finalized source execution, not merely wait a timer. The finalized-source observer remains unimplemented.

Five distinct approvals from seven configured signers authorize bridge actions. Signatures are canonical secp256k1 with low S. This is federated custody trust: a dishonest quorum can attest false source events. Limits constrain exposure but do not remove that trust. Seven keys do not prove independent operators.

Native supply is fixed at 1,000,000,000 GOD with 18 decimals. The keeper transfers existing reserve funds, never mints or burns. Initialization requires the full supply in restricted reserve and no unresolved withdrawals; synthetic node balances cannot be relabeled as backed genesis. Native GOD outside reserve must equal attested deposits minus attested finalized payments. Pending withdrawals stay segregated from spendable balances and G redemption liquidity.

## State transitions

A finalized deposit quorum releases the exact matched reserve amount once. A future authenticated native router must verify withdrawal ownership before locking GOD. FIFO authorization requires the 24-hour delay and available rolling capacity. Source payment requires matching approvals and exact custody decrease and recipient increase. Finalized payment returns the locked native GOD to reserve.

Payment and permanent cancellation are mutually exclusive. Only a finalized irreversible cancellation tombstone permits native refund; timeout or cancellation-request signatures alone cannot. Replay binds the asset domain, source transaction and log index, so re-inclusion under a different block hash cannot credit twice. Pauses only restrict directions; they cannot extract assets or resume operations.

Prototype limits are 10,000 GOD exposure, 1,000 GOD per transfer and 1,000 GOD outflow per rolling 24 hours. Midnight is not a reset. Native authorization and source payment reserve capacity at their own timestamps; cancellation does not return native rolling capacity. Queue span and recent-window records are bounded to 1,024 for resource control, not measured production gas capacity. Permanent replay records still grow.

Custody rejects failed, false-return and inexact token transfers. Raw donations do not create escrow credit. Supply and decimals are checked, but actual issuer permissions, rebasing, upgrade powers and RH compatibility require separate review. There is no administrative withdrawal, mint, custody upgrade, signer rotation or resumption path.

## Packet encoding

Packets use SHA-256, not personal signing, EIP-712 or JSON. Integers are unsigned big-endian. ASCII chain names and action tags use uint16 byte-length prefixes. Addresses are raw 20 bytes; amounts and asset bindings are 32 bytes; sequences and heights are uint64; log indexes and nanoseconds are uint32. Signatures are 65 bytes with recovery ID 0 or 1. Supplied approvals must all be valid and distinct.

The domain binds `GOD Chain bridge attestation`, version, source/native names, asset binding and action. Distinct actions identify deposits, withdrawals, authorizations, cancellation requests, finalized outcomes, event identity and pauses. Withdrawal identity includes sequence, sender, recipient, amount and exact queue time. Resolution adds source height, block hash, transaction hash and log index.

The asset domain binds `GOD Chain bridge asset`, version 1, source chain ID, token, custody instance and decimals. A fresh custody deployment has a different binding, not permission to reset replay state. No populated domain configuration is included.

## Verification and integration gates

Private checks exercised compiled custody bytecode and native stores with synthetic assets, including conservation and hostile token behavior. Compiler provenance is in [UPSTREAM.lock.json](UPSTREAM.lock.json): Solidity 0.8.37, optimizer 200, no IR and a Cancun local execution target. Private tests, synthetic-token source, compiler files, artifacts and operational scripts are not distributed. Public Go builds do not rerun those custody checks. See [VERIFICATION.md](VERIFICATION.md).

Authenticated messages, application mounting, backed validators/genesis, finalized-source observation, independent signer operations and recoverable relaying remain required. Custody continuity and governance need additional design. Deployment, bridge funding and real-asset activation require separate explicit authorization.
