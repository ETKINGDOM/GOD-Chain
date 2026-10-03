# GOD Chain core verification

Local verification on 2026-10-03 used macOS arm64 and Go 1.26.8. These are developer-run prototype checks, not a deployed network, independent audit or mainnet safety certification. The private verification suite is not included in this reviewed production-source snapshot.

## Observed local checks

- All 55 named tests passed. Fixtures used the real God SDK account and bank keepers, BaseApp transaction execution and IAVL committed storage with synthetic balances. Signing keys were generated randomly in memory, not stored or published.
- Race detection, Go vet, module checksum verification and formatting checks passed. Native protobuf regeneration reproduced an identical message file.
- Address tests covered native/EVM round trips, role and prefix rejection, checksums and malformed input. Separate earlier 10-second fuzz windows passed 21,998 address round-trip inputs and 27,435 parser inputs; these are inputs, not wallets created.
- Integer reward tests covered pool conservation, pending settlement, duplicate heights, emission bounds, locks, stale quotes, deadlines and failed-payment rollback. The earlier quote fuzz window passed 15,891 executions.
- Native signed messages covered all six enabled operations, wrong chain and account numbers, forged senders, signature/message/fee tampering, unsupported keys and paths, insufficient fees and balances, gas exhaustion, batch rollback, response decoding and owner-scoped unlocks.
- Replay was rejected in the same block, after commitment and after application reload from a committed memory database. This is not a process restart, consensus recovery or disk-crash test.
- Rejected ante-stage execution committed neither fee nor sequence. Accepted transactions retained fees and sequence after business errors or message-stage gas exhaustion, with all G and bank business writes reverted.
- Empty-pool, expired and stale-quote redemptions retained G. Successful redemption paid the beneficiary, burned only offered G and preserved fixed GOD bank supply.
- A malformed-envelope fuzz run found a missing-fee getter panic. Explicit structural validation was added before SDK getters, with a byte regression and twelve structured rejection subcases. A subsequent 15-second decoder fuzz window passed 35,556 inputs; a separate 10-second amount-parser window passed 42,361 inputs.
- Diagnostic, keeper, address and native transaction test binaries compiled with CGO disabled for Linux amd64/arm64, macOS amd64/arm64 and Windows amd64. Only macOS arm64 binaries were executed. Cross-compilation is not an operating-system runtime test.
- The keeper's separate disk database close/reopen check preserved balances, locks and duplicate-height protection. Hardware failure, process crashes, corrupted disks and recovery were not tested.

The diagnostic command reports `nodeReady: false` and `realAssets: false`; `start` exits with an error. No validator network was started and no source-chain backing was verified.

## Scope and reproducibility

Published production source retains the reviewed core implementation, protobuf schema and generated messages, module pins, checksums and attribution. Private tests, fixtures, populated operational data, compiled artifacts and logs are excluded. Build commands compile the public source; they do not reproduce the private test count or its fuzz runs.

Source and reachable-history screening must reject populated addresses, credentials and disallowed files before publication. This screening does not establish the absence of every possible secret or replace independent review.

## Unverified release gates

Persistent wallets, HD key derivation and application integration are absent. Native authentication remains a local restricted prototype. Complete EVM execution and authentication, validator-derived G allocation, consensus hooks, networking, independently verified RH bridging, nonburning penalties, governance and backing conservation remain unfinished.

There is no independent audit, ordinary-computer minimum specification or worldwide-load measurement. Daily account scans, retained lock identifiers, sustainable validator incentives, hostile economics, halt recovery and real-asset custody need separate design and verification before release.
