# GOD Chain

GOD Chain is the independent blockchain project for ETERNAL KINGDOM, a global faith-centered world for prayer, confession, praise and fellowship. This repository contains the [Whitepaper Draft](WHITEPAPER.md) and an early core source prototype. It is not a running mainnet, EVM node, RH bridge or staking service.

God EVM + God SDK + GodCometBFT are the project component names. Necessary upstream import paths, licenses and attribution are retained; these names do not claim independent invention of imported infrastructure.

## Implemented prototype

The G reward keeper uses God SDK account and bank storage. It implements pending and settled rewards, daily settlement, exact-once reward-height accounting, claiming, transfers, owner-scoped locks and voluntary G-to-GOD redemption. G has no fixed lifetime supply cap. The current network-wide ceiling of 10,000 G per UTC day is a proposed prototype setting, not a production guarantee.

Existing GOD gas fees and voluntary contributions enter pending funds. At the next UTC day boundary, previous earned G and pending GOD settle together. Skipped days create no catch-up issuance. Settled unclaimed and locked G remain in outstanding supply and in the redemption denominator.

Redemption pays `floor(P * g / T)` from the settled GOD pool. It requires at least 1 G, a positive minimum GOD output and a consensus-time deadline. Only successful redemption burns the offered G. Empty liquidity, an expired deadline, a stale minimum output or failed payment preserves G. An accepted signed transaction can still pay its normal GOD fee when its business operation fails.

The keeper never mints or burns GOD. It requires an already initialized bank supply of 1,000,000,000 GOD and keeps bridge reserves separate from reward liquidity. Equal bank supply is not proof of RH backing. Bridge verification, issuance backing and reserve release are not implemented.

## Addresses and native authentication

Native accounts use canonical lowercase Bech32 with prefix `god`, shown schematically as `god1…`. The compatibility encoding remains complete EIP-55 EVM hex with a `0x` prefix. Both encodings map to the same 20-byte account identifier. Conversion creates no wallet, transfers no funds and performs no bridge operation. No populated wallet or contract address is included.

The address codec rejects wrong lengths, checksums, roles, mixed native case and surrounding whitespace. Presentation decoding also accepts all-uppercase Bech32 and canonicalizes it to lowercase; signed native messages require canonical lowercase text.

The native protobuf transaction prototype supports six participant messages: claim G, transfer G, lock G, unlock G, redeem G and donate existing GOD. It reuses God SDK `SIGN_MODE_DIRECT`, EVM-compatible public-key derivation, stored account numbers and ordered sequences. One signer must own all messages in a batch. There is no public issuance, mint, reserve-release, bridge, governance or generic bank-transfer message.

The bounded decoder rejects incomplete fee and signature structures before SDK getters run. The ante chain checks ownership, chain ID, account number, sequence, fee denomination and explicit gas limits. Its fee policy uses integer smallest GOD units per requested gas unit; no mainnet gas price is finalized. Only `agod` fees are accepted. The signed fee is paid in full without an unused-gas refund in this prototype.

Rejected account validation or ante-stage gas exhaustion commits neither fee nor sequence. After ante acceptance, business failure or message-stage gas exhaustion retains the fee and sequence while reverting all business writes. This includes G burns and GOD donations. A batch succeeds or rolls back as a whole; accepted fees enter pending liquidity, not the same day's redemption pool.

Fee grants, alternate fee payers, multisignatures, unordered transactions, transaction extension options, nonempty memos and simulation are disabled. Ethereum-format transactions, personal signing, EIP-712 and MetaMask submission are not enabled by this native protobuf path. Persistent wallets and HD key derivation are not provided.

## Build and inspect

The compatible reference versions are recorded in [UPSTREAM.lock.json](UPSTREAM.lock.json); module checksums are in `go.sum`. Local verification used Go 1.26.8 with the documented JSON-library compatibility override.

```sh
go mod download
go vet -p 2 ./...
go build -trimpath -o build/godd ./cmd/godd
./build/godd status
```

The diagnostic command reports `nodeReady: false` and `realAssets: false`. Startup and financial commands fail closed. Building it does not start a blockchain or demonstrate staking, rewards or live assets.

`make compile-targets` compiles the diagnostic command for Linux amd64/arm64, macOS amd64/arm64 and Windows amd64. Only native macOS execution has been checked locally. Compilation for another platform does not establish runtime support or ordinary-computer minimum requirements.

The local verification record is in [VERIFICATION.md](VERIFICATION.md). This reviewed source snapshot intentionally omits private test suites, fixtures, generated account data, binaries and logs. A build or `go test` over the published packages is not a rerun of the private 55-test suite.

The source schema is `proto/godchain/godrewards/v1/tx.proto`. Generated Go messages are included, so ordinary builds do not need a protobuf compiler. `make generate-proto` uses protoc 33.0 and the locked gocosmos generator; tool provenance is recorded in the dependency lock. Intermediate output remains ignored.

## Remaining release gates

The next work is complete God EVM application assembly and GodCometBFT block-hook integration. Reward allocations must be derived from actual validator signatures and eligible stake; participants cannot supply allocation amounts through a transaction. The reference application's inflation, fee distribution and burning penalties must not become GOD Chain economic rules by default.

EVM execution and authentication, consensus networking, independently verified RH bridging, fixed-supply governance and penalty behavior, sustainable operator incentives, recovery and adversarial economics remain unfinished. Daily account scans and retained lock identifiers need bounded storage and load tests before worldwide use. Ordinary-computer participation is a design objective, not a measured capacity claim.

No independent audit, token price, guaranteed return, mainnet safety claim or launch date is promised. Publishing this source does not activate a chain, authorize real assets or prove backing. Application privacy is a separate integration requirement: private faith text must be encrypted on the user's device and must not enter public logs or telemetry.

## Attribution

See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) for upstream dependency attribution. Source imports and version pins do not complete binary redistribution review. Future distributed binaries require the relevant licenses and notices, including applicable execution-library obligations.

English is the default documentation language. GOD Chain is intended for participants worldwide.
