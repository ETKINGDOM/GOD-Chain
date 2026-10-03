# GOD Chain

GOD Chain is the independent blockchain project for ETERNAL KINGDOM, a global faith-centered world for prayer, confession, praise and fellowship. This repository contains the [Whitepaper Draft](WHITEPAPER.md) and source candidates for an isolated, synthetic-asset prototype. God EVM execution, God SDK staking and GodCometBFT consensus have been connected in local tests. No public network, RH connection or real assets are enabled.

God EVM + God SDK + GodCometBFT are the project component names. Necessary upstream import paths, licenses and attribution remain intact; these names do not claim independent invention of imported infrastructure. English is the default documentation language.

## Execution and validators

`internal/godnode` assembles accounts, bank storage, staking, execution and the local consensus adapter. It accepts bounded legacy, access-list and dynamic-fee Ethereum transactions, including contract deployment and storage. Native direct-signature transactions share account sequences with the Ethereum path. There is no Ethereum JSON-RPC server or wallet interface.

Signed validator creation requires 1,000 GOD self-stake and a fixed 10 percent commission. Delegation requires 1 GOD; unbonding takes 21 days. The prototype selects up to 32 active validators and bounds delegators per validator to 256 for local resource control, not as a finalized worldwide limit. G allocation uses the preceding verified commit and stake captured before that block's transactions. Missed signatures receive no allocation. The per-block budget must be explicitly configured; no mainnet emission rule is selected.

The balance adapter transfers existing GOD instead of granting mint or burn permission. Fees remaining after Ethereum gas refunds enter pending rewards. Native signed fees retain their full-requested-fee rule. Value otherwise discarded by self-destruction-to-self is quarantined, an intentional native-value difference. Default inflation and burning penalties are not imported as economic rules. Misconduct evidence and offline suspension remain unwired; evidence-bearing blocks fail closed, preventing production use. See [NODE_RUNTIME.md](NODE_RUNTIME.md).

## G rewards and fixed GOD supply

G has no fixed lifetime cap. The prototype bounds issuance to 10,000 G per UTC day. Earned G and GOD fees settle at the next UTC-day boundary; skipped days create no catch-up issuance. Settled unclaimed and locked G count in outstanding supply exactly once. Participants cannot submit issuance allocations.

Voluntary redemption pays `floor(P * g / T)` from the settled GOD pool, with at least 1 G, positive minimum output and a consensus-time deadline. Only a successful GOD payment burns the offered G. Empty liquidity, stale quotes and failed payments retain G, which can also be transferred or locked. An accepted transaction can still pay its normal fee on business failure.

GOD bank supply remains 1,000,000,000 with 18 decimal places. Synthetic genesis assigns local balances and holds the remainder in restricted reserve. Equal supply does not prove RH backing. Protected protocol funds cannot be treated as participant balances or redemption liquidity.

## Addresses and authentication

Native accounts use canonical lowercase Bech32 with prefix `god`, shown schematically as `god1…`. EVM compatibility uses a complete EIP-55 address with a `0x` prefix. Both encode the same 20 bytes; conversion creates no wallet and moves no assets. No populated address or operational configuration is supplied.

The native protobuf path supports claim, transfer, lock, unlock, redeem and donate, plus the node's restricted staking messages. It checks the single owner, chain ID, account number, ordered sequence, fee denomination and explicit gas bounds. Ante rejection commits neither fee nor sequence; accepted business failure retains both while reverting message writes. Keeper methods are internal protocol APIs, not unsigned public services.

## Bridge scope

`x/godbridge` and `contracts/GodBridgeEscrow.sol` implement isolated one-for-one ledger and custody state machines with five-of-seven approvals, replay protection, segregated withdrawals, FIFO order, a 24-hour delay, rolling limits and permanent cancellation records. Timeout alone cannot authorize a refund. Quorum attestations are a custody trust model, not cryptographic source-finality proofs.

The bridge is not mounted in the node. Authenticated bridge messages, finalized-source observation, independent signers, backed genesis and a relayer remain absent. No deployment or funding command is provided. See [BRIDGE.md](BRIDGE.md).

## Build and inspect

Versions and tool provenance are recorded in [UPSTREAM.lock.json](UPSTREAM.lock.json); module checksums are in `go.sum`. Local verification used Go 1.26.8 with the recorded JSON-library compatibility override.

```sh
go mod download
make check
make status
```

The diagnostic reports `local-consensus-execution-prototype`, `localNodePrototype: true`, `nodeReady: false` and `realAssets: false`. Startup and financial commands fail closed. A build does not start a chain or verify backing. `make compile-targets` cross-compiles only this diagnostic; foreign-platform compilation is not runtime certification.

Generated protobuf messages are included, so ordinary builds need no protobuf compiler. `make generate-proto` uses protoc 33.0 and the locked generator. Solidity custody source requires the separately verified compiler described in the lock; no compiler, private fixture, bytecode or operational build script is distributed.

Private test suites, fixtures, runtime data, keys, addresses, binaries and logs are intentionally excluded. Public build commands do not reproduce the private verification suite. See [VERIFICATION.md](VERIFICATION.md) for observed checks and their limits.

## Remaining release gates

Durable signing, combined-node restart and crash recovery, adversarial peer tests, misconduct handling, nonburning penalties, suspension and governance remain incomplete. Production emission and gas policy, authenticated bridge integration and backing, wallet support and ordinary-computer resource measurements need separate approval and verification. Daily settlement scans, delegation work and retained tombstones need growth bounds.

There is no independent audit, guaranteed return, mainnet safety claim or launch date. Publication does not authorize chain activation or real assets. Private faith text must be encrypted by the application on the user's device and must not enter public chat, telemetry or logs.

## Attribution

See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). Imported dependency names are technical and legal identifiers, not alternative project branding. Source publication does not complete license review for future binary redistribution.
